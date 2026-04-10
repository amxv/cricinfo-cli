package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
)

type fakeCompetitionService struct {
	showResult       cricinfo.NormalizedResult
	officialsResult  cricinfo.NormalizedResult
	broadcastsResult cricinfo.NormalizedResult
	ticketsResult    cricinfo.NormalizedResult
	oddsResult       cricinfo.NormalizedResult
	metadataResult   cricinfo.NormalizedResult

	showQueries       []string
	officialsQueries  []string
	broadcastsQueries []string
	metadataQueries   []string
	showOpts          []cricinfo.CompetitionLookupOptions
	officialsOpts     []cricinfo.CompetitionLookupOptions
	broadcastsOpts    []cricinfo.CompetitionLookupOptions
	metadataOpts      []cricinfo.CompetitionLookupOptions
}

func (f *fakeCompetitionService) Close() error { return nil }

func (f *fakeCompetitionService) Show(_ context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error) {
	f.showQueries = append(f.showQueries, query)
	f.showOpts = append(f.showOpts, opts)
	return f.showResult, nil
}

func (f *fakeCompetitionService) Officials(_ context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error) {
	f.officialsQueries = append(f.officialsQueries, query)
	f.officialsOpts = append(f.officialsOpts, opts)
	return f.officialsResult, nil
}

func (f *fakeCompetitionService) Broadcasts(_ context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error) {
	f.broadcastsQueries = append(f.broadcastsQueries, query)
	f.broadcastsOpts = append(f.broadcastsOpts, opts)
	return f.broadcastsResult, nil
}

func (f *fakeCompetitionService) Tickets(context.Context, string, cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.ticketsResult, nil
}

func (f *fakeCompetitionService) Odds(context.Context, string, cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.oddsResult, nil
}

func (f *fakeCompetitionService) Metadata(_ context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error) {
	f.metadataQueries = append(f.metadataQueries, query)
	f.metadataOpts = append(f.metadataOpts, opts)
	return f.metadataResult, nil
}

func TestCompetitionsCommandsRenderAndPreserveLookupInputs(t *testing.T) {
	service := &fakeCompetitionService{
		showResult: cricinfo.NewDataResult(cricinfo.EntityCompetition, cricinfo.Competition{
			ID:               "1529474",
			CompetitionID:    "1529474",
			LeagueID:         "19138",
			EventID:          "1529474",
			Description:      "3rd Match",
			ShortDescription: "3rd Match",
			OfficialsRef:     "http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/officials",
		}),
		officialsResult: cricinfo.NewListResult(cricinfo.EntityCompOfficial, []any{
			cricinfo.CompetitionMetadataEntry{DisplayName: "Anil Chaugai", Role: "umpire", Order: 1},
		}),
		broadcastsResult: cricinfo.NewListResult(cricinfo.EntityCompBroadcast, []any{}),
		ticketsResult:    cricinfo.NewListResult(cricinfo.EntityCompTicket, []any{}),
		oddsResult: cricinfo.NewListResult(cricinfo.EntityCompOdds, []any{
			cricinfo.CompetitionMetadataEntry{Name: "Win Probability", Value: "0.61"},
		}),
		metadataResult: cricinfo.NewDataResult(cricinfo.EntityCompMetadata, cricinfo.CompetitionMetadataSummary{
			Competition: cricinfo.Competition{ID: "1529474"},
			Officials:   []cricinfo.CompetitionMetadataEntry{{DisplayName: "Anil Chaugai"}},
			Broadcasts:  []cricinfo.CompetitionMetadataEntry{},
			Tickets:     []cricinfo.CompetitionMetadataEntry{},
			Odds:        []cricinfo.CompetitionMetadataEntry{{Name: "Win Probability", Value: "0.61"}},
		}),
	}

	originalFactory := newCompetitionService
	newCompetitionService = func() (competitionCommandService, error) { return service, nil }
	defer func() {
		newCompetitionService = originalFactory
	}()

	var showOut bytes.Buffer
	var showErr bytes.Buffer
	if err := Run([]string{"competitions", "show", "3rd", "Match", "--league", "19138", "--format", "json"}, &showOut, &showErr); err != nil {
		t.Fatalf("Run competitions show error: %v", err)
	}

	var showPayload map[string]any
	if err := json.Unmarshal(showOut.Bytes(), &showPayload); err != nil {
		t.Fatalf("decode competitions show payload: %v", err)
	}
	if showPayload["kind"] != string(cricinfo.EntityCompetition) {
		t.Fatalf("expected kind %q, got %#v", cricinfo.EntityCompetition, showPayload["kind"])
	}

	var officialsOut bytes.Buffer
	var officialsErr bytes.Buffer
	if err := Run([]string{"competitions", "officials", "1529474", "--league", "19138", "--format", "text"}, &officialsOut, &officialsErr); err != nil {
		t.Fatalf("Run competitions officials error: %v", err)
	}
	if !strings.Contains(officialsOut.String(), "Anil Chaugai") {
		t.Fatalf("expected officials text output to include official name, got %q", officialsOut.String())
	}

	var broadcastsOut bytes.Buffer
	var broadcastsErr bytes.Buffer
	if err := Run([]string{"competitions", "broadcasts", "1529474", "--league", "19138", "--format", "text"}, &broadcastsOut, &broadcastsErr); err != nil {
		t.Fatalf("Run competitions broadcasts error: %v", err)
	}
	if !strings.Contains(strings.ToLower(broadcastsOut.String()), "no competition broadcasts found") {
		t.Fatalf("expected clean zero-result message for broadcasts, got %q", broadcastsOut.String())
	}

	var metadataOut bytes.Buffer
	var metadataErr bytes.Buffer
	if err := Run([]string{"competitions", "metadata", "3rd", "Match", "--league", "19138", "--format", "json"}, &metadataOut, &metadataErr); err != nil {
		t.Fatalf("Run competitions metadata error: %v", err)
	}
	var metadataPayload map[string]any
	if err := json.Unmarshal(metadataOut.Bytes(), &metadataPayload); err != nil {
		t.Fatalf("decode competitions metadata payload: %v", err)
	}
	if metadataPayload["kind"] != string(cricinfo.EntityCompMetadata) {
		t.Fatalf("expected kind %q, got %#v", cricinfo.EntityCompMetadata, metadataPayload["kind"])
	}

	if len(service.showQueries) != 1 || service.showQueries[0] != "3rd Match" {
		t.Fatalf("expected joined show query, got %+v", service.showQueries)
	}
	if len(service.officialsQueries) != 1 || service.officialsQueries[0] != "1529474" {
		t.Fatalf("expected officials query 1529474, got %+v", service.officialsQueries)
	}
	if len(service.metadataQueries) != 1 || service.metadataQueries[0] != "3rd Match" {
		t.Fatalf("expected joined metadata query, got %+v", service.metadataQueries)
	}
	if len(service.showOpts) != 1 || service.showOpts[0].LeagueID != "19138" {
		t.Fatalf("expected show league option to propagate, got %+v", service.showOpts)
	}
	if len(service.officialsOpts) != 1 || service.officialsOpts[0].LeagueID != "19138" {
		t.Fatalf("expected officials league option to propagate, got %+v", service.officialsOpts)
	}
	if len(service.broadcastsOpts) != 1 || service.broadcastsOpts[0].LeagueID != "19138" {
		t.Fatalf("expected broadcasts league option to propagate, got %+v", service.broadcastsOpts)
	}
	if len(service.metadataOpts) != 1 || service.metadataOpts[0].LeagueID != "19138" {
		t.Fatalf("expected metadata league option to propagate, got %+v", service.metadataOpts)
	}
}
