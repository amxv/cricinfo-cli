package cricinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlayerServicePhase10SearchProfileNewsAndStatistics(t *testing.T) {
	t.Parallel()

	athleteFixture := mustReadFixtureBytes(t, "players/athlete-1361257.json")
	statisticsFixture := mustReadFixtureBytes(t, "players/athlete-1361257-statistics.json")

	newsPage := []byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"http://core.espnuk.org/v2/sports/cricket/news/1530499"}]}`)
	newsItem := []byte(`{"$ref":"http://core.espnuk.org/v2/sports/cricket/news/1530499","id":"1530499","uid":"s:200~n:1530499","headline":"Virat Kohli news headline","title":"Virat Kohli news headline","linkText":"Virat Kohli news headline","byline":"ESPNcricinfo staff","description":"Story description","published":"2026-04-04T00:00Z","lastModified":"2026-04-05T11:08Z","links":{"api":{"v1":{"href":"http://api.espncricinfo.com/1/story/1530499"}},"web":{"href":"https://www.espn.in/ci/content/story/1530499.html"}}}`)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := server.URL
		switch r.URL.Path {
		case "/v2/sports/cricket/events":
			_, _ = w.Write([]byte(`{"count":0,"pageIndex":1,"pageSize":25,"pageCount":0,"items":[]}`))
		case "/v2/sports/cricket/athletes/1361257", "/athletes/1361257":
			_, _ = w.Write(rewriteFixtureBaseURL(athleteFixture, base))
		case "/v2/sports/cricket/athletes/1361257/statistics", "/athletes/1361257/statistics":
			_, _ = w.Write(rewriteFixtureBaseURL(statisticsFixture, base))
		case "/v2/sports/cricket/athletes/1361257/news", "/athletes/1361257/news":
			_, _ = w.Write(rewriteFixtureBaseURL(newsPage, base))
		case "/v2/sports/cricket/news/1530499", "/news/1530499":
			_, _ = w.Write(rewriteFixtureBaseURL(newsItem, base))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL + "/v2/sports/cricket"})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	index, err := OpenEntityIndex(t.TempDir() + "/entity-index.json")
	if err != nil {
		t.Fatalf("OpenEntityIndex error: %v", err)
	}
	if err := index.Upsert(IndexedEntity{
		Kind:      EntityPlayer,
		ID:        "1361257",
		Ref:       server.URL + "/v2/sports/cricket/athletes/1361257",
		Name:      "Fazal Haq Shaheen",
		ShortName: "Fazal Haq Shaheen",
		Aliases:   []string{"Fazal Haq Shaheen", "fazal haq shaheen", "1361257"},
	}); err != nil {
		t.Fatalf("index upsert error: %v", err)
	}

	resolver, err := NewResolver(ResolverConfig{
		Client:       client,
		Index:        index,
		EventSeedTTL: 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewResolver error: %v", err)
	}

	service, err := NewPlayerService(PlayerServiceConfig{
		Client:   client,
		Resolver: resolver,
	})
	if err != nil {
		t.Fatalf("NewPlayerService error: %v", err)
	}

	ctx := context.Background()

	searchResult, err := service.Search(ctx, "Fazal Haq Shaheen", PlayerLookupOptions{Limit: 5})
	if err != nil {
		t.Fatalf("PlayerService.Search error: %v", err)
	}
	if len(searchResult.Items) == 0 {
		t.Fatalf("expected search items")
	}

	profileResult, err := service.Profile(ctx, "Fazal Haq Shaheen", PlayerLookupOptions{})
	if err != nil {
		t.Fatalf("PlayerService.Profile error: %v", err)
	}
	profile, ok := profileResult.Data.(Player)
	if !ok {
		t.Fatalf("expected player profile data, got %T", profileResult.Data)
	}
	if profile.ID != "1361257" || profile.Team == nil {
		t.Fatalf("expected normalized player profile with team, got %+v", profile)
	}
	if len(profile.Styles) == 0 || len(profile.MajorTeams) == 0 {
		t.Fatalf("expected normalized styles and major teams, got %+v", profile)
	}

	statsResult, err := service.Stats(ctx, "Fazal Haq Shaheen", PlayerLookupOptions{})
	if err != nil {
		t.Fatalf("PlayerService.Stats error: %v", err)
	}
	stats, ok := statsResult.Data.(PlayerStatistics)
	if !ok {
		t.Fatalf("expected player statistics data, got %T", statsResult.Data)
	}
	if stats.PlayerID != "1361257" || len(stats.Categories) == 0 {
		t.Fatalf("expected grouped player statistics, got %+v", stats)
	}

	newsResult, err := service.News(ctx, "Fazal Haq Shaheen", PlayerLookupOptions{Limit: 5})
	if err != nil {
		t.Fatalf("PlayerService.News error: %v", err)
	}
	if newsResult.Kind != EntityNewsArticle {
		t.Fatalf("expected news result kind %q, got %q", EntityNewsArticle, newsResult.Kind)
	}
	if len(newsResult.Items) != 1 {
		t.Fatalf("expected 1 news item, got %d (status=%s message=%q warnings=%v)", len(newsResult.Items), newsResult.Status, newsResult.Message, newsResult.Warnings)
	}
	article, ok := newsResult.Items[0].(NewsArticle)
	if !ok {
		t.Fatalf("expected normalized news article item, got %T", newsResult.Items[0])
	}
	if !strings.Contains(article.Headline, "Virat Kohli") {
		t.Fatalf("expected normalized headline, got %+v", article)
	}
}

func TestPlayerServicePhase10MissingPlayerReturnsEmptyResult(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/sports/cricket/events":
			_, _ = w.Write([]byte(`{"count":0,"pageIndex":1,"pageSize":25,"pageCount":0,"items":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	index, err := OpenEntityIndex(t.TempDir() + "/entity-index.json")
	if err != nil {
		t.Fatalf("OpenEntityIndex error: %v", err)
	}
	client, err := NewClient(Config{BaseURL: server.URL + "/v2/sports/cricket"})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	resolver, err := NewResolver(ResolverConfig{Client: client, Index: index})
	if err != nil {
		t.Fatalf("NewResolver error: %v", err)
	}
	service, err := NewPlayerService(PlayerServiceConfig{Client: client, Resolver: resolver})
	if err != nil {
		t.Fatalf("NewPlayerService error: %v", err)
	}

	result, err := service.Profile(context.Background(), "missing player", PlayerLookupOptions{})
	if err != nil {
		t.Fatalf("PlayerService.Profile error: %v", err)
	}
	if result.Status != ResultStatusEmpty {
		t.Fatalf("expected empty result, got %+v", result)
	}
	if !strings.Contains(result.Message, "no players found") {
		t.Fatalf("expected missing player message, got %q", result.Message)
	}
}
