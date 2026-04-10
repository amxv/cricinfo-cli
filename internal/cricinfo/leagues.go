package cricinfo

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultLeagueListLimit     = 20
	defaultLeagueEventsLimit   = 20
	defaultLeagueAthletesLimit = 20
	maxStandingsTraversalDepth = 12
)

// LeagueServiceConfig configures league/season/standings command behavior.
type LeagueServiceConfig struct {
	Client   *Client
	Resolver *Resolver
}

// LeagueListOptions controls list-style league command behavior.
type LeagueListOptions struct {
	Limit int
}

// SeasonLookupOptions controls season, type, and group traversal behavior.
type SeasonLookupOptions struct {
	SeasonQuery string
	TypeQuery   string
}

// LeagueService implements league, season, and standings navigation commands.
type LeagueService struct {
	client       *Client
	resolver     *Resolver
	ownsResolver bool
}

// NewLeagueService builds a league service using default client/resolver when omitted.
func NewLeagueService(cfg LeagueServiceConfig) (*LeagueService, error) {
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

	return &LeagueService{
		client:       client,
		resolver:     resolver,
		ownsResolver: ownsResolver,
	}, nil
}

// Close persists resolver cache when owned by this service.
func (s *LeagueService) Close() error {
	if !s.ownsResolver || s.resolver == nil {
		return nil
	}
	return s.resolver.Close()
}

// List resolves league refs from /leagues into normalized league entries.
func (s *LeagueService) List(ctx context.Context, opts LeagueListOptions) (NormalizedResult, error) {
	resolved, err := s.client.ResolveRefChain(ctx, "/leagues")
	if err != nil {
		return NewTransportErrorResult(EntityLeague, "/leagues", err), nil
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("decode /leagues page: %w", err)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLeagueListLimit
	}
	if limit > len(page.Items) {
		limit = len(page.Items)
	}

	items := make([]any, 0, limit)
	warnings := make([]string, 0)
	for i := 0; i < limit; i++ {
		ref := strings.TrimSpace(page.Items[i].URL)
		if ref == "" {
			warnings = append(warnings, "skip league item with empty ref")
			continue
		}

		league, _, warning, lookupErr := s.fetchLeagueByRef(ctx, ref)
		if lookupErr != nil {
			warnings = append(warnings, fmt.Sprintf("league %s: %v", ref, lookupErr))
			continue
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
		s.upsertLeagueEntity(*league)
		items = append(items, *league)
	}

	result := NewListResult(EntityLeague, items)
	if compact := compactWarnings(warnings); len(compact) > 0 {
		result = NewPartialListResult(EntityLeague, items, compact...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Show resolves one league by id/ref/alias and returns a normalized league payload.
func (s *LeagueService) Show(ctx context.Context, leagueQuery string) (NormalizedResult, error) {
	lookup, passthrough := s.resolveLeagueLookup(ctx, leagueQuery)
	if passthrough != nil {
		return *passthrough, nil
	}

	result := NewDataResult(EntityLeague, lookup.league)
	if len(lookup.warnings) > 0 {
		result = NewPartialResult(EntityLeague, lookup.league, lookup.warnings...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	return result, nil
}

// Events resolves one league and lists normalized match entries from /leagues/{id}/events traversal.
func (s *LeagueService) Events(ctx context.Context, leagueQuery string, opts LeagueListOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveLeagueLookup(ctx, leagueQuery)
	if passthrough != nil {
		passthrough.Kind = EntityMatch
		return *passthrough, nil
	}

	eventsRef := nonEmpty(extensionRef(lookup.league.Extensions, "events"), "/leagues/"+strings.TrimSpace(lookup.league.ID)+"/events")
	resolved, err := s.client.ResolveRefChain(ctx, eventsRef)
	if err != nil {
		return NewTransportErrorResult(EntityMatch, eventsRef, err), nil
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("decode league events page %q: %w", resolved.CanonicalRef, err)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLeagueEventsLimit
	}

	items := make([]any, 0, limit)
	warnings := append([]string{}, lookup.warnings...)
	for _, eventRef := range page.Items {
		if len(items) >= limit {
			break
		}

		ref := strings.TrimSpace(eventRef.URL)
		if ref == "" {
			warnings = append(warnings, "skip event item with empty ref")
			continue
		}

		eventResolved, eventErr := s.client.ResolveRefChain(ctx, ref)
		if eventErr != nil {
			warnings = append(warnings, fmt.Sprintf("event %s: %v", ref, eventErr))
			continue
		}

		matches, normalizeErr := NormalizeMatchesFromEvent(eventResolved.Body)
		if normalizeErr != nil {
			warnings = append(warnings, fmt.Sprintf("event %s: %v", eventResolved.CanonicalRef, normalizeErr))
			continue
		}
		for _, match := range matches {
			if strings.TrimSpace(match.LeagueID) == "" {
				match.LeagueID = strings.TrimSpace(lookup.league.ID)
			}
			items = append(items, match)
			if len(items) >= limit {
				break
			}
		}
	}

	result := NewListResult(EntityMatch, items)
	if compact := compactWarnings(warnings); len(compact) > 0 {
		result = NewPartialListResult(EntityMatch, items, compact...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Calendar resolves one league and normalizes section-shaped calendar routes into calendar-day entries.
func (s *LeagueService) Calendar(ctx context.Context, leagueQuery string) (NormalizedResult, error) {
	lookup, passthrough := s.resolveLeagueLookup(ctx, leagueQuery)
	if passthrough != nil {
		passthrough.Kind = EntityCalendarDay
		return *passthrough, nil
	}

	calendarRef := "/leagues/" + strings.TrimSpace(lookup.league.ID) + "/calendar"
	resolved, err := s.client.ResolveRefChain(ctx, calendarRef)
	if err != nil {
		return NewTransportErrorResult(EntityCalendarDay, calendarRef, err), nil
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("decode calendar root %q: %w", resolved.CanonicalRef, err)
	}

	items := make([]CalendarDay, 0)
	warnings := append([]string{}, lookup.warnings...)
	for _, item := range page.Items {
		ref := strings.TrimSpace(item.URL)
		if ref == "" {
			continue
		}

		itemResolved, itemErr := s.client.ResolveRefChain(ctx, ref)
		if itemErr != nil {
			warnings = append(warnings, fmt.Sprintf("calendar section %s: %v", ref, itemErr))
			continue
		}

		days, normalizeErr := NormalizeCalendarDays(itemResolved.Body)
		if normalizeErr != nil {
			warnings = append(warnings, fmt.Sprintf("calendar section %s: %v", itemResolved.CanonicalRef, normalizeErr))
			continue
		}
		items = append(items, days...)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Date != items[j].Date {
			return items[i].Date < items[j].Date
		}
		return items[i].DayType < items[j].DayType
	})

	renderItems := make([]any, 0, len(items))
	for _, day := range items {
		renderItems = append(renderItems, day)
	}

	result := NewListResult(EntityCalendarDay, renderItems)
	if compact := compactWarnings(warnings); len(compact) > 0 {
		result = NewPartialListResult(EntityCalendarDay, renderItems, compact...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Athletes resolves one league and returns league-athlete views, falling back to event-roster traversal when needed.
func (s *LeagueService) Athletes(ctx context.Context, leagueQuery string, opts LeagueListOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveLeagueLookup(ctx, leagueQuery)
	if passthrough != nil {
		passthrough.Kind = EntityPlayer
		return *passthrough, nil
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLeagueAthletesLimit
	}

	warnings := append([]string{}, lookup.warnings...)
	players, directWarnings, directErr := s.playersFromLeagueAthletePage(ctx, *lookup.league, limit)
	warnings = append(warnings, directWarnings...)
	if directErr != nil {
		warnings = append(warnings, directErr.Error())
	}

	if len(players) == 0 {
		fallbackPlayers, fallbackWarnings := s.playersFromLeagueEventRosters(ctx, *lookup.league, limit)
		players = append(players, fallbackPlayers...)
		warnings = append(warnings, fallbackWarnings...)
	}

	items := make([]any, 0, len(players))
	for _, player := range players {
		items = append(items, player)
	}

	result := NewListResult(EntityPlayer, items)
	if compact := compactWarnings(warnings); len(compact) > 0 {
		result = NewPartialListResult(EntityPlayer, items, compact...)
	}
	result.RequestedRef = "/leagues/" + strings.TrimSpace(lookup.league.ID) + "/athletes"
	result.CanonicalRef = result.RequestedRef
	return result, nil
}

// Standings resolves one league and hides multi-hop standings traversal behind a single command response.
func (s *LeagueService) Standings(ctx context.Context, leagueQuery string) (NormalizedResult, error) {
	lookup, passthrough := s.resolveLeagueLookup(ctx, leagueQuery)
	if passthrough != nil {
		passthrough.Kind = EntityStandingsGroup
		return *passthrough, nil
	}

	standingsRef := "/leagues/" + strings.TrimSpace(lookup.league.ID) + "/standings"
	groups, warnings, err := s.collectStandingsGroups(ctx, standingsRef, map[string]struct{}{}, 0)
	if err != nil {
		return NewTransportErrorResult(EntityStandingsGroup, standingsRef, err), nil
	}
	s.hydrateStandingsTeamNames(ctx, groups, &warnings)

	items := make([]any, 0, len(groups))
	for _, group := range groups {
		items = append(items, group)
	}

	result := NewListResult(EntityStandingsGroup, items)
	combinedWarnings := append([]string{}, lookup.warnings...)
	combinedWarnings = append(combinedWarnings, warnings...)
	if compact := compactWarnings(combinedWarnings); len(compact) > 0 {
		result = NewPartialListResult(EntityStandingsGroup, items, compact...)
	}
	result.RequestedRef = standingsRef
	result.CanonicalRef = standingsRef
	return result, nil
}

// Seasons resolves one league and returns normalized season refs.
func (s *LeagueService) Seasons(ctx context.Context, leagueQuery string) (NormalizedResult, error) {
	lookup, passthrough := s.resolveLeagueLookup(ctx, leagueQuery)
	if passthrough != nil {
		passthrough.Kind = EntitySeason
		return *passthrough, nil
	}

	resolved, seasons, warnings, err := s.fetchLeagueSeasons(ctx, *lookup.league)
	if err != nil {
		return NewTransportErrorResult(EntitySeason, "/leagues/"+lookup.league.ID+"/seasons", err), nil
	}

	items := make([]any, 0, len(seasons))
	for _, season := range seasons {
		items = append(items, season)
	}

	combinedWarnings := append([]string{}, lookup.warnings...)
	combinedWarnings = append(combinedWarnings, warnings...)
	result := NewListResult(EntitySeason, items)
	if compact := compactWarnings(combinedWarnings); len(compact) > 0 {
		result = NewPartialListResult(EntitySeason, items, compact...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// SeasonShow resolves one league season selection and returns the normalized season payload.
func (s *LeagueService) SeasonShow(ctx context.Context, leagueQuery string, opts SeasonLookupOptions) (NormalizedResult, error) {
	selection, passthrough := s.resolveSeasonSelection(ctx, leagueQuery, opts.SeasonQuery)
	if passthrough != nil {
		return *passthrough, nil
	}

	result := NewDataResult(EntitySeason, selection.season)
	if len(selection.warnings) > 0 {
		result = NewPartialResult(EntitySeason, selection.season, selection.warnings...)
	}
	result.RequestedRef = selection.resolved.RequestedRef
	result.CanonicalRef = selection.resolved.CanonicalRef
	return result, nil
}

// SeasonTypes resolves one league season selection and returns normalized season-type entries.
func (s *LeagueService) SeasonTypes(ctx context.Context, leagueQuery string, opts SeasonLookupOptions) (NormalizedResult, error) {
	selection, passthrough := s.resolveSeasonSelection(ctx, leagueQuery, opts.SeasonQuery)
	if passthrough != nil {
		passthrough.Kind = EntitySeasonType
		return *passthrough, nil
	}

	resolved, types, warnings, err := s.fetchSeasonTypes(ctx, selection.season)
	if err != nil {
		return NewTransportErrorResult(EntitySeasonType, seasonTypesRef(selection.season), err), nil
	}

	items := make([]any, 0, len(types))
	for _, seasonType := range types {
		items = append(items, seasonType)
	}

	combinedWarnings := append([]string{}, selection.warnings...)
	combinedWarnings = append(combinedWarnings, warnings...)
	result := NewListResult(EntitySeasonType, items)
	if compact := compactWarnings(combinedWarnings); len(compact) > 0 {
		result = NewPartialListResult(EntitySeasonType, items, compact...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// SeasonGroups resolves one league season+type selection and returns normalized season-group entries.
func (s *LeagueService) SeasonGroups(ctx context.Context, leagueQuery string, opts SeasonLookupOptions) (NormalizedResult, error) {
	selection, passthrough := s.resolveSeasonSelection(ctx, leagueQuery, opts.SeasonQuery)
	if passthrough != nil {
		passthrough.Kind = EntitySeasonGroup
		return *passthrough, nil
	}

	typeSelection, typePassthrough := s.resolveSeasonTypeSelection(ctx, selection.season, opts.TypeQuery)
	if typePassthrough != nil {
		return *typePassthrough, nil
	}

	resolved, groups, warnings, err := s.fetchSeasonGroups(ctx, typeSelection.seasonType)
	if err != nil {
		return NewTransportErrorResult(EntitySeasonGroup, seasonGroupsRef(typeSelection.seasonType), err), nil
	}

	items := make([]any, 0, len(groups))
	for _, group := range groups {
		items = append(items, group)
	}

	combinedWarnings := append([]string{}, selection.warnings...)
	combinedWarnings = append(combinedWarnings, typeSelection.warnings...)
	combinedWarnings = append(combinedWarnings, warnings...)
	result := NewListResult(EntitySeasonGroup, items)
	if compact := compactWarnings(combinedWarnings); len(compact) > 0 {
		result = NewPartialListResult(EntitySeasonGroup, items, compact...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

type leagueLookup struct {
	league   *League
	resolved *ResolvedDocument
	warnings []string
}

type seasonSelection struct {
	league   *League
	season   Season
	resolved *ResolvedDocument
	warnings []string
}

type seasonTypeSelection struct {
	seasonType SeasonType
	warnings   []string
}

func (s *LeagueService) resolveLeagueLookup(ctx context.Context, query string) (*leagueLookup, *NormalizedResult) {
	query = strings.TrimSpace(query)
	if query == "" {
		result := NormalizedResult{
			Kind:    EntityLeague,
			Status:  ResultStatusEmpty,
			Message: "league query is required",
		}
		return nil, &result
	}

	warnings := make([]string, 0)
	searchResult, err := s.resolver.Search(ctx, EntityLeague, query, ResolveOptions{Limit: 5})
	if err != nil {
		result := NewTransportErrorResult(EntityLeague, query, err)
		return nil, &result
	}
	warnings = append(warnings, searchResult.Warnings...)

	if len(searchResult.Entities) > 0 {
		entity := searchResult.Entities[0]
		ref := nonEmpty(strings.TrimSpace(entity.Ref), "/leagues/"+strings.TrimSpace(entity.ID))
		league, resolved, warning, lookupErr := s.fetchLeagueByRef(ctx, ref)
		if lookupErr != nil {
			result := NewTransportErrorResult(EntityLeague, ref, lookupErr)
			return nil, &result
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
		s.upsertLeagueEntity(*league)
		return &leagueLookup{
			league:   league,
			resolved: resolved,
			warnings: compactWarnings(warnings),
		}, nil
	}

	if isKnownRefQuery(query) || isNumeric(query) {
		ref := strings.TrimSpace(query)
		if isNumeric(query) {
			ref = "/leagues/" + strings.TrimSpace(query)
		}
		league, resolved, warning, lookupErr := s.fetchLeagueByRef(ctx, ref)
		if lookupErr != nil {
			result := NewTransportErrorResult(EntityLeague, ref, lookupErr)
			return nil, &result
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
		s.upsertLeagueEntity(*league)
		return &leagueLookup{
			league:   league,
			resolved: resolved,
			warnings: compactWarnings(warnings),
		}, nil
	}

	ref, fallbackWarnings := s.findLeagueRefByNameFallback(ctx, query)
	warnings = append(warnings, fallbackWarnings...)
	if ref != "" {
		league, resolved, warning, lookupErr := s.fetchLeagueByRef(ctx, ref)
		if lookupErr != nil {
			result := NewTransportErrorResult(EntityLeague, ref, lookupErr)
			return nil, &result
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
		s.upsertLeagueEntity(*league)
		return &leagueLookup{
			league:   league,
			resolved: resolved,
			warnings: compactWarnings(warnings),
		}, nil
	}

	result := NormalizedResult{
		Kind:    EntityLeague,
		Status:  ResultStatusEmpty,
		Message: fmt.Sprintf("no leagues found for %q", query),
	}
	return nil, &result
}

func (s *LeagueService) fetchLeagueByRef(ctx context.Context, ref string) (*League, *ResolvedDocument, string, error) {
	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return nil, nil, "", err
	}
	league, err := NormalizeLeague(resolved.Body)
	if err != nil {
		return nil, nil, "", fmt.Errorf("normalize league %q: %w", resolved.CanonicalRef, err)
	}
	if strings.TrimSpace(league.ID) == "" {
		league.ID = strings.TrimSpace(refIDs(resolved.CanonicalRef)["leagueId"])
	}
	return league, resolved, "", nil
}

func (s *LeagueService) findLeagueRefByNameFallback(ctx context.Context, query string) (string, []string) {
	resolved, err := s.client.ResolveRefChain(ctx, "/leagues")
	if err != nil {
		return "", []string{fmt.Sprintf("league fallback scan failed: %v", err)}
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return "", []string{fmt.Sprintf("decode /leagues page during fallback scan: %v", err)}
	}

	target := normalizeAlias(query)
	if target == "" {
		return "", nil
	}

	warnings := make([]string, 0)
	for _, item := range page.Items {
		ref := strings.TrimSpace(item.URL)
		if ref == "" {
			continue
		}
		league, _, warning, lookupErr := s.fetchLeagueByRef(ctx, ref)
		if lookupErr != nil {
			warnings = append(warnings, fmt.Sprintf("league fallback %s: %v", ref, lookupErr))
			continue
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
		s.upsertLeagueEntity(*league)

		aliases := []string{
			strings.TrimSpace(league.ID),
			strings.TrimSpace(league.Name),
			strings.TrimSpace(league.Slug),
		}
		for _, alias := range aliases {
			if normalizeAlias(alias) == target {
				return ref, compactWarnings(warnings)
			}
		}
	}

	return "", compactWarnings(warnings)
}

func (s *LeagueService) fetchLeagueSeasons(ctx context.Context, league League) (*ResolvedDocument, []Season, []string, error) {
	seasonsRef := nonEmpty(extensionRef(league.Extensions, "seasons"), "/leagues/"+strings.TrimSpace(league.ID)+"/seasons")
	resolved, err := s.client.ResolveRefChain(ctx, seasonsRef)
	if err != nil {
		return nil, nil, nil, err
	}

	seasons, err := NormalizeSeasonList(resolved.Body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("normalize seasons list %q: %w", resolved.CanonicalRef, err)
	}
	for i := range seasons {
		if strings.TrimSpace(seasons[i].LeagueID) == "" {
			seasons[i].LeagueID = strings.TrimSpace(league.ID)
		}
	}
	return resolved, seasons, nil, nil
}

func (s *LeagueService) resolveSeasonSelection(ctx context.Context, leagueQuery, seasonQuery string) (*seasonSelection, *NormalizedResult) {
	lookup, passthrough := s.resolveLeagueLookup(ctx, leagueQuery)
	if passthrough != nil {
		passthrough.Kind = EntitySeason
		return nil, passthrough
	}

	seasonQuery = strings.TrimSpace(seasonQuery)
	if seasonQuery == "" {
		result := NormalizedResult{
			Kind:    EntitySeason,
			Status:  ResultStatusEmpty,
			Message: "--season is required",
		}
		return nil, &result
	}

	_, seasons, seasonWarnings, err := s.fetchLeagueSeasons(ctx, *lookup.league)
	if err != nil {
		result := NewTransportErrorResult(EntitySeason, "/leagues/"+lookup.league.ID+"/seasons", err)
		return nil, &result
	}

	selectedRef := ""
	queryIDs := refIDs(seasonQuery)
	for _, season := range seasons {
		ids := refIDs(season.Ref)
		candidates := []string{
			strings.TrimSpace(season.ID),
			strings.TrimSpace(strconv.Itoa(season.Year)),
			strings.TrimSpace(ids["seasonId"]),
			strings.TrimSpace(queryIDs["seasonId"]),
		}
		for _, candidate := range candidates {
			if candidate != "" && strings.EqualFold(candidate, strings.TrimSpace(seasonQuery)) {
				selectedRef = strings.TrimSpace(season.Ref)
				break
			}
		}
		if selectedRef != "" {
			break
		}
	}

	if selectedRef == "" && isKnownRefQuery(seasonQuery) {
		selectedRef = strings.TrimSpace(seasonQuery)
	}
	if selectedRef == "" && isNumeric(seasonQuery) {
		selectedRef = "/leagues/" + strings.TrimSpace(lookup.league.ID) + "/seasons/" + strings.TrimSpace(seasonQuery)
	}
	if selectedRef == "" {
		result := NormalizedResult{
			Kind:    EntitySeason,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("season %q not found for league %q", seasonQuery, lookup.league.ID),
		}
		return nil, &result
	}

	resolved, err := s.client.ResolveRefChain(ctx, selectedRef)
	if err != nil {
		result := NewTransportErrorResult(EntitySeason, selectedRef, err)
		return nil, &result
	}

	season, err := NormalizeSeason(resolved.Body)
	if err != nil {
		return nil, &NormalizedResult{
			Kind:    EntitySeason,
			Status:  ResultStatusError,
			Message: fmt.Sprintf("normalize season %q: %v", resolved.CanonicalRef, err),
		}
	}

	if strings.TrimSpace(season.LeagueID) == "" {
		season.LeagueID = strings.TrimSpace(lookup.league.ID)
	}

	warnings := append([]string{}, lookup.warnings...)
	warnings = append(warnings, seasonWarnings...)
	return &seasonSelection{
		league:   lookup.league,
		season:   *season,
		resolved: resolved,
		warnings: compactWarnings(warnings),
	}, nil
}

func (s *LeagueService) fetchSeasonTypes(ctx context.Context, season Season) (*ResolvedDocument, []SeasonType, []string, error) {
	typesRef := seasonTypesRef(season)
	resolved, err := s.client.ResolveRefChain(ctx, typesRef)
	if err != nil {
		return nil, nil, nil, err
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode season types page %q: %w", resolved.CanonicalRef, err)
	}

	types := make([]SeasonType, 0, len(page.Items))
	warnings := make([]string, 0)
	for _, item := range page.Items {
		ref := strings.TrimSpace(item.URL)
		if ref == "" {
			continue
		}
		itemResolved, itemErr := s.client.ResolveRefChain(ctx, ref)
		if itemErr != nil {
			warnings = append(warnings, fmt.Sprintf("season type %s: %v", ref, itemErr))
			continue
		}
		seasonType, normalizeErr := NormalizeSeasonType(itemResolved.Body)
		if normalizeErr != nil {
			warnings = append(warnings, fmt.Sprintf("season type %s: %v", itemResolved.CanonicalRef, normalizeErr))
			continue
		}
		if seasonType.SeasonID == "" {
			seasonType.SeasonID = season.ID
		}
		if seasonType.LeagueID == "" {
			seasonType.LeagueID = season.LeagueID
		}
		types = append(types, *seasonType)
	}

	return resolved, types, compactWarnings(warnings), nil
}

func (s *LeagueService) resolveSeasonTypeSelection(ctx context.Context, season Season, typeQuery string) (*seasonTypeSelection, *NormalizedResult) {
	typeQuery = strings.TrimSpace(typeQuery)
	if typeQuery == "" {
		result := NormalizedResult{
			Kind:    EntitySeasonType,
			Status:  ResultStatusEmpty,
			Message: "--type is required",
		}
		return nil, &result
	}

	_, types, warnings, err := s.fetchSeasonTypes(ctx, season)
	if err != nil {
		result := NewTransportErrorResult(EntitySeasonType, seasonTypesRef(season), err)
		return nil, &result
	}
	if len(types) == 0 {
		result := NormalizedResult{
			Kind:    EntitySeasonType,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("no season types found for season %q", season.ID),
		}
		return nil, &result
	}

	queryIDs := refIDs(typeQuery)
	queryNorm := normalizeAlias(typeQuery)
	for _, seasonType := range types {
		candidates := []string{
			strings.TrimSpace(seasonType.ID),
			strings.TrimSpace(refIDs(seasonType.Ref)["typeId"]),
			strings.TrimSpace(queryIDs["typeId"]),
		}
		for _, candidate := range candidates {
			if candidate != "" && strings.EqualFold(candidate, typeQuery) {
				return &seasonTypeSelection{seasonType: seasonType, warnings: warnings}, nil
			}
		}
		names := []string{seasonType.Name, seasonType.Abbreviation}
		for _, name := range names {
			if normalizeAlias(name) != "" && normalizeAlias(name) == queryNorm {
				return &seasonTypeSelection{seasonType: seasonType, warnings: warnings}, nil
			}
		}
	}

	result := NormalizedResult{
		Kind:    EntitySeasonType,
		Status:  ResultStatusEmpty,
		Message: fmt.Sprintf("season type %q not found for season %q", typeQuery, season.ID),
	}
	return nil, &result
}

func (s *LeagueService) fetchSeasonGroups(ctx context.Context, seasonType SeasonType) (*ResolvedDocument, []SeasonGroup, []string, error) {
	groupsRef := seasonGroupsRef(seasonType)
	resolved, err := s.client.ResolveRefChain(ctx, groupsRef)
	if err != nil {
		return nil, nil, nil, err
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode season groups page %q: %w", resolved.CanonicalRef, err)
	}

	groups := make([]SeasonGroup, 0, len(page.Items))
	warnings := make([]string, 0)
	for _, item := range page.Items {
		ref := strings.TrimSpace(item.URL)
		if ref == "" {
			continue
		}
		itemResolved, itemErr := s.client.ResolveRefChain(ctx, ref)
		if itemErr != nil {
			warnings = append(warnings, fmt.Sprintf("season group %s: %v", ref, itemErr))
			continue
		}
		group, normalizeErr := NormalizeSeasonGroup(itemResolved.Body)
		if normalizeErr != nil {
			warnings = append(warnings, fmt.Sprintf("season group %s: %v", itemResolved.CanonicalRef, normalizeErr))
			continue
		}
		if group.SeasonID == "" {
			group.SeasonID = seasonType.SeasonID
		}
		if group.LeagueID == "" {
			group.LeagueID = seasonType.LeagueID
		}
		if group.TypeID == "" {
			group.TypeID = seasonType.ID
		}
		groups = append(groups, *group)
	}

	return resolved, groups, compactWarnings(warnings), nil
}

func (s *LeagueService) playersFromLeagueAthletePage(ctx context.Context, league League, limit int) ([]Player, []string, error) {
	athletesRef := nonEmpty(extensionRef(league.Extensions, "athletes"), "/leagues/"+strings.TrimSpace(league.ID)+"/athletes")
	resolved, err := s.client.ResolveRefChain(ctx, athletesRef)
	if err != nil {
		return nil, nil, err
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("decode league athletes page %q: %w", resolved.CanonicalRef, err)
	}

	if len(page.Items) == 0 {
		return nil, []string{"league athletes page returned no item refs; falling back to event-roster traversal"}, nil
	}

	players := make([]Player, 0, minInt(limit, len(page.Items)))
	warnings := make([]string, 0)
	for _, item := range page.Items {
		if len(players) >= limit {
			break
		}
		ref := strings.TrimSpace(item.URL)
		if ref == "" {
			continue
		}
		itemResolved, itemErr := s.client.ResolveRefChain(ctx, ref)
		if itemErr != nil {
			warnings = append(warnings, fmt.Sprintf("league athlete %s: %v", ref, itemErr))
			continue
		}
		player, normalizeErr := NormalizePlayer(itemResolved.Body)
		if normalizeErr != nil {
			warnings = append(warnings, fmt.Sprintf("league athlete %s: %v", itemResolved.CanonicalRef, normalizeErr))
			continue
		}
		players = append(players, *player)
	}

	return players, compactWarnings(warnings), nil
}

func (s *LeagueService) playersFromLeagueEventRosters(ctx context.Context, league League, limit int) ([]Player, []string) {
	eventsRef := nonEmpty(extensionRef(league.Extensions, "events"), "/leagues/"+strings.TrimSpace(league.ID)+"/events")
	resolved, err := s.client.ResolveRefChain(ctx, eventsRef)
	if err != nil {
		return nil, []string{fmt.Sprintf("fallback events traversal failed: %v", err)}
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, []string{fmt.Sprintf("decode fallback events page %q: %v", resolved.CanonicalRef, err)}
	}

	players := make([]Player, 0, limit)
	warnings := make([]string, 0)
	seenPlayers := map[string]struct{}{}

	for _, eventRef := range page.Items {
		if len(players) >= limit {
			break
		}

		ref := strings.TrimSpace(eventRef.URL)
		if ref == "" {
			continue
		}
		eventResolved, eventErr := s.client.ResolveRefChain(ctx, ref)
		if eventErr != nil {
			warnings = append(warnings, fmt.Sprintf("fallback event %s: %v", ref, eventErr))
			continue
		}
		matches, normalizeErr := NormalizeMatchesFromEvent(eventResolved.Body)
		if normalizeErr != nil {
			warnings = append(warnings, fmt.Sprintf("fallback event %s: %v", eventResolved.CanonicalRef, normalizeErr))
			continue
		}

		for _, match := range matches {
			if len(players) >= limit {
				break
			}
			for _, team := range match.Teams {
				if len(players) >= limit {
					break
				}
				rosterRef := nonEmpty(strings.TrimSpace(team.RosterRef), competitorSubresourceRef(match, team.ID, "roster"))
				if rosterRef == "" {
					continue
				}
				rosterResolved, rosterErr := s.client.ResolveRefChain(ctx, rosterRef)
				if rosterErr != nil {
					warnings = append(warnings, fmt.Sprintf("fallback roster %s: %v", rosterRef, rosterErr))
					continue
				}

				entries, entryErr := NormalizeTeamRosterEntries(rosterResolved.Body, team, TeamScopeMatch, match.ID)
				if entryErr != nil {
					warnings = append(warnings, fmt.Sprintf("fallback roster %s: %v", rosterResolved.CanonicalRef, entryErr))
					continue
				}
				for _, entry := range entries {
					if len(players) >= limit {
						break
					}
					playerID := strings.TrimSpace(entry.PlayerID)
					if playerID == "" {
						continue
					}
					if _, ok := seenPlayers[playerID]; ok {
						continue
					}

					player, playerWarning, playerErr := s.fetchLeagueScopedPlayer(ctx, league.ID, playerID)
					if playerErr != nil {
						warnings = append(warnings, fmt.Sprintf("fallback athlete %s: %v", playerID, playerErr))
						continue
					}
					if playerWarning != "" {
						warnings = append(warnings, playerWarning)
					}

					seenPlayers[playerID] = struct{}{}
					players = append(players, *player)
				}
			}
		}
	}

	return players, compactWarnings(warnings)
}

func (s *LeagueService) fetchLeagueScopedPlayer(ctx context.Context, leagueID, playerID string) (*Player, string, error) {
	leagueRef := "/leagues/" + strings.TrimSpace(leagueID) + "/athletes/" + strings.TrimSpace(playerID)
	resolved, err := s.client.ResolveRefChain(ctx, leagueRef)
	if err == nil {
		player, normalizeErr := NormalizePlayer(resolved.Body)
		if normalizeErr != nil {
			return nil, "", normalizeErr
		}
		return player, "", nil
	}

	globalRef := "/athletes/" + strings.TrimSpace(playerID)
	globalResolved, globalErr := s.client.ResolveRefChain(ctx, globalRef)
	if globalErr != nil {
		return nil, "", globalErr
	}
	player, normalizeErr := NormalizePlayer(globalResolved.Body)
	if normalizeErr != nil {
		return nil, "", normalizeErr
	}
	warning := fmt.Sprintf("league-athlete route unavailable for %s; used global athlete profile", playerID)
	return player, warning, nil
}

func (s *LeagueService) collectStandingsGroups(
	ctx context.Context,
	ref string,
	visited map[string]struct{},
	depth int,
) ([]StandingsGroup, []string, error) {
	if depth > maxStandingsTraversalDepth {
		return nil, nil, fmt.Errorf("standings traversal exceeded max depth for %q", ref)
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	canonical := strings.TrimSpace(resolved.CanonicalRef)
	if canonical == "" {
		canonical = strings.TrimSpace(ref)
	}
	if _, ok := visited[canonical]; ok {
		return nil, nil, nil
	}
	visited[canonical] = struct{}{}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("decode standings payload %q: %w", canonical, err)
	}

	groups := make([]StandingsGroup, 0)
	warnings := make([]string, 0)

	if hasStandaloneStandingsPayload(payload) {
		group := NormalizeStandingsGroupFromMap(payload)
		if group != nil {
			groups = append(groups, *group)
		}
	}

	for _, childRef := range standingsChildRefs(payload) {
		childGroups, childWarnings, childErr := s.collectStandingsGroups(ctx, childRef, visited, depth+1)
		if childErr != nil {
			warnings = append(warnings, fmt.Sprintf("standings child %s: %v", childRef, childErr))
			continue
		}
		groups = append(groups, childGroups...)
		warnings = append(warnings, childWarnings...)
	}

	groups = dedupeStandingsGroups(groups)
	return groups, compactWarnings(warnings), nil
}

func (s *LeagueService) hydrateStandingsTeamNames(ctx context.Context, groups []StandingsGroup, warnings *[]string) {
	cache := map[string]teamIdentity{}
	matchHelper := &MatchService{client: s.client}

	for groupIndex := range groups {
		for teamIndex := range groups[groupIndex].Entries {
			team := &groups[groupIndex].Entries[teamIndex]
			if strings.TrimSpace(team.Name) != "" && strings.TrimSpace(team.ShortName) != "" {
				continue
			}
			identity, err := matchHelper.fetchTeamIdentity(ctx, team, cache)
			if err != nil {
				if warnings != nil {
					*warnings = append(*warnings, fmt.Sprintf("standings team %s: %v", nonEmpty(team.Ref, team.ID), err))
				}
				continue
			}
			if strings.TrimSpace(team.Name) == "" {
				team.Name = identity.name
			}
			if strings.TrimSpace(team.ShortName) == "" {
				team.ShortName = identity.shortName
			}
		}
	}
}

func hasStandaloneStandingsPayload(payload map[string]any) bool {
	return len(mapSliceField(payload, "standings")) > 0 || len(mapSliceField(payload, "entries")) > 0
}

func standingsChildRefs(payload map[string]any) []string {
	refs := make([]string, 0)
	addRef := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		refs = append(refs, value)
	}

	for _, item := range mapSliceField(payload, "items") {
		addRef(stringField(item, "$ref"))
		addRef(refFromField(item, "standings"))
	}

	for _, key := range []string{"standings", "groups", "children"} {
		value := payload[key]
		switch typed := value.(type) {
		case map[string]any:
			addRef(stringField(typed, "$ref"))
		case []any:
			for _, item := range typed {
				asMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				addRef(stringField(asMap, "$ref"))
			}
		}
	}

	return compactWarnings(refs)
}

func dedupeStandingsGroups(groups []StandingsGroup) []StandingsGroup {
	seen := map[string]struct{}{}
	out := make([]StandingsGroup, 0, len(groups))
	for _, group := range groups {
		key := strings.TrimSpace(group.Ref)
		if key == "" {
			key = strings.TrimSpace(group.SeasonID + ":" + group.GroupID + ":" + group.ID)
		}
		if key == "" {
			key = strings.TrimSpace(group.ID)
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, group)
	}
	return out
}

func seasonTypesRef(season Season) string {
	if ref := extensionRef(season.Extensions, "types"); ref != "" {
		return ref
	}
	if season.LeagueID == "" || season.ID == "" {
		return ""
	}
	return "/leagues/" + strings.TrimSpace(season.LeagueID) + "/seasons/" + strings.TrimSpace(season.ID) + "/types"
}

func seasonGroupsRef(seasonType SeasonType) string {
	if ref := strings.TrimSpace(seasonType.GroupsRef); ref != "" {
		return ref
	}
	if seasonType.LeagueID == "" || seasonType.SeasonID == "" || seasonType.ID == "" {
		return ""
	}
	return "/leagues/" + strings.TrimSpace(seasonType.LeagueID) + "/seasons/" + strings.TrimSpace(seasonType.SeasonID) + "/types/" + strings.TrimSpace(seasonType.ID) + "/groups"
}

func (s *LeagueService) upsertLeagueEntity(league League) {
	if s.resolver == nil || s.resolver.index == nil {
		return
	}
	leagueID := strings.TrimSpace(league.ID)
	if leagueID == "" {
		return
	}
	_ = s.resolver.index.Upsert(IndexedEntity{
		Kind:      EntityLeague,
		ID:        leagueID,
		Ref:       strings.TrimSpace(league.Ref),
		Name:      strings.TrimSpace(league.Name),
		ShortName: strings.TrimSpace(league.Slug),
		Aliases: []string{
			strings.TrimSpace(league.Name),
			strings.TrimSpace(league.Slug),
			leagueID,
		},
	})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
