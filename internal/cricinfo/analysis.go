package cricinfo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	analysisScopeMatch   = "match"
	analysisScopeSeason  = "season"
	analysisScopeSeasons = "seasons"

	analysisMetricEconomy       = "economy"
	analysisMetricDots          = "dots"
	analysisMetricSixesConceded = "sixes-conceded"
	analysisMetricFours         = "fours"
	analysisMetricSixes         = "sixes"
	analysisMetricStrikeRate    = "strike-rate"
)

var (
	analysisGroupDismissAllowed = map[string]struct{}{
		"player":         {},
		"team":           {},
		"league":         {},
		"season":         {},
		"dismissal-type": {},
		"innings":        {},
	}
	analysisGroupBowlingAllowed = map[string]struct{}{
		"player": {},
		"team":   {},
		"league": {},
		"season": {},
	}
	analysisGroupBattingAllowed = map[string]struct{}{
		"player": {},
		"team":   {},
		"league": {},
		"season": {},
	}
	analysisGroupPartnershipAllowed = map[string]struct{}{
		"team":    {},
		"league":  {},
		"season":  {},
		"innings": {},
	}
)

// AnalysisServiceConfig configures analysis command behavior.
type AnalysisServiceConfig struct {
	Client    *Client
	Resolver  *Resolver
	Hydration *HistoricalHydrationService
}

// AnalysisDismissalOptions configures dismissal analysis execution.
type AnalysisDismissalOptions struct {
	LeagueQuery   string
	Seasons       string
	TypeQuery     string
	GroupQuery    string
	DateFrom      string
	DateTo        string
	MatchLimit    int
	GroupBy       string
	TeamQuery     string
	PlayerQuery   string
	DismissalType string
	Innings       int
	Period        int
	Top           int
}

// AnalysisMetricOptions configures bowling/batting/partnership analysis execution.
type AnalysisMetricOptions struct {
	Metric        string
	Scope         string
	LeagueQuery   string
	TypeQuery     string
	GroupQuery    string
	DateFrom      string
	DateTo        string
	MatchLimit    int
	GroupBy       string
	TeamQuery     string
	PlayerQuery   string
	DismissalType string
	Innings       int
	Period        int
	Top           int
}

// AnalysisService derives ranked cricket analysis over normalized hydrated data.
type AnalysisService struct {
	client        *Client
	resolver      *Resolver
	hydration     *HistoricalHydrationService
	ownsResolver  bool
	ownsHydration bool
}

// NewAnalysisService builds an analysis service using default client/resolver/hydration when omitted.
func NewAnalysisService(cfg AnalysisServiceConfig) (*AnalysisService, error) {
	client := cfg.Client
	if client == nil {
		var err error
		client, err = NewClient(Config{})
		if err != nil {
			return nil, err
		}
	}

	resolver := cfg.Resolver
	ownsResolver := false
	if resolver == nil {
		var err error
		resolver, err = NewResolver(ResolverConfig{Client: client})
		if err != nil {
			return nil, err
		}
		ownsResolver = true
	}

	hydration := cfg.Hydration
	ownsHydration := false
	if hydration == nil {
		var err error
		hydration, err = NewHistoricalHydrationService(HistoricalHydrationServiceConfig{
			Client:   client,
			Resolver: resolver,
		})
		if err != nil {
			if ownsResolver {
				_ = resolver.Close()
			}
			return nil, err
		}
		ownsHydration = true
	}

	return &AnalysisService{
		client:        client,
		resolver:      resolver,
		hydration:     hydration,
		ownsResolver:  ownsResolver,
		ownsHydration: ownsHydration,
	}, nil
}

// Close persists resolver state when owned by this service.
func (s *AnalysisService) Close() error {
	var errs []error
	if s.ownsHydration && s.hydration != nil {
		if err := s.hydration.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.ownsResolver && s.resolver != nil {
		if err := s.resolver.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// Dismissals ranks dismissal patterns across league+season scope.
func (s *AnalysisService) Dismissals(ctx context.Context, opts AnalysisDismissalOptions) (NormalizedResult, error) {
	leagueQuery := strings.TrimSpace(opts.LeagueQuery)
	if leagueQuery == "" {
		return NormalizedResult{
			Kind:    EntityAnalysisDismiss,
			Status:  ResultStatusEmpty,
			Message: "--league is required",
		}, nil
	}

	seasons, err := parseSeasonRange(opts.Seasons)
	if err != nil {
		return NormalizedResult{
			Kind:    EntityAnalysisDismiss,
			Status:  ResultStatusEmpty,
			Message: err.Error(),
		}, nil
	}

	groupBy, err := parseGroupBy(opts.GroupBy, []string{"dismissal-type"}, analysisGroupDismissAllowed)
	if err != nil {
		return NormalizedResult{Kind: EntityAnalysisDismiss, Status: ResultStatusEmpty, Message: err.Error()}, nil
	}
	filters := analysisFiltersFromDismissal(opts)
	top := limitOrDefault(opts.Top, 20)

	agg := map[string]*analysisAggregate{}
	warnings := make([]string, 0)
	combinedMatchIDs := make([]string, 0)
	combinedMetrics := HydrationMetrics{}
	leagueName := ""

	for _, seasonQuery := range seasons {
		session, scopeSummary, beginWarnings, passthrough := s.beginSeasonScope(ctx, analysisSeasonScopeRequest{
			LeagueQuery: leagueQuery,
			SeasonQuery: seasonQuery,
			TypeQuery:   opts.TypeQuery,
			GroupQuery:  opts.GroupQuery,
			DateFrom:    opts.DateFrom,
			DateTo:      opts.DateTo,
			MatchLimit:  opts.MatchLimit,
		}, EntityAnalysisDismiss)
		if passthrough != nil {
			return *passthrough, nil
		}
		warnings = append(warnings, beginWarnings...)

		scope := scopeSummary
		if strings.TrimSpace(leagueName) == "" {
			leagueName = strings.TrimSpace(scope.League.Name)
		}
		combinedMatchIDs = append(combinedMatchIDs, scope.MatchIDs...)
		metrics := session.Metrics()
		combinedMetrics.ResolveCacheHits += metrics.ResolveCacheHits
		combinedMetrics.ResolveCacheMisses += metrics.ResolveCacheMisses
		combinedMetrics.DomainCacheHits += metrics.DomainCacheHits
		combinedMetrics.DomainCacheMisses += metrics.DomainCacheMisses

		seasonID := seasonIdentifier(scope, seasonQuery)
		matches := session.ScopedMatches()
		for _, match := range matches {
			playerByID, playerWarnings := s.playerNameMapForMatch(ctx, session, match.ID)
			warnings = append(warnings, playerWarnings...)

			innings, inningsWarnings, hydrateErr := session.HydrateInnings(ctx, match.ID)
			if hydrateErr != nil {
				if statusErr := analysisTransportResult(EntityAnalysisDismiss, match.ID, hydrateErr); statusErr != nil {
					return *statusErr, nil
				}
				warnings = append(warnings, fmt.Sprintf("match %s innings: %v", match.ID, hydrateErr))
				continue
			}
			warnings = append(warnings, inningsWarnings...)

			for _, inn := range innings {
				for _, wicket := range inn.WicketTimeline {
					dismissalType := firstNonEmptyString(wicket.FOWType, wicket.DismissalCard)
					if dismissalType == "" {
						continue
					}
					playerID := strings.TrimSpace(refIDs(wicket.AthleteRef)["athleteId"])
					row := analysisSourceRow{
						MatchID:       strings.TrimSpace(match.ID),
						LeagueID:      strings.TrimSpace(match.LeagueID),
						SeasonID:      seasonID,
						TeamID:        strings.TrimSpace(inn.TeamID),
						TeamName:      strings.TrimSpace(inn.TeamName),
						PlayerID:      playerID,
						PlayerName:    strings.TrimSpace(playerByID[playerID]),
						DismissalType: dismissalType,
						InningsNumber: inn.InningsNumber,
						Period:        inn.Period,
						CountValue:    1,
					}
					if row.PlayerName == "" {
						row.PlayerName = row.PlayerID
					}

					if !filters.matches(row) {
						continue
					}
					key, dims := buildAnalysisGroup(row, groupBy)
					entry := agg[key]
					if entry == nil {
						entry = &analysisAggregate{row: dims, matchIDs: map[string]struct{}{}}
						agg[key] = entry
					}
					entry.count += row.CountValue
					entry.matchIDs[row.MatchID] = struct{}{}
				}
			}
		}
	}

	rows := make([]AnalysisRow, 0, len(agg))
	for key, entry := range agg {
		row := entry.row
		row.Key = key
		row.Metric = "dismissals"
		row.Value = float64(entry.count)
		row.Count = entry.count
		row.Matches = len(entry.matchIDs)
		rows = append(rows, row)
	}
	rows = rankAnalysisRows(rows, false)
	rows = trimAnalysisRows(rows, top)

	view := AnalysisView{
		Command: "dismissals",
		Metric:  "dismissals",
		Scope: AnalysisScope{
			Mode:            analysisScopeSeasons,
			LeagueID:        leagueQuery,
			LeagueName:      leagueName,
			Seasons:         seasons,
			MatchIDs:        dedupeStrings(combinedMatchIDs),
			MatchCount:      len(dedupeStrings(combinedMatchIDs)),
			DateFrom:        strings.TrimSpace(opts.DateFrom),
			DateTo:          strings.TrimSpace(opts.DateTo),
			TypeQuery:       strings.TrimSpace(opts.TypeQuery),
			GroupQuery:      strings.TrimSpace(opts.GroupQuery),
			HydrationMetric: combinedMetrics,
		},
		GroupBy: groupBy,
		Filters: AnalysisFilters{
			TeamQuery:     strings.TrimSpace(opts.TeamQuery),
			PlayerQuery:   strings.TrimSpace(opts.PlayerQuery),
			DismissalType: strings.TrimSpace(opts.DismissalType),
			Innings:       opts.Innings,
			Period:        opts.Period,
		},
		Rows: rows,
	}

	return analysisResult(EntityAnalysisDismiss, view, warnings), nil
}

// Bowling ranks bowling metrics over match or season scope.
func (s *AnalysisService) Bowling(ctx context.Context, opts AnalysisMetricOptions) (NormalizedResult, error) {
	metric, err := normalizeBowlingMetric(opts.Metric)
	if err != nil {
		return NormalizedResult{Kind: EntityAnalysisBowl, Status: ResultStatusEmpty, Message: err.Error()}, nil
	}
	groupBy, err := parseGroupBy(opts.GroupBy, []string{"player"}, analysisGroupBowlingAllowed)
	if err != nil {
		return NormalizedResult{Kind: EntityAnalysisBowl, Status: ResultStatusEmpty, Message: err.Error()}, nil
	}
	filters := analysisFiltersFromMetric(opts)
	top := limitOrDefault(opts.Top, 20)

	run, passthrough := s.resolveMetricScope(ctx, opts, EntityAnalysisBowl)
	if passthrough != nil {
		return *passthrough, nil
	}

	agg := map[string]*analysisAggregate{}
	warnings := append([]string{}, run.warnings...)
	for _, match := range run.session.ScopedMatches() {
		seasonID := seasonForMatch(match, run.seasonHint)
		players, playerWarnings, hydrateErr := run.session.HydratePlayerMatchSummaries(ctx, match.ID)
		if hydrateErr != nil {
			if statusErr := analysisTransportResult(EntityAnalysisBowl, match.ID, hydrateErr); statusErr != nil {
				return *statusErr, nil
			}
			warnings = append(warnings, fmt.Sprintf("match %s player summary: %v", match.ID, hydrateErr))
			continue
		}
		warnings = append(warnings, playerWarnings...)

		for _, player := range players {
			totals := extractBowlingTotals(player)
			playerName := analysisDisplayPlayerName(s.resolver, player.PlayerID, player.PlayerName)
			teamName := analysisDisplayTeamName(s.resolver, player.TeamID, player.TeamName)
			row := analysisSourceRow{
				MatchID:       strings.TrimSpace(player.MatchID),
				LeagueID:      strings.TrimSpace(player.LeagueID),
				SeasonID:      seasonID,
				TeamID:        strings.TrimSpace(player.TeamID),
				TeamName:      strings.TrimSpace(teamName),
				PlayerID:      strings.TrimSpace(player.PlayerID),
				PlayerName:    strings.TrimSpace(playerName),
				CountValue:    1,
				Dots:          totals.dots,
				SixesConceded: totals.sixesConceded,
				Balls:         totals.balls,
				RunsConceded:  totals.conceded,
				EconomySample: totals.economy,
			}
			if !filters.matches(row) {
				continue
			}
			key, dims := buildAnalysisGroup(row, groupBy)
			entry := agg[key]
			if entry == nil {
				entry = &analysisAggregate{row: dims, matchIDs: map[string]struct{}{}}
				agg[key] = entry
			}
			entry.matchIDs[row.MatchID] = struct{}{}
			entry.dots += row.Dots
			entry.sixesConceded += row.SixesConceded
			entry.balls += row.Balls
			entry.runsConceded += row.RunsConceded
			if row.EconomySample > 0 {
				entry.economyTotal += row.EconomySample
				entry.economyCount++
			}
		}
	}

	rows := make([]AnalysisRow, 0, len(agg))
	for key, entry := range agg {
		if entry == nil {
			continue
		}
		if !hasBowlingActivity(entry) {
			continue
		}
		row := entry.row
		row.Key = key
		row.Metric = metric
		row.Matches = len(entry.matchIDs)

		switch metric {
		case analysisMetricEconomy:
			row.Value = economyFromAggregate(entry)
			row.Extras = map[string]any{
				"runsConceded":  entry.runsConceded,
				"balls":         entry.balls,
				"dots":          entry.dots,
				"sixesConceded": entry.sixesConceded,
			}
		case analysisMetricDots:
			row.Value = float64(entry.dots)
			row.Count = entry.dots
		case analysisMetricSixesConceded:
			row.Value = float64(entry.sixesConceded)
			row.Count = entry.sixesConceded
		}
		rows = append(rows, row)
	}

	rows = rankAnalysisRows(rows, metric == analysisMetricEconomy)
	rows = trimAnalysisRows(rows, top)

	view := AnalysisView{
		Command: "bowling",
		Metric:  metric,
		Scope:   buildSingleScope(run, opts),
		GroupBy: groupBy,
		Filters: AnalysisFilters{TeamQuery: strings.TrimSpace(opts.TeamQuery), PlayerQuery: strings.TrimSpace(opts.PlayerQuery)},
		Rows:    rows,
	}
	return analysisResult(EntityAnalysisBowl, view, warnings), nil
}

// Batting ranks batting metrics over match or season scope.
func (s *AnalysisService) Batting(ctx context.Context, opts AnalysisMetricOptions) (NormalizedResult, error) {
	metric, err := normalizeBattingMetric(opts.Metric)
	if err != nil {
		return NormalizedResult{Kind: EntityAnalysisBat, Status: ResultStatusEmpty, Message: err.Error()}, nil
	}
	groupBy, err := parseGroupBy(opts.GroupBy, []string{"player"}, analysisGroupBattingAllowed)
	if err != nil {
		return NormalizedResult{Kind: EntityAnalysisBat, Status: ResultStatusEmpty, Message: err.Error()}, nil
	}
	filters := analysisFiltersFromMetric(opts)
	top := limitOrDefault(opts.Top, 20)

	run, passthrough := s.resolveMetricScope(ctx, opts, EntityAnalysisBat)
	if passthrough != nil {
		return *passthrough, nil
	}

	agg := map[string]*analysisAggregate{}
	warnings := append([]string{}, run.warnings...)
	for _, match := range run.session.ScopedMatches() {
		seasonID := seasonForMatch(match, run.seasonHint)
		players, playerWarnings, hydrateErr := run.session.HydratePlayerMatchSummaries(ctx, match.ID)
		if hydrateErr != nil {
			if statusErr := analysisTransportResult(EntityAnalysisBat, match.ID, hydrateErr); statusErr != nil {
				return *statusErr, nil
			}
			warnings = append(warnings, fmt.Sprintf("match %s player summary: %v", match.ID, hydrateErr))
			continue
		}
		warnings = append(warnings, playerWarnings...)

		for _, player := range players {
			totals := extractBattingTotals(player)
			playerName := analysisDisplayPlayerName(s.resolver, player.PlayerID, player.PlayerName)
			teamName := analysisDisplayTeamName(s.resolver, player.TeamID, player.TeamName)
			row := analysisSourceRow{
				MatchID:      strings.TrimSpace(player.MatchID),
				LeagueID:     strings.TrimSpace(player.LeagueID),
				SeasonID:     seasonID,
				TeamID:       strings.TrimSpace(player.TeamID),
				TeamName:     strings.TrimSpace(teamName),
				PlayerID:     strings.TrimSpace(player.PlayerID),
				PlayerName:   strings.TrimSpace(playerName),
				CountValue:   1,
				Fours:        totals.fours,
				BattingSixes: totals.sixes,
				RunsScored:   totals.runs,
				BallsFaced:   totals.balls,
				StrikeSample: totals.strikeRate,
			}
			if !filters.matches(row) {
				continue
			}
			key, dims := buildAnalysisGroup(row, groupBy)
			entry := agg[key]
			if entry == nil {
				entry = &analysisAggregate{row: dims, matchIDs: map[string]struct{}{}}
				agg[key] = entry
			}
			entry.matchIDs[row.MatchID] = struct{}{}
			entry.fours += row.Fours
			entry.battingSixes += row.BattingSixes
			entry.runsScored += row.RunsScored
			entry.ballsFaced += row.BallsFaced
			if row.StrikeSample > 0 {
				entry.strikeRateTotal += row.StrikeSample
				entry.strikeRateCount++
			}
		}
	}

	rows := make([]AnalysisRow, 0, len(agg))
	for key, entry := range agg {
		row := entry.row
		row.Key = key
		row.Metric = metric
		row.Matches = len(entry.matchIDs)

		switch metric {
		case analysisMetricFours:
			row.Value = float64(entry.fours)
			row.Count = entry.fours
		case analysisMetricSixes:
			row.Value = float64(entry.battingSixes)
			row.Count = entry.battingSixes
		case analysisMetricStrikeRate:
			row.Value = strikeRateFromAggregate(entry)
		}
		row.Extras = map[string]any{
			"runs":       entry.runsScored,
			"ballsFaced": entry.ballsFaced,
			"fours":      entry.fours,
			"sixes":      entry.battingSixes,
		}
		rows = append(rows, row)
	}

	rows = rankAnalysisRows(rows, false)
	rows = trimAnalysisRows(rows, top)

	view := AnalysisView{
		Command: "batting",
		Metric:  metric,
		Scope:   buildSingleScope(run, opts),
		GroupBy: groupBy,
		Filters: AnalysisFilters{TeamQuery: strings.TrimSpace(opts.TeamQuery), PlayerQuery: strings.TrimSpace(opts.PlayerQuery)},
		Rows:    rows,
	}
	return analysisResult(EntityAnalysisBat, view, warnings), nil
}

// Partnerships ranks partnerships over match or season scope.
func (s *AnalysisService) Partnerships(ctx context.Context, opts AnalysisMetricOptions) (NormalizedResult, error) {
	groupBy, err := parseGroupBy(opts.GroupBy, []string{"innings"}, analysisGroupPartnershipAllowed)
	if err != nil {
		return NormalizedResult{Kind: EntityAnalysisPart, Status: ResultStatusEmpty, Message: err.Error()}, nil
	}
	filters := analysisFiltersFromMetric(opts)
	top := limitOrDefault(opts.Top, 20)

	run, passthrough := s.resolveMetricScope(ctx, opts, EntityAnalysisPart)
	if passthrough != nil {
		return *passthrough, nil
	}

	agg := map[string]*analysisAggregate{}
	warnings := append([]string{}, run.warnings...)
	for _, match := range run.session.ScopedMatches() {
		seasonID := seasonForMatch(match, run.seasonHint)
		partnerships, partnershipWarnings, hydrateErr := run.session.HydratePartnershipSummaries(ctx, match.ID)
		if hydrateErr != nil {
			if statusErr := analysisTransportResult(EntityAnalysisPart, match.ID, hydrateErr); statusErr != nil {
				return *statusErr, nil
			}
			warnings = append(warnings, fmt.Sprintf("match %s partnerships: %v", match.ID, hydrateErr))
			continue
		}
		warnings = append(warnings, partnershipWarnings...)

		for _, partnership := range partnerships {
			inningsNumber := parseInt(partnership.InningsID)
			period := parseInt(partnership.Period)
			row := analysisSourceRow{
				MatchID:       strings.TrimSpace(partnership.MatchID),
				LeagueID:      strings.TrimSpace(match.LeagueID),
				SeasonID:      seasonID,
				TeamID:        strings.TrimSpace(partnership.TeamID),
				TeamName:      strings.TrimSpace(partnership.TeamName),
				InningsNumber: inningsNumber,
				Period:        period,
				RunsScored:    partnership.Runs,
				CountValue:    1,
			}
			if !filters.matchesPartnership(row) {
				continue
			}
			key, dims := buildAnalysisGroup(row, groupBy)
			entry := agg[key]
			if entry == nil {
				entry = &analysisAggregate{row: dims, matchIDs: map[string]struct{}{}}
				agg[key] = entry
			}
			entry.matchIDs[row.MatchID] = struct{}{}
			entry.runsScored += row.RunsScored
			entry.count += row.CountValue
		}
	}

	rows := make([]AnalysisRow, 0, len(agg))
	for key, entry := range agg {
		row := entry.row
		row.Key = key
		row.Metric = "partnership-runs"
		row.Value = float64(entry.runsScored)
		row.Count = entry.count
		row.Matches = len(entry.matchIDs)
		rows = append(rows, row)
	}
	rows = rankAnalysisRows(rows, false)
	rows = trimAnalysisRows(rows, top)

	view := AnalysisView{
		Command: "partnerships",
		Metric:  "partnership-runs",
		Scope:   buildSingleScope(run, opts),
		GroupBy: groupBy,
		Filters: AnalysisFilters{TeamQuery: strings.TrimSpace(opts.TeamQuery), Innings: opts.Innings},
		Rows:    rows,
	}
	return analysisResult(EntityAnalysisPart, view, warnings), nil
}

type analysisSeasonScopeRequest struct {
	LeagueQuery string
	SeasonQuery string
	TypeQuery   string
	GroupQuery  string
	DateFrom    string
	DateTo      string
	MatchLimit  int
}

type analysisScopeRun struct {
	session    *HistoricalScopeSession
	scope      HistoricalScopeSummary
	mode       string
	seasonHint string
	warnings   []string
}

func (s *AnalysisService) beginSeasonScope(
	ctx context.Context,
	req analysisSeasonScopeRequest,
	kind EntityKind,
) (*HistoricalScopeSession, HistoricalScopeSummary, []string, *NormalizedResult) {
	session, err := s.hydration.BeginScope(ctx, HistoricalScopeOptions{
		LeagueQuery: strings.TrimSpace(req.LeagueQuery),
		SeasonQuery: strings.TrimSpace(req.SeasonQuery),
		TypeQuery:   strings.TrimSpace(req.TypeQuery),
		GroupQuery:  strings.TrimSpace(req.GroupQuery),
		DateFrom:    strings.TrimSpace(req.DateFrom),
		DateTo:      strings.TrimSpace(req.DateTo),
		MatchLimit:  req.MatchLimit,
	})
	if err != nil {
		if transport := analysisTransportResult(kind, req.LeagueQuery, err); transport != nil {
			return nil, HistoricalScopeSummary{}, nil, transport
		}
		result := NormalizedResult{Kind: kind, Status: ResultStatusError, Message: err.Error()}
		return nil, HistoricalScopeSummary{}, nil, &result
	}
	scope := session.Scope()
	return session, scope, scope.Warnings, nil
}

func (s *AnalysisService) resolveMetricScope(ctx context.Context, opts AnalysisMetricOptions, kind EntityKind) (*analysisScopeRun, *NormalizedResult) {
	scopeMode, scopeQuery, err := parseMetricScope(opts.Scope)
	if err != nil {
		result := NormalizedResult{Kind: kind, Status: ResultStatusEmpty, Message: err.Error()}
		return nil, &result
	}

	switch scopeMode {
	case analysisScopeSeason:
		if strings.TrimSpace(opts.LeagueQuery) == "" {
			result := NormalizedResult{Kind: kind, Status: ResultStatusEmpty, Message: "--league is required for season scope"}
			return nil, &result
		}
		session, scope, warnings, passthrough := s.beginSeasonScope(ctx, analysisSeasonScopeRequest{
			LeagueQuery: opts.LeagueQuery,
			SeasonQuery: scopeQuery,
			TypeQuery:   opts.TypeQuery,
			GroupQuery:  opts.GroupQuery,
			DateFrom:    opts.DateFrom,
			DateTo:      opts.DateTo,
			MatchLimit:  opts.MatchLimit,
		}, kind)
		if passthrough != nil {
			return nil, passthrough
		}
		return &analysisScopeRun{session: session, scope: scope, mode: analysisScopeSeason, seasonHint: seasonIdentifier(scope, scopeQuery), warnings: warnings}, nil
	case analysisScopeMatch:
		match, warnings, err := s.resolveMatchByQuery(ctx, scopeQuery, opts.LeagueQuery)
		if err != nil {
			if transport := analysisTransportResult(kind, scopeQuery, err); transport != nil {
				return nil, transport
			}
			result := NormalizedResult{Kind: kind, Status: ResultStatusError, Message: err.Error()}
			return nil, &result
		}

		session := newHistoricalScopeSession(s.client, s.resolver, HistoricalScopeOptions{
			LeagueQuery: strings.TrimSpace(nonEmpty(match.LeagueID, opts.LeagueQuery)),
		})
		session.matches = []Match{*match}
		session.league = League{ID: strings.TrimSpace(nonEmpty(match.LeagueID, opts.LeagueQuery))}
		session.warnings = compactWarnings(warnings)
		scope := session.Scope()
		return &analysisScopeRun{session: session, scope: scope, mode: analysisScopeMatch, seasonHint: seasonForMatch(*match, ""), warnings: warnings}, nil
	default:
		result := NormalizedResult{Kind: kind, Status: ResultStatusEmpty, Message: "unsupported scope"}
		return nil, &result
	}
}

func (s *AnalysisService) resolveMatchByQuery(ctx context.Context, query, leagueHint string) (*Match, []string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil, fmt.Errorf("match scope query is required")
	}

	searchResult, err := s.resolver.Search(ctx, EntityMatch, query, ResolveOptions{
		Limit:    5,
		LeagueID: strings.TrimSpace(leagueHint),
	})
	if err != nil {
		return nil, nil, err
	}
	if len(searchResult.Entities) == 0 {
		return nil, searchResult.Warnings, fmt.Errorf("no matches found for %q", query)
	}

	entity := searchResult.Entities[0]
	ref := buildMatchRef(entity)
	if strings.TrimSpace(ref) == "" {
		return nil, searchResult.Warnings, fmt.Errorf("unable to resolve match ref for %q", query)
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return nil, searchResult.Warnings, err
	}
	match, err := NormalizeMatch(resolved.Body)
	if err != nil {
		return nil, searchResult.Warnings, fmt.Errorf("normalize match %q: %w", resolved.CanonicalRef, err)
	}
	return match, searchResult.Warnings, nil
}

func (s *AnalysisService) playerNameMapForMatch(ctx context.Context, session *HistoricalScopeSession, matchID string) (map[string]string, []string) {
	players, warnings, err := session.HydratePlayerMatchSummaries(ctx, matchID)
	if err != nil {
		return map[string]string{}, []string{fmt.Sprintf("match %s player names: %v", matchID, err)}
	}
	out := map[string]string{}
	for _, player := range players {
		id := strings.TrimSpace(player.PlayerID)
		if id == "" {
			continue
		}
		if _, ok := out[id]; ok {
			continue
		}
		out[id] = strings.TrimSpace(player.PlayerName)
	}
	return out, warnings
}

func analysisResult(kind EntityKind, view AnalysisView, warnings []string) NormalizedResult {
	if len(view.Rows) == 0 {
		result := NormalizedResult{
			Kind:    kind,
			Status:  ResultStatusEmpty,
			Message: "no analysis rows found for selected scope",
			Data:    view,
		}
		if compact := compactWarnings(warnings); len(compact) > 0 {
			result.Status = ResultStatusPartial
			result.Warnings = compact
		}
		return result
	}

	if compact := compactWarnings(warnings); len(compact) > 0 {
		return NewPartialResult(kind, view, compact...)
	}
	return NewDataResult(kind, view)
}

func buildSingleScope(run *analysisScopeRun, opts AnalysisMetricOptions) AnalysisScope {
	scope := run.scope
	seasons := []string{}
	if run.mode == analysisScopeSeason {
		seasons = append(seasons, seasonIdentifier(scope, run.seasonHint))
	} else if run.seasonHint != "" {
		seasons = append(seasons, strings.TrimSpace(run.seasonHint))
	}

	return AnalysisScope{
		Mode:              run.mode,
		RequestedLeagueID: strings.TrimSpace(opts.LeagueQuery),
		LeagueID:          strings.TrimSpace(scope.League.ID),
		LeagueName:        strings.TrimSpace(scope.League.Name),
		Seasons:           compactWarnings(seasons),
		MatchIDs:          scope.MatchIDs,
		MatchCount:        len(scope.MatchIDs),
		DateFrom:          strings.TrimSpace(nonEmpty(scope.DateFrom, opts.DateFrom)),
		DateTo:            strings.TrimSpace(nonEmpty(scope.DateTo, opts.DateTo)),
		TypeQuery:         strings.TrimSpace(opts.TypeQuery),
		GroupQuery:        strings.TrimSpace(opts.GroupQuery),
		HydrationMetric:   run.session.Metrics(),
	}
}

func analysisTransportResult(kind EntityKind, requestedRef string, err error) *NormalizedResult {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		result := NewTransportErrorResult(kind, requestedRef, err)
		return &result
	}
	return nil
}

func normalizeBowlingMetric(raw string) (string, error) {
	metric := strings.ToLower(strings.TrimSpace(raw))
	metric = strings.ReplaceAll(metric, "_", "-")
	metric = strings.ReplaceAll(metric, " ", "-")
	switch metric {
	case analysisMetricEconomy, analysisMetricDots, analysisMetricSixesConceded:
		return metric, nil
	default:
		return "", fmt.Errorf("--metric must be one of: economy, dots, sixes-conceded")
	}
}

func normalizeBattingMetric(raw string) (string, error) {
	metric := strings.ToLower(strings.TrimSpace(raw))
	metric = strings.ReplaceAll(metric, "_", "-")
	metric = strings.ReplaceAll(metric, " ", "-")
	switch metric {
	case analysisMetricFours, analysisMetricSixes, analysisMetricStrikeRate:
		return metric, nil
	default:
		return "", fmt.Errorf("--metric must be one of: fours, sixes, strike-rate")
	}
}

func parseMetricScope(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("--scope is required (match:<match> or season:<season>)")
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("--scope must use explicit mode: match:<match> or season:<season>")
	}
	mode := strings.ToLower(strings.TrimSpace(parts[0]))
	query := strings.TrimSpace(parts[1])
	if query == "" {
		return "", "", fmt.Errorf("scope query is required")
	}
	switch mode {
	case analysisScopeMatch, analysisScopeSeason:
		return mode, query, nil
	default:
		return "", "", fmt.Errorf("unsupported --scope mode %q (expected match or season)", mode)
	}
}

func parseSeasonRange(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("--seasons is required (example: 2023-2025)")
	}

	items := strings.Split(raw, ",")
	out := make([]string, 0)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "-") {
			parts := strings.SplitN(item, "-", 2)
			left := parseYear(parts[0])
			right := parseYear(parts[1])
			if left == 0 || right == 0 {
				return nil, fmt.Errorf("invalid season range %q", item)
			}
			if left > right {
				left, right = right, left
			}
			for year := left; year <= right; year++ {
				out = append(out, fmt.Sprintf("%d", year))
			}
			continue
		}
		if parseYear(item) == 0 {
			return nil, fmt.Errorf("invalid season %q", item)
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--seasons did not resolve any seasons")
	}
	return dedupeStrings(out), nil
}

func parseGroupBy(raw string, defaults []string, allowed map[string]struct{}) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]string{}, defaults...), nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		normalized := normalizeGroupField(part)
		if normalized == "" {
			continue
		}
		if _, ok := allowed[normalized]; !ok {
			return nil, fmt.Errorf("unsupported --group-by field %q", strings.TrimSpace(part))
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return append([]string{}, defaults...), nil
	}
	return out, nil
}

func normalizeGroupField(raw string) string {
	field := strings.ToLower(strings.TrimSpace(raw))
	field = strings.ReplaceAll(field, "_", "-")
	field = strings.ReplaceAll(field, " ", "-")
	switch field {
	case "dismissal", "dismissals", "dismissaltype", "dismissal-type":
		return "dismissal-type"
	case "inning", "innings", "innings-period":
		return "innings"
	default:
		return field
	}
}

type analysisSourceRow struct {
	MatchID       string
	LeagueID      string
	SeasonID      string
	TeamID        string
	TeamName      string
	PlayerID      string
	PlayerName    string
	DismissalType string
	InningsNumber int
	Period        int
	CountValue    int

	Dots          int
	SixesConceded int
	Balls         int
	RunsConceded  int
	EconomySample float64

	Fours        int
	BattingSixes int
	RunsScored   int
	BallsFaced   int
	StrikeSample float64
}

type analysisAggregate struct {
	row      AnalysisRow
	matchIDs map[string]struct{}
	count    int

	dots          int
	sixesConceded int
	balls         int
	runsConceded  int
	economyTotal  float64
	economyCount  int

	fours           int
	battingSixes    int
	runsScored      int
	ballsFaced      int
	strikeRateTotal float64
	strikeRateCount int
}

func buildAnalysisGroup(row analysisSourceRow, groupBy []string) (string, AnalysisRow) {
	parts := make([]string, 0, len(groupBy))
	dims := AnalysisRow{}

	for _, field := range groupBy {
		switch field {
		case "player":
			label := firstNonEmptyString(row.PlayerName, row.PlayerID)
			if label == "" {
				label = "unknown-player"
			}
			parts = append(parts, "player="+label)
			dims.PlayerID = row.PlayerID
			dims.PlayerName = row.PlayerName
		case "team":
			label := firstNonEmptyString(row.TeamName, row.TeamID)
			if label == "" {
				label = "unknown-team"
			}
			parts = append(parts, "team="+label)
			dims.TeamID = row.TeamID
			dims.TeamName = row.TeamName
		case "league":
			label := firstNonEmptyString(row.LeagueID)
			if label == "" {
				label = "unknown-league"
			}
			parts = append(parts, "league="+label)
			dims.LeagueID = row.LeagueID
		case "season":
			label := firstNonEmptyString(row.SeasonID)
			if label == "" {
				label = "unknown-season"
			}
			parts = append(parts, "season="+label)
			dims.SeasonID = row.SeasonID
		case "dismissal-type":
			label := firstNonEmptyString(row.DismissalType)
			if label == "" {
				label = "unknown-dismissal"
			}
			parts = append(parts, "dismissal="+label)
			dims.DismissalType = row.DismissalType
		case "innings":
			label := fmt.Sprintf("%d/%d", row.InningsNumber, row.Period)
			if row.InningsNumber <= 0 {
				label = "unknown-innings"
			}
			parts = append(parts, "innings="+label)
			dims.InningsNumber = row.InningsNumber
			dims.Period = row.Period
		}
	}

	if len(parts) == 0 {
		return "all", dims
	}
	return strings.Join(parts, " | "), dims
}

func rankAnalysisRows(rows []AnalysisRow, asc bool) []AnalysisRow {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Value != rows[j].Value {
			if asc {
				return rows[i].Value < rows[j].Value
			}
			return rows[i].Value > rows[j].Value
		}
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Key < rows[j].Key
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows
}

func trimAnalysisRows(rows []AnalysisRow, top int) []AnalysisRow {
	if top <= 0 || top >= len(rows) {
		return rows
	}
	return append([]AnalysisRow(nil), rows[:top]...)
}

func economyFromAggregate(agg *analysisAggregate) float64 {
	if agg == nil {
		return 0
	}
	if agg.balls > 0 {
		overs := float64(agg.balls) / 6.0
		if overs > 0 {
			return float64(agg.runsConceded) / overs
		}
	}
	if agg.economyCount > 0 {
		return agg.economyTotal / float64(agg.economyCount)
	}
	return 0
}

func hasBowlingActivity(agg *analysisAggregate) bool {
	if agg == nil {
		return false
	}
	return agg.balls > 0 || agg.runsConceded > 0 || agg.dots > 0 || agg.sixesConceded > 0 || agg.economyCount > 0
}

func strikeRateFromAggregate(agg *analysisAggregate) float64 {
	if agg == nil {
		return 0
	}
	if agg.ballsFaced > 0 {
		return (float64(agg.runsScored) * 100.0) / float64(agg.ballsFaced)
	}
	if agg.strikeRateCount > 0 {
		return agg.strikeRateTotal / float64(agg.strikeRateCount)
	}
	return 0
}

type bowlingTotals struct {
	dots          int
	sixesConceded int
	balls         int
	conceded      int
	economy       float64
}

func extractBowlingTotals(player PlayerMatch) bowlingTotals {
	totals := bowlingTotals{
		dots:          player.Summary.Dots,
		sixesConceded: player.Summary.SixesConceded,
		economy:       player.Summary.EconomyRate,
	}

	for _, category := range player.Bowling {
		for _, stat := range category.Stats {
			switch normalizeStatName(stat.Name) {
			case "dots":
				totals.dots += statAsInt(stat)
			case "sixesconceded":
				totals.sixesConceded += statAsInt(stat)
			case "balls":
				totals.balls += statAsInt(stat)
			case "conceded":
				totals.conceded += statAsInt(stat)
			case "economyrate":
				if value := statAsFloat(stat); value > 0 {
					totals.economy = value
				}
			}
		}
	}

	// summary already includes merged dots/sixes values in most payloads; avoid double counting.
	if totals.dots > 0 && player.Summary.Dots > 0 {
		totals.dots = analysisMaxInt(totals.dots, player.Summary.Dots)
	}
	if totals.sixesConceded > 0 && player.Summary.SixesConceded > 0 {
		totals.sixesConceded = analysisMaxInt(totals.sixesConceded, player.Summary.SixesConceded)
	}
	return totals
}

type battingTotals struct {
	fours      int
	sixes      int
	runs       int
	balls      int
	strikeRate float64
}

func extractBattingTotals(player PlayerMatch) battingTotals {
	totals := battingTotals{strikeRate: player.Summary.StrikeRate, balls: player.Summary.BallsFaced}
	for _, category := range player.Batting {
		for _, stat := range category.Stats {
			switch normalizeStatName(stat.Name) {
			case "fours":
				totals.fours += statAsInt(stat)
			case "sixes":
				totals.sixes += statAsInt(stat)
			case "runs":
				totals.runs += statAsInt(stat)
			case "ballsfaced":
				totals.balls += statAsInt(stat)
			case "strikerate":
				if value := statAsFloat(stat); value > 0 {
					totals.strikeRate = value
				}
			}
		}
	}
	return totals
}

func seasonIdentifier(scope HistoricalScopeSummary, fallback string) string {
	if scope.Season != nil {
		if strings.TrimSpace(scope.Season.ID) != "" {
			return strings.TrimSpace(scope.Season.ID)
		}
		if scope.Season.Year > 0 {
			return fmt.Sprintf("%d", scope.Season.Year)
		}
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return ""
}

func seasonForMatch(match Match, fallback string) string {
	if date := strings.TrimSpace(match.Date); date != "" {
		if parsed, ok := parseMatchTime(match); ok {
			return fmt.Sprintf("%d", parsed.UTC().Year())
		}
		if len(date) >= 4 {
			if _, err := strconv.Atoi(date[:4]); err == nil {
				return date[:4]
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func analysisMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func analysisDisplayPlayerName(resolver *Resolver, playerID, fallback string) string {
	name := strings.TrimSpace(fallback)
	if name != "" {
		return name
	}
	if resolver != nil && resolver.index != nil {
		if indexed, ok := resolver.index.FindByID(EntityPlayer, strings.TrimSpace(playerID)); ok {
			name = nonEmpty(indexed.Name, indexed.ShortName)
		}
	}
	return strings.TrimSpace(name)
}

func analysisDisplayTeamName(resolver *Resolver, teamID, fallback string) string {
	name := strings.TrimSpace(fallback)
	if name != "" {
		return name
	}
	if resolver != nil && resolver.index != nil {
		if indexed, ok := resolver.index.FindByID(EntityTeam, strings.TrimSpace(teamID)); ok {
			name = nonEmpty(indexed.ShortName, indexed.Name)
		}
	}
	return strings.TrimSpace(name)
}

type analysisFilterSpec struct {
	teamQuery     string
	playerQuery   string
	dismissalType string
	innings       int
	period        int
}

func analysisFiltersFromDismissal(opts AnalysisDismissalOptions) analysisFilterSpec {
	return analysisFilterSpec{
		teamQuery:     strings.TrimSpace(opts.TeamQuery),
		playerQuery:   strings.TrimSpace(opts.PlayerQuery),
		dismissalType: strings.TrimSpace(opts.DismissalType),
		innings:       opts.Innings,
		period:        opts.Period,
	}
}

func analysisFiltersFromMetric(opts AnalysisMetricOptions) analysisFilterSpec {
	return analysisFilterSpec{
		teamQuery:     strings.TrimSpace(opts.TeamQuery),
		playerQuery:   strings.TrimSpace(opts.PlayerQuery),
		dismissalType: strings.TrimSpace(opts.DismissalType),
		innings:       opts.Innings,
		period:        opts.Period,
	}
}

func (f analysisFilterSpec) matches(row analysisSourceRow) bool {
	if !f.matchesTeam(row) {
		return false
	}
	if !f.matchesPlayer(row) {
		return false
	}
	if !f.matchesDismissal(row) {
		return false
	}
	if !f.matchesInnings(row) {
		return false
	}
	if !f.matchesPeriod(row) {
		return false
	}
	return true
}

func (f analysisFilterSpec) matchesPartnership(row analysisSourceRow) bool {
	if !f.matchesTeam(row) {
		return false
	}
	if !f.matchesInnings(row) {
		return false
	}
	return true
}

func (f analysisFilterSpec) matchesTeam(row analysisSourceRow) bool {
	query := normalizeAlias(f.teamQuery)
	if query == "" {
		return true
	}
	candidates := []string{normalizeAlias(row.TeamID), normalizeAlias(row.TeamName)}
	for _, candidate := range candidates {
		if candidate != "" && candidate == query {
			return true
		}
	}
	return false
}

func (f analysisFilterSpec) matchesPlayer(row analysisSourceRow) bool {
	query := normalizeAlias(f.playerQuery)
	if query == "" {
		return true
	}
	candidates := []string{normalizeAlias(row.PlayerID), normalizeAlias(row.PlayerName)}
	for _, candidate := range candidates {
		if candidate != "" && candidate == query {
			return true
		}
	}
	return false
}

func (f analysisFilterSpec) matchesDismissal(row analysisSourceRow) bool {
	query := normalizeAlias(f.dismissalType)
	if query == "" {
		return true
	}
	return normalizeAlias(row.DismissalType) == query
}

func (f analysisFilterSpec) matchesInnings(row analysisSourceRow) bool {
	if f.innings <= 0 {
		return true
	}
	return row.InningsNumber == f.innings
}

func (f analysisFilterSpec) matchesPeriod(row analysisSourceRow) bool {
	if f.period <= 0 {
		return true
	}
	return row.Period == f.period
}
