package cricinfo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMatchServicePhase7DetailsAndPlaysRenderDeliveryEvents(t *testing.T) {
	t.Parallel()

	service := newPhase7TestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	detailsResult, err := service.Details(ctx, "3rd Match", MatchLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Details error: %v", err)
	}
	if detailsResult.Kind != EntityDeliveryEvent {
		t.Fatalf("expected details kind %q, got %q", EntityDeliveryEvent, detailsResult.Kind)
	}
	if detailsResult.Status == ResultStatusError {
		t.Fatalf("expected non-error details result, got %+v", detailsResult)
	}
	if !strings.Contains(detailsResult.RequestedRef, "/details") {
		t.Fatalf("expected details requested ref to contain /details, got %q", detailsResult.RequestedRef)
	}
	if len(detailsResult.Items) == 0 {
		t.Fatalf("expected detail items")
	}
	firstDetail, ok := detailsResult.Items[0].(DeliveryEvent)
	if !ok {
		t.Fatalf("expected first details item to be DeliveryEvent, got %T", detailsResult.Items[0])
	}
	if firstDetail.PlayType == nil {
		t.Fatalf("expected playType in detail delivery event")
	}
	if firstDetail.Dismissal == nil {
		t.Fatalf("expected dismissal in detail delivery event")
	}

	playsResult, err := service.Plays(ctx, "3rd Match", MatchLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Plays error: %v", err)
	}
	if playsResult.Kind != EntityDeliveryEvent {
		t.Fatalf("expected plays kind %q, got %q", EntityDeliveryEvent, playsResult.Kind)
	}
	if !strings.Contains(playsResult.RequestedRef, "/plays") {
		t.Fatalf("expected plays requested ref to contain /plays, got %q", playsResult.RequestedRef)
	}
	if len(playsResult.Items) == 0 {
		t.Fatalf("expected play items")
	}
}

func TestMatchServicePhase7ScorecardAndSituation(t *testing.T) {
	t.Parallel()

	service := newPhase7TestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	scorecardResult, err := service.Scorecard(ctx, "3rd Match", MatchLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Scorecard error: %v", err)
	}
	if scorecardResult.Kind != EntityMatchScorecard {
		t.Fatalf("expected scorecard kind %q, got %q", EntityMatchScorecard, scorecardResult.Kind)
	}
	if scorecardResult.Status == ResultStatusError {
		t.Fatalf("expected non-error scorecard result, got %+v", scorecardResult)
	}
	scorecard, ok := scorecardResult.Data.(*MatchScorecard)
	if !ok {
		t.Fatalf("expected scorecard data type *MatchScorecard, got %T", scorecardResult.Data)
	}
	if len(scorecard.BattingCards) == 0 {
		t.Fatalf("expected batting cards in scorecard result")
	}
	if len(scorecard.BowlingCards) == 0 {
		t.Fatalf("expected bowling cards in scorecard result")
	}
	if len(scorecard.PartnershipCards) == 0 {
		t.Fatalf("expected partnerships cards in scorecard result")
	}

	situationResult, err := service.Situation(ctx, "3rd Match", MatchLookupOptions{LeagueID: "19138"})
	if err != nil {
		t.Fatalf("Situation error: %v", err)
	}
	if situationResult.Kind != EntityMatchSituation {
		t.Fatalf("expected situation kind %q, got %q", EntityMatchSituation, situationResult.Kind)
	}
	if situationResult.Status != ResultStatusEmpty {
		t.Fatalf("expected sparse situation to return empty status, got %q", situationResult.Status)
	}
	if strings.TrimSpace(situationResult.Message) == "" {
		t.Fatalf("expected sparse situation message to be populated")
	}
}

func newPhase7TestService(t *testing.T) *MatchService {
	t.Helper()

	playsFixture := mustReadFixtureBytes(t, "details-plays/plays.json")
	detailFixture := mustReadFixtureBytes(t, "details-plays/detail-110.json")
	scorecardFixture := mustReadFixtureBytes(t, "matches-competitions/matchcards-1527966.json")
	situationFixture := mustReadFixtureBytes(t, "matches-competitions/situation-1529474.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host + "/v2/sports/cricket"
		competitionPath := "/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474"

		switch r.URL.Path {
		case competitionPath:
			competition := fmt.Sprintf(`{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474","id":"1529474","description":"3rd Match","shortDescription":"3rd Match","date":"2026-04-09T05:30Z","details":{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/details"},"plays":null,"matchcards":{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/matchcards"},"situation":{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/situation"},"competitors":[]}`,
				base, base, base, base)
			_, _ = w.Write([]byte(competition))
		case competitionPath + "/details":
			_, _ = w.Write(rewriteFixtureBaseURL(playsFixture, base))
		case competitionPath + "/plays":
			_, _ = w.Write(rewriteFixtureBaseURL(playsFixture, base))
		case competitionPath + "/details/110":
			_, _ = w.Write(rewriteFixtureBaseURL(detailFixture, base))
		case competitionPath + "/matchcards":
			_, _ = w.Write(rewriteFixtureBaseURL(scorecardFixture, base))
		case competitionPath + "/situation":
			_, _ = w.Write(rewriteFixtureBaseURL(situationFixture, base))
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
		Aliases:   []string{"3rd Match", "match 1529474"},
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

	service, err := NewMatchService(MatchServiceConfig{Client: client, Resolver: resolver})
	if err != nil {
		t.Fatalf("NewMatchService error: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return service
}

func mustReadFixtureBytes(t *testing.T, fixturePath string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "fixtures", fixturePath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q: %v", path, err)
	}
	return data
}

func rewriteFixtureBaseURL(data []byte, baseURL string) []byte {
	updated := strings.ReplaceAll(string(data), "http://core.espnuk.org/v2/sports/cricket", baseURL)
	return []byte(updated)
}
