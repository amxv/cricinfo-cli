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

func TestPhase11FixtureNormalizationForPlayerBattingAndBowlingExtraction(t *testing.T) {
	t.Parallel()

	matchStatsBody := mustReadFixtureBytes(t, "players/roster-1361257-statistics-0.json")
	matchCategories, err := NormalizeStatCategories(matchStatsBody)
	if err != nil {
		t.Fatalf("NormalizeStatCategories match stats fixture error: %v", err)
	}

	batting, bowling, _ := splitPlayerStatCategories(matchCategories)
	if len(batting) == 0 {
		t.Fatalf("expected batting categories from roster-player stats fixture")
	}
	if len(bowling) == 0 {
		t.Fatalf("expected bowling categories from roster-player stats fixture")
	}

	summary := summarizePlayerMatchCategories(matchCategories)
	if summary.BallsFaced == 0 {
		t.Fatalf("expected ballsFaced summary from roster-player stats fixture, got %+v", summary)
	}
	if summary.StrikeRate == 0 {
		t.Fatalf("expected strikeRate summary from roster-player stats fixture, got %+v", summary)
	}
	if strings.TrimSpace(summary.DismissalName) == "" || strings.TrimSpace(summary.DismissalCard) == "" {
		t.Fatalf("expected dismissalName and dismissalCard in summary, got %+v", summary)
	}
	if summary.EconomyRate == 0 || summary.Maidens == 0 {
		t.Fatalf("expected bowling rate/maidens summary fields, got %+v", summary)
	}
	if strings.TrimSpace(summary.BowlerPlayerID) == "" || strings.TrimSpace(summary.FielderPlayerID) == "" {
		t.Fatalf("expected bowlerPlayerId and fielderPlayerId in summary, got %+v", summary)
	}

	inningsBattingBody := mustReadFixtureBytes(t, "players/roster-1361257-linescores-1-1-statistics-0.json")
	inningsBattingCategories, err := NormalizeStatCategories(inningsBattingBody)
	if err != nil {
		t.Fatalf("NormalizeStatCategories innings batting fixture error: %v", err)
	}
	inningsBatting, inningsBowling, _ := splitPlayerStatCategories(inningsBattingCategories)
	if len(inningsBatting) == 0 || len(inningsBowling) != 0 {
		t.Fatalf("expected batting-only linescore split for period 1/1")
	}

	inningsBowlingBody := mustReadFixtureBytes(t, "players/roster-1361257-linescores-1-2-statistics-0.json")
	inningsBowlingCategories, err := NormalizeStatCategories(inningsBowlingBody)
	if err != nil {
		t.Fatalf("NormalizeStatCategories innings bowling fixture error: %v", err)
	}
	inningsBatting2, inningsBowling2, _ := splitPlayerStatCategories(inningsBowlingCategories)
	if len(inningsBatting2) != 0 || len(inningsBowling2) == 0 {
		t.Fatalf("expected bowling-only linescore split for period 1/2")
	}
}

func TestPhase11FixtureNormalizationPreservesDismissalAndDeliveryMetadata(t *testing.T) {
	t.Parallel()

	statsBody := mustReadFixtureBytes(t, "team-competitor/statistics-789643.json")
	overs, wickets, err := NormalizeInningsPeriodStatistics(statsBody)
	if err != nil {
		t.Fatalf("NormalizeInningsPeriodStatistics fixture error: %v", err)
	}
	if len(overs) == 0 || len(wickets) == 0 {
		t.Fatalf("expected over and wicket timelines from statistics fixture")
	}
	if wickets[0].DetailRef == "" || wickets[0].DismissalCard == "" {
		t.Fatalf("expected wicket detailRef and dismissalCard in normalized wicket timeline, got %+v", wickets[0])
	}
	if wickets[0].StrikeRate == 0 {
		t.Fatalf("expected wicket strikeRate in normalized wicket timeline, got %+v", wickets[0])
	}

	detailBody := mustReadFixtureBytes(t, "details-plays/detail-52559021.json")
	delivery, err := NormalizeDeliveryEvent(detailBody)
	if err != nil {
		t.Fatalf("NormalizeDeliveryEvent dismissal fixture error: %v", err)
	}
	if strings.TrimSpace(delivery.BatsmanPlayerID) == "" {
		t.Fatalf("expected batsmanPlayerId in normalized delivery, got %+v", delivery)
	}
	if strings.TrimSpace(delivery.BowlerPlayerID) == "" {
		t.Fatalf("expected bowlerPlayerId in normalized delivery, got %+v", delivery)
	}
	if strings.TrimSpace(delivery.DismissalType) == "" || strings.TrimSpace(delivery.DismissalName) == "" {
		t.Fatalf("expected dismissal type/name in normalized delivery, got %+v", delivery)
	}
}

func TestPlayerServicePhase11MatchContextCommands(t *testing.T) {
	t.Parallel()

	service := newPhase11PlayerTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	matchStatsResult, err := service.MatchStats(ctx, "Fazal Haq Shaheen", "1529474", PlayerLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("PlayerService.MatchStats error: %v", err)
	}
	if matchStatsResult.Kind != EntityPlayerMatch {
		t.Fatalf("expected kind %q, got %q", EntityPlayerMatch, matchStatsResult.Kind)
	}
	matchStats, ok := matchStatsResult.Data.(PlayerMatch)
	if !ok {
		t.Fatalf("expected PlayerMatch payload, got %T", matchStatsResult.Data)
	}
	if matchStats.PlayerID != "1361257" {
		t.Fatalf("expected player id 1361257, got %+v", matchStats)
	}
	if matchStats.Summary.BallsFaced == 0 || matchStats.Summary.EconomyRate == 0 {
		t.Fatalf("expected batting and bowling summary fields in match stats, got %+v", matchStats.Summary)
	}

	inningsResult, err := service.Innings(ctx, "Fazal Haq Shaheen", "1529474", PlayerLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("PlayerService.Innings error: %v", err)
	}
	if inningsResult.Kind != EntityPlayerInnings {
		t.Fatalf("expected kind %q, got %q", EntityPlayerInnings, inningsResult.Kind)
	}
	if len(inningsResult.Items) == 0 {
		t.Fatalf("expected player innings entries")
	}
	firstInnings, ok := inningsResult.Items[0].(PlayerInnings)
	if !ok {
		t.Fatalf("expected PlayerInnings item, got %T", inningsResult.Items[0])
	}
	if strings.TrimSpace(firstInnings.StatisticsRef) == "" {
		t.Fatalf("expected statistics ref in player innings entry")
	}

	deliveriesResult, err := service.Deliveries(ctx, "Fazal Haq Shaheen", "1529474", PlayerLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("PlayerService.Deliveries error: %v", err)
	}
	if deliveriesResult.Kind != EntityPlayerDelivery {
		t.Fatalf("expected kind %q, got %q", EntityPlayerDelivery, deliveriesResult.Kind)
	}
	if len(deliveriesResult.Items) == 0 {
		t.Fatalf("expected delivery events for player")
	}
	foundDismissalDelivery := false
	for _, raw := range deliveriesResult.Items {
		delivery, ok := raw.(DeliveryEvent)
		if !ok {
			t.Fatalf("expected DeliveryEvent item, got %T", raw)
		}
		if strings.TrimSpace(delivery.BatsmanPlayerID) != "1361257" {
			continue
		}
		if strings.Contains(delivery.Ref, "/details/52559021") {
			foundDismissalDelivery = true
			if strings.TrimSpace(delivery.DismissalType) == "" {
				t.Fatalf("expected dismissal type for dismissal delivery, got %+v", delivery)
			}
			if len(delivery.Involvement) == 0 {
				t.Fatalf("expected player involvement roles on delivery, got %+v", delivery)
			}
		}
	}
	if !foundDismissalDelivery {
		t.Fatalf("expected dismissal delivery detail ref 52559021 in player deliveries output")
	}

	dismissalsResult, err := service.Dismissals(ctx, "Fazal Haq Shaheen", "1529474", PlayerLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("PlayerService.Dismissals error: %v", err)
	}
	if dismissalsResult.Kind != EntityPlayerDismissal {
		t.Fatalf("expected kind %q, got %q", EntityPlayerDismissal, dismissalsResult.Kind)
	}
	if len(dismissalsResult.Items) == 0 {
		t.Fatalf("expected player dismissal entries")
	}
	firstDismissal, ok := dismissalsResult.Items[0].(PlayerDismissal)
	if !ok {
		t.Fatalf("expected PlayerDismissal item, got %T", dismissalsResult.Items[0])
	}
	if strings.TrimSpace(firstDismissal.DetailRef) == "" {
		t.Fatalf("expected dismissal detailRef in player dismissal output")
	}
	if strings.TrimSpace(firstDismissal.DismissalCard) == "" {
		t.Fatalf("expected dismissalCard in player dismissal output")
	}
	if strings.TrimSpace(firstDismissal.BowlerPlayerID) == "" {
		t.Fatalf("expected bowlerPlayerId in player dismissal output")
	}
}

func newPhase11PlayerTestService(t *testing.T) *PlayerService {
	t.Helper()

	competitionFixture := mustReadFixtureBytes(t, "matches-competitions/competition.json")
	rosterFixture := mustReadFixtureBytes(t, "team-competitor/roster-789643.json")
	playerStatsFixture := mustReadFixtureBytes(t, "players/roster-1361257-statistics-0.json")
	playerLinescoresFixture := mustReadFixtureBytes(t, "players/roster-1361257-linescores.json")
	playerLinescore11StatsFixture := mustReadFixtureBytes(t, "players/roster-1361257-linescores-1-1-statistics-0.json")
	playerLinescore12StatsFixture := mustReadFixtureBytes(t, "players/roster-1361257-linescores-1-2-statistics-0.json")
	detail110Fixture := mustReadFixtureBytes(t, "details-plays/detail-110.json")
	detailDismissalFixture := mustReadFixtureBytes(t, "details-plays/detail-52559021.json")
	inningsFixture := mustReadFixtureBytes(t, "innings-fow-partnerships/innings-1-2.json")
	statisticsFixture := mustReadFixtureBytes(t, "team-competitor/statistics-789643.json")
	teamFixture := mustReadFixtureBytes(t, "team-competitor/team-789643.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host + "/v2/sports/cricket"
		competitionPath := "/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474"

		switch {
		case r.URL.Path == competitionPath:
			competition := rewriteFixtureBaseURL(competitionFixture, base)
			_, _ = w.Write(competition)
		case r.URL.Path == "/v2/sports/cricket/teams/789643":
			_, _ = w.Write(rewriteFixtureBaseURL(teamFixture, base))
		case r.URL.Path == "/v2/sports/cricket/teams/789647":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/teams/789647","id":"789647","displayName":"Speen Ghar Region","shortDisplayName":"SGH","abbreviation":"SGH"}`))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/roster"):
			_, _ = w.Write(rewriteFixtureBaseURL(rosterFixture, base))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/roster/1361257/statistics/0"):
			_, _ = w.Write(rewriteFixtureBaseURL(playerStatsFixture, base))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/roster/1361257/linescores"):
			_, _ = w.Write(rewriteFixtureBaseURL(playerLinescoresFixture, base))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/roster/1361257/linescores/1/1/statistics/0"):
			_, _ = w.Write(rewriteFixtureBaseURL(playerLinescore11StatsFixture, base))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/roster/1361257/linescores/1/2/statistics/0"):
			_, _ = w.Write(rewriteFixtureBaseURL(playerLinescore12StatsFixture, base))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/roster/1361257/linescores/1/3/statistics/0"):
			_, _ = w.Write(rewriteFixtureBaseURL(playerLinescore11StatsFixture, base))
		case strings.HasSuffix(r.URL.Path, "/details"):
			plays := fmt.Sprintf(`{"count":2,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/details/110"},{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/details/52559021"}]}`, base, base)
			_, _ = w.Write([]byte(plays))
		case strings.HasSuffix(r.URL.Path, "/details/110"):
			_, _ = w.Write(rewriteFixtureBaseURL(detail110Fixture, base))
		case strings.HasSuffix(r.URL.Path, "/details/52559021"):
			_, _ = w.Write(rewriteFixtureBaseURL(detailDismissalFixture, base))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/linescores"):
			_, _ = w.Write([]byte(fmt.Sprintf(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/competitors/789643/linescores/1/2"}]}`, base)))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/linescores/1/2"):
			_, _ = w.Write(rewriteFixtureBaseURL(inningsFixture, base))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/linescores/1/2/statistics/0"):
			_, _ = w.Write(rewriteFixtureBaseURL(statisticsFixture, base))
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
		{
			Kind:      EntityPlayer,
			ID:        "1361257",
			Ref:       "/athletes/1361257",
			Name:      "Fazal Haq Shaheen",
			ShortName: "Fazal Haq Shaheen",
			LeagueID:  "19138",
			EventID:   "1529474",
			MatchID:   "1529474",
			Aliases:   []string{"Fazal Haq Shaheen", "1361257"},
			UpdatedAt: time.Now().UTC(),
		},
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

	service, err := NewPlayerService(PlayerServiceConfig{
		Client:   client,
		Resolver: resolver,
	})
	if err != nil {
		t.Fatalf("NewPlayerService error: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return service
}
