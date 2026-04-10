package cricinfo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchRetriesServerErrorsAndPreservesCanonicalRef(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	slept := make([]time.Duration, 0, 2)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() == "" {
			t.Fatalf("expected user agent to be set")
		}

		current := attempts.Add(1)
		if current <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"transient"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"$ref":"` + server.URL + `/canonical/events","count":0,"items":[],"pageCount":0,"pageIndex":1,"pageSize":25}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:        server.URL,
		MaxRetries:     2,
		RetryBaseDelay: 10 * time.Millisecond,
		RetryMaxDelay:  25 * time.Millisecond,
		RetryJitter:    0.25,
		RandomFloat64:  func() float64 { return 0.5 },
		Sleep: func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	doc, err := client.Fetch(context.Background(), "/events")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}

	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
	if len(slept) != 2 {
		t.Fatalf("expected 2 backoff sleeps, got %d", len(slept))
	}
	if slept[0] != 10*time.Millisecond || slept[1] != 20*time.Millisecond {
		t.Fatalf("unexpected backoff durations: %v", slept)
	}
	if doc.RequestedRef != server.URL+"/events" {
		t.Fatalf("unexpected requested ref: %q", doc.RequestedRef)
	}
	if doc.CanonicalRef != server.URL+"/canonical/events" {
		t.Fatalf("unexpected canonical ref: %q", doc.CanonicalRef)
	}
}

func TestFetchTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:    server.URL,
		Timeout:    25 * time.Millisecond,
		MaxRetries: 0,
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	start := time.Now()
	_, err = client.Fetch(context.Background(), "/slow")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestFetchDoesNotRetryAfterContextCancel(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, MaxRetries: 3})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.Fetch(ctx, "/events")
	if err == nil {
		t.Fatal("expected canceled context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if got := hits.Load(); got > 1 {
		t.Fatalf("expected at most one hit after cancellation, got %d", got)
	}
}

func TestRetryDelayBackoffJitterAndCap(t *testing.T) {
	t.Parallel()

	client := &Client{
		retryBaseDelay: 100 * time.Millisecond,
		retryMaxDelay:  250 * time.Millisecond,
		retryJitter:    0.5,
		randomFloat64:  func() float64 { return 1.0 },
	}

	if got := client.retryDelay(0); got != 150*time.Millisecond {
		t.Fatalf("attempt 0 delay mismatch: %v", got)
	}
	if got := client.retryDelay(1); got != 250*time.Millisecond {
		t.Fatalf("attempt 1 delay should cap at max delay, got %v", got)
	}
	if got := client.retryDelay(8); got != 250*time.Millisecond {
		t.Fatalf("late-attempt delay should stay capped, got %v", got)
	}
}

func TestResolveRefTreatsLeadingSlashAsAPIRelative(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{BaseURL: DefaultBaseURL})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	got, err := client.resolveRef("/events")
	if err != nil {
		t.Fatalf("resolveRef error: %v", err)
	}
	if got != DefaultBaseURL+"/events" {
		t.Fatalf("unexpected API-relative URL: %q", got)
	}

	absoluteGot, absoluteErr := client.resolveRef("/v2/sports/cricket/events")
	if absoluteErr != nil {
		t.Fatalf("resolveRef absolute path error: %v", absoluteErr)
	}
	if absoluteGot != DefaultBaseURL+"/events" {
		t.Fatalf("unexpected absolute API URL: %q", absoluteGot)
	}
}

func TestResolveRefChainFollowsPointers(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			_, _ = w.Write([]byte(`{"$ref":"` + server.URL + `/middle"}`))
		case "/middle":
			_, _ = w.Write([]byte(`{"$ref":"/final"}`))
		case "/final":
			_, _ = w.Write([]byte(`{"$ref":"http://core.espnuk.org/v2/sports/cricket/leagues/1174248/events/1/competitions/1/status","summary":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, MaxRetries: 0})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	resolved, err := client.ResolveRefChain(context.Background(), "/start")
	if err != nil {
		t.Fatalf("ResolveRefChain error: %v", err)
	}

	expectedTraversed := []string{server.URL + "/start", server.URL + "/middle", server.URL + "/final"}
	if len(resolved.TraversedRef) != len(expectedTraversed) {
		t.Fatalf("expected %d traversed refs, got %d", len(expectedTraversed), len(resolved.TraversedRef))
	}
	for i := range expectedTraversed {
		if resolved.TraversedRef[i] != expectedTraversed[i] {
			t.Fatalf("unexpected traversed ref at %d: got %q want %q", i, resolved.TraversedRef[i], expectedTraversed[i])
		}
	}

	if resolved.RequestedRef != server.URL+"/start" {
		t.Fatalf("unexpected requested ref: %q", resolved.RequestedRef)
	}
	if resolved.CanonicalRef != "http://core.espnuk.org/v2/sports/cricket/leagues/1174248/events/1/competitions/1/status" {
		t.Fatalf("unexpected canonical ref: %q", resolved.CanonicalRef)
	}

	var payload map[string]any
	if err := json.Unmarshal(resolved.Body, &payload); err != nil {
		t.Fatalf("unmarshal resolved body: %v", err)
	}
	if payload["summary"] != "ok" {
		t.Fatalf("unexpected summary: %v", payload["summary"])
	}
}

func TestResolveRefChainDetectsLoop(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			_, _ = w.Write([]byte(`{"$ref":"/b"}`))
		case "/b":
			_, _ = w.Write([]byte(`{"$ref":"/a"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, MaxRetries: 0})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	_, err = client.ResolveRefChain(context.Background(), "/a")
	if err == nil {
		t.Fatal("expected loop error")
	}
	if !errors.Is(err, ErrPointerLoop) {
		t.Fatalf("expected pointer loop error, got %v", err)
	}
}

func TestFollowRefMissing(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	_, err = client.FollowRef(context.Background(), nil)
	if !errors.Is(err, ErrMissingRef) {
		t.Fatalf("expected missing ref error, got %v", err)
	}
}
