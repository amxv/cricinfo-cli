package cricinfo

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const defaultMatchListLimit = 20

// MatchServiceConfig configures match discovery and lookup behavior.
type MatchServiceConfig struct {
	Client   *Client
	Resolver *Resolver
}

// MatchListOptions controls list/live traversal behavior.
type MatchListOptions struct {
	Limit int
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

	scorecardRef := matchSubresourceRef(*lookup.match, "matchcards", "matchcards")
	if scorecardRef == "" {
		return NormalizedResult{
			Kind:    EntityMatchScorecard,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("scorecard route unavailable for match %q", lookup.match.ID),
		}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, scorecardRef)
	if err != nil {
		return NewTransportErrorResult(EntityMatchScorecard, scorecardRef, err), nil
	}

	scorecard, err := NormalizeMatchScorecard(resolved.Body, *lookup.match)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize matchcards %q: %w", resolved.CanonicalRef, err)
	}

	warnings := append([]string{}, lookup.warnings...)
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

	return s.deliveryEventsFromRoute(ctx, detailsRef, lookup.warnings)
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

	return s.deliveryEventsFromRoute(ctx, playsRef, lookup.warnings)
}

// Situation resolves and returns normalized match situation data.
func (s *MatchService) Situation(ctx context.Context, query string, opts MatchLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveMatchLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityMatchSituation
		return *passthrough, nil
	}

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
	if len(lookup.warnings) > 0 {
		result = NewPartialResult(EntityMatchSituation, situation, lookup.warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
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
	resolved, err := s.client.ResolveRefChain(ctx, "/events")
	if err != nil {
		return NewTransportErrorResult(EntityMatch, "/events", err), nil
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("decode /events page: %w", err)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultMatchListLimit
	}

	statusCache := map[string]matchStatusSnapshot{}

	matches := make([]Match, 0, limit)
	warnings := make([]string, 0)
	for _, eventRef := range page.Items {
		if len(matches) >= limit {
			break
		}

		eventMatches, eventWarnings, eventErr := s.matchesFromEventRef(ctx, eventRef.URL)
		if eventErr != nil {
			warnings = append(warnings, fmt.Sprintf("event %s: %v", strings.TrimSpace(eventRef.URL), eventErr))
			continue
		}
		warnings = append(warnings, eventWarnings...)

		for _, eventMatch := range eventMatches {
			match := eventMatch
			s.enrichMatchTeamsFromIndex(&match)
			if liveOnly && !isLiveMatch(match) {
				warnings = append(warnings, s.hydrateMatchStatusOnly(ctx, &match, statusCache)...)
			}
			if liveOnly && !isLiveMatch(match) {
				continue
			}
			match.ScoreSummary = matchScoreSummary(match.Teams)
			matches = append(matches, match)
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
	if len(searchResult.Entities) == 0 {
		result := NormalizedResult{
			Kind:    EntityMatch,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("no matches found for %q", query),
		}
		return nil, &result
	}

	entity := searchResult.Entities[0]
	ref := buildMatchRef(entity)
	if ref == "" {
		result := NormalizedResult{
			Kind:    EntityMatch,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("unable to resolve match ref for %q", query),
		}
		return nil, &result
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
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
		warnings: searchResult.Warnings,
	}, nil
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

	for _, item := range pageItems {
		itemRef := strings.TrimSpace(item.URL)
		if itemRef == "" {
			warnings = append(warnings, "skip detail item with empty ref")
			continue
		}

		itemResolved, itemErr := s.resolveRefChainResilient(ctx, itemRef)
		if itemErr != nil {
			warnings = append(warnings, fmt.Sprintf("detail %s: %v", itemRef, itemErr))
			continue
		}

		delivery, normalizeErr := NormalizeDeliveryEvent(itemResolved.Body)
		if normalizeErr != nil {
			warnings = append(warnings, fmt.Sprintf("detail %s: %v", itemRef, normalizeErr))
			continue
		}
		events = append(events, *delivery)
	}

	result := NewListResult(EntityDeliveryEvent, events)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityDeliveryEvent, events, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
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

	items := make([]any, 0, len(pageItems))
	for _, item := range pageItems {
		itemRef := strings.TrimSpace(item.URL)
		if itemRef == "" {
			warnings = append(warnings, "skip item with empty ref")
			continue
		}

		itemResolved, itemErr := s.resolveRefChainResilient(ctx, itemRef)
		if itemErr != nil {
			warnings = append(warnings, fmt.Sprintf("item %s: %v", itemRef, itemErr))
			continue
		}

		normalized, normalizeErr := normalize(itemResolved.Body)
		if normalizeErr != nil {
			warnings = append(warnings, fmt.Sprintf("item %s: %v", itemRef, normalizeErr))
			continue
		}
		items = append(items, normalized)
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

	warnings := make([]string, 0)
	baseRef := firstNonEmptyString(first.CanonicalRef, first.RequestedRef)
	for pageIndex := 2; pageIndex <= page.PageCount; pageIndex++ {
		pageRef := pagedRef(baseRef, pageIndex)
		if pageRef == "" {
			warnings = append(warnings, fmt.Sprintf("page %d unavailable for %s", pageIndex, baseRef))
			continue
		}
		pageDoc, pageErr := s.resolveRefChainResilient(ctx, pageRef)
		if pageErr != nil {
			warnings = append(warnings, fmt.Sprintf("page %d %s: %v", pageIndex, pageRef, pageErr))
			continue
		}
		nextPage, decodeErr := DecodePage[Ref](pageDoc.Body)
		if decodeErr != nil {
			warnings = append(warnings, fmt.Sprintf("page %d %s: %v", pageIndex, pageDoc.CanonicalRef, decodeErr))
			continue
		}
		items = append(items, nextPage.Items...)
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

func isSparseSituation(situation *MatchSituation) bool {
	if situation == nil {
		return true
	}
	return len(situation.Data) == 0
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
