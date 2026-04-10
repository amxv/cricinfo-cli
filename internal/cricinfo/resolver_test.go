package cricinfo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolverSearchSupportsNumericAndKnownRef(t *testing.T) {
	t.Parallel()

	server, eventsHits := newResolverFixtureServer(t)
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:    server.URL,
		MaxRetries: 0,
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	resolver, err := NewResolver(ResolverConfig{
		Client:       client,
		IndexPath:    filepath.Join(t.TempDir(), "resolver-index.json"),
		EventSeedTTL: time.Hour,
		MaxEventSeed: 5,
	})
	if err != nil {
		t.Fatalf("NewResolver error: %v", err)
	}
	defer func() {
		_ = resolver.Close()
	}()

	players, err := resolver.Search(context.Background(), EntityPlayer, "51", ResolveOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Search players error: %v", err)
	}
	if len(players.Entities) == 0 || players.Entities[0].ID != "51" {
		t.Fatalf("expected player 51 from numeric resolution, got %+v", players.Entities)
	}

	teams, err := resolver.Search(context.Background(), EntityTeam, server.URL+"/teams/31", ResolveOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Search teams error: %v", err)
	}
	if len(teams.Entities) == 0 || teams.Entities[0].ID != "31" {
		t.Fatalf("expected team 31 from known-ref resolution, got %+v", teams.Entities)
	}

	leagues, err := resolver.Search(context.Background(), EntityLeague, "11", ResolveOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Search leagues error: %v", err)
	}
	if len(leagues.Entities) == 0 || leagues.Entities[0].ID != "11" {
		t.Fatalf("expected league 11 from numeric resolution, got %+v", leagues.Entities)
	}

	matches, err := resolver.Search(context.Background(), EntityMatch, "41", ResolveOptions{Limit: 3, LeagueID: "11"})
	if err != nil {
		t.Fatalf("Search matches error: %v", err)
	}
	if len(matches.Entities) == 0 || matches.Entities[0].ID != "41" {
		t.Fatalf("expected match 41 from numeric/context-aware resolution, got %+v", matches.Entities)
	}

	if got := eventsHits.Load(); got == 0 {
		t.Fatalf("expected /events to be used for incremental seed")
	}
}

func TestResolverSearchReusesPersistedEventSeedToAvoidTransportChurn(t *testing.T) {
	t.Parallel()

	server, eventsHits := newResolverFixtureServer(t)
	defer server.Close()

	cachePath := filepath.Join(t.TempDir(), "resolver-index.json")

	newResolverFromDisk := func() *Resolver {
		t.Helper()
		client, err := NewClient(Config{
			BaseURL:    server.URL,
			MaxRetries: 0,
			Timeout:    2 * time.Second,
		})
		if err != nil {
			t.Fatalf("NewClient error: %v", err)
		}
		resolver, err := NewResolver(ResolverConfig{
			Client:       client,
			IndexPath:    cachePath,
			EventSeedTTL: 24 * time.Hour,
			MaxEventSeed: 5,
			Now:          func() time.Time { return time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC) },
		})
		if err != nil {
			t.Fatalf("NewResolver error: %v", err)
		}
		return resolver
	}

	resolver1 := newResolverFromDisk()
	if _, err := resolver1.Search(context.Background(), EntityMatch, "", ResolveOptions{Limit: 5}); err != nil {
		t.Fatalf("first Search error: %v", err)
	}
	if err := resolver1.Close(); err != nil {
		t.Fatalf("resolver1 Close error: %v", err)
	}

	firstEventsHits := eventsHits.Load()
	if firstEventsHits == 0 {
		t.Fatalf("expected first search to hit /events")
	}

	resolver2 := newResolverFromDisk()
	if _, err := resolver2.Search(context.Background(), EntityMatch, "", ResolveOptions{Limit: 5}); err != nil {
		t.Fatalf("second Search error: %v", err)
	}
	if err := resolver2.Close(); err != nil {
		t.Fatalf("resolver2 Close error: %v", err)
	}

	if got := eventsHits.Load(); got != firstEventsHits {
		t.Fatalf("expected second search to reuse cached /events seed (hits=%d), got %d", firstEventsHits, got)
	}
}

func TestResolverSearchHydratesNamelessCachedPlayerByID(t *testing.T) {
	t.Parallel()

	server, _ := newResolverFixtureServer(t)
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:    server.URL,
		MaxRetries: 0,
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	resolver, err := NewResolver(ResolverConfig{
		Client:       client,
		IndexPath:    filepath.Join(t.TempDir(), "resolver-index.json"),
		EventSeedTTL: time.Hour,
		MaxEventSeed: 5,
	})
	if err != nil {
		t.Fatalf("NewResolver error: %v", err)
	}
	defer func() {
		_ = resolver.Close()
	}()

	if err := resolver.index.Upsert(IndexedEntity{
		Kind:      EntityPlayer,
		ID:        "51",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed nameless player error: %v", err)
	}

	result, err := resolver.Search(context.Background(), EntityPlayer, "51", ResolveOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Search player by id error: %v", err)
	}
	if len(result.Entities) == 0 {
		t.Fatalf("expected player search results")
	}
	if strings.TrimSpace(result.Entities[0].Name) == "" {
		t.Fatalf("expected resolver to hydrate nameless cached player, got %+v", result.Entities[0])
	}
}

func newResolverFixtureServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	eventsHits := &atomic.Int32{}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/events":
			eventsHits.Add(1)
			_, _ = fmt.Fprintf(w, `{"count":1,"items":[{"$ref":"%s/leagues/11/events/21"}],"pageCount":1,"pageIndex":1,"pageSize":25}`,
				server.URL,
			)
		case "/events/41":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		case "/leagues/11/events/21":
			_, _ = fmt.Fprintf(w, `{"$ref":"%s/leagues/11/events/21","id":"21","shortDescription":"Fixture Event","leagues":[{"$ref":"%s/leagues/11","id":"11","name":"League Eleven"}],"competitions":[{"$ref":"%s/leagues/11/events/21/competitions/41","id":"41","description":"Fixture Match","shortDescription":"Match 41","competitors":[{"$ref":"%s/leagues/11/events/21/competitions/41/competitors/31","id":"31","team":{"$ref":"%s/teams/31"},"roster":{"$ref":"%s/leagues/11/events/21/competitions/41/competitors/31/roster"}}]}]}`,
				server.URL,
				server.URL,
				server.URL,
				server.URL,
				server.URL,
				server.URL,
			)
		case "/leagues/11/events/21/competitions/41":
			_, _ = fmt.Fprintf(w, `{"$ref":"%s/leagues/11/events/21/competitions/41","id":"41","description":"Fixture Match","shortDescription":"Match 41","competitors":[{"$ref":"%s/leagues/11/events/21/competitions/41/competitors/31","id":"31","team":{"$ref":"%s/teams/31"},"roster":{"$ref":"%s/leagues/11/events/21/competitions/41/competitors/31/roster"}}]}`,
				server.URL,
				server.URL,
				server.URL,
				server.URL,
			)
		case "/leagues/11/events/21/competitions/41/competitors/31/roster":
			_, _ = fmt.Fprintf(w, `{"entries":[{"playerId":"51","athlete":{"$ref":"%s/athletes/51"}}]}`,
				server.URL,
			)
		case "/athletes/51":
			_, _ = fmt.Fprintf(w, `{"$ref":"%s/athletes/51","id":"51","displayName":"Fixture Player","fullName":"Fixture Player"}`,
				server.URL,
			)
		case "/teams/31":
			_, _ = fmt.Fprintf(w, `{"$ref":"%s/teams/31","id":"31","displayName":"Fixture Team","shortDisplayName":"FTeam","abbreviation":"FT"}`,
				server.URL,
			)
		case "/leagues/11":
			_, _ = fmt.Fprintf(w, `{"$ref":"%s/leagues/11","id":"11","name":"League Eleven","slug":"league-eleven","abbreviation":"L11"}`,
				server.URL,
			)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(w, `{"error":"unexpected path %s"}`,
				r.URL.Path,
			)
		}
	}))

	return server, eventsHits
}
