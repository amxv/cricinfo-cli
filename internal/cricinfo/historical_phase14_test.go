package cricinfo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPhase14ScopedTraversalBySeasonGroupAndDateRange(t *testing.T) {
	t.Parallel()

	harness := newPhase14HistoricalHarness(t, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	seasonSession, err := harness.service.BeginScope(ctx, HistoricalScopeOptions{
		LeagueQuery: "Mirwais Nika",
		SeasonQuery: "2025",
	})
	if err != nil {
		t.Fatalf("BeginScope season error: %v", err)
	}

	seasonMatches := seasonSession.ScopedMatches()
	if len(seasonMatches) != 2 {
		t.Fatalf("expected 2 season-scoped matches, got %d", len(seasonMatches))
	}

	groupSession, err := harness.service.BeginScope(ctx, HistoricalScopeOptions{
		LeagueQuery: "Mirwais Nika",
		SeasonQuery: "2025",
		TypeQuery:   "1",
		GroupQuery:  "1",
	})
	if err != nil {
		t.Fatalf("BeginScope group error: %v", err)
	}

	groupMatches := groupSession.ScopedMatches()
	if len(groupMatches) != 1 {
		t.Fatalf("expected 1 group-scoped match, got %d", len(groupMatches))
	}
	if groupMatches[0].ID != "1529474" {
		t.Fatalf("expected group-scoped match 1529474, got %+v", groupMatches[0])
	}

	dateSession, err := harness.service.BeginScope(ctx, HistoricalScopeOptions{
		LeagueQuery: "Mirwais Nika",
		DateFrom:    "2025-05-01",
		DateTo:      "2025-05-30",
	})
	if err != nil {
		t.Fatalf("BeginScope date-range error: %v", err)
	}

	dateMatches := dateSession.ScopedMatches()
	if len(dateMatches) != 1 {
		t.Fatalf("expected 1 date-range match, got %d", len(dateMatches))
	}
	if dateMatches[0].ID != "1529475" {
		t.Fatalf("expected date-range match 1529475, got %+v", dateMatches[0])
	}
}

func TestPhase14RunScopedHydrationReuseAvoidsDuplicateFetches(t *testing.T) {
	t.Parallel()

	harness := newPhase14HistoricalHarness(t, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := harness.service.BeginScope(ctx, HistoricalScopeOptions{
		LeagueQuery: "Mirwais Nika",
		SeasonQuery: "2025",
		MatchLimit:  1,
	})
	if err != nil {
		t.Fatalf("BeginScope error: %v", err)
	}

	matches := session.ScopedMatches()
	if len(matches) == 0 {
		t.Fatalf("expected at least one scoped match")
	}
	matchID := matches[0].ID

	if _, _, err := session.HydrateMatchSummaries(ctx); err != nil {
		t.Fatalf("HydrateMatchSummaries error: %v", err)
	}
	if _, _, err := session.HydrateInnings(ctx, matchID); err != nil {
		t.Fatalf("HydrateInnings error: %v", err)
	}
	if _, _, err := session.HydratePlayerMatchSummaries(ctx, matchID); err != nil {
		t.Fatalf("HydratePlayerMatchSummaries error: %v", err)
	}
	if _, _, err := session.HydrateDeliverySummaries(ctx, matchID); err != nil {
		t.Fatalf("HydrateDeliverySummaries error: %v", err)
	}

	metricsAfterFirst := session.Metrics()
	countsAfterFirst := harness.requestCounts()

	if _, _, err := session.HydrateMatchSummaries(ctx); err != nil {
		t.Fatalf("HydrateMatchSummaries second pass error: %v", err)
	}
	if _, _, err := session.HydrateInnings(ctx, matchID); err != nil {
		t.Fatalf("HydrateInnings second pass error: %v", err)
	}
	if _, _, err := session.HydratePlayerMatchSummaries(ctx, matchID); err != nil {
		t.Fatalf("HydratePlayerMatchSummaries second pass error: %v", err)
	}
	if _, _, err := session.HydrateDeliverySummaries(ctx, matchID); err != nil {
		t.Fatalf("HydrateDeliverySummaries second pass error: %v", err)
	}

	metricsAfterSecond := session.Metrics()
	countsAfterSecond := harness.requestCounts()

	if metricsAfterSecond.ResolveCacheMisses != metricsAfterFirst.ResolveCacheMisses {
		t.Fatalf("expected no new resolve cache misses in second pass, first=%+v second=%+v", metricsAfterFirst, metricsAfterSecond)
	}
	if metricsAfterSecond.DomainCacheHits <= metricsAfterFirst.DomainCacheHits {
		t.Fatalf("expected domain cache hits to increase in second pass, first=%+v second=%+v", metricsAfterFirst, metricsAfterSecond)
	}
	if !reflect.DeepEqual(countsAfterFirst, countsAfterSecond) {
		t.Fatalf("expected no new HTTP requests in second pass\nfirst=%v\nsecond=%v", countsAfterFirst, countsAfterSecond)
	}
}

func TestPhase14PerformanceLimitedMultiMatchSample(t *testing.T) {
	t.Parallel()

	harness := newPhase14HistoricalHarness(t, 8*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := harness.service.BeginScope(ctx, HistoricalScopeOptions{
		LeagueQuery: "Mirwais Nika",
		SeasonQuery: "2025",
		MatchLimit:  2,
	})
	if err != nil {
		t.Fatalf("BeginScope error: %v", err)
	}

	matches := session.ScopedMatches()
	if len(matches) < 2 {
		t.Fatalf("expected at least two matches for limited performance sample")
	}

	firstStart := time.Now()
	if _, _, err := session.HydrateMatchSummaries(ctx); err != nil {
		t.Fatalf("HydrateMatchSummaries error: %v", err)
	}
	for _, match := range matches {
		if _, _, err := session.HydrateInnings(ctx, match.ID); err != nil {
			t.Fatalf("HydrateInnings(%s) error: %v", match.ID, err)
		}
		if _, _, err := session.HydratePlayerMatchSummaries(ctx, match.ID); err != nil {
			t.Fatalf("HydratePlayerMatchSummaries(%s) error: %v", match.ID, err)
		}
		if _, _, err := session.HydrateDeliverySummaries(ctx, match.ID); err != nil {
			t.Fatalf("HydrateDeliverySummaries(%s) error: %v", match.ID, err)
		}
	}
	firstDuration := time.Since(firstStart)
	firstMetrics := session.Metrics()

	secondStart := time.Now()
	if _, _, err := session.HydrateMatchSummaries(ctx); err != nil {
		t.Fatalf("HydrateMatchSummaries second pass error: %v", err)
	}
	for _, match := range matches {
		if _, _, err := session.HydrateInnings(ctx, match.ID); err != nil {
			t.Fatalf("HydrateInnings second pass (%s) error: %v", match.ID, err)
		}
		if _, _, err := session.HydratePlayerMatchSummaries(ctx, match.ID); err != nil {
			t.Fatalf("HydratePlayerMatchSummaries second pass (%s) error: %v", match.ID, err)
		}
		if _, _, err := session.HydrateDeliverySummaries(ctx, match.ID); err != nil {
			t.Fatalf("HydrateDeliverySummaries second pass (%s) error: %v", match.ID, err)
		}
	}
	secondDuration := time.Since(secondStart)
	secondMetrics := session.Metrics()

	if firstDuration > 5*time.Second {
		t.Fatalf("expected limited multi-match sample to finish quickly, duration=%s", firstDuration)
	}
	if secondDuration >= firstDuration {
		t.Fatalf("expected reused in-process pass to be faster, first=%s second=%s", firstDuration, secondDuration)
	}
	if secondMetrics.ResolveCacheMisses != firstMetrics.ResolveCacheMisses {
		t.Fatalf("expected no additional resolve misses in second pass, first=%+v second=%+v", firstMetrics, secondMetrics)
	}
}

func TestLivePhase14ScopedHydrationReuse(t *testing.T) {
	t.Parallel()
	requireLiveMatrix(t)

	service, err := NewHistoricalHydrationService(HistoricalHydrationServiceConfig{})
	if err != nil {
		t.Fatalf("NewHistoricalHydrationService error: %v", err)
	}
	defer func() {
		_ = service.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	session, err := service.BeginScope(ctx, HistoricalScopeOptions{
		LeagueQuery: "19138",
		MatchLimit:  2,
	})
	if err != nil {
		if isLive503(err) {
			t.Skipf("skipping scoped traversal after transient 503: %v", err)
		}
		t.Fatalf("BeginScope live error: %v", err)
	}

	matches := session.ScopedMatches()
	if len(matches) == 0 {
		t.Skip("skipping live scoped hydration: no matches found for selected scope")
	}
	matchID := matches[0].ID

	if _, warnings, err := session.HydrateMatchSummaries(ctx); err != nil {
		if isLive503(err) {
			t.Skipf("skipping live match summary hydration after transient 503: %v", err)
		}
		t.Fatalf("HydrateMatchSummaries live error: %v", err)
	} else if hasLive503Warning(warnings) {
		t.Skipf("skipping live run after transient 503 warnings: %v", warnings)
	}

	if _, warnings, err := session.HydrateInnings(ctx, matchID); err != nil {
		if isLive503(err) {
			t.Skipf("skipping live innings hydration after transient 503: %v", err)
		}
		t.Fatalf("HydrateInnings live error: %v", err)
	} else if hasLive503Warning(warnings) {
		t.Skipf("skipping live run after transient 503 innings warnings: %v", warnings)
	}

	if _, warnings, err := session.HydratePlayerMatchSummaries(ctx, matchID); err != nil {
		if isLive503(err) {
			t.Skipf("skipping live player hydration after transient 503: %v", err)
		}
		t.Fatalf("HydratePlayerMatchSummaries live error: %v", err)
	} else if hasLive503Warning(warnings) {
		t.Skipf("skipping live run after transient 503 player warnings: %v", warnings)
	}

	if _, warnings, err := session.HydrateDeliverySummaries(ctx, matchID); err != nil {
		if isLive503(err) {
			t.Skipf("skipping live delivery hydration after transient 503: %v", err)
		}
		t.Fatalf("HydrateDeliverySummaries live error: %v", err)
	} else if hasLive503Warning(warnings) {
		t.Skipf("skipping live run after transient 503 delivery warnings: %v", warnings)
	}

	firstMetrics := session.Metrics()

	if _, _, err := session.HydrateMatchSummaries(ctx); err != nil {
		t.Fatalf("HydrateMatchSummaries live second pass error: %v", err)
	}
	if _, _, err := session.HydrateInnings(ctx, matchID); err != nil {
		t.Fatalf("HydrateInnings live second pass error: %v", err)
	}
	if _, _, err := session.HydratePlayerMatchSummaries(ctx, matchID); err != nil {
		t.Fatalf("HydratePlayerMatchSummaries live second pass error: %v", err)
	}
	if _, _, err := session.HydrateDeliverySummaries(ctx, matchID); err != nil {
		t.Fatalf("HydrateDeliverySummaries live second pass error: %v", err)
	}

	secondMetrics := session.Metrics()
	if secondMetrics.ResolveCacheMisses != firstMetrics.ResolveCacheMisses {
		t.Fatalf("expected no new resolve misses in second live pass, first=%+v second=%+v", firstMetrics, secondMetrics)
	}
	if secondMetrics.DomainCacheHits <= firstMetrics.DomainCacheHits {
		t.Fatalf("expected domain cache hits to increase in second live pass, first=%+v second=%+v", firstMetrics, secondMetrics)
	}
}

type phase14HistoricalHarness struct {
	service *HistoricalHydrationService

	mu     sync.Mutex
	counts map[string]int
}

func (h *phase14HistoricalHarness) requestCounts() map[string]int {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make(map[string]int, len(h.counts))
	for path, count := range h.counts {
		out[path] = count
	}
	return out
}

func newPhase14HistoricalHarness(t *testing.T, latency time.Duration) *phase14HistoricalHarness {
	t.Helper()

	h := &phase14HistoricalHarness{counts: map[string]int{}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.counts[r.URL.Path]++
		h.mu.Unlock()

		if latency > 0 {
			time.Sleep(latency)
		}

		base := "http://" + r.Host + "/v2/sports/cricket"
		path := r.URL.Path

		switch path {
		case "/v2/sports/cricket/events":
			_, _ = w.Write([]byte(`{"count":0,"pageIndex":1,"pageSize":25,"pageCount":0,"items":[]}`))
		case "/v2/sports/cricket/leagues":
			_, _ = w.Write([]byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + base + `/leagues/19138"}]}`))
		case "/v2/sports/cricket/leagues/19138":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138","id":"19138","name":"Mirwais Nika Provincial 3-Day","slug":"19138","events":{"$ref":"` + base + `/leagues/19138/events"},"seasons":{"$ref":"` + base + `/leagues/19138/seasons"}}`))
		case "/v2/sports/cricket/leagues/19138/events":
			_, _ = w.Write([]byte(`{"count":3,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + base + `/leagues/19138/events/1529474"},{"$ref":"` + base + `/leagues/19138/events/1529475"},{"$ref":"` + base + `/leagues/19138/events/1529476"}]}`))
		case "/v2/sports/cricket/leagues/19138/seasons":
			_, _ = w.Write([]byte(`{"count":2,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + base + `/leagues/19138/seasons/2025"},{"$ref":"` + base + `/leagues/19138/seasons/2024"}]}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2025":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2025","id":"2025","year":2025,"types":{"$ref":"` + base + `/leagues/19138/seasons/2025/types"}}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2024":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2024","id":"2024","year":2024,"types":{"$ref":"` + base + `/leagues/19138/seasons/2024/types"}}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2025/types":
			_, _ = w.Write([]byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + base + `/leagues/19138/seasons/2025/types/1"}]}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2024/types":
			_, _ = w.Write([]byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + base + `/leagues/19138/seasons/2024/types/1"}]}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2025/types/1":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2025/types/1","id":"1","name":"Regular Season","abbreviation":"RS","startDate":"2025-01-01","endDate":"2025-12-31","groups":{"$ref":"` + base + `/leagues/19138/seasons/2025/types/1/groups"}}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2024/types/1":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2024/types/1","id":"1","name":"Regular Season","abbreviation":"RS","startDate":"2024-01-01","endDate":"2024-12-31","groups":{"$ref":"` + base + `/leagues/19138/seasons/2024/types/1/groups"}}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2025/types/1/groups":
			_, _ = w.Write([]byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + base + `/leagues/19138/seasons/2025/types/1/groups/1"}]}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2024/types/1/groups":
			_, _ = w.Write([]byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + base + `/leagues/19138/seasons/2024/types/1/groups/1"}]}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2025/types/1/groups/1":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2025/types/1/groups/1","id":"1","name":"Group A","abbreviation":"A","standings":{"$ref":"` + base + `/leagues/19138/seasons/2025/types/1/groups/1/standings"}}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2024/types/1/groups/1":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2024/types/1/groups/1","id":"1","name":"Group A","abbreviation":"A","standings":{"$ref":"` + base + `/leagues/19138/seasons/2024/types/1/groups/1/standings"}}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2025/types/1/groups/1/standings":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2025/types/1/groups/1/standings","items":[{"$ref":"` + base + `/leagues/19138/seasons/2025/types/1/groups/1/standings/1"}]}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2024/types/1/groups/1/standings":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2024/types/1/groups/1/standings","items":[{"$ref":"` + base + `/leagues/19138/seasons/2024/types/1/groups/1/standings/1"}]}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2025/types/1/groups/1/standings/1":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2025/types/1/groups/1/standings/1","id":"1","standings":[{"team":{"$ref":"` + base + `/teams/789643","id":"789643","displayName":"Group Team 1","shortDisplayName":"GT1"}},{"team":{"$ref":"` + base + `/teams/789644","id":"789644","displayName":"Group Team 2","shortDisplayName":"GT2"}}]}`))
		case "/v2/sports/cricket/leagues/19138/seasons/2024/types/1/groups/1/standings/1":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138/seasons/2024/types/1/groups/1/standings/1","id":"1","standings":[]}`))
		case "/v2/sports/cricket/leagues/19138/events/1529474":
			_, _ = w.Write([]byte(phase14EventPayload(base, "1529474", "2025-04-10T10:00:00Z", "789643", "789644", "Season Match A")))
		case "/v2/sports/cricket/leagues/19138/events/1529475":
			_, _ = w.Write([]byte(phase14EventPayload(base, "1529475", "2025-05-12T10:00:00Z", "789650", "789651", "Season Match B")))
		case "/v2/sports/cricket/leagues/19138/events/1529476":
			_, _ = w.Write([]byte(phase14EventPayload(base, "1529476", "2024-05-12T10:00:00Z", "789660", "789661", "Older Match")))
		default:
			if strings.HasPrefix(path, "/v2/sports/cricket/leagues/19138/events/") {
				if handled := writePhase14MatchSubresource(w, base, path); handled {
					return
				}
			}
			if strings.HasPrefix(path, "/v2/sports/cricket/teams/") {
				teamID := strings.TrimPrefix(path, "/v2/sports/cricket/teams/")
				_, _ = w.Write([]byte(`{"$ref":"` + base + `/teams/` + teamID + `","id":"` + teamID + `","displayName":"Team ` + teamID + `","shortDisplayName":"T` + teamID + `","abbreviation":"T` + teamID + `"}`))
				return
			}
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	index, err := OpenEntityIndex(filepath.Join(t.TempDir(), "resolver-index.json"))
	if err != nil {
		t.Fatalf("OpenEntityIndex error: %v", err)
	}
	if err := index.Upsert(IndexedEntity{
		Kind:      EntityLeague,
		ID:        "19138",
		Ref:       "/leagues/19138",
		Name:      "Mirwais Nika Provincial 3-Day",
		ShortName: "19138",
		Aliases:   []string{"Mirwais Nika", "19138"},
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("index upsert error: %v", err)
	}
	index.SetLastEventsSeedAt(time.Now().UTC())

	client, err := NewClient(Config{BaseURL: server.URL + "/v2/sports/cricket"})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	resolver, err := NewResolver(ResolverConfig{
		Client:       client,
		Index:        index,
		EventSeedTTL: 24 * time.Hour,
		Now:          func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("NewResolver error: %v", err)
	}
	t.Cleanup(func() {
		_ = resolver.Close()
	})

	service, err := NewHistoricalHydrationService(HistoricalHydrationServiceConfig{
		Client:   client,
		Resolver: resolver,
	})
	if err != nil {
		t.Fatalf("NewHistoricalHydrationService error: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	h.service = service
	return h
}

func phase14EventPayload(base, id, date, teamA, teamB, label string) string {
	competitionRef := base + "/leagues/19138/events/" + id + "/competitions/" + id
	teamABase := competitionRef + "/competitors/" + teamA
	teamBBase := competitionRef + "/competitors/" + teamB

	return fmt.Sprintf(`{"$ref":"%s/leagues/19138/events/%s","id":"%s","description":"%s","shortDescription":"%s","date":"%s","competitions":[{"$ref":"%s","id":"%s","description":"%s","shortDescription":"%s","date":"%s","status":{"$ref":"%s/status"},"details":{"$ref":"%s/details"},"competitors":[{"$ref":"%s","id":"%s","team":{"$ref":"%s/teams/%s","id":"%s","displayName":"Team %s","shortDisplayName":"T%s"},"score":{"$ref":"%s/scores"},"roster":{"$ref":"%s/roster"},"linescores":{"$ref":"%s/linescores"}},{"$ref":"%s","id":"%s","team":{"$ref":"%s/teams/%s","id":"%s","displayName":"Team %s","shortDisplayName":"T%s"},"score":{"$ref":"%s/scores"},"roster":{"$ref":"%s/roster"},"linescores":{"$ref":"%s/linescores"}}]}]}`,
		base, id, id, label, label, date,
		competitionRef, id, label, label, date,
		competitionRef, competitionRef,
		teamABase, teamA, base, teamA, teamA, teamA, teamA, teamABase, teamABase, teamABase,
		teamBBase, teamB, base, teamB, teamB, teamB, teamB, teamBBase, teamBBase, teamBBase,
	)
}

func writePhase14MatchSubresource(w http.ResponseWriter, base, path string) bool {
	trimmed := strings.TrimPrefix(path, "/v2/sports/cricket/")
	segments := strings.Split(trimmed, "/")
	if len(segments) < 7 {
		return false
	}

	leagueID := segments[1]
	eventID := segments[3]
	competitionID := segments[5]
	if leagueID != "19138" || eventID != competitionID {
		return false
	}

	baseCompetition := base + "/leagues/19138/events/" + eventID + "/competitions/" + competitionID
	subroute := strings.Join(segments[6:], "/")

	switch subroute {
	case "status":
		_, _ = w.Write([]byte(`{"$ref":"` + baseCompetition + `/status","summary":"In Progress","longSummary":"In Progress","type":{"state":"in","detail":"live","shortDetail":"live"}}`))
		return true
	case "details":
		_, _ = w.Write([]byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + baseCompetition + `/details/110"}]}`))
		return true
	case "details/110":
		_, _ = w.Write([]byte(`{"$ref":"` + baseCompetition + `/details/110","id":"110","period":1,"periodText":"1","over":{"number":1,"ball":1},"scoreValue":1,"shortText":"single","text":"single to midwicket","homeScore":"1/0","awayScore":"0/0","batsman":{"athlete":{"$ref":"` + base + `/athletes/1361257"}},"bowler":{"athlete":{"$ref":"` + base + `/athletes/1436502"}},"dismissal":{"dismissal":false}}`))
		return true
	}

	if len(segments) < 9 || segments[6] != "competitors" {
		return false
	}

	competitorID := segments[7]
	competitorBase := baseCompetition + "/competitors/" + competitorID
	playerID := phase14PlayerIDForTeam(competitorID)
	subroute = strings.Join(segments[8:], "/")

	switch subroute {
	case "scores":
		_, _ = w.Write([]byte(`{"$ref":"` + competitorBase + `/scores","displayValue":"120/3","value":"120/3"}`))
		return true
	case "roster":
		_, _ = w.Write([]byte(`{"$ref":"` + competitorBase + `/roster","entries":[{"playerId":"` + playerID + `","athlete":{"$ref":"` + base + `/athletes/` + playerID + `","displayName":"Player ` + playerID + `"},"statistics":{"$ref":"` + competitorBase + `/roster/` + playerID + `/statistics/0"},"linescores":{"$ref":"` + competitorBase + `/roster/` + playerID + `/linescores"}}]}`))
		return true
	case "roster/" + playerID + "/statistics/0":
		_, _ = w.Write([]byte(`{"$ref":"` + competitorBase + `/roster/` + playerID + `/statistics/0","athlete":{"$ref":"` + base + `/athletes/` + playerID + `"},"competition":{"$ref":"` + baseCompetition + `"},"splits":{"id":"0","name":"Total","abbreviation":"Total","categories":[{"name":"general","displayName":"General","stats":[{"name":"runs","displayValue":"33","value":33},{"name":"ballsFaced","displayValue":"60","value":60},{"name":"dots","displayValue":"18","value":18},{"name":"economyRate","displayValue":"4.50","value":4.5},{"name":"dismissalName","displayValue":"caught","value":"caught"},{"name":"dismissalCard","displayValue":"c","value":"c"}]}]}}`))
		return true
	case "roster/" + playerID + "/linescores":
		_, _ = w.Write([]byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + competitorBase + `/roster/` + playerID + `/linescores/1/1","period":1,"value":1,"isBatting":true,"statistics":{"$ref":"` + competitorBase + `/roster/` + playerID + `/linescores/1/1/statistics/0"}}]}`))
		return true
	case "linescores":
		_, _ = w.Write([]byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"` + competitorBase + `/linescores/1/1","period":1,"value":1,"runs":120,"wickets":3,"score":"120/3","statistics":{"$ref":"` + competitorBase + `/linescores/1/1/statistics/0"},"partnerships":{"$ref":"` + competitorBase + `/linescores/1/1/partnerships"},"fow":{"$ref":"` + competitorBase + `/linescores/1/1/fow"}}]}`))
		return true
	case "linescores/1/1/statistics/0":
		_, _ = w.Write([]byte(`{"$ref":"` + competitorBase + `/linescores/1/1/statistics/0","splits":{"overs":[{"number":1,"runs":6,"wicket":[{"number":1,"fow":"10/1","over":"1.4","fowType":"caught","runs":10,"ballsFaced":8,"strikeRate":125,"dismissalCard":"c","shortText":"c fielder","details":{"$ref":"` + baseCompetition + `/details/110","shortText":"single","text":"single to midwicket"}}]}]}}`))
		return true
	case "linescores/1/1/partnerships":
		_, _ = w.Write([]byte(`{"count":0,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[]}`))
		return true
	case "linescores/1/1/fow":
		_, _ = w.Write([]byte(`{"count":0,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[]}`))
		return true
	default:
		return false
	}
}

func phase14PlayerIDForTeam(teamID string) string {
	switch teamID {
	case "789643":
		return "1361257"
	case "789644":
		return "1361258"
	case "789650":
		return "1361260"
	case "789651":
		return "1361261"
	case "789660":
		return "1361262"
	case "789661":
		return "1361263"
	default:
		return "1361200"
	}
}
