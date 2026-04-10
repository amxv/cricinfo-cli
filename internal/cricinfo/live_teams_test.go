package cricinfo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLiveTeamCompetitorRoutes(t *testing.T) {
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
		keys []string
	}{
		{name: "roster", ref: "/leagues/19138/events/1529474/competitions/1529474/competitors/789643/roster", keys: []string{"entries", "team", "competition"}},
		{name: "leaders", ref: "/leagues/19138/events/1529474/competitions/1529474/competitors/789643/leaders", keys: []string{"categories", "name"}},
		{name: "statistics", ref: "/leagues/19138/events/1529474/competitions/1529474/competitors/789643/statistics", keys: []string{"splits", "team"}},
		{name: "records", ref: "/leagues/19138/events/1529474/competitions/1529474/competitors/789643/records", keys: []string{"items", "count"}},
		{name: "scores", ref: "/leagues/19138/events/1529474/competitions/1529474/competitors/789643/scores", keys: []string{"displayValue", "value"}},
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

			var payload map[string]any
			if err := json.Unmarshal(resolved.Body, &payload); err != nil {
				t.Fatalf("unmarshal %s payload: %v", tc.name, err)
			}

			requireAnyKey(t, payload, tc.keys...)
		})
	}
}

func TestLiveTeamServiceMatchScopeByIDAndAlias(t *testing.T) {
	t.Parallel()
	requireLiveMatrix(t)

	service, err := NewTeamService(TeamServiceConfig{})
	if err != nil {
		t.Fatalf("NewTeamService error: %v", err)
	}
	defer func() {
		_ = service.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rosterResult, err := service.Roster(ctx, "789643", TeamLookupOptions{LeagueID: "19138", MatchQuery: "1529474"})
	if err != nil {
		t.Fatalf("TeamService.Roster id error: %v", err)
	}
	if rosterResult.Status == ResultStatusError {
		if rosterResult.Error != nil && rosterResult.Error.StatusCode == 503 {
			t.Skipf("skipping roster service route after persistent 503: %s", rosterResult.Message)
		}
		t.Fatalf("unexpected roster error result: %+v", rosterResult)
	}
	if len(rosterResult.Items) == 0 && hasLive503Warning(rosterResult.Warnings) {
		t.Skipf("skipping roster service after 503 warnings: %v", rosterResult.Warnings)
	}

	leadersResult, err := service.Leaders(ctx, "Boost Region", TeamLookupOptions{LeagueID: "19138", MatchQuery: "1529474"})
	if err != nil {
		t.Fatalf("TeamService.Leaders alias error: %v", err)
	}
	if leadersResult.Status == ResultStatusError {
		if leadersResult.Error != nil && leadersResult.Error.StatusCode == 503 {
			t.Skipf("skipping leaders service route after persistent 503: %s", leadersResult.Message)
		}
		t.Fatalf("unexpected leaders error result: %+v", leadersResult)
	}
	if leadersResult.Kind != EntityTeamLeaders {
		t.Fatalf("expected leaders kind %q, got %q", EntityTeamLeaders, leadersResult.Kind)
	}
	if leadersResult.Status == ResultStatusEmpty && !strings.Contains(strings.Join(leadersResult.Warnings, " "), "503") {
		t.Fatalf("expected non-empty leaders result for alias lookup")
	}
}
