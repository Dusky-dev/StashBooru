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
}

type Post struct {
	ID         string `json:"id"`
	SourceID   string `json:"sourceID"`
	PostURL    string `json:"postURL"`
	PreviewURL string `json:"previewURL,omitempty"`
	SampleURL  string `json:"sampleURL,omitempty"`
	Width      int64  `json:"width,omitempty"`
	Height     int64  `json:"height,omitempty"`
	Bytes      int64  `json:"bytes,omitempty"`
	MD5        string `json:"md5,omitempty"`
	Rating     string `json:"rating,omitempty"`
	Format     string `json:"format,omitempty"`
	Tags       []Tag  `json:"tags"`
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
	http     *http.Client
	global   chan struct{}
	hosts    sync.Map
	interval time.Duration
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
	return &Client{http: &http.Client{Transport: transport, Timeout: 15 * time.Second}, global: make(chan struct{}, 4), interval: time.Second}
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
	if strings.TrimSpace(query) == "" || len(query) > 1024 || strings.ContainsAny(query, "\x00\r\n") {
		return &Error{Code: "invalid-query", Message: "Enter a search of 1–1024 bytes without control lines."}
	}
	if page < 1 || page > MaxPage || limit < 1 || limit > MaxLimit {
		return &Error{Code: "invalid-page", Message: "Page must be 1–750 and page size 1–50."}
	}
	return nil
}

func (s Source) request(ctx context.Context, query string, page, limit int) (*http.Request, error) {
	base := strings.TrimRight(s.URL, "/")
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
	if err := source.Validate(); err != nil {
		return Result{}, &Error{Code: "configuration", Message: "The source configuration is invalid."}
	}
	if err := ValidateQuery(query, page, limit); err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	base, _ := url.Parse(source.URL)
	host := strings.ToLower(strings.TrimSuffix(base.Hostname(), "."))
	entry, _ := c.hosts.LoadOrStore(host, &hostGate{busy: make(chan struct{}, 1)})
	gate := entry.(*hostGate)
	select {
	case gate.busy <- struct{}{}:
		defer func() { <-gate.busy }()
	case <-ctx.Done():
		return Result{}, requestError(ctx)
	}
	if remaining := time.Until(gate.retryUntil); remaining > 0 {
		return Result{}, &Error{Code: "rate-limited", Message: "Provider requested a pause before retrying.", RetryAfter: int(remaining.Seconds()) + 1}
	}
	timer := time.NewTimer(time.Until(gate.last.Add(c.interval)))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return Result{}, requestError(ctx)
	}
	select {
	case c.global <- struct{}{}:
		defer func() { <-c.global }()
	case <-ctx.Done():
		return Result{}, requestError(ctx)
	}
	gate.last = time.Now()
	req, err := source.request(ctx, query, page, limit)
	if err != nil {
		return Result{}, &Error{Code: "configuration", Message: "Cannot construct the provider request."}
	}
	client := *c.http
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		// Never forward credentials or follow a provider's arbitrary redirect.
		return http.ErrUseLastResponse
	}
	response, err := client.Do(req)
	if err != nil {
		// Do not return url.Error: its URL may contain API credentials.
		return Result{}, requestError(ctx)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		failure := &Error{Code: "provider-error", Message: fmt.Sprintf("Provider returned HTTP %d. Try again later or check its API requirements.", response.StatusCode)}
		switch response.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			failure.Code, failure.Message = "authentication", "Provider denied access. Check server-side API credentials, account restrictions or its current API policy."
		case http.StatusTooManyRequests:
			failure.Code, failure.Message = "rate-limited", "Provider rate limit reached. Wait before retrying."
			failure.RetryAfter = retryDelay(response.Header.Get("Retry-After"))
			gate.retryUntil = time.Now().Add(time.Duration(failure.RetryAfter) * time.Second)
		}
		return Result{}, failure
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil || len(data) > responseLimit {
		return Result{}, &Error{Code: "invalid-response", Message: "Provider response could not be read or exceeded 8 MiB."}
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
