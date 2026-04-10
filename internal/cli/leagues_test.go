package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
)

type fakeLeagueService struct {
	listResult      cricinfo.NormalizedResult
	showResult      cricinfo.NormalizedResult
	eventsResult    cricinfo.NormalizedResult
	calendarResult  cricinfo.NormalizedResult
	athletesResult  cricinfo.NormalizedResult
	standingsResult cricinfo.NormalizedResult
	seasonsResult   cricinfo.NormalizedResult
	seasonShow      cricinfo.NormalizedResult
	seasonTypes     cricinfo.NormalizedResult
	seasonGroups    cricinfo.NormalizedResult

	showQueries        []string
	standingsQueries   []string
	seasonShowQueries  []string
	seasonTypeQueries  []string
	seasonGroupQueries []string
	seasonShowOpts     []cricinfo.SeasonLookupOptions
	seasonTypeOpts     []cricinfo.SeasonLookupOptions
	seasonGroupOpts    []cricinfo.SeasonLookupOptions
}

func (f *fakeLeagueService) Close() error { return nil }

func (f *fakeLeagueService) List(context.Context, cricinfo.LeagueListOptions) (cricinfo.NormalizedResult, error) {
	return f.listResult, nil
}

func (f *fakeLeagueService) Show(_ context.Context, leagueQuery string) (cricinfo.NormalizedResult, error) {
	f.showQueries = append(f.showQueries, leagueQuery)
	return f.showResult, nil
}

func (f *fakeLeagueService) Events(context.Context, string, cricinfo.LeagueListOptions) (cricinfo.NormalizedResult, error) {
	return f.eventsResult, nil
}

func (f *fakeLeagueService) Calendar(context.Context, string) (cricinfo.NormalizedResult, error) {
	return f.calendarResult, nil
}

func (f *fakeLeagueService) Athletes(context.Context, string, cricinfo.LeagueListOptions) (cricinfo.NormalizedResult, error) {
	return f.athletesResult, nil
}

func (f *fakeLeagueService) Standings(_ context.Context, leagueQuery string) (cricinfo.NormalizedResult, error) {
	f.standingsQueries = append(f.standingsQueries, leagueQuery)
	return f.standingsResult, nil
}

func (f *fakeLeagueService) Seasons(context.Context, string) (cricinfo.NormalizedResult, error) {
	return f.seasonsResult, nil
}

func (f *fakeLeagueService) SeasonShow(_ context.Context, leagueQuery string, opts cricinfo.SeasonLookupOptions) (cricinfo.NormalizedResult, error) {
	f.seasonShowQueries = append(f.seasonShowQueries, leagueQuery)
	f.seasonShowOpts = append(f.seasonShowOpts, opts)
	return f.seasonShow, nil
}

func (f *fakeLeagueService) SeasonTypes(_ context.Context, leagueQuery string, opts cricinfo.SeasonLookupOptions) (cricinfo.NormalizedResult, error) {
	f.seasonTypeQueries = append(f.seasonTypeQueries, leagueQuery)
	f.seasonTypeOpts = append(f.seasonTypeOpts, opts)
	return f.seasonTypes, nil
}

func (f *fakeLeagueService) SeasonGroups(_ context.Context, leagueQuery string, opts cricinfo.SeasonLookupOptions) (cricinfo.NormalizedResult, error) {
	f.seasonGroupQueries = append(f.seasonGroupQueries, leagueQuery)
	f.seasonGroupOpts = append(f.seasonGroupOpts, opts)
	return f.seasonGroups, nil
}

func TestLeaguesAndStandingsCommandsResolveLeagueName(t *testing.T) {
	service := &fakeLeagueService{
		showResult: cricinfo.NewDataResult(cricinfo.EntityLeague, cricinfo.League{
			ID:   "19138",
			Name: "Mirwais Nika Provincial 3-Day",
			Slug: "19138",
		}),
		standingsResult: cricinfo.NewListResult(cricinfo.EntityStandingsGroup, []any{
			cricinfo.StandingsGroup{ID: "1", SeasonID: "2026", GroupID: "1"},
		}),
	}

	originalFactory := newLeagueService
	newLeagueService = func() (leagueCommandService, error) { return service, nil }
	defer func() {
		newLeagueService = originalFactory
	}()

	var showOut bytes.Buffer
	var showErr bytes.Buffer
	if err := Run([]string{"leagues", "show", "Mirwais", "Nika", "--format", "json"}, &showOut, &showErr); err != nil {
		t.Fatalf("Run leagues show error: %v", err)
	}
	if len(service.showQueries) != 1 || service.showQueries[0] != "Mirwais Nika" {
		t.Fatalf("expected joined league alias query, got %+v", service.showQueries)
	}

	var showPayload map[string]any
	if err := json.Unmarshal(showOut.Bytes(), &showPayload); err != nil {
		t.Fatalf("decode leagues show payload: %v", err)
	}
	if showPayload["kind"] != string(cricinfo.EntityLeague) {
		t.Fatalf("expected kind %q, got %#v", cricinfo.EntityLeague, showPayload["kind"])
	}

	var standingsOut bytes.Buffer
	var standingsErr bytes.Buffer
	if err := Run([]string{"standings", "show", "Mirwais", "Nika", "--format", "json"}, &standingsOut, &standingsErr); err != nil {
		t.Fatalf("Run standings show error: %v", err)
	}
	if len(service.standingsQueries) != 1 || service.standingsQueries[0] != "Mirwais Nika" {
		t.Fatalf("expected joined standings league query, got %+v", service.standingsQueries)
	}
}

func TestSeasonsCommandsRequireAndPropagateSelectors(t *testing.T) {
	service := &fakeLeagueService{
		seasonShow: cricinfo.NewDataResult(cricinfo.EntitySeason, cricinfo.Season{
			ID:       "2025",
			Year:     2025,
			LeagueID: "19138",
		}),
		seasonTypes: cricinfo.NewListResult(cricinfo.EntitySeasonType, []any{
			cricinfo.SeasonType{ID: "1", SeasonID: "2025", LeagueID: "19138", Name: "Regular Season"},
		}),
		seasonGroups: cricinfo.NewListResult(cricinfo.EntitySeasonGroup, []any{
			cricinfo.SeasonGroup{ID: "1", SeasonID: "2025", TypeID: "1", LeagueID: "1479935"},
		}),
	}

	originalFactory := newLeagueService
	newLeagueService = func() (leagueCommandService, error) { return service, nil }
	defer func() {
		newLeagueService = originalFactory
	}()

	var showOut bytes.Buffer
	var showErr bytes.Buffer
	if err := Run([]string{"seasons", "show", "Mirwais", "Nika", "--season", "2025", "--format", "json"}, &showOut, &showErr); err != nil {
		t.Fatalf("Run seasons show error: %v", err)
	}
	if len(service.seasonShowOpts) != 1 || service.seasonShowOpts[0].SeasonQuery != "2025" {
		t.Fatalf("expected season selector to be propagated, got %+v", service.seasonShowOpts)
	}

	var typesOut bytes.Buffer
	var typesErr bytes.Buffer
	if err := Run([]string{"seasons", "types", "Mirwais", "Nika", "--season", "2025", "--format", "json"}, &typesOut, &typesErr); err != nil {
		t.Fatalf("Run seasons types error: %v", err)
	}
	if len(service.seasonTypeOpts) != 1 || service.seasonTypeOpts[0].SeasonQuery != "2025" {
		t.Fatalf("expected season selector for types, got %+v", service.seasonTypeOpts)
	}

	var groupsOut bytes.Buffer
	var groupsErr bytes.Buffer
	if err := Run([]string{"seasons", "groups", "Mirwais", "Nika", "--season", "2025", "--type", "1", "--format", "json"}, &groupsOut, &groupsErr); err != nil {
		t.Fatalf("Run seasons groups error: %v", err)
	}
	if len(service.seasonGroupOpts) != 1 {
		t.Fatalf("expected one season groups invocation, got %+v", service.seasonGroupOpts)
	}
	if service.seasonGroupOpts[0].SeasonQuery != "2025" || service.seasonGroupOpts[0].TypeQuery != "1" {
		t.Fatalf("expected season/type selectors to be propagated, got %+v", service.seasonGroupOpts[0])
	}

	var missingSeasonOut bytes.Buffer
	var missingSeasonErr bytes.Buffer
	err := Run([]string{"seasons", "show", "Mirwais", "Nika"}, &missingSeasonOut, &missingSeasonErr)
	if err == nil || !strings.Contains(err.Error(), "--season is required") {
		t.Fatalf("expected --season required error, got %v", err)
	}

	var missingTypeOut bytes.Buffer
	var missingTypeErr bytes.Buffer
	err = Run([]string{"seasons", "groups", "Mirwais", "Nika", "--season", "2025"}, &missingTypeOut, &missingTypeErr)
	if err == nil || !strings.Contains(err.Error(), "--type is required") {
		t.Fatalf("expected --type required error, got %v", err)
	}
}
