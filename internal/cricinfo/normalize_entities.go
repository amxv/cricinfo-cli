package cricinfo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// NormalizeMatch maps a competition payload into the normalized match shape.
func NormalizeMatch(data []byte) (*Match, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)

	var venueName string
	if venue := mapField(payload, "venue"); venue != nil {
		venueName = stringField(venue, "fullName")
	}

	teams := make([]Team, 0)
	for _, item := range mapSliceField(payload, "competitors") {
		teams = append(teams, normalizeTeamMap(item))
	}

	match := &Match{
		Ref:              ref,
		ID:               nonEmpty(stringField(payload, "id"), ids["competitionId"]),
		UID:              stringField(payload, "uid"),
		LeagueID:         ids["leagueId"],
		EventID:          ids["eventId"],
		CompetitionID:    ids["competitionId"],
		Description:      stringField(payload, "description"),
		ShortDescription: stringField(payload, "shortDescription"),
		Note:             stringField(payload, "note"),
		Date:             stringField(payload, "date"),
		EndDate:          stringField(payload, "endDate"),
		VenueName:        venueName,
		StatusRef:        refFromField(payload, "status"),
		DetailsRef:       refFromField(payload, "details"),
		Teams:            teams,
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "uid", "description", "shortDescription", "note", "date", "endDate",
			"status", "details", "competitors",
		),
	}

	return match, nil
}

// NormalizePlayer maps an athlete profile payload into the normalized player shape.
func NormalizePlayer(data []byte) (*Player, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)

	styles := uniqueStrings(append(styleDescriptions(payload, "style"), styleDescriptions(payload, "styles")...))

	player := &Player{
		Ref:          ref,
		ID:           nonEmpty(stringField(payload, "id"), ids["athleteId"]),
		UID:          stringField(payload, "uid"),
		DisplayName:  stringField(payload, "displayName"),
		FullName:     stringField(payload, "fullName"),
		ShortName:    stringField(payload, "shortName"),
		BattingName:  stringField(payload, "battingName"),
		FieldingName: stringField(payload, "fieldingName"),
		Gender:       stringField(payload, "gender"),
		Age:          intField(payload, "age"),
		TeamRef:      refFromField(payload, "team"),
		Position:     stringField(mapField(payload, "position"), "name"),
		Styles:       styles,
		NewsRef:      refFromField(payload, "news"),
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "uid", "displayName", "fullName", "shortName", "battingName", "fieldingName",
			"gender", "age", "team", "position", "style", "styles", "news",
		),
	}

	return player, nil
}

// NormalizeTeam maps a competitor/team payload into the normalized team shape.
func NormalizeTeam(data []byte) (*Team, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	team := normalizeTeamMap(payload)
	return &team, nil
}

// NormalizeLeague maps a league/root payload into the normalized league shape.
func NormalizeLeague(data []byte) (*League, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)

	league := &League{
		Ref:       ref,
		ID:        nonEmpty(stringField(payload, "id"), ids["leagueId"]),
		UID:       stringField(payload, "uid"),
		Name:      stringField(payload, "name"),
		Slug:      stringField(payload, "slug"),
		SeasonRef: refFromField(payload, "season"),
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "uid", "name", "slug", "season",
		),
	}

	return league, nil
}

// NormalizeSeasonList maps a seasons page payload into normalized season entries.
func NormalizeSeasonList(data []byte) ([]Season, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	items := mapSliceField(payload, "items")
	seasons := make([]Season, 0, len(items))
	for _, item := range items {
		ref := stringField(item, "$ref")
		ids := refIDs(ref)
		season := Season{
			Ref:      ref,
			ID:       ids["seasonId"],
			LeagueID: ids["leagueId"],
			Year:     parseYear(ids["seasonId"]),
			Extensions: extensionsFromMap(item,
				"$ref",
			),
		}
		seasons = append(seasons, season)
	}

	return seasons, nil
}

// NormalizeStandingsGroups maps standings payloads into normalized group entries.
func NormalizeStandingsGroups(data []byte) ([]StandingsGroup, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	items := mapSliceField(payload, "items")
	groups := make([]StandingsGroup, 0, len(items))
	for _, item := range items {
		ref := stringField(item, "$ref")
		ids := refIDs(ref)
		group := StandingsGroup{
			Ref:      ref,
			ID:       ids["standingsId"],
			LeagueID: ids["leagueId"],
			SeasonID: ids["seasonId"],
			GroupID:  ids["groupId"],
			Entries:  normalizeStandingsEntries(item),
			Extensions: extensionsFromMap(item,
				"$ref", "entries",
			),
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// NormalizeInnings maps an innings payload into the normalized innings shape.
func NormalizeInnings(data []byte) (*Innings, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)

	innings := &Innings{
		Ref:             ref,
		ID:              ids["inningsId"],
		Period:          intField(payload, "period"),
		Runs:            intField(payload, "runs"),
		Wickets:         intField(payload, "wickets"),
		Overs:           floatField(payload, "overs"),
		Score:           stringField(payload, "score"),
		Description:     stringField(payload, "description"),
		Target:          intField(payload, "target"),
		StatisticsRef:   refFromField(payload, "statistics"),
		LeadersRef:      refFromField(payload, "leaders"),
		PartnershipsRef: refFromField(payload, "partnerships"),
		FallOfWicketRef: refFromField(payload, "fow"),
		Extensions: extensionsFromMap(payload,
			"$ref", "period", "runs", "wickets", "overs", "score", "description", "target",
			"statistics", "leaders", "partnerships", "fow",
		),
	}

	return innings, nil
}

// NormalizeDeliveryEvent maps a detail payload into the normalized delivery-event shape.
func NormalizeDeliveryEvent(data []byte) (*DeliveryEvent, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	over := mapField(payload, "over")
	dismissal := mapField(payload, "dismissal")

	event := &DeliveryEvent{
		Ref:           ref,
		ID:            nonEmpty(stringField(payload, "id"), ids["detailId"]),
		Period:        intField(payload, "period"),
		PeriodText:    stringField(payload, "periodText"),
		OverNumber:    intField(over, "number"),
		BallNumber:    intField(over, "ball"),
		ScoreValue:    intField(payload, "scoreValue"),
		ShortText:     stringField(payload, "shortText"),
		Text:          stringField(payload, "text"),
		HomeScore:     stringField(payload, "homeScore"),
		AwayScore:     stringField(payload, "awayScore"),
		BatsmanRef:    nestedRef(payload, "batsman", "athlete"),
		BowlerRef:     nestedRef(payload, "bowler", "athlete"),
		DismissalType: stringField(dismissal, "type"),
		DismissalText: stringField(dismissal, "text"),
		SpeedKPH:      floatField(payload, "speedKPH"),
		CoordinateX:   nullableFloatField(payload, "xCoordinate"),
		CoordinateY:   nullableFloatField(payload, "yCoordinate"),
		Timestamp:     int64Field(payload, "bbbTimestamp"),
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "period", "periodText", "over", "scoreValue", "shortText", "text", "homeScore", "awayScore",
			"batsman", "bowler", "dismissal", "speedKPH", "xCoordinate", "yCoordinate", "bbbTimestamp",
		),
	}

	return event, nil
}

// NormalizeStatCategories maps a stats payload into normalized category entries.
func NormalizeStatCategories(data []byte) ([]StatCategory, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	splits := mapField(payload, "splits")
	if splits == nil {
		return []StatCategory{}, nil
	}

	categories := make([]StatCategory, 0)
	for _, item := range mapSliceField(splits, "categories") {
		stats := make([]StatValue, 0)
		for _, statRaw := range mapSliceField(item, "stats") {
			stats = append(stats, StatValue{
				Name:         stringField(statRaw, "name"),
				DisplayName:  stringField(statRaw, "displayName"),
				ShortName:    stringField(statRaw, "shortDisplayName"),
				Description:  stringField(statRaw, "description"),
				Abbreviation: stringField(statRaw, "abbreviation"),
				DisplayValue: stringField(statRaw, "displayValue"),
				Value:        statRaw["value"],
				Type:         stringField(statRaw, "type"),
				Extensions: extensionsFromMap(statRaw,
					"name", "displayName", "shortDisplayName", "description", "abbreviation", "displayValue", "value", "type",
				),
			})
		}

		categories = append(categories, StatCategory{
			Name:         stringField(item, "name"),
			DisplayName:  stringField(item, "displayName"),
			ShortName:    stringField(item, "shortDisplayName"),
			Abbreviation: stringField(item, "abbreviation"),
			Summary:      stringField(item, "summary"),
			Stats:        stats,
			Extensions: extensionsFromMap(item,
				"name", "displayName", "shortDisplayName", "abbreviation", "summary", "stats",
			),
		})
	}

	return categories, nil
}

// NormalizePartnerships maps a partnerships page into normalized partnership entries.
func NormalizePartnerships(data []byte) ([]Partnership, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	items := mapSliceField(payload, "items")
	partnerships := make([]Partnership, 0, len(items))
	for _, item := range items {
		ref := stringField(item, "$ref")
		ids := refIDs(ref)
		partnerships = append(partnerships, Partnership{
			Ref:       ref,
			ID:        ids["partnershipId"],
			InningsID: ids["inningsId"],
			Period:    ids["periodId"],
			Order:     parseInt(ids["partnershipId"]),
			Extensions: extensionsFromMap(item,
				"$ref",
			),
		})
	}

	return partnerships, nil
}

// NormalizeFallOfWickets maps a fall-of-wicket page into normalized entries.
func NormalizeFallOfWickets(data []byte) ([]FallOfWicket, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	items := mapSliceField(payload, "items")
	wickets := make([]FallOfWicket, 0, len(items))
	for _, item := range items {
		ref := stringField(item, "$ref")
		ids := refIDs(ref)
		wickets = append(wickets, FallOfWicket{
			Ref:          ref,
			ID:           ids["fowId"],
			InningsID:    ids["inningsId"],
			WicketNumber: parseInt(ids["fowId"]),
			Extensions: extensionsFromMap(item,
				"$ref",
			),
		})
	}

	return wickets, nil
}

func normalizeStandingsEntries(item map[string]any) []Team {
	entries := make([]Team, 0)
	for _, raw := range mapSliceField(item, "entries") {
		entries = append(entries, normalizeTeamMap(raw))
	}
	return entries
}

func normalizeTeamMap(payload map[string]any) Team {
	ref := stringField(payload, "$ref")
	ids := refIDs(ref)

	name := stringField(payload, "displayName")
	if name == "" {
		name = stringField(payload, "name")
	}
	shortName := stringField(payload, "shortName")
	if shortName == "" {
		shortName = stringField(payload, "shortDisplayName")
	}

	id := nonEmpty(stringField(payload, "id"), ids["teamId"], ids["competitorId"])

	return Team{
		Ref:           ref,
		ID:            id,
		UID:           stringField(payload, "uid"),
		Name:          name,
		ShortName:     shortName,
		Abbreviation:  stringField(payload, "abbreviation"),
		Type:          stringField(payload, "type"),
		HomeAway:      stringField(payload, "homeAway"),
		Order:         intField(payload, "order"),
		Winner:        boolField(payload, "winner"),
		ScoreRef:      refFromField(payload, "score"),
		RosterRef:     refFromField(payload, "roster"),
		LeadersRef:    refFromField(payload, "leaders"),
		StatisticsRef: refFromField(payload, "statistics"),
		RecordRef:     nonEmpty(refFromField(payload, "record"), refFromField(payload, "records")),
		LinescoresRef: refFromField(payload, "linescores"),
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "uid", "displayName", "name", "shortName", "shortDisplayName", "abbreviation",
			"type", "homeAway", "order", "winner", "score", "roster", "leaders", "statistics", "record", "records", "linescores",
		),
	}
}

func decodePayloadMap(data []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	if payload == nil {
		return nil, fmt.Errorf("decode payload: empty object")
	}
	return payload, nil
}

func extensionsFromMap(payload map[string]any, knownKeys ...string) map[string]any {
	if len(payload) == 0 {
		return nil
	}

	known := map[string]struct{}{}
	for _, key := range knownKeys {
		known[key] = struct{}{}
	}

	ext := map[string]any{}
	for key, value := range payload {
		if _, ok := known[key]; ok {
			continue
		}
		ext[key] = value
	}
	if len(ext) == 0 {
		return nil
	}
	return ext
}

func stringField(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func intField(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func int64Field(payload map[string]any, key string) int64 {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func floatField(payload map[string]any, key string) float64 {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func nullableFloatField(payload map[string]any, key string) *float64 {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}
	parsed := floatField(payload, key)
	return &parsed
}

func boolField(payload map[string]any, key string) bool {
	if payload == nil {
		return false
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false
		}
		return parsed
	default:
		return false
	}
}

func mapField(payload map[string]any, key string) map[string]any {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}
	mapped, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return mapped
}

func mapSliceField(payload map[string]any, key string) []map[string]any {
	if payload == nil {
		return []map[string]any{}
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return []map[string]any{}
	}
	rawItems, ok := value.([]any)
	if !ok {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(rawItems))
	for _, item := range rawItems {
		mapped, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, mapped)
	}
	return out
}

func refFromField(payload map[string]any, key string) string {
	ref := mapField(payload, key)
	if ref == nil {
		return ""
	}
	return stringField(ref, "$ref")
}

func nestedRef(payload map[string]any, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}

	current := payload
	for _, key := range keys {
		next := mapField(current, key)
		if next == nil {
			return ""
		}
		current = next
	}

	return stringField(current, "$ref")
}

func styleDescriptions(payload map[string]any, field string) []string {
	entries := mapSliceField(payload, field)
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		description := stringField(entry, "description")
		if description == "" {
			continue
		}
		out = append(out, description)
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
