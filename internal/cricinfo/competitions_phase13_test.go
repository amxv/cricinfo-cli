package cricinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestCompetitionServicePhase13MetadataRoutesAndEmptyCollections(t *testing.T) {
	t.Parallel()

	service := newPhase13CompetitionTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	showResult, err := service.Show(ctx, "3rd Match", CompetitionLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Show error: %v", err)
	}
	if showResult.Kind != EntityCompetition {
		t.Fatalf("expected show kind %q, got %q", EntityCompetition, showResult.Kind)
	}
	showCompetition, ok := showResult.Data.(Competition)
	if !ok {
		t.Fatalf("expected show data type Competition, got %T", showResult.Data)
	}
	if showCompetition.OfficialsRef == "" || showCompetition.BroadcastsRef == "" || showCompetition.TicketsRef == "" || showCompetition.OddsRef == "" {
		t.Fatalf("expected competition metadata refs in show payload, got %+v", showCompetition)
	}

	officialsResult, err := service.Officials(ctx, "3rd Match", CompetitionLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Officials error: %v", err)
	}
	if officialsResult.Kind != EntityCompOfficial {
		t.Fatalf("expected officials kind %q, got %q", EntityCompOfficial, officialsResult.Kind)
	}
	if len(officialsResult.Items) == 0 {
		t.Fatalf("expected officials entries")
	}

	broadcastsResult, err := service.Broadcasts(ctx, "3rd Match", CompetitionLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Broadcasts error: %v", err)
	}
	if broadcastsResult.Kind != EntityCompBroadcast {
		t.Fatalf("expected broadcasts kind %q, got %q", EntityCompBroadcast, broadcastsResult.Kind)
	}
	if broadcastsResult.Status != ResultStatusEmpty {
		t.Fatalf("expected empty status for empty broadcasts collection, got %q", broadcastsResult.Status)
	}
	if len(broadcastsResult.Warnings) > 0 {
		t.Fatalf("expected no warnings for empty-but-valid broadcasts collection, got %+v", broadcastsResult.Warnings)
	}

	ticketsResult, err := service.Tickets(ctx, "3rd Match", CompetitionLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Tickets error: %v", err)
	}
	if ticketsResult.Status != ResultStatusEmpty {
		t.Fatalf("expected empty status for empty tickets collection, got %q", ticketsResult.Status)
	}
	if len(ticketsResult.Warnings) > 0 {
		t.Fatalf("expected no warnings for empty-but-valid tickets collection, got %+v", ticketsResult.Warnings)
	}

	oddsResult, err := service.Odds(ctx, "3rd Match", CompetitionLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Odds error: %v", err)
	}
	if oddsResult.Kind != EntityCompOdds {
		t.Fatalf("expected odds kind %q, got %q", EntityCompOdds, oddsResult.Kind)
	}
	if len(oddsResult.Items) == 0 {
		t.Fatalf("expected odds entries")
	}

	metadataResult, err := service.Metadata(ctx, "3rd Match", CompetitionLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Metadata error: %v", err)
	}
	if metadataResult.Kind != EntityCompMetadata {
		t.Fatalf("expected metadata kind %q, got %q", EntityCompMetadata, metadataResult.Kind)
	}
	if metadataResult.Status != ResultStatusOK {
		t.Fatalf("expected metadata status ok, got %q (warnings=%+v)", metadataResult.Status, metadataResult.Warnings)
	}
	metadataSummary, ok := metadataResult.Data.(CompetitionMetadataSummary)
	if !ok {
		t.Fatalf("expected metadata data type CompetitionMetadataSummary, got %T", metadataResult.Data)
	}
	if len(metadataSummary.Officials) == 0 {
		t.Fatalf("expected aggregated officials entries")
	}
	if len(metadataSummary.Broadcasts) != 0 || len(metadataSummary.Tickets) != 0 {
		t.Fatalf("expected empty broadcasts/tickets in metadata summary, got broadcasts=%d tickets=%d", len(metadataSummary.Broadcasts), len(metadataSummary.Tickets))
	}
	if len(metadataSummary.Odds) == 0 {
		t.Fatalf("expected aggregated odds entries")
	}
}

func newPhase13CompetitionTestService(t *testing.T) *CompetitionService {
	t.Helper()

	competitionFixture := mustReadFixtureBytes(t, "matches-competitions/competition.json")
	officialsFixture := mustReadFixtureBytes(t, "aux-competition-metadata/officials.json")
	broadcastsFixture := mustReadFixtureBytes(t, "aux-competition-metadata/broadcasts.json")
	leagueFixture := []byte(`{"$ref":"http://core.espnuk.org/v2/sports/cricket/leagues/19138","id":"19138","name":"Indian Premier League","shortName":"IPL","slug":"ipl","abbreviation":"IPL"}`)
	oddsFixture := []byte(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/odds/1","displayName":"Win Probability","value":"0.61","type":"win-probability"}]}`)
	ticketsFixture := []byte(`{"count":0,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[]}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host + "/v2/sports/cricket"
		competitionPath := "/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474"

		switch r.URL.Path {
		case "/v2/sports/cricket/leagues/19138":
			_, _ = w.Write(rewriteFixtureBaseURL(leagueFixture, base))
		case competitionPath:
			_, _ = w.Write(rewriteFixtureBaseURL(competitionFixture, base))
		case competitionPath + "/officials":
			_, _ = w.Write(rewriteFixtureBaseURL(officialsFixture, base))
		case competitionPath + "/broadcasts":
			_, _ = w.Write(rewriteFixtureBaseURL(broadcastsFixture, base))
		case competitionPath + "/tickets":
			_, _ = w.Write(rewriteFixtureBaseURL(ticketsFixture, base))
		case competitionPath + "/odds":
			_, _ = w.Write(rewriteFixtureBaseURL(oddsFixture, base))
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

	service, err := NewCompetitionService(CompetitionServiceConfig{Client: client, Resolver: resolver})
	if err != nil {
		t.Fatalf("NewCompetitionService error: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return service
}

func TestLiveCompetitionMetadataRoutes(t *testing.T) {
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
		{name: "competition", ref: "/leagues/19138/events/1529474/competitions/1529474", keys: []string{"officials", "broadcasts", "tickets", "odds"}},
		{name: "officials", ref: "/leagues/11132/events/1527944/competitions/1527944/officials", keys: []string{"items", "count"}},
		{name: "broadcasts", ref: "/leagues/11132/events/1527944/competitions/1527944/broadcasts", keys: []string{"items", "count"}},
		{name: "tickets", ref: "/leagues/11132/events/1527944/competitions/1527944/tickets", keys: []string{"items", "count"}},
		{name: "odds", ref: "/leagues/11132/events/1527944/competitions/1527944/odds", keys: []string{"items", "count"}},
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

			payload, err := decodePayloadMap(resolved.Body)
			if err != nil {
				t.Fatalf("decode payload for %s: %v", tc.name, err)
			}
			requireAnyKey(t, payload, tc.keys...)
		})
	}
}
