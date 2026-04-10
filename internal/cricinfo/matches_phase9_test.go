package cricinfo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMatchServicePhase9InningsPartnershipsFOWAndDeliveries(t *testing.T) {
	t.Parallel()

	service := newPhase9MatchTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	inningsResult, err := service.Innings(ctx, "1529474", MatchInningsOptions{LeagueID: "19138", TeamQuery: "789643"})
	if err != nil {
		t.Fatalf("Innings error: %v", err)
	}
	if inningsResult.Kind != EntityInnings {
		t.Fatalf("expected innings kind %q, got %q", EntityInnings, inningsResult.Kind)
	}
	if len(inningsResult.Items) == 0 {
		t.Fatalf("expected innings items")
	}
	firstInnings, ok := inningsResult.Items[0].(Innings)
	if !ok {
		t.Fatalf("expected innings item type Innings, got %T", inningsResult.Items[0])
	}
	if firstInnings.TeamID != "789643" {
		t.Fatalf("expected innings team id 789643, got %q", firstInnings.TeamID)
	}
	if len(firstInnings.OverTimeline) == 0 {
		t.Fatalf("expected over timeline from period statistics")
	}
	if len(firstInnings.WicketTimeline) == 0 {
		t.Fatalf("expected wicket timeline from period statistics")
	}
	if strings.TrimSpace(firstInnings.WicketTimeline[0].DetailRef) == "" {
		t.Fatalf("expected wicket timeline detail ref link")
	}

	partnershipsResult, err := service.Partnerships(ctx, "1529474", MatchInningsOptions{LeagueID: "19138", TeamQuery: "789643", Innings: 1, Period: 2})
	if err != nil {
		t.Fatalf("Partnerships error: %v", err)
	}
	if partnershipsResult.Kind != EntityPartnership {
		t.Fatalf("expected partnerships kind %q, got %q", EntityPartnership, partnershipsResult.Kind)
	}
	if len(partnershipsResult.Items) == 0 {
		t.Fatalf("expected partnership items")
	}
	firstPartnership, ok := partnershipsResult.Items[0].(Partnership)
	if !ok {
		t.Fatalf("expected partnership item type Partnership, got %T", partnershipsResult.Items[0])
	}
	if firstPartnership.WicketNumber == 0 || firstPartnership.Runs == 0 {
		t.Fatalf("expected detailed partnership payload, got %+v", firstPartnership)
	}

	fowResult, err := service.FallOfWicket(ctx, "1529474", MatchInningsOptions{LeagueID: "19138", TeamQuery: "789643", Innings: 1, Period: 2})
	if err != nil {
		t.Fatalf("FallOfWicket error: %v", err)
	}
	if fowResult.Kind != EntityFallOfWicket {
		t.Fatalf("expected fow kind %q, got %q", EntityFallOfWicket, fowResult.Kind)
	}
	if len(fowResult.Items) == 0 {
		t.Fatalf("expected fow items")
	}
	firstFOW, ok := fowResult.Items[0].(FallOfWicket)
	if !ok {
		t.Fatalf("expected fow item type FallOfWicket, got %T", fowResult.Items[0])
	}
	if firstFOW.WicketNumber == 0 || firstFOW.WicketOver == 0 {
		t.Fatalf("expected detailed fow payload, got %+v", firstFOW)
	}

	deliveriesResult, err := service.Deliveries(ctx, "1529474", MatchInningsOptions{LeagueID: "19138", TeamQuery: "789643", Innings: 1, Period: 2})
	if err != nil {
		t.Fatalf("Deliveries error: %v", err)
	}
	if deliveriesResult.Kind != EntityInnings {
		t.Fatalf("expected deliveries kind %q, got %q", EntityInnings, deliveriesResult.Kind)
	}
	deliveriesInnings, ok := deliveriesResult.Data.(Innings)
	if !ok {
		t.Fatalf("expected deliveries data Innings, got %T", deliveriesResult.Data)
	}
	if len(deliveriesInnings.OverTimeline) == 0 || len(deliveriesInnings.WicketTimeline) == 0 {
		t.Fatalf("expected deliveries timelines in innings payload, got %+v", deliveriesInnings)
	}
}

func TestMatchServicePhase9MissingInningsOrPeriodShowsAvailableHint(t *testing.T) {
	t.Parallel()

	service := newPhase9MatchTestService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	result, err := service.Partnerships(ctx, "1529474", MatchInningsOptions{LeagueID: "19138", TeamQuery: "789643", Innings: 9, Period: 9})
	if err != nil {
		t.Fatalf("Partnerships error: %v", err)
	}
	if result.Status != ResultStatusEmpty {
		t.Fatalf("expected empty result for missing innings/period, got status=%q", result.Status)
	}
	if !strings.Contains(result.Message, "available") || !strings.Contains(result.Message, "1/2") {
		t.Fatalf("expected available innings/period hint, got %q", result.Message)
	}
}

func newPhase9MatchTestService(t *testing.T) *MatchService {
	t.Helper()

	competitionFixture := mustReadFixtureBytes(t, "matches-competitions/competition.json")
	teamFixture := mustReadFixtureBytes(t, "team-competitor/team-789643.json")
	inningsFixture := mustReadFixtureBytes(t, "innings-fow-partnerships/innings-1-2.json")
	partnershipsFixture := mustReadFixtureBytes(t, "innings-fow-partnerships/partnerships.json")
	partnershipFixture := mustReadFixtureBytes(t, "innings-fow-partnerships/partnership-1.json")
	fowFixture := mustReadFixtureBytes(t, "innings-fow-partnerships/fow.json")
	fowItemFixture := mustReadFixtureBytes(t, "innings-fow-partnerships/fow-1.json")
	statisticsFixture := mustReadFixtureBytes(t, "team-competitor/statistics-789643.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host + "/v2/sports/cricket"
		competitionPath := "/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474"

		switch {
		case r.URL.Path == competitionPath:
			_, _ = w.Write(rewriteFixtureBaseURL(competitionFixture, base))
		case r.URL.Path == "/v2/sports/cricket/teams/789643":
			_, _ = w.Write(rewriteFixtureBaseURL(teamFixture, base))
		case r.URL.Path == "/v2/sports/cricket/teams/789647":
			_, _ = w.Write([]byte(`{"$ref":"` + base + `/teams/789647","id":"789647","displayName":"Speen Ghar Region","shortDisplayName":"SGH","abbreviation":"SGH"}`))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/linescores/1"):
			payload := fmt.Sprintf(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[%s]}`,
				string(rewriteFixtureBaseURL(inningsFixture, base)),
			)
			_, _ = w.Write([]byte(payload))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/linescores"):
			payload := fmt.Sprintf(`{"count":1,"pageIndex":1,"pageSize":25,"pageCount":1,"items":[{"$ref":"%s/leagues/19138/events/1529474/competitions/1529474/competitors/789643/linescores/1/2"}]}`,
				base,
			)
			_, _ = w.Write([]byte(payload))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/linescores/1/2"):
			_, _ = w.Write(rewriteFixtureBaseURL(inningsFixture, base))
		case strings.HasSuffix(r.URL.Path, "/competitors/789643/linescores/1/2/statistics/0"):
			_, _ = w.Write(rewriteFixtureBaseURL(statisticsFixture, base))
		case strings.HasSuffix(r.URL.Path, "/linescores/1/2/partnerships"):
			_, _ = w.Write(rewriteFixtureBaseURL(partnershipsFixture, base))
		case strings.Contains(r.URL.Path, "/linescores/1/2/partnerships/"):
			partnershipID := lastPathComponent(r.URL.Path)
			item := strings.ReplaceAll(string(rewriteFixtureBaseURL(partnershipFixture, base)), "/partnerships/1\"", "/partnerships/"+partnershipID+"\"")
			item = strings.ReplaceAll(item, `"wicketNumber": 1`, `"wicketNumber": `+partnershipID)
			_, _ = w.Write([]byte(item))
		case strings.HasSuffix(r.URL.Path, "/linescores/1/2/fow"):
			_, _ = w.Write(rewriteFixtureBaseURL(fowFixture, base))
		case strings.Contains(r.URL.Path, "/linescores/1/2/fow/"):
			wicketID := lastPathComponent(r.URL.Path)
			item := strings.ReplaceAll(string(rewriteFixtureBaseURL(fowItemFixture, base)), "/fow/1\"", "/fow/"+wicketID+"\"")
			item = strings.ReplaceAll(item, `"wicketNumber": 1`, `"wicketNumber": `+wicketID)
			_, _ = w.Write([]byte(item))
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

	service, err := NewMatchService(MatchServiceConfig{Client: client, Resolver: resolver})
	if err != nil {
		t.Fatalf("NewMatchService error: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return service
}

func lastPathComponent(path string) string {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return "1"
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	if _, err := strconv.Atoi(last); err != nil {
		return "1"
	}
	return last
}
