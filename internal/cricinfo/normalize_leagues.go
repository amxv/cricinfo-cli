package cricinfo

import (
	"encoding/json"
	"strings"
)

// NormalizeSeason maps a season payload into the normalized season shape.
func NormalizeSeason(data []byte) (*Season, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}
	return NormalizeSeasonFromMap(payload), nil
}

// NormalizeSeasonFromMap maps a decoded season payload into the normalized season shape.
func NormalizeSeasonFromMap(payload map[string]any) *Season {
	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	seasonID := nonEmpty(ids["seasonId"], stringField(payload, "year"))

	return &Season{
		Ref:      ref,
		ID:       seasonID,
		LeagueID: nonEmpty(ids["leagueId"], stringField(payload, "id")),
		Year:     firstNonZeroValue(intField(payload, "year"), parseYear(seasonID)),
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "year",
		),
	}
}

// NormalizeSeasonType maps a season-type payload into the normalized season type shape.
func NormalizeSeasonType(data []byte) (*SeasonType, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	seasonRef := refFromField(payload, "season")
	seasonIDs := refIDs(seasonRef)

	return &SeasonType{
		Ref:          ref,
		ID:           nonEmpty(stringField(payload, "id"), ids["typeId"]),
		LeagueID:     nonEmpty(ids["leagueId"], seasonIDs["leagueId"]),
		SeasonID:     nonEmpty(ids["seasonId"], seasonIDs["seasonId"]),
		Name:         nonEmpty(stringField(payload, "name"), stringField(payload, "displayName")),
		Abbreviation: stringField(payload, "abbreviation"),
		StartDate:    stringField(payload, "startDate"),
		EndDate:      stringField(payload, "endDate"),
		HasGroups:    boolField(payload, "hasGroups"),
		HasStandings: boolField(payload, "hasStandings"),
		GroupsRef:    refFromField(payload, "groups"),
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "name", "displayName", "abbreviation", "startDate", "endDate", "hasGroups", "hasStandings", "groups", "season",
		),
	}, nil
}

// NormalizeSeasonGroup maps a season-group payload into the normalized season group shape.
func NormalizeSeasonGroup(data []byte) (*SeasonGroup, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	seasonRef := refFromField(payload, "season")
	seasonIDs := refIDs(seasonRef)
	standingsRef := refFromField(payload, "standings")
	standingsIDs := refIDs(standingsRef)

	return &SeasonGroup{
		Ref:          ref,
		ID:           nonEmpty(stringField(payload, "id"), ids["groupId"]),
		LeagueID:     nonEmpty(ids["leagueId"], seasonIDs["leagueId"], standingsIDs["leagueId"]),
		SeasonID:     nonEmpty(ids["seasonId"], seasonIDs["seasonId"], standingsIDs["seasonId"]),
		TypeID:       nonEmpty(ids["typeId"], standingsIDs["typeId"]),
		Name:         nonEmpty(stringField(payload, "name"), stringField(payload, "displayName")),
		Abbreviation: stringField(payload, "abbreviation"),
		StandingsRef: standingsRef,
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "name", "displayName", "abbreviation", "season", "standings",
		),
	}, nil
}

// NormalizeCalendarDays maps one section-shaped calendar payload into normalized calendar-day entries.
func NormalizeCalendarDays(data []byte) ([]CalendarDay, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	eventDate := mapField(payload, "eventDate")
	dayType := nonEmpty(stringField(eventDate, "type"), strings.TrimPrefix(lastPathSegment(ref), "/"), stringField(payload, "type"))
	startDate := stringField(payload, "startDate")
	endDate := stringField(payload, "endDate")
	sections := calendarSectionLabels(payload)
	dates := stringSliceValueFromAny(eventDate["dates"])

	if len(dates) == 0 {
		dates = append(dates, nonEmpty(startDate, endDate))
	}

	items := make([]CalendarDay, 0, len(dates))
	for _, date := range dates {
		date = strings.TrimSpace(date)
		if date == "" {
			continue
		}
		items = append(items, CalendarDay{
			Ref:       ref,
			LeagueID:  ids["leagueId"],
			Date:      date,
			DayType:   dayType,
			StartDate: startDate,
			EndDate:   endDate,
			Sections:  sections,
			Extensions: extensionsFromMap(payload,
				"$ref", "eventDate", "startDate", "endDate", "sections", "type",
			),
		})
	}

	if len(items) == 0 {
		return []CalendarDay{}, nil
	}
	return items, nil
}

// NormalizeStandingsGroup maps one standings payload into a normalized standings group.
func NormalizeStandingsGroup(data []byte) (*StandingsGroup, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}
	return NormalizeStandingsGroupFromMap(payload), nil
}

// NormalizeStandingsGroupFromMap maps a decoded standings payload into a normalized standings group.
func NormalizeStandingsGroupFromMap(payload map[string]any) *StandingsGroup {
	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	entries := normalizeStandingsRows(payload)
	standingsID := nonEmpty(stringField(payload, "id"), ids["standingsId"])

	return &StandingsGroup{
		Ref:      ref,
		ID:       standingsID,
		LeagueID: ids["leagueId"],
		SeasonID: ids["seasonId"],
		GroupID:  ids["groupId"],
		Entries:  entries,
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "name", "displayName", "standings", "entries",
		),
	}
}

func normalizeStandingsRows(payload map[string]any) []Team {
	rows := mapSliceField(payload, "standings")
	if len(rows) == 0 {
		rows = mapSliceField(payload, "entries")
	}
	if len(rows) == 0 {
		return []Team{}
	}

	out := make([]Team, 0, len(rows))
	for _, row := range rows {
		out = append(out, normalizeStandingsTeamRow(row))
	}
	return out
}

func normalizeStandingsTeamRow(row map[string]any) Team {
	teamMap := mapField(row, "team")
	teamRef := stringField(teamMap, "$ref")
	teamIDs := refIDs(teamRef)

	team := Team{
		Ref:          teamRef,
		ID:           nonEmpty(stringField(teamMap, "id"), teamIDs["teamId"], teamIDs["competitorId"]),
		Name:         nonEmpty(stringField(teamMap, "displayName"), stringField(teamMap, "name")),
		ShortName:    nonEmpty(stringField(teamMap, "shortDisplayName"), stringField(teamMap, "shortName"), stringField(teamMap, "abbreviation")),
		Abbreviation: stringField(teamMap, "abbreviation"),
		Extensions: extensionsFromMap(row,
			"team", "records", "record",
		),
	}

	records := standingsRecordRows(row)
	rank := standingsStatValue(records, "rank")
	points := standingsStatValue(records, "matchpoints", "points")
	played := standingsStatValue(records, "matchesplayed")

	parts := make([]string, 0, 3)
	if rank != "" {
		parts = append(parts, "Rank "+rank)
	}
	if points != "" {
		parts = append(parts, "Pts "+points)
	}
	if played != "" {
		parts = append(parts, "P "+played)
	}
	team.ScoreSummary = strings.Join(parts, " | ")

	if team.Extensions == nil {
		team.Extensions = map[string]any{}
	}
	if len(records) > 0 {
		team.Extensions["records"] = records
	}

	return team
}

func standingsRecordRows(row map[string]any) []map[string]any {
	records := mapSliceField(row, "records")
	if len(records) > 0 {
		return records
	}
	record := mapField(row, "record")
	if len(record) == 0 {
		return nil
	}
	return []map[string]any{record}
}

func standingsStatValue(records []map[string]any, names ...string) string {
	if len(records) == 0 || len(names) == 0 {
		return ""
	}

	targets := map[string]struct{}{}
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key != "" {
			targets[key] = struct{}{}
		}
	}

	for _, record := range records {
		for _, stat := range mapSliceField(record, "stats") {
			name := strings.ToLower(strings.TrimSpace(stringField(stat, "name")))
			typ := strings.ToLower(strings.TrimSpace(stringField(stat, "type")))
			if _, ok := targets[name]; !ok {
				if _, ok = targets[typ]; !ok {
					continue
				}
			}

			value := nonEmpty(stringField(stat, "displayValue"), stringField(stat, "value"))
			if value != "" {
				return value
			}
		}
	}

	return ""
}

func calendarSectionLabels(payload map[string]any) []string {
	sections := mapSliceField(payload, "sections")
	if len(sections) == 0 {
		return nil
	}

	out := make([]string, 0, len(sections))
	seen := map[string]struct{}{}
	for _, section := range sections {
		label := nonEmpty(stringField(section, "label"), stringField(section, "name"), stringField(section, "displayName"))
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
	}
	return out
}

func stringSliceValueFromAny(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		asString, ok := entry.(string)
		if !ok {
			continue
		}
		asString = strings.TrimSpace(asString)
		if asString == "" {
			continue
		}
		out = append(out, asString)
	}
	return out
}

func firstNonZeroValue(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func lastPathSegment(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	ids := refIDs(ref)
	if len(ids) == 0 {
		return ""
	}
	parts := strings.Split(strings.Trim(ref, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func encodePayloadMap(payload map[string]any) []byte {
	data, err := json.Marshal(payload)
	if err != nil {
		return []byte("{}")
	}
	return data
}
