package cricinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPhase12FixtureNormalizationForCalendarSectionsAndStandingsChains(t *testing.T) {
	t.Parallel()

	onDaysBody := mustReadFixtureBytes(t, "leagues-seasons-standings/calendar-ondays.json")
	onDays, err := NormalizeCalendarDays(onDaysBody)
	if err != nil {
		t.Fatalf("NormalizeCalendarDays ondays error: %v", err)
	}
	if len(onDays) == 0 {
		t.Fatalf("expected calendar day entries from ondays fixture")
	}
	if onDays[0].DayType != "ondays" {
		t.Fatalf("expected ondays type, got %+v", onDays[0])
	}
	if len(onDays[0].Sections) == 0 || onDays[0].Sections[0] != "Regular Season" {
		t.Fatalf("expected section labels from ondays fixture, got %+v", onDays[0])
	}

	offDaysBody := mustReadFixtureBytes(t, "leagues-seasons-standings/calendar-offdays.json")
	offDays, err := NormalizeCalendarDays(offDaysBody)
	if err != nil {
		t.Fatalf("NormalizeCalendarDays offdays error: %v", err)
	}
	if len(offDays) == 0 || offDays[0].DayType != "offdays" {
		t.Fatalf("expected offdays entries, got %+v", offDays)
	}

	standingsItemBody := mustReadFixtureBytes(t, "leagues-seasons-standings/standings-item-1.json")
	group, err := NormalizeStandingsGroup(standingsItemBody)
	if err != nil {
		t.Fatalf("NormalizeStandingsGroup error: %v", err)
	}
	if group.ID != "1" || len(group.Entries) == 0 {
		t.Fatalf("expected standings group entries from standings fixture, got %+v", group)
	}
	if !strings.Contains(group.Entries[0].ScoreSummary, "Rank 1") || !strings.Contains(group.Entries[0].ScoreSummary, "Pts 16") {
		t.Fatalf("expected rank/points summary in standings entry, got %+v", group.Entries[0])
	}
}

func TestLeagueServicePhase12NavigationAndStandingsTraversal(t *testing.T) {
	t.Parallel()

	service := newPhase12LeagueTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	listResult, err := service.List(ctx, LeagueListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("LeagueService.List error: %v", err)
	}
	if listResult.Kind != EntityLeague || listResult.Status == ResultStatusError {
		t.Fatalf("unexpected list result: %+v", listResult)
	}
	if len(listResult.Items) == 0 {
		t.Fatalf("expected list items from /leagues")
	}

	showResult, err := service.Show(ctx, "Mirwais Nika")
	if err != nil {
		t.Fatalf("LeagueService.Show error: %v", err)
	}
	if showResult.Kind != EntityLeague || showResult.Status == ResultStatusError {
		t.Fatalf("unexpected show result: %+v", showResult)
	}
	shownLeague, ok := showResult.Data.(*League)
	if !ok {
		t.Fatalf("expected show data type League, got %T", showResult.Data)
	}
	if shownLeague.ID != "19138" {
		t.Fatalf("expected resolved league id 19138, got %+v", shownLeague)
	}

	eventsResult, err := service.Events(ctx, "Mirwais Nika", LeagueListOptions{Limit: 5})
	if err != nil {
		t.Fatalf("LeagueService.Events error: %v", err)
	}
	if eventsResult.Kind != EntityMatch {
		t.Fatalf("expected events result kind %q, got %q", EntityMatch, eventsResult.Kind)
	}
	if len(eventsResult.Items) == 0 {
		t.Fatalf("expected at least one league event/match item")
	}

	calendarResult, err := service.Calendar(ctx, "Mirwais Nika")
	if err != nil {
		t.Fatalf("LeagueService.Calendar error: %v", err)
	}
	if calendarResult.Kind != EntityCalendarDay {
		t.Fatalf("expected calendar kind %q, got %q", EntityCalendarDay, calendarResult.Kind)
	}
	if len(calendarResult.Items) < 3 {
		t.Fatalf("expected normalized calendar day items, got %d", len(calendarResult.Items))
	}

	athletesResult, err := service.Athletes(ctx, "Mirwais Nika", LeagueListOptions{Limit: 5})
	if err != nil {
		t.Fatalf("LeagueService.Athletes error: %v", err)
	}
	if athletesResult.Kind != EntityPlayer {
		t.Fatalf("expected athletes kind %q, got %q", EntityPlayer, athletesResult.Kind)
	}
	if len(athletesResult.Items) == 0 {
		t.Fatalf("expected athlete results from fallback roster traversal")
	}

	seasonsResult, err := service.Seasons(ctx, "Mirwais Nika")
	if err != nil {
		t.Fatalf("LeagueService.Seasons error: %v", err)
	}
	if seasonsResult.Kind != EntitySeason {
		t.Fatalf("expected seasons kind %q, got %q", EntitySeason, seasonsResult.Kind)
	}
	if len(seasonsResult.Items) == 0 {
		t.Fatalf("expected season refs in seasons output")
	}

	seasonShow, err := service.SeasonShow(ctx, "Mirwais Nika", SeasonLookupOptions{SeasonQuery: "2025"})
	if err != nil {
		t.Fatalf("LeagueService.SeasonShow error: %v", err)
	}
	if seasonShow.Kind != EntitySeason {
		t.Fatalf("expected season show kind %q, got %q", EntitySeason, seasonShow.Kind)
	}
	season, ok := seasonShow.Data.(Season)
	if !ok {
		t.Fatalf("expected season show data type Season, got %T", seasonShow.Data)
	}
	if season.ID != "2025" || season.Year != 2025 {
		t.Fatalf("expected selected season 2025 payload, got %+v", season)
	}

	seasonTypes, err := service.SeasonTypes(ctx, "Mirwais Nika", SeasonLookupOptions{SeasonQuery: "2025"})
	if err != nil {
		t.Fatalf("LeagueService.SeasonTypes error: %v", err)
	}
	if seasonTypes.Kind != EntitySeasonType || len(seasonTypes.Items) == 0 {
		t.Fatalf("expected season types list, got %+v", seasonTypes)
	}

	seasonGroups, err := service.SeasonGroups(ctx, "Mirwais Nika", SeasonLookupOptions{SeasonQuery: "2025", TypeQuery: "1"})
	if err != nil {
		t.Fatalf("LeagueService.SeasonGroups error: %v", err)
	}
	if seasonGroups.Kind != EntitySeasonGroup || len(seasonGroups.Items) == 0 {
		t.Fatalf("expected season groups list, got %+v", seasonGroups)
	}
	firstGroup, ok := seasonGroups.Items[0].(SeasonGroup)
	if !ok {
		t.Fatalf("expected season group item type SeasonGroup, got %T", seasonGroups.Items[0])
	}
	if strings.TrimSpace(firstGroup.StandingsRef) == "" {
		t.Fatalf("expected season group standings ref, got %+v", firstGroup)
	}

	standingsResult, err := service.Standings(ctx, "Mirwais Nika")
	if err != nil {
		t.Fatalf("LeagueService.Standings error: %v", err)
	}
	if standingsResult.Kind != EntityStandingsGroup {
		t.Fatalf("expected standings kind %q, got %q", EntityStandingsGroup, standingsResult.Kind)
	}
	if len(standingsResult.Items) == 0 {
		t.Fatalf("expected standings groups from ref-chain traversal")
	}
	groupItem, ok := standingsResult.Items[0].(StandingsGroup)
	if !ok {
		t.Fatalf("expected standings item type StandingsGroup, got %T", standingsResult.Items[0])
	}
	if len(groupItem.Entries) == 0 {
		t.Fatalf("expected standings entries after traversal, got %+v", groupItem)
	}
	if strings.TrimSpace(groupItem.Entries[0].Name) == "" {
		t.Fatalf("expected team name hydration in standings entries, got %+v", groupItem.Entries[0])
	}
}

func newPhase12LeagueTestService(t *testing.T) *LeagueService {
	t.Helper()

	listFixture := []byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"http://core.espnuk.org/v2/sports/cricket/leagues/19138"}]}`)
	eventsPageFixture := []byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474"}]}`)
	calendarFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/calendar.json")
	onDaysFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/calendar-ondays.json")
	offDaysFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/calendar-offdays.json")
	seasonsFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/seasons.json")
	seasonFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/season-2025.json")
	seasonTypesFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/season-types.json")
	seasonTypeFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/season-type-1.json")
	seasonGroupsFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/season-groups.json")
	seasonGroupFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/season-group-1.json")
	standingsRootFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/standings-root.json")
	standingsListFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/standings.json")
	standingsItemFixture := mustReadFixtureBytes(t, "leagues-seasons-standings/standings-item-1.json")
	rosterFixture := mustReadFixtureBytes(t, "team-competitor/roster-789643.json")
	playerFixture := mustReadFixtureBytes(t, "players/athlete-1361257.json")
	teamFixture := mustReadFixtureBytes(t, "team-competitor/team-789643.json")
	eventFixture := []byte(`{"$ref":"http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474","id":"1529474","shortDescription":"3rd Match","description":"3rd Match, Mirwais Nika","date":"2026-04-09T05:30:00Z","competitions":[{"$ref":"http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474","id":"1529474","description":"3rd Match","shortDescription":"3rd Match","competitors":[{"$ref":"http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/competitors/789643","id":"789643","team":{"$ref":"http://core.espnuk.org/v2/sports/cricket/events/1529474/teams/789643"},"roster":{"$ref":"http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/competitors/789643/roster"}}]}]}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host + "/v2/sports/cricket"
		switch r.URL.Path {
		case "/v2/sports/cricket/events":
			_, _ = w.Write([]byte(`{"count":0,"pageIndex":1,"pageSize":25,"pageCount":0,"items":[]}`))
		case "/v2/sports/cricket/leagues":
			_, _ = w.Write(rewriteFixtureBaseURL(listFixture, base))
		case "/v2/sports/cricket/leagues/19138":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/leagues/19138","id":"19138","uid":"s:200~l:19138","name":"Mirwais Nika Provincial 3-Day","slug":"19138","events":{"$ref":"` + base + `/leagues/19138/events"},"seasons":{"$ref":"` + base + `/leagues/19138/seasons"}}`))
		case "/v2/sports/cricket/leagues/19138/events":
			_, _ = w.Write(rewriteFixtureBaseURL(eventsPageFixture, base))
		case "/v2/sports/cricket/leagues/19138/events/1529474":
			_, _ = w.Write(rewriteFixtureBaseURL(eventFixture, base))
		case "/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/competitors/789643/roster":
			_, _ = w.Write(rewriteFixtureBaseURL(rosterFixture, base))
		case "/v2/sports/cricket/leagues/19138/athletes":
			_, _ = w.Write([]byte(`{"count":0,"pageIndex":1,"pageSize":25,"pageCount":0,"items":[]}`))
		case "/v2/sports/cricket/leagues/19138/athletes/1361257":
			_, _ = w.Write(rewriteFixtureBaseURL(playerFixture, base))
		case "/v2/sports/cricket/athletes/1361257":
			_, _ = w.Write(rewriteFixtureBaseURL(playerFixture, base))
		case "/v2/sports/cricket/leagues/19138/calendar":
			_, _ = w.Write(rewriteFixtureBaseURL(calendarFixture, base))
		case "/v2/sports/cricket/leagues/19138/calendar/ondays":
			_, _ = w.Write(rewriteFixtureBaseURL(onDaysFixture, base))
		case "/v2/sports/cricket/leagues/19138/calendar/offdays":
			_, _ = w.Write(rewriteFixtureBaseURL(offDaysFixture, base))
		case "/v2/sports/cricket/leagues/19138/seasons":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonsFixture, base))
		case "/v2/sports/cricket/leagues/19138/seasons/2025":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonFixture, base))
		case "/v2/sports/cricket/leagues/1479935/seasons/2025":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonFixture, base))
		case "/v2/sports/cricket/leagues/19138/seasons/2025/types":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonTypesFixture, base))
		case "/v2/sports/cricket/leagues/1479935/seasons/2025/types":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonTypesFixture, base))
		case "/v2/sports/cricket/leagues/19138/seasons/2025/types/1":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonTypeFixture, base))
		case "/v2/sports/cricket/leagues/1479935/seasons/2025/types/1":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonTypeFixture, base))
		case "/v2/sports/cricket/leagues/19138/seasons/2025/types/1/groups":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonGroupsFixture, base))
		case "/v2/sports/cricket/leagues/1479935/seasons/2025/types/1/groups":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonGroupsFixture, base))
		case "/v2/sports/cricket/leagues/1479935/seasons/2025/types/1/groups/1":
			_, _ = w.Write(rewriteFixtureBaseURL(seasonGroupFixture, base))
		case "/v2/sports/cricket/leagues/19138/standings":
			_, _ = w.Write(rewriteFixtureBaseURL(standingsRootFixture, base))
		case "/v2/sports/cricket/leagues/1529471/seasons/2026/types/1/groups/1/standings":
			_, _ = w.Write(rewriteFixtureBaseURL(standingsListFixture, base))
		case "/v2/sports/cricket/leagues/1529471/seasons/2026/types/1/groups/1/standings/1":
			_, _ = w.Write(rewriteFixtureBaseURL(standingsItemFixture, base))
		case "/v2/sports/cricket/teams/789643":
			_, _ = w.Write(rewriteFixtureBaseURL(teamFixture, base))
		case "/v2/sports/cricket/teams/789649":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/teams/789649","id":"789649","displayName":"Speen Ghar Region","shortDisplayName":"SGR","abbreviation":"SGR"}`))
		case "/v2/sports/cricket/teams/789651":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/teams/789651","id":"789651","displayName":"Amo Region","shortDisplayName":"AMO","abbreviation":"AMO"}`))
		default:
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

	service, err := NewLeagueService(LeagueServiceConfig{
		Client:   client,
		Resolver: resolver,
	})
	if err != nil {
		t.Fatalf("NewLeagueService error: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return service
}
