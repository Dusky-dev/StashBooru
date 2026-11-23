package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/logger"
)

// errBooruRateLimited reports that a site asked StashBooru to slow down.
var errBooruRateLimited = errors.New("site rate limit reached")

// errBooruNotChecked means at least one chosen site could not answer, so a
// missing match is not proof that no site has the post.
var errBooruNotChecked = errors.New("not every MD5 lookup site could be checked")

// A bulk job asks each site about one request per second. It stops asking a
// site for the rest of the job once it rate limits, rejects the saved
// credentials, or fails this many times in a row.
const (
	bulkBooruHostInterval   = time.Second
	bulkBooruFailuresToDrop = 3
)

type booruHostGatesKey struct{}

// booruHostGates spaces requests to each host. A nil gate set (single-Image
// lookups) does not wait.
type booruHostGates struct {
	mu       sync.Mutex
	interval time.Duration
	next     map[string]time.Time
	blocked  map[string]bool
}

func newBooruHostGates(interval time.Duration) *booruHostGates {
	return &booruHostGates{interval: interval, next: map[string]time.Time{}, blocked: map[string]bool{}}
}

func withBooruHostGates(ctx context.Context, gates *booruHostGates) context.Context {
	return context.WithValue(ctx, booruHostGatesKey{}, gates)
}

func booruHostGatesFrom(ctx context.Context) *booruHostGates {
	gates, _ := ctx.Value(booruHostGatesKey{}).(*booruHostGates)
	return gates
}

func booruHostKey(host string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSuffix(host, ".")), "www.")
}

// wait reserves the host's next request slot and sleeps until it.
func (g *booruHostGates) wait(ctx context.Context, host string) error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	if g.blocked[host] {
		g.mu.Unlock()
		return errBooruRateLimited
	}
	now := time.Now()
	slot := g.next[host]
	if slot.Before(now) {
		slot = now
	}
	g.next[host] = slot.Add(g.interval)
	g.mu.Unlock()

	timer := time.NewTimer(time.Until(slot))
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *booruHostGates) block(host string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.blocked[host] = true
	g.mu.Unlock()
}

func isBooruRateLimited(err error) bool {
	if errors.Is(err, errBooruRateLimited) {
		return true
	}
	var failure *booru.Error
	return errors.As(err, &failure) && failure.Code == "rate-limited"
}

type bulkBooruResult struct {
	provider booruProvider
	post     *booruPost
	err      error
}

// bulkBooruLookup asks the chosen sites one at a time, in priority order, and
// stops at the first exact match. Results are reused for repeated hashes and
// tag categories are remembered for the whole job.
type bulkBooruLookup struct {
	providers  []booruProvider
	gates      *booruHostGates
	dropped    map[string]bool
	failures   map[string]int
	results    map[string]bulkBooruResult
	categories map[string]map[string]string
	// uncategorized counts tags skipped because the site gave no category.
	uncategorized int
}

func newBulkBooruLookup(providers []booruProvider) *bulkBooruLookup {
	return &bulkBooruLookup{
		providers:  providers,
		gates:      newBooruHostGates(bulkBooruHostInterval),
		dropped:    map[string]bool{},
		failures:   map[string]int{},
		results:    map[string]bulkBooruResult{},
		categories: map[string]map[string]string{},
	}
}

// hasPostLink reports whether one of the urls is a post link on a chosen site.
func (l *bulkBooruLookup) hasPostLink(urls []string) bool {
	for _, raw := range urls {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		host := booruHostKey(parsed.Hostname())
		for _, provider := range l.providers {
			for _, providerHost := range provider.hosts {
				if host == providerHost || strings.HasSuffix(host, "."+providerHost) {
					return true
				}
			}
		}
	}
	return false
}

func (l *bulkBooruLookup) lookup(ctx context.Context, hash string) (booruProvider, *booruPost, error) {
	hash = strings.ToLower(hash)
	if cached, ok := l.results[hash]; ok {
		return cached.provider, cached.post, cached.err
	}
	result := l.ask(withBooruHostGates(ctx, l.gates), hash)
	if ctx.Err() == nil {
		l.results[hash] = result
	}
	return result.provider, result.post, result.err
}

func (l *bulkBooruLookup) ask(ctx context.Context, hash string) bulkBooruResult {
	incomplete := false
	for _, provider := range l.providers {
		if l.dropped[provider.name] {
			incomplete = true
			continue
		}
		requestCtx, cancel := context.WithTimeout(ctx, booruLookupTimeout)
		post, err := fetchBooruProvider(requestCtx, provider, hash)
		cancel()
		switch {
		case err == nil:
			l.failures[provider.name] = 0
			return bulkBooruResult{provider: provider, post: post}
		case errors.Is(err, errBooruNoMatch):
			l.failures[provider.name] = 0
		case ctx.Err() != nil:
			return bulkBooruResult{err: ctx.Err()}
		case isBooruRateLimited(err):
			l.drop(provider.name, "rate limited MD5 lookups")
			incomplete = true
		case isBooruAuthenticationError(err):
			l.drop(provider.name, "rejected the saved credentials")
			incomplete = true
		default:
			incomplete = true
			l.failures[provider.name]++
			logger.Debugf("Image Tagging: MD5 lookup on %s failed: %v", provider.name, err)
			if l.failures[provider.name] >= bulkBooruFailuresToDrop {
				l.drop(provider.name, fmt.Sprintf("failed %d times in a row (%v)", bulkBooruFailuresToDrop, err))
			}
		}
	}
	if incomplete {
		return bulkBooruResult{err: errBooruNotChecked}
	}
	return bulkBooruResult{err: errBooruNoMatch}
}

func (l *bulkBooruLookup) drop(name, reason string) {
	l.dropped[name] = true
	logger.Warnf("Image Tagging: %s %s; it is skipped for the rest of this job", name, reason)
}

func isBooruAuthenticationError(err error) bool {
	var failure *booru.Error
	return errors.As(err, &failure) && failure.Code == "authentication"
}

// tagCategories resolves a match's flat tag names, asking the site only about
// names this job has not seen from it yet.
func (l *bulkBooruLookup) tagCategories(ctx context.Context, provider booruProvider, post *booruPost) map[string]string {
	names := flatBooruTagNames(post)
	if len(names) == 0 || provider.parseTagCategories == nil {
		return nil
	}
	known := l.categories[provider.name]
	if known == nil {
		known = map[string]string{}
		l.categories[provider.name] = known
	}
	var missing []string
	for _, name := range names {
		if _, ok := known[strings.ToLower(name)]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		// Spaced requests take longer than a single-Image lookup allows.
		categoryCtx, cancel := context.WithTimeout(withBooruHostGates(ctx, l.gates), 2*time.Minute)
		resolved := resolveBooruTagCategoryNames(categoryCtx, provider, missing)
		cancel()
		// Only answers are remembered: a name the site did not answer is
		// asked again for the next Image that has it.
		for name, category := range resolved {
			known[strings.ToLower(name)] = category
		}
	}
	result := make(map[string]string, len(names))
	for _, name := range names {
		if category := known[strings.ToLower(name)]; category != "" {
			result[strings.ToLower(name)] = category
		}
	}
	return result
}

// bulkBooruPredictions turns a match into predictions. Tags the site gave no
// category for are left out: without a reviewer, a Character or Artist
// name must never be written as an ordinary Tag.
func (l *bulkBooruLookup) bulkBooruPredictions(ctx context.Context, provider booruProvider, post *booruPost) []camietagger.Tag {
	uncategorized := map[string]bool{}
	for name := range post.Uncategorized {
		uncategorized[name] = true
	}
	var categories map[string]string
	if names := flatBooruTagNames(post); len(names) > 0 {
		categories = l.tagCategories(ctx, provider, post)
		for _, name := range names {
			if categories[strings.ToLower(name)] == "" {
				uncategorized[strings.ToLower(name)] = true
			}
		}
	}
	predictions := booruTags(post, provider.name, categories)
	kept := predictions[:0]
	skipped := 0
	for _, prediction := range predictions {
		if uncategorized[strings.ToLower(prediction.RawName)] {
			skipped++
			continue
		}
		kept = append(kept, normalizeCamiePrediction(prediction))
	}
	l.uncategorized += skipped
	if skipped > 0 {
		logger.Debugf("Image Tagging: %s gave no category for %d tag(s) of post %s; they were not applied", provider.name, skipped, post.ID)
	}
	return kept
}
