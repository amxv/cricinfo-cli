package cricinfo

import (
	"context"
	"fmt"
	"strings"
)

// CompetitionServiceConfig configures competition metadata command behavior.
type CompetitionServiceConfig struct {
	Client   *Client
	Resolver *Resolver
}

// CompetitionLookupOptions controls resolver-backed competition lookup behavior.
type CompetitionLookupOptions struct {
	LeagueID string
}

// CompetitionService implements competition metadata command surfaces.
type CompetitionService struct {
	client       *Client
	resolver     *Resolver
	ownsResolver bool
}

// NewCompetitionService builds a competition service using default client/resolver when omitted.
func NewCompetitionService(cfg CompetitionServiceConfig) (*CompetitionService, error) {
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

	return &CompetitionService{
		client:       client,
		resolver:     resolver,
		ownsResolver: ownsResolver,
	}, nil
}

// Close persists resolver cache when owned by this service.
func (s *CompetitionService) Close() error {
	if !s.ownsResolver || s.resolver == nil {
		return nil
	}
	return s.resolver.Close()
}

// Show resolves and returns one competition summary.
func (s *CompetitionService) Show(ctx context.Context, query string, opts CompetitionLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveCompetitionLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityCompetition
		return *passthrough, nil
	}

	result := NewDataResult(EntityCompetition, lookup.competition)
	if len(lookup.warnings) > 0 {
		result = NewPartialResult(EntityCompetition, lookup.competition, lookup.warnings...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	return result, nil
}

// Officials resolves and returns competition officials entries.
func (s *CompetitionService) Officials(ctx context.Context, query string, opts CompetitionLookupOptions) (NormalizedResult, error) {
	return s.subresourceList(ctx, query, opts, EntityCompOfficial, "officials", "officials")
}

// Broadcasts resolves and returns competition broadcast entries.
func (s *CompetitionService) Broadcasts(ctx context.Context, query string, opts CompetitionLookupOptions) (NormalizedResult, error) {
	return s.subresourceList(ctx, query, opts, EntityCompBroadcast, "broadcasts", "broadcasts")
}

// Tickets resolves and returns competition ticket entries.
func (s *CompetitionService) Tickets(ctx context.Context, query string, opts CompetitionLookupOptions) (NormalizedResult, error) {
	return s.subresourceList(ctx, query, opts, EntityCompTicket, "tickets", "tickets")
}

// Odds resolves and returns competition odds entries.
func (s *CompetitionService) Odds(ctx context.Context, query string, opts CompetitionLookupOptions) (NormalizedResult, error) {
	return s.subresourceList(ctx, query, opts, EntityCompOdds, "odds", "odds")
}

// Metadata resolves and returns an aggregated competition metadata view.
func (s *CompetitionService) Metadata(ctx context.Context, query string, opts CompetitionLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolveCompetitionLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = EntityCompMetadata
		return *passthrough, nil
	}

	warnings := append([]string{}, lookup.warnings...)
	summary := CompetitionMetadataSummary{
		Competition: lookup.competition,
	}

	subresources := []struct {
		key    string
		suffix string
		assign func([]CompetitionMetadataEntry)
	}{
		{key: "officials", suffix: "officials", assign: func(entries []CompetitionMetadataEntry) { summary.Officials = entries }},
		{key: "broadcasts", suffix: "broadcasts", assign: func(entries []CompetitionMetadataEntry) { summary.Broadcasts = entries }},
		{key: "tickets", suffix: "tickets", assign: func(entries []CompetitionMetadataEntry) { summary.Tickets = entries }},
		{key: "odds", suffix: "odds", assign: func(entries []CompetitionMetadataEntry) { summary.Odds = entries }},
	}

	for _, subresource := range subresources {
		ref := competitionSubresourceRef(lookup.competition, lookup.match, subresource.key, subresource.suffix)
		if ref == "" {
			warnings = append(warnings, fmt.Sprintf("%s route unavailable for match %q", subresource.key, lookup.match.ID))
			continue
		}

		resolved, err := s.client.ResolveRefChain(ctx, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s %s: %v", subresource.key, ref, err))
			continue
		}

		payload, err := decodePayloadMap(resolved.Body)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s %s: %v", subresource.key, resolved.CanonicalRef, err))
			continue
		}

		entries, _ := normalizeCompetitionMetadataPayload(payload)
		subresource.assign(entries)
	}

	result := NewDataResult(EntityCompMetadata, summary)
	if len(compactWarnings(warnings)) > 0 {
		result = NewPartialResult(EntityCompMetadata, summary, warnings...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	return result, nil
}

type competitionLookup struct {
	competition Competition
	match       Match
	resolved    *ResolvedDocument
	warnings    []string
}

func (s *CompetitionService) resolveCompetitionLookup(
	ctx context.Context,
	query string,
	opts CompetitionLookupOptions,
) (*competitionLookup, *NormalizedResult) {
	helper := &MatchService{client: s.client, resolver: s.resolver}
	lookup, passthrough := helper.resolveMatchLookup(ctx, query, MatchLookupOptions{LeagueID: strings.TrimSpace(opts.LeagueID)})
	if passthrough != nil {
		passthrough.Kind = EntityCompetition
		return nil, passthrough
	}
	helper.enrichMatchTeamsFromIndex(lookup.match)

	competition, err := NormalizeCompetition(lookup.resolved.Body, *lookup.match)
	if err != nil {
		result := NormalizedResult{
			Kind:    EntityCompetition,
			Status:  ResultStatusError,
			Message: fmt.Sprintf("normalize competition %q: %v", lookup.resolved.CanonicalRef, err),
		}
		return nil, &result
	}

	return &competitionLookup{
		competition: *competition,
		match:       *lookup.match,
		resolved:    lookup.resolved,
		warnings:    lookup.warnings,
	}, nil
}

func (s *CompetitionService) subresourceList(
	ctx context.Context,
	query string,
	opts CompetitionLookupOptions,
	kind EntityKind,
	key string,
	suffix string,
) (NormalizedResult, error) {
	lookup, passthrough := s.resolveCompetitionLookup(ctx, query, opts)
	if passthrough != nil {
		passthrough.Kind = kind
		return *passthrough, nil
	}

	ref := competitionSubresourceRef(lookup.competition, lookup.match, key, suffix)
	if ref == "" {
		return NormalizedResult{
			Kind:    kind,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("%s route unavailable for match %q", key, lookup.match.ID),
		}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return NewTransportErrorResult(kind, ref, err), nil
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("decode %s payload %q: %w", key, resolved.CanonicalRef, err)
	}

	entries, isCollection := normalizeCompetitionMetadataPayload(payload)
	items := make([]any, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry)
	}

	result := NewListResult(kind, items)
	warnings := compactWarnings(lookup.warnings)
	if isCollection && len(items) == 0 {
		// Empty collection envelopes are valid for metadata routes and should render
		// as a clean zero-result state rather than inheriting lookup warnings.
		result.Status = ResultStatusEmpty
		result.Warnings = nil
		result.RequestedRef = resolved.RequestedRef
		result.CanonicalRef = resolved.CanonicalRef
		return result, nil
	}
	if len(warnings) > 0 {
		result = NewPartialListResult(kind, items, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// NormalizeCompetition maps a competition payload into the normalized competition shape.
func NormalizeCompetition(data []byte, match Match) (*Competition, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	competition := &Competition{
		Ref:              nonEmpty(stringField(payload, "$ref"), match.Ref, matchSubresourceRef(match, "", "")),
		ID:               nonEmpty(stringField(payload, "id"), match.ID, match.CompetitionID),
		LeagueID:         nonEmpty(match.LeagueID, refIDs(stringField(payload, "$ref"))["leagueId"]),
		EventID:          nonEmpty(match.EventID, refIDs(stringField(payload, "$ref"))["eventId"]),
		CompetitionID:    nonEmpty(match.CompetitionID, stringField(payload, "id"), match.ID),
		Description:      nonEmpty(stringField(payload, "description"), match.Description),
		ShortDescription: nonEmpty(stringField(payload, "shortDescription"), match.ShortDescription),
		Date:             nonEmpty(stringField(payload, "date"), match.Date),
		EndDate:          nonEmpty(stringField(payload, "endDate"), match.EndDate),
		MatchState:       nonEmpty(stringField(payload, "state"), stringField(payload, "summary"), match.MatchState),
		VenueName:        nonEmpty(stringField(mapField(payload, "venue"), "fullName"), match.VenueName),
		VenueSummary:     nonEmpty(venueAddressSummary(mapField(payload, "venue")), match.VenueSummary),
		ScoreSummary:     nonEmpty(match.ScoreSummary, matchScoreSummary(match.Teams)),
		StatusRef:        nonEmpty(refFromField(payload, "status"), match.StatusRef, matchSubresourceRef(match, "status", "status")),
		DetailsRef:       nonEmpty(refFromField(payload, "details"), match.DetailsRef, matchSubresourceRef(match, "details", "details")),
		MatchcardsRef:    nonEmpty(refFromField(payload, "matchcards"), matchSubresourceRef(match, "matchcards", "matchcards")),
		SituationRef:     nonEmpty(refFromField(payload, "situation"), matchSubresourceRef(match, "situation", "situation")),
		OfficialsRef:     nonEmpty(refFromField(payload, "officials"), matchSubresourceRef(match, "officials", "officials")),
		BroadcastsRef:    nonEmpty(refFromField(payload, "broadcasts"), matchSubresourceRef(match, "broadcasts", "broadcasts")),
		TicketsRef:       nonEmpty(refFromField(payload, "tickets"), matchSubresourceRef(match, "tickets", "tickets")),
		OddsRef:          nonEmpty(refFromField(payload, "odds"), matchSubresourceRef(match, "odds", "odds")),
		Teams:            match.Teams,
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "description", "shortDescription", "date", "endDate", "state", "summary",
			"venue", "status", "details", "matchcards", "situation", "officials", "broadcasts", "tickets", "odds", "competitors",
		),
	}

	return competition, nil
}

func competitionSubresourceRef(competition Competition, match Match, extensionKey, suffix string) string {
	switch strings.TrimSpace(extensionKey) {
	case "officials":
		if strings.TrimSpace(competition.OfficialsRef) != "" {
			return strings.TrimSpace(competition.OfficialsRef)
		}
	case "broadcasts":
		if strings.TrimSpace(competition.BroadcastsRef) != "" {
			return strings.TrimSpace(competition.BroadcastsRef)
		}
	case "tickets":
		if strings.TrimSpace(competition.TicketsRef) != "" {
			return strings.TrimSpace(competition.TicketsRef)
		}
	case "odds":
		if strings.TrimSpace(competition.OddsRef) != "" {
			return strings.TrimSpace(competition.OddsRef)
		}
	case "status":
		if strings.TrimSpace(competition.StatusRef) != "" {
			return strings.TrimSpace(competition.StatusRef)
		}
	case "details":
		if strings.TrimSpace(competition.DetailsRef) != "" {
			return strings.TrimSpace(competition.DetailsRef)
		}
	}

	return matchSubresourceRef(match, extensionKey, suffix)
}

func normalizeCompetitionMetadataPayload(payload map[string]any) ([]CompetitionMetadataEntry, bool) {
	if payload == nil {
		return nil, false
	}

	_, hasItems := payload["items"]
	if hasItems {
		rows := mapSliceField(payload, "items")
		entries := make([]CompetitionMetadataEntry, 0, len(rows))
		for _, row := range rows {
			entry := normalizeCompetitionMetadataEntry(row)
			if isEmptyCompetitionMetadataEntry(entry) {
				continue
			}
			entries = append(entries, entry)
		}
		return entries, true
	}

	entry := normalizeCompetitionMetadataEntry(payload)
	if isEmptyCompetitionMetadataEntry(entry) {
		return []CompetitionMetadataEntry{}, false
	}
	return []CompetitionMetadataEntry{entry}, false
}

func normalizeCompetitionMetadataEntry(payload map[string]any) CompetitionMetadataEntry {
	if payload == nil {
		return CompetitionMetadataEntry{}
	}

	position := mapField(payload, "position")
	linksMap := mapField(payload, "links")
	entry := CompetitionMetadataEntry{
		Ref:         stringField(payload, "$ref"),
		ID:          nonEmpty(stringField(payload, "id"), refIDs(stringField(payload, "$ref"))["detailId"]),
		DisplayName: nonEmpty(stringField(payload, "displayName"), stringField(payload, "shortDisplayName")),
		Name:        nonEmpty(stringField(payload, "name"), stringField(payload, "description")),
		Role:        nonEmpty(stringField(position, "displayName"), stringField(position, "name"), stringField(payload, "position")),
		Type:        nonEmpty(stringField(payload, "type"), stringField(position, "name")),
		Order:       intField(payload, "order"),
		Text:        nonEmpty(stringField(payload, "text"), stringField(payload, "shortText"), stringField(payload, "summary")),
		Value:       nonEmpty(stringField(payload, "displayValue"), stringField(payload, "value")),
		Href:        nonEmpty(stringField(payload, "href"), stringField(linksMap, "href"), firstHrefFromLinks(payload)),
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "displayName", "shortDisplayName", "name", "description", "position", "type",
			"order", "text", "shortText", "summary", "displayValue", "value", "href", "links",
		),
	}

	if entry.Name == "" {
		entry.Name = entry.DisplayName
	}
	if entry.DisplayName == "" {
		entry.DisplayName = entry.Name
	}

	return entry
}

func firstHrefFromLinks(payload map[string]any) string {
	for _, item := range mapSliceField(payload, "links") {
		href := stringField(item, "href")
		if href != "" {
			return href
		}
	}
	return ""
}

func isEmptyCompetitionMetadataEntry(entry CompetitionMetadataEntry) bool {
	return strings.TrimSpace(entry.Ref) == "" &&
		strings.TrimSpace(entry.ID) == "" &&
		strings.TrimSpace(entry.DisplayName) == "" &&
		strings.TrimSpace(entry.Name) == "" &&
		strings.TrimSpace(entry.Role) == "" &&
		strings.TrimSpace(entry.Type) == "" &&
		entry.Order == 0 &&
		strings.TrimSpace(entry.Text) == "" &&
		strings.TrimSpace(entry.Value) == "" &&
		strings.TrimSpace(entry.Href) == "" &&
		len(entry.Extensions) == 0
}
