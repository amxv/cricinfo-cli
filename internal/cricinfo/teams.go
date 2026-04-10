package cricinfo

import (
	"context"
	"fmt"
	"strings"
)

// TeamServiceConfig configures team discovery and match-scoped team commands.
type TeamServiceConfig struct {
	Client   *Client
	Resolver *Resolver
}

// TeamLookupOptions controls resolver-backed team lookup behavior.
type TeamLookupOptions struct {
	LeagueID   string
	MatchQuery string
}

// TeamService implements domain-level team and competitor commands.
type TeamService struct {
	client       *Client
	resolver     *Resolver
	ownsResolver bool
}

// NewTeamService builds a team service using default client/resolver when omitted.
func NewTeamService(cfg TeamServiceConfig) (*TeamService, error) {
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

	return &TeamService{
		client:       client,
		resolver:     resolver,
		ownsResolver: ownsResolver,
	}, nil
}

// Close persists resolver cache when owned by this service.
func (s *TeamService) Close() error {
	if !s.ownsResolver || s.resolver == nil {
		return nil
	}
	return s.resolver.Close()
}

// Show resolves and returns one team summary, merging global team identity with match-scoped competitor fields when available.
func (s *TeamService) Show(ctx context.Context, teamQuery string, opts TeamLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveTeamLookup(ctx, teamQuery, opts)
	if passthrough != nil {
		passthrough.Kind = EntityTeam
		return *passthrough, nil
	}

	result := NewDataResult(EntityTeam, lookup.team)
	if len(lookup.warnings) > 0 {
		result = NewPartialResult(EntityTeam, lookup.team, lookup.warnings...)
	}
	if lookup.teamResolved != nil {
		result.RequestedRef = lookup.teamResolved.RequestedRef
		result.CanonicalRef = lookup.teamResolved.CanonicalRef
	}
	return result, nil
}

// Roster resolves and returns a team roster. Without --match it uses global team athletes; with --match it uses competitor roster.
func (s *TeamService) Roster(ctx context.Context, teamQuery string, opts TeamLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveTeamLookup(ctx, teamQuery, opts)
	if passthrough != nil {
		passthrough.Kind = EntityTeamRoster
		return *passthrough, nil
	}

	warnings := append([]string{}, lookup.warnings...)
	useMatchScope := strings.TrimSpace(opts.MatchQuery) != "" && lookup.match != nil

	rosterRef := ""
	scope := TeamScopeGlobal
	if useMatchScope {
		rosterRef = nonEmpty(strings.TrimSpace(lookup.team.RosterRef), competitorSubresourceRef(*lookup.match, lookup.team.ID, "roster"))
		scope = TeamScopeMatch
	} else {
		rosterRef = nonEmpty(extensionRef(lookup.team.Extensions, "athletes"), "/teams/"+strings.TrimSpace(lookup.team.ID)+"/athletes")
	}

	if strings.TrimSpace(rosterRef) == "" {
		result := NormalizedResult{
			Kind:    EntityTeamRoster,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("roster route unavailable for team %q", lookup.team.ID),
		}
		return result, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, rosterRef)
	if err != nil {
		return NewTransportErrorResult(EntityTeamRoster, rosterRef, err), nil
	}

	var entries []TeamRosterEntry
	if scope == TeamScopeMatch {
		entries, err = NormalizeTeamRosterEntries(resolved.Body, lookup.team, scope, lookup.match.ID)
	} else {
		entries, err = NormalizeTeamAthletePage(resolved.Body, lookup.team)
	}
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize team roster %q: %w", resolved.CanonicalRef, err)
	}

	s.enrichRosterEntries(entries)

	items := make([]any, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry)
	}

	result := NewListResult(EntityTeamRoster, items)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityTeamRoster, items, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Scores resolves and returns one match-scoped team score response.
func (s *TeamService) Scores(ctx context.Context, teamQuery string, opts TeamLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveTeamLookup(ctx, teamQuery, opts)
	if passthrough != nil {
		passthrough.Kind = EntityTeamScore
		return *passthrough, nil
	}
	if lookup.match == nil {
		return matchScopeRequiredResult(EntityTeamScore), nil
	}

	scoreRef := nonEmpty(strings.TrimSpace(lookup.team.ScoreRef), competitorSubresourceRef(*lookup.match, lookup.team.ID, "scores"))
	if scoreRef == "" {
		return NormalizedResult{Kind: EntityTeamScore, Status: ResultStatusEmpty, Message: fmt.Sprintf("score route unavailable for team %q", lookup.team.ID)}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, scoreRef)
	if err != nil {
		return NewTransportErrorResult(EntityTeamScore, scoreRef, err), nil
	}

	score, err := NormalizeTeamScore(resolved.Body, lookup.team, TeamScopeMatch, lookup.match.ID)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize team score %q: %w", resolved.CanonicalRef, err)
	}

	result := NewDataResult(EntityTeamScore, score)
	if len(lookup.warnings) > 0 {
		result = NewPartialResult(EntityTeamScore, score, lookup.warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Leaders resolves and returns one match-scoped team leaders payload.
func (s *TeamService) Leaders(ctx context.Context, teamQuery string, opts TeamLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveTeamLookup(ctx, teamQuery, opts)
	if passthrough != nil {
		passthrough.Kind = EntityTeamLeaders
		return *passthrough, nil
	}
	if lookup.match == nil {
		return matchScopeRequiredResult(EntityTeamLeaders), nil
	}

	leadersRef := nonEmpty(strings.TrimSpace(lookup.team.LeadersRef), competitorSubresourceRef(*lookup.match, lookup.team.ID, "leaders"))
	if leadersRef == "" {
		return NormalizedResult{Kind: EntityTeamLeaders, Status: ResultStatusEmpty, Message: fmt.Sprintf("leaders route unavailable for team %q", lookup.team.ID)}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, leadersRef)
	if err != nil {
		return NewTransportErrorResult(EntityTeamLeaders, leadersRef, err), nil
	}

	leaders, err := NormalizeTeamLeaders(resolved.Body, lookup.team, TeamScopeMatch, lookup.match.ID)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize team leaders %q: %w", resolved.CanonicalRef, err)
	}
	s.enrichTeamLeaders(leaders)

	result := NewDataResult(EntityTeamLeaders, leaders)
	if len(lookup.warnings) > 0 {
		result = NewPartialResult(EntityTeamLeaders, leaders, lookup.warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Statistics resolves and returns match-scoped team statistics categories.
func (s *TeamService) Statistics(ctx context.Context, teamQuery string, opts TeamLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveTeamLookup(ctx, teamQuery, opts)
	if passthrough != nil {
		passthrough.Kind = EntityTeamStatistics
		return *passthrough, nil
	}
	if lookup.match == nil {
		return matchScopeRequiredResult(EntityTeamStatistics), nil
	}

	statisticsRef := nonEmpty(strings.TrimSpace(lookup.team.StatisticsRef), competitorSubresourceRef(*lookup.match, lookup.team.ID, "statistics"))
	if statisticsRef == "" {
		return NormalizedResult{Kind: EntityTeamStatistics, Status: ResultStatusEmpty, Message: fmt.Sprintf("statistics route unavailable for team %q", lookup.team.ID)}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, statisticsRef)
	if err != nil {
		return NewTransportErrorResult(EntityTeamStatistics, statisticsRef, err), nil
	}

	categories, err := NormalizeStatCategories(resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize team statistics %q: %w", resolved.CanonicalRef, err)
	}

	items := make([]any, 0, len(categories))
	for _, category := range categories {
		items = append(items, category)
	}

	result := NewListResult(EntityTeamStatistics, items)
	if len(lookup.warnings) > 0 {
		result = NewPartialListResult(EntityTeamStatistics, items, lookup.warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Records resolves and returns match-scoped team records categories.
func (s *TeamService) Records(ctx context.Context, teamQuery string, opts TeamLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveTeamLookup(ctx, teamQuery, opts)
	if passthrough != nil {
		passthrough.Kind = EntityTeamRecords
		return *passthrough, nil
	}
	if lookup.match == nil {
		return matchScopeRequiredResult(EntityTeamRecords), nil
	}

	recordsRef := nonEmpty(strings.TrimSpace(lookup.team.RecordRef), competitorSubresourceRef(*lookup.match, lookup.team.ID, "records"))
	if recordsRef == "" {
		return NormalizedResult{Kind: EntityTeamRecords, Status: ResultStatusEmpty, Message: fmt.Sprintf("records route unavailable for team %q", lookup.team.ID)}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, recordsRef)
	if err != nil {
		return NewTransportErrorResult(EntityTeamRecords, recordsRef, err), nil
	}

	categories, err := NormalizeTeamRecordCategories(resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize team records %q: %w", resolved.CanonicalRef, err)
	}

	items := make([]any, 0, len(categories))
	for _, category := range categories {
		items = append(items, category)
	}

	result := NewListResult(EntityTeamRecords, items)
	if len(lookup.warnings) > 0 {
		result = NewPartialListResult(EntityTeamRecords, items, lookup.warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

type teamLookup struct {
	entity       IndexedEntity
	team         Team
	match        *Match
	teamResolved *ResolvedDocument
	warnings     []string
}

func (s *TeamService) resolveTeamLookup(ctx context.Context, teamQuery string, opts TeamLookupOptions) (*teamLookup, *NormalizedResult) {
	teamQuery = strings.TrimSpace(teamQuery)
	if teamQuery == "" {
		result := NormalizedResult{Kind: EntityTeam, Status: ResultStatusEmpty, Message: "team query is required"}
		return nil, &result
	}

	warnings := make([]string, 0)
	var match *Match
	if strings.TrimSpace(opts.MatchQuery) != "" {
		resolvedMatch, matchWarnings, passthrough := s.resolveMatchContext(ctx, opts)
		if passthrough != nil {
			return nil, passthrough
		}
		match = resolvedMatch
		warnings = append(warnings, matchWarnings...)
	}

	searchResult, err := s.resolver.Search(ctx, EntityTeam, teamQuery, ResolveOptions{
		Limit:    5,
		LeagueID: strings.TrimSpace(opts.LeagueID),
		MatchID:  teamLookupMatchID(match),
	})
	if err != nil {
		result := NewTransportErrorResult(EntityTeam, teamQuery, err)
		return nil, &result
	}
	if len(searchResult.Entities) == 0 {
		result := NormalizedResult{Kind: EntityTeam, Status: ResultStatusEmpty, Message: fmt.Sprintf("no teams found for %q", teamQuery)}
		return nil, &result
	}

	warnings = append(warnings, searchResult.Warnings...)
	entity := searchResult.Entities[0]
	team, teamResolved, teamWarning := s.fetchGlobalTeam(ctx, entity)
	if strings.TrimSpace(teamWarning) != "" {
		warnings = append(warnings, teamWarning)
	}

	if match != nil {
		if competitor := matchTeamByID(*match, entity.ID); competitor != nil {
			team = mergeTeamViews(team, *competitor, *match)
		} else {
			warnings = append(warnings, fmt.Sprintf("team %s not found in match %s competitors", entity.ID, match.ID))
		}
	} else {
		if team.Extensions == nil {
			team.Extensions = map[string]any{}
		}
		team.Extensions["scope"] = string(TeamScopeGlobal)
	}

	return &teamLookup{
		entity:       entity,
		team:         team,
		match:        match,
		teamResolved: teamResolved,
		warnings:     compactWarnings(warnings),
	}, nil
}

func (s *TeamService) resolveMatchContext(ctx context.Context, opts TeamLookupOptions) (*Match, []string, *NormalizedResult) {
	query := strings.TrimSpace(opts.MatchQuery)
	if query == "" {
		return nil, nil, nil
	}

	searchResult, err := s.resolver.Search(ctx, EntityMatch, query, ResolveOptions{Limit: 5, LeagueID: strings.TrimSpace(opts.LeagueID)})
	if err != nil {
		result := NewTransportErrorResult(EntityMatch, query, err)
		return nil, nil, &result
	}
	if len(searchResult.Entities) == 0 {
		result := NormalizedResult{Kind: EntityMatch, Status: ResultStatusEmpty, Message: fmt.Sprintf("no matches found for %q", query)}
		return nil, nil, &result
	}

	entity := searchResult.Entities[0]
	ref := buildMatchRef(entity)
	if strings.TrimSpace(ref) == "" {
		result := NormalizedResult{Kind: EntityMatch, Status: ResultStatusEmpty, Message: fmt.Sprintf("unable to resolve match ref for %q", query)}
		return nil, nil, &result
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		result := NewTransportErrorResult(EntityMatch, ref, err)
		return nil, nil, &result
	}

	match, err := NormalizeMatch(resolved.Body)
	if err != nil {
		result := NormalizedResult{Kind: EntityMatch, Status: ResultStatusError, Message: fmt.Sprintf("normalize competition match %q: %v", resolved.CanonicalRef, err)}
		return nil, nil, &result
	}

	warnings := append([]string{}, searchResult.Warnings...)
	return match, compactWarnings(warnings), nil
}

func (s *TeamService) fetchGlobalTeam(ctx context.Context, entity IndexedEntity) (Team, *ResolvedDocument, string) {
	fallback := Team{
		Ref:       entity.Ref,
		ID:        entity.ID,
		Name:      entity.Name,
		ShortName: entity.ShortName,
	}

	teamRef := strings.TrimSpace(entity.Ref)
	if teamRef == "" || strings.Contains(teamRef, "/competitors/") {
		teamRef = "/teams/" + strings.TrimSpace(entity.ID)
	}

	resolved, err := s.client.ResolveRefChain(ctx, teamRef)
	if err != nil {
		return fallback, nil, fmt.Sprintf("team %s: %v", entity.ID, err)
	}

	team, err := NormalizeTeam(resolved.Body)
	if err != nil {
		return fallback, resolved, fmt.Sprintf("team %s: %v", entity.ID, err)
	}
	if team.ID == "" {
		team.ID = entity.ID
	}
	if team.Name == "" {
		team.Name = entity.Name
	}
	if team.ShortName == "" {
		team.ShortName = entity.ShortName
	}
	if team.Extensions == nil {
		team.Extensions = map[string]any{}
	}
	team.Extensions["scope"] = string(TeamScopeGlobal)

	return *team, resolved, ""
}

func (s *TeamService) enrichRosterEntries(entries []TeamRosterEntry) {
	if s.resolver == nil || s.resolver.index == nil {
		return
	}

	for i := range entries {
		if entries[i].DisplayName != "" || strings.TrimSpace(entries[i].PlayerID) == "" {
			continue
		}
		if player, ok := s.resolver.index.FindByID(EntityPlayer, entries[i].PlayerID); ok {
			entries[i].DisplayName = nonEmpty(player.Name, player.ShortName)
		}
	}
}

func (s *TeamService) enrichTeamLeaders(leaders *TeamLeaders) {
	if leaders == nil || s.resolver == nil || s.resolver.index == nil {
		return
	}

	for categoryIndex := range leaders.Categories {
		for leaderIndex := range leaders.Categories[categoryIndex].Leaders {
			entry := &leaders.Categories[categoryIndex].Leaders[leaderIndex]
			if strings.TrimSpace(entry.AthleteName) != "" {
				continue
			}
			if strings.TrimSpace(entry.AthleteID) == "" {
				entry.AthleteName = entry.AthleteID
				continue
			}
			if player, ok := s.resolver.index.FindByID(EntityPlayer, entry.AthleteID); ok {
				entry.AthleteName = nonEmpty(player.Name, player.ShortName, entry.AthleteID)
			} else {
				entry.AthleteName = entry.AthleteID
			}
		}
	}
}

func matchScopeRequiredResult(kind EntityKind) NormalizedResult {
	return NormalizedResult{
		Kind:    kind,
		Status:  ResultStatusEmpty,
		Message: "match scope is required (use --match <match>)",
	}
}

func competitorSubresourceRef(match Match, teamID, suffix string) string {
	teamID = strings.TrimSpace(teamID)
	suffix = strings.Trim(strings.TrimSpace(suffix), "/")
	if teamID == "" {
		return ""
	}

	base := matchSubresourceRef(match, "", "")
	if base == "" {
		return ""
	}

	ref := strings.TrimRight(base, "/") + "/competitors/" + teamID
	if suffix != "" {
		ref += "/" + suffix
	}
	return ref
}

func matchTeamByID(match Match, teamID string) *Team {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil
	}

	for i := range match.Teams {
		candidate := &match.Teams[i]
		ids := []string{
			strings.TrimSpace(candidate.ID),
			strings.TrimSpace(refIDs(candidate.Ref)["teamId"]),
			strings.TrimSpace(refIDs(candidate.Ref)["competitorId"]),
		}
		for _, id := range ids {
			if id != "" && id == teamID {
				return candidate
			}
		}
	}

	return nil
}

func mergeTeamViews(global Team, competitor Team, match Match) Team {
	merged := global
	if merged.Ref == "" {
		merged.Ref = competitor.Ref
	}
	if merged.ID == "" {
		merged.ID = competitor.ID
	}
	if merged.Name == "" {
		merged.Name = competitor.Name
	}
	if merged.ShortName == "" {
		merged.ShortName = competitor.ShortName
	}
	if merged.Abbreviation == "" {
		merged.Abbreviation = competitor.Abbreviation
	}
	if competitor.ScoreSummary != "" {
		merged.ScoreSummary = competitor.ScoreSummary
	}
	if competitor.Type != "" {
		merged.Type = competitor.Type
	}
	if competitor.HomeAway != "" {
		merged.HomeAway = competitor.HomeAway
	}
	if competitor.Order != 0 {
		merged.Order = competitor.Order
	}
	merged.Winner = competitor.Winner
	merged.ScoreRef = nonEmpty(competitor.ScoreRef, merged.ScoreRef)
	merged.RosterRef = nonEmpty(competitor.RosterRef, merged.RosterRef)
	merged.LeadersRef = nonEmpty(competitor.LeadersRef, merged.LeadersRef)
	merged.StatisticsRef = nonEmpty(competitor.StatisticsRef, merged.StatisticsRef)
	merged.RecordRef = nonEmpty(competitor.RecordRef, merged.RecordRef)
	merged.LinescoresRef = nonEmpty(competitor.LinescoresRef, merged.LinescoresRef)

	if merged.Extensions == nil {
		merged.Extensions = map[string]any{}
	}
	merged.Extensions["scope"] = string(TeamScopeMatch)
	merged.Extensions["matchId"] = match.ID
	merged.Extensions["competitionId"] = match.CompetitionID
	merged.Extensions["eventId"] = match.EventID
	merged.Extensions["leagueId"] = match.LeagueID
	return merged
}

func teamLookupMatchID(match *Match) string {
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match.ID)
}
