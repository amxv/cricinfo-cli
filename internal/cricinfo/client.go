package cricinfo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/amxv/cricinfo-cli/internal/buildinfo"
)

const (
	DefaultBaseURL         = "http://core.espnuk.org/v2/sports/cricket"
	defaultTimeout         = 8 * time.Second
	defaultMaxRetries      = 3
	defaultRetryBaseDelay  = 200 * time.Millisecond
	defaultRetryMaxDelay   = 2 * time.Second
	defaultRetryJitter     = 0.25
	defaultMaxRefHops      = 8
	defaultRetryStatusCode = 500
)

var (
	ErrMissingRef        = errors.New("missing ref")
	ErrPointerLoop       = errors.New("pointer loop detected")
	ErrPointerChainLimit = errors.New("pointer chain exceeded max hops")
)

// Config controls transport behavior for Cricinfo requests.
type Config struct {
	BaseURL        string
	HTTPClient     *http.Client
	Timeout        time.Duration
	MaxRetries     int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	RetryJitter    float64
	UserAgent      string
	MaxRefHops     int
	Sleep          func(context.Context, time.Duration) error
	RandomFloat64  func() float64
}

// HTTPStatusError captures non-2xx responses.
type HTTPStatusError struct {
	URL        string
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("request %q returned status %d", e.URL, e.StatusCode)
	}
	return fmt.Sprintf("request %q returned status %d: %s", e.URL, e.StatusCode, e.Body)
}

// Client is a Cricinfo HTTP transport with retry and ref traversal helpers.
type Client struct {
	baseURL        *url.URL
	httpClient     *http.Client
	timeout        time.Duration
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
	retryJitter    float64
	userAgent      string
	maxRefHops     int
	sleep          func(context.Context, time.Duration) error
	randomFloat64  func() float64
}

// NewClient creates a configured Cricinfo client.
func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if !parsedBase.IsAbs() {
		return nil, fmt.Errorf("base url must be absolute: %q", baseURL)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = defaultMaxRetries
	}

	retryBaseDelay := cfg.RetryBaseDelay
	if retryBaseDelay <= 0 {
		retryBaseDelay = defaultRetryBaseDelay
	}

	retryMaxDelay := cfg.RetryMaxDelay
	if retryMaxDelay <= 0 {
		retryMaxDelay = defaultRetryMaxDelay
	}
	if retryMaxDelay < retryBaseDelay {
		retryMaxDelay = retryBaseDelay
	}

	retryJitter := cfg.RetryJitter
	if retryJitter < 0 {
		retryJitter = 0
	}
	if retryJitter > 1 {
		retryJitter = 1
	}
	if cfg.RetryJitter == 0 {
		retryJitter = defaultRetryJitter
	}

	userAgent := strings.TrimSpace(cfg.UserAgent)
	if userAgent == "" {
		userAgent = fmt.Sprintf("cricinfo-cli/%s", buildinfo.CurrentVersion())
	}

	maxRefHops := cfg.MaxRefHops
	if maxRefHops <= 0 {
		maxRefHops = defaultMaxRefHops
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	sleep := cfg.Sleep
	if sleep == nil {
		sleep = defaultSleep
	}

	randomFloat64 := cfg.RandomFloat64
	if randomFloat64 == nil {
		randomFloat64 = rand.Float64
	}

	return &Client{
		baseURL:        parsedBase,
		httpClient:     httpClient,
		timeout:        timeout,
		maxRetries:     maxRetries,
		retryBaseDelay: retryBaseDelay,
		retryMaxDelay:  retryMaxDelay,
		retryJitter:    retryJitter,
		userAgent:      userAgent,
		maxRefHops:     maxRefHops,
		sleep:          sleep,
		randomFloat64:  randomFloat64,
	}, nil
}

// Fetch gets a JSON resource and returns request/canonical metadata.
func (c *Client) Fetch(ctx context.Context, ref string) (*Document, error) {
	requestURL, err := c.resolveRef(ref)
	if err != nil {
		return nil, err
	}

	attempts := c.maxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx := ctx
		cancel := func() {}
		if c.timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, c.timeout)
		}

		req, reqErr := http.NewRequestWithContext(attemptCtx, http.MethodGet, requestURL, nil)
		if reqErr != nil {
			cancel()
			return nil, fmt.Errorf("create request %q: %w", requestURL, reqErr)
		}
		req.Header.Set("User-Agent", c.userAgent)

		resp, doErr := c.httpClient.Do(req)
		cancel()
		if doErr != nil {
			if attempt < c.maxRetries && c.shouldRetryError(doErr, ctx) {
				if sleepErr := c.sleep(ctx, c.retryDelay(attempt)); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			return nil, fmt.Errorf("request %q failed: %w", requestURL, doErr)
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response %q: %w", requestURL, readErr)
		}

		if resp.StatusCode >= defaultRetryStatusCode && attempt < c.maxRetries {
			if sleepErr := c.sleep(ctx, c.retryDelay(attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &HTTPStatusError{
				URL:        requestURL,
				StatusCode: resp.StatusCode,
				Body:       sanitizeBodyPreview(body),
			}
		}

		canonicalRef, canonicalErr := extractCanonicalRef(body)
		if canonicalErr != nil {
			return nil, fmt.Errorf("decode canonical ref for %q: %w", requestURL, canonicalErr)
		}

		if canonicalRef != "" {
			canonicalRef, canonicalErr = resolveURL(requestURL, canonicalRef)
			if canonicalErr != nil {
				return nil, fmt.Errorf("resolve canonical ref %q from %q: %w", canonicalRef, requestURL, canonicalErr)
			}
		} else {
			canonicalRef = requestURL
		}

		return &Document{
			RequestedRef: requestURL,
			CanonicalRef: canonicalRef,
			StatusCode:   resp.StatusCode,
			Body:         body,
		}, nil
	}

	return nil, fmt.Errorf("request %q failed after retries", requestURL)
}

// GetJSON fetches and decodes a JSON payload.
func (c *Client) GetJSON(ctx context.Context, ref string, target any) (*Document, error) {
	doc, err := c.Fetch(ctx, ref)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(doc.Body, target); err != nil {
		return nil, fmt.Errorf("decode %q: %w", doc.CanonicalRef, err)
	}
	return doc, nil
}

// FollowRef fetches a Ref target when present.
func (c *Client) FollowRef(ctx context.Context, ref *Ref) (*Document, error) {
	if ref == nil || strings.TrimSpace(ref.URL) == "" {
		return nil, ErrMissingRef
	}
	return c.Fetch(ctx, ref.URL)
}

// ResolveRefChain follows pointer-only resources until the payload has real fields.
func (c *Client) ResolveRefChain(ctx context.Context, ref string) (*ResolvedDocument, error) {
	requestedRef, err := c.resolveRef(ref)
	if err != nil {
		return nil, err
	}

	currentRef := requestedRef
	traversed := make([]string, 0, c.maxRefHops+1)
	seen := map[string]struct{}{}

	for hop := 0; hop <= c.maxRefHops; hop++ {
		doc, fetchErr := c.Fetch(ctx, currentRef)
		if fetchErr != nil {
			return nil, fetchErr
		}

		traversed = append(traversed, currentRef)
		seen[currentRef] = struct{}{}

		nextRef, isPointer, ptrErr := extractPointerRef(doc.Body)
		if ptrErr != nil {
			return nil, fmt.Errorf("decode pointer ref for %q: %w", currentRef, ptrErr)
		}

		if !isPointer {
			return &ResolvedDocument{
				RequestedRef: requestedRef,
				CanonicalRef: doc.CanonicalRef,
				TraversedRef: traversed,
				StatusCode:   doc.StatusCode,
				Body:         doc.Body,
			}, nil
		}

		nextRef, err = resolveURL(currentRef, nextRef)
		if err != nil {
			return nil, fmt.Errorf("resolve pointer ref %q from %q: %w", nextRef, currentRef, err)
		}

		if _, ok := seen[nextRef]; ok {
			return nil, fmt.Errorf("%w: %q", ErrPointerLoop, nextRef)
		}

		currentRef = nextRef
	}

	return nil, fmt.Errorf("%w (start: %s)", ErrPointerChainLimit, requestedRef)
}

func (c *Client) resolveRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "/") {
		apiRoot := strings.TrimRight(c.baseURL.Path, "/")
		if apiRoot != "" && apiRoot != "/" &&
			!strings.HasPrefix(ref, apiRoot+"/") &&
			ref != apiRoot &&
			!strings.HasPrefix(ref, "/v2/") {
			ref = strings.TrimPrefix(ref, "/")
		}
	}

	resolved, err := resolveURL(c.baseURL.String(), ref)
	if err != nil {
		return "", fmt.Errorf("resolve ref %q: %w", ref, err)
	}
	return resolved, nil
}

func (c *Client) shouldRetryError(err error, parentCtx context.Context) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return parentCtx.Err() == nil
	}

	return parentCtx.Err() == nil
}

func (c *Client) retryDelay(attempt int) time.Duration {
	delay := float64(c.retryBaseDelay)
	delay *= math.Pow(2, float64(attempt))
	if delay > float64(c.retryMaxDelay) {
		delay = float64(c.retryMaxDelay)
	}

	if c.retryJitter > 0 {
		multiplier := 1 + ((c.randomFloat64()*2)-1)*c.retryJitter
		if multiplier < 0 {
			multiplier = 0
		}
		delay *= multiplier
	}

	if delay > float64(c.retryMaxDelay) {
		delay = float64(c.retryMaxDelay)
	}

	return time.Duration(delay)
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func sanitizeBodyPreview(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	if len(trimmed) > 240 {
		return trimmed[:240] + "..."
	}
	return trimmed
}

func resolveURL(baseValue, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty ref")
	}

	relative, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	if relative.IsAbs() {
		return relative.String(), nil
	}

	if baseValue == "" {
		return "", fmt.Errorf("base url is required for relative ref %q", ref)
	}

	baseURL, err := url.Parse(baseValue)
	if err != nil {
		return "", err
	}
	if !baseURL.IsAbs() {
		return "", fmt.Errorf("base url must be absolute: %q", baseValue)
	}

	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/"
	}

	return baseURL.ResolveReference(relative).String(), nil
}

func extractCanonicalRef(data []byte) (string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return "", nil
	}

	raw, ok := top["$ref"]
	if !ok || string(raw) == "null" {
		return "", nil
	}

	var ref string
	if err := json.Unmarshal(raw, &ref); err != nil {
		return "", err
	}

	return strings.TrimSpace(ref), nil
}

func extractPointerRef(data []byte) (string, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return "", false, nil
	}

	raw, ok := top["$ref"]
	if !ok {
		return "", false, nil
	}

	var ref string
	if err := json.Unmarshal(raw, &ref); err != nil {
		return "", false, err
	}

	if len(top) == 1 {
		return strings.TrimSpace(ref), true, nil
	}

	return "", false, nil
}
