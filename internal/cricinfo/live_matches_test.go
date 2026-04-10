package cricinfo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLiveMatchesTraversalFromEvents(t *testing.T) {
	t.Parallel()
	requireLiveMatrix(t)

	service, err := NewMatchService(MatchServiceConfig{})
	if err != nil {
		t.Fatalf("NewMatchService error: %v", err)
	}
	defer func() {
		_ = service.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := service.List(ctx, MatchListOptions{Limit: 5})
	if err != nil {
		t.Fatalf("MatchService.List error: %v", err)
	}

	if result.Status == ResultStatusError {
		if result.Error != nil && result.Error.StatusCode == 503 {
			t.Skipf("skipping live traversal after persistent 503: %s", result.Message)
		}
		t.Fatalf("unexpected error result: %+v", result)
	}
	if !strings.Contains(result.RequestedRef, "/events") {
		t.Fatalf("expected list traversal to begin from /events, requestedRef=%q", result.RequestedRef)
	}

	if len(result.Items) == 0 {
		if hasLive503Warning(result.Warnings) {
			t.Skipf("skipping after transient 503 warnings: %v", result.Warnings)
		}
		t.Fatalf("expected at least one match from /events traversal")
	}

	first, ok := result.Items[0].(Match)
	if !ok {
		t.Fatalf("expected first result item to be Match, got %T", result.Items[0])
	}
	if first.LeagueID == "" || first.EventID == "" || first.CompetitionID == "" {
		t.Fatalf("expected match IDs for drill-down use, got %+v", first)
	}
	if len(first.Teams) == 0 {
		t.Fatalf("expected match teams in live traversal output")
	}
	if strings.TrimSpace(first.Date) == "" {
		t.Fatalf("expected match date in live traversal output")
	}
}

func TestLiveMatchDrillDownRoutes(t *testing.T) {
	t.Parallel()
	requireLiveMatrix(t)

	client, err := NewClient(Config{
		Timeout:    12 * time.Second,
		MaxRetries: 3,
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	routes := []struct {
		name string
		ref  string
	}{
		{name: "details", ref: "/leagues/19138/events/1529474/competitions/1529474/details"},
		{name: "plays", ref: "/leagues/19138/events/1529474/competitions/1529474/plays"},
		{name: "matchcards", ref: "/leagues/11132/events/1527966/competitions/1527966/matchcards"},
		{name: "situation", ref: "/leagues/19138/events/1529474/competitions/1529474/situation"},
	}

	for _, tc := range routes {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			resolved, err := client.ResolveRefChain(ctx, tc.ref)
			if err != nil {
				if isLive503(err) {
					t.Skipf("skipping %s after transient 503: %v", tc.name, err)
				}
				t.Fatalf("ResolveRefChain(%q) error: %v", tc.ref, err)
			}

			switch tc.name {
			case "details", "plays":
				page, err := DecodePage[Ref](resolved.Body)
				if err != nil {
					t.Fatalf("DecodePage %s error: %v", tc.name, err)
				}
				if len(page.Items) == 0 {
					t.Fatalf("expected at least one %s item ref", tc.name)
				}
			case "matchcards":
				var payload map[string]any
				if err := json.Unmarshal(resolved.Body, &payload); err != nil {
					t.Fatalf("unmarshal matchcards payload: %v", err)
				}
				items := mapSliceField(payload, "items")
				if len(items) == 0 {
					t.Fatalf("expected matchcards items")
				}
				headlines := map[string]bool{}
				for _, item := range items {
					headline := strings.ToLower(strings.TrimSpace(stringField(item, "headline")))
					if headline != "" {
						headlines[headline] = true
					}
				}
				if !headlines["batting"] || !headlines["bowling"] {
					t.Fatalf("expected matchcards to include batting and bowling sections, got %v", headlines)
				}
			case "situation":
				var payload map[string]any
				if err := json.Unmarshal(resolved.Body, &payload); err != nil {
					t.Fatalf("unmarshal situation payload: %v", err)
				}
				if len(payload) == 0 {
					t.Fatalf("expected non-empty situation payload")
				}
				if _, ok := payload["$ref"]; !ok {
					t.Fatalf("expected situation payload to include $ref")
				}
			}
		})
	}
}
