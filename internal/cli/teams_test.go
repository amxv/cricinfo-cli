package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
)

type fakeTeamService struct {
	showResult       cricinfo.NormalizedResult
	rosterResult     cricinfo.NormalizedResult
	scoresResult     cricinfo.NormalizedResult
	leadersResult    cricinfo.NormalizedResult
	statisticsResult cricinfo.NormalizedResult
	recordsResult    cricinfo.NormalizedResult

	showQueries    []string
	rosterQueries  []string
	leadersQueries []string
	rosterOpts     []cricinfo.TeamLookupOptions
	leadersOpts    []cricinfo.TeamLookupOptions
}

func (f *fakeTeamService) Close() error { return nil }

func (f *fakeTeamService) Show(_ context.Context, query string, _ cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error) {
	f.showQueries = append(f.showQueries, query)
	return f.showResult, nil
}

func (f *fakeTeamService) Roster(_ context.Context, query string, opts cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error) {
	f.rosterQueries = append(f.rosterQueries, query)
	f.rosterOpts = append(f.rosterOpts, opts)
	return f.rosterResult, nil
}

func (f *fakeTeamService) Scores(context.Context, string, cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.scoresResult, nil
}

func (f *fakeTeamService) Leaders(_ context.Context, query string, opts cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error) {
	f.leadersQueries = append(f.leadersQueries, query)
	f.leadersOpts = append(f.leadersOpts, opts)
	return f.leadersResult, nil
}

func (f *fakeTeamService) Statistics(context.Context, string, cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.statisticsResult, nil
}

func (f *fakeTeamService) Records(context.Context, string, cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.recordsResult, nil
}

func TestTeamsCommandsSupportIDAndAliasInputs(t *testing.T) {
	leaders := cricinfo.TeamLeaders{
		TeamID:  "789643",
		MatchID: "1529474",
		Categories: []cricinfo.TeamLeaderCategory{
			{
				Name:        "runs",
				DisplayName: "Runs",
				Leaders: []cricinfo.TeamLeaderEntry{
					{AthleteID: "1108510", AthleteName: "Mohammad Ishaq", DisplayValue: "107", Balls: "141", Fours: "11", Sixes: "1"},
				},
			},
			{
				Name:        "wickets",
				DisplayName: "Wickets",
				Leaders: []cricinfo.TeamLeaderEntry{
					{AthleteID: "1076674", AthleteName: "Amanullah Safi", DisplayValue: "7", Overs: "19.0", Runs: "81", EconomyRate: "4.26"},
				},
			},
		},
	}

	service := &fakeTeamService{
		showResult: cricinfo.NewDataResult(cricinfo.EntityTeam, cricinfo.Team{
			ID:        "789643",
			Name:      "Boost Region",
			ShortName: "BOOST",
		}),
		rosterResult: cricinfo.NewListResult(cricinfo.EntityTeamRoster, []any{
			cricinfo.TeamRosterEntry{PlayerID: "1361257", PlayerRef: "http://core.espnuk.org/v2/sports/cricket/athletes/1361257", DisplayName: "Fazal Haq"},
		}),
		leadersResult: cricinfo.NewDataResult(cricinfo.EntityTeamLeaders, leaders),
	}

	originalFactory := newTeamService
	newTeamService = func() (teamCommandService, error) { return service, nil }
	defer func() {
		newTeamService = originalFactory
	}()

	var idOut bytes.Buffer
	var idErr bytes.Buffer
	if err := Run([]string{"teams", "show", "789643", "--format", "json"}, &idOut, &idErr); err != nil {
		t.Fatalf("Run teams show id error: %v", err)
	}

	payload := decodeCLIJSONMap(t, idOut.Bytes())
	if payload["kind"] != string(cricinfo.EntityTeam) {
		t.Fatalf("expected kind %q, got %#v", cricinfo.EntityTeam, payload["kind"])
	}

	var aliasOut bytes.Buffer
	var aliasErr bytes.Buffer
	if err := Run([]string{"teams", "show", "Boost", "Region", "--format", "json"}, &aliasOut, &aliasErr); err != nil {
		t.Fatalf("Run teams show alias error: %v", err)
	}

	if len(service.showQueries) != 2 {
		t.Fatalf("expected 2 show queries, got %d", len(service.showQueries))
	}
	if service.showQueries[0] != "789643" {
		t.Fatalf("expected first show query to be team ID, got %q", service.showQueries[0])
	}
	if service.showQueries[1] != "Boost Region" {
		t.Fatalf("expected second show query to be alias, got %q", service.showQueries[1])
	}

	var rosterIDOut bytes.Buffer
	var rosterIDErr bytes.Buffer
	if err := Run([]string{"teams", "roster", "789643", "--match", "1529474", "--format", "json"}, &rosterIDOut, &rosterIDErr); err != nil {
		t.Fatalf("Run teams roster id error: %v", err)
	}

	var rosterAliasOut bytes.Buffer
	var rosterAliasErr bytes.Buffer
	if err := Run([]string{"teams", "roster", "Boost", "Region", "--match", "3rd Match", "--format", "json"}, &rosterAliasOut, &rosterAliasErr); err != nil {
		t.Fatalf("Run teams roster alias error: %v", err)
	}

	if len(service.rosterQueries) != 2 {
		t.Fatalf("expected 2 roster queries, got %d", len(service.rosterQueries))
	}
	if service.rosterQueries[0] != "789643" {
		t.Fatalf("expected roster id query, got %q", service.rosterQueries[0])
	}
	if service.rosterQueries[1] != "Boost Region" {
		t.Fatalf("expected roster alias query, got %q", service.rosterQueries[1])
	}
	if service.rosterOpts[0].MatchQuery != "1529474" || service.rosterOpts[1].MatchQuery != "3rd Match" {
		t.Fatalf("expected roster match opts to preserve caller input, got %+v", service.rosterOpts)
	}

	var leadersOut bytes.Buffer
	var leadersErr bytes.Buffer
	if err := Run([]string{"teams", "leaders", "Boost", "Region", "--match", "3rd Match", "--format", "text"}, &leadersOut, &leadersErr); err != nil {
		t.Fatalf("Run teams leaders text error: %v", err)
	}
	leadersText := leadersOut.String()
	if !strings.Contains(leadersText, "Batting Leaders") || !strings.Contains(leadersText, "Bowling Leaders") {
		t.Fatalf("expected batting and bowling sections in leaders text output, got %q", leadersText)
	}
	if !strings.Contains(leadersText, "Mohammad Ishaq") || !strings.Contains(leadersText, "Amanullah Safi") {
		t.Fatalf("expected leader names in text output, got %q", leadersText)
	}
}

func TestTeamsMatchScopedCommandsRequireMatchFlag(t *testing.T) {
	service := &fakeTeamService{}

	originalFactory := newTeamService
	newTeamService = func() (teamCommandService, error) { return service, nil }
	defer func() {
		newTeamService = originalFactory
	}()

	var out bytes.Buffer
	var errBuf bytes.Buffer
	err := Run([]string{"teams", "scores", "789643"}, &out, &errBuf)
	if err == nil {
		t.Fatalf("expected error when --match is missing")
	}
	if !strings.Contains(err.Error(), "--match is required") {
		t.Fatalf("expected --match required message, got %v", err)
	}
}

func decodeCLIJSONMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode CLI JSON: %v", err)
	}
	return payload
}
