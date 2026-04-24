package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
)

type fakePlayerService struct {
	searchResult  cricinfo.NormalizedResult
	profileResult cricinfo.NormalizedResult
	newsResult    cricinfo.NormalizedResult
	statsResult   cricinfo.NormalizedResult
	careerResult  cricinfo.NormalizedResult
	matchResult   cricinfo.NormalizedResult
	inningsResult cricinfo.NormalizedResult
	dismissals    cricinfo.NormalizedResult
	deliveries    cricinfo.NormalizedResult
	bowlingResult cricinfo.NormalizedResult
	battingResult cricinfo.NormalizedResult

	searchQueries   []string
	profileQueries  []string
	newsQueries     []string
	statsQueries    []string
	careerQueries   []string
	matchQueries    []string
	matchRefs       []string
	inningsQueries  []string
	inningsRefs     []string
	dismissQueries  []string
	dismissRefs     []string
	deliveryQueries []string
	deliveryRefs    []string
	bowlingQueries  []string
	bowlingRefs     []string
	battingQueries  []string
	battingRefs     []string
	searchOpts      []cricinfo.PlayerLookupOptions
	profileOpts     []cricinfo.PlayerLookupOptions
	newsOpts        []cricinfo.PlayerLookupOptions
	statsOpts       []cricinfo.PlayerLookupOptions
	careerOpts      []cricinfo.PlayerLookupOptions
	matchOpts       []cricinfo.PlayerLookupOptions
	inningsOpts     []cricinfo.PlayerLookupOptions
	dismissOpts     []cricinfo.PlayerLookupOptions
	deliveryOpts    []cricinfo.PlayerLookupOptions
	bowlingOpts     []cricinfo.PlayerLookupOptions
	battingOpts     []cricinfo.PlayerLookupOptions
}

func (f *fakePlayerService) Close() error { return nil }

func (f *fakePlayerService) Search(_ context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.searchQueries = append(f.searchQueries, query)
	f.searchOpts = append(f.searchOpts, opts)
	return f.searchResult, nil
}

func (f *fakePlayerService) Profile(_ context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.profileQueries = append(f.profileQueries, query)
	f.profileOpts = append(f.profileOpts, opts)
	return f.profileResult, nil
}

func (f *fakePlayerService) News(_ context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.newsQueries = append(f.newsQueries, query)
	f.newsOpts = append(f.newsOpts, opts)
	return f.newsResult, nil
}

func (f *fakePlayerService) Stats(_ context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.statsQueries = append(f.statsQueries, query)
	f.statsOpts = append(f.statsOpts, opts)
	return f.statsResult, nil
}

func (f *fakePlayerService) Career(_ context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.careerQueries = append(f.careerQueries, query)
	f.careerOpts = append(f.careerOpts, opts)
	return f.careerResult, nil
}

func (f *fakePlayerService) MatchStats(_ context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.matchQueries = append(f.matchQueries, playerQuery)
	f.matchRefs = append(f.matchRefs, matchQuery)
	f.matchOpts = append(f.matchOpts, opts)
	return f.matchResult, nil
}

func (f *fakePlayerService) Innings(_ context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.inningsQueries = append(f.inningsQueries, playerQuery)
	f.inningsRefs = append(f.inningsRefs, matchQuery)
	f.inningsOpts = append(f.inningsOpts, opts)
	return f.inningsResult, nil
}

func (f *fakePlayerService) Dismissals(_ context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.dismissQueries = append(f.dismissQueries, playerQuery)
	f.dismissRefs = append(f.dismissRefs, matchQuery)
	f.dismissOpts = append(f.dismissOpts, opts)
	return f.dismissals, nil
}

func (f *fakePlayerService) Deliveries(_ context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.deliveryQueries = append(f.deliveryQueries, playerQuery)
	f.deliveryRefs = append(f.deliveryRefs, matchQuery)
	f.deliveryOpts = append(f.deliveryOpts, opts)
	return f.deliveries, nil
}

func (f *fakePlayerService) Bowling(_ context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.bowlingQueries = append(f.bowlingQueries, playerQuery)
	f.bowlingRefs = append(f.bowlingRefs, matchQuery)
	f.bowlingOpts = append(f.bowlingOpts, opts)
	return f.bowlingResult, nil
}

func (f *fakePlayerService) Batting(_ context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error) {
	f.battingQueries = append(f.battingQueries, playerQuery)
	f.battingRefs = append(f.battingRefs, matchQuery)
	f.battingOpts = append(f.battingOpts, opts)
	return f.battingResult, nil
}

func TestPlayersCommandsResolveNamesAndRenderStableJSON(t *testing.T) {
	service := &fakePlayerService{
		searchResult: cricinfo.NewListResult(cricinfo.EntityPlayer, []any{
			cricinfo.Player{ID: "253802", DisplayName: "Virat Kohli"},
		}),
		profileResult: cricinfo.NewDataResult(cricinfo.EntityPlayer, cricinfo.Player{
			ID:          "253802",
			DisplayName: "Virat Kohli",
			FullName:    "Virat Kohli",
			Position:    "Top Order Batter",
			NewsRef:     "http://core.espnuk.org/v2/sports/cricket/athletes/253802/news",
			Team:        &cricinfo.PlayerAffiliation{ID: "6", Ref: "http://core.espnuk.org/v2/sports/cricket/teams/6"},
			MajorTeams: []cricinfo.PlayerAffiliation{
				{ID: "6", Ref: "http://core.espnuk.org/v2/sports/cricket/teams/6"},
			},
		}),
		newsResult: cricinfo.NewListResult(cricinfo.EntityNewsArticle, []any{
			cricinfo.NewsArticle{ID: "1530499", Headline: "Players with a hat-trick of POTM awards feat. Kohli, Kallis, Sehwag and more", Published: "2026-04-04T00:00Z"},
		}),
		statsResult: cricinfo.NewDataResult(cricinfo.EntityPlayerStats, cricinfo.PlayerStatistics{
			PlayerID: "253802",
			Name:     "Total",
			Categories: []cricinfo.StatCategory{
				{
					Name:        "general",
					DisplayName: "General",
					Stats: []cricinfo.StatValue{
						{Name: "matches", DisplayName: "Matches", DisplayValue: "302"},
					},
				},
			},
		}),
		careerResult: cricinfo.NewDataResult(cricinfo.EntityPlayerStats, cricinfo.PlayerStatistics{
			PlayerID: "253802",
			Name:     "Total",
		}),
		matchResult: cricinfo.NewDataResult(cricinfo.EntityPlayerMatch, cricinfo.PlayerMatch{
			PlayerID:   "253802",
			PlayerName: "Virat Kohli",
			MatchID:    "1529474",
			Summary: cricinfo.PlayerMatchSummary{
				BallsFaced:    60,
				StrikeRate:    55,
				DismissalName: "caught",
				DismissalCard: "c",
			},
		}),
		inningsResult: cricinfo.NewListResult(cricinfo.EntityPlayerInnings, []any{
			cricinfo.PlayerInnings{PlayerID: "253802", InningsNumber: 1, Period: 1},
		}),
		dismissals: cricinfo.NewListResult(cricinfo.EntityPlayerDismissal, []any{
			cricinfo.PlayerDismissal{PlayerID: "253802", DetailRef: "http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/details/52545007", DismissalCard: "c"},
		}),
		deliveries: cricinfo.NewListResult(cricinfo.EntityPlayerDelivery, []any{
			cricinfo.DeliveryEvent{ID: "110", BatsmanPlayerID: "253802", BowlerPlayerID: "976585", Involvement: []string{"batting"}},
		}),
		bowlingResult: cricinfo.NewDataResult(cricinfo.EntityPlayerMatch, cricinfo.PlayerMatch{
			PlayerID: "253802",
			MatchID:  "1529474",
			Summary:  cricinfo.PlayerMatchSummary{Dots: 20, EconomyRate: 3.5},
		}),
		battingResult: cricinfo.NewDataResult(cricinfo.EntityPlayerMatch, cricinfo.PlayerMatch{
			PlayerID: "253802",
			MatchID:  "1529474",
			Summary:  cricinfo.PlayerMatchSummary{BallsFaced: 60, StrikeRate: 55},
		}),
	}

	originalFactory := newPlayerService
	newPlayerService = func() (playerCommandService, error) { return service, nil }
	defer func() {
		newPlayerService = originalFactory
	}()

	var searchOut bytes.Buffer
	var searchErr bytes.Buffer
	if err := Run([]string{"players", "search", "Virat", "Kohli", "--format", "json", "--limit", "5"}, &searchOut, &searchErr); err != nil {
		t.Fatalf("Run players search error: %v", err)
	}
	searchPayload := decodeCLIJSONMap(t, searchOut.Bytes())
	if searchPayload["kind"] != string(cricinfo.EntityPlayer) {
		t.Fatalf("expected kind %q, got %#v", cricinfo.EntityPlayer, searchPayload["kind"])
	}
	if len(service.searchQueries) != 1 || service.searchQueries[0] != "Virat Kohli" {
		t.Fatalf("expected joined search query, got %+v", service.searchQueries)
	}
	if service.searchOpts[0].Limit != 5 {
		t.Fatalf("expected search limit 5, got %+v", service.searchOpts)
	}

	var profileOut bytes.Buffer
	var profileErr bytes.Buffer
	if err := Run([]string{"players", "profile", "Virat", "Kohli", "--format", "json"}, &profileOut, &profileErr); err != nil {
		t.Fatalf("Run players profile error: %v", err)
	}
	profilePayload := decodeCLIJSONMap(t, profileOut.Bytes())
	if profilePayload["kind"] != string(cricinfo.EntityPlayer) {
		t.Fatalf("expected player profile kind, got %#v", profilePayload["kind"])
	}
	profileData, ok := profilePayload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected player profile object")
	}
	if profileData["displayName"] != "Virat Kohli" {
		t.Fatalf("expected displayName in player profile, got %#v", profileData["displayName"])
	}
	if _, ok := profileData["majorTeams"]; !ok {
		t.Fatalf("expected majorTeams in player profile output")
	}

	var newsOut bytes.Buffer
	var newsErr bytes.Buffer
	if err := Run([]string{"players", "news", "Virat", "Kohli", "--format", "text"}, &newsOut, &newsErr); err != nil {
		t.Fatalf("Run players news error: %v", err)
	}
	if !strings.Contains(newsOut.String(), "POTM awards") {
		t.Fatalf("expected news headline in text output, got %q", newsOut.String())
	}

	var statsOut bytes.Buffer
	var statsErr bytes.Buffer
	if err := Run([]string{"players", "stats", "Virat", "Kohli", "--format", "json"}, &statsOut, &statsErr); err != nil {
		t.Fatalf("Run players stats error: %v", err)
	}
	statsPayload := decodeCLIJSONMap(t, statsOut.Bytes())
	if statsPayload["kind"] != string(cricinfo.EntityPlayerStats) {
		t.Fatalf("expected kind %q, got %#v", cricinfo.EntityPlayerStats, statsPayload["kind"])
	}
	statsData, ok := statsPayload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected player statistics object")
	}
	categories, ok := statsData["categories"].([]any)
	if !ok || len(categories) == 0 {
		t.Fatalf("expected grouped categories in player statistics output")
	}

	var careerOut bytes.Buffer
	var careerErr bytes.Buffer
	if err := Run([]string{"players", "career", "Virat", "Kohli", "--format", "json"}, &careerOut, &careerErr); err != nil {
		t.Fatalf("Run players career error: %v", err)
	}

	var matchOut bytes.Buffer
	var matchErr bytes.Buffer
	if err := Run([]string{"players", "match-stats", "Virat", "Kohli", "--match", "1529474", "--format", "json"}, &matchOut, &matchErr); err != nil {
		t.Fatalf("Run players match-stats error: %v", err)
	}
	matchPayload := decodeCLIJSONMap(t, matchOut.Bytes())
	if matchPayload["kind"] != string(cricinfo.EntityPlayerMatch) {
		t.Fatalf("expected kind %q, got %#v", cricinfo.EntityPlayerMatch, matchPayload["kind"])
	}

	var inningsOut bytes.Buffer
	var inningsErr bytes.Buffer
	if err := Run([]string{"players", "innings", "Virat", "Kohli", "--match", "1529474", "--format", "json"}, &inningsOut, &inningsErr); err != nil {
		t.Fatalf("Run players innings error: %v", err)
	}

	var dismissOut bytes.Buffer
	var dismissErr bytes.Buffer
	if err := Run([]string{"players", "dismissals", "Virat", "Kohli", "--match", "1529474", "--format", "json"}, &dismissOut, &dismissErr); err != nil {
		t.Fatalf("Run players dismissals error: %v", err)
	}

	var deliveriesOut bytes.Buffer
	var deliveriesErr bytes.Buffer
	if err := Run([]string{"players", "deliveries", "Virat", "Kohli", "--match", "1529474", "--format", "json"}, &deliveriesOut, &deliveriesErr); err != nil {
		t.Fatalf("Run players deliveries error: %v", err)
	}

	var bowlingOut bytes.Buffer
	var bowlingErr bytes.Buffer
	if err := Run([]string{"players", "bowling", "Virat", "Kohli", "--match", "1529474", "--format", "json"}, &bowlingOut, &bowlingErr); err != nil {
		t.Fatalf("Run players bowling error: %v", err)
	}

	var battingOut bytes.Buffer
	var battingErr bytes.Buffer
	if err := Run([]string{"players", "batting", "Virat", "Kohli", "--match", "1529474", "--format", "json"}, &battingOut, &battingErr); err != nil {
		t.Fatalf("Run players batting error: %v", err)
	}

	if service.profileQueries[0] != "Virat Kohli" || service.newsQueries[0] != "Virat Kohli" || service.statsQueries[0] != "Virat Kohli" || service.careerQueries[0] != "Virat Kohli" {
		t.Fatalf("expected all player commands to preserve joined alias query")
	}
	if len(service.matchRefs) != 1 || service.matchRefs[0] != "1529474" {
		t.Fatalf("expected match context to be passed through for player match commands")
	}
}

func TestPlayersHelpListsPhase10Commands(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var errBuf bytes.Buffer
	if err := Run([]string{"players", "--help"}, &out, &errBuf); err != nil {
		t.Fatalf("Run players --help error: %v", err)
	}

	helpText := out.String()
	for _, snippet := range []string{
		"players search", "players profile", "players news", "players stats", "players career",
		"players match-stats", "players innings", "players dismissals", "players deliveries", "players bowling", "players batting",
		"players map-history",
	} {
		if !strings.Contains(helpText, snippet) {
			t.Fatalf("expected help text to include %q, got %q", snippet, helpText)
		}
	}
}

func TestPlayersMapHistoryRequiresScopeFlag(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var errBuf bytes.Buffer
	err := Run([]string{"players", "map-history", "Virat", "Kohli"}, &out, &errBuf)
	if err == nil || !strings.Contains(err.Error(), "--scope is required") {
		t.Fatalf("expected --scope required error, got %v", err)
	}
}

func TestPlayersMatchContextCommandsRequireMatchFlag(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"players", "match-stats", "Virat Kohli"},
		{"players", "innings", "Virat Kohli"},
		{"players", "dismissals", "Virat Kohli"},
		{"players", "deliveries", "Virat Kohli"},
		{"players", "bowling", "Virat Kohli"},
		{"players", "batting", "Virat Kohli"},
	} {
		var out bytes.Buffer
		var errBuf bytes.Buffer
		err := Run(args, &out, &errBuf)
		if err == nil || !strings.Contains(err.Error(), "--match is required") {
			t.Fatalf("expected --match required error for %v, got %v", args, err)
		}
	}
}

func TestPlayersProfileJSONIsStable(t *testing.T) {
	t.Parallel()

	player := cricinfo.Player{
		ID:          "1361257",
		DisplayName: "Fazal Haq Shaheen",
		Styles: []cricinfo.PlayerStyle{
			{Type: "batting", Description: "Left-hand bat", ShortDescription: "Lhb"},
		},
	}

	var out bytes.Buffer
	if err := cricinfo.Render(&out, cricinfo.NewDataResult(cricinfo.EntityPlayer, player), cricinfo.RenderOptions{Format: "json"}); err != nil {
		t.Fatalf("Render player profile json error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode rendered player profile json: %v", err)
	}
	if payload["kind"] != string(cricinfo.EntityPlayer) {
		t.Fatalf("expected player kind, got %#v", payload["kind"])
	}
}
