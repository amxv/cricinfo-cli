package cricinfo

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const defaultPlayerNewsLimit = 10

// PlayerLookupOptions controls resolver-backed player lookup behavior.
type PlayerLookupOptions struct {
	LeagueID string
	Limit    int
}

// PlayerServiceConfig configures player discovery and global player commands.
type PlayerServiceConfig struct {
	Client   *Client
	Resolver *Resolver
}

// PlayerService implements domain-level player discovery, profile, news, and statistics commands.
type PlayerService struct {
	client       *Client
	resolver     *Resolver
	ownsResolver bool
}

// NewPlayerService builds a player service using default client/resolver when omitted.
func NewPlayerService(cfg PlayerServiceConfig) (*PlayerService, error) {
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

	return &PlayerService{
		client:       client,
		resolver:     resolver,
		ownsResolver: ownsResolver,
	}, nil
}

// Close persists resolver cache when owned by this service.
func (s *PlayerService) Close() error {
	if !s.ownsResolver || s.resolver == nil {
		return nil
	}
	return s.resolver.Close()
}

// Search resolves player entities for discovery.
func (s *PlayerService) Search(ctx context.Context, query string, opts PlayerLookupOptions) (NormalizedResult, error) {
	query = strings.TrimSpace(query)
	searchResult, err := s.resolver.Search(ctx, EntityPlayer, query, ResolveOptions{
		Limit:    limitOrDefault(opts.Limit, 10),
		LeagueID: strings.TrimSpace(opts.LeagueID),
	})
	if err != nil {
		return NewTransportErrorResult(EntityPlayer, query, err), nil
	}

	items := make([]any, 0, len(searchResult.Entities))
	for _, entity := range searchResult.Entities {
		items = append(items, entity.ToRenderable())
	}

	result := NewListResult(EntityPlayer, items)
	if len(searchResult.Warnings) > 0 {
		result = NewPartialListResult(EntityPlayer, items, searchResult.Warnings...)
	}
	return result, nil
}

// Profile resolves and returns a normalized global player profile.
func (s *PlayerService) Profile(ctx context.Context, query string, opts PlayerLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolvePlayerLookup(ctx, query, opts, EntityPlayer)
	if passthrough != nil {
		return *passthrough, nil
	}

	result := NewDataResult(EntityPlayer, lookup.player)
	if len(lookup.warnings) > 0 {
		result = NewPartialResult(EntityPlayer, lookup.player, lookup.warnings...)
	}
	result.RequestedRef = lookup.resolved.RequestedRef
	result.CanonicalRef = lookup.resolved.CanonicalRef
	return result, nil
}

// News resolves and returns normalized news articles for a player.
func (s *PlayerService) News(ctx context.Context, query string, opts PlayerLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolvePlayerLookup(ctx, query, opts, EntityNewsArticle)
	if passthrough != nil {
		return *passthrough, nil
	}
	if strings.TrimSpace(lookup.player.NewsRef) == "" {
		return NormalizedResult{
			Kind:    EntityNewsArticle,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("news route unavailable for player %q", lookup.player.ID),
		}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, lookup.player.NewsRef)
	if err != nil {
		return NewTransportErrorResult(EntityNewsArticle, lookup.player.NewsRef, err), nil
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("decode player news page %q: %w", resolved.CanonicalRef, err)
	}

	limit := limitOrDefault(opts.Limit, defaultPlayerNewsLimit)
	if limit > len(page.Items) {
		limit = len(page.Items)
	}

	items := make([]any, 0, limit)
	warnings := append([]string{}, lookup.warnings...)
	for i := 0; i < limit; i++ {
		itemRef := strings.TrimSpace(page.Items[i].URL)
		if itemRef == "" {
			continue
		}

		itemResolved, itemErr := s.client.ResolveRefChain(ctx, itemRef)
		if itemErr != nil {
			warnings = append(warnings, fmt.Sprintf("news article %s: %v", itemRef, itemErr))
			continue
		}

		article, normalizeErr := NormalizeNewsArticle(itemResolved.Body)
		if normalizeErr != nil {
			warnings = append(warnings, fmt.Sprintf("news article %s: %v", itemResolved.CanonicalRef, normalizeErr))
			continue
		}
		items = append(items, *article)
	}

	result := NewListResult(EntityNewsArticle, items)
	if len(warnings) > 0 {
		result = NewPartialListResult(EntityNewsArticle, items, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	if len(items) == 0 && strings.TrimSpace(result.Message) == "" {
		result.Message = fmt.Sprintf("no news articles found for %q", lookup.player.DisplayName)
	}
	return result, nil
}

// Stats resolves and returns grouped global player statistics.
func (s *PlayerService) Stats(ctx context.Context, query string, opts PlayerLookupOptions) (NormalizedResult, error) {
	return s.statistics(ctx, query, opts)
}

// Career resolves and returns grouped career statistics.
func (s *PlayerService) Career(ctx context.Context, query string, opts PlayerLookupOptions) (NormalizedResult, error) {
	return s.statistics(ctx, query, opts)
}

// MatchStats resolves and returns player-in-match batting/bowling/fielding statistics.
func (s *PlayerService) MatchStats(ctx context.Context, playerQuery, matchQuery string, opts PlayerLookupOptions) (NormalizedResult, error) {
	contextData, passthrough := s.resolvePlayerMatchContext(ctx, playerQuery, matchQuery, opts, EntityPlayerMatch)
	if passthrough != nil {
		return *passthrough, nil
	}

	statsRef := rosterPlayerStatisticsRef(contextData.match, contextData.team, contextData.roster)
	if statsRef == "" {
		return NormalizedResult{
			Kind:    EntityPlayerMatch,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("match statistics route unavailable for player %q", contextData.playerID),
		}, nil
	}

	resolved, categories, err := s.fetchStatCategories(ctx, statsRef)
	if err != nil {
		return NewTransportErrorResult(EntityPlayerMatch, statsRef, err), nil
	}

	batting, bowling, fielding := splitPlayerStatCategories(categories)
	summary := summarizePlayerMatchCategories(categories)
	playerMatch := PlayerMatch{
		PlayerID:      contextData.playerID,
		PlayerRef:     contextData.roster.PlayerRef,
		PlayerName:    contextData.playerName,
		MatchID:       contextData.match.ID,
		CompetitionID: nonEmpty(contextData.match.CompetitionID, contextData.match.ID),
		EventID:       contextData.match.EventID,
		LeagueID:      contextData.match.LeagueID,
		TeamID:        contextData.team.ID,
		TeamName:      teamDisplayLabel(contextData.team),
		StatisticsRef: resolved.CanonicalRef,
		LinescoresRef: rosterPlayerLinescoresRef(contextData.match, contextData.team, contextData.roster),
		Batting:       batting,
		Bowling:       bowling,
		Fielding:      fielding,
		Summary:       summary,
	}

	result := NewDataResult(EntityPlayerMatch, playerMatch)
	warnings := compactWarnings(append(contextData.warnings, contextData.routeWarnings...))
	if len(warnings) > 0 {
		result = NewPartialResult(EntityPlayerMatch, playerMatch, warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

// Innings resolves and returns player linescore splits for a selected match.
func (s *PlayerService) Innings(ctx context.Context, playerQuery, matchQuery string, opts PlayerLookupOptions) (NormalizedResult, error) {
	contextData, passthrough := s.resolvePlayerMatchContext(ctx, playerQuery, matchQuery, opts, EntityPlayerInnings)
	if passthrough != nil {
		return *passthrough, nil
	}

	linescoresRef := rosterPlayerLinescoresRef(contextData.match, contextData.team, contextData.roster)
	if linescoresRef == "" {
		return NormalizedResult{
			Kind:    EntityPlayerInnings,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("player linescores route unavailable for player %q", contextData.playerID),
		}, nil
	}

	resolved, err := s.client.ResolveRefChain(ctx, linescoresRef)
	if err != nil {
		return NewTransportErrorResult(EntityPlayerInnings, linescoresRef, err), nil
	}

	payload, err := decodePayloadMap(resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("decode player linescores %q: %w", resolved.CanonicalRef, err)
	}

	rows := mapSliceField(payload, "items")
	if len(rows) == 0 && len(payload) > 0 {
		rows = append(rows, payload)
	}

	entries := make([]PlayerInnings, 0, len(rows))
	warnings := append([]string{}, contextData.warnings...)
	warnings = append(warnings, contextData.routeWarnings...)

	for _, row := range rows {
		rowRef := stringField(row, "$ref")
		rowIDs := refIDs(rowRef)
		inningsNumber := intField(row, "value")
		if inningsNumber == 0 {
			inningsNumber = parseInt(rowIDs["inningsId"])
		}
		period := intField(row, "period")
		if period == 0 {
			period = parseInt(rowIDs["periodId"])
		}

		statisticsRef := nonEmpty(
			stringField(row, "statistics"),
			firstPlayerLinescoreStatisticsRef(row),
			rosterPlayerLinescoreStatisticsRef(contextData.match, contextData.team, contextData.roster, inningsNumber, period),
		)

		inningsEntry := PlayerInnings{
			Ref:           rowRef,
			PlayerID:      contextData.playerID,
			PlayerName:    contextData.playerName,
			MatchID:       contextData.match.ID,
			CompetitionID: nonEmpty(contextData.match.CompetitionID, contextData.match.ID),
			EventID:       contextData.match.EventID,
			LeagueID:      contextData.match.LeagueID,
			TeamID:        contextData.team.ID,
			TeamName:      teamDisplayLabel(contextData.team),
			InningsNumber: inningsNumber,
			Period:        period,
			Order:         intField(row, "order"),
			IsBatting:     boolField(row, "isBatting"),
			StatisticsRef: statisticsRef,
			Extensions: extensionsFromMap(row,
				"$ref", "period", "value", "displayValue", "isBatting", "order", "mediaId", "statistics", "linescores",
			),
		}

		if statisticsRef != "" {
			_, categories, statsErr := s.fetchStatCategories(ctx, statisticsRef)
			if statsErr != nil {
				warnings = append(warnings, fmt.Sprintf("player innings statistics %s: %v", statisticsRef, statsErr))
			} else {
				batting, bowling, fielding := splitPlayerStatCategories(categories)
				inningsEntry.Batting = batting
				inningsEntry.Bowling = bowling
				inningsEntry.Fielding = fielding
				inningsEntry.Summary = summarizePlayerMatchCategories(categories)
			}
		}

		entries = append(entries, inningsEntry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].InningsNumber != entries[j].InningsNumber {
			return entries[i].InningsNumber < entries[j].InningsNumber
		}
		if entries[i].Period != entries[j].Period {
			return entries[i].Period < entries[j].Period
		}
		return entries[i].Order < entries[j].Order
	})

	items := make([]any, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry)
	}

	result := NewListResult(EntityPlayerInnings, items)
	if compact := compactWarnings(warnings); len(compact) > 0 {
		result = NewPartialListResult(EntityPlayerInnings, items, compact...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	if len(items) == 0 && strings.TrimSpace(result.Message) == "" {
		result.Message = fmt.Sprintf("no innings splits found for player %q in match %q", contextData.playerName, contextData.match.ID)
	}
	return result, nil
}

// Dismissals resolves dismissal-focused wicket views for a player in one match.
func (s *PlayerService) Dismissals(ctx context.Context, playerQuery, matchQuery string, opts PlayerLookupOptions) (NormalizedResult, error) {
	contextData, passthrough := s.resolvePlayerMatchContext(ctx, playerQuery, matchQuery, opts, EntityPlayerDismissal)
	if passthrough != nil {
		return *passthrough, nil
	}

	resolved, deliveries, deliveryWarnings, err := s.fetchPlayerDeliveries(ctx, contextData, true)
	if err != nil {
		return NewTransportErrorResult(EntityPlayerDismissal, matchSubresourceRef(contextData.match, "details", "details"), err), nil
	}

	wicketByRef, wicketByID, wicketWarnings := s.collectMatchWicketMetadata(ctx, contextData.match)
	warnings := append([]string{}, contextData.warnings...)
	warnings = append(warnings, contextData.routeWarnings...)
	warnings = append(warnings, deliveryWarnings...)
	warnings = append(warnings, wicketWarnings...)

	items := make([]any, 0, len(deliveries))
	for _, delivery := range deliveries {
		if !isDismissalDelivery(delivery) {
			continue
		}

		wicketMeta, ok := wicketByRef[strings.TrimSpace(delivery.Ref)]
		if !ok {
			detailID := strings.TrimSpace(refIDs(delivery.Ref)["detailId"])
			if detailID != "" {
				wicketMeta, ok = wicketByID[detailID]
			}
		}

		playerDismissal := PlayerDismissal{
			PlayerID:        contextData.playerID,
			PlayerName:      contextData.playerName,
			MatchID:         contextData.match.ID,
			CompetitionID:   nonEmpty(contextData.match.CompetitionID, contextData.match.ID),
			EventID:         contextData.match.EventID,
			LeagueID:        contextData.match.LeagueID,
			TeamID:          nonEmpty(wicketMeta.team.ID, contextData.team.ID),
			TeamName:        nonEmpty(teamDisplayLabel(wicketMeta.team), teamDisplayLabel(contextData.team), "Unknown Team"),
			InningsNumber:   wicketMeta.innings.InningsNumber,
			Period:          wicketMeta.innings.Period,
			WicketNumber:    wicketMeta.wicket.Number,
			FOW:             wicketMeta.wicket.FOW,
			Over:            nonEmpty(wicketMeta.wicket.Over, fmt.Sprintf("%.1f", wicketMeta.wicket.WicketOver)),
			DetailRef:       nonEmpty(wicketMeta.wicket.DetailRef, delivery.Ref),
			DetailShortText: nonEmpty(wicketMeta.wicket.DetailShortText, delivery.ShortText),
			DetailText:      nonEmpty(wicketMeta.wicket.DetailText, delivery.Text),
			DismissalName:   nonEmpty(delivery.DismissalName, delivery.DismissalType, wicketMeta.wicket.FOWType),
			DismissalCard:   nonEmpty(wicketMeta.wicket.DismissalCard, delivery.DismissalCard),
			DismissalType:   delivery.DismissalType,
			DismissalText:   delivery.DismissalText,
			BallsFaced:      firstNonZero(wicketMeta.wicket.BallsFaced, wicketMeta.wicket.RunsScored),
			StrikeRate:      wicketMeta.wicket.StrikeRate,
			BatsmanPlayerID: delivery.BatsmanPlayerID,
			BowlerPlayerID:  delivery.BowlerPlayerID,
			FielderPlayerID: delivery.FielderPlayerID,
		}
		items = append(items, playerDismissal)
	}

	result := NewListResult(EntityPlayerDismissal, items)
	if compact := compactWarnings(warnings); len(compact) > 0 {
		result = NewPartialListResult(EntityPlayerDismissal, items, compact...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	if len(items) == 0 && strings.TrimSpace(result.Message) == "" {
		result.Message = fmt.Sprintf("no dismissal events found for player %q in match %q", contextData.playerName, contextData.match.ID)
	}
	return result, nil
}

// Deliveries resolves delivery events for a player in one match, preserving coordinates and dismissal metadata.
func (s *PlayerService) Deliveries(ctx context.Context, playerQuery, matchQuery string, opts PlayerLookupOptions) (NormalizedResult, error) {
	contextData, passthrough := s.resolvePlayerMatchContext(ctx, playerQuery, matchQuery, opts, EntityPlayerDelivery)
	if passthrough != nil {
		return *passthrough, nil
	}

	resolved, deliveries, deliveryWarnings, err := s.fetchPlayerDeliveries(ctx, contextData, false)
	if err != nil {
		return NewTransportErrorResult(EntityPlayerDelivery, matchSubresourceRef(contextData.match, "details", "details"), err), nil
	}

	items := make([]any, 0, len(deliveries))
	for _, delivery := range deliveries {
		items = append(items, delivery)
	}

	warnings := append([]string{}, contextData.warnings...)
	warnings = append(warnings, contextData.routeWarnings...)
	warnings = append(warnings, deliveryWarnings...)
	result := NewListResult(EntityPlayerDelivery, items)
	if compact := compactWarnings(warnings); len(compact) > 0 {
		result = NewPartialListResult(EntityPlayerDelivery, items, compact...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	if len(items) == 0 && strings.TrimSpace(result.Message) == "" {
		result.Message = fmt.Sprintf("no delivery events found for player %q in match %q", contextData.playerName, contextData.match.ID)
	}
	return result, nil
}

// Bowling resolves only the bowling-focused player-in-match categories.
func (s *PlayerService) Bowling(ctx context.Context, playerQuery, matchQuery string, opts PlayerLookupOptions) (NormalizedResult, error) {
	return s.playerMatchSplitView(ctx, playerQuery, matchQuery, opts, "bowling")
}

// Batting resolves only the batting-focused player-in-match categories.
func (s *PlayerService) Batting(ctx context.Context, playerQuery, matchQuery string, opts PlayerLookupOptions) (NormalizedResult, error) {
	return s.playerMatchSplitView(ctx, playerQuery, matchQuery, opts, "batting")
}

func (s *PlayerService) playerMatchSplitView(
	ctx context.Context,
	playerQuery, matchQuery string,
	opts PlayerLookupOptions,
	view string,
) (NormalizedResult, error) {
	result, err := s.MatchStats(ctx, playerQuery, matchQuery, opts)
	if err != nil {
		return result, err
	}
	if result.Status == ResultStatusError || result.Data == nil {
		return result, nil
	}

	playerMatch, ok := result.Data.(PlayerMatch)
	if !ok {
		return result, nil
	}

	switch strings.ToLower(strings.TrimSpace(view)) {
	case "batting":
		playerMatch.Bowling = nil
		playerMatch.Fielding = nil
		playerMatch.Summary = summarizePlayerMatchCategories(playerMatch.Batting)
	case "bowling":
		playerMatch.Batting = nil
		playerMatch.Fielding = nil
		playerMatch.Summary = summarizePlayerMatchCategories(playerMatch.Bowling)
	default:
		// no-op
	}

	result.Data = playerMatch
	return result, nil
}

type playerMatchContext struct {
	playerID      string
	playerName    string
	playerEntity  IndexedEntity
	match         Match
	team          Team
	roster        TeamRosterEntry
	warnings      []string
	routeWarnings []string
}

type wicketMetadata struct {
	team    Team
	innings Innings
	wicket  InningsWicket
}

type playerLookup struct {
	entity    IndexedEntity
	player    Player
	resolved  *ResolvedDocument
	warnings  []string
	statsRef  string
	statsKind EntityKind
}

func (s *PlayerService) statistics(ctx context.Context, query string, opts PlayerLookupOptions) (NormalizedResult, error) {
	lookup, passthrough := s.resolvePlayerLookup(ctx, query, opts, EntityPlayerStats)
	if passthrough != nil {
		return *passthrough, nil
	}

	statsRef := nonEmpty(lookup.statsRef, "/athletes/"+strings.TrimSpace(lookup.player.ID)+"/statistics")
	resolved, err := s.client.ResolveRefChain(ctx, statsRef)
	if err != nil {
		return NewTransportErrorResult(EntityPlayerStats, statsRef, err), nil
	}

	playerStats, err := NormalizePlayerStatistics(resolved.Body)
	if err != nil {
		return NormalizedResult{}, fmt.Errorf("normalize player statistics %q: %w", resolved.CanonicalRef, err)
	}
	if strings.TrimSpace(lookup.player.ID) != "" {
		playerStats.PlayerID = strings.TrimSpace(lookup.player.ID)
	}
	if strings.TrimSpace(lookup.player.Ref) != "" {
		playerStats.PlayerRef = strings.TrimSpace(lookup.player.Ref)
	}

	result := NewDataResult(EntityPlayerStats, *playerStats)
	if len(lookup.warnings) > 0 {
		result = NewPartialResult(EntityPlayerStats, *playerStats, lookup.warnings...)
	}
	result.RequestedRef = resolved.RequestedRef
	result.CanonicalRef = resolved.CanonicalRef
	return result, nil
}

func (s *PlayerService) resolvePlayerLookup(ctx context.Context, query string, opts PlayerLookupOptions, kind EntityKind) (*playerLookup, *NormalizedResult) {
	query = strings.TrimSpace(query)
	if query == "" {
		result := NormalizedResult{Kind: kind, Status: ResultStatusEmpty, Message: "player query is required"}
		return nil, &result
	}

	searchResult, err := s.resolver.Search(ctx, EntityPlayer, query, ResolveOptions{
		Limit:    5,
		LeagueID: strings.TrimSpace(opts.LeagueID),
	})
	if err != nil {
		result := NewTransportErrorResult(kind, query, err)
		return nil, &result
	}
	if len(searchResult.Entities) == 0 {
		result := NormalizedResult{Kind: kind, Status: ResultStatusEmpty, Message: fmt.Sprintf("no players found for %q", query)}
		return nil, &result
	}

	entity := searchResult.Entities[0]
	ref := nonEmpty(strings.TrimSpace(entity.Ref), "/athletes/"+strings.TrimSpace(entity.ID))
	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		result := NewTransportErrorResult(kind, ref, err)
		return nil, &result
	}

	player, err := NormalizePlayer(resolved.Body)
	if err != nil {
		return nil, &NormalizedResult{
			Kind:    kind,
			Status:  ResultStatusError,
			Message: fmt.Sprintf("normalize player profile %q: %v", resolved.CanonicalRef, err),
		}
	}
	s.enrichPlayerProfile(ctx, player)

	return &playerLookup{
		entity:    entity,
		player:    *player,
		resolved:  resolved,
		warnings:  searchResult.Warnings,
		statsRef:  "/athletes/" + strings.TrimSpace(player.ID) + "/statistics",
		statsKind: kind,
	}, nil
}

func (s *PlayerService) enrichPlayerProfile(ctx context.Context, player *Player) {
	if s == nil || s.resolver == nil || player == nil {
		return
	}
	if player.Team != nil {
		enriched := s.enrichPlayerAffiliation(ctx, *player.Team)
		player.Team = &enriched
	}
	for i := range player.MajorTeams {
		player.MajorTeams[i] = s.enrichPlayerAffiliation(ctx, player.MajorTeams[i])
	}
}

func (s *PlayerService) enrichPlayerAffiliation(ctx context.Context, affiliation PlayerAffiliation) PlayerAffiliation {
	teamID := strings.TrimSpace(affiliation.ID)
	if teamID == "" {
		teamID = strings.TrimSpace(refIDs(affiliation.Ref)["teamId"])
	}
	if teamID == "" {
		return affiliation
	}
	affiliation.ID = teamID
	if strings.TrimSpace(affiliation.Name) != "" {
		return affiliation
	}
	if s.resolver != nil {
		_ = s.resolver.seedTeamByID(ctx, teamID, "", "")
		if indexed, ok := s.resolver.index.FindByID(EntityTeam, teamID); ok {
			affiliation.Name = nonEmpty(indexed.Name, indexed.ShortName)
			if strings.TrimSpace(affiliation.Ref) == "" {
				affiliation.Ref = indexed.Ref
			}
		}
	}
	return affiliation
}

func (s *PlayerService) resolvePlayerMatchContext(
	ctx context.Context,
	playerQuery, matchQuery string,
	opts PlayerLookupOptions,
	kind EntityKind,
) (*playerMatchContext, *NormalizedResult) {
	playerQuery = strings.TrimSpace(playerQuery)
	matchQuery = strings.TrimSpace(matchQuery)
	if playerQuery == "" {
		result := NormalizedResult{Kind: kind, Status: ResultStatusEmpty, Message: "player query is required"}
		return nil, &result
	}
	if matchQuery == "" {
		result := NormalizedResult{Kind: kind, Status: ResultStatusEmpty, Message: "--match is required"}
		return nil, &result
	}

	match, warnings, passthrough := s.resolveMatchForPlayer(ctx, matchQuery, opts.LeagueID, kind)
	if passthrough != nil {
		return nil, passthrough
	}

	searchResult, err := s.resolver.Search(ctx, EntityPlayer, playerQuery, ResolveOptions{
		Limit:    10,
		LeagueID: nonEmpty(strings.TrimSpace(opts.LeagueID), match.LeagueID),
		MatchID:  strings.TrimSpace(match.ID),
	})
	if err != nil {
		result := NewTransportErrorResult(kind, playerQuery, err)
		return nil, &result
	}
	warnings = append(warnings, searchResult.Warnings...)

	candidateIDs := make([]string, 0, len(searchResult.Entities)+1)
	candidateNames := make([]string, 0, len(searchResult.Entities)+1)
	for _, entity := range searchResult.Entities {
		if strings.TrimSpace(entity.ID) != "" {
			candidateIDs = append(candidateIDs, strings.TrimSpace(entity.ID))
		}
		name := nonEmpty(entity.Name, entity.ShortName)
		if name != "" {
			candidateNames = append(candidateNames, strings.TrimSpace(name))
		}
	}
	if isNumeric(playerQuery) {
		candidateIDs = append(candidateIDs, strings.TrimSpace(playerQuery))
	}

	team, roster, routeWarnings, found := s.findPlayerRosterEntry(ctx, *match, playerQuery, candidateIDs, candidateNames)
	warnings = append(warnings, routeWarnings...)
	if !found {
		result := NormalizedResult{
			Kind:    kind,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("player %q not found in match %q roster", playerQuery, match.ID),
		}
		return nil, &result
	}
	team = s.enrichTeamIdentityFromIndex(team)

	playerID := strings.TrimSpace(roster.PlayerID)
	if playerID == "" {
		playerID = firstNonEmptyString(candidateIDs...)
	}
	if playerID == "" {
		result := NormalizedResult{
			Kind:    kind,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("unable to resolve player id for %q in match %q", playerQuery, match.ID),
		}
		return nil, &result
	}

	roster = s.enrichRosterEntryFromIndex(roster)
	playerName := nonEmpty(roster.DisplayName, firstNonEmptyString(candidateNames...), strings.TrimSpace(playerQuery), "Unknown Player")
	playerEntity := IndexedEntity{Kind: EntityPlayer, ID: playerID, Name: playerName}
	if len(searchResult.Entities) > 0 {
		playerEntity = searchResult.Entities[0]
		if strings.TrimSpace(playerEntity.ID) == "" {
			playerEntity.ID = playerID
		}
		if strings.TrimSpace(playerEntity.Name) == "" {
			playerEntity.Name = playerName
		}
	}

	return &playerMatchContext{
		playerID:      playerID,
		playerName:    playerName,
		playerEntity:  playerEntity,
		match:         *match,
		team:          team,
		roster:        roster,
		warnings:      compactWarnings(warnings),
		routeWarnings: compactWarnings(routeWarnings),
	}, nil
}

func (s *PlayerService) resolveMatchForPlayer(
	ctx context.Context,
	matchQuery, leagueID string,
	kind EntityKind,
) (*Match, []string, *NormalizedResult) {
	searchResult, err := s.resolver.Search(ctx, EntityMatch, strings.TrimSpace(matchQuery), ResolveOptions{
		Limit:    5,
		LeagueID: strings.TrimSpace(leagueID),
	})
	if err != nil {
		result := NewTransportErrorResult(kind, matchQuery, err)
		return nil, nil, &result
	}
	if len(searchResult.Entities) == 0 {
		result := NormalizedResult{
			Kind:    kind,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("no matches found for %q", matchQuery),
		}
		return nil, nil, &result
	}

	ref := buildMatchRef(searchResult.Entities[0])
	if ref == "" {
		result := NormalizedResult{
			Kind:    kind,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("unable to resolve match ref for %q", matchQuery),
		}
		return nil, nil, &result
	}

	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		result := NewTransportErrorResult(kind, ref, err)
		return nil, nil, &result
	}

	match, err := NormalizeMatch(resolved.Body)
	if err != nil {
		result := NormalizedResult{
			Kind:    kind,
			Status:  ResultStatusError,
			Message: fmt.Sprintf("normalize competition match %q: %v", resolved.CanonicalRef, err),
		}
		return nil, nil, &result
	}
	statusCache := map[string]matchStatusSnapshot{}
	teamCache := map[string]teamIdentity{}
	scoreCache := map[string]string{}
	helper := &MatchService{client: s.client, resolver: s.resolver}
	warnings := append([]string{}, searchResult.Warnings...)
	warnings = append(warnings, helper.hydrateMatch(ctx, match, statusCache, teamCache, scoreCache)...)
	return match, compactWarnings(warnings), nil
}

func (s *PlayerService) findPlayerRosterEntry(
	ctx context.Context,
	match Match,
	playerQuery string,
	candidateIDs []string,
	candidateNames []string,
) (Team, TeamRosterEntry, []string, bool) {
	normalizedQuery := normalizeAlias(playerQuery)
	queryTokens := strings.Fields(normalizedQuery)
	useCandidateIDs := isNumeric(strings.TrimSpace(playerQuery)) || isKnownRefQuery(strings.TrimSpace(playerQuery))
	idSet := map[string]struct{}{}
	for _, id := range candidateIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		idSet[id] = struct{}{}
	}
	warnings := make([]string, 0)
	bestScore := 0
	var bestTeam Team
	var bestEntry TeamRosterEntry
	for _, team := range match.Teams {
		rosterRef := nonEmpty(strings.TrimSpace(team.RosterRef), competitorSubresourceRef(match, team.ID, "roster"))
		if rosterRef == "" {
			continue
		}

		resolved, err := s.client.ResolveRefChain(ctx, rosterRef)
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
			entry = s.enrichRosterEntryFromIndex(entry)
			playerID := strings.TrimSpace(entry.PlayerID)

			score := 0
			if useCandidateIDs {
				if _, ok := idSet[playerID]; ok && playerID != "" {
					score = 5000
				}
			}
			if !useCandidateIDs && normalizedQuery != "" {
				if normalizedID := normalizeAlias(playerID); normalizedID != "" && normalizedID == normalizedQuery {
					score = 5000
				}
			}

			aliases := compactWarnings([]string{
				entry.DisplayName,
				playerID,
				refIDs(entry.PlayerRef)["athleteId"],
			})
			for _, alias := range aliases {
				normalizedAlias := normalizeAlias(alias)
				if normalizedAlias == "" || normalizedQuery == "" {
					continue
				}
				score = maxInt(score, aliasMatchScore(normalizedAlias, normalizedQuery, queryTokens))
			}

			if score > bestScore {
				bestScore = score
				bestTeam = team
				bestEntry = entry
			}
		}
	}

	if bestScore >= 300 {
		return bestTeam, bestEntry, compactWarnings(warnings), true
	}
	return Team{}, TeamRosterEntry{}, compactWarnings(warnings), false
}

func (s *PlayerService) enrichRosterEntryFromIndex(entry TeamRosterEntry) TeamRosterEntry {
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

func (s *PlayerService) enrichTeamIdentityFromIndex(team Team) Team {
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

func rosterPlayerStatisticsRef(match Match, team Team, entry TeamRosterEntry) string {
	if ref := strings.TrimSpace(entry.StatisticsRef); ref != "" {
		return ref
	}
	if base := competitorSubresourceRef(match, team.ID, ""); base != "" && strings.TrimSpace(entry.PlayerID) != "" {
		return strings.TrimRight(base, "/") + "/roster/" + strings.TrimSpace(entry.PlayerID) + "/statistics/0"
	}
	return ""
}

func rosterPlayerLinescoresRef(match Match, team Team, entry TeamRosterEntry) string {
	if ref := strings.TrimSpace(entry.LinescoresRef); ref != "" {
		return ref
	}
	if base := competitorSubresourceRef(match, team.ID, ""); base != "" && strings.TrimSpace(entry.PlayerID) != "" {
		return strings.TrimRight(base, "/") + "/roster/" + strings.TrimSpace(entry.PlayerID) + "/linescores"
	}
	return ""
}

func rosterPlayerLinescoreStatisticsRef(match Match, team Team, entry TeamRosterEntry, innings, period int) string {
	if innings <= 0 || period <= 0 {
		return ""
	}
	base := competitorSubresourceRef(match, team.ID, "")
	if base == "" || strings.TrimSpace(entry.PlayerID) == "" {
		return ""
	}
	return fmt.Sprintf("%s/roster/%s/linescores/%d/%d/statistics/0", strings.TrimRight(base, "/"), strings.TrimSpace(entry.PlayerID), innings, period)
}

func firstPlayerLinescoreStatisticsRef(row map[string]any) string {
	linescores := mapSliceField(row, "linescores")
	if len(linescores) == 0 {
		return ""
	}
	return stringField(linescores[0], "statistics")
}

func (s *PlayerService) fetchStatCategories(ctx context.Context, ref string) (*ResolvedDocument, []StatCategory, error) {
	resolved, err := s.client.ResolveRefChain(ctx, ref)
	if err != nil {
		return nil, nil, err
	}

	categories, err := NormalizeStatCategories(resolved.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("normalize stat categories %q: %w", resolved.CanonicalRef, err)
	}
	return resolved, categories, nil
}

func splitPlayerStatCategories(categories []StatCategory) ([]StatCategory, []StatCategory, []StatCategory) {
	batting := make([]StatCategory, 0)
	bowling := make([]StatCategory, 0)
	fielding := make([]StatCategory, 0)

	for _, category := range categories {
		role := playerStatCategoryRole(category)
		switch role {
		case "batting":
			batting = append(batting, category)
		case "bowling":
			bowling = append(bowling, category)
		default:
			fielding = append(fielding, category)
		}
	}

	return batting, bowling, fielding
}

func playerStatCategoryRole(category StatCategory) string {
	battingScore := 0
	bowlingScore := 0
	fieldingScore := 0
	for _, stat := range category.Stats {
		name := normalizeStatName(stat.Name)
		switch name {
		case "ballsfaced", "batted", "battingid", "battingposition", "ducks", "fiftyplus", "fours", "highscore", "hundreds", "minutes", "notouts", "outs", "retireddescription", "runs", "sixes", "strikerate", "dismissalname", "dismissalcard":
			battingScore++
		case "balls", "bowled", "bowlingid", "bowlingposition", "bpo", "conceded", "dots", "economyrate", "fivewickets", "fourpluswickets", "foursconceded", "illegaloverlimit", "maidens", "noballs", "overs", "sixesconceded", "tenwickets", "wickets", "wides":
			bowlingScore++
		case "dismissals", "fielded", "caught", "caughtfielder", "caughtkeeper", "stumped", "runout":
			fieldingScore++
		}
	}

	switch {
	case battingScore >= bowlingScore && battingScore >= fieldingScore && battingScore > 0:
		return "batting"
	case bowlingScore >= battingScore && bowlingScore >= fieldingScore && bowlingScore > 0:
		return "bowling"
	case fieldingScore > 0:
		return "fielding"
	default:
		return "fielding"
	}
}

func summarizePlayerMatchCategories(categories []StatCategory) PlayerMatchSummary {
	summary := PlayerMatchSummary{}
	ballsBowled := 0
	concededRuns := 0
	strikeRateCount := 0
	economyRateCount := 0
	totalRuns := 0

	for _, category := range categories {
		for _, stat := range category.Stats {
			name := normalizeStatName(stat.Name)
			intValue := statAsInt(stat)
			floatValue := statAsFloat(stat)
			stringValue := firstNonEmptyString(strings.TrimSpace(stat.DisplayValue), statAsString(stat))

			switch name {
			case "dismissalname":
				if summary.DismissalName == "" {
					summary.DismissalName = stringValue
				}
			case "dismissalcard":
				if summary.DismissalCard == "" {
					summary.DismissalCard = stringValue
				}
			case "ballsfaced":
				summary.BallsFaced += intValue
			case "strikerate":
				if floatValue > 0 {
					summary.StrikeRate += floatValue
					strikeRateCount++
				}
			case "dots":
				summary.Dots += intValue
			case "economyrate":
				if floatValue > 0 {
					summary.EconomyRate += floatValue
					economyRateCount++
				}
			case "maidens":
				summary.Maidens += intValue
			case "foursconceded":
				summary.FoursConceded += intValue
			case "sixesconceded":
				summary.SixesConceded += intValue
			case "wides":
				summary.Wides += intValue
			case "noballs":
				summary.Noballs += intValue
			case "bowlerplayerid":
				if summary.BowlerPlayerID == "" {
					summary.BowlerPlayerID = stringValue
				}
			case "fielderplayerid":
				if summary.FielderPlayerID == "" {
					summary.FielderPlayerID = stringValue
				}
			case "runs":
				totalRuns += intValue
			case "balls":
				ballsBowled += intValue
			case "conceded":
				concededRuns += intValue
			}
		}
	}

	if summary.BallsFaced > 0 && totalRuns > 0 {
		summary.StrikeRate = (float64(totalRuns) * 100) / float64(summary.BallsFaced)
	} else if strikeRateCount > 0 {
		summary.StrikeRate = summary.StrikeRate / float64(strikeRateCount)
	}
	if ballsBowled > 0 && concededRuns > 0 {
		overs := float64(ballsBowled) / 6.0
		if overs > 0 {
			summary.EconomyRate = float64(concededRuns) / overs
		}
	} else if economyRateCount > 0 {
		summary.EconomyRate = summary.EconomyRate / float64(economyRateCount)
	}

	return summary
}

func normalizeStatName(name string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(name)))
}

func statAsString(stat StatValue) string {
	switch typed := stat.Value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%g", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", stat.Value))
	}
}

func statAsInt(stat StatValue) int {
	raw := firstNonEmptyString(statAsString(stat), strings.TrimSpace(stat.DisplayValue))
	value, err := strconvAtoi(raw)
	if err == nil {
		return value
	}
	floatValue, floatErr := strconvParseFloat(raw)
	if floatErr == nil {
		return int(floatValue)
	}
	return 0
}

func statAsFloat(stat StatValue) float64 {
	raw := firstNonEmptyString(statAsString(stat), strings.TrimSpace(stat.DisplayValue))
	value, err := strconvParseFloat(raw)
	if err == nil {
		return value
	}
	return 0
}

func (s *PlayerService) fetchPlayerDeliveries(
	ctx context.Context,
	contextData *playerMatchContext,
	dismissalsOnly bool,
) (*ResolvedDocument, []DeliveryEvent, []string, error) {
	detailsRef := nonEmpty(strings.TrimSpace(contextData.match.DetailsRef), matchSubresourceRef(contextData.match, "details", "details"))
	if detailsRef == "" {
		return nil, nil, nil, fmt.Errorf("details route unavailable for match %q", contextData.match.ID)
	}

	resolved, err := s.client.ResolveRefChain(ctx, detailsRef)
	if err != nil {
		return nil, nil, nil, err
	}

	page, err := DecodePage[Ref](resolved.Body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode details page %q: %w", resolved.CanonicalRef, err)
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

	deliveries := make([]DeliveryEvent, 0, len(pageItems))
	for _, item := range pageItems {
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
			warnings = append(warnings, fmt.Sprintf("detail %s: %v", itemResolved.CanonicalRef, normalizeErr))
			continue
		}

		roles := playerInvolvement(*delivery, contextData.playerID)
		if len(roles) == 0 {
			continue
		}
		if dismissalsOnly && !isDismissalDelivery(*delivery) {
			continue
		}

		delivery.MatchID = nonEmpty(delivery.MatchID, contextData.match.ID)
		delivery.CompetitionID = nonEmpty(delivery.CompetitionID, contextData.match.CompetitionID, contextData.match.ID)
		delivery.EventID = nonEmpty(delivery.EventID, contextData.match.EventID)
		delivery.LeagueID = nonEmpty(delivery.LeagueID, contextData.match.LeagueID)
		delivery.TeamID = nonEmpty(delivery.TeamID, contextData.team.ID)
		delivery.Involvement = roles
		deliveries = append(deliveries, *delivery)
	}

	return resolved, deliveries, compactWarnings(warnings), nil
}

func (s *PlayerService) collectMatchWicketMetadata(ctx context.Context, match Match) (map[string]wicketMetadata, map[string]wicketMetadata, []string) {
	helper := &MatchService{client: s.client, resolver: s.resolver}
	byRef := map[string]wicketMetadata{}
	byDetailID := map[string]wicketMetadata{}
	warnings := make([]string, 0)

	for _, team := range match.Teams {
		inningsList, _, inningsWarnings := helper.fetchTeamInnings(ctx, match, team)
		warnings = append(warnings, inningsWarnings...)

		for i := range inningsList {
			warnings = append(warnings, helper.hydrateInningsTimelines(ctx, &inningsList[i])...)
			for _, wicket := range inningsList[i].WicketTimeline {
				if strings.TrimSpace(wicket.DetailRef) == "" {
					continue
				}
				meta := wicketMetadata{
					team:    team,
					innings: inningsList[i],
					wicket:  wicket,
				}
				byRef[strings.TrimSpace(wicket.DetailRef)] = meta
				if detailID := strings.TrimSpace(refIDs(wicket.DetailRef)["detailId"]); detailID != "" {
					byDetailID[detailID] = meta
				}
			}
		}
	}

	return byRef, byDetailID, compactWarnings(warnings)
}

func playerInvolvement(delivery DeliveryEvent, playerID string) []string {
	playerID = strings.TrimSpace(playerID)
	if playerID == "" {
		return nil
	}

	roles := make([]string, 0, 3)
	if strings.TrimSpace(delivery.BatsmanPlayerID) == playerID {
		roles = append(roles, "batting")
	}
	if strings.TrimSpace(delivery.BowlerPlayerID) == playerID {
		roles = append(roles, "bowling")
	}
	if strings.TrimSpace(delivery.FielderPlayerID) == playerID {
		roles = append(roles, "fielding")
	}
	for _, involved := range delivery.AthletePlayerIDs {
		if strings.TrimSpace(involved) == playerID && !containsString(roles, "involved") {
			roles = append(roles, "involved")
			break
		}
	}
	if len(roles) == 0 {
		return nil
	}
	return roles
}

func isDismissalDelivery(delivery DeliveryEvent) bool {
	return boolField(delivery.Dismissal, "dismissal")
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(needle) {
			return true
		}
	}
	return false
}

func teamDisplayLabel(team Team) string {
	return nonEmpty(strings.TrimSpace(team.ShortName), strings.TrimSpace(team.Name), "Unknown Team")
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func strconvAtoi(raw string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(raw))
}

func strconvParseFloat(raw string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(raw), 64)
}

func limitOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
