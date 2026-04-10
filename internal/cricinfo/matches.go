package cricinfo

import (
	"context"
	"fmt"
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
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}

	matches := make([]Match, 0, limit)
	warnings := make([]string, 0)
	for _, eventRef := range page.Items {
		if len(matches) >= limit {
			break
		}

		eventMatches, eventWarnings, eventErr := s.matchesFromEventRef(ctx, eventRef.URL, statusCache, teamCache, scoreCache)
		if eventErr != nil {
			warnings = append(warnings, fmt.Sprintf("event %s: %v", strings.TrimSpace(eventRef.URL), eventErr))
			continue
		}
		warnings = append(warnings, eventWarnings...)

		for _, match := range eventMatches {
			if liveOnly && !isLiveMatch(match) {
				continue
			}
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
	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return NewTransportErrorResult(EntityDeliveryEvent, ref, err), nil
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("decode detail page %q: %w", resolved.CanonicalRef, err)
	}

	events := make([]any, 0, len(page.Items))
	warnings := make([]string, 0, len(baseWarnings))
	warnings = append(warnings, baseWarnings...)

	for _, item := range page.Items {
		itemRef := strings.TrimSpace(item.URL)
		if itemRef == "" {
			warnings = append(warnings, "skip detail item with empty ref")
			continue
		}

		itemResolved, itemErr := s.client.ResolveRefChain(ctx, itemRef)
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

func (s *MatchService) matchesFromEventRef(
	ctx context.Context,
	ref string,
	statusCache map[string]matchStatusSnapshot,
	teamCache map[string]teamIdentity,
	scoreCache map[string]string,
) ([]Match, []string, error) {
	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return nil, nil, err
	}

	matches, err := NormalizeMatchesFromEvent(resolved.Body)
	if err != nil {
		return nil, nil, err
	}

	warnings := make([]string, 0)
	for i := range matches {
		warnings = append(warnings, s.hydrateMatch(ctx, &matches[i], statusCache, teamCache, scoreCache)...)
	}
	return matches, compactWarnings(warnings), nil
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
