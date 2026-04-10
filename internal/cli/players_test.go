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

	searchQueries  []string
	profileQueries []string
	newsQueries    []string
	statsQueries   []string
	careerQueries  []string
	searchOpts     []cricinfo.PlayerLookupOptions
	profileOpts    []cricinfo.PlayerLookupOptions
	newsOpts       []cricinfo.PlayerLookupOptions
	statsOpts      []cricinfo.PlayerLookupOptions
	careerOpts     []cricinfo.PlayerLookupOptions
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

	if service.profileQueries[0] != "Virat Kohli" || service.newsQueries[0] != "Virat Kohli" || service.statsQueries[0] != "Virat Kohli" || service.careerQueries[0] != "Virat Kohli" {
		t.Fatalf("expected all player commands to preserve joined alias query")
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
	for _, snippet := range []string{"players search", "players profile", "players news", "players stats", "players career"} {
		if !strings.Contains(helpText, snippet) {
			t.Fatalf("expected help text to include %q, got %q", snippet, helpText)
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
