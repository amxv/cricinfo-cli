package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
)

type fakeMatchService struct {
	listResult   cricinfo.NormalizedResult
	liveResult   cricinfo.NormalizedResult
	showResult   cricinfo.NormalizedResult
	statusResult cricinfo.NormalizedResult
}

func (f *fakeMatchService) Close() error { return nil }

func (f *fakeMatchService) List(context.Context, cricinfo.MatchListOptions) (cricinfo.NormalizedResult, error) {
	return f.listResult, nil
}

func (f *fakeMatchService) Live(context.Context, cricinfo.MatchListOptions) (cricinfo.NormalizedResult, error) {
	return f.liveResult, nil
}

func (f *fakeMatchService) Show(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.showResult, nil
}

func (f *fakeMatchService) Status(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.statusResult, nil
}

func TestMatchesCommandsRenderTextAndJSON(t *testing.T) {
	match := cricinfo.Match{
		ID:            "1529474",
		LeagueID:      "19138",
		EventID:       "1529474",
		CompetitionID: "1529474",
		Description:   "3rd Match",
		MatchState:    "Boost lead by 20 runs",
		Date:          "2026-04-09T05:30Z",
		VenueSummary:  "Aino Maina, Kandahar, Afghanistan",
		ScoreSummary:  "BOOST 319 & 69/2 (19 ov) | SGH 368",
		Teams: []cricinfo.Team{
			{ID: "789643", Name: "Boost Region", ShortName: "BOOST", ScoreSummary: "319 & 69/2 (19 ov)"},
			{ID: "789647", Name: "Speen Ghar Region", ShortName: "SGH", ScoreSummary: "368"},
		},
		StatusRef:  "http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/status",
		DetailsRef: "http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/details",
	}

	service := &fakeMatchService{
		listResult:   cricinfo.NewListResult(cricinfo.EntityMatch, []any{match}),
		liveResult:   cricinfo.NewListResult(cricinfo.EntityMatch, []any{match}),
		showResult:   cricinfo.NewDataResult(cricinfo.EntityMatch, match),
		statusResult: cricinfo.NewDataResult(cricinfo.EntityMatch, match),
	}

	originalFactory := newMatchService
	newMatchService = func() (matchCommandService, error) { return service, nil }
	defer func() {
		newMatchService = originalFactory
	}()

	var textOut bytes.Buffer
	var textErr bytes.Buffer
	if err := Run([]string{"matches", "list", "--format", "text"}, &textOut, &textErr); err != nil {
		t.Fatalf("Run matches list --format text error: %v", err)
	}
	text := textOut.String()
	if !strings.Contains(text, "Matches (1)") {
		t.Fatalf("expected text list header, got %q", text)
	}
	if !strings.Contains(text, "BOOST") || !strings.Contains(text, "SGH") {
		t.Fatalf("expected team names in text output, got %q", text)
	}
	if !strings.Contains(text, "Boost lead by 20 runs") {
		t.Fatalf("expected match state in text output, got %q", text)
	}

	var jsonOut bytes.Buffer
	var jsonErr bytes.Buffer
	if err := Run([]string{"matches", "show", "1529474", "--format", "json"}, &jsonOut, &jsonErr); err != nil {
		t.Fatalf("Run matches show --format json error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(jsonOut.Bytes(), &payload); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if payload["kind"] != string(cricinfo.EntityMatch) {
		t.Fatalf("expected kind %q, got %#v", cricinfo.EntityMatch, payload["kind"])
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data in json output")
	}
	if data["id"] != "1529474" {
		t.Fatalf("expected id 1529474 in json output, got %#v", data["id"])
	}
	if data["matchState"] != "Boost lead by 20 runs" {
		t.Fatalf("expected matchState in json output, got %#v", data["matchState"])
	}
	if data["scoreSummary"] == nil {
		t.Fatalf("expected scoreSummary in json output")
	}
}

func TestMatchesHelpIncludesDrillDownHints(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer

	if err := Run([]string{"matches", "status", "--help"}, &out, &errBuf); err != nil {
		t.Fatalf("Run matches status --help error: %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "matches scorecard") {
		t.Fatalf("expected scorecard drill-down hint in help output, got %q", help)
	}
	if !strings.Contains(help, "matches innings") {
		t.Fatalf("expected innings drill-down hint in help output, got %q", help)
	}
}
