// Package characterdedup conservatively plans and applies Character identity merges.
package characterdedup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/stashapp/stash/pkg/models"
	"golang.org/x/text/unicode/norm"
)

type Character struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type Group struct {
	Destination Character   `json:"destination"`
	Sources     []Character `json:"sources"`
	Copyrights  []Character `json:"copyrights"`
	Reason      string      `json:"reason"`
}
type Skipped struct {
	Character Character `json:"character"`
	Reason    string    `json:"reason"`
}
type Plan struct {
	Fingerprint string    `json:"fingerprint"`
	Groups      []Group   `json:"groups"`
	Skipped     []Skipped `json:"skipped"`
	Scanned     int       `json:"scanned"`
}
type record struct {
	Performer  *models.Performer
	Copyrights []Character
	Fields     map[string]interface{}
	Image      []byte // SHA-256 digest; do not retain all portraits in memory.
	ImageID    int
}

func words(name string) []string {
	return strings.FieldsFunc(strings.ToLower(norm.NFKC.String(strings.TrimSpace(name))), func(r rune) bool { return unicode.IsSpace(r) || r == '_' || r == ',' })
}
func key(name string) string {
	parts := words(name)
	if len(parts) == 2 {
		sort.Strings(parts)
	}
	return strings.Join(parts, " ")
}
func identity(p *models.Performer) Character { return Character{p.ID, p.Name} }
func scope(r record) string {
	ids := []int{}
	for _, c := range r.Copyrights {
		ids = append(ids, c.ID)
	}
	sort.Ints(ids)
	return fmt.Sprint(ids)
}

func load(ctx context.Context, repo models.Repository) ([]record, error) {
	// Bound both the preview response and a single atomic apply transaction.
	limit := 10001
	ps, _, err := repo.Performer.Query(ctx, nil, &models.FindFilterType{PerPage: &limit})
	if err != nil {
		return nil, err
	}
	if len(ps) > 10000 {
		return nil, errors.New("Character merge preview supports up to 10000 Characters per library")
	}
	slices.SortFunc(ps, func(a, b *models.Performer) int { return a.ID - b.ID })
	result := make([]record, 0, len(ps))
	for _, p := range ps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := p.LoadRelationships(ctx, repo.Performer); err != nil {
			return nil, err
		}
		if err := p.LoadURLs(ctx, repo.Performer); err != nil {
			return nil, err
		}
		cs, err := repo.Copyright.FindByPerformerID(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		copyrights := []Character{}
		for _, c := range cs {
			copyrights = append(copyrights, Character{c.ID, c.Name})
		}
		slices.SortFunc(copyrights, func(a, b Character) int { return a.ID - b.ID })
		fields, err := repo.Performer.GetCustomFields(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		image, err := repo.Performer.GetImage(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		var digest []byte
		if len(image) > 0 {
			sum := sha256.Sum256(image)
			digest = sum[:]
		}
		result = append(result, record{Performer: p, Copyrights: copyrights, Fields: fields, Image: digest, ImageID: p.ID})
	}
	return result, nil
}

// merge combines only compatible profiles. It never silently picks between
// conflicting scalar/custom fields or different portraits.
func merge(destination, source record) (record, error) {
	d := *destination.Performer
	if source.Performer.CreatedAt.Before(d.CreatedAt) {
		d.CreatedAt = source.Performer.CreatedAt
	}
	sv, dv := reflect.ValueOf(*source.Performer), reflect.ValueOf(&d).Elem()
	for i := 0; i < dv.NumField(); i++ {
		name := dv.Type().Field(i).Name
		switch name {
		case "ID", "Name", "CreatedAt", "UpdatedAt", "Aliases", "URLs", "TagIDs", "StashIDs":
			continue
		case "Favorite", "IgnoreAutoTag":
			dv.Field(i).SetBool(dv.Field(i).Bool() || sv.Field(i).Bool())
			continue
		}
		a, b := dv.Field(i), sv.Field(i)
		if !a.IsZero() && !b.IsZero() && !reflect.DeepEqual(a.Interface(), b.Interface()) {
			return record{}, fmt.Errorf("conflicting %s", name)
		}
		if a.IsZero() {
			a.Set(b)
		}
	}
	fields := map[string]interface{}{}
	for k, v := range destination.Fields {
		fields[k] = v
	}
	for k, v := range source.Fields {
		if existing, ok := fields[k]; ok && !reflect.DeepEqual(existing, v) {
			return record{}, fmt.Errorf("conflicting custom field %s", k)
		}
		fields[k] = v
	}
	if len(destination.Image) > 0 && len(source.Image) > 0 && !bytes.Equal(destination.Image, source.Image) {
		return record{}, errors.New("different portraits; review manually")
	}
	portrait := destination.Image
	imageID := destination.ImageID
	if len(portrait) == 0 {
		portrait = source.Image
		imageID = source.ImageID
	}
	aliases := append([]string{}, d.Aliases.List()...)
	// Former canonical names are qualified, so adding a shortened name does not
	// introduce an unqualified auto-tag alias across unrelated Copyrights.
	for _, c := range destination.Copyrights {
		aliases = append(aliases, source.Performer.Name+" ("+c.Name+")")
	}
	aliases = append(aliases, source.Performer.Aliases.List()...)
	slices.Sort(aliases)
	aliases = slices.Compact(aliases)
	d.Aliases = models.NewRelatedStrings(aliases)
	urls := append(append([]string{}, d.URLs.List()...), source.Performer.URLs.List()...)
	slices.Sort(urls)
	d.URLs = models.NewRelatedStrings(slices.Compact(urls))
	tags := append(append([]int{}, d.TagIDs.List()...), source.Performer.TagIDs.List()...)
	slices.Sort(tags)
	d.TagIDs = models.NewRelatedIDs(slices.Compact(tags))
	stash := append([]models.StashID{}, d.StashIDs.List()...)
	for _, sid := range source.Performer.StashIDs.List() {
		found := false
		for _, old := range stash {
			if old.Endpoint == sid.Endpoint {
				if old.StashID != sid.StashID {
					return record{}, errors.New("conflicting Stash IDs")
				}
				found = true
			}
		}
		if !found {
			stash = append(stash, sid)
		}
	}
	d.StashIDs = models.NewRelatedStashIDs(stash)
	return record{Performer: &d, Copyrights: destination.Copyrights, Fields: fields, Image: portrait, ImageID: imageID}, nil
}

func plan(records []record) (Plan, error) {
	p := Plan{Groups: []Group{}, Skipped: []Skipped{}, Scanned: len(records)}
	// Snapshot includes all candidates, aliases, profiles, Copyrights and portraits.
	// A new ambiguous Character or changed context invalidates the entire review.
	// Related model containers keep their values private; include them explicitly.
	type snapshotRecord struct {
		record
		Aliases, URLs []string
		Tags          []int
		StashIDs      []models.StashID
	}
	snapshots := make([]snapshotRecord, 0, len(records))
	for _, r := range records {
		snapshots = append(snapshots, snapshotRecord{r, r.Performer.Aliases.List(), r.Performer.URLs.List(), r.Performer.TagIDs.List(), r.Performer.StashIDs.List()})
	}
	snapshot, err := json.Marshal(snapshots)
	if err != nil {
		return p, err
	}
	digest := sha256.Sum256(snapshot)
	p.Fingerprint = hex.EncodeToString(digest[:])
	parents := map[int]bool{}
	for _, r := range records {
		if r.Performer.ParentID != nil {
			parents[*r.Performer.ParentID] = true
		}
	}
	full := map[string][]record{}
	eligible := map[int]bool{}
	short := []record{}
	skip := func(r record, reason string) { p.Skipped = append(p.Skipped, Skipped{identity(r.Performer), reason}) }
	for _, r := range records {
		v := r.Performer
		if len(r.Copyrights) == 0 {
			skip(r, "no directly assigned Copyright")
			continue
		}
		if v.ParentID != nil || parents[v.ID] {
			skip(r, "variant or Character with variants")
			continue
		}
		if v.DisambiguationCopyrightID != nil {
			found := false
			for _, c := range r.Copyrights {
				found = found || c.ID == *v.DisambiguationCopyrightID
			}
			if !found {
				skip(r, "disambiguation Copyright differs from assigned Copyrights")
				continue
			}
		}
		eligible[v.ID] = true
		w := words(v.Name)
		if len(w) < 2 {
			short = append(short, r)
			continue
		}
		k := scope(r) + "/" + key(v.Name)
		full[k] = append(full[k], r)
	}
	// Short names attach only to exactly one full-name identity in the same exact
	// Copyright set. Never connect two distinct full names through a shared token.
	index := map[string]map[string]bool{}
	// Even ineligible profiles must count toward short-name ambiguity.
	for _, r := range records {
		if len(words(r.Performer.Name)) < 2 {
			continue
		}
		k := scope(r) + "/" + key(r.Performer.Name)
		if !eligible[r.Performer.ID] {
			k += fmt.Sprintf("/excluded/%d", r.Performer.ID)
		}
		for _, part := range words(r.Performer.Name) {
			token := scope(r) + "/" + part
			if index[token] == nil {
				index[token] = map[string]bool{}
			}
			index[token][k] = true
		}
	}
	for _, r := range short {
		name := strings.Join(words(r.Performer.Name), " ")
		matches := index[scope(r)+"/"+name]
		if utf8.RuneCountInString(name) < 3 || len(matches) != 1 {
			skip(r, "short name is ambiguous or has no unique full-name match")
			continue
		}
		for k := range matches {
			if len(full[k]) == 0 {
				skip(r, "full-name match is not eligible for automatic merging")
			} else {
				full[k] = append(full[k], r)
			}
		}
	}
	keys := []string{}
	for k := range full {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		group := full[k]
		if len(group) < 2 {
			continue
		}
		// Prefer a full name, then the lowest stable catalogue ID.
		slices.SortFunc(group, func(a, b record) int {
			na, nb := len(words(a.Performer.Name)), len(words(b.Performer.Name))
			if na != nb {
				return nb - na
			}
			return a.Performer.ID - b.Performer.ID
		})
		merged := group[0]
		conflict := ""
		for _, source := range group[1:] {
			merged, err = merge(merged, source)
			if err != nil {
				conflict = err.Error()
				break
			}
		}
		if conflict != "" {
			for _, r := range group {
				skip(r, conflict)
			}
			continue
		}
		g := Group{Destination: identity(group[0].Performer), Copyrights: group[0].Copyrights, Reason: "same Copyrights and full-name identity", Sources: []Character{}}
		for _, r := range group[1:] {
			g.Sources = append(g.Sources, identity(r.Performer))
			if len(words(r.Performer.Name)) == 1 {
				g.Reason = "same Copyrights and unique shortened name"
			}
		}
		p.Groups = append(p.Groups, g)
	}
	return p, nil
}

func Preview(ctx context.Context, repo models.Repository) (Plan, error) {
	var result Plan
	err := repo.WithReadTxn(ctx, func(ctx context.Context) error {
		rs, err := load(ctx, repo)
		if err != nil {
			return err
		}
		result, err = plan(rs)
		return err
	})
	return result, err
}

// Apply revalidates and merges a reviewed batch in one transaction. Cancellation
// or any failure rolls the entire batch back, including transferred relations.
func Apply(ctx context.Context, repo models.Repository, fingerprint string, progress func(int, int)) error {
	return repo.WithTxn(ctx, func(ctx context.Context) error {
		rs, err := load(ctx, repo)
		if err != nil {
			return err
		}
		p, err := plan(rs)
		if err != nil {
			return err
		}
		if fingerprint == "" || fingerprint != p.Fingerprint {
			return errors.New("Character catalogue changed; preview again before merging")
		}
		byID := map[int]record{}
		for _, r := range rs {
			byID[r.Performer.ID] = r
		}
		for i, g := range p.Groups {
			if err := ctx.Err(); err != nil {
				return err
			}
			merged := byID[g.Destination.ID]
			ids := []int{}
			for _, source := range g.Sources {
				merged, err = merge(merged, byID[source.ID])
				if err != nil {
					return err
				}
				ids = append(ids, source.ID)
			}
			var portrait []byte
			if len(merged.Image) > 0 && merged.ImageID != g.Destination.ID {
				portrait, err = repo.Performer.GetImage(ctx, merged.ImageID)
				if err != nil {
					return err
				}
			}
			if err := repo.Performer.Merge(ctx, ids, g.Destination.ID); err != nil {
				return err
			}
			merged.Performer.UpdatedAt = time.Now()
			if err := repo.Performer.Update(ctx, &models.UpdatePerformerInput{Performer: merged.Performer, CustomFields: models.CustomFieldsInput{Full: merged.Fields}}); err != nil {
				return err
			}
			if len(portrait) > 0 {
				if err := repo.Performer.UpdateImage(ctx, g.Destination.ID, portrait); err != nil {
					return err
				}
			}
			if progress != nil {
				progress(i+1, len(p.Groups))
			}
		}
		return ctx.Err()
	})
}
