package cricinfo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultResolverEventSeedTTL  = 5 * time.Minute
	defaultResolverMaxEventSeeds = 24
)

// ResolverConfig controls entity resolution behavior.
type ResolverConfig struct {
	Client       *Client
	Index        *EntityIndex
	IndexPath    string
	EventSeedTTL time.Duration
	MaxEventSeed int
	Now          func() time.Time
}

// ResolveOptions controls a search invocation.
type ResolveOptions struct {
	Limit     int
	LeagueID  string
	MatchID   string
	LeagueRef string
	SeasonRef string
}

// SearchResult is a resolver search output with warnings.
type SearchResult struct {
	Entities []IndexedEntity
	Warnings []string
}

// Resolver resolves players/teams/leagues/matches from ids, refs, and aliases.
type Resolver struct {
	client *Client
	index  *EntityIndex

	now          func() time.Time
	eventSeedTTL time.Duration
	maxEventSeed int
}

// NewResolver creates a resolver with default transport and cache if omitted.
func NewResolver(cfg ResolverConfig) (*Resolver, error) {
	client := cfg.Client
	if client == nil {
		newClient, err := NewClient(Config{})
		if err != nil {
			return nil, err
		}
		client = newClient
	}

	index := cfg.Index
	if index == nil {
		opened, err := OpenEntityIndex(cfg.IndexPath)
		if err != nil {
			return nil, err
		}
		index = opened
	}

	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	eventSeedTTL := cfg.EventSeedTTL
	if eventSeedTTL <= 0 {
		eventSeedTTL = defaultResolverEventSeedTTL
	}

	maxEventSeed := cfg.MaxEventSeed
	if maxEventSeed <= 0 {
		maxEventSeed = defaultResolverMaxEventSeeds
	}

	return &Resolver{
		client:       client,
		index:        index,
		now:          now,
		eventSeedTTL: eventSeedTTL,
		maxEventSeed: maxEventSeed,
	}, nil
}

// Close persists the resolver cache to disk.
func (r *Resolver) Close() error {
	return r.index.Persist()
}

// Search resolves entities for a search command.
func (r *Resolver) Search(ctx context.Context, kind EntityKind, query string, opts ResolveOptions) (SearchResult, error) {
	query = strings.TrimSpace(query)
	warnings := make([]string, 0)

	if err := r.seedContext(ctx, opts); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := r.seedFromEvents(ctx); err != nil {
		warnings = append(warnings, err.Error())
	}

	if query != "" {
		if isKnownRefQuery(query) {
			if err := r.seedKnownRef(ctx, kind, query, opts); err != nil {
				warnings = append(warnings, err.Error())
			}
		}
		if isNumeric(query) {
			if err := r.seedNumericID(ctx, kind, query, opts); err != nil {
				warnings = append(warnings, err.Error())
			}
		}
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	entities := r.index.Search(kind, query, limit, SearchContext{
		PreferredLeagueID: strings.TrimSpace(opts.LeagueID),
		PreferredMatchID:  strings.TrimSpace(opts.MatchID),
	})

	if err := r.index.Persist(); err != nil {
		warnings = append(warnings, fmt.Sprintf("persist cache: %v", err))
	}

	return SearchResult{Entities: entities, Warnings: compactWarnings(warnings)}, nil
}

func (r *Resolver) seedContext(ctx context.Context, opts ResolveOptions) error {
	errs := make([]string, 0)

	if leagueID := strings.TrimSpace(opts.LeagueID); leagueID != "" {
		if err := r.seedLeagueByID(ctx, leagueID); err != nil {
			errs = append(errs, fmt.Sprintf("league context seed failed for %s: %v", leagueID, err))
		}
	}
	if leagueRef := strings.TrimSpace(opts.LeagueRef); leagueRef != "" {
		if err := r.seedLeagueRef(ctx, leagueRef); err != nil {
			errs = append(errs, fmt.Sprintf("league ref seed failed for %s: %v", leagueRef, err))
		}
	}
	if seasonRef := strings.TrimSpace(opts.SeasonRef); seasonRef != "" {
		ids := refIDs(seasonRef)
		if leagueID := strings.TrimSpace(ids["leagueId"]); leagueID != "" {
			if err := r.seedLeagueByID(ctx, leagueID); err != nil {
				errs = append(errs, fmt.Sprintf("season league seed failed for %s: %v", leagueID, err))
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

func (r *Resolver) seedFromEvents(ctx context.Context) error {
	last := r.index.LastEventsSeedAt()
	if !last.IsZero() && r.now().Sub(last) < r.eventSeedTTL {
		return nil
	}

	resolved, err := r.client.ResolveRefChain(ctx, "/events")
	if err != nil {
		return fmt.Errorf("events seed failed: %w", err)
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return fmt.Errorf("decode /events page: %w", err)
	}

	limit := r.maxEventSeed
	if limit > len(page.Items) {
		limit = len(page.Items)
	}
	successCount := 0
	seedErrors := make([]string, 0)
	for i := 0; i < limit; i++ {
		if err := r.seedEventRef(ctx, page.Items[i].URL); err != nil {
			seedErrors = append(seedErrors, fmt.Sprintf("%s (%v)", page.Items[i].URL, err))
			continue
		}
		successCount++
	}
	if successCount == 0 && len(seedErrors) > 0 {
		return fmt.Errorf("seed events failed: %s", seedErrors[0])
	}

	r.index.SetLastEventsSeedAt(r.now())
	r.markHydrated(resolved.RequestedRef)
	r.markHydrated(resolved.CanonicalRef)
	return nil
}

func (r *Resolver) seedKnownRef(ctx context.Context, kind EntityKind, ref string, opts ResolveOptions) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}

	switch kind {
	case EntityPlayer:
		return r.seedPlayerRef(ctx, ref, opts.LeagueID, opts.MatchID)
	case EntityTeam:
		ids := refIDs(ref)
		teamID := nonEmpty(ids["teamId"], ids["competitorId"])
		if teamID != "" {
			return r.seedTeamByID(ctx, teamID, opts.LeagueID, opts.MatchID)
		}
		return r.seedTeamRef(ctx, ref, opts.LeagueID, opts.MatchID)
	case EntityLeague:
		return r.seedLeagueRef(ctx, ref)
	case EntityMatch:
		if strings.Contains(ref, "/competitions/") {
			return r.seedCompetitionRef(ctx, ref)
		}
		if strings.Contains(ref, "/events/") {
			return r.seedEventRef(ctx, ref)
		}
		return fmt.Errorf("unsupported match ref %q", ref)
	default:
		return nil
	}
}

func (r *Resolver) seedNumericID(ctx context.Context, kind EntityKind, id string, opts ResolveOptions) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}

	switch kind {
	case EntityPlayer:
		return r.seedPlayerByID(ctx, id, opts.LeagueID, opts.MatchID)
	case EntityTeam:
		return r.seedTeamByID(ctx, id, opts.LeagueID, opts.MatchID)
	case EntityLeague:
		return r.seedLeagueByID(ctx, id)
	case EntityMatch:
		// Match IDs are usually competition IDs and often match event IDs.
		if err := r.seedEventRef(ctx, "/events/"+id); err == nil {
			return nil
		}
		if leagueID := strings.TrimSpace(opts.LeagueID); leagueID != "" {
			competitionRef := fmt.Sprintf("/leagues/%s/events/%s/competitions/%s", leagueID, id, id)
			if err := r.seedCompetitionRef(ctx, competitionRef); err == nil {
				return nil
			}
		}
		return fmt.Errorf("unable to resolve match id %s without known league/event context", id)
	default:
		return nil
	}
}

func (r *Resolver) seedEventRef(ctx context.Context, ref string) error {
	ref = r.absoluteRef(ref)
	if ref == "" {
		return fmt.Errorf("empty event ref")
	}
	if r.isHydrated(ref) {
		return nil
	}

	resolved, err := r.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return err
	}

	ids := refIDs(nonEmpty(resolved.CanonicalRef, resolved.RequestedRef, ref))
	eventID := nonEmpty(stringField(payload, "id"), ids["eventId"])
	leagueID := strings.TrimSpace(ids["leagueId"])

	leagueRefs := mapSliceField(payload, "leagues")
	for _, leagueRaw := range leagueRefs {
		leagueRef := refFromField(leagueRaw, "$ref")
		if leagueRef == "" {
			leagueRef = stringField(leagueRaw, "$ref")
		}
		leagueID = nonEmpty(leagueID, stringField(leagueRaw, "id"), refIDs(leagueRef)["leagueId"])

		leagueName := nonEmpty(stringField(leagueRaw, "name"), stringField(leagueRaw, "shortName"))
		leagueShort := nonEmpty(stringField(leagueRaw, "abbreviation"), stringField(leagueRaw, "slug"))
		if leagueID != "" {
			_ = r.index.Upsert(IndexedEntity{
				Kind:      EntityLeague,
				ID:        leagueID,
				Ref:       leagueRef,
				Name:      leagueName,
				ShortName: leagueShort,
				UpdatedAt: r.now(),
			})
		}
		if leagueRef != "" {
			_ = r.seedLeagueRef(ctx, leagueRef)
		}
	}

	eventName := nonEmpty(stringField(payload, "shortDescription"), stringField(payload, "description"), stringField(payload, "name"))

	competitions := mapSliceField(payload, "competitions")
	for _, comp := range competitions {
		if err := r.seedCompetitionMap(ctx, comp, leagueID, eventID, eventName); err != nil {
			return err
		}
	}

	r.markHydrated(ref)
	r.markHydrated(resolved.RequestedRef)
	r.markHydrated(resolved.CanonicalRef)
	return nil
}

func (r *Resolver) seedCompetitionRef(ctx context.Context, ref string) error {
	ref = r.absoluteRef(ref)
	if ref == "" {
		return fmt.Errorf("empty competition ref")
	}
	if r.isHydrated(ref) {
		return nil
	}

	resolved, err := r.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return err
	}

	ids := refIDs(nonEmpty(resolved.CanonicalRef, resolved.RequestedRef, ref))
	if err := r.seedCompetitionMap(ctx, payload, ids["leagueId"], ids["eventId"], ""); err != nil {
		return err
	}

	r.markHydrated(ref)
	r.markHydrated(resolved.RequestedRef)
	r.markHydrated(resolved.CanonicalRef)
	return nil
}

func (r *Resolver) seedCompetitionMap(ctx context.Context, comp map[string]any, leagueID, eventID, eventName string) error {
	compRef := stringField(comp, "$ref")
	if compRef == "" {
		compRef = r.buildCompetitionRef(leagueID, eventID, stringField(comp, "id"))
	}
	compIDs := refIDs(compRef)
	competitionID := nonEmpty(stringField(comp, "id"), compIDs["competitionId"])
	leagueID = nonEmpty(strings.TrimSpace(leagueID), compIDs["leagueId"])
	eventID = nonEmpty(strings.TrimSpace(eventID), compIDs["eventId"])

	if competitionID == "" {
		return nil
	}

	matchName := nonEmpty(stringField(comp, "shortDescription"), stringField(comp, "description"), stringField(comp, "note"), eventName, stringField(comp, "date"))
	matchShort := nonEmpty(stringField(comp, "shortDescription"), eventName)

	if err := r.index.Upsert(IndexedEntity{
		Kind:      EntityMatch,
		ID:        competitionID,
		Ref:       compRef,
		Name:      matchName,
		ShortName: matchShort,
		LeagueID:  leagueID,
		EventID:   eventID,
		MatchID:   competitionID,
		Aliases: []string{
			stringField(comp, "description"),
			stringField(comp, "shortDescription"),
			stringField(comp, "note"),
			eventName,
			competitionID,
			eventID,
		},
		UpdatedAt: r.now(),
	}); err != nil {
		return err
	}

	competitors := mapSliceField(comp, "competitors")
	for _, competitor := range competitors {
		if err := r.seedCompetitorMap(ctx, competitor, leagueID, eventID, competitionID); err != nil {
			return err
		}
	}

	if compRef != "" {
		r.markHydrated(r.absoluteRef(compRef))
	}
	return nil
}

func (r *Resolver) seedCompetitorMap(ctx context.Context, competitor map[string]any, leagueID, eventID, matchID string) error {
	competitorRef := stringField(competitor, "$ref")
	teamRef := refFromField(competitor, "team")
	teamIDs := refIDs(teamRef)
	competitorIDs := refIDs(competitorRef)

	teamID := nonEmpty(teamIDs["teamId"], stringField(mapField(competitor, "team"), "id"), stringField(competitor, "id"), competitorIDs["competitorId"])
	teamName := nonEmpty(stringField(mapField(competitor, "team"), "displayName"), stringField(mapField(competitor, "team"), "name"), stringField(competitor, "displayName"), stringField(competitor, "name"))
	teamShort := nonEmpty(stringField(mapField(competitor, "team"), "shortDisplayName"), stringField(mapField(competitor, "team"), "abbreviation"), stringField(competitor, "abbreviation"))

	if teamID != "" {
		if err := r.index.Upsert(IndexedEntity{
			Kind:      EntityTeam,
			ID:        teamID,
			Ref:       nonEmpty(teamRef, competitorRef),
			Name:      teamName,
			ShortName: teamShort,
			LeagueID:  leagueID,
			EventID:   eventID,
			MatchID:   matchID,
			Aliases: []string{
				teamName,
				teamShort,
				stringField(competitor, "homeAway"),
				teamID,
			},
			UpdatedAt: r.now(),
		}); err != nil {
			return err
		}

		if teamName == "" {
			_ = r.seedTeamByID(ctx, teamID, leagueID, matchID)
		}
	}

	rosterRef := refFromField(competitor, "roster")
	if rosterRef != "" {
		if err := r.seedRosterRef(ctx, rosterRef, leagueID, eventID, matchID); err != nil {
			return err
		}
	}

	return nil
}

func (r *Resolver) seedRosterRef(ctx context.Context, ref, leagueID, eventID, matchID string) error {
	ref = r.absoluteRef(ref)
	if ref == "" {
		return nil
	}
	if r.isHydrated(ref) {
		return nil
	}

	resolved, err := r.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return err
	}

	entries := mapSliceField(payload, "entries")
	for _, entry := range entries {
		athleteRef := refFromField(entry, "athlete")
		playerID := nonEmpty(stringField(entry, "playerId"), refIDs(athleteRef)["athleteId"], refIDs(stringField(entry, "$ref"))["athleteId"])
		if playerID == "" {
			continue
		}

		playerName := nonEmpty(
			stringField(mapField(entry, "athlete"), "displayName"),
			stringField(mapField(entry, "athlete"), "fullName"),
		)

		if err := r.index.Upsert(IndexedEntity{
			Kind:      EntityPlayer,
			ID:        playerID,
			Ref:       athleteRef,
			Name:      playerName,
			LeagueID:  leagueID,
			EventID:   eventID,
			MatchID:   matchID,
			Aliases:   []string{playerName, playerID},
			UpdatedAt: r.now(),
		}); err != nil {
			return err
		}

		if playerName == "" {
			if err := r.seedPlayerByID(ctx, playerID, leagueID, matchID); err != nil {
				return err
			}
		}
	}

	r.markHydrated(ref)
	r.markHydrated(resolved.RequestedRef)
	r.markHydrated(resolved.CanonicalRef)
	return nil
}

func (r *Resolver) seedPlayerRef(ctx context.Context, ref, leagueID, matchID string) error {
	ids := refIDs(ref)
	if playerID := ids["athleteId"]; playerID != "" {
		return r.seedPlayerByID(ctx, playerID, leagueID, matchID)
	}

	resolved, err := r.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return err
	}

	playerID := nonEmpty(stringField(payload, "id"), refIDs(resolved.CanonicalRef)["athleteId"])
	if playerID == "" {
		return nil
	}

	return r.index.Upsert(IndexedEntity{
		Kind:      EntityPlayer,
		ID:        playerID,
		Ref:       resolved.CanonicalRef,
		Name:      nonEmpty(stringField(payload, "displayName"), stringField(payload, "fullName"), stringField(payload, "name")),
		ShortName: stringField(payload, "shortName"),
		LeagueID:  strings.TrimSpace(leagueID),
		MatchID:   strings.TrimSpace(matchID),
		Aliases: []string{
			stringField(payload, "displayName"),
			stringField(payload, "fullName"),
			stringField(payload, "name"),
			stringField(payload, "battingName"),
			stringField(payload, "fieldingName"),
		},
		UpdatedAt: r.now(),
	})
}

func (r *Resolver) seedPlayerByID(ctx context.Context, id, leagueID, matchID string) error {
	if _, ok := r.index.FindByID(EntityPlayer, id); ok {
		return nil
	}

	resolved, err := r.client.ResolveRefChain(ctx, "/athletes/"+id)
	if err != nil {
		return err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return err
	}

	if err := r.index.Upsert(IndexedEntity{
		Kind:      EntityPlayer,
		ID:        nonEmpty(stringField(payload, "id"), id),
		Ref:       resolved.CanonicalRef,
		Name:      nonEmpty(stringField(payload, "displayName"), stringField(payload, "fullName"), stringField(payload, "name")),
		ShortName: stringField(payload, "shortName"),
		LeagueID:  strings.TrimSpace(leagueID),
		MatchID:   strings.TrimSpace(matchID),
		Aliases: []string{
			stringField(payload, "displayName"),
			stringField(payload, "fullName"),
			stringField(payload, "name"),
			stringField(payload, "battingName"),
			stringField(payload, "fieldingName"),
			id,
		},
		UpdatedAt: r.now(),
	}); err != nil {
		return err
	}

	r.markHydrated(resolved.RequestedRef)
	r.markHydrated(resolved.CanonicalRef)
	return nil
}

func (r *Resolver) seedTeamRef(ctx context.Context, ref, leagueID, matchID string) error {
	ids := refIDs(ref)
	teamID := nonEmpty(ids["teamId"], ids["competitorId"])
	if teamID != "" {
		return r.seedTeamByID(ctx, teamID, leagueID, matchID)
	}

	resolved, err := r.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return err
	}

	teamID = nonEmpty(stringField(payload, "id"), refIDs(resolved.CanonicalRef)["teamId"], refIDs(resolved.CanonicalRef)["competitorId"])
	if teamID == "" {
		return nil
	}

	if err := r.index.Upsert(IndexedEntity{
		Kind:      EntityTeam,
		ID:        teamID,
		Ref:       resolved.CanonicalRef,
		Name:      nonEmpty(stringField(payload, "displayName"), stringField(payload, "name")),
		ShortName: nonEmpty(stringField(payload, "shortDisplayName"), stringField(payload, "shortName"), stringField(payload, "abbreviation")),
		LeagueID:  strings.TrimSpace(leagueID),
		MatchID:   strings.TrimSpace(matchID),
		Aliases: []string{
			stringField(payload, "displayName"),
			stringField(payload, "name"),
			stringField(payload, "shortDisplayName"),
			stringField(payload, "shortName"),
			stringField(payload, "abbreviation"),
		},
		UpdatedAt: r.now(),
	}); err != nil {
		return err
	}

	r.markHydrated(resolved.RequestedRef)
	r.markHydrated(resolved.CanonicalRef)
	return nil
}

func (r *Resolver) seedTeamByID(ctx context.Context, id, leagueID, matchID string) error {
	if existing, ok := r.index.FindByID(EntityTeam, id); ok && strings.TrimSpace(existing.Name) != "" {
		return nil
	}

	resolved, err := r.client.ResolveRefChain(ctx, "/teams/"+id)
	if err != nil {
		return err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return err
	}

	if err := r.index.Upsert(IndexedEntity{
		Kind:      EntityTeam,
		ID:        nonEmpty(stringField(payload, "id"), id),
		Ref:       resolved.CanonicalRef,
		Name:      nonEmpty(stringField(payload, "displayName"), stringField(payload, "name"), stringField(payload, "shortDisplayName")),
		ShortName: nonEmpty(stringField(payload, "shortDisplayName"), stringField(payload, "shortName"), stringField(payload, "abbreviation"), stringField(payload, "slug")),
		LeagueID:  strings.TrimSpace(leagueID),
		MatchID:   strings.TrimSpace(matchID),
		Aliases: []string{
			stringField(payload, "displayName"),
			stringField(payload, "name"),
			stringField(payload, "shortDisplayName"),
			stringField(payload, "shortName"),
			stringField(payload, "abbreviation"),
			stringField(payload, "slug"),
			id,
		},
		UpdatedAt: r.now(),
	}); err != nil {
		return err
	}

	r.markHydrated(resolved.RequestedRef)
	r.markHydrated(resolved.CanonicalRef)
	return nil
}

func (r *Resolver) seedLeagueRef(ctx context.Context, ref string) error {
	ids := refIDs(ref)
	if leagueID := ids["leagueId"]; leagueID != "" {
		return r.seedLeagueByID(ctx, leagueID)
	}

	resolved, err := r.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return err
	}

	leagueID := nonEmpty(stringField(payload, "id"), refIDs(resolved.CanonicalRef)["leagueId"])
	if leagueID == "" {
		return nil
	}

	return r.index.Upsert(IndexedEntity{
		Kind:      EntityLeague,
		ID:        leagueID,
		Ref:       resolved.CanonicalRef,
		Name:      nonEmpty(stringField(payload, "name"), stringField(payload, "shortName")),
		ShortName: nonEmpty(stringField(payload, "slug"), stringField(payload, "abbreviation"), stringField(payload, "shortName")),
		Aliases: []string{
			stringField(payload, "name"),
			stringField(payload, "shortName"),
			stringField(payload, "slug"),
			stringField(payload, "abbreviation"),
			leagueID,
		},
		UpdatedAt: r.now(),
	})
}

func (r *Resolver) seedLeagueByID(ctx context.Context, id string) error {
	if existing, ok := r.index.FindByID(EntityLeague, id); ok && strings.TrimSpace(existing.Name) != "" {
		return nil
	}

	resolved, err := r.client.ResolveRefChain(ctx, "/leagues/"+id)
	if err != nil {
		return err
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return err
	}

	if err := r.index.Upsert(IndexedEntity{
		Kind:      EntityLeague,
		ID:        nonEmpty(stringField(payload, "id"), id),
		Ref:       resolved.CanonicalRef,
		Name:      nonEmpty(stringField(payload, "name"), stringField(payload, "shortName")),
		ShortName: nonEmpty(stringField(payload, "slug"), stringField(payload, "abbreviation"), stringField(payload, "shortName")),
		Aliases: []string{
			stringField(payload, "name"),
			stringField(payload, "shortName"),
			stringField(payload, "slug"),
			stringField(payload, "abbreviation"),
			id,
		},
		UpdatedAt: r.now(),
	}); err != nil {
		return err
	}

	r.markHydrated(resolved.RequestedRef)
	r.markHydrated(resolved.CanonicalRef)
	return nil
}

func (r *Resolver) absoluteRef(ref string) string {
	resolved, err := r.client.resolveRef(ref)
	if err != nil {
		return strings.TrimSpace(ref)
	}
	return resolved
}

func (r *Resolver) markHydrated(ref string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return
	}
	r.index.MarkHydratedRef(ref, r.now())
}

func (r *Resolver) isHydrated(ref string) bool {
	if ref == "" {
		return false
	}
	_, ok := r.index.HydratedRefAt(ref)
	return ok
}

func (r *Resolver) buildCompetitionRef(leagueID, eventID, competitionID string) string {
	leagueID = strings.TrimSpace(leagueID)
	eventID = strings.TrimSpace(eventID)
	competitionID = strings.TrimSpace(competitionID)
	if leagueID == "" || eventID == "" || competitionID == "" {
		return ""
	}
	return fmt.Sprintf("/leagues/%s/events/%s/competitions/%s", leagueID, eventID, competitionID)
}

func isNumeric(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	_, err := strconv.ParseInt(raw, 10, 64)
	return err == nil
}

func isKnownRefQuery(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "/")
}
