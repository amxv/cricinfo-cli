package cricinfo

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// HistoricalScopeOptions defines a historical traversal scope for cross-match reasoning.
type HistoricalScopeOptions struct {
	LeagueQuery string
	SeasonQuery string
	TypeQuery   string
	GroupQuery  string
	DateFrom    string
	DateTo      string
	MatchLimit  int
}

// HistoricalScopeSummary describes the resolved domain scope for one session.
type HistoricalScopeSummary struct {
	League   League
	Season   *Season
	Type     *SeasonType
	Group    *SeasonGroup
	DateFrom string
	DateTo   string
	MatchIDs []string
	Warnings []string
}

// HydrationMetrics tracks scoped in-process reuse behavior.
type HydrationMetrics struct {
	ResolveCacheHits   int
	ResolveCacheMisses int
	DomainCacheHits    int
	DomainCacheMisses  int
}

// HistoricalHydrationServiceConfig configures scoped traversal and hydration behavior.
type HistoricalHydrationServiceConfig struct {
	Client   *Client
	Resolver *Resolver
}

// HistoricalHydrationService provides real-time historical traversal with run-scoped reuse.
type HistoricalHydrationService struct {
	client       *Client
	resolver     *Resolver
	ownsResolver bool
}

// NewHistoricalHydrationService builds a scope traversal/hydration service.
func NewHistoricalHydrationService(cfg HistoricalHydrationServiceConfig) (*HistoricalHydrationService, error) {
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

	return &HistoricalHydrationService{
		client:       client,
		resolver:     resolver,
		ownsResolver: ownsResolver,
	}, nil
}

// Close persists resolver state when owned by this service.
func (s *HistoricalHydrationService) Close() error {
	if !s.ownsResolver || s.resolver == nil {
		return nil
	}
	return s.resolver.Close()
}

// BeginScope resolves the requested historical scope and returns a run-scoped hydration session.
func (s *HistoricalHydrationService) BeginScope(ctx context.Context, opts HistoricalScopeOptions) (*HistoricalScopeSession, error) {
	session := newHistoricalScopeSession(s.client, s.resolver, opts)

	if err := session.initialize(ctx); err != nil {
		return nil, err
	}
	return session, nil
}

// HistoricalScopeSession keeps one active run scope and reuses hydrated domain data in-process.
type HistoricalScopeSession struct {
	client   *Client
	resolver *Resolver
	opts     HistoricalScopeOptions

	league     League
	season     *Season
	seasonType *SeasonType
	group      *SeasonGroup
	rangeSpec  historicalDateRange

	warnings []string
	matches  []Match

	resolvedDocs      map[string]*ResolvedDocument
	statusByRef       map[string]matchStatusSnapshot
	teamIdentityByRef map[string]teamIdentity
	teamScoreByRef    map[string]string

	hydratedMatches []Match
	matchWarnings   []string

	inningsByMatch         map[string][]Innings
	ingsWarningsByMatch    map[string][]string
	playersByMatch         map[string][]PlayerMatch
	playerWarningsByMatch  map[string][]string
	deliveriesByMatch      map[string][]DeliveryEvent
	deliveryWarnByMatch    map[string][]string
	partnershipsByMatch    map[string][]Partnership
	partnershipWarnByMatch map[string][]string

	metrics HydrationMetrics
}

func newHistoricalScopeSession(client *Client, resolver *Resolver, opts HistoricalScopeOptions) *HistoricalScopeSession {
	return &HistoricalScopeSession{
		client:                 client,
		resolver:               resolver,
		opts:                   opts,
		resolvedDocs:           map[string]*ResolvedDocument{},
		statusByRef:            map[string]matchStatusSnapshot{},
		teamIdentityByRef:      map[string]teamIdentity{},
		teamScoreByRef:         map[string]string{},
		inningsByMatch:         map[string][]Innings{},
		ingsWarningsByMatch:    map[string][]string{},
		playersByMatch:         map[string][]PlayerMatch{},
		playerWarningsByMatch:  map[string][]string{},
		deliveriesByMatch:      map[string][]DeliveryEvent{},
		deliveryWarnByMatch:    map[string][]string{},
		partnershipsByMatch:    map[string][]Partnership{},
		partnershipWarnByMatch: map[string][]string{},
	}
}

// Scope returns resolved scope metadata.
func (s *HistoricalScopeSession) Scope() HistoricalScopeSummary {
	matchIDs := make([]string, 0, len(s.matches))
	for _, match := range s.matches {
		matchIDs = append(matchIDs, matchCacheKey(match))
	}

	summary := HistoricalScopeSummary{
		League:   s.league,
		DateFrom: strings.TrimSpace(s.opts.DateFrom),
		DateTo:   strings.TrimSpace(s.opts.DateTo),
		MatchIDs: matchIDs,
		Warnings: append([]string(nil), s.warnings...),
	}
	if s.season != nil {
		copySeason := *s.season
		summary.Season = &copySeason
	}
	if s.seasonType != nil {
		copyType := *s.seasonType
		summary.Type = &copyType
	}
	if s.group != nil {
		copyGroup := *s.group
		summary.Group = &copyGroup
	}
	return summary
}

// Warnings returns warnings produced while resolving the scope.
func (s *HistoricalScopeSession) Warnings() []string {
	return append([]string(nil), s.warnings...)
}

// ScopedMatches returns scope matches without additional hydration.
func (s *HistoricalScopeSession) ScopedMatches() []Match {
	return append([]Match(nil), s.matches...)
}

// Metrics reports in-process reuse stats for the active run.
func (s *HistoricalScopeSession) Metrics() HydrationMetrics {
	return s.metrics
}

// HydrateMatchSummaries hydrates status/team/score summaries for all scoped matches.
func (s *HistoricalScopeSession) HydrateMatchSummaries(ctx context.Context) ([]Match, []string, error) {
	if s.hydratedMatches != nil {
		s.metrics.DomainCacheHits++
		return append([]Match(nil), s.hydratedMatches...), append([]string(nil), s.matchWarnings...), nil
	}
	s.metrics.DomainCacheMisses++

	hydrated := append([]Match(nil), s.matches...)
	warnings := make([]string, 0)

	for i := range hydrated {
		warnings = append(warnings, s.hydrateMatchSummary(ctx, &hydrated[i])...)
	}

	s.hydratedMatches = hydrated
	s.matchWarnings = compactWarnings(warnings)
	return append([]Match(nil), hydrated...), append([]string(nil), s.matchWarnings...), nil
}

// HydrateInnings hydrates innings and timeline summaries for one scoped match.
func (s *HistoricalScopeSession) HydrateInnings(ctx context.Context, matchID string) ([]Innings, []string, error) {
	key, match, err := s.matchByID(matchID)
	if err != nil {
		return nil, nil, err
	}

	if cached, ok := s.inningsByMatch[key]; ok {
		s.metrics.DomainCacheHits++
		warnings := append([]string(nil), s.ingsWarningsByMatch[key]...)
		return append([]Innings(nil), cached...), warnings, nil
	}
	s.metrics.DomainCacheMisses++

	items := make([]Innings, 0)
	warnings := make([]string, 0)
	for _, team := range match.Teams {
		teamInnings, teamWarnings := s.collectTeamInnings(ctx, match, team)
		items = append(items, teamInnings...)
		warnings = append(warnings, teamWarnings...)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].TeamID != items[j].TeamID {
			return items[i].TeamID < items[j].TeamID
		}
		if items[i].InningsNumber != items[j].InningsNumber {
			return items[i].InningsNumber < items[j].InningsNumber
		}
		return items[i].Period < items[j].Period
	})

	s.inningsByMatch[key] = append([]Innings(nil), items...)
	s.ingsWarningsByMatch[key] = compactWarnings(warnings)
	return append([]Innings(nil), items...), append([]string(nil), s.ingsWarningsByMatch[key]...), nil
}

// HydratePlayerMatchSummaries hydrates match-context player summaries for one scoped match.
func (s *HistoricalScopeSession) HydratePlayerMatchSummaries(ctx context.Context, matchID string) ([]PlayerMatch, []string, error) {
	key, match, err := s.matchByID(matchID)
	if err != nil {
		return nil, nil, err
	}

	if cached, ok := s.playersByMatch[key]; ok {
		s.metrics.DomainCacheHits++
		warnings := append([]string(nil), s.playerWarningsByMatch[key]...)
		return append([]PlayerMatch(nil), cached...), warnings, nil
	}
	s.metrics.DomainCacheMisses++

	items := make([]PlayerMatch, 0)
	warnings := make([]string, 0)
	for _, team := range match.Teams {
		team = s.enrichTeamIdentityFromIndex(team)
		rosterRef := nonEmpty(strings.TrimSpace(team.RosterRef), competitorSubresourceRef(match, team.ID, "roster"))
		if rosterRef == "" {
			warnings = append(warnings, fmt.Sprintf("roster route unavailable for team %q", team.ID))
			continue
		}

		resolved, err := s.resolve(ctx, rosterRef)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("roster %s: %v", rosterRef, err))
			continue
		}

		entries, err := NormalizeTeamRosterEntries(resolved.Body, team, TeamScopeMatch, match.ID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("roster %s: %v", resolved.CanonicalRef, err))
			continue
		}

		for _, entry := range entries {
			playerID := strings.TrimSpace(entry.PlayerID)
			if playerID == "" {
				continue
			}
			if strings.TrimSpace(entry.DisplayName) == "" && s.resolver != nil {
				_ = s.resolver.seedPlayerByID(ctx, playerID, match.LeagueID, match.ID)
			}
			entry = s.enrichRosterEntryFromIndex(entry)

			statsRef := rosterPlayerStatisticsRef(match, team, entry)
			if statsRef == "" {
				warnings = append(warnings, fmt.Sprintf("player %s has no match statistics route", playerID))
				continue
			}

			statsDoc, err := s.resolve(ctx, statsRef)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("player statistics %s: %v", statsRef, err))
				continue
			}

			categories, err := NormalizeStatCategories(statsDoc.Body)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("player statistics %s: %v", statsDoc.CanonicalRef, err))
				continue
			}

			batting, bowling, fielding := splitPlayerStatCategories(categories)
			items = append(items, PlayerMatch{
				PlayerID:      playerID,
				PlayerRef:     entry.PlayerRef,
				PlayerName:    nonEmpty(entry.DisplayName, "Unknown Player"),
				MatchID:       match.ID,
				CompetitionID: nonEmpty(match.CompetitionID, match.ID),
				EventID:       match.EventID,
				LeagueID:      match.LeagueID,
				TeamID:        team.ID,
				TeamName:      nonEmpty(team.ShortName, team.Name, "Unknown Team"),
				StatisticsRef: statsDoc.CanonicalRef,
				LinescoresRef: rosterPlayerLinescoresRef(match, team, entry),
				Batting:       batting,
				Bowling:       bowling,
				Fielding:      fielding,
				Summary:       summarizePlayerMatchCategories(categories),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].TeamID != items[j].TeamID {
			return items[i].TeamID < items[j].TeamID
		}
		if items[i].PlayerName != items[j].PlayerName {
			return items[i].PlayerName < items[j].PlayerName
		}
		return items[i].PlayerID < items[j].PlayerID
	})

	s.playersByMatch[key] = append([]PlayerMatch(nil), items...)
	s.playerWarningsByMatch[key] = compactWarnings(warnings)
	return append([]PlayerMatch(nil), items...), append([]string(nil), s.playerWarningsByMatch[key]...), nil
}

// HydrateDeliverySummaries hydrates delivery events for one scoped match.
func (s *HistoricalScopeSession) HydrateDeliverySummaries(ctx context.Context, matchID string) ([]DeliveryEvent, []string, error) {
	key, match, err := s.matchByID(matchID)
	if err != nil {
		return nil, nil, err
	}

	if cached, ok := s.deliveriesByMatch[key]; ok {
		s.metrics.DomainCacheHits++
		warnings := append([]string(nil), s.deliveryWarnByMatch[key]...)
		return append([]DeliveryEvent(nil), cached...), warnings, nil
	}
	s.metrics.DomainCacheMisses++

	detailsRef := nonEmpty(strings.TrimSpace(match.DetailsRef), matchSubresourceRef(match, "details", "details"))
	if detailsRef == "" {
		s.deliveriesByMatch[key] = []DeliveryEvent{}
		s.deliveryWarnByMatch[key] = []string{fmt.Sprintf("details route unavailable for match %q", key)}
		return []DeliveryEvent{}, append([]string(nil), s.deliveryWarnByMatch[key]...), nil
	}

	resolved, err := s.resolve(ctx, detailsRef)
	if err != nil {
		return nil, nil, err
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("decode details page %q: %w", resolved.CanonicalRef, err)
	}

	warnings := make([]string, 0)
	pageItems := append([]Ref(nil), page.Items...)
	if page.PageCount > 1 {
		helper := &MatchService{client: s.client, resolver: s.resolver}
		extraItems, pageWarnings, pageErr := helper.resolvePageRefs(ctx, resolved)
		if pageErr != nil {
			warnings = append(warnings, pageErr.Error())
		} else {
			pageItems = extraItems
			warnings = append(warnings, pageWarnings...)
		}
	}

	helper := &MatchService{client: s.client, resolver: s.resolver}
	loaded, loadWarnings := helper.loadDeliveryEvents(ctx, pageItems)
	warnings = append(warnings, loadWarnings...)

	items := make([]DeliveryEvent, 0, len(loaded))
	for _, delivery := range loaded {
		delivery.MatchID = nonEmpty(delivery.MatchID, match.ID)
		delivery.CompetitionID = nonEmpty(delivery.CompetitionID, match.CompetitionID, match.ID)
		delivery.EventID = nonEmpty(delivery.EventID, match.EventID)
		delivery.LeagueID = nonEmpty(delivery.LeagueID, match.LeagueID)
		items = append(items, delivery)
	}

	s.deliveriesByMatch[key] = append([]DeliveryEvent(nil), items...)
	s.deliveryWarnByMatch[key] = compactWarnings(warnings)
	return append([]DeliveryEvent(nil), items...), append([]string(nil), s.deliveryWarnByMatch[key]...), nil
}

// HydratePartnershipSummaries hydrates detailed partnerships for one scoped match.
func (s *HistoricalScopeSession) HydratePartnershipSummaries(ctx context.Context, matchID string) ([]Partnership, []string, error) {
	key, match, err := s.matchByID(matchID)
	if err != nil {
		return nil, nil, err
	}

	if cached, ok := s.partnershipsByMatch[key]; ok {
		s.metrics.DomainCacheHits++
		warnings := append([]string(nil), s.partnershipWarnByMatch[key]...)
		return append([]Partnership(nil), cached...), warnings, nil
	}
	s.metrics.DomainCacheMisses++

	innings, inningsWarnings, err := s.HydrateInnings(ctx, key)
	if err != nil {
		return nil, nil, err
	}

	items := make([]Partnership, 0)
	warnings := append([]string{}, inningsWarnings...)
	for _, inn := range innings {
		ref := strings.TrimSpace(inn.PartnershipsRef)
		if ref == "" {
			continue
		}

		pageDoc, err := s.resolve(ctx, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("partnerships %s: %v", ref, err))
			continue
		}

		page, err := DecodePage[Ref](pageDoc.Body)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("decode partnerships page %s: %v", pageDoc.CanonicalRef, err))
			continue
		}

		for _, item := range page.Items {
			itemRef := strings.TrimSpace(item.URL)
			if itemRef == "" {
				continue
			}
			itemDoc, err := s.resolve(ctx, itemRef)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("partnership %s: %v", itemRef, err))
				continue
			}
			partnership, err := NormalizePartnership(itemDoc.Body)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("partnership %s: %v", itemDoc.CanonicalRef, err))
				continue
			}

			partnership.MatchID = nonEmpty(partnership.MatchID, match.ID)
			partnership.TeamID = nonEmpty(partnership.TeamID, inn.TeamID)
			partnership.TeamName = nonEmpty(partnership.TeamName, inn.TeamName)
			partnership.InningsID = nonEmpty(partnership.InningsID, fmt.Sprintf("%d", inn.InningsNumber))
			partnership.Period = nonEmpty(partnership.Period, fmt.Sprintf("%d", inn.Period))
			if partnership.Order == 0 {
				partnership.Order = partnership.WicketNumber
			}
			items = append(items, *partnership)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].TeamID != items[j].TeamID {
			return items[i].TeamID < items[j].TeamID
		}
		if items[i].InningsID != items[j].InningsID {
			return items[i].InningsID < items[j].InningsID
		}
		if items[i].Period != items[j].Period {
			return items[i].Period < items[j].Period
		}
		if items[i].Order != items[j].Order {
			return items[i].Order < items[j].Order
		}
		if items[i].Runs != items[j].Runs {
			return items[i].Runs > items[j].Runs
		}
		return items[i].ID < items[j].ID
	})

	s.partnershipsByMatch[key] = append([]Partnership(nil), items...)
	s.partnershipWarnByMatch[key] = compactWarnings(warnings)
	return append([]Partnership(nil), items...), append([]string(nil), s.partnershipWarnByMatch[key]...), nil
}

func (s *HistoricalScopeSession) initialize(ctx context.Context) error {
	leagueQuery := strings.TrimSpace(s.opts.LeagueQuery)
	if leagueQuery == "" {
		return fmt.Errorf("league query is required")
	}

	league, warnings, err := s.resolveLeague(ctx, leagueQuery)
	if err != nil {
		return err
	}
	s.league = league
	s.warnings = append(s.warnings, warnings...)

	if strings.TrimSpace(s.opts.SeasonQuery) != "" {
		season, seasonWarnings, seasonErr := s.resolveSeason(ctx, league, s.opts.SeasonQuery)
		if seasonErr != nil {
			return seasonErr
		}
		s.season = season
		s.warnings = append(s.warnings, seasonWarnings...)
	}

	if strings.TrimSpace(s.opts.TypeQuery) != "" || strings.TrimSpace(s.opts.GroupQuery) != "" {
		if s.season == nil {
			return fmt.Errorf("season query is required when type or group scope is requested")
		}
	}

	if strings.TrimSpace(s.opts.TypeQuery) != "" {
		seasonType, typeWarnings, typeErr := s.resolveSeasonType(ctx, *s.season, s.opts.TypeQuery)
		if typeErr != nil {
			return typeErr
		}
		s.seasonType = seasonType
		s.warnings = append(s.warnings, typeWarnings...)
	}

	if strings.TrimSpace(s.opts.GroupQuery) != "" {
		group, groupWarnings, groupErr := s.resolveSeasonGroup(ctx, *s.season, s.seasonType, s.opts.TypeQuery, s.opts.GroupQuery)
		if groupErr != nil {
			return groupErr
		}
		s.group = group
		s.warnings = append(s.warnings, groupWarnings...)
	}

	dateRange, dateWarnings, err := buildHistoricalDateRange(s.opts, s.season, s.seasonType)
	if err != nil {
		return err
	}
	s.rangeSpec = dateRange
	s.warnings = append(s.warnings, dateWarnings...)

	groupTeamIDs, groupWarnings, err := s.collectGroupTeamIDs(ctx, s.group)
	if err != nil {
		return err
	}
	s.warnings = append(s.warnings, groupWarnings...)

	matches, matchWarnings, err := s.collectScopedMatches(ctx, league, dateRange, groupTeamIDs)
	if err != nil {
		return err
	}
	s.warnings = append(s.warnings, matchWarnings...)

	if s.opts.MatchLimit > 0 && len(matches) > s.opts.MatchLimit {
		matches = matches[:s.opts.MatchLimit]
	}
	s.matches = matches
	s.warnings = compactWarnings(s.warnings)
	return nil
}

func (s *HistoricalScopeSession) resolveLeague(ctx context.Context, query string) (League, []string, error) {
	query = strings.TrimSpace(query)
	searchResult, err := s.resolver.Search(ctx, EntityLeague, query, ResolveOptions{Limit: 5})
	if err != nil {
		return League{}, nil, err
	}

	warnings := append([]string{}, searchResult.Warnings...)
	if len(searchResult.Entities) > 0 {
		entity := searchResult.Entities[0]
		ref := nonEmpty(strings.TrimSpace(entity.Ref), "/leagues/"+strings.TrimSpace(entity.ID))
		league, err := s.fetchLeagueByRef(ctx, ref)
		return league, warnings, err
	}

	if isKnownRefQuery(query) {
		league, err := s.fetchLeagueByRef(ctx, query)
		return league, warnings, err
	}
	if isNumeric(query) {
		league, err := s.fetchLeagueByRef(ctx, "/leagues/"+query)
		return league, warnings, err
	}

	resolved, err := s.resolve(ctx, "/leagues")
	if err != nil {
		return League{}, warnings, err
	}
	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return League{}, warnings, fmt.Errorf("decode /leagues page: %w", err)
	}
	needle := normalizeAlias(query)
	for _, item := range page.Items {
		ref := strings.TrimSpace(item.URL)
		if ref == "" {
			continue
		}
		league, lookupErr := s.fetchLeagueByRef(ctx, ref)
		if lookupErr != nil {
			warnings = append(warnings, fmt.Sprintf("league fallback %s: %v", ref, lookupErr))
			continue
		}
		aliases := []string{league.ID, league.Name, league.Slug}
		for _, alias := range aliases {
			if normalizeAlias(alias) != "" && normalizeAlias(alias) == needle {
				return league, compactWarnings(warnings), nil
			}
		}
	}

	return League{}, compactWarnings(warnings), fmt.Errorf("no leagues found for %q", query)
}

func (s *HistoricalScopeSession) fetchLeagueByRef(ctx context.Context, ref string) (League, error) {
	resolved, err := s.resolve(ctx, ref)
	if err != nil {
		return League{}, err
	}
	league, err := NormalizeLeague(resolved.Body)
	if err != nil {
		return League{}, fmt.Errorf("normalize league %q: %w", resolved.CanonicalRef, err)
	}
	if strings.TrimSpace(league.ID) == "" {
		league.ID = strings.TrimSpace(refIDs(resolved.CanonicalRef)["leagueId"])
	}
	if strings.TrimSpace(league.Ref) == "" {
		league.Ref = resolved.CanonicalRef
	}
	return *league, nil
}

func (s *HistoricalScopeSession) resolveSeason(ctx context.Context, league League, query string) (*Season, []string, error) {
	seasons, warnings, err := s.fetchLeagueSeasons(ctx, league)
	if err != nil {
		return nil, warnings, err
	}

	query = strings.TrimSpace(query)
	selectedRef := ""
	queryIDs := refIDs(query)
	for _, season := range seasons {
		ids := refIDs(season.Ref)
		candidates := []string{
			strings.TrimSpace(season.ID),
			strings.TrimSpace(strconv.Itoa(season.Year)),
			strings.TrimSpace(ids["seasonId"]),
			strings.TrimSpace(queryIDs["seasonId"]),
		}
		for _, candidate := range candidates {
			if candidate != "" && strings.EqualFold(candidate, query) {
				selectedRef = strings.TrimSpace(season.Ref)
				break
			}
		}
		if selectedRef != "" {
			break
		}
	}

	if selectedRef == "" && isKnownRefQuery(query) {
		selectedRef = query
	}
	if selectedRef == "" && isNumeric(query) {
		selectedRef = "/leagues/" + strings.TrimSpace(league.ID) + "/seasons/" + query
	}
	if selectedRef == "" {
		return nil, warnings, fmt.Errorf("season %q not found for league %q", query, league.ID)
	}

	resolved, err := s.resolve(ctx, selectedRef)
	if err != nil {
		return nil, warnings, err
	}
	season, err := NormalizeSeason(resolved.Body)
	if err != nil {
		return nil, warnings, fmt.Errorf("normalize season %q: %w", resolved.CanonicalRef, err)
	}
	if strings.TrimSpace(season.LeagueID) == "" {
		season.LeagueID = strings.TrimSpace(league.ID)
	}
	return season, warnings, nil
}

func (s *HistoricalScopeSession) fetchLeagueSeasons(ctx context.Context, league League) ([]Season, []string, error) {
	seasonsRef := nonEmpty(extensionRef(league.Extensions, "seasons"), "/leagues/"+strings.TrimSpace(league.ID)+"/seasons")
	resolved, err := s.resolve(ctx, seasonsRef)
	if err != nil {
		return nil, nil, err
	}
	seasons, err := NormalizeSeasonList(resolved.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("normalize seasons list %q: %w", resolved.CanonicalRef, err)
	}
	for i := range seasons {
		if strings.TrimSpace(seasons[i].LeagueID) == "" {
			seasons[i].LeagueID = strings.TrimSpace(league.ID)
		}
	}
	return seasons, nil, nil
}

func (s *HistoricalScopeSession) resolveSeasonType(ctx context.Context, season Season, query string) (*SeasonType, []string, error) {
	types, warnings, err := s.fetchSeasonTypes(ctx, season)
	if err != nil {
		return nil, warnings, err
	}

	query = strings.TrimSpace(query)
	queryIDs := refIDs(query)
	queryNorm := normalizeAlias(query)
	for _, seasonType := range types {
		candidates := []string{
			strings.TrimSpace(seasonType.ID),
			strings.TrimSpace(refIDs(seasonType.Ref)["typeId"]),
			strings.TrimSpace(queryIDs["typeId"]),
		}
		for _, candidate := range candidates {
			if candidate != "" && strings.EqualFold(candidate, query) {
				typed := seasonType
				return &typed, warnings, nil
			}
		}

		names := []string{seasonType.Name, seasonType.Abbreviation}
		for _, name := range names {
			if normalizeAlias(name) != "" && normalizeAlias(name) == queryNorm {
				typed := seasonType
				return &typed, warnings, nil
			}
		}
	}

	return nil, warnings, fmt.Errorf("season type %q not found for season %q", query, season.ID)
}

func (s *HistoricalScopeSession) fetchSeasonTypes(ctx context.Context, season Season) ([]SeasonType, []string, error) {
	typesRef := seasonTypesRef(season)
	if strings.TrimSpace(typesRef) == "" {
		return []SeasonType{}, nil, fmt.Errorf("season types route unavailable for season %q", season.ID)
	}

	resolved, err := s.resolve(ctx, typesRef)
	if err != nil {
		return nil, nil, err
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("decode season types page %q: %w", resolved.CanonicalRef, err)
	}

	items := make([]SeasonType, 0, len(page.Items))
	warnings := make([]string, 0)
	for _, item := range page.Items {
		ref := strings.TrimSpace(item.URL)
		if ref == "" {
			continue
		}
		itemDoc, err := s.resolve(ctx, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("season type %s: %v", ref, err))
			continue
		}
		seasonType, err := NormalizeSeasonType(itemDoc.Body)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("season type %s: %v", itemDoc.CanonicalRef, err))
			continue
		}
		if seasonType.SeasonID == "" {
			seasonType.SeasonID = season.ID
		}
		if seasonType.LeagueID == "" {
			seasonType.LeagueID = season.LeagueID
		}
		items = append(items, *seasonType)
	}

	return items, compactWarnings(warnings), nil
}

func (s *HistoricalScopeSession) resolveSeasonGroup(
	ctx context.Context,
	season Season,
	seasonType *SeasonType,
	typeQuery,
	groupQuery string,
) (*SeasonGroup, []string, error) {
	typeQuery = strings.TrimSpace(typeQuery)
	groupQuery = strings.TrimSpace(groupQuery)
	if groupQuery == "" {
		return nil, nil, nil
	}

	candidateTypes := make([]SeasonType, 0)
	warnings := make([]string, 0)
	if seasonType != nil {
		candidateTypes = append(candidateTypes, *seasonType)
	} else if typeQuery != "" {
		selected, selectedWarnings, err := s.resolveSeasonType(ctx, season, typeQuery)
		if err != nil {
			return nil, selectedWarnings, err
		}
		warnings = append(warnings, selectedWarnings...)
		candidateTypes = append(candidateTypes, *selected)
		s.seasonType = selected
	} else {
		items, typeWarnings, err := s.fetchSeasonTypes(ctx, season)
		if err != nil {
			return nil, typeWarnings, err
		}
		warnings = append(warnings, typeWarnings...)
		candidateTypes = append(candidateTypes, items...)
	}

	if len(candidateTypes) == 0 {
		return nil, warnings, fmt.Errorf("no season types available for season %q", season.ID)
	}

	queryIDs := refIDs(groupQuery)
	queryNorm := normalizeAlias(groupQuery)
	for _, candidateType := range candidateTypes {
		groups, groupWarnings, err := s.fetchSeasonGroups(ctx, candidateType)
		warnings = append(warnings, groupWarnings...)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}

		for _, group := range groups {
			candidates := []string{
				strings.TrimSpace(group.ID),
				strings.TrimSpace(refIDs(group.Ref)["groupId"]),
				strings.TrimSpace(queryIDs["groupId"]),
			}
			for _, id := range candidates {
				if id != "" && strings.EqualFold(id, groupQuery) {
					selected := group
					if s.seasonType == nil {
						typed := candidateType
						s.seasonType = &typed
					}
					return &selected, compactWarnings(warnings), nil
				}
			}

			names := []string{group.Name, group.Abbreviation}
			for _, name := range names {
				if normalizeAlias(name) != "" && normalizeAlias(name) == queryNorm {
					selected := group
					if s.seasonType == nil {
						typed := candidateType
						s.seasonType = &typed
					}
					return &selected, compactWarnings(warnings), nil
				}
			}
		}
	}

	return nil, compactWarnings(warnings), fmt.Errorf("season group %q not found", groupQuery)
}

func (s *HistoricalScopeSession) fetchSeasonGroups(ctx context.Context, seasonType SeasonType) ([]SeasonGroup, []string, error) {
	groupsRef := seasonGroupsRef(seasonType)
	if strings.TrimSpace(groupsRef) == "" {
		return []SeasonGroup{}, nil, fmt.Errorf("season groups route unavailable for season type %q", seasonType.ID)
	}

	resolved, err := s.resolve(ctx, groupsRef)
	if err != nil {
		return nil, nil, err
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("decode season groups page %q: %w", resolved.CanonicalRef, err)
	}

	items := make([]SeasonGroup, 0, len(page.Items))
	warnings := make([]string, 0)
	for _, item := range page.Items {
		ref := strings.TrimSpace(item.URL)
		if ref == "" {
			continue
		}

		itemDoc, err := s.resolve(ctx, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("season group %s: %v", ref, err))
			continue
		}

		group, err := NormalizeSeasonGroup(itemDoc.Body)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("season group %s: %v", itemDoc.CanonicalRef, err))
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
		items = append(items, *group)
	}

	return items, compactWarnings(warnings), nil
}

func (s *HistoricalScopeSession) collectGroupTeamIDs(ctx context.Context, group *SeasonGroup) (map[string]struct{}, []string, error) {
	if group == nil || strings.TrimSpace(group.StandingsRef) == "" {
		return nil, nil, nil
	}

	groups, warnings, err := s.collectStandingsGroups(ctx, group.StandingsRef, map[string]struct{}{}, 0)
	if err != nil {
		return nil, warnings, err
	}

	teamIDs := map[string]struct{}{}
	for _, item := range groups {
		for _, entry := range item.Entries {
			teamID := strings.TrimSpace(entry.ID)
			if teamID == "" {
				teamID = strings.TrimSpace(refIDs(entry.Ref)["teamId"])
			}
			if teamID == "" {
				teamID = strings.TrimSpace(refIDs(entry.Ref)["competitorId"])
			}
			if teamID == "" {
				continue
			}
			teamIDs[teamID] = struct{}{}
		}
	}

	if len(teamIDs) == 0 {
		warnings = append(warnings, fmt.Sprintf("season group %q did not yield any standings team ids", group.ID))
	}
	return teamIDs, compactWarnings(warnings), nil
}

func (s *HistoricalScopeSession) collectStandingsGroups(
	ctx context.Context,
	ref string,
	visited map[string]struct{},
	depth int,
) ([]StandingsGroup, []string, error) {
	if depth > maxStandingsTraversalDepth {
		return nil, nil, fmt.Errorf("standings traversal exceeded max depth for %q", ref)
	}

	resolved, err := s.resolve(ctx, ref)
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

func (s *HistoricalScopeSession) collectScopedMatches(
	ctx context.Context,
	league League,
	rangeSpec historicalDateRange,
	groupTeamIDs map[string]struct{},
) ([]Match, []string, error) {
	eventsRef := nonEmpty(extensionRef(league.Extensions, "events"), "/leagues/"+strings.TrimSpace(league.ID)+"/events")
	resolved, err := s.resolve(ctx, eventsRef)
	if err != nil {
		return nil, nil, err
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("decode league events page %q: %w", resolved.CanonicalRef, err)
	}

	warnings := make([]string, 0)
	seen := map[string]struct{}{}
	matches := make([]Match, 0)
	for _, item := range page.Items {
		ref := strings.TrimSpace(item.URL)
		if ref == "" {
			continue
		}

		eventDoc, err := s.resolve(ctx, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("event %s: %v", ref, err))
			continue
		}

		eventMatches, err := NormalizeMatchesFromEvent(eventDoc.Body)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("event %s: %v", eventDoc.CanonicalRef, err))
			continue
		}

		for _, match := range eventMatches {
			if strings.TrimSpace(match.LeagueID) == "" {
				match.LeagueID = strings.TrimSpace(league.ID)
			}

			allowed, reason := matchAllowedInScope(match, rangeSpec, groupTeamIDs)
			if !allowed {
				if reason != "" {
					warnings = append(warnings, reason)
				}
				continue
			}

			key := matchCacheKey(match)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			matches = append(matches, match)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		leftTime, leftOK := parseMatchTime(matches[i])
		rightTime, rightOK := parseMatchTime(matches[j])
		if leftOK && rightOK {
			if !leftTime.Equal(rightTime) {
				return leftTime.Before(rightTime)
			}
		}
		if matches[i].Date != matches[j].Date {
			return matches[i].Date < matches[j].Date
		}
		return matchCacheKey(matches[i]) < matchCacheKey(matches[j])
	})

	return matches, compactWarnings(warnings), nil
}

func (s *HistoricalScopeSession) hydrateMatchSummary(ctx context.Context, match *Match) []string {
	if match == nil {
		return nil
	}

	warnings := make([]string, 0)
	if statusRef := strings.TrimSpace(match.StatusRef); statusRef != "" {
		snapshot, err := s.resolveStatus(ctx, statusRef)
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
			identity, err := s.resolveTeamIdentity(ctx, team)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("team %s: %v", nonEmpty(team.Ref, team.ID), err))
			} else {
				if strings.TrimSpace(team.Name) == "" {
					team.Name = identity.name
				}
				if strings.TrimSpace(team.ShortName) == "" {
					team.ShortName = identity.shortName
				}
			}
		}

		if strings.TrimSpace(team.ScoreSummary) == "" && strings.TrimSpace(team.ScoreRef) != "" {
			score, err := s.resolveTeamScore(ctx, team.ScoreRef)
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

func (s *HistoricalScopeSession) resolveStatus(ctx context.Context, ref string) (matchStatusSnapshot, error) {
	ref = strings.TrimSpace(ref)
	if cached, ok := s.statusByRef[ref]; ok {
		return cached, nil
	}

	resolved, err := s.resolve(ctx, ref)
	if err != nil {
		return matchStatusSnapshot{}, err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return matchStatusSnapshot{}, err
	}

	typed := mapField(payload, "type")
	snapshot := matchStatusSnapshot{
		summary:     stringField(payload, "summary"),
		longSummary: stringField(payload, "longSummary"),
		state:       stringField(typed, "state"),
		detail:      stringField(typed, "detail"),
		shortDetail: stringField(typed, "shortDetail"),
		description: stringField(typed, "description"),
	}
	s.statusByRef[ref] = snapshot
	return snapshot, nil
}

func (s *HistoricalScopeSession) resolveTeamIdentity(ctx context.Context, team *Team) (teamIdentity, error) {
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
	if cached, ok := s.teamIdentityByRef[ref]; ok {
		return cached, nil
	}

	resolved, err := s.resolve(ctx, ref)
	if err != nil {
		return teamIdentity{}, err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return teamIdentity{}, err
	}

	identity := teamIdentity{
		name:      nonEmpty(stringField(payload, "displayName"), stringField(payload, "name"), strings.TrimSpace(team.ID)),
		shortName: nonEmpty(stringField(payload, "shortDisplayName"), stringField(payload, "shortName"), stringField(payload, "abbreviation")),
	}
	s.teamIdentityByRef[ref] = identity
	return identity, nil
}

func (s *HistoricalScopeSession) resolveTeamScore(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("score ref is empty")
	}
	if cached, ok := s.teamScoreByRef[ref]; ok {
		return cached, nil
	}

	resolved, err := s.resolve(ctx, ref)
	if err != nil {
		return "", err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return "", err
	}
	score := nonEmpty(stringField(payload, "displayValue"), stringField(payload, "value"), stringField(payload, "summary"))
	s.teamScoreByRef[ref] = score
	return score, nil
}

func (s *HistoricalScopeSession) collectTeamInnings(ctx context.Context, match Match, team Team) ([]Innings, []string) {
	candidates := compactWarnings([]string{
		strings.TrimSpace(team.LinescoresRef),
		strings.TrimSpace(competitorSubresourceRef(match, team.ID, "linescores")),
	})
	if len(candidates) == 0 {
		return nil, []string{fmt.Sprintf("linescores route unavailable for team %q", team.ID)}
	}

	warnings := make([]string, 0)
	seen := map[string]struct{}{}
	for _, ref := range candidates {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}

		resolved, err := s.resolve(ctx, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("linescores %s: %v", ref, err))
			continue
		}

		innings, collectWarnings, err := s.collectInningsFromPayload(ctx, resolved.Body)
		warnings = append(warnings, collectWarnings...)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("linescores %s: %v", ref, err))
			continue
		}

		for i := range innings {
			innings[i].TeamID = nonEmpty(strings.TrimSpace(team.ID), innings[i].TeamID)
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
			warnings = append(warnings, s.hydrateInningsTimeline(ctx, &innings[i])...)
		}

		if len(innings) > 0 {
			return innings, compactWarnings(warnings)
		}
	}

	return nil, compactWarnings(warnings)
}

func (s *HistoricalScopeSession) collectInningsFromPayload(ctx context.Context, body []byte) ([]Innings, []string, error) {
	payload, err := decodePayloadMap(body)
	if err != nil {
		return nil, nil, err
	}

	warnings := make([]string, 0)
	items := make([]Innings, 0)

	appendInningsMap := func(row map[string]any) {
		if row == nil {
			return
		}
		if stringField(row, "$ref") == "" && intField(row, "period") == 0 && intField(row, "runs") == 0 && intField(row, "wickets") == 0 && stringField(row, "score") == "" {
			return
		}
		items = append(items, *normalizeInningsFromMap(row))
	}

	rows := mapSliceField(payload, "items")
	if len(rows) > 0 {
		for _, row := range rows {
			itemRef := strings.TrimSpace(stringField(row, "$ref"))
			if itemRef != "" && intField(row, "period") == 0 && stringField(row, "score") == "" && intField(row, "runs") == 0 && intField(row, "wickets") == 0 {
				itemDoc, err := s.resolve(ctx, itemRef)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("innings %s: %v", itemRef, err))
					continue
				}
				normalized, err := NormalizeInnings(itemDoc.Body)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("innings %s: %v", itemDoc.CanonicalRef, err))
					continue
				}
				items = append(items, *normalized)
				continue
			}
			appendInningsMap(row)
		}
		return items, compactWarnings(warnings), nil
	}

	appendInningsMap(payload)
	return items, compactWarnings(warnings), nil
}

func (s *HistoricalScopeSession) hydrateInningsTimeline(ctx context.Context, innings *Innings) []string {
	if innings == nil || strings.TrimSpace(innings.StatisticsRef) == "" {
		return nil
	}

	resolved, err := s.resolve(ctx, innings.StatisticsRef)
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

func (s *HistoricalScopeSession) matchByID(matchID string) (string, Match, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		if len(s.matches) == 0 {
			return "", Match{}, fmt.Errorf("scope produced no matches")
		}
		first := s.matches[0]
		return matchCacheKey(first), first, nil
	}

	for _, match := range s.matches {
		ids := []string{
			matchCacheKey(match),
			strings.TrimSpace(match.ID),
			strings.TrimSpace(match.CompetitionID),
			strings.TrimSpace(refIDs(match.Ref)["competitionId"]),
			strings.TrimSpace(refIDs(match.Ref)["eventId"]),
		}
		for _, id := range ids {
			if id != "" && id == matchID {
				return matchCacheKey(match), match, nil
			}
		}
	}

	return "", Match{}, fmt.Errorf("match %q is outside the active scope", matchID)
}

func (s *HistoricalScopeSession) resolve(ctx context.Context, ref string) (*ResolvedDocument, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("ref is empty")
	}

	if cached, ok := s.resolvedDocs[ref]; ok {
		s.metrics.ResolveCacheHits++
		return cached, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return nil, err
	}
	s.metrics.ResolveCacheMisses++

	copied := *resolved
	pointer := &copied
	keys := compactWarnings([]string{ref, copied.RequestedRef, copied.CanonicalRef})
	for _, key := range keys {
		s.resolvedDocs[key] = pointer
	}
	return pointer, nil
}

type historicalDateRange struct {
	from    time.Time
	to      time.Time
	hasFrom bool
	hasTo   bool
}

func buildHistoricalDateRange(opts HistoricalScopeOptions, season *Season, seasonType *SeasonType) (historicalDateRange, []string, error) {
	rangeSpec := historicalDateRange{}
	warnings := make([]string, 0)

	if rawFrom := strings.TrimSpace(opts.DateFrom); rawFrom != "" {
		parsed, err := parseScopeDate(rawFrom, false)
		if err != nil {
			return rangeSpec, warnings, fmt.Errorf("invalid --date-from value %q: %w", rawFrom, err)
		}
		rangeSpec.from = parsed
		rangeSpec.hasFrom = true
	}
	if rawTo := strings.TrimSpace(opts.DateTo); rawTo != "" {
		parsed, err := parseScopeDate(rawTo, true)
		if err != nil {
			return rangeSpec, warnings, fmt.Errorf("invalid --date-to value %q: %w", rawTo, err)
		}
		rangeSpec.to = parsed
		rangeSpec.hasTo = true
	}

	if season != nil && season.Year > 0 {
		seasonStart := time.Date(season.Year, time.January, 1, 0, 0, 0, 0, time.UTC)
		seasonEnd := time.Date(season.Year, time.December, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
		rangeSpec = intersectDateRange(rangeSpec, seasonStart, seasonEnd)
	}

	if seasonType != nil {
		if rawStart := strings.TrimSpace(seasonType.StartDate); rawStart != "" {
			parsed, err := parseScopeDate(rawStart, false)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("unable to parse season type start date %q: %v", rawStart, err))
			} else {
				rangeSpec = intersectDateRange(rangeSpec, parsed, time.Time{})
			}
		}
		if rawEnd := strings.TrimSpace(seasonType.EndDate); rawEnd != "" {
			parsed, err := parseScopeDate(rawEnd, true)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("unable to parse season type end date %q: %v", rawEnd, err))
			} else {
				rangeSpec = intersectDateRange(rangeSpec, time.Time{}, parsed)
			}
		}
	}

	if rangeSpec.hasFrom && rangeSpec.hasTo && rangeSpec.from.After(rangeSpec.to) {
		return rangeSpec, warnings, fmt.Errorf("date range is invalid: from %s is after %s", rangeSpec.from.Format(time.RFC3339), rangeSpec.to.Format(time.RFC3339))
	}

	return rangeSpec, compactWarnings(warnings), nil
}

func parseScopeDate(raw string, endOfDay bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	formats := []string{time.RFC3339, "2006-01-02"}
	for _, layout := range formats {
		parsed, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		parsed = parsed.UTC()
		if layout == "2006-01-02" && endOfDay {
			parsed = parsed.Add(24*time.Hour - time.Nanosecond)
		}
		return parsed, nil
	}

	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD")
}

func intersectDateRange(existing historicalDateRange, from, to time.Time) historicalDateRange {
	if !from.IsZero() {
		if !existing.hasFrom || from.After(existing.from) {
			existing.from = from
			existing.hasFrom = true
		}
	}
	if !to.IsZero() {
		if !existing.hasTo || to.Before(existing.to) {
			existing.to = to
			existing.hasTo = true
		}
	}
	return existing
}

func matchAllowedInScope(match Match, rangeSpec historicalDateRange, groupTeamIDs map[string]struct{}) (bool, string) {
	if len(groupTeamIDs) > 0 {
		matched := false
		for _, team := range match.Teams {
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
			if _, ok := groupTeamIDs[teamID]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false, ""
		}
	}

	if !rangeSpec.hasFrom && !rangeSpec.hasTo {
		return true, ""
	}

	matchTime, ok := parseMatchTime(match)
	if !ok {
		return false, fmt.Sprintf("skip match %s: unable to parse date %q", matchCacheKey(match), match.Date)
	}
	if rangeSpec.hasFrom && matchTime.Before(rangeSpec.from) {
		return false, ""
	}
	if rangeSpec.hasTo && matchTime.After(rangeSpec.to) {
		return false, ""
	}
	return true, ""
}

func parseMatchTime(match Match) (time.Time, bool) {
	value := strings.TrimSpace(match.Date)
	if value == "" {
		return time.Time{}, false
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04Z07:00",
		"2006-01-02T15:04Z",
		"2006-01-02",
	}
	for _, layout := range formats {
		parsed, err := time.Parse(layout, value)
		if err != nil {
			continue
		}
		return parsed.UTC(), true
	}

	return time.Time{}, false
}

func matchCacheKey(match Match) string {
	return firstNonEmptyString(
		strings.TrimSpace(match.ID),
		strings.TrimSpace(match.CompetitionID),
		strings.TrimSpace(refIDs(match.Ref)["competitionId"]),
		strings.TrimSpace(refIDs(match.Ref)["eventId"]),
		strings.TrimSpace(match.Ref),
	)
}

func (s *HistoricalScopeSession) enrichRosterEntryFromIndex(entry TeamRosterEntry) TeamRosterEntry {
	if s == nil || s.resolver == nil || s.resolver.index == nil {
		return entry
	}
	playerID := strings.TrimSpace(entry.PlayerID)
	if playerID == "" {
		return entry
	}
	player, ok := s.resolver.index.FindByID(EntityPlayer, playerID)
	if !ok {
		return entry
	}
	if strings.TrimSpace(entry.DisplayName) == "" {
		entry.DisplayName = nonEmpty(player.Name, player.ShortName)
	}
	if strings.TrimSpace(entry.PlayerRef) == "" {
		entry.PlayerRef = strings.TrimSpace(player.Ref)
	}
	return entry
}

func (s *HistoricalScopeSession) enrichTeamIdentityFromIndex(team Team) Team {
	if s == nil || s.resolver == nil || s.resolver.index == nil {
		return team
	}
	teamID := strings.TrimSpace(team.ID)
	if teamID == "" {
		teamID = strings.TrimSpace(refIDs(team.Ref)["teamId"])
	}
	if teamID == "" {
		teamID = strings.TrimSpace(refIDs(team.Ref)["competitorId"])
	}
	if teamID == "" {
		return team
	}
	indexed, ok := s.resolver.index.FindByID(EntityTeam, teamID)
	if !ok {
		return team
	}
	if strings.TrimSpace(team.Name) == "" {
		team.Name = strings.TrimSpace(indexed.Name)
	}
	if strings.TrimSpace(team.ShortName) == "" {
		team.ShortName = strings.TrimSpace(indexed.ShortName)
	}
	if strings.TrimSpace(team.Ref) == "" {
		team.Ref = strings.TrimSpace(indexed.Ref)
	}
	return team
}
