package cricinfo

import (
	"context"
	"fmt"
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

	return &playerLookup{
		entity:    entity,
		player:    *player,
		resolved:  resolved,
		warnings:  searchResult.Warnings,
		statsRef:  "/athletes/" + strings.TrimSpace(player.ID) + "/statistics",
		statsKind: kind,
	}, nil
}

func limitOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
