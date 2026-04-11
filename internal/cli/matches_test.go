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
	listResult      cricinfo.NormalizedResult
	liveResult      cricinfo.NormalizedResult
	lineupResult    cricinfo.NormalizedResult
	showResult      cricinfo.NormalizedResult
	statusResult    cricinfo.NormalizedResult
	scorecardResult cricinfo.NormalizedResult
	detailsResult   cricinfo.NormalizedResult
	playsResult     cricinfo.NormalizedResult
	situationResult cricinfo.NormalizedResult
	phasesResult    cricinfo.NormalizedResult
	inningsResult   cricinfo.NormalizedResult
	partnerships    cricinfo.NormalizedResult
	fowResult       cricinfo.NormalizedResult
	deliveries      cricinfo.NormalizedResult

	inningsQueries      []string
	inningsOpts         []cricinfo.MatchInningsOptions
	partnershipsQueries []string
	partnershipsOpts    []cricinfo.MatchInningsOptions
	fowQueries          []string
	fowOpts             []cricinfo.MatchInningsOptions
	deliveriesQueries   []string
	deliveriesOpts      []cricinfo.MatchInningsOptions
}

func (f *fakeMatchService) Close() error { return nil }

func (f *fakeMatchService) List(context.Context, cricinfo.MatchListOptions) (cricinfo.NormalizedResult, error) {
	return f.listResult, nil
}

func (f *fakeMatchService) Live(context.Context, cricinfo.MatchListOptions) (cricinfo.NormalizedResult, error) {
	return f.liveResult, nil
}

func (f *fakeMatchService) Lineups(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.lineupResult, nil
}

func (f *fakeMatchService) Show(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.showResult, nil
}

func (f *fakeMatchService) Status(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.statusResult, nil
}

func (f *fakeMatchService) Scorecard(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.scorecardResult, nil
}

func (f *fakeMatchService) Details(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.detailsResult, nil
}

func (f *fakeMatchService) Plays(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.playsResult, nil
}

func (f *fakeMatchService) Situation(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.situationResult, nil
}

func (f *fakeMatchService) Phases(context.Context, string, cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error) {
	return f.phasesResult, nil
}

func (f *fakeMatchService) Innings(_ context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error) {
	f.inningsQueries = append(f.inningsQueries, query)
	f.inningsOpts = append(f.inningsOpts, opts)
	return f.inningsResult, nil
}

func (f *fakeMatchService) Partnerships(_ context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error) {
	f.partnershipsQueries = append(f.partnershipsQueries, query)
	f.partnershipsOpts = append(f.partnershipsOpts, opts)
	return f.partnerships, nil
}

func (f *fakeMatchService) FallOfWicket(_ context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error) {
	f.fowQueries = append(f.fowQueries, query)
	f.fowOpts = append(f.fowOpts, opts)
	return f.fowResult, nil
}

func (f *fakeMatchService) Deliveries(_ context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error) {
	f.deliveriesQueries = append(f.deliveriesQueries, query)
	f.deliveriesOpts = append(f.deliveriesOpts, opts)
	return f.deliveries, nil
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

	scorecard := cricinfo.MatchScorecard{
		MatchID:      "1529474",
		BattingCards: []cricinfo.BattingCard{{InningsNumber: 3, TeamName: "Boost", Players: []cricinfo.BattingCardEntry{{PlayerName: "Numan Shah", Runs: "52"}}}},
		BowlingCards: []cricinfo.BowlingCard{{InningsNumber: 3, TeamName: "Speen-Ghar", Players: []cricinfo.BowlingCardEntry{{PlayerName: "Hayatullah Noori", Wickets: "1"}}}},
		PartnershipCards: []cricinfo.PartnershipCard{
			{InningsNumber: 3, TeamName: "Boost", Players: []cricinfo.PartnershipCardEntry{{PartnershipWicketName: "1st", PartnershipRuns: "50"}}},
		},
	}

	x := 12.5
	y := 27.25
	delivery := cricinfo.DeliveryEvent{
		ID:           "110",
		ShortText:    "Amanullah to Fazal Haq Shaheen, 1 run",
		BatsmanRef:   "http://core.espnuk.org/v2/sports/cricket/leagues/19138/athletes/1361257",
		BowlerRef:    "http://core.espnuk.org/v2/sports/cricket/leagues/19138/athletes/976585",
		ScoreValue:   1,
		Dismissal:    map[string]any{"dismissal": false, "type": ""},
		PlayType:     map[string]any{"id": "1", "description": "run"},
		BBBTimestamp: 0,
		XCoordinate:  &x,
		YCoordinate:  &y,
	}

	situation := cricinfo.MatchSituation{
		Ref:     "http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/situation",
		OddsRef: "http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/situation/odds",
		Data:    map[string]any{"session": "Day 2"},
	}

	service := &fakeMatchService{
		listResult:      cricinfo.NewListResult(cricinfo.EntityMatch, []any{match}),
		liveResult:      cricinfo.NewListResult(cricinfo.EntityMatch, []any{match}),
		lineupResult:    cricinfo.NewListResult(cricinfo.EntityTeamRoster, []any{cricinfo.TeamRosterEntry{DisplayName: "Player One", TeamName: "BOOST"}}),
		showResult:      cricinfo.NewDataResult(cricinfo.EntityMatch, match),
		statusResult:    cricinfo.NewDataResult(cricinfo.EntityMatch, match),
		scorecardResult: cricinfo.NewDataResult(cricinfo.EntityMatchScorecard, scorecard),
		detailsResult:   cricinfo.NewListResult(cricinfo.EntityDeliveryEvent, []any{delivery}),
		playsResult:     cricinfo.NewListResult(cricinfo.EntityDeliveryEvent, []any{delivery}),
		situationResult: cricinfo.NewDataResult(cricinfo.EntityMatchSituation, situation),
		phasesResult: cricinfo.NewDataResult(cricinfo.EntityMatchPhases, cricinfo.MatchPhases{
			MatchID: "1529474",
			Innings: []cricinfo.MatchPhaseInning{
				{TeamName: "BOOST", InningsNumber: 1, Period: 3, Score: "69/2 (19 ov)"},
			},
		}),
		inningsResult: cricinfo.NewListResult(cricinfo.EntityInnings, []any{
			cricinfo.Innings{
				TeamName:      "BOOST",
				InningsNumber: 1,
				Period:        3,
				Score:         "69/2 (19 ov)",
				OverTimeline: []cricinfo.InningsOver{
					{Number: 12, Runs: 10, WicketCount: 1},
				},
				WicketTimeline: []cricinfo.InningsWicket{
					{Number: 1, FOW: "60/1", Over: "11.2", DetailRef: "http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/details/52545007"},
				},
			},
		}),
		partnerships: cricinfo.NewListResult(cricinfo.EntityPartnership, []any{
			cricinfo.Partnership{WicketName: "1st", Runs: 60, Overs: 11.2, InningsID: "1", Period: "1"},
		}),
		fowResult: cricinfo.NewListResult(cricinfo.EntityFallOfWicket, []any{
			cricinfo.FallOfWicket{WicketNumber: 1, Runs: 60, WicketOver: 11.2, InningsID: "1", Period: "1"},
		}),
		deliveries: cricinfo.NewDataResult(cricinfo.EntityInnings, cricinfo.Innings{
			TeamName:      "BOOST",
			InningsNumber: 1,
			Period:        1,
			OverTimeline: []cricinfo.InningsOver{
				{Number: 12, Runs: 10, WicketCount: 1},
			},
			WicketTimeline: []cricinfo.InningsWicket{
				{Number: 1, FOW: "60/1", Over: "11.2", DetailRef: "http://core.espnuk.org/v2/sports/cricket/leagues/19138/events/1529474/competitions/1529474/details/52545007"},
			},
		}),
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

	var scorecardOut bytes.Buffer
	var scorecardErr bytes.Buffer
	if err := Run([]string{"matches", "scorecard", "1529474", "--format", "text"}, &scorecardOut, &scorecardErr); err != nil {
		t.Fatalf("Run matches scorecard --format text error: %v", err)
	}
	scorecardText := scorecardOut.String()
	if !strings.Contains(scorecardText, "Batting") || !strings.Contains(scorecardText, "Bowling") || !strings.Contains(scorecardText, "Partnerships") {
		t.Fatalf("expected scorecard sections in text output, got %q", scorecardText)
	}

	var detailsOut bytes.Buffer
	var detailsErr bytes.Buffer
	if err := Run([]string{"matches", "details", "1529474", "--format", "json"}, &detailsOut, &detailsErr); err != nil {
		t.Fatalf("Run matches details --format json error: %v", err)
	}

	var detailsPayload map[string]any
	if err := json.Unmarshal(detailsOut.Bytes(), &detailsPayload); err != nil {
		t.Fatalf("decode details JSON output: %v", err)
	}
	items, ok := detailsPayload["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected items in details json output")
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first details item object")
	}
	if _, ok := first["dismissal"]; !ok {
		t.Fatalf("expected dismissal field in details json output")
	}
	if _, ok := first["playType"]; !ok {
		t.Fatalf("expected playType field in details json output")
	}
	if _, ok := first["bbbTimestamp"]; !ok {
		t.Fatalf("expected bbbTimestamp field in details json output")
	}
	if _, ok := first["xCoordinate"]; !ok {
		t.Fatalf("expected xCoordinate field in details json output")
	}
	if _, ok := first["yCoordinate"]; !ok {
		t.Fatalf("expected yCoordinate field in details json output")
	}

	var playsOut bytes.Buffer
	var playsErr bytes.Buffer
	if err := Run([]string{"matches", "plays", "1529474", "--format", "text"}, &playsOut, &playsErr); err != nil {
		t.Fatalf("Run matches plays --format text error: %v", err)
	}
	if !strings.Contains(playsOut.String(), "Amanullah to Fazal Haq Shaheen, 1 run") {
		t.Fatalf("expected plays text output to include normalized short text, got %q", playsOut.String())
	}

	var lineupOut bytes.Buffer
	var lineupErr bytes.Buffer
	if err := Run([]string{"matches", "lineup", "1529474", "--format", "text"}, &lineupOut, &lineupErr); err != nil {
		t.Fatalf("Run matches lineup --format text error: %v", err)
	}
	if !strings.Contains(lineupOut.String(), "Player One") {
		t.Fatalf("expected lineup text to include roster names, got %q", lineupOut.String())
	}

	var situationOut bytes.Buffer
	var situationErr bytes.Buffer
	if err := Run([]string{"matches", "situation", "1529474", "--format", "json"}, &situationOut, &situationErr); err != nil {
		t.Fatalf("Run matches situation --format json error: %v", err)
	}
	var situationPayload map[string]any
	if err := json.Unmarshal(situationOut.Bytes(), &situationPayload); err != nil {
		t.Fatalf("decode situation JSON output: %v", err)
	}
	if situationPayload["kind"] != string(cricinfo.EntityMatchSituation) {
		t.Fatalf("expected kind %q in situation output, got %#v", cricinfo.EntityMatchSituation, situationPayload["kind"])
	}

	var inningsOut bytes.Buffer
	var inningsErr bytes.Buffer
	if err := Run([]string{"matches", "innings", "1529474", "--team", "BOOST", "--format", "text"}, &inningsOut, &inningsErr); err != nil {
		t.Fatalf("Run matches innings --format text error: %v", err)
	}
	inningsText := inningsOut.String()
	if !strings.Contains(strings.ToLower(inningsText), "innings 1/3") {
		t.Fatalf("expected innings heading in text output, got %q", inningsText)
	}

	var partnershipsOut bytes.Buffer
	var partnershipsErr bytes.Buffer
	if err := Run([]string{"matches", "partnerships", "1529474", "--team", "BOOST", "--innings", "1", "--period", "1", "--format", "json"}, &partnershipsOut, &partnershipsErr); err != nil {
		t.Fatalf("Run matches partnerships --format json error: %v", err)
	}
	var partnershipsPayload map[string]any
	if err := json.Unmarshal(partnershipsOut.Bytes(), &partnershipsPayload); err != nil {
		t.Fatalf("decode partnerships JSON output: %v", err)
	}
	if partnershipsPayload["kind"] != string(cricinfo.EntityPartnership) {
		t.Fatalf("expected kind %q in partnerships output, got %#v", cricinfo.EntityPartnership, partnershipsPayload["kind"])
	}

	var fowOut bytes.Buffer
	var fowErr bytes.Buffer
	if err := Run([]string{"matches", "fow", "1529474", "--team", "BOOST", "--innings", "1", "--period", "1", "--format", "text"}, &fowOut, &fowErr); err != nil {
		t.Fatalf("Run matches fow --format text error: %v", err)
	}
	if !strings.Contains(fowOut.String(), "Wicket 1") && !strings.Contains(fowOut.String(), "wicket 1") {
		t.Fatalf("expected wicket summary in fow output, got %q", fowOut.String())
	}

	var deliveriesOut bytes.Buffer
	var deliveriesErr bytes.Buffer
	if err := Run([]string{"matches", "deliveries", "1529474", "--team", "BOOST", "--innings", "1", "--period", "1", "--format", "text"}, &deliveriesOut, &deliveriesErr); err != nil {
		t.Fatalf("Run matches deliveries --format text error: %v", err)
	}
	deliveriesText := deliveriesOut.String()
	if !strings.Contains(deliveriesText, "Over Timeline") || !strings.Contains(deliveriesText, "Wicket Timeline") {
		t.Fatalf("expected deliveries timeline sections in text output, got %q", deliveriesText)
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

func TestMatchesInningsDepthSelectorsAndRequiredFlags(t *testing.T) {
	service := &fakeMatchService{
		inningsResult: cricinfo.NewListResult(cricinfo.EntityInnings, []any{
			cricinfo.Innings{InningsNumber: 1, Period: 2, TeamName: "BOOST"},
		}),
		partnerships: cricinfo.NewListResult(cricinfo.EntityPartnership, []any{
			cricinfo.Partnership{InningsID: "1", Period: "2", Runs: 32, Overs: 5.5},
		}),
		fowResult: cricinfo.NewListResult(cricinfo.EntityFallOfWicket, []any{
			cricinfo.FallOfWicket{InningsID: "1", Period: "2", WicketNumber: 1},
		}),
		deliveries: cricinfo.NewDataResult(cricinfo.EntityInnings, cricinfo.Innings{InningsNumber: 1, Period: 2}),
	}

	originalFactory := newMatchService
	newMatchService = func() (matchCommandService, error) { return service, nil }
	defer func() {
		newMatchService = originalFactory
	}()

	var out bytes.Buffer
	var errBuf bytes.Buffer
	if err := Run([]string{"matches", "partnerships", "1529474", "--team", "Boost Region", "--innings", "1", "--period", "2", "--format", "json"}, &out, &errBuf); err != nil {
		t.Fatalf("Run matches partnerships with selectors error: %v", err)
	}
	if len(service.partnershipsOpts) != 1 {
		t.Fatalf("expected partnerships opts capture, got %d", len(service.partnershipsOpts))
	}
	if service.partnershipsOpts[0].TeamQuery != "Boost Region" || service.partnershipsOpts[0].Innings != 1 || service.partnershipsOpts[0].Period != 2 {
		t.Fatalf("unexpected partnerships options: %+v", service.partnershipsOpts[0])
	}

	var missingPeriodOut bytes.Buffer
	var missingPeriodErr bytes.Buffer
	err := Run([]string{"matches", "partnerships", "1529474", "--team", "Boost Region", "--innings", "1"}, &missingPeriodOut, &missingPeriodErr)
	if err == nil || !strings.Contains(err.Error(), "--period is required") {
		t.Fatalf("expected --period required error, got %v", err)
	}

	var missingTeamOut bytes.Buffer
	var missingTeamErr bytes.Buffer
	err = Run([]string{"matches", "fow", "1529474", "--innings", "1", "--period", "2"}, &missingTeamOut, &missingTeamErr)
	if err == nil || !strings.Contains(err.Error(), "--team is required") {
		t.Fatalf("expected --team required error, got %v", err)
	}

	var missingInningsOut bytes.Buffer
	var missingInningsErr bytes.Buffer
	err = Run([]string{"matches", "deliveries", "1529474", "--team", "Boost Region", "--period", "2"}, &missingInningsOut, &missingInningsErr)
	if err == nil || !strings.Contains(err.Error(), "--innings is required") {
		t.Fatalf("expected --innings required error, got %v", err)
	}
}
