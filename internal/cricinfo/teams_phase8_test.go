package cricinfo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTeamServicePhase8GlobalAndMatchRosterScopes(t *testing.T) {
	t.Parallel()

	service := newPhase8TeamTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	showResult, err := service.Show(ctx, "Boost Region", TeamLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Show error: %v", err)
	}
	if showResult.Kind != EntityTeam {
		t.Fatalf("expected show kind %q, got %q", EntityTeam, showResult.Kind)
	}
	team, ok := showResult.Data.(Team)
	if !ok {
		t.Fatalf("expected show data Team, got %T", showResult.Data)
	}
	if team.ID != "789643" {
		t.Fatalf("expected team id 789643, got %q", team.ID)
	}

	globalRosterResult, err := service.Roster(ctx, "789643", TeamLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Roster global error: %v", err)
	}
	if globalRosterResult.Kind != EntityTeamRoster {
		t.Fatalf("expected global roster kind %q, got %q", EntityTeamRoster, globalRosterResult.Kind)
	}
	if globalRosterResult.Status == ResultStatusError {
		t.Fatalf("expected non-error global roster result, got %+v", globalRosterResult)
	}
	if !strings.Contains(globalRosterResult.RequestedRef, "/teams/789643/athletes") {
		t.Fatalf("expected global roster route to request /teams/{id}/athletes, requestedRef=%q", globalRosterResult.RequestedRef)
	}

	matchRosterResult, err := service.Roster(ctx, "Boost Region", TeamLookupOptions{LeagueID: "19138", MatchQuery: "3rd Match"})
	if err != nil {
		t.Fatalf("Roster match error: %v", err)
	}
	if matchRosterResult.Kind != EntityTeamRoster {
		t.Fatalf("expected match roster kind %q, got %q", EntityTeamRoster, matchRosterResult.Kind)
	}
	if matchRosterResult.Status == ResultStatusError {
		t.Fatalf("expected non-error match roster result, got %+v", matchRosterResult)
	}
	if len(matchRosterResult.Items) == 0 {
		t.Fatalf("expected roster entries in match-scoped roster result")
	}
	if !strings.Contains(matchRosterResult.RequestedRef, "/competitors/789643/roster") {
		t.Fatalf("expected match roster route to request competitor roster, requestedRef=%q", matchRosterResult.RequestedRef)
	}
}

func TestTeamServicePhase8LeadersStatisticsRecordsAndScores(t *testing.T) {
	t.Parallel()

	service := newPhase8TeamTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	scoresResult, err := service.Scores(ctx, "789643", TeamLookupOptions{LeagueID: "19138", MatchQuery: "1529474"})
	if err != nil {
		t.Fatalf("Scores error: %v", err)
	}
	if scoresResult.Kind != EntityTeamScore {
		t.Fatalf("expected scores kind %q, got %q", EntityTeamScore, scoresResult.Kind)
	}
	score, ok := scoresResult.Data.(*TeamScore)
	if !ok {
		t.Fatalf("expected scores data *TeamScore, got %T", scoresResult.Data)
	}
	if strings.TrimSpace(score.DisplayValue) == "" {
		t.Fatalf("expected non-empty display score")
	}

	leadersResult, err := service.Leaders(ctx, "Boost Region", TeamLookupOptions{LeagueID: "19138", MatchQuery: "3rd Match"})
	if err != nil {
		t.Fatalf("Leaders error: %v", err)
	}
	if leadersResult.Kind != EntityTeamLeaders {
		t.Fatalf("expected leaders kind %q, got %q", EntityTeamLeaders, leadersResult.Kind)
	}
	leaders, ok := leadersResult.Data.(*TeamLeaders)
	if !ok {
		t.Fatalf("expected leaders data *TeamLeaders, got %T (status=%s message=%q error=%+v)", leadersResult.Data, leadersResult.Status, leadersResult.Message, leadersResult.Error)
	}
	if len(leaders.Categories) == 0 {
		t.Fatalf("expected leader categories in result")
	}

	statisticsResult, err := service.Statistics(ctx, "789643", TeamLookupOptions{LeagueID: "19138", MatchQuery: "1529474"})
	if err != nil {
		t.Fatalf("Statistics error: %v", err)
	}
	if statisticsResult.Kind != EntityTeamStatistics {
		t.Fatalf("expected statistics kind %q, got %q", EntityTeamStatistics, statisticsResult.Kind)
	}
	if len(statisticsResult.Items) == 0 {
		t.Fatalf("expected statistics categories in result")
	}

	recordsResult, err := service.Records(ctx, "789643", TeamLookupOptions{LeagueID: "19138", MatchQuery: "1529474"})
	if err != nil {
		t.Fatalf("Records error: %v", err)
	}
	if recordsResult.Kind != EntityTeamRecords {
		t.Fatalf("expected records kind %q, got %q", EntityTeamRecords, recordsResult.Kind)
	}
	if len(recordsResult.Items) == 0 {
		t.Fatalf("expected record categories in result")
	}
}

func newPhase8TeamTestService(t *testing.T) *TeamService {
	t.Helper()

	teamFixture := mustReadFixtureBytes(t, "team-competitor/team-789643.json")
	athletesFixture := mustReadFixtureBytes(t, "team-competitor/team-789643-athletes.json")
	rosterFixture := mustReadFixtureBytes(t, "team-competitor/roster-1147772.json")
	scoresFixture := mustReadFixtureBytes(t, "team-competitor/scores-789643.json")
	leadersFixture := mustReadFixtureBytes(t, "team-competitor/leaders-789643.json")
	statisticsFixture := mustReadFixtureBytes(t, "team-competitor/statistics-789643.json")
	recordsFixture := mustReadFixtureBytes(t, "team-competitor/records-789643.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host + "/v2/sports/cricket"
		competitionPath := "/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474"

		switch r.URL.Path {
		case "/v2/sports/cricket/teams/789643":
			_, _ = w.Write(rewriteFixtureBaseURL(teamFixture, base))
		case "/v2/sports/cricket/teams/789643/athletes":
			_, _ = w.Write(rewriteFixtureBaseURL(athletesFixture, base))
		case competitionPath:
			competition := fmt.Sprintf(`{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474","id":"1529474","description":"3rd Match","shortDescription":"3rd Match","date":"2026-04-09T05:30Z","competitors":[{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/competitors/789643","id":"789643","team":{"$ref":"%s/events/1529474/teams/789643"},"score":{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/competitors/789643/scores"},"roster":{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/competitors/789643/roster"},"leaders":{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/competitors/789643/leaders"},"statistics":{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/competitors/789643/statistics"},"record":{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/competitors/789643/records"}}]}`,
				base, base, base, base, base, base, base, base)
			_, _ = w.Write([]byte(competition))
		case competitionPath + "/competitors/789643/roster":
			_, _ = w.Write(rewriteFixtureBaseURL(rosterFixture, base))
		case competitionPath + "/competitors/789643/scores":
			_, _ = w.Write(rewriteFixtureBaseURL(scoresFixture, base))
		case competitionPath + "/competitors/789643/leaders":
			_, _ = w.Write(rewriteFixtureBaseURL(leadersFixture, base))
		case competitionPath + "/competitors/789643/statistics":
			_, _ = w.Write(rewriteFixtureBaseURL(statisticsFixture, base))
		case competitionPath + "/competitors/789643/records":
			_, _ = w.Write(rewriteFixtureBaseURL(recordsFixture, base))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	index, err := OpenEntityIndex(filepath.Join(t.TempDir(), "resolver-index.json"))
	if err != nil {
		t.Fatalf("OpenEntityIndex error: %v", err)
	}
	if err := index.UpsertMany([]IndexedEntity{
		{
			Kind:      EntityTeam,
			ID:        "789643",
			Ref:       "/teams/789643",
			Name:      "Boost Region",
			ShortName: "BOOST",
			LeagueID:  "19138",
			EventID:   "1529474",
			MatchID:   "1529474",
			Aliases:   []string{"Boost Region", "BOOST", "789643"},
			UpdatedAt: time.Now().UTC(),
		},
		{
			Kind:      EntityMatch,
			ID:        "1529474",
			Ref:       "/leagues/19138/events/1529474/competitions/1529474",
			Name:      "3rd Match",
			ShortName: "3rd Match",
			LeagueID:  "19138",
			EventID:   "1529474",
			MatchID:   "1529474",
			Aliases:   []string{"3rd Match", "1529474"},
			UpdatedAt: time.Now().UTC(),
		},
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

	service, err := NewTeamService(TeamServiceConfig{Client: client, Resolver: resolver})
	if err != nil {
		t.Fatalf("NewTeamService error: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return service
}
