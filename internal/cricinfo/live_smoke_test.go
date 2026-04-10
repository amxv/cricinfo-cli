package cricinfo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const liveSmokeEnv = "CRICINFO_LIVE_SMOKE"

func TestLiveSmokeValidatedRoutes(t *testing.T) {
	t.Parallel()
	requireLiveSmoke(t)

	client, err := NewClient(Config{
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	tests := []struct {
		name     string
		ref      string
		validate func(t *testing.T, resolved *ResolvedDocument)
	}{
		{
			name: "events",
			ref:  "/events",
			validate: func(t *testing.T, resolved *ResolvedDocument) {
				t.Helper()
				page, err := DecodePage[Ref](resolved.Body)
				if err != nil {
					t.Fatalf("DecodePage error: %v", err)
				}
				if page.PageSize == 0 {
					t.Fatalf("expected non-zero page size")
				}
			},
		},
		{
			name: "athlete",
			ref:  "/athletes/1361257",
			validate: func(t *testing.T, resolved *ResolvedDocument) {
				t.Helper()
				var payload map[string]any
				if err := json.Unmarshal(resolved.Body, &payload); err != nil {
					t.Fatalf("unmarshal athlete: %v", err)
				}
				if payload["id"] == nil {
					t.Fatalf("expected athlete id in payload")
				}
			},
		},
		{
			name: "athlete statistics",
			ref:  "/athletes/1361257/statistics",
			validate: func(t *testing.T, resolved *ResolvedDocument) {
				t.Helper()
				stats, err := DecodeStatsObject(resolved.Body)
				if err != nil {
					t.Fatalf("DecodeStatsObject error: %v", err)
				}
				if len(stats.Splits) == 0 {
					t.Fatalf("expected non-empty stats splits")
				}
			},
		},
		{
			name: "competition status",
			ref:  "/leagues/19138/events/1529474/competitions/1529474/status",
			validate: func(t *testing.T, resolved *ResolvedDocument) {
				t.Helper()
				var payload map[string]any
				if err := json.Unmarshal(resolved.Body, &payload); err != nil {
					t.Fatalf("unmarshal status payload: %v", err)
				}
				if payload["summary"] == nil && payload["longSummary"] == nil {
					t.Fatalf("expected summary or longSummary in status payload")
				}
			},
		},
		{
			name: "competition detail item",
			ref:  "/leagues/19138/events/1529474/competitions/1529474/details/110",
			validate: func(t *testing.T, resolved *ResolvedDocument) {
				t.Helper()
				var payload map[string]any
				if err := json.Unmarshal(resolved.Body, &payload); err != nil {
					t.Fatalf("unmarshal detail payload: %v", err)
				}
				if payload["text"] == nil && payload["over"] == nil {
					t.Fatalf("expected detail payload fields in response")
				}
			},
		},
		{
			name: "league standings",
			ref:  "/leagues/19138/standings",
			validate: func(t *testing.T, resolved *ResolvedDocument) {
				t.Helper()
				if len(resolved.TraversedRef) == 0 {
					t.Fatalf("expected at least one traversed ref")
				}
				if len(strings.TrimSpace(string(resolved.Body))) == 0 {
					t.Fatalf("expected non-empty standings body")
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()

			resolved, err := client.ResolveRefChain(ctx, tc.ref)
			if err != nil {
				t.Fatalf("ResolveRefChain(%q) error: %v", tc.ref, err)
			}
			if resolved.RequestedRef == "" || resolved.CanonicalRef == "" {
				t.Fatalf("expected requested and canonical refs to be populated")
			}
			tc.validate(t, resolved)
		})
	}
}

func TestLiveSmokeTransient503Retry(t *testing.T) {
	t.Parallel()
	requireLiveSmoke(t)

	transport := &transient503Transport{
		base:       http.DefaultTransport,
		targetPath: "/v2/sports/cricket/events",
	}

	httpClient := &http.Client{Transport: transport}
	client, err := NewClient(Config{
		HTTPClient:  httpClient,
		Timeout:     10 * time.Second,
		MaxRetries:  2,
		RetryJitter: 0,
		RandomFloat64: func() float64 {
			return 0.5
		},
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	doc, err := client.Fetch(ctx, "/events")
	if err != nil {
		t.Fatalf("Fetch /events with transient 503 injection failed: %v", err)
	}

	if !transport.injected.Load() {
		t.Fatalf("expected synthetic transient 503 to be injected")
	}
	if transport.hits.Load() < 2 {
		t.Fatalf("expected retry attempt after transient 503, got %d hits", transport.hits.Load())
	}

	if _, err := DecodePage[Ref](doc.Body); err != nil {
		t.Fatalf("DecodePage for /events after retry failed: %v", err)
	}
}

func requireLiveSmoke(t *testing.T) {
	t.Helper()
	if os.Getenv(liveSmokeEnv) != "1" {
		t.Skip("set CRICINFO_LIVE_SMOKE=1 to run live smoke tests")
	}
}

type transient503Transport struct {
	base       http.RoundTripper
	targetPath string
	injected   atomic.Bool
	hits       atomic.Int32
}

func (t *transient503Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == t.targetPath {
		hit := t.hits.Add(1)
		if hit == 1 {
			t.injected.Store(true)
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Status:     "503 Service Unavailable",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"synthetic transient"}`)),
				Request:    req,
			}, nil
		}
	}

	return t.base.RoundTrip(req)
}
