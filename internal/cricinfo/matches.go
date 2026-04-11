package cricinfo

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultMatchListLimit = 20
const matchListEventFetchConcurrency = 3
const matchListStatusFetchConcurrency = 3
const matchListEventFetchTimeout = 4500 * time.Millisecond
const matchListStatusFetchTimeout = 4 * time.Second
const matchLineupRosterFetchConcurrency = 2
const matchLineupRosterFetchTimeout = 5 * time.Second
const deliveryFetchConcurrency = 96
const detailSubresourceFetchConcurrency = 24
const detailItemFetchTimeout = 3 * time.Second
const liveViewRecentDeliveryFetchCount = 60
const matchTeamQueryScanRange = 6
const maxTeamQueryEventCandidates = 36
const teamQueryEventFetchTimeout = 1500 * time.Millisecond

// MatchServiceConfig configures match discovery and lookup behavior.
type MatchServiceConfig struct {
	Client   *Client
	Resolver *Resolver
}

// MatchListOptions controls list/live traversal behavior.
type MatchListOptions struct {
	Limit    int
	LeagueID string
}

// MatchLookupOptions controls resolver-backed single match lookup.
type MatchLookupOptions struct {
	LeagueID string
}

// MatchInningsOptions controls innings-depth lookup behavior.
type MatchInningsOptions struct {
	LeagueID  string
	TeamQuery string
	Innings   int
	Period    int
}

// MatchDuelOptions controls batter-vs-bowler matchup lookup behavior.
type MatchDuelOptions struct {
	LeagueID    string
	BatterQuery string
	BowlerQuery string
}

// MatchService implements domain-level match discovery and lookup commands.
type MatchService struct {
	client       *Client
	resolver     *Resolver
	ownsResolver bool
}

// NewMatchService builds a match service using default client/resolver when omitted.
func NewMatchService(cfg MatchServiceConfig) (*MatchService, error) {
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

	return &MatchService{
		client:       client,
		resolver:     resolver,
		ownsResolver: ownsResolver,
	}, nil
}

// Close persists resolver cache when owned by this service.
func (s *MatchService) Close() error {
	if !s.ownsResolver || s.resolver == nil {
		return nil
	}
	return s.resolver.Close()
}

// List discovers current matches from /events.
func (s *MatchService) List(ctx context.Context, opts MatchListOptions) (NormalizedResult, error) {
	return s.listFromEvents(ctx, opts, false)
}

// Live discovers current in-progress matches from /events.
func (s *MatchService) Live(ctx context.Context, opts MatchListOptions) (NormalizedResult, error) {
	return s.listFromEvents(ctx, opts, true)
}

// Lineups resolves one match and returns match-scoped roster entries for both teams.
func (s *MatchService) Lineups(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityTeamRoster
		return *passthrough, nil
	}

	if len(lookup.match.Teams) == 0 {
		return NormalizedResult{
			Kind:    EntityTeamRoster,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("no teams found for match %q", lookup.match.ID),
		}, nil
	}

	type lineupLoadResult struct {
		entries []TeamRosterEntry
		warns   []string
	}

	results := make([]lineupLoadResult, len(lookup.match.Teams))
	sem := make(chan struct{}, matchLineupRosterFetchConcurrency)
	var wg sync.WaitGroup
	teamCache := map[string]teamIdentity{}

	teamService := &TeamService{
		client:   s.client,
		resolver: s.resolver,
	}

	for i := range lookup.match.Teams {
		team := lookup.match.Teams[i]
		teamID := strings.TrimSpace(team.ID)
		if teamID == "" {
			teamID = strings.TrimSpace(refIDs(team.Ref)["teamId"])
		}
		if teamID == "" {
			teamID = strings.TrimSpace(refIDs(team.Ref)["competitorId"])
		}
		if teamID == "" {
			results[i].warns = []string{fmt.Sprintf("skip team with missing id/ref in match %q", lookup.match.ID)}
			continue
		}
		team.ID = teamID
		if strings.TrimSpace(team.Name) == "" || strings.TrimSpace(team.ShortName) == "" {
			identity, err := s.fetchTeamIdentity(ctx, &team, teamCache)
			if err != nil {
				results[i].warns = append(results[i].warns, fmt.Sprintf("team %s: %v", nonEmpty(team.Ref, team.ID), err))
			} else {
				team.Name = nonEmpty(team.Name, identity.name)
				team.ShortName = nonEmpty(team.ShortName, identity.shortName)
			}
		}

		wg.Add(1)
		go func(index int, team Team) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			rosterRef := nonEmpty(strings.TrimSpace(team.RosterRef), competitorSubresourceRef(*lookup.match, team.ID, "roster"))
			if rosterRef == "" {
				results[index].warns = []string{fmt.Sprintf("roster route unavailable for team %q", team.ID)}
				return
			}

			reqCtx, cancel := context.WithTimeout(ctx, matchLineupRosterFetchTimeout)
			resolved, err := s.client.ResolveRefChain(reqCtx, rosterRef)
			cancel()
			if err != nil {
				results[index].warns = []string{fmt.Sprintf("roster %s: %v", rosterRef, err)}
				return
			}

			entries, err := NormalizeTeamRosterEntries(resolved.Body, team, TeamScopeMatch, lookup.match.ID)
			if err != nil {
				results[index].warns = []string{fmt.Sprintf("roster %s: %v", resolved.CanonicalRef, err)}
				return
			}

			for i := range entries {
				entries[i].TeamName = nonEmpty(entries[i].TeamName, team.ShortName, team.Name, team.ID)
			}

			reqCtx, cancel = context.WithTimeout(ctx, matchLineupRosterFetchTimeout)
			hydrateWarnings := teamService.enrichRosterEntries(reqCtx, entries)
			cancel()
			results[index] = lineupLoadResult{
				entries: entries,
				warns:   hydrateWarnings,
			}
		}(i, team)
	}

	wg.Wait()

	warnings := append([]string{}, lookup.warnings...)
	items := make([]any, 0)
	for i := range results {
		warnings = append(warnings, results[i].warns...)
		for _, entry := range results[i].entries {
			items = append(items, entry)
		}
	}

	result := NewListResult(EntityTeamRoster, items)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityTeamRoster, items, compactWarnings(warnings)...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	if len(items) == 0 && strings.TrimSpace(result.Message) == "" {
		result.Message = "no lineup entries found for this match"
	}
	return result, nil
}

// Show resolves and returns one match with normalized summary fields.
func (s *MatchService) Show(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	return s.lookupMatch(ctx, query, opts, false)
}

// Status resolves and returns one match with status-focused summary fields.
func (s *MatchService) Status(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	return s.lookupMatch(ctx, query, opts, true)
}

// Scorecard resolves and returns matchcards rendered as batting/bowling/partnership views.
func (s *MatchService) Scorecard(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityMatchScorecard
		return *passthrough, nil
	}
	statusCache := map[string]matchStatusSnapshot{}
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}
	hydrationWarnings := s.hydrateMatch(ctx, lookup.match, statusCache, teamCache, scoreCache)

	scorecardRef := matchSubresourceRef(*lookup.match, "matchcards", "matchcards")
	if scorecardRef == "" {
		return NormalizedResult{
			Kind:    EntityMatchScorecard,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("scorecard route unavailable for match %q", lookup.match.ID),
		}, nil
	}

	resolved, err := s.resolveRefChainResilient(ctx, scorecardRef)
	if err != nil {
		if live, liveWarnings := s.buildLiveView(ctx, *lookup.match); live != nil {
			scorecard := &MatchScorecard{
				Ref:           scorecardRef,
				LeagueID:      lookup.match.LeagueID,
				EventID:       lookup.match.EventID,
				CompetitionID: lookup.match.CompetitionID,
				MatchID:       lookup.match.ID,
			}
			augmentScorecardFromLive(scorecard, live)
			warnings := append([]string{}, lookup.warnings...)
			warnings = append(warnings, hydrationWarnings...)
			warnings = append(warnings, liveWarnings...)
			warnings = append(warnings, fmt.Sprintf("scorecard fallback used after %v", err))
			result := NewPartialResult(EntityMatchScorecard, scorecard, warnings...)
			result.RequestedRef = scorecardRef
			result.CanonicalRef = scorecardRef
			return result, nil
		}
		return NewTransportErrorResult(EntityMatchScorecard, scorecardRef, err), nil
	}

	scorecard, err := NormalizeMatchScorecard(resolved.Body, *lookup.match)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize matchcards %q: %w", resolved.CanonicalRef, err)
	}
	enrichmentWarnings := []string{}
	if len(scorecard.BattingCards) == 0 || len(scorecard.BowlingCards) == 0 {
		if live, warns := s.buildLiveView(ctx, *lookup.match); live != nil {
			enrichmentWarnings = append(enrichmentWarnings, warns...)
			augmentScorecardFromLive(scorecard, live)
		}
	}

	warnings := append([]string{}, lookup.warnings...)
	warnings = append(warnings, hydrationWarnings...)
	warnings = append(warnings, enrichmentWarnings...)
	result := NewDataResult(EntityMatchScorecard, scorecard)
	if len(warnings) > 0 {
		result = NewPartialResult(EntityMatchScorecard, scorecard, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Details resolves and returns normalized delivery events from the details route.
func (s *MatchService) Details(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityDeliveryEvent
		return *passthrough, nil
	}

	detailsRef := nonEmpty(strings.TrimSpace(lookup.match.DetailsRef), matchSubresourceRef(*lookup.match, "details", "details"))
	if detailsRef == "" {
		return NormalizedResult{
			Kind:    EntityDeliveryEvent,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("details route unavailable for match %q", lookup.match.ID),
		}, nil
	}

	events, warnings, err := s.deliveryEventsForMatchRefs(ctx, detailsRef, matchSubresourceRef(*lookup.match, "plays", "plays"))
	if err != nil {
		return NewTransportErrorResult(EntityDeliveryEvent, detailsRef, err), nil
	}
	warnings = append(lookup.warnings, warnings...)

	items := make([]any, 0, len(events))
	for _, delivery := range events {
		items = append(items, delivery)
	}
	result := NewListResult(EntityDeliveryEvent, items)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityDeliveryEvent, items, warnings...)
	}
	result.RequestedRef = detailsRef
	result.CanonicalRef = detailsRef
	return result, nil
}

// Plays resolves and returns normalized delivery events from the plays route.
func (s *MatchService) Plays(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityDeliveryEvent
		return *passthrough, nil
	}

	playsRef := matchSubresourceRef(*lookup.match, "plays", "plays")
	if playsRef == "" {
		return NormalizedResult{
			Kind:    EntityDeliveryEvent,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("plays route unavailable for match %q", lookup.match.ID),
		}, nil
	}

	events, warnings, err := s.deliveryEventsForMatchRefs(ctx, playsRef, nonEmpty(strings.TrimSpace(lookup.match.DetailsRef), matchSubresourceRef(*lookup.match, "details", "details")))
	if err != nil {
		return NewTransportErrorResult(EntityDeliveryEvent, playsRef, err), nil
	}
	warnings = append(lookup.warnings, warnings...)

	items := make([]any, 0, len(events))
	for _, delivery := range events {
		items = append(items, delivery)
	}
	result := NewListResult(EntityDeliveryEvent, items)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityDeliveryEvent, items, warnings...)
	}
	result.RequestedRef = playsRef
	result.CanonicalRef = playsRef
	return result, nil
}

// Situation resolves and returns normalized match situation data.
func (s *MatchService) Situation(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityMatchSituation
		return *passthrough, nil
	}

	statusCache := map[string]matchStatusSnapshot{}
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}
	hydrationWarnings := s.hydrateMatch(ctx, lookup.match, statusCache, teamCache, scoreCache)

	situationRef := matchSubresourceRef(*lookup.match, "situation", "situation")
	if situationRef == "" {
		return NormalizedResult{
			Kind:    EntityMatchSituation,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("situation route unavailable for match %q", lookup.match.ID),
		}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, situationRef)
	if err != nil {
		return NewTransportErrorResult(EntityMatchSituation, situationRef, err), nil
	}

	situation, err := NormalizeMatchSituation(resolved.Body, *lookup.match)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize situation %q: %w", resolved.CanonicalRef, err)
	}

	if isSparseSituation(situation) {
		if live, warnings := s.buildLiveView(ctx, *lookup.match); live != nil {
			situation.Live = live
			result := NewDataResult(EntityMatchSituation, situation)
			combinedWarnings := append([]string{}, lookup.warnings...)
			combinedWarnings = append(combinedWarnings, hydrationWarnings...)
			combinedWarnings = append(combinedWarnings, warnings...)
			if len(combinedWarnings) > 0 {
				result = NewPartialResult(EntityMatchSituation, situation, combinedWarnings...)
			}
			result.RequestedRef = resolved.RequestedRef
			result.CanonicalRef = resolved.CanonicalRef
			return result, nil
		}
		result := NormalizedResult{
			Kind:         EntityMatchSituation,
			Status:       ResultStatusEmpty,
			RequestedRef: resolved.RequestedRef,
			CanonicalRef: resolved.CanonicalRef,
			Message:      "no situation data available for this match",
		}
		return result, nil
	}

	result := NewDataResult(EntityMatchSituation, situation)
	combinedWarnings := append([]string{}, lookup.warnings...)
	combinedWarnings = append(combinedWarnings, hydrationWarnings...)
	if len(combinedWarnings) > 0 {
		result = NewPartialResult(EntityMatchSituation, situation, combinedWarnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// LiveView resolves and returns a fan-first live view synthesized from delivery details.
func (s *MatchService) LiveView(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityMatchSituation
		return *passthrough, nil
	}

	statusCache := map[string]matchStatusSnapshot{}
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}
	hydrationWarnings := s.hydrateMatch(ctx, lookup.match, statusCache, teamCache, scoreCache)

	live, liveWarnings := s.buildLiveView(ctx, *lookup.match)
	if live == nil {
		fallback, fallbackErr := s.Situation(ctx, query, opts)
		if fallbackErr == nil && fallback.Status != ResultStatusError {
			combinedWarnings := append([]string{}, fallback.Warnings...)
			combinedWarnings = append(combinedWarnings, lookup.warnings...)
			combinedWarnings = append(combinedWarnings, hydrationWarnings...)
			combinedWarnings = append(combinedWarnings, liveWarnings...)
			combinedWarnings = append(combinedWarnings, "live-view fallback: showing situation data")
			combinedWarnings = compactWarnings(combinedWarnings)

			if fallback.Data != nil && len(combinedWarnings) > 0 {
				partial := NewPartialResult(EntityMatchSituation, fallback.Data, combinedWarnings...)
				partial.RequestedRef = nonEmpty(fallback.RequestedRef, lookup.resolved.RequestedRef)
				partial.CanonicalRef = nonEmpty(fallback.CanonicalRef, lookup.resolved.CanonicalRef)
				return partial, nil
			}
			return fallback, nil
		}

		result := NormalizedResult{
			Kind:    EntityMatchSituation,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("no live view data available for match %q", lookup.match.ID),
		}
		warnings := compactWarnings(append(append(append([]string{}, lookup.warnings...), hydrationWarnings...), liveWarnings...))
		if len(warnings) > 0 {
			result.Status = ResultStatusPartial
			result.Message = "live view unavailable"
			result.Warnings = warnings
		}
		return result, nil
	}

	situation := &MatchSituation{
		Ref:           matchSubresourceRef(*lookup.match, "situation", "situation"),
		LeagueID:      lookup.match.LeagueID,
		EventID:       lookup.match.EventID,
		CompetitionID: lookup.match.CompetitionID,
		MatchID:       lookup.match.ID,
		Live:          live,
	}
	warnings := append([]string{}, lookup.warnings...)
	warnings = append(warnings, hydrationWarnings...)
	warnings = append(warnings, liveWarnings...)

	result := NewDataResult(EntityMatchSituation, situation)
	if len(warnings) > 0 {
		result = NewPartialResult(EntityMatchSituation, situation, warnings...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	return result, nil
}

// Duel resolves and returns a batter-vs-bowler matchup summary for one match.
func (s *MatchService) Duel(ctx context.Context, query string, opts MatchDuelOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, MatchLookupOptions{LeagueID: opts.LeagueID})
	if passthrough != nil {
		passthrough.Kind = EntityMatchDuel
		return *passthrough, nil
	}
	if strings.TrimSpace(opts.BatterQuery) == "" || strings.TrimSpace(opts.BowlerQuery) == "" {
		return NormalizedResult{
			Kind:    EntityMatchDuel,
			Status:  ResultStatusEmpty,
			Message: "--batter and --bowler are required",
		}, nil
	}
	statusCache := map[string]matchStatusSnapshot{}
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}
	hydrationWarnings := s.hydrateMatch(ctx, lookup.match, statusCache, teamCache, scoreCache)

	detailsRef := nonEmpty(strings.TrimSpace(lookup.match.DetailsRef), matchSubresourceRef(*lookup.match, "details", "details"))
	playsRef := matchSubresourceRef(*lookup.match, "plays", "plays")
	primaryRef := nonEmpty(detailsRef, playsRef)
	deliveries, warnings, err := s.deliveryEventsForMatchRefs(ctx, primaryRef, playsRef)
	if err != nil {
		return NewTransportErrorResult(EntityMatchDuel, primaryRef, err), nil
	}
	if len(deliveries) == 0 {
		return NormalizedResult{
			Kind:    EntityMatchDuel,
			Status:  ResultStatusEmpty,
			Message: "no delivery data available for duel analysis",
		}, nil
	}
	matchLatest := deliveries[len(deliveries)-1]

	batterID, batterName := resolveDuelIdentity(deliveries, strings.TrimSpace(opts.BatterQuery), true)
	bowlerID, bowlerName := resolveDuelIdentity(deliveries, strings.TrimSpace(opts.BowlerQuery), false)
	if batterID == "" && normalizeAlias(batterName) == "" {
		return NormalizedResult{Kind: EntityMatchDuel, Status: ResultStatusEmpty, Message: fmt.Sprintf("batter %q not found in this match stream", opts.BatterQuery)}, nil
	}
	if bowlerID == "" && normalizeAlias(bowlerName) == "" {
		return NormalizedResult{Kind: EntityMatchDuel, Status: ResultStatusEmpty, Message: fmt.Sprintf("bowler %q not found in this match stream", opts.BowlerQuery)}, nil
	}

	runs := 0
	dots := 0
	fours := 0
	sixes := 0
	wickets := 0
	balls := 0
	recent := make([]DeliveryEvent, 0, 8)
	lastUpdate := int64(0)
	for _, delivery := range deliveries {
		if !deliveryMatchesDuel(delivery, batterID, batterName, bowlerID, bowlerName) {
			continue
		}
		balls++
		if delivery.ScoreValue > 0 {
			runs += delivery.ScoreValue
		} else {
			dots++
		}
		short := strings.ToUpper(strings.TrimSpace(delivery.ShortText))
		if strings.Contains(short, "FOUR") {
			fours++
		}
		if strings.Contains(short, "SIX") {
			sixes++
		}
		if truthyField(delivery.Dismissal, "dismissal") {
			wickets++
		}
		recent = append(recent, delivery)
		if delivery.BBBTimestamp > lastUpdate {
			lastUpdate = delivery.BBBTimestamp
		}
	}
	if balls == 0 {
		return NormalizedResult{
			Kind:    EntityMatchDuel,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("no deliveries found for %s vs %s", nonEmpty(batterName, opts.BatterQuery), nonEmpty(bowlerName, opts.BowlerQuery)),
		}, nil
	}
	if len(recent) > 8 {
		recent = recent[len(recent)-8:]
	}
	liveScore := firstNonEmpty(matchScoreLabel(matchLatest.HomeScore), matchScoreLabel(matchLatest.AwayScore), lookup.match.ScoreSummary)
	liveOver := overBallString(matchLatest.OverNumber, matchLatest.BallNumber)
	duelScore := liveScore
	if liveScore != "" && liveOver != "" {
		duelScore = fmt.Sprintf("%s (%s ov)", liveScore, liveOver)
	}

	duel := MatchDuel{
		MatchID:      lookup.match.ID,
		Fixture:      nonEmpty(lookup.match.ShortDescription, lookup.match.Description),
		Score:        duelScore,
		BatterID:     batterID,
		BatterName:   nonEmpty(batterName, opts.BatterQuery),
		BowlerID:     bowlerID,
		BowlerName:   nonEmpty(bowlerName, opts.BowlerQuery),
		Balls:        balls,
		Runs:         runs,
		Dots:         dots,
		Fours:        fours,
		Sixes:        sixes,
		Wickets:      wickets,
		StrikeRate:   strikeRate(runs, balls),
		RecentBalls:  recent,
		LastUpdateMS: lastUpdate,
		SnapshotAt:   time.Now().UTC().Format(time.RFC3339),
		SourceRoute:  primaryRef,
	}

	allWarnings := append([]string{}, lookup.warnings...)
	allWarnings = append(allWarnings, hydrationWarnings...)
	allWarnings = append(allWarnings, warnings...)
	result := NewDataResult(EntityMatchDuel, duel)
	if len(allWarnings) > 0 {
		result = NewPartialResult(EntityMatchDuel, duel, allWarnings...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	return result, nil
}

// Phases resolves and returns fan-oriented innings phase splits (powerplay/middle/death).
func (s *MatchService) Phases(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityMatchPhases
		return *passthrough, nil
	}

	statusCache := map[string]matchStatusSnapshot{}
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}
	warnings := append([]string{}, lookup.warnings...)
	warnings = append(warnings, s.hydrateMatch(ctx, lookup.match, statusCache, teamCache, scoreCache)...)

	teams, teamWarnings, teamResult := s.selectTeamsFromMatch(ctx, *lookup.match, "", opts.LeagueID)
	if teamResult != nil {
		teamResult.Kind = EntityMatchPhases
		return *teamResult, nil
	}
	warnings = append(warnings, teamWarnings...)

	report := MatchPhases{
		MatchID:       lookup.match.ID,
		LeagueID:      lookup.match.LeagueID,
		EventID:       lookup.match.EventID,
		CompetitionID: nonEmpty(lookup.match.CompetitionID, lookup.match.ID),
		Fixture:       nonEmpty(lookup.match.ShortDescription, lookup.match.Description),
		Result:        nonEmpty(lookup.match.MatchState, lookup.match.Note),
		Innings:       make([]MatchPhaseInning, 0),
	}

	for _, team := range teams {
		inningsList, _, inningsWarnings := s.fetchTeamInnings(ctx, *lookup.match, team)
		warnings = append(warnings, inningsWarnings...)
		for i := range inningsList {
			innings := inningsList[i]
			statsWarnings := s.hydrateInningsTimelines(ctx, &innings)
			warnings = append(warnings, statsWarnings...)
			if !isMeaningfulPhaseInnings(innings) {
				continue
			}

			phaseInnings := buildPhaseInnings(team, innings)
			if !phaseInningsHasData(phaseInnings) {
				continue
			}
			report.Innings = append(report.Innings, phaseInnings)
		}
	}

	result := NewDataResult(EntityMatchPhases, report)
	if len(warnings) > 0 {
		result = NewPartialResult(EntityMatchPhases, report, compactWarnings(warnings)...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	return result, nil
}

func isMeaningfulPhaseInnings(innings Innings) bool {
	if strings.TrimSpace(innings.Score) != "" {
		return true
	}
	if innings.Runs > 0 || innings.Wickets > 0 || innings.Target > 0 {
		return true
	}
	return len(innings.OverTimeline) > 0 || len(innings.WicketTimeline) > 0
}

func buildPhaseInnings(team Team, innings Innings) MatchPhaseInning {
	out := MatchPhaseInning{
		TeamID:        nonEmpty(strings.TrimSpace(team.ID), strings.TrimSpace(innings.TeamID)),
		TeamName:      nonEmpty(strings.TrimSpace(team.ShortName), strings.TrimSpace(team.Name), strings.TrimSpace(innings.TeamName), strings.TrimSpace(innings.TeamID)),
		InningsNumber: innings.InningsNumber,
		Period:        innings.Period,
		Score:         innings.Score,
		Target:        innings.Target,
		Powerplay:     PhaseSummary{Name: "Powerplay"},
		Middle:        PhaseSummary{Name: "Middle"},
		Death:         PhaseSummary{Name: "Death"},
	}

	bestRuns := -1
	for _, over := range innings.OverTimeline {
		phase := phaseBucket(over.Number)
		switch phase {
		case "Powerplay":
			accumulatePhase(&out.Powerplay, over)
		case "Middle":
			accumulatePhase(&out.Middle, over)
		case "Death":
			accumulatePhase(&out.Death, over)
		}

		if over.Runs > bestRuns {
			bestRuns = over.Runs
			out.BestScoringOver = over.Number
			out.BestScoringOverRuns = over.Runs
		}
		if over.WicketCount > out.CollapseWickets {
			out.CollapseWickets = over.WicketCount
			out.CollapseOver = over.Number
		}
	}

	finalizePhase(&out.Powerplay)
	finalizePhase(&out.Middle)
	finalizePhase(&out.Death)

	return out
}

func phaseBucket(overNumber int) string {
	switch {
	case overNumber >= 1 && overNumber <= 6:
		return "Powerplay"
	case overNumber >= 7 && overNumber <= 15:
		return "Middle"
	case overNumber >= 16:
		return "Death"
	default:
		return "Middle"
	}
}

func accumulatePhase(phase *PhaseSummary, over InningsOver) {
	if phase == nil {
		return
	}
	phase.Runs += over.Runs
	phase.Wickets += over.WicketCount
	phase.Overs += 1
}

func finalizePhase(phase *PhaseSummary) {
	if phase == nil || phase.Overs <= 0 {
		return
	}
	phase.RunRate = float64(phase.Runs) / phase.Overs
}

func phaseInningsHasData(innings MatchPhaseInning) bool {
	return innings.Powerplay.Overs > 0 || innings.Middle.Overs > 0 || innings.Death.Overs > 0
}

// Innings resolves and returns innings summaries with over and wicket timelines when period statistics are available.
func (s *MatchService) Innings(ctx context.Context, query string, opts MatchInningsOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, MatchLookupOptions{LeagueID: opts.LeagueID})
	if passthrough != nil {
		passthrough.Kind = EntityInnings
		return *passthrough, nil
	}
	statusCache := map[string]matchStatusSnapshot{}
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}
	lookup.warnings = append(lookup.warnings, s.hydrateMatch(ctx, lookup.match, statusCache, teamCache, scoreCache)...)

	teams, teamWarnings, teamResult := s.selectTeamsFromMatch(ctx, *lookup.match, opts.TeamQuery, opts.LeagueID)
	if teamResult != nil {
		teamResult.Kind = EntityInnings
		return *teamResult, nil
	}

	warnings := append([]string{}, lookup.warnings...)
	warnings = append(warnings, teamWarnings...)

	items := make([]any, 0)
	for _, team := range teams {
		innings, resolvedRef, inningsWarnings := s.fetchTeamInnings(ctx, *lookup.match, team)
		warnings = append(warnings, inningsWarnings...)
		for i := range innings {
			if strings.TrimSpace(team.ID) != "" {
				innings[i].TeamID = strings.TrimSpace(team.ID)
			}
			innings[i].TeamName = nonEmpty(team.ShortName, team.Name, team.ID, innings[i].TeamName)
			innings[i].MatchID = nonEmpty(innings[i].MatchID, lookup.match.ID)
			innings[i].CompetitionID = nonEmpty(innings[i].CompetitionID, lookup.match.CompetitionID, lookup.match.ID)
			innings[i].EventID = nonEmpty(innings[i].EventID, lookup.match.EventID)
			innings[i].LeagueID = nonEmpty(innings[i].LeagueID, lookup.match.LeagueID)

			statsWarnings := s.hydrateInningsTimelines(ctx, &innings[i])
			warnings = append(warnings, statsWarnings...)
			items = append(items, innings[i])
		}
		if strings.TrimSpace(resolvedRef) != "" && len(items) == 0 {
			warnings = append(warnings, fmt.Sprintf("no innings found at %s", resolvedRef))
		}
	}

	result := NewListResult(EntityInnings, items)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityInnings, items, warnings...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	if len(items) == 0 && strings.TrimSpace(result.Message) == "" {
		result.Message = "no innings available for selected scope"
	}
	return result, nil
}

// Partnerships resolves detailed partnership objects for a selected team/innings/period.
func (s *MatchService) Partnerships(ctx context.Context, query string, opts MatchInningsOptions) (NormalizedResult, error) {
	selected, passthrough := s.resolveSelectedInnings(ctx, query, opts, true)
	if passthrough != nil {
		passthrough.Kind = EntityPartnership
		return *passthrough, nil
	}
	if strings.TrimSpace(selected.innings.PartnershipsRef) == "" {
		return NormalizedResult{
			Kind:    EntityPartnership,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("partnership route unavailable for team %q innings %d/%d", selected.team.ID, selected.innings.InningsNumber, selected.innings.Period),
		}, nil
	}

	resolved, items, warnings, err := s.fetchDetailedRefCollection(
		ctx,
		selected.innings.PartnershipsRef,
		func(itemBody []byte) (any, error) {
			partnership, normalizeErr := NormalizePartnership(itemBody)
			if normalizeErr != nil {
				return nil, normalizeErr
			}
			if strings.TrimSpace(selected.team.ID) != "" {
				partnership.TeamID = strings.TrimSpace(selected.team.ID)
			}
			partnership.TeamName = nonEmpty(selected.team.ShortName, selected.team.Name, selected.team.ID, partnership.TeamName)
			partnership.MatchID = nonEmpty(partnership.MatchID, selected.match.ID)
			partnership.InningsID = nonEmpty(partnership.InningsID, fmt.Sprintf("%d", selected.innings.InningsNumber))
			partnership.Period = nonEmpty(partnership.Period, fmt.Sprintf("%d", selected.innings.Period))
			if partnership.Order == 0 {
				partnership.Order = partnership.WicketNumber
			}
			return *partnership, nil
		},
	)
	if err != nil {
		return NewTransportErrorResult(EntityPartnership, selected.innings.PartnershipsRef, err), nil
	}

	warnings = append(selected.warnings, warnings...)
	result := NewListResult(EntityPartnership, items)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityPartnership, items, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// FallOfWicket resolves detailed fall-of-wicket objects for a selected team/innings/period.
func (s *MatchService) FallOfWicket(ctx context.Context, query string, opts MatchInningsOptions) (NormalizedResult, error) {
	selected, passthrough := s.resolveSelectedInnings(ctx, query, opts, true)
	if passthrough != nil {
		passthrough.Kind = EntityFallOfWicket
		return *passthrough, nil
	}
	if strings.TrimSpace(selected.innings.FallOfWicketRef) == "" {
		return NormalizedResult{
			Kind:    EntityFallOfWicket,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("fall-of-wicket route unavailable for team %q innings %d/%d", selected.team.ID, selected.innings.InningsNumber, selected.innings.Period),
		}, nil
	}

	resolved, items, warnings, err := s.fetchDetailedRefCollection(
		ctx,
		selected.innings.FallOfWicketRef,
		func(itemBody []byte) (any, error) {
			fow, normalizeErr := NormalizeFallOfWicket(itemBody)
			if normalizeErr != nil {
				return nil, normalizeErr
			}
			if strings.TrimSpace(selected.team.ID) != "" {
				fow.TeamID = strings.TrimSpace(selected.team.ID)
			}
			fow.TeamName = nonEmpty(selected.team.ShortName, selected.team.Name, selected.team.ID, fow.TeamName)
			fow.MatchID = nonEmpty(fow.MatchID, selected.match.ID)
			fow.InningsID = nonEmpty(fow.InningsID, fmt.Sprintf("%d", selected.innings.InningsNumber))
			fow.Period = nonEmpty(fow.Period, fmt.Sprintf("%d", selected.innings.Period))
			return *fow, nil
		},
	)
	if err != nil {
		return NewTransportErrorResult(EntityFallOfWicket, selected.innings.FallOfWicketRef, err), nil
	}

	warnings = append(selected.warnings, warnings...)
	result := NewListResult(EntityFallOfWicket, items)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityFallOfWicket, items, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Deliveries resolves period statistics into over and wicket timelines for a selected team/innings/period.
func (s *MatchService) Deliveries(ctx context.Context, query string, opts MatchInningsOptions) (NormalizedResult, error) {
	selected, passthrough := s.resolveSelectedInnings(ctx, query, opts, true)
	if passthrough != nil {
		passthrough.Kind = EntityInnings
		return *passthrough, nil
	}
	if strings.TrimSpace(selected.innings.StatisticsRef) == "" {
		return NormalizedResult{
			Kind:    EntityInnings,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("period statistics route unavailable for team %q innings %d/%d", selected.team.ID, selected.innings.InningsNumber, selected.innings.Period),
		}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, selected.innings.StatisticsRef)
	if err != nil {
		return NewTransportErrorResult(EntityInnings, selected.innings.StatisticsRef, err), nil
	}

	overs, wickets, err := NormalizeInningsPeriodStatistics(resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize period statistics %q: %w", resolved.CanonicalRef, err)
	}

	innings := selected.innings
	innings.OverTimeline = overs
	innings.WicketTimeline = wickets

	result := NewDataResult(EntityInnings, innings)
	if len(selected.warnings) > 0 {
		result = NewPartialResult(EntityInnings, innings, selected.warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

func (s *MatchService) listFromEvents(ctx context.Context, opts MatchListOptions, liveOnly bool) (NormalizedResult, error) {
	rootRef := "/events"
	if leagueID := strings.TrimSpace(opts.LeagueID); leagueID != "" {
		rootRef = "/leagues/" + leagueID + "/events"
	}

	resolved, err := s.client.ResolveRefChain(ctx, rootRef)
	if err != nil {
		return NewTransportErrorResult(EntityMatch, rootRef, err), nil
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("decode /events page: %w", err)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultMatchListLimit
	}

	matches := make([]Match, 0, limit)
	warnings := make([]string, 0)
	eventResults := s.fetchEventMatchesConcurrent(ctx, page.Items)
	candidates := make([]*Match, 0, len(page.Items))

	for _, eventResult := range eventResults {
		if len(matches) >= limit && !liveOnly {
			break
		}

		if eventResult.err != nil {
			warnings = append(warnings, fmt.Sprintf("event %s: %v", strings.TrimSpace(eventResult.ref), eventResult.err))
			continue
		}
		warnings = append(warnings, eventResult.warnings...)

		for i := range eventResult.matches {
			match := eventResult.matches[i]
			s.enrichMatchTeamsFromIndex(&match)
			if liveOnly {
				candidates = append(candidates, &match)
				continue
			}

			match.ScoreSummary = matchScoreSummary(match.Teams)
			matches = append(matches, match)
			if len(matches) >= limit {
				break
			}
		}
	}

	if liveOnly {
		warnings = append(warnings, s.hydrateMatchStatusesConcurrent(ctx, candidates)...)
		for _, candidate := range candidates {
			if candidate == nil || !isLiveMatch(*candidate) {
				continue
			}
			candidate.ScoreSummary = matchScoreSummary(candidate.Teams)
			matches = append(matches, *candidate)
			if len(matches) >= limit {
				break
			}
		}
	}

	items := make([]any, 0, len(matches))
	for i := range matches {
		items = append(items, matches[i])
	}

	result := NewListResult(EntityMatch, items)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityMatch, items, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

type eventMatchesResult struct {
	ref      string
	matches  []Match
	warnings []string
	err      error
}

func (s *MatchService) fetchEventMatchesConcurrent(ctx context.Context, refs []Ref) []eventMatchesResult {
	results := make([]eventMatchesResult, len(refs))
	sem := make(chan struct{}, matchListEventFetchConcurrency)
	var wg sync.WaitGroup

	for i, item := range refs {
		wg.Add(1)
		go func(index int, item Ref) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ref := strings.TrimSpace(item.URL)
			if ref == "" {
				results[index] = eventMatchesResult{
					ref: ref,
					err: fmt.Errorf("empty event ref"),
				}
				return
			}

			reqCtx, cancel := context.WithTimeout(ctx, matchListEventFetchTimeout)
			matches, warnings, err := s.matchesFromEventRef(reqCtx, ref)
			cancel()
			results[index] = eventMatchesResult{
				ref:      ref,
				matches:  matches,
				warnings: warnings,
				err:      err,
			}
		}(i, item)
	}

	wg.Wait()
	return results
}

func (s *MatchService) hydrateMatchStatusesConcurrent(ctx context.Context, matches []*Match) []string {
	type statusHydrationResult struct {
		warnings []string
	}

	results := make([]statusHydrationResult, len(matches))
	sem := make(chan struct{}, matchListStatusFetchConcurrency)
	var wg sync.WaitGroup

	for i, match := range matches {
		if match == nil || isLiveMatch(*match) || strings.TrimSpace(match.StatusRef) == "" {
			continue
		}

		wg.Add(1)
		go func(index int, match *Match) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			reqCtx, cancel := context.WithTimeout(ctx, matchListStatusFetchTimeout)
			warnings := s.hydrateMatchStatusOnly(reqCtx, match, map[string]matchStatusSnapshot{})
			cancel()
			results[index] = statusHydrationResult{warnings: warnings}
		}(i, match)
	}

	wg.Wait()

	warnings := make([]string, 0)
	for _, result := range results {
		warnings = append(warnings, result.warnings...)
	}
	return compactWarnings(warnings)
}

func (s *MatchService) lookupMatch(ctx context.Context, query string, opts MatchLookupOptions, statusOnly bool) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, opts)
	if passthrough != nil {
		return *passthrough, nil
	}

	statusCache := map[string]matchStatusSnapshot{}
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}
	warnings := make([]string, 0, len(lookup.warnings)+2)
	warnings = append(warnings, lookup.warnings...)

	hydrationWarnings := s.hydrateMatch(ctx, lookup.match, statusCache, teamCache, scoreCache)
	warnings = append(warnings, hydrationWarnings...)

	if statusOnly {
		lookup.match.Extensions = nil
	}

	result := NewDataResult(EntityMatch, lookup.match)
	if len(warnings) > 0 {
		result = NewPartialResult(EntityMatch, lookup.match, warnings...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	return result, nil
}

type matchLookup struct {
	match    *Match
	resolved *ResolvedDocument
	warnings []string
}

func (s *MatchService) resolveMatchLookup(ctx context.Context, query string, opts MatchLookupOptions) (*matchLookup, *NormalizedResult) {
	query = strings.TrimSpace(query)
	if query == "" {
		result := NormalizedResult{
			Kind:    EntityMatch,
			Status:  ResultStatusEmpty,
			Message: "match query is required",
		}
		return nil, &result
	}

	searchResult, err := s.resolver.Search(ctx, EntityMatch, query, ResolveOptions{
		Limit:    5,
		LeagueID: strings.TrimSpace(opts.LeagueID),
	})
	if err != nil {
		result := NewTransportErrorResult(EntityMatch, query, err)
		return nil, &result
	}
	warnings := append([]string{}, searchResult.Warnings...)
	entity := IndexedEntity{}
	if len(searchResult.Entities) > 0 {
		entity = searchResult.Entities[0]
	} else {
		discovered, discoveryWarnings := s.discoverMatchByTeamQuery(ctx, query, strings.TrimSpace(opts.LeagueID))
		warnings = append(warnings, discoveryWarnings...)
		if discovered == nil {
			result := NormalizedResult{
				Kind:    EntityMatch,
				Status:  ResultStatusEmpty,
				Message: fmt.Sprintf("no matches found for %q", query),
			}
			return nil, &result
		}
		entity = *discovered
	}

	ref := buildMatchRef(entity)
	if ref == "" {
		result := NormalizedResult{
			Kind:    EntityMatch,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("unable to resolve match ref for %q", query),
		}
		return nil, &result
	}

	resolved, err := s.resolveRefChainResilient(ctx, ref)
	if err != nil {
		result := NewTransportErrorResult(EntityMatch, ref, err)
		return nil, &result
	}

	match, err := NormalizeMatch(resolved.Body)
	if err != nil {
		result := NormalizedResult{
			Kind:    EntityMatch,
			Status:  ResultStatusError,
			Message: fmt.Sprintf("normalize competition match %q: %v", resolved.CanonicalRef, err),
		}
		return nil, &result
	}

	return &matchLookup{
		match:    match,
		resolved: resolved,
		warnings: compactWarnings(warnings),
	}, nil
}

func (s *MatchService) discoverMatchByTeamQuery(ctx context.Context, query, leagueID string) (*IndexedEntity, []string) {
	left, right, ok := parseTeamVsQuery(query)
	if !ok {
		return nil, nil
	}

	preferredLeagueID := inferTeamQueryLeagueHint(left, right)
	candidates, err := s.buildTeamQueryEventCandidates(ctx, leagueID, preferredLeagueID)
	if err != nil {
		return nil, []string{fmt.Sprintf("team-query fallback unavailable: %v", err)}
	}
	if len(candidates) == 0 {
		return nil, []string{"team-query fallback found no event candidates"}
	}

	for _, eventID := range candidates {
		reqCtx, cancel := context.WithTimeout(ctx, teamQueryEventFetchTimeout)
		resolved, resolveErr := s.client.ResolveRefChain(reqCtx, "/events/"+eventID)
		cancel()
		if resolveErr != nil {
			continue
		}

		payload, decodeErr := decodePayloadMap(resolved.Body)
		if decodeErr != nil {
			continue
		}
		if !eventMatchesTeamQuery(payload, left, right) {
			continue
		}

		competitions := mapSliceField(payload, "competitions")
		competitionID := ""
		competitionRef := ""
		if len(competitions) > 0 {
			competitionID = stringField(competitions[0], "id")
			competitionRef = stringField(competitions[0], "$ref")
		}

		competitionID = nonEmpty(competitionID, stringField(payload, "id"))
		if competitionID == "" {
			continue
		}

		refIDsMap := refIDs(nonEmpty(competitionRef, resolved.CanonicalRef, resolved.RequestedRef))
		eventIDResolved := nonEmpty(stringField(payload, "id"), refIDsMap["eventId"])
		leagueIDResolved := nonEmpty(refIDsMap["leagueId"], strings.TrimSpace(leagueID))
		if competitionRef == "" && leagueIDResolved != "" && eventIDResolved != "" {
			competitionRef = fmt.Sprintf("/leagues/%s/events/%s/competitions/%s", leagueIDResolved, eventIDResolved, competitionID)
		}
		if competitionRef == "" {
			continue
		}

		entity := IndexedEntity{
			Kind:      EntityMatch,
			ID:        competitionID,
			Ref:       competitionRef,
			Name:      nonEmpty(stringField(payload, "name"), stringField(payload, "shortDescription"), stringField(payload, "description")),
			ShortName: nonEmpty(stringField(payload, "shortName"), stringField(payload, "shortDescription")),
			LeagueID:  leagueIDResolved,
			EventID:   eventIDResolved,
			MatchID:   competitionID,
			Aliases: []string{
				stringField(payload, "name"),
				stringField(payload, "shortName"),
				stringField(payload, "shortDescription"),
				stringField(payload, "description"),
				left,
				right,
				competitionID,
				eventIDResolved,
			},
			UpdatedAt: time.Now().UTC(),
		}
		if s.resolver != nil && s.resolver.index != nil {
			_ = s.resolver.index.Upsert(entity)
		}
		return &entity, []string{fmt.Sprintf("team-query fallback matched event %s", eventIDResolved)}
	}

	return nil, []string{"team-query fallback scanned recent events with no match"}
}

func (s *MatchService) buildTeamQueryEventCandidates(ctx context.Context, leagueID, preferredLeagueID string) ([]string, error) {
	rootRef := "/events"
	if strings.TrimSpace(leagueID) != "" {
		rootRef = "/leagues/" + strings.TrimSpace(leagueID) + "/events"
	}

	seedRefs := make([]Ref, 0)
	if refs, err := s.fetchEventRefs(ctx, rootRef); err == nil {
		seedRefs = append(seedRefs, refs...)
	}
	if len(seedRefs) == 0 && rootRef != "/events" {
		refs, err := s.fetchEventRefs(ctx, "/events")
		if err != nil {
			return nil, err
		}
		seedRefs = append(seedRefs, refs...)
	}

	seen := map[string]struct{}{}
	candidates := make([]string, 0, maxTeamQueryEventCandidates)
	type eventSeed struct {
		id       int
		leagueID string
	}
	seeds := make([]eventSeed, 0, len(seedRefs))
	for _, item := range seedRefs {
		ids := refIDs(item.URL)
		eventID := strings.TrimSpace(ids["eventId"])
		if eventID == "" {
			continue
		}

		seed, err := strconv.Atoi(eventID)
		if err != nil {
			continue
		}
		seeds = append(seeds, eventSeed{id: seed, leagueID: strings.TrimSpace(ids["leagueId"])})
	}

	sort.Slice(seeds, func(i, j int) bool {
		iPref := strings.TrimSpace(preferredLeagueID) != "" && seeds[i].leagueID == strings.TrimSpace(preferredLeagueID)
		jPref := strings.TrimSpace(preferredLeagueID) != "" && seeds[j].leagueID == strings.TrimSpace(preferredLeagueID)
		if iPref != jPref {
			return iPref
		}
		return seeds[i].id > seeds[j].id
	})

	for delta := 0; delta <= matchTeamQueryScanRange; delta++ {
		for _, seed := range seeds {
			down := strconv.Itoa(seed.id - delta)
			if _, ok := seen[down]; !ok {
				seen[down] = struct{}{}
				candidates = append(candidates, down)
				if len(candidates) >= maxTeamQueryEventCandidates {
					return candidates, nil
				}
			}

			if delta == 0 {
				continue
			}
			up := strconv.Itoa(seed.id + delta)
			if _, ok := seen[up]; !ok {
				seen[up] = struct{}{}
				candidates = append(candidates, up)
				if len(candidates) >= maxTeamQueryEventCandidates {
					return candidates, nil
				}
			}
		}
	}

	return candidates, nil
}

func (s *MatchService) fetchEventRefs(ctx context.Context, ref string) ([]Ref, error) {
	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return nil, err
	}
	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func parseTeamVsQuery(query string) (string, string, bool) {
	normalized := normalizeAlias(query)
	if normalized == "" {
		return "", "", false
	}

	separators := []string{" versus ", " vs ", " v "}
	for _, sep := range separators {
		parts := strings.SplitN(normalized, sep, 2)
		if len(parts) != 2 {
			continue
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if left == "" || right == "" {
			return "", "", false
		}
		return left, right, true
	}
	return "", "", false
}

func eventMatchesTeamQuery(payload map[string]any, left, right string) bool {
	parts := []string{
		stringField(payload, "name"),
		stringField(payload, "shortName"),
		stringField(payload, "shortDescription"),
		stringField(payload, "description"),
	}
	for _, competition := range mapSliceField(payload, "competitions") {
		parts = append(parts,
			stringField(competition, "name"),
			stringField(competition, "shortName"),
			stringField(competition, "shortDescription"),
			stringField(competition, "description"),
			stringField(competition, "note"),
		)
	}

	haystack := normalizeAlias(strings.Join(parts, " "))
	return teamQuerySideMatches(haystack, left) && teamQuerySideMatches(haystack, right)
}

func teamQuerySideMatches(haystack, side string) bool {
	if haystack == "" {
		return false
	}
	for _, variant := range teamQueryVariants(side) {
		if variant != "" && strings.Contains(haystack, variant) {
			return true
		}
	}
	return false
}

func teamQueryVariants(side string) []string {
	base := normalizeAlias(side)
	if base == "" {
		return nil
	}

	seen := map[string]struct{}{base: {}}
	variants := []string{base}

	add := func(value string) {
		value = normalizeAlias(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		variants = append(variants, value)
	}

	if strings.Contains(base, "bangalore") {
		add(strings.ReplaceAll(base, "bangalore", "bengaluru"))
	}
	if strings.Contains(base, "bengaluru") {
		add(strings.ReplaceAll(base, "bengaluru", "bangalore"))
	}

	for _, alias := range knownIPLTeamAliases[base] {
		add(alias)
	}

	return variants
}

func inferTeamQueryLeagueHint(left, right string) string {
	left = normalizeAlias(left)
	right = normalizeAlias(right)
	if left == "" || right == "" {
		return ""
	}
	_, leftKnown := knownIPLTeamAliases[left]
	_, rightKnown := knownIPLTeamAliases[right]
	if leftKnown && rightKnown {
		return "8048"
	}
	return ""
}

var knownIPLTeamAliases = map[string][]string{
	"csk":                         {"chennai super kings", "chennai"},
	"chennai super kings":         {"csk", "chennai"},
	"chennai":                     {"csk", "chennai super kings"},
	"dc":                          {"delhi capitals", "delhi"},
	"delhi capitals":              {"dc", "delhi"},
	"delhi":                       {"dc", "delhi capitals"},
	"gt":                          {"gujarat titans", "gujarat"},
	"gujarat titans":              {"gt", "gujarat"},
	"gujarat":                     {"gt", "gujarat titans"},
	"kkr":                         {"kolkata knight riders", "kolkata"},
	"kolkata knight riders":       {"kkr", "kolkata"},
	"kolkata":                     {"kkr", "kolkata knight riders"},
	"lsg":                         {"lucknow super giants", "lucknow"},
	"lucknow super giants":        {"lsg", "lucknow"},
	"lucknow":                     {"lsg", "lucknow super giants"},
	"mi":                          {"mumbai indians", "mumbai"},
	"mumbai indians":              {"mi", "mumbai"},
	"mumbai":                      {"mi", "mumbai indians"},
	"pbks":                        {"punjab kings", "punjab"},
	"punjab kings":                {"pbks", "punjab", "kxip"},
	"punjab":                      {"pbks", "punjab kings", "kxip"},
	"kxip":                        {"pbks", "punjab kings", "punjab"},
	"rcb":                         {"royal challengers bengaluru", "royal challengers bangalore", "bangalore", "bengaluru"},
	"royal challengers bengaluru": {"rcb", "royal challengers bangalore", "bangalore", "bengaluru"},
	"royal challengers bangalore": {"rcb", "royal challengers bengaluru", "bangalore", "bengaluru"},
	"bangalore":                   {"rcb", "royal challengers bengaluru", "royal challengers bangalore", "bengaluru"},
	"bengaluru":                   {"rcb", "royal challengers bengaluru", "royal challengers bangalore", "bangalore"},
	"rr":                          {"rajasthan royals", "rajasthan"},
	"rajasthan royals":            {"rr", "rajasthan"},
	"rajasthan":                   {"rr", "rajasthan royals"},
	"srh":                         {"sunrisers hyderabad", "hyderabad"},
	"sunrisers hyderabad":         {"srh", "hyderabad"},
	"hyderabad":                   {"srh", "sunrisers hyderabad"},
}

func (s *MatchService) deliveryEventsFromRoute(ctx context.Context, ref string, baseWarnings []string) (NormalizedResult, error) {
	resolved, err := s.resolveRefChainResilient(ctx, ref)
	if err != nil {
		return NewTransportErrorResult(EntityDeliveryEvent, ref, err), nil
	}

	pageItems, pageWarnings, err := s.resolvePageRefs(ctx, resolved)
	if err != nil {
		return NormalizedResult{}, err
	}

	events := make([]any, 0, len(pageItems))
	warnings := make([]string, 0, len(baseWarnings))
	warnings = append(warnings, baseWarnings...)
	warnings = append(warnings, pageWarnings...)
	loaded, loadWarnings := s.loadDeliveryEvents(ctx, pageItems)
	warnings = append(warnings, loadWarnings...)
	for _, delivery := range loaded {
		events = append(events, delivery)
	}

	result := NewListResult(EntityDeliveryEvent, events)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityDeliveryEvent, events, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

func (s *MatchService) loadDeliveryEvents(ctx context.Context, refs []Ref) ([]DeliveryEvent, []string) {
	type deliveryLoadResult struct {
		index    int
		delivery *DeliveryEvent
		warning  string
	}

	results := make([]deliveryLoadResult, len(refs))
	sem := make(chan struct{}, deliveryFetchConcurrency)
	var wg sync.WaitGroup
	for i, item := range refs {
		wg.Add(1)
		go func(index int, item Ref) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			itemRef := strings.TrimSpace(item.URL)
			if itemRef == "" {
				results[index] = deliveryLoadResult{index: index, warning: "skip detail item with empty ref"}
				return
			}

			itemCtx, cancel := context.WithTimeout(ctx, detailItemFetchTimeout)
			itemResolved, itemErr := s.resolveRefChainResilient(itemCtx, itemRef)
			cancel()
			if itemErr != nil {
				results[index] = deliveryLoadResult{index: index, warning: fmt.Sprintf("detail %s: %v", itemRef, itemErr)}
				return
			}

			delivery, normalizeErr := NormalizeDeliveryEvent(itemResolved.Body)
			if normalizeErr != nil {
				results[index] = deliveryLoadResult{index: index, warning: fmt.Sprintf("detail %s: %v", itemRef, normalizeErr)}
				return
			}
			results[index] = deliveryLoadResult{index: index, delivery: delivery}
		}(i, item)
	}
	wg.Wait()

	deliveries := make([]DeliveryEvent, 0, len(results))
	warnings := make([]string, 0)
	for _, result := range results {
		if result.warning != "" {
			warnings = append(warnings, result.warning)
			continue
		}
		if result.delivery != nil {
			if isRenderableDelivery(*result.delivery) {
				deliveries = append(deliveries, *result.delivery)
			}
		}
	}
	return deliveries, compactWarnings(warnings)
}

func (s *MatchService) deliveryEventsForMatchRefs(ctx context.Context, primaryRef string, alternateRefs ...string) ([]DeliveryEvent, []string, error) {
	primaryRef = strings.TrimSpace(primaryRef)
	primaryEvents, primaryWarnings, primaryErr := s.loadDeliveryEventsFromRoute(ctx, primaryRef)
	if primaryErr == nil && len(primaryEvents) > 0 && len(primaryWarnings) == 0 {
		return primaryEvents, nil, nil
	}

	merged := append([]DeliveryEvent{}, primaryEvents...)
	warnings := append([]string{}, primaryWarnings...)
	if primaryErr != nil {
		warnings = append(warnings, fmt.Sprintf("delivery route %s: %v", primaryRef, primaryErr))
	}

	for _, ref := range alternateRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" || ref == primaryRef {
			continue
		}
		events, routeWarnings, err := s.loadDeliveryEventsFromRoute(ctx, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("delivery route %s: %v", ref, err))
			continue
		}
		warnings = append(warnings, routeWarnings...)
		merged = append(merged, events...)
	}

	merged = dedupeDeliveryEvents(merged)
	sortDeliveryEvents(merged)
	if len(merged) == 0 && primaryErr != nil {
		return nil, compactWarnings(warnings), primaryErr
	}
	return merged, compactWarnings(warnings), nil
}

func (s *MatchService) loadDeliveryEventsFromRoute(ctx context.Context, ref string) ([]DeliveryEvent, []string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil, fmt.Errorf("empty delivery route")
	}
	resolved, err := s.resolveRefChainResilient(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	pageItems, pageWarnings, err := s.resolvePageRefs(ctx, resolved)
	if err != nil {
		return nil, nil, err
	}
	loaded, loadWarnings := s.loadDeliveryEvents(ctx, pageItems)
	warnings := make([]string, 0, len(pageWarnings)+len(loadWarnings))
	warnings = append(warnings, pageWarnings...)
	warnings = append(warnings, loadWarnings...)
	return loaded, compactWarnings(warnings), nil
}

func (s *MatchService) loadRecentDeliveryEventsFromRoute(ctx context.Context, ref string, maxItems int) ([]DeliveryEvent, []string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil, fmt.Errorf("empty delivery route")
	}
	resolved, err := s.resolveRefChainResilient(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	pageItems, pageWarnings, err := s.resolvePageRefs(ctx, resolved)
	if err != nil {
		return nil, nil, err
	}
	if maxItems > 0 && len(pageItems) > maxItems {
		pageItems = append([]Ref(nil), pageItems[len(pageItems)-maxItems:]...)
	}
	loaded, loadWarnings := s.loadDeliveryEvents(ctx, pageItems)
	warnings := make([]string, 0, len(pageWarnings)+len(loadWarnings))
	warnings = append(warnings, pageWarnings...)
	warnings = append(warnings, loadWarnings...)
	return loaded, compactWarnings(warnings), nil
}

func isRenderableDelivery(delivery DeliveryEvent) bool {
	if delivery.OverNumber > 0 && delivery.BallNumber > 0 {
		return true
	}
	text := strings.TrimSpace(firstNonEmpty(delivery.ShortText, delivery.Text))
	if text == "" || text == "/" || text == "-" {
		return false
	}
	lowered := strings.ToLower(text)
	return !strings.Contains(lowered, "over - ball")
}

func sortDeliveryEvents(deliveries []DeliveryEvent) {
	sort.SliceStable(deliveries, func(i, j int) bool {
		if deliveries[i].Period != deliveries[j].Period {
			return deliveries[i].Period < deliveries[j].Period
		}
		if deliveries[i].OverNumber != deliveries[j].OverNumber {
			return deliveries[i].OverNumber < deliveries[j].OverNumber
		}
		if deliveries[i].BallNumber != deliveries[j].BallNumber {
			return deliveries[i].BallNumber < deliveries[j].BallNumber
		}
		if deliveries[i].Sequence != deliveries[j].Sequence {
			return deliveries[i].Sequence < deliveries[j].Sequence
		}
		return deliveries[i].BBBTimestamp < deliveries[j].BBBTimestamp
	})
}

func dedupeDeliveryEvents(deliveries []DeliveryEvent) []DeliveryEvent {
	if len(deliveries) <= 1 {
		return deliveries
	}
	seen := map[string]int{}
	out := make([]DeliveryEvent, 0, len(deliveries))
	for _, delivery := range deliveries {
		key := strings.TrimSpace(delivery.ID)
		if key == "" {
			key = fmt.Sprintf("%d|%d|%d|%d|%s|%s|%s",
				delivery.Period,
				delivery.OverNumber,
				delivery.BallNumber,
				delivery.Sequence,
				strings.TrimSpace(delivery.ShortText),
				strings.TrimSpace(delivery.HomeScore),
				strings.TrimSpace(delivery.AwayScore),
			)
		}
		if idx, ok := seen[key]; ok {
			if delivery.Sequence > out[idx].Sequence || delivery.BBBTimestamp > out[idx].BBBTimestamp {
				out[idx] = delivery
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, delivery)
	}
	return out
}

type selectedInningsContext struct {
	match    Match
	team     Team
	innings  Innings
	warnings []string
}

func (s *MatchService) resolveSelectedInnings(
	ctx context.Context,
	query string,
	opts MatchInningsOptions,
	requireTeam bool,
) (*selectedInningsContext, *NormalizedResult) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, MatchLookupOptions{LeagueID: opts.LeagueID})
	if passthrough != nil {
		return nil, passthrough
	}
	statusCache := map[string]matchStatusSnapshot{}
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}
	lookup.warnings = append(lookup.warnings, s.hydrateMatch(ctx, lookup.match, statusCache, teamCache, scoreCache)...)

	teamQuery := strings.TrimSpace(opts.TeamQuery)
	if requireTeam && teamQuery == "" {
		result := NormalizedResult{
			Kind:    EntityInnings,
			Status:  ResultStatusEmpty,
			Message: "--team is required",
		}
		return nil, &result
	}

	teams, teamWarnings, teamResult := s.selectTeamsFromMatch(ctx, *lookup.match, teamQuery, opts.LeagueID)
	if teamResult != nil {
		return nil, teamResult
	}
	if len(teams) == 0 {
		result := NormalizedResult{
			Kind:    EntityInnings,
			Status:  ResultStatusEmpty,
			Message: "no teams available for match selection",
		}
		return nil, &result
	}

	team := teams[0]
	inningsList, _, inningsWarnings := s.fetchTeamInnings(ctx, *lookup.match, team)
	if len(inningsWarnings) > 0 {
		teamWarnings = append(teamWarnings, inningsWarnings...)
	}

	if len(inningsList) == 0 {
		result := NormalizedResult{
			Kind:    EntityInnings,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("no innings found for team %q", team.ID),
		}
		return nil, &result
	}

	requestedInnings := opts.Innings
	requestedPeriod := opts.Period
	if requestedInnings <= 0 || requestedPeriod <= 0 {
		result := NormalizedResult{
			Kind:    EntityInnings,
			Status:  ResultStatusEmpty,
			Message: "--innings and --period are required",
		}
		return nil, &result
	}

	var selected *Innings
	for i := range inningsList {
		if inningsList[i].InningsNumber == requestedInnings && inningsList[i].Period == requestedPeriod {
			candidate := inningsList[i]
			if strings.TrimSpace(team.ID) != "" {
				candidate.TeamID = strings.TrimSpace(team.ID)
			}
			candidate.TeamName = nonEmpty(team.ShortName, team.Name, team.ID, candidate.TeamName)
			candidate.MatchID = nonEmpty(candidate.MatchID, lookup.match.ID)
			candidate.CompetitionID = nonEmpty(candidate.CompetitionID, lookup.match.CompetitionID, lookup.match.ID)
			candidate.EventID = nonEmpty(candidate.EventID, lookup.match.EventID)
			candidate.LeagueID = nonEmpty(candidate.LeagueID, lookup.match.LeagueID)
			selected = &candidate
			break
		}
	}

	if selected == nil {
		result := NormalizedResult{
			Kind:   EntityInnings,
			Status: ResultStatusEmpty,
			Message: fmt.Sprintf(
				"requested innings/period %d/%d was not found; available: %s",
				requestedInnings,
				requestedPeriod,
				availableInningsPeriods(inningsList),
			),
		}
		return nil, &result
	}

	warnings := append([]string{}, lookup.warnings...)
	warnings = append(warnings, teamWarnings...)
	return &selectedInningsContext{
		match:    *lookup.match,
		team:     team,
		innings:  *selected,
		warnings: compactWarnings(warnings),
	}, nil
}

func (s *MatchService) selectTeamsFromMatch(
	ctx context.Context,
	match Match,
	teamQuery string,
	leagueID string,
) ([]Team, []string, *NormalizedResult) {
	teamQuery = strings.TrimSpace(teamQuery)
	if teamQuery == "" {
		teams := make([]Team, 0, len(match.Teams))
		for _, team := range match.Teams {
			if strings.TrimSpace(team.ID) == "" {
				continue
			}
			teams = append(teams, team)
		}
		if len(teams) == 0 {
			result := NormalizedResult{
				Kind:    EntityInnings,
				Status:  ResultStatusEmpty,
				Message: "no teams available in match competitors",
			}
			return nil, nil, &result
		}
		return teams, nil, nil
	}

	if direct := findTeamInMatch(match, teamQuery); direct != nil {
		return []Team{*direct}, nil, nil
	}

	searchResult, err := s.resolver.Search(ctx, EntityTeam, teamQuery, ResolveOptions{
		Limit:    5,
		LeagueID: strings.TrimSpace(leagueID),
		MatchID:  strings.TrimSpace(match.ID),
	})
	if err != nil {
		result := NewTransportErrorResult(EntityTeam, teamQuery, err)
		return nil, nil, &result
	}

	for _, entity := range searchResult.Entities {
		if found := matchTeamByID(match, entity.ID); found != nil {
			return []Team{*found}, searchResult.Warnings, nil
		}
	}

	result := NormalizedResult{
		Kind:    EntityTeam,
		Status:  ResultStatusEmpty,
		Message: fmt.Sprintf("team %q not found in match; available: %s", teamQuery, availableMatchTeams(match)),
	}
	return nil, searchResult.Warnings, &result
}

func (s *MatchService) fetchTeamInnings(ctx context.Context, match Match, team Team) ([]Innings, string, []string) {
	candidates := compactWarnings([]string{
		strings.TrimSpace(team.LinescoresRef),
		strings.TrimSpace(competitorSubresourceRef(match, team.ID, "linescores")),
	})
	if len(candidates) == 0 {
		return []Innings{}, "", []string{fmt.Sprintf("linescores route unavailable for team %q", team.ID)}
	}

	seen := map[string]struct{}{}
	warnings := make([]string, 0)
	for _, ref := range candidates {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}

		resolved, err := s.client.ResolveRefChain(ctx, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("linescores %s: %v", ref, err))
			continue
		}

		innings, collectWarnings, collectErr := s.collectInningsFromPayload(ctx, resolved.Body)
		warnings = append(warnings, collectWarnings...)
		if collectErr != nil {
			warnings = append(warnings, fmt.Sprintf("linescores %s: %v", ref, collectErr))
			continue
		}

		for i := range innings {
			if strings.TrimSpace(team.ID) != "" {
				innings[i].TeamID = strings.TrimSpace(team.ID)
			}
			innings[i].TeamName = nonEmpty(team.ShortName, team.Name, team.ID, innings[i].TeamName)
			innings[i].MatchID = nonEmpty(innings[i].MatchID, match.ID)
			innings[i].CompetitionID = nonEmpty(innings[i].CompetitionID, match.CompetitionID, match.ID)
			innings[i].EventID = nonEmpty(innings[i].EventID, match.EventID)
			innings[i].LeagueID = nonEmpty(innings[i].LeagueID, match.LeagueID)
			if scopedRef := inningsSubresourceRef(match, team.ID, innings[i].InningsNumber, innings[i].Period, "statistics/0"); scopedRef != "" {
				innings[i].StatisticsRef = scopedRef
			}
			if scopedRef := inningsSubresourceRef(match, team.ID, innings[i].InningsNumber, innings[i].Period, "partnerships"); scopedRef != "" {
				innings[i].PartnershipsRef = scopedRef
			}
			if scopedRef := inningsSubresourceRef(match, team.ID, innings[i].InningsNumber, innings[i].Period, "fow"); scopedRef != "" {
				innings[i].FallOfWicketRef = scopedRef
			}
		}

		if len(innings) > 0 {
			return innings, resolved.CanonicalRef, compactWarnings(warnings)
		}
	}

	return []Innings{}, "", compactWarnings(warnings)
}

func (s *MatchService) collectInningsFromPayload(ctx context.Context, body []byte) ([]Innings, []string, error) {
	payload, err := decodePayloadMap(body)
	if err != nil {
		return nil, nil, err
	}

	warnings := make([]string, 0)
	innings := make([]Innings, 0)

	appendInningsMap := func(item map[string]any) {
		if item == nil {
			return
		}
		if stringField(item, "$ref") == "" && intField(item, "period") == 0 && intField(item, "runs") == 0 && intField(item, "wickets") == 0 && stringField(item, "score") == "" {
			return
		}
		innings = append(innings, *normalizeInningsFromMap(item))
	}

	items := mapSliceField(payload, "items")
	if len(items) > 0 {
		for _, item := range items {
			itemRef := strings.TrimSpace(stringField(item, "$ref"))
			if itemRef != "" && intField(item, "period") == 0 && stringField(item, "score") == "" && intField(item, "runs") == 0 && intField(item, "wickets") == 0 {
				resolved, itemErr := s.client.ResolveRefChain(ctx, itemRef)
				if itemErr != nil {
					warnings = append(warnings, fmt.Sprintf("innings %s: %v", itemRef, itemErr))
					continue
				}
				normalized, normalizeErr := NormalizeInnings(resolved.Body)
				if normalizeErr != nil {
					warnings = append(warnings, fmt.Sprintf("innings %s: %v", itemRef, normalizeErr))
					continue
				}
				innings = append(innings, *normalized)
				continue
			}
			appendInningsMap(item)
		}
		return innings, compactWarnings(warnings), nil
	}

	appendInningsMap(payload)
	return innings, compactWarnings(warnings), nil
}

func (s *MatchService) hydrateInningsTimelines(ctx context.Context, innings *Innings) []string {
	if innings == nil || strings.TrimSpace(innings.StatisticsRef) == "" {
		return nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, innings.StatisticsRef)
	if err != nil {
		return []string{fmt.Sprintf("period statistics %s: %v", innings.StatisticsRef, err)}
	}

	overs, wickets, err := NormalizeInningsPeriodStatistics(resolved.Body)
	if err != nil {
		return []string{fmt.Sprintf("period statistics %s: %v", resolved.CanonicalRef, err)}
	}
	innings.OverTimeline = overs
	innings.WicketTimeline = wickets
	return nil
}

func (s *MatchService) fetchDetailedRefCollection(
	ctx context.Context,
	ref string,
	normalize func(itemBody []byte) (any, error),
) (*ResolvedDocument, []any, []string, error) {
	resolved, err := s.resolveRefChainResilient(ctx, ref)
	if err != nil {
		return nil, nil, nil, err
	}

	pageItems, warnings, err := s.resolvePageRefs(ctx, resolved)
	if err != nil {
		return nil, nil, nil, err
	}

	type normalizedItemResult struct {
		index   int
		item    any
		warning string
	}

	results := make([]normalizedItemResult, len(pageItems))
	sem := make(chan struct{}, detailSubresourceFetchConcurrency)
	var wg sync.WaitGroup
	for i, item := range pageItems {
		wg.Add(1)
		go func(index int, item Ref) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			itemRef := strings.TrimSpace(item.URL)
			if itemRef == "" {
				results[index] = normalizedItemResult{index: index, warning: "skip item with empty ref"}
				return
			}

			itemCtx, cancel := context.WithTimeout(ctx, detailItemFetchTimeout)
			itemResolved, itemErr := s.resolveRefChainResilient(itemCtx, itemRef)
			cancel()
			if itemErr != nil {
				results[index] = normalizedItemResult{index: index, warning: fmt.Sprintf("item %s: %v", itemRef, itemErr)}
				return
			}

			normalized, normalizeErr := normalize(itemResolved.Body)
			if normalizeErr != nil {
				results[index] = normalizedItemResult{index: index, warning: fmt.Sprintf("item %s: %v", itemRef, normalizeErr)}
				return
			}
			results[index] = normalizedItemResult{index: index, item: normalized}
		}(i, item)
	}
	wg.Wait()

	items := make([]any, 0, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.warning) != "" {
			warnings = append(warnings, result.warning)
			continue
		}
		if result.item != nil {
			items = append(items, result.item)
		}
	}

	return resolved, items, compactWarnings(warnings), nil
}

func (s *MatchService) resolvePageRefs(ctx context.Context, first *ResolvedDocument) ([]Ref, []string, error) {
	if first == nil {
		return nil, nil, fmt.Errorf("resolved page is nil")
	}
	page, err := DecodePage[Ref](first.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("decode page %q: %w", first.CanonicalRef, err)
	}

	items := append([]Ref(nil), page.Items...)
	if page.PageCount <= 1 {
		return items, nil, nil
	}

	type pageLoadResult struct {
		page    int
		items   []Ref
		warning string
	}

	results := make([]pageLoadResult, page.PageCount+1)
	sem := make(chan struct{}, detailSubresourceFetchConcurrency)
	var wg sync.WaitGroup
	baseRef := firstNonEmptyString(first.CanonicalRef, first.RequestedRef)
	for pageIndex := 2; pageIndex <= page.PageCount; pageIndex++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pageRef := pagedRef(baseRef, index)
			if pageRef == "" {
				results[index] = pageLoadResult{page: index, warning: fmt.Sprintf("page %d unavailable for %s", index, baseRef)}
				return
			}

			pageDoc, pageErr := s.resolveRefChainResilient(ctx, pageRef)
			if pageErr != nil {
				results[index] = pageLoadResult{page: index, warning: fmt.Sprintf("page %d %s: %v", index, pageRef, pageErr)}
				return
			}

			nextPage, decodeErr := DecodePage[Ref](pageDoc.Body)
			if decodeErr != nil {
				results[index] = pageLoadResult{page: index, warning: fmt.Sprintf("page %d %s: %v", index, pageDoc.CanonicalRef, decodeErr)}
				return
			}
			results[index] = pageLoadResult{page: index, items: nextPage.Items}
		}(pageIndex)
	}
	wg.Wait()

	warnings := make([]string, 0)
	for pageIndex := 2; pageIndex <= page.PageCount; pageIndex++ {
		result := results[pageIndex]
		if strings.TrimSpace(result.warning) != "" {
			warnings = append(warnings, result.warning)
			continue
		}
		if len(result.items) > 0 {
			items = append(items, result.items...)
		}
	}

	return items, compactWarnings(warnings), nil
}

func pagedRef(ref string, page int) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || page <= 1 {
		return ref
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		separator := "?"
		if strings.Contains(ref, "?") {
			separator = "&"
		}
		return ref + separator + "page=" + strconv.Itoa(page)
	}
	query := parsed.Query()
	query.Set("page", strconv.Itoa(page))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (s *MatchService) resolveRefChainResilient(ctx context.Context, ref string) (*ResolvedDocument, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resolved, err := s.client.ResolveRefChain(ctx, ref)
		if err == nil {
			return resolved, nil
		}
		lastErr = err
		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) && !strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
			break
		}
		if statusErr != nil && statusErr.StatusCode != 503 {
			break
		}
	}
	return nil, lastErr
}

func availableInningsPeriods(innings []Innings) string {
	if len(innings) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(innings))
	seen := map[string]struct{}{}
	for _, item := range innings {
		if item.InningsNumber == 0 || item.Period == 0 {
			continue
		}
		label := fmt.Sprintf("%d/%d", item.InningsNumber, item.Period)
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func findTeamInMatch(match Match, query string) *Team {
	query = normalizeAlias(query)
	queryTokens := strings.Fields(query)
	if query == "" {
		return nil
	}

	bestScore := 0
	var best *Team
	for i := range match.Teams {
		candidate := &match.Teams[i]
		values := []string{
			strings.TrimSpace(candidate.ID),
			strings.TrimSpace(candidate.Name),
			strings.TrimSpace(candidate.ShortName),
			strings.TrimSpace(candidate.Abbreviation),
			strings.TrimSpace(refIDs(candidate.Ref)["teamId"]),
			strings.TrimSpace(refIDs(candidate.Ref)["competitorId"]),
		}
		for _, value := range values {
			normalized := normalizeAlias(value)
			if normalized == "" {
				continue
			}
			score := aliasMatchScore(normalized, query, queryTokens)
			if score > bestScore {
				bestScore = score
				best = candidate
			}
		}
	}

	if bestScore >= 300 {
		return best
	}
	return nil
}

func availableMatchTeams(match Match) string {
	parts := make([]string, 0, len(match.Teams))
	for _, team := range match.Teams {
		name := nonEmpty(team.ShortName, team.Name, team.ID)
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func inningsSubresourceRef(match Match, teamID string, innings, period int, suffix string) string {
	base := competitorSubresourceRef(match, teamID, "")
	if base == "" || innings <= 0 || period <= 0 {
		return ""
	}

	suffix = strings.Trim(strings.TrimSpace(suffix), "/")
	ref := fmt.Sprintf("%s/linescores/%d/%d", strings.TrimRight(base, "/"), innings, period)
	if suffix != "" {
		ref += "/" + suffix
	}
	return ref
}

func (s *MatchService) matchesFromEventRef(ctx context.Context, ref string) ([]Match, []string, error) {
	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return nil, nil, err
	}

	matches, err := NormalizeMatchesFromEvent(resolved.Body)
	if err != nil {
		return nil, nil, err
	}

	return matches, nil, nil
}

func (s *MatchService) hydrateMatch(
	ctx context.Context,
	match *Match,
	statusCache map[string]matchStatusSnapshot,
	teamCache map[string]teamIdentity,
	scoreCache map[string]string,
) []string {
	if match == nil {
		return nil
	}

	warnings := make([]string, 0)

	if statusRef := strings.TrimSpace(match.StatusRef); statusRef != "" {
		snapshot, err := s.fetchStatus(ctx, statusRef, statusCache)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("status %s: %v", statusRef, err))
		} else {
			match.MatchState = nonEmpty(match.MatchState, snapshot.stateSummary())
			if strings.TrimSpace(match.Note) == "" {
				match.Note = snapshot.longSummary
			}
			if match.Extensions == nil {
				match.Extensions = map[string]any{}
			}
			match.Extensions["statusState"] = snapshot.state
			match.Extensions["statusDetail"] = snapshot.detail
			match.Extensions["statusShortDetail"] = snapshot.shortDetail
		}
	}

	for i := range match.Teams {
		team := &match.Teams[i]

		if strings.TrimSpace(team.Name) == "" || strings.TrimSpace(team.ShortName) == "" {
			identity, err := s.fetchTeamIdentity(ctx, team, teamCache)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("team %s: %v", nonEmpty(team.Ref, team.ID), err))
			} else {
				if team.Name == "" {
					team.Name = identity.name
				}
				if team.ShortName == "" {
					team.ShortName = identity.shortName
				}
			}
		}

		if strings.TrimSpace(team.ScoreSummary) == "" && strings.TrimSpace(team.ScoreRef) != "" {
			score, err := s.fetchTeamScore(ctx, team.ScoreRef, scoreCache)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("score %s: %v", team.ScoreRef, err))
			} else {
				team.ScoreSummary = score
			}
		}
	}

	match.ScoreSummary = matchScoreSummary(match.Teams)
	return compactWarnings(warnings)
}

func (s *MatchService) hydrateMatchStatusOnly(
	ctx context.Context,
	match *Match,
	statusCache map[string]matchStatusSnapshot,
) []string {
	if match == nil {
		return nil
	}
	statusRef := strings.TrimSpace(match.StatusRef)
	if statusRef == "" {
		return nil
	}
	snapshot, err := s.fetchStatus(ctx, statusRef, statusCache)
	if err != nil {
		return []string{fmt.Sprintf("status %s: %v", statusRef, err)}
	}
	match.MatchState = nonEmpty(match.MatchState, snapshot.stateSummary())
	if strings.TrimSpace(match.Note) == "" {
		match.Note = snapshot.longSummary
	}
	if match.Extensions == nil {
		match.Extensions = map[string]any{}
	}
	match.Extensions["statusState"] = snapshot.state
	match.Extensions["statusDetail"] = snapshot.detail
	match.Extensions["statusShortDetail"] = snapshot.shortDetail
	return nil
}

func (s *MatchService) enrichMatchTeamsFromIndex(match *Match) {
	if match == nil || s == nil || s.resolver == nil || s.resolver.index == nil {
		return
	}
	for i := range match.Teams {
		team := &match.Teams[i]
		teamID := strings.TrimSpace(team.ID)
		if teamID == "" {
			teamID = strings.TrimSpace(refIDs(team.Ref)["teamId"])
		}
		if teamID == "" {
			teamID = strings.TrimSpace(refIDs(team.Ref)["competitorId"])
		}
		if teamID == "" {
			continue
		}
		cached, ok := s.resolver.index.FindByID(EntityTeam, teamID)
		if !ok {
			continue
		}
		if strings.TrimSpace(team.Name) == "" {
			team.Name = strings.TrimSpace(cached.Name)
		}
		if strings.TrimSpace(team.ShortName) == "" {
			team.ShortName = strings.TrimSpace(cached.ShortName)
		}
		if strings.TrimSpace(team.Ref) == "" {
			team.Ref = strings.TrimSpace(cached.Ref)
		}
	}
}

func (s *MatchService) fetchStatus(ctx context.Context, ref string, cache map[string]matchStatusSnapshot) (matchStatusSnapshot, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return matchStatusSnapshot{}, fmt.Errorf("status ref is empty")
	}
	if cached, ok := cache[ref]; ok {
		return cached, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return matchStatusSnapshot{}, err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return matchStatusSnapshot{}, err
	}

	typed := mapField(payload, "type")
	status := matchStatusSnapshot{
		summary:     stringField(payload, "summary"),
		longSummary: stringField(payload, "longSummary"),
		state:       stringField(typed, "state"),
		detail:      stringField(typed, "detail"),
		shortDetail: stringField(typed, "shortDetail"),
		description: stringField(typed, "description"),
	}
	cache[ref] = status
	return status, nil
}

func (s *MatchService) fetchTeamIdentity(ctx context.Context, team *Team, cache map[string]teamIdentity) (teamIdentity, error) {
	if team == nil {
		return teamIdentity{}, fmt.Errorf("team is nil")
	}

	ref := strings.TrimSpace(team.Ref)
	if ref == "" && strings.TrimSpace(team.ID) != "" {
		ref = "/teams/" + strings.TrimSpace(team.ID)
	}
	if ref == "" {
		return teamIdentity{}, fmt.Errorf("team ref is empty")
	}

	if cached, ok := cache[ref]; ok {
		return cached, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return teamIdentity{}, err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return teamIdentity{}, err
	}

	identity := teamIdentity{
		name:      nonEmpty(stringField(payload, "displayName"), stringField(payload, "name")),
		shortName: nonEmpty(stringField(payload, "shortDisplayName"), stringField(payload, "shortName"), stringField(payload, "abbreviation")),
	}
	if identity.name == "" && strings.TrimSpace(team.ID) != "" {
		identity.name = team.ID
	}
	cache[ref] = identity
	return identity, nil
}

func (s *MatchService) fetchTeamScore(ctx context.Context, ref string, cache map[string]string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("score ref is empty")
	}
	if cached, ok := cache[ref]; ok {
		return cached, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return "", err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return "", err
	}

	score := nonEmpty(stringField(payload, "displayValue"), stringField(payload, "value"), stringField(payload, "summary"))
	cache[ref] = score
	return score, nil
}

func buildMatchRef(entity IndexedEntity) string {
	if strings.TrimSpace(entity.Ref) != "" {
		return entity.Ref
	}

	leagueID := strings.TrimSpace(entity.LeagueID)
	eventID := strings.TrimSpace(entity.EventID)
	matchID := strings.TrimSpace(entity.ID)
	if leagueID == "" || eventID == "" || matchID == "" {
		return ""
	}

	return fmt.Sprintf("/leagues/%s/events/%s/competitions/%s", leagueID, eventID, matchID)
}

func matchSubresourceRef(match Match, extensionKey, suffix string) string {
	extensionKey = strings.TrimSpace(extensionKey)
	suffix = strings.Trim(strings.TrimSpace(suffix), "/")

	if extensionKey != "" {
		if ref := extensionRef(match.Extensions, extensionKey); ref != "" {
			return ref
		}
	}

	base := strings.TrimSpace(match.Ref)
	if base != "" {
		if suffix == "" {
			return base
		}
		return strings.TrimRight(base, "/") + "/" + suffix
	}

	leagueID := strings.TrimSpace(match.LeagueID)
	eventID := strings.TrimSpace(match.EventID)
	competitionID := strings.TrimSpace(match.CompetitionID)
	if competitionID == "" {
		competitionID = strings.TrimSpace(match.ID)
	}
	if leagueID == "" || eventID == "" || competitionID == "" {
		return ""
	}

	base = fmt.Sprintf("/leagues/%s/events/%s/competitions/%s", leagueID, eventID, competitionID)
	if suffix == "" {
		return base
	}
	return base + "/" + suffix
}

func extensionRef(extensions map[string]any, key string) string {
	if extensions == nil {
		return ""
	}
	raw, ok := extensions[key]
	if !ok || raw == nil {
		return ""
	}
	refMap, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringField(refMap, "$ref"))
}

func (s *MatchService) buildLiveView(ctx context.Context, match Match) (*MatchLiveView, []string) {
	detailsRef := nonEmpty(strings.TrimSpace(match.DetailsRef), matchSubresourceRef(match, "details", "details"))
	playsRef := matchSubresourceRef(match, "plays", "plays")
	if detailsRef == "" && playsRef == "" {
		return nil, nil
	}
	primaryRef := nonEmpty(detailsRef, playsRef)
	deliveries, warnings, err := s.loadRecentDeliveryEventsFromRoute(ctx, primaryRef, liveViewRecentDeliveryFetchCount)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("live deliveries %s: %v", primaryRef, err))
		deliveries = nil
	}
	if len(deliveries) < 6 || len(warnings) > 0 {
		alternate := strings.TrimSpace(playsRef)
		if alternate != "" && alternate != primaryRef {
			altDeliveries, altWarnings, altErr := s.loadRecentDeliveryEventsFromRoute(ctx, alternate, liveViewRecentDeliveryFetchCount)
			if altErr != nil {
				warnings = append(warnings, fmt.Sprintf("live deliveries %s: %v", alternate, altErr))
			} else if len(altDeliveries) > 0 {
				deliveries = append(deliveries, altDeliveries...)
				warnings = append(warnings, altWarnings...)
			}
		}
	}
	deliveries = dedupeDeliveryEvents(deliveries)
	sortDeliveryEvents(deliveries)
	if len(deliveries) == 0 {
		return nil, warnings
	}
	latest := deliveries[len(deliveries)-1]

	nameMap := map[string]string{}
	for _, event := range deliveries {
		bowlerName, batsmanName := parseNamesFromDeliveryShortText(event.ShortText)
		if strings.TrimSpace(event.BowlerPlayerID) != "" && strings.TrimSpace(bowlerName) != "" {
			if _, ok := nameMap[event.BowlerPlayerID]; !ok {
				nameMap[event.BowlerPlayerID] = bowlerName
			}
		}
		if strings.TrimSpace(event.BatsmanPlayerID) != "" && strings.TrimSpace(batsmanName) != "" {
			if _, ok := nameMap[event.BatsmanPlayerID]; !ok {
				nameMap[event.BatsmanPlayerID] = batsmanName
			}
		}
	}
	score := firstNonEmpty(matchScoreLabel(latest.HomeScore), matchScoreLabel(latest.AwayScore))
	if score == "" {
		score = match.ScoreSummary
	}

	battingTeam := teamLabelByID(match, latest.TeamID)
	bowlingTeam := otherTeamLabelByID(match, latest.TeamID)
	if battingTeam == "" {
		battingTeam = firstNonEmpty(matchTeamsLabelFromMatch(match), latest.TeamID)
	}

	batters := make([]LiveBatterView, 0, 2)
	if latest.BatsmanPlayerID != "" {
		batters = append(batters, LiveBatterView{
			PlayerID:   latest.BatsmanPlayerID,
			PlayerName: firstNonEmpty(nameMap[latest.BatsmanPlayerID], latest.BatsmanPlayerID),
			Runs:       latest.BatsmanTotalRuns,
			Balls:      latest.BatsmanBalls,
			Fours:      latest.BatsmanFours,
			Sixes:      latest.BatsmanSixes,
			StrikeRate: strikeRate(latest.BatsmanTotalRuns, latest.BatsmanBalls),
			OnStrike:   true,
		})
	}
	if latest.OtherBatsmanID != "" {
		batters = append(batters, LiveBatterView{
			PlayerID:   latest.OtherBatsmanID,
			PlayerName: firstNonEmpty(nameMap[latest.OtherBatsmanID], latest.OtherBatsmanID),
			Runs:       latest.OtherBatterRuns,
			Balls:      latest.OtherBatterBalls,
			Fours:      latest.OtherBatterFours,
			Sixes:      latest.OtherBatterSixes,
			StrikeRate: strikeRate(latest.OtherBatterRuns, latest.OtherBatterBalls),
		})
	}

	bowlers := make([]LiveBowlerView, 0, 2)
	if latest.BowlerPlayerID != "" {
		bowlers = append(bowlers, LiveBowlerView{
			PlayerID:   latest.BowlerPlayerID,
			PlayerName: firstNonEmpty(nameMap[latest.BowlerPlayerID], latest.BowlerPlayerID),
			Overs:      latest.BowlerOvers,
			Balls:      latest.BowlerBalls,
			Maidens:    latest.BowlerMaidens,
			Conceded:   latest.BowlerConceded,
			Wickets:    latest.BowlerWickets,
			Economy:    economy(latest.BowlerConceded, latest.BowlerBalls, latest.BowlerOvers),
		})
	}
	if latest.OtherBowlerID != "" && latest.OtherBowlerID != latest.BowlerPlayerID {
		bowlers = append(bowlers, LiveBowlerView{
			PlayerID:   latest.OtherBowlerID,
			PlayerName: firstNonEmpty(nameMap[latest.OtherBowlerID], latest.OtherBowlerID),
			Overs:      latest.OtherBowlerOvers,
			Balls:      latest.OtherBowlerBalls,
			Maidens:    latest.OtherBowlerMaidens,
			Conceded:   latest.OtherBowlerConceded,
			Wickets:    latest.OtherBowlerWickets,
			Economy:    economy(latest.OtherBowlerConceded, latest.OtherBowlerBalls, latest.OtherBowlerOvers),
		})
	}

	startRecent := len(deliveries) - 6
	if startRecent < 0 {
		startRecent = 0
	}
	recent := append([]DeliveryEvent(nil), deliveries[startRecent:]...)
	currentOver := make([]DeliveryEvent, 0, 6)
	for _, event := range deliveries {
		if event.OverNumber == latest.OverNumber && event.Period == latest.Period {
			currentOver = append(currentOver, event)
		}
	}

	view := &MatchLiveView{
		Fixture:      nonEmpty(match.ShortDescription, match.Description),
		Status:       nonEmpty(match.MatchState, match.Note),
		Score:        score,
		Overs:        overBallString(latest.OverNumber, latest.BallNumber),
		CurrentOver:  latest.OverNumber,
		BallInOver:   latest.BallNumber,
		BattingTeam:  battingTeam,
		BowlingTeam:  bowlingTeam,
		Batters:      batters,
		Bowlers:      bowlers,
		RecentBalls:  recent,
		CurrentBalls: currentOver,
		LastDetailID: strings.TrimSpace(latest.ID),
		LastUpdateMS: latest.BBBTimestamp,
		SnapshotAt:   time.Now().UTC().Format(time.RFC3339),
		SourceRoute:  primaryRef,
	}
	stale, reason := detectLiveStaleness(match.ScoreSummary, score)
	view.Stale = stale
	view.StaleReason = reason
	return view, compactWarnings(warnings)
}

func augmentScorecardFromLive(scorecard *MatchScorecard, live *MatchLiveView) {
	if scorecard == nil || live == nil {
		return
	}

	if len(scorecard.BattingCards) == 0 && len(live.Batters) > 0 {
		card := BattingCard{
			InningsNumber: 1,
			TeamName:      live.BattingTeam,
			Runs:          live.Score,
			Players:       make([]BattingCardEntry, 0, len(live.Batters)),
		}
		for _, batter := range live.Batters {
			card.Players = append(card.Players, BattingCardEntry{
				PlayerID:   batter.PlayerID,
				PlayerName: batter.PlayerName,
				Runs:       strconv.Itoa(batter.Runs),
				BallsFaced: strconv.Itoa(batter.Balls),
				Fours:      strconv.Itoa(batter.Fours),
				Sixes:      strconv.Itoa(batter.Sixes),
			})
		}
		scorecard.BattingCards = append(scorecard.BattingCards, card)
	}

	if len(scorecard.BowlingCards) == 0 && len(live.Bowlers) > 0 {
		card := BowlingCard{
			InningsNumber: 1,
			TeamName:      live.BowlingTeam,
			Players:       make([]BowlingCardEntry, 0, len(live.Bowlers)),
		}
		for _, bowler := range live.Bowlers {
			card.Players = append(card.Players, BowlingCardEntry{
				PlayerID:    bowler.PlayerID,
				PlayerName:  bowler.PlayerName,
				Overs:       overFromBallsOrFloat(bowler.Balls, bowler.Overs),
				Maidens:     strconv.Itoa(bowler.Maidens),
				Conceded:    strconv.Itoa(bowler.Conceded),
				Wickets:     strconv.Itoa(bowler.Wickets),
				EconomyRate: fmt.Sprintf("%.2f", bowler.Economy),
			})
		}
		scorecard.BowlingCards = append(scorecard.BowlingCards, card)
	}
}

func strikeRate(runs, balls int) float64 {
	if balls <= 0 {
		return 0
	}
	return float64(runs) * 100.0 / float64(balls)
}

func economy(conceded, balls int, overs float64) float64 {
	if balls > 0 {
		return float64(conceded) / (float64(balls) / 6.0)
	}
	if overs > 0 {
		return float64(conceded) / overs
	}
	return 0
}

func overFromBallsOrFloat(balls int, overs float64) string {
	if balls > 0 {
		return fmt.Sprintf("%d.%d", balls/6, balls%6)
	}
	if overs > 0 {
		return fmt.Sprintf("%.1f", overs)
	}
	return "0.0"
}

func teamLabelByID(match Match, teamID string) string {
	teamID = strings.TrimSpace(teamID)
	for _, team := range match.Teams {
		if strings.TrimSpace(team.ID) == teamID {
			return nonEmpty(strings.TrimSpace(team.ShortName), strings.TrimSpace(team.Name), teamID)
		}
	}
	return ""
}

func otherTeamLabelByID(match Match, teamID string) string {
	teamID = strings.TrimSpace(teamID)
	for _, team := range match.Teams {
		if strings.TrimSpace(team.ID) != teamID {
			return nonEmpty(strings.TrimSpace(team.ShortName), strings.TrimSpace(team.Name), strings.TrimSpace(team.ID))
		}
	}
	return ""
}

func matchTeamsLabelFromMatch(match Match) string {
	parts := make([]string, 0, len(match.Teams))
	for _, team := range match.Teams {
		label := nonEmpty(strings.TrimSpace(team.ShortName), strings.TrimSpace(team.Name), strings.TrimSpace(team.ID))
		if label != "" {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, ", ")
}

func parseNamesFromDeliveryShortText(shortText string) (string, string) {
	shortText = strings.TrimSpace(shortText)
	if shortText == "" {
		return "", ""
	}
	toParts := strings.SplitN(shortText, " to ", 2)
	if len(toParts) != 2 {
		return "", ""
	}
	bowler := strings.TrimSpace(toParts[0])
	right := toParts[1]
	commaParts := strings.SplitN(right, ",", 2)
	if len(commaParts) == 0 {
		return "", ""
	}
	batsman := strings.TrimSpace(commaParts[0])
	return bowler, batsman
}

func detectLiveStaleness(matchScore, liveScore string) (bool, string) {
	matchRuns, matchWkts, okMatch := parseRunsWkts(matchScore)
	liveRuns, liveWkts, okLive := parseRunsWkts(liveScore)
	if !okMatch || !okLive {
		return false, ""
	}
	if liveRuns < matchRuns || liveWkts < matchWkts {
		return true, fmt.Sprintf("live snapshot %d/%d trails match summary %d/%d", liveRuns, liveWkts, matchRuns, matchWkts)
	}
	return false, ""
}

func parseRunsWkts(raw string) (int, int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, 0, false
	}
	idx := strings.Index(raw, "/")
	if idx <= 0 || idx >= len(raw)-1 {
		return 0, 0, false
	}
	left := idx - 1
	for left >= 0 && raw[left] >= '0' && raw[left] <= '9' {
		left--
	}
	right := idx + 1
	for right < len(raw) && raw[right] >= '0' && raw[right] <= '9' {
		right++
	}
	runStr := strings.TrimSpace(raw[left+1 : idx])
	wktStr := strings.TrimSpace(raw[idx+1 : right])
	runs, err1 := strconv.Atoi(runStr)
	wkts, err2 := strconv.Atoi(wktStr)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return runs, wkts, true
}

func resolveDuelIdentity(deliveries []DeliveryEvent, query string, isBatter bool) (string, string) {
	q := normalizeAlias(query)
	if q == "" {
		return "", ""
	}

	type candidate struct {
		id    string
		name  string
		score int
	}
	best := candidate{}
	for _, delivery := range deliveries {
		bowlerName, batsmanName := parseNamesFromDeliveryShortText(delivery.ShortText)
		id := delivery.BowlerPlayerID
		name := bowlerName
		if isBatter {
			id = delivery.BatsmanPlayerID
			name = batsmanName
		}
		if strings.TrimSpace(id) == "" && strings.TrimSpace(name) == "" {
			continue
		}
		score := aliasMatchScore(normalizeAlias(nonEmpty(name, id)), q, strings.Fields(q))
		if normalizeAlias(id) == q {
			score = 1000
		}
		if score > best.score {
			best = candidate{id: strings.TrimSpace(id), name: strings.TrimSpace(name), score: score}
		}
	}
	if best.score >= 300 || best.score == 1000 {
		return best.id, best.name
	}
	return "", ""
}

func deliveryMatchesDuel(delivery DeliveryEvent, batterID, batterName, bowlerID, bowlerName string) bool {
	shortBowler, shortBatter := parseNamesFromDeliveryShortText(delivery.ShortText)
	batterMatch := strings.TrimSpace(batterID) != "" && strings.TrimSpace(delivery.BatsmanPlayerID) == strings.TrimSpace(batterID)
	if !batterMatch && normalizeAlias(batterName) != "" {
		batterMatch = normalizeAlias(shortBatter) == normalizeAlias(batterName)
	}
	if !batterMatch {
		return false
	}

	bowlerMatch := strings.TrimSpace(bowlerID) != "" && strings.TrimSpace(delivery.BowlerPlayerID) == strings.TrimSpace(bowlerID)
	if !bowlerMatch && normalizeAlias(bowlerName) != "" {
		bowlerMatch = normalizeAlias(shortBowler) == normalizeAlias(bowlerName)
	}
	return bowlerMatch
}

func matchScoreLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "/") {
		return raw
	}
	return ""
}

func isSparseSituation(situation *MatchSituation) bool {
	if situation == nil {
		return true
	}
	return len(situation.Data) == 0 && situation.Live == nil
}

func isLiveMatch(match Match) bool {
	if state := strings.ToLower(strings.TrimSpace(statusString(match.Extensions, "statusState"))); state == "in" {
		return true
	}
	if detail := strings.ToLower(strings.TrimSpace(statusString(match.Extensions, "statusDetail"))); detail == "live" {
		return true
	}
	if detail := strings.ToLower(strings.TrimSpace(statusString(match.Extensions, "statusShortDetail"))); detail == "live" {
		return true
	}

	state := strings.ToLower(strings.TrimSpace(match.MatchState))
	if strings.Contains(state, "live") {
		return true
	}
	return strings.Contains(state, " in progress") || strings.Contains(state, "stumps")
}

func statusString(extensions map[string]any, key string) string {
	if extensions == nil {
		return ""
	}
	raw, ok := extensions[key]
	if !ok || raw == nil {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}

type matchStatusSnapshot struct {
	summary     string
	longSummary string
	state       string
	detail      string
	shortDetail string
	description string
}

func (s matchStatusSnapshot) stateSummary() string {
	return nonEmpty(s.summary, s.longSummary, s.shortDetail, s.detail, s.description, s.state)
}

type teamIdentity struct {
	name      string
	shortName string
}
