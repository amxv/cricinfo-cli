package cricinfo

import (
	"context"
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
