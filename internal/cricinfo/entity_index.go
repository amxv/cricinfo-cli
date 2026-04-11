package cricinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	entityIndexVersion  = 1
	entityIndexFileName = "entity-index-v1.json"
)

var ErrMissingEntityID = errors.New("entity id is required")

// IndexedEntity stores a lightweight searchable record for a known Cricinfo entity.
type IndexedEntity struct {
	Kind       EntityKind `json:"kind"`
	ID         string     `json:"id"`
	Ref        string     `json:"ref,omitempty"`
	Name       string     `json:"name,omitempty"`
	ShortName  string     `json:"shortName,omitempty"`
	LeagueID   string     `json:"leagueId,omitempty"`
	EventID    string     `json:"eventId,omitempty"`
	MatchID    string     `json:"matchId,omitempty"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	Aliases    []string   `json:"aliases,omitempty"`
	SourceRefs []string   `json:"sourceRefs,omitempty"`
}

// SearchContext improves ranking using currently selected domain context.
type SearchContext struct {
	PreferredLeagueID string
	PreferredMatchID  string
}

type indexFileState struct {
	Version          int                  `json:"version"`
	SavedAt          time.Time            `json:"savedAt"`
	LastEventsSeedAt time.Time            `json:"lastEventsSeedAt,omitempty"`
	HydratedRefs     map[string]time.Time `json:"hydratedRefs,omitempty"`
	Entities         []IndexedEntity      `json:"entities"`
}

// EntityIndex provides alias-backed search over cached entities.
type EntityIndex struct {
	mu sync.RWMutex

	path string

	lastEventsSeedAt time.Time
	hydratedRefs     map[string]time.Time

	entitiesByKey map[string]IndexedEntity
	aliasesByKey  map[string]map[string]struct{}
	dirty         bool
}

// OpenEntityIndex loads a file-backed index (or creates a new empty index).
func OpenEntityIndex(path string) (*EntityIndex, error) {
	resolved := strings.TrimSpace(path)
	if resolved == "" {
		defaultPath, err := DefaultEntityIndexPath()
		if err != nil {
			return nil, err
		}
		resolved = defaultPath
	}

	idx := &EntityIndex{
		path:          resolved,
		hydratedRefs:  map[string]time.Time{},
		entitiesByKey: map[string]IndexedEntity{},
		aliasesByKey:  map[string]map[string]struct{}{},
	}

	blob, err := os.ReadFile(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return idx, nil
		}
		return nil, fmt.Errorf("read entity index %q: %w", resolved, err)
	}
	if len(strings.TrimSpace(string(blob))) == 0 {
		return idx, nil
	}

	var state indexFileState
	if err := json.Unmarshal(blob, &state); err != nil {
		return nil, fmt.Errorf("decode entity index %q: %w", resolved, err)
	}

	for _, entity := range state.Entities {
		idx.upsertNoLock(entity)
	}
	idx.lastEventsSeedAt = state.LastEventsSeedAt
	for rawRef, seenAt := range state.HydratedRefs {
		ref := normalizeRef(rawRef)
		if ref == "" {
			continue
		}
		idx.hydratedRefs[ref] = seenAt
	}

	idx.dirty = false
	return idx, nil
}

// DefaultEntityIndexPath returns the default cache location for the index file.
func DefaultEntityIndexPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}
	return filepath.Join(cacheDir, "cricinfo-cli", entityIndexFileName), nil
}

// Persist writes the index to disk if any mutations have occurred.
func (i *EntityIndex) Persist() error {
	i.mu.RLock()
	if !i.dirty {
		i.mu.RUnlock()
		return nil
	}

	state := indexFileState{
		Version:          entityIndexVersion,
		SavedAt:          time.Now().UTC(),
		LastEventsSeedAt: i.lastEventsSeedAt,
		HydratedRefs:     map[string]time.Time{},
		Entities:         make([]IndexedEntity, 0, len(i.entitiesByKey)),
	}

	for ref, seenAt := range i.hydratedRefs {
		state.HydratedRefs[ref] = seenAt
	}
	for _, entity := range i.entitiesByKey {
		state.Entities = append(state.Entities, entity)
	}
	i.mu.RUnlock()

	sort.Slice(state.Entities, func(a, b int) bool {
		if state.Entities[a].Kind != state.Entities[b].Kind {
			return state.Entities[a].Kind < state.Entities[b].Kind
		}
		return state.Entities[a].ID < state.Entities[b].ID
	})

	blob, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode entity index: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(i.path), 0o755); err != nil {
		return fmt.Errorf("create index directory: %w", err)
	}

	tmpPath := i.path + ".tmp"
	if err := os.WriteFile(tmpPath, blob, 0o644); err != nil {
		return fmt.Errorf("write temp entity index: %w", err)
	}
	if err := os.Rename(tmpPath, i.path); err != nil {
		return fmt.Errorf("replace entity index: %w", err)
	}

	i.mu.Lock()
	i.dirty = false
	i.mu.Unlock()
	return nil
}

// Upsert inserts or updates an entity record.
func (i *EntityIndex) Upsert(entity IndexedEntity) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.upsertNoLock(entity)
}

// UpsertMany inserts or updates many entity records.
func (i *EntityIndex) UpsertMany(entities []IndexedEntity) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, entity := range entities {
		if err := i.upsertNoLock(entity); err != nil {
			return err
		}
	}
	return nil
}

func (i *EntityIndex) upsertNoLock(entity IndexedEntity) error {
	entity.Kind = EntityKind(strings.TrimSpace(string(entity.Kind)))
	entity.ID = strings.TrimSpace(entity.ID)
	if entity.Kind == "" {
		return fmt.Errorf("entity kind is required")
	}
	if entity.ID == "" {
		return ErrMissingEntityID
	}

	entity.Ref = normalizeRef(entity.Ref)
	entity.Name = strings.TrimSpace(entity.Name)
	entity.ShortName = strings.TrimSpace(entity.ShortName)
	entity.LeagueID = strings.TrimSpace(entity.LeagueID)
	entity.EventID = strings.TrimSpace(entity.EventID)
	entity.MatchID = strings.TrimSpace(entity.MatchID)
	if entity.UpdatedAt.IsZero() {
		entity.UpdatedAt = time.Now().UTC()
	}

	key := entityIndexKey(entity.Kind, entity.ID)
	existing, exists := i.entitiesByKey[key]
	if exists {
		entity = mergeIndexedEntity(existing, entity)
	}

	entity.Aliases = mergeAliasSlices(existing.Aliases, entity.Aliases, generateDefaultAliases(entity))
	entity.SourceRefs = mergeAliasSlices(existing.SourceRefs, entity.SourceRefs, []string{entity.Ref})

	i.entitiesByKey[key] = entity
	i.aliasesByKey[key] = aliasSet(entity.Aliases)
	i.dirty = true
	return nil
}

// FindByID returns a cached entity by kind/id.
func (i *EntityIndex) FindByID(kind EntityKind, id string) (IndexedEntity, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return IndexedEntity{}, false
	}
	entity, ok := i.entitiesByKey[entityIndexKey(kind, id)]
	return entity, ok
}

// FindByRef returns a cached entity by exact canonical ref.
func (i *EntityIndex) FindByRef(kind EntityKind, ref string) (IndexedEntity, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	normalized := normalizeRef(ref)
	if normalized == "" {
		return IndexedEntity{}, false
	}

	for _, entity := range i.entitiesByKey {
		if entity.Kind != kind {
			continue
		}
		if normalizeRef(entity.Ref) == normalized {
			return entity, true
		}
		for _, sourceRef := range entity.SourceRefs {
			if normalizeRef(sourceRef) == normalized {
				return entity, true
			}
		}
	}

	return IndexedEntity{}, false
}

// Search performs exact/fuzzy alias lookup for a single entity family.
func (i *EntityIndex) Search(kind EntityKind, query string, limit int, context SearchContext) []IndexedEntity {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	queryNormalized := normalizeAlias(query)
	queryTokens := strings.Fields(queryNormalized)

	type scored struct {
		entity IndexedEntity
		score  int
	}

	matches := make([]scored, 0)
	for key, entity := range i.entitiesByKey {
		if entity.Kind != kind {
			continue
		}

		score := 0
		if queryNormalized == "" {
			score = 10
		} else {
			aliases := i.aliasesByKey[key]
			for alias := range aliases {
				score = maxInt(score, aliasMatchScore(alias, queryNormalized, queryTokens))
			}
		}

		if score == 0 {
			continue
		}
		if context.PreferredLeagueID != "" && entity.LeagueID == context.PreferredLeagueID {
			score += 200
		}
		if context.PreferredMatchID != "" && entity.MatchID == context.PreferredMatchID {
			score += 500
		}

		matches = append(matches, scored{entity: entity, score: score})
	}

	if preferredMatchID := strings.TrimSpace(context.PreferredMatchID); preferredMatchID != "" {
		matchScoped := make([]scored, 0, len(matches))
		for _, candidate := range matches {
			if strings.TrimSpace(candidate.entity.MatchID) == preferredMatchID {
				matchScoped = append(matchScoped, candidate)
			}
		}
		if len(matchScoped) > 0 {
			matches = matchScoped
		}
	}

	if preferredLeagueID := strings.TrimSpace(context.PreferredLeagueID); preferredLeagueID != "" {
		leagueScoped := make([]scored, 0, len(matches))
		for _, candidate := range matches {
			if strings.TrimSpace(candidate.entity.LeagueID) == preferredLeagueID {
				leagueScoped = append(leagueScoped, candidate)
			}
		}
		if len(leagueScoped) > 0 {
			matches = leagueScoped
		}
	}

	sort.Slice(matches, func(a, b int) bool {
		if matches[a].score != matches[b].score {
			return matches[a].score > matches[b].score
		}
		if !matches[a].entity.UpdatedAt.Equal(matches[b].entity.UpdatedAt) {
			return matches[a].entity.UpdatedAt.After(matches[b].entity.UpdatedAt)
		}
		return matches[a].entity.ID < matches[b].entity.ID
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	out := make([]IndexedEntity, 0, len(matches))
	for _, item := range matches {
		out = append(out, item.entity)
	}
	return out
}

// LastEventsSeedAt returns the timestamp for the last /events hydration pass.
func (i *EntityIndex) LastEventsSeedAt() time.Time {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.lastEventsSeedAt
}

// SetLastEventsSeedAt updates the /events hydration marker.
func (i *EntityIndex) SetLastEventsSeedAt(ts time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if ts.IsZero() {
		return
	}
	i.lastEventsSeedAt = ts.UTC()
	i.dirty = true
}

// MarkHydratedRef records that a ref has already been traversed.
func (i *EntityIndex) MarkHydratedRef(ref string, when time.Time) {
	ref = normalizeRef(ref)
	if ref == "" {
		return
	}
	if when.IsZero() {
		when = time.Now().UTC()
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	i.hydratedRefs[ref] = when.UTC()
	i.dirty = true
}

// HydratedRefAt returns when a ref was hydrated, if known.
func (i *EntityIndex) HydratedRefAt(ref string) (time.Time, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	ref = normalizeRef(ref)
	if ref == "" {
		return time.Time{}, false
	}
	ts, ok := i.hydratedRefs[ref]
	return ts, ok
}

func mergeIndexedEntity(existing, incoming IndexedEntity) IndexedEntity {
	merged := existing
	if incoming.Ref != "" {
		merged.Ref = incoming.Ref
	}
	if incoming.Name != "" {
		merged.Name = incoming.Name
	}
	if incoming.ShortName != "" {
		merged.ShortName = incoming.ShortName
	}
	if incoming.LeagueID != "" {
		merged.LeagueID = incoming.LeagueID
	}
	if incoming.EventID != "" {
		merged.EventID = incoming.EventID
	}
	if incoming.MatchID != "" {
		merged.MatchID = incoming.MatchID
	}
	if !incoming.UpdatedAt.IsZero() && incoming.UpdatedAt.After(existing.UpdatedAt) {
		merged.UpdatedAt = incoming.UpdatedAt
	}
	if merged.UpdatedAt.IsZero() {
		merged.UpdatedAt = time.Now().UTC()
	}
	return merged
}

func generateDefaultAliases(entity IndexedEntity) []string {
	aliases := []string{entity.ID, entity.Name, entity.ShortName}
	if entity.Ref != "" {
		ids := refIDs(entity.Ref)
		switch entity.Kind {
		case EntityPlayer:
			aliases = append(aliases, ids["athleteId"])
		case EntityTeam:
			aliases = append(aliases, ids["teamId"], ids["competitorId"])
		case EntityLeague:
			aliases = append(aliases, ids["leagueId"])
		case EntityMatch:
			aliases = append(aliases, ids["competitionId"], ids["eventId"])
		}
	}
	return aliases
}

func mergeAliasSlices(slices ...[]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, current := range slices {
		for _, value := range current {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			normalized := normalizeAlias(value)
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			out = append(out, value)
			if acronym := aliasAcronym(normalized); acronym != "" {
				if _, ok := seen[acronym]; !ok {
					seen[acronym] = struct{}{}
					out = append(out, acronym)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func aliasAcronym(normalized string) string {
	tokens := strings.Fields(strings.TrimSpace(normalized))
	if len(tokens) < 2 {
		return ""
	}

	var builder strings.Builder
	for _, token := range tokens {
		if token == "" {
			continue
		}
		builder.WriteByte(token[0])
	}

	acronym := strings.TrimSpace(builder.String())
	if len(acronym) < 2 {
		return ""
	}
	return acronym
}

func aliasSet(aliases []string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, alias := range aliases {
		normalized := normalizeAlias(alias)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	return set
}

func aliasMatchScore(alias, query string, queryTokens []string) int {
	if alias == "" || query == "" {
		return 0
	}
	if alias == query {
		return 1000
	}
	if strings.HasPrefix(alias, query) {
		return 800
	}
	if strings.Contains(alias, query) {
		return 650
	}
	aliasTokens := strings.Fields(alias)
	if len(aliasTokens) == 0 || len(queryTokens) == 0 {
		return 0
	}

	matched := 0
	for _, qToken := range queryTokens {
		for _, aliasToken := range aliasTokens {
			if aliasTokenMatchesQuery(aliasToken, qToken) {
				matched++
				break
			}
		}
	}
	if matched == 0 {
		return 0
	}
	return 300 + (matched * 60)
}

func aliasTokenMatchesQuery(aliasToken, queryToken string) bool {
	if aliasToken == "" || queryToken == "" {
		return false
	}
	if aliasToken == queryToken {
		return true
	}
	if strings.HasPrefix(aliasToken, queryToken) {
		return true
	}
	// Avoid treating single-letter initials as a match for full-name tokens.
	return len(aliasToken) >= 2 && strings.HasPrefix(queryToken, aliasToken)
}

func normalizeAlias(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	lastSpace := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func normalizeRef(ref string) string {
	return strings.TrimSpace(ref)
}

func entityIndexKey(kind EntityKind, id string) string {
	return strings.TrimSpace(string(kind)) + ":" + strings.TrimSpace(id)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ToRenderable converts a cached entity into a normalized render contract type.
func (e IndexedEntity) ToRenderable() any {
	switch e.Kind {
	case EntityPlayer:
		return Player{
			Ref:         e.Ref,
			ID:          e.ID,
			DisplayName: e.Name,
			ShortName:   e.ShortName,
		}
	case EntityTeam:
		return Team{
			Ref:       e.Ref,
			ID:        e.ID,
			Name:      e.Name,
			ShortName: e.ShortName,
		}
	case EntityLeague:
		return League{
			Ref:  e.Ref,
			ID:   e.ID,
			Name: e.Name,
			Slug: e.ShortName,
		}
	case EntityMatch:
		return Match{
			Ref:              e.Ref,
			ID:               e.ID,
			CompetitionID:    e.ID,
			EventID:          e.EventID,
			LeagueID:         e.LeagueID,
			Description:      e.Name,
			ShortDescription: e.ShortName,
		}
	default:
		return map[string]any{
			"id":   e.ID,
			"name": e.Name,
			"ref":  e.Ref,
		}
	}
}
