package booru

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const responseLimit = 8 << 20

type Tag struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Label    string `json:"label,omitempty"`
}

type Post struct {
	ID                string `json:"id"`
	SourceID          string `json:"sourceID"`
	PostURL           string `json:"postURL"`
	PreviewURL        string `json:"previewURL,omitempty"`
	SampleURL         string `json:"sampleURL,omitempty"`
	OriginalURL       string `json:"-"`
	DirectPreviewURL  string `json:"directPreviewURL,omitempty"`
	DirectSampleURL   string `json:"directSampleURL,omitempty"`
	MediaKey          string `json:"mediaKey,omitempty"`
	DownloadAvailable bool   `json:"downloadAvailable"`
	Width             int64  `json:"width,omitempty"`
	Height            int64  `json:"height,omitempty"`
	Bytes             int64  `json:"bytes,omitempty"`
	MD5               string `json:"md5,omitempty"`
	Rating            string `json:"rating,omitempty"`
	Format            string `json:"format,omitempty"`
	Tags              []Tag  `json:"tags"`
	Attachments       []Post `json:"attachments,omitempty"`
	RemoteID          string `json:"remoteID,omitempty"`
	Title             string `json:"title,omitempty"`
	Author            string `json:"author,omitempty"`
	AuthorID          string `json:"authorID,omitempty"`
	Filename          string `json:"filename,omitempty"`
	Position          int    `json:"position,omitempty"`
	Score             int64  `json:"score,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
	Favorite          bool   `json:"favorite,omitempty"`
	MediaNotice       string `json:"mediaNotice,omitempty"`
}

type Result struct {
	SourceID  string    `json:"sourceID"`
	Posts     []Post    `json:"posts"`
	Page      int       `json:"page"`
	Limit     int       `json:"limit"`
	HasMore   bool      `json:"hasMore"`
	Total     *int64    `json:"total,omitempty"`
	FetchedAt time.Time `json:"fetchedAt"`
}

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"error"`
	RetryAfter int    `json:"retryAfter,omitempty"`
}

func (e *Error) Error() string { return e.Message }

type hostGate struct {
	busy       chan struct{}
	last       time.Time // Protected by busy.
	retryUntil time.Time
}

// Client is shared across requests: global concurrency and a one-request/second
// host gate apply even when several clients search the same site. No retries
// occur automatically; a provider's throttle is surfaced for explicit retry.
type Client struct {
	http              *http.Client
	global            chan struct{}
	hosts             sync.Map
	interval          time.Duration
	media             chan struct{}
	autocompleteMu    sync.Mutex
	autocompleteCache map[string]autocompleteEntry
	creatorMu         sync.Mutex
	creatorCache      map[string]creatorEntry
	creatorPending    map[string]chan struct{}
	rule34CategoryMu sync.Mutex
	rule34CategoryCache map[string]rule34CategoryEntry
}

func NewClient() *Client {
	transport := &http.Transport{
		// Pin validated public DNS results at connect time. Environment proxies
		// could resolve a different destination and bypass this boundary.
		DialContext:           publicDialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   2,
		MaxConnsPerHost:       2,
		IdleConnTimeout:       30 * time.Second,
	}
	return &Client{http: &http.Client{Transport: transport, Timeout: 15 * time.Second}, global: make(chan struct{}, 4), interval: time.Second, media: make(chan struct{}, 4)}
}

func publicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid provider address")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("provider DNS lookup failed")
	}
	return dialPublicAddresses(ctx, network, port, ips, (&net.Dialer{Timeout: 5 * time.Second}).DialContext)
}

func dialPublicAddresses(ctx context.Context, network, port string, ips []netip.Addr, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	for _, ip := range ips {
		if !publicAddress(ip) {
			return nil, fmt.Errorf("provider resolved to a non-public address")
		}
	}
	for _, ip := range ips {
		conn, err := dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("provider connection failed")
}

func ValidateQuery(query string, page, limit int) error {
	if len(query) > 1024 || strings.ContainsAny(query, "\x00\r\n") {
		return &Error{Code: "invalid-query", Message: "Enter a search of at most 1024 bytes without control lines."}
	}
	if page < 1 || page > MaxPage || limit < 1 || limit > MaxLimit {
		return &Error{Code: "invalid-page", Message: "Page must be 1–750 and page size 1–50."}
	}
	return nil
}

func (s Source) request(ctx context.Context, query string, page, limit int, options ...SearchOptions) (*http.Request, error) {
	base := s.rule34APIBase()
	path := "/posts.json"
	values := url.Values{"tags": {query}, "page": {strconv.Itoa(page)}, "limit": {strconv.Itoa(limit)}}
	switch s.Engine {
	case "gelbooru":
		path = "/index.php"
		values = url.Values{"page": {"dapi"}, "s": {"post"}, "q": {"index"}, "json": {"1"}, "tags": {query}, "pid": {strconv.Itoa(page - 1)}, "limit": {strconv.Itoa(limit)}}
		if s.APIKey != "" {
			values.Set("api_key", s.APIKey)
			values.Set("user_id", s.UserID)
		}
	case "moebooru":
		path = "/post.json"
	case "philomena":
		path = "/api/v1/json/search/images"
		values = url.Values{"q": {query}, "page": {strconv.Itoa(page)}, "per_page": {strconv.Itoa(limit)}, "sf": {"id"}, "sd": {"desc"}}
		if len(options) > 0 {
			if options[0].Sort == "popular" || options[0].Sort == "score_desc" || options[0].Sort == "score_asc" {
				values.Set("sf", "score")
			}
			if options[0].Sort == "oldest" || options[0].Sort == "score_asc" {
				values.Set("sd", "asc")
			}
		}
		if s.APIKey != "" {
			values.Set("key", s.APIKey)
		}
	case "e621":
		values.Set("v2", "true")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path+"?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("invalid provider request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "StashBooru/1.0 (https://github.com/Dusky-dev/StashBooru)")
	if (s.Engine == "danbooru" || s.Engine == "e621") && s.APIKey != "" {
		req.SetBasicAuth(s.Login, s.APIKey)
	}
	return req, nil
}

func (c *Client) Search(ctx context.Context, source Source, query string, page, limit int) (Result, error) {
	return c.SearchFiltered(ctx, source, query, page, limit, SearchOptions{})
}
func (c *Client) SearchFiltered(ctx context.Context, source Source, query string, page, limit int, options SearchOptions) (Result, error) {
	if err := source.Validate(); err != nil {
		return Result{}, &Error{Code: "configuration", Message: "The source configuration is invalid."}
	}
	if err := ValidateQuery(query, page, limit); err != nil {
		return Result{}, err
	}
	if source.isRule34() && (source.APIKey == "" || source.UserID == "") {
		return Result{}, &Error{Code: "authentication", Message: "Rule34.xxx requires an API key and user ID. Sign in at rule34.xxx, open Account > Options > API Access Credentials, generate a key if needed, then save both values in Booru Settings > Sources."}
	}
	if source.Engine == "pixiv" || source.Engine == "kemono" || source.Engine == "pawchive" || source.Engine == "shimmie" {
		return c.searchExtended(ctx, source, query, page, limit, options)
	}
	query, err := source.filterQuery(query, options)
	if err != nil {
		return Result{}, err
	}
	if err = ValidateQuery(query, page, limit); err != nil {
		return Result{}, err
	}
	req, err := source.request(ctx, query, page, limit, options)
	if err != nil {
		return Result{}, &Error{Code: "configuration", Message: "Cannot construct the provider request."}
	}
	data, err := c.fetchJSON(ctx, source, req, responseLimit)
	if err != nil {
		return Result{}, err
	}
	result, err := parse(source, data, page, limit)
	if err != nil {
		return Result{}, &Error{Code: "invalid-response", Message: "Provider returned an unsupported or invalid JSON response. Its API may have changed or rejected the search."}
	}
	result.FetchedAt = time.Now().UTC()
	return result, nil
}

func requestError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return &Error{Code: "cancelled", Message: "Search cancelled."}
	}
	return &Error{Code: "unavailable", Message: "Provider could not be reached within the search deadline. Check server network access and try again."}
}

func retryDelay(raw string) int {
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		if until, dateErr := http.ParseTime(raw); dateErr == nil {
			seconds = int(time.Until(until).Seconds()) + 1
		}
	}
	return max(0, min(seconds, 3600))
}

// fetchJSON shares search/autocomplete host throttles, cancellation and redirect policy.
func (c *Client) fetchJSON(ctx context.Context, source Source, req *http.Request, maxBytes int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// Throttle by the actual request host. Rule34 API requests use a separate
	// host from its human-facing website.
	host := strings.ToLower(strings.TrimSuffix(req.URL.Hostname(), "."))
	entry, _ := c.hosts.LoadOrStore(host, &hostGate{busy: make(chan struct{}, 1)})
	gate := entry.(*hostGate)
	select {
	case gate.busy <- struct{}{}:
		defer func() { <-gate.busy }()
	case <-ctx.Done():
		return nil, requestError(ctx)
	}
	if remaining := time.Until(gate.retryUntil); remaining > 0 {
		return nil, &Error{Code: "rate-limited", Message: "Provider requested a pause before retrying.", RetryAfter: int(remaining.Seconds()) + 1}
	}
	timer := time.NewTimer(time.Until(gate.last.Add(c.interval)))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return nil, requestError(ctx)
	}
	select {
	case c.global <- struct{}{}:
		defer func() { <-c.global }()
	case <-ctx.Done():
		return nil, requestError(ctx)
	}
	gate.last = time.Now()
	req = req.Clone(ctx)
	client := *c.http
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		// Never forward credentials or follow a provider's arbitrary redirect.
		return http.ErrUseLastResponse
	}
	response, err := client.Do(req)
	if err != nil {
		// Do not return url.Error: its URL may contain API credentials.
		return nil, requestError(ctx)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		failure := &Error{Code: "provider-error", Message: fmt.Sprintf("Provider returned HTTP %d. Try again later or check its API requirements.", response.StatusCode)}
		switch response.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			failure.Code, failure.Message = "authentication", "Provider denied access. Check server-side API credentials, account restrictions or its current API policy."
			if source.isRule34() {
				failure.Message = "Rule34.xxx rejected the API request. Check your user ID and API key in Booru Settings > Sources. If both are correct, Rule34 may be blocking the request through Cloudflare or its account/rate restrictions (HTTP " + strconv.Itoa(response.StatusCode) + ")."
			}
		case http.StatusTooManyRequests:
			failure.Code, failure.Message = "rate-limited", "Provider rate limit reached. Wait before retrying."
			failure.RetryAfter = retryDelay(response.Header.Get("Retry-After"))
			gate.retryUntil = time.Now().Add(time.Duration(failure.RetryAfter) * time.Second)
		}
		return nil, failure
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(maxBytes)+1))
	if err != nil || len(data) > maxBytes {
		return nil, &Error{Code: "invalid-response", Message: "Provider response could not be read or exceeded its size limit."}
	}
	return data, nil
}
