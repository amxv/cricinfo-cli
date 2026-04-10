package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
)

type fakeAnalysisService struct {
	dismissalsResult  cricinfo.NormalizedResult
	bowlingResult     cricinfo.NormalizedResult
	battingResult     cricinfo.NormalizedResult
	partnershipResult cricinfo.NormalizedResult

	dismissOpts []cricinfo.AnalysisDismissalOptions
	bowlingOpts []cricinfo.AnalysisMetricOptions
	battingOpts []cricinfo.AnalysisMetricOptions
	partnerOpts []cricinfo.AnalysisMetricOptions
}

func (f *fakeAnalysisService) Close() error { return nil }

func (f *fakeAnalysisService) Dismissals(_ context.Context, opts cricinfo.AnalysisDismissalOptions) (cricinfo.NormalizedResult, error) {
	f.dismissOpts = append(f.dismissOpts, opts)
	return f.dismissalsResult, nil
}

func (f *fakeAnalysisService) Bowling(_ context.Context, opts cricinfo.AnalysisMetricOptions) (cricinfo.NormalizedResult, error) {
	f.bowlingOpts = append(f.bowlingOpts, opts)
	return f.bowlingResult, nil
}

func (f *fakeAnalysisService) Batting(_ context.Context, opts cricinfo.AnalysisMetricOptions) (cricinfo.NormalizedResult, error) {
	f.battingOpts = append(f.battingOpts, opts)
	return f.battingResult, nil
}

func (f *fakeAnalysisService) Partnerships(_ context.Context, opts cricinfo.AnalysisMetricOptions) (cricinfo.NormalizedResult, error) {
	f.partnerOpts = append(f.partnerOpts, opts)
	return f.partnershipResult, nil
}

func TestAnalysisCommandsCallServiceWithScopedOptions(t *testing.T) {
	service := &fakeAnalysisService{
		dismissalsResult:  cricinfo.NewDataResult(cricinfo.EntityAnalysisDismiss, cricinfo.AnalysisView{Command: "dismissals"}),
		bowlingResult:     cricinfo.NewDataResult(cricinfo.EntityAnalysisBowl, cricinfo.AnalysisView{Command: "bowling", Metric: "economy"}),
		battingResult:     cricinfo.NewDataResult(cricinfo.EntityAnalysisBat, cricinfo.AnalysisView{Command: "batting", Metric: "strike-rate"}),
		partnershipResult: cricinfo.NewDataResult(cricinfo.EntityAnalysisPart, cricinfo.AnalysisView{Command: "partnerships"}),
	}

	originalFactory := newAnalysisService
	newAnalysisService = func() (analysisCommandService, error) { return service, nil }
	defer func() {
		newAnalysisService = originalFactory
	}()

	var dismissOut bytes.Buffer
	var dismissErr bytes.Buffer
	if err := Run([]string{
		"analysis", "dismissals",
		"--league", "19138",
		"--seasons", "2024-2025",
		"--group-by", "dismissal-type,team",
		"--team", "BOOST",
		"--dismissal-type", "caught",
		"--innings", "1",
		"--top", "12",
		"--format", "json",
	}, &dismissOut, &dismissErr); err != nil {
		t.Fatalf("Run analysis dismissals error: %v", err)
	}
	dismissPayload := decodeCLIJSONMap(t, dismissOut.Bytes())
	if dismissPayload["kind"] != string(cricinfo.EntityAnalysisDismiss) {
		t.Fatalf("expected kind %q, got %#v", cricinfo.EntityAnalysisDismiss, dismissPayload["kind"])
	}
	if len(service.dismissOpts) != 1 {
		t.Fatalf("expected one dismissals call, got %d", len(service.dismissOpts))
	}
	if service.dismissOpts[0].LeagueQuery != "19138" || service.dismissOpts[0].Seasons != "2024-2025" {
		t.Fatalf("unexpected dismissals opts: %+v", service.dismissOpts[0])
	}

	var bowlingOut bytes.Buffer
	var bowlingErr bytes.Buffer
	if err := Run([]string{
		"analysis", "bowling",
		"--scope", "season:2025",
		"--league", "19138",
		"--metric", "economy",
		"--group-by", "player,team",
		"--player", "1361257",
		"--top", "5",
		"--format", "json",
	}, &bowlingOut, &bowlingErr); err != nil {
		t.Fatalf("Run analysis bowling error: %v", err)
	}
	if len(service.bowlingOpts) != 1 {
		t.Fatalf("expected one bowling call, got %d", len(service.bowlingOpts))
	}
	if service.bowlingOpts[0].Metric != "economy" || service.bowlingOpts[0].Scope != "season:2025" {
		t.Fatalf("unexpected bowling opts: %+v", service.bowlingOpts[0])
	}

	var battingOut bytes.Buffer
	var battingErr bytes.Buffer
	if err := Run([]string{
		"analysis", "batting",
		"--scope", "match:1529474",
		"--metric", "strike-rate",
		"--group-by", "player",
		"--top", "7",
		"--format", "json",
	}, &battingOut, &battingErr); err != nil {
		t.Fatalf("Run analysis batting error: %v", err)
	}
	if len(service.battingOpts) != 1 {
		t.Fatalf("expected one batting call, got %d", len(service.battingOpts))
	}
	if service.battingOpts[0].Metric != "strike-rate" || service.battingOpts[0].Scope != "match:1529474" {
		t.Fatalf("unexpected batting opts: %+v", service.battingOpts[0])
	}

	var partnershipsOut bytes.Buffer
	var partnershipsErr bytes.Buffer
	if err := Run([]string{
		"analysis", "partnerships",
		"--scope", "season:2025",
		"--league", "19138",
		"--group-by", "innings,team",
		"--innings", "1",
		"--top", "9",
		"--format", "json",
	}, &partnershipsOut, &partnershipsErr); err != nil {
		t.Fatalf("Run analysis partnerships error: %v", err)
	}
	if len(service.partnerOpts) != 1 {
		t.Fatalf("expected one partnerships call, got %d", len(service.partnerOpts))
	}
	if service.partnerOpts[0].Scope != "season:2025" || service.partnerOpts[0].Innings != 1 {
		t.Fatalf("unexpected partnerships opts: %+v", service.partnerOpts[0])
	}
}

func TestAnalysisCommandsValidateRequiredFlags(t *testing.T) {
	service := &fakeAnalysisService{}

	originalFactory := newAnalysisService
	newAnalysisService = func() (analysisCommandService, error) { return service, nil }
	defer func() {
		newAnalysisService = originalFactory
	}()

	assertErrContains := func(args []string, want string) {
		t.Helper()
		var out bytes.Buffer
		var errBuf bytes.Buffer
		err := Run(args, &out, &errBuf)
		if err == nil {
			t.Fatalf("expected error for %v", args)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in error for %v, got %v", want, args, err)
		}
	}

	assertErrContains([]string{"analysis", "dismissals", "--seasons", "2025"}, "--league is required")
	assertErrContains([]string{"analysis", "dismissals", "--league", "19138"}, "--seasons is required")
	assertErrContains([]string{"analysis", "bowling", "--metric", "economy"}, "--scope is required")
	assertErrContains([]string{"analysis", "bowling", "--scope", "match:1529474"}, "--metric is required")
	assertErrContains([]string{"analysis", "batting", "--scope", "match:1529474"}, "--metric is required")
	assertErrContains([]string{"analysis", "partnerships"}, "--scope is required")
}
