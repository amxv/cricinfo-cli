package cricinfo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLiveLeagueSeasonStandingsRoutes(t *testing.T) {
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
		{name: "leagues", ref: "/leagues", keys: []string{"items", "count"}},
		{name: "league", ref: "/leagues/19138", keys: []string{"id", "name", "events", "seasons"}},
		{name: "calendar", ref: "/leagues/19138/calendar", keys: []string{"items", "count"}},
		{name: "calendar-ondays", ref: "/leagues/19138/calendar/ondays", keys: []string{"eventDate", "sections"}},
		{name: "seasons", ref: "/leagues/19138/seasons", keys: []string{"items", "count"}},
		{name: "season", ref: "/leagues/19138/seasons/2025", keys: []string{"year", "types"}},
		{name: "season-types", ref: "/leagues/19138/seasons/2025/types", keys: []string{"items", "count"}},
		{name: "season-type", ref: "/leagues/19138/seasons/2025/types/1", keys: []string{"id", "groups", "hasGroups"}},
		{name: "season-groups", ref: "/leagues/19138/seasons/2025/types/1/groups", keys: []string{"items", "count"}},
		{name: "standings", ref: "/leagues/19138/standings", keys: []string{"$ref", "items", "standings", "entries"}},
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

func TestLiveLeagueServiceSeasonAndStandingsTraversal(t *testing.T) {
	t.Parallel()
	requireLiveMatrix(t)

	service, err := NewLeagueService(LeagueServiceConfig{})
	if err != nil {
		t.Fatalf("NewLeagueService error: %v", err)
	}
	defer func() {
		_ = service.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	listResult, err := service.List(ctx, LeagueListOptions{Limit: 5})
	if err != nil {
		t.Fatalf("LeagueService.List error: %v", err)
	}
	if listResult.Status == ResultStatusError {
		if listResult.Error != nil && listResult.Error.StatusCode == 503 {
			t.Skipf("skipping league list after persistent 503: %s", listResult.Message)
		}
		t.Fatalf("unexpected list error result: %+v", listResult)
	}
	if len(listResult.Items) == 0 {
		if hasLive503Warning(listResult.Warnings) {
			t.Skipf("skipping league list after transient 503 warnings: %v", listResult.Warnings)
		}
		t.Fatalf("expected list items from live /leagues traversal")
	}

	showResult, err := service.Show(ctx, "19138")
	if err != nil {
		t.Fatalf("LeagueService.Show error: %v", err)
	}
	if showResult.Status == ResultStatusError {
		if showResult.Error != nil && showResult.Error.StatusCode == 503 {
			t.Skipf("skipping league show after persistent 503: %s", showResult.Message)
		}
		t.Fatalf("unexpected show error result: %+v", showResult)
	}

	calendarResult, err := service.Calendar(ctx, "19138")
	if err != nil {
		t.Fatalf("LeagueService.Calendar error: %v", err)
	}
	if calendarResult.Status == ResultStatusError {
		if calendarResult.Error != nil && calendarResult.Error.StatusCode == 503 {
			t.Skipf("skipping calendar traversal after persistent 503: %s", calendarResult.Message)
		}
		t.Fatalf("unexpected calendar error result: %+v", calendarResult)
	}
	if calendarResult.Status == ResultStatusEmpty && !strings.Contains(strings.ToLower(strings.Join(calendarResult.Warnings, " ")), "503") {
		t.Fatalf("expected calendar traversal to return day entries")
	}

	seasonsResult, err := service.Seasons(ctx, "19138")
	if err != nil {
		t.Fatalf("LeagueService.Seasons error: %v", err)
	}
	if seasonsResult.Status == ResultStatusError {
		if seasonsResult.Error != nil && seasonsResult.Error.StatusCode == 503 {
			t.Skipf("skipping seasons traversal after persistent 503: %s", seasonsResult.Message)
		}
		t.Fatalf("unexpected seasons error result: %+v", seasonsResult)
	}
	if len(seasonsResult.Items) == 0 && hasLive503Warning(seasonsResult.Warnings) {
		t.Skipf("skipping seasons traversal after transient 503 warnings: %v", seasonsResult.Warnings)
	}

	typesResult, err := service.SeasonTypes(ctx, "19138", SeasonLookupOptions{SeasonQuery: "2025"})
	if err != nil {
		t.Fatalf("LeagueService.SeasonTypes error: %v", err)
	}
	if typesResult.Status == ResultStatusError {
		if typesResult.Error != nil && typesResult.Error.StatusCode == 503 {
			t.Skipf("skipping season types traversal after persistent 503: %s", typesResult.Message)
		}
		t.Fatalf("unexpected season types error result: %+v", typesResult)
	}

	groupsResult, err := service.SeasonGroups(ctx, "19138", SeasonLookupOptions{SeasonQuery: "2025", TypeQuery: "1"})
	if err != nil {
		t.Fatalf("LeagueService.SeasonGroups error: %v", err)
	}
	if groupsResult.Status == ResultStatusError {
		if groupsResult.Error != nil && groupsResult.Error.StatusCode == 503 {
			t.Skipf("skipping season groups traversal after persistent 503: %s", groupsResult.Message)
		}
		t.Fatalf("unexpected season groups error result: %+v", groupsResult)
	}

	standingsResult, err := service.Standings(ctx, "19138")
	if err != nil {
		t.Fatalf("LeagueService.Standings error: %v", err)
	}
	if standingsResult.Status == ResultStatusError {
		if standingsResult.Error != nil && standingsResult.Error.StatusCode == 503 {
			t.Skipf("skipping standings traversal after persistent 503: %s", standingsResult.Message)
		}
		t.Fatalf("unexpected standings error result: %+v", standingsResult)
	}
	if len(standingsResult.Items) == 0 && hasLive503Warning(standingsResult.Warnings) {
		t.Skipf("skipping standings traversal after transient 503 warnings: %v", standingsResult.Warnings)
	}
}
