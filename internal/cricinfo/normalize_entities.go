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

	match := normalizeMatchFromCompetitionMap(payload, eventMatchContext{})
	return &match, nil
}

// NormalizeMatchesFromEvent maps an event payload into one or more normalized match entries.
func NormalizeMatchesFromEvent(data []byte) ([]Match, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	context := buildEventMatchContext(payload)
	competitions := mapSliceField(payload, "competitions")
	if len(competitions) == 0 {
		return []Match{}, nil
	}

	matches := make([]Match, 0, len(competitions))
	for _, competition := range competitions {
		matches = append(matches, normalizeMatchFromCompetitionMap(competition, context))
	}

	return matches, nil
}

// NormalizePlayer maps an athlete profile payload into the normalized player shape.
func NormalizePlayer(data []byte) (*Player, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	position := mapField(payload, "position")
	styles := normalizePlayerStyles(payload)
	majorTeams := normalizePlayerAffiliations(mapSliceField(payload, "majorTeams"))
	debuts := normalizePlayerDebuts(mapSliceField(payload, "debuts"))
	team := normalizePlayerAffiliation(mapField(payload, "team"))

	player := &Player{
		Ref:                  ref,
		ID:                   nonEmpty(stringField(payload, "id"), ids["athleteId"]),
		UID:                  stringField(payload, "uid"),
		GUID:                 stringField(payload, "guid"),
		Type:                 stringField(payload, "type"),
		Name:                 stringField(payload, "name"),
		FirstName:            stringField(payload, "firstName"),
		MiddleName:           stringField(payload, "middleName"),
		LastName:             stringField(payload, "lastName"),
		DisplayName:          stringField(payload, "displayName"),
		FullName:             stringField(payload, "fullName"),
		ShortName:            stringField(payload, "shortName"),
		BattingName:          stringField(payload, "battingName"),
		FieldingName:         stringField(payload, "fieldingName"),
		Gender:               stringField(payload, "gender"),
		Age:                  intField(payload, "age"),
		DateOfBirth:          stringField(payload, "dateOfBirth"),
		DateOfBirthDisplay:   nonEmpty(stringField(payload, "dateOfBirthStr"), stringField(payload, "dateOfBirth")),
		Active:               truthyField(payload, "active") || truthyField(payload, "isActive"),
		Position:             stringField(position, "name"),
		PositionRef:          stringField(position, "$ref"),
		PositionAbbreviation: stringField(position, "abbreviation"),
		Styles:               styles,
		Team:                 team,
		MajorTeams:           majorTeams,
		Debuts:               debuts,
		NewsRef:              refFromField(payload, "news"),
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "uid", "guid", "type", "name", "firstName", "middleName", "lastName",
			"displayName", "fullName", "shortName", "battingName", "fieldingName", "gender", "age",
			"dateOfBirth", "dateOfBirthStr", "active", "isActive", "team", "position", "style", "styles",
			"majorTeams", "debuts", "news",
		),
	}

	return player, nil
}

// NormalizePlayerStatistics maps an athlete statistics payload into a grouped split/category view.
func NormalizePlayerStatistics(data []byte) (*PlayerStatistics, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	athleteRef := refFromField(payload, "athlete")
	splits := mapField(payload, "splits")

	playerStats := &PlayerStatistics{
		Ref:          ref,
		PlayerID:     nonEmpty(refIDs(athleteRef)["athleteId"], ids["athleteId"]),
		PlayerRef:    athleteRef,
		SplitID:      stringField(splits, "id"),
		Name:         stringField(splits, "name"),
		Abbreviation: stringField(splits, "abbreviation"),
		Categories:   []StatCategory{},
		Extensions: extensionsFromMap(payload,
			"$ref", "athlete", "competition", "team", "splits",
		),
	}

	for _, item := range mapSliceField(splits, "categories") {
		stats := make([]StatValue, 0, len(mapSliceField(item, "stats")))
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

		playerStats.Categories = append(playerStats.Categories, StatCategory{
			Name:         stringField(item, "name"),
			DisplayName:  nonEmpty(stringField(item, "displayName"), stringField(item, "name")),
			ShortName:    stringField(item, "shortDisplayName"),
			Abbreviation: stringField(item, "abbreviation"),
			Summary:      stringField(item, "summary"),
			Stats:        stats,
			Extensions: extensionsFromMap(item,
				"name", "displayName", "shortDisplayName", "abbreviation", "summary", "stats",
			),
		})
	}

	return playerStats, nil
}

// NormalizeNewsArticle maps one Cricinfo news payload into a normalized article object.
func NormalizeNewsArticle(data []byte) (*NewsArticle, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	links := mapField(payload, "links")
	web := mapField(links, "web")
	api := mapField(mapField(links, "api"), "v1")

	article := &NewsArticle{
		Ref:          stringField(payload, "$ref"),
		ID:           nonEmpty(stringField(payload, "id"), refIDs(stringField(payload, "$ref"))["newsId"]),
		UID:          stringField(payload, "uid"),
		Type:         stringField(payload, "type"),
		Headline:     stringField(payload, "headline"),
		Title:        stringField(payload, "title"),
		LinkText:     stringField(payload, "linkText"),
		Byline:       stringField(payload, "byline"),
		Description:  stringField(payload, "description"),
		Published:    stringField(payload, "published"),
		LastModified: stringField(payload, "lastModified"),
		WebURL:       stringField(web, "href"),
		APIURL:       stringField(api, "href"),
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "uid", "type", "headline", "title", "linkText", "byline", "description",
			"published", "lastModified", "links",
		),
	}

	return article, nil
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

// NormalizeTeamRosterEntries maps match-scoped competitor roster payloads into roster entries.
func NormalizeTeamRosterEntries(data []byte, team Team, scope TeamScope, matchID string) ([]TeamRosterEntry, error) {
	entries, err := DecodeObjectCollection[map[string]any](data, "entries")
	if err != nil {
		return nil, err
	}

	normalized := make([]TeamRosterEntry, 0, len(entries))
	for _, entry := range entries {
		athlete := mapField(entry, "athlete")
		athleteRef := refFromField(entry, "athlete")
		playerID := nonEmpty(stringField(entry, "playerId"), refIDs(athleteRef)["athleteId"], refIDs(stringField(entry, "$ref"))["athleteId"])
		displayName := nonEmpty(
			stringField(athlete, "displayName"),
			stringField(athlete, "fullName"),
			stringField(athlete, "name"),
			stringField(entry, "displayName"),
			stringField(entry, "fullName"),
			stringField(entry, "name"),
		)

		normalized = append(normalized, TeamRosterEntry{
			PlayerID:      playerID,
			PlayerRef:     athleteRef,
			DisplayName:   displayName,
			TeamID:        team.ID,
			TeamName:      nonEmpty(team.ShortName, team.Name, team.ID),
			TeamRef:       team.Ref,
			MatchID:       strings.TrimSpace(matchID),
			Scope:         scope,
			Captain:       boolField(entry, "captain"),
			Starter:       boolField(entry, "starter"),
			Active:        boolField(entry, "active"),
			ActiveName:    stringField(entry, "activeName"),
			PositionRef:   refFromField(entry, "position"),
			LinescoresRef: refFromField(entry, "linescores"),
			StatisticsRef: refFromField(entry, "statistics"),
			Extensions: extensionsFromMap(entry,
				"$ref", "playerId", "athlete", "captain", "starter", "active", "activeName", "position", "linescores", "statistics",
			),
		})
	}

	return normalized, nil
}

// NormalizeTeamAthletePage maps a global team athletes page into roster-like entries for player-command bridging.
func NormalizeTeamAthletePage(data []byte, team Team) ([]TeamRosterEntry, error) {
	page, err := DecodePage[Ref](data)
	if err != nil {
		return nil, err
	}

	entries := make([]TeamRosterEntry, 0, len(page.Items))
	for _, item := range page.Items {
		playerRef := strings.TrimSpace(item.URL)
		if playerRef == "" {
			continue
		}

		entries = append(entries, TeamRosterEntry{
			PlayerID:  refIDs(playerRef)["athleteId"],
			PlayerRef: playerRef,
			TeamID:    team.ID,
			TeamName:  nonEmpty(team.ShortName, team.Name, team.ID),
			TeamRef:   team.Ref,
			Scope:     TeamScopeGlobal,
		})
	}

	return entries, nil
}

// NormalizeTeamScore maps a score payload into a stable team score object.
func NormalizeTeamScore(data []byte, team Team, scope TeamScope, matchID string) (*TeamScore, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	score := &TeamScore{
		Ref:          nonEmpty(stringField(payload, "$ref"), team.ScoreRef),
		TeamID:       team.ID,
		MatchID:      strings.TrimSpace(matchID),
		Scope:        scope,
		DisplayValue: stringField(payload, "displayValue"),
		Value:        stringField(payload, "value"),
		Place:        stringField(payload, "place"),
		Source:       stringField(payload, "source"),
		Winner:       boolField(payload, "winner"),
		Extensions: extensionsFromMap(payload,
			"$ref", "displayValue", "value", "place", "source", "winner",
		),
	}

	return score, nil
}

// NormalizeTeamLeaders maps category-based team leaders payloads into batting/bowling-friendly structures.
func NormalizeTeamLeaders(data []byte, team Team, scope TeamScope, matchID string) (*TeamLeaders, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	leaders := &TeamLeaders{
		Ref:        stringField(payload, "$ref"),
		TeamID:     team.ID,
		TeamName:   nonEmpty(team.ShortName, team.Name),
		MatchID:    strings.TrimSpace(matchID),
		Scope:      scope,
		Name:       nonEmpty(stringField(payload, "displayName"), stringField(payload, "name"), nonEmpty(team.ShortName, team.Name), "Leaders"),
		Categories: []TeamLeaderCategory{},
		Extensions: extensionsFromMap(payload, "$ref", "id", "name", "displayName", "abbreviation", "categories"),
	}

	for _, rawCategory := range mapSliceField(payload, "categories") {
		category := TeamLeaderCategory{
			Name:         stringField(rawCategory, "name"),
			DisplayName:  nonEmpty(stringField(rawCategory, "displayName"), stringField(rawCategory, "name")),
			ShortName:    stringField(rawCategory, "shortDisplayName"),
			Abbreviation: stringField(rawCategory, "abbreviation"),
			Leaders:      []TeamLeaderEntry{},
			Extensions: extensionsFromMap(rawCategory,
				"name", "displayName", "shortDisplayName", "abbreviation", "leaders",
			),
		}

		for _, rawLeader := range mapSliceField(rawCategory, "leaders") {
			athlete := mapField(rawLeader, "athlete")
			athleteRef := refFromField(rawLeader, "athlete")
			athleteID := nonEmpty(
				stringField(rawLeader, "athleteId"),
				stringField(athlete, "id"),
				refIDs(athleteRef)["athleteId"],
			)
			athleteName := nonEmpty(
				stringField(athlete, "displayName"),
				stringField(athlete, "fullName"),
				stringField(athlete, "name"),
				stringField(rawLeader, "athleteDisplayName"),
				stringField(rawLeader, "name"),
			)
			entry := TeamLeaderEntry{
				Order:         intField(rawLeader, "order"),
				DisplayValue:  stringField(rawLeader, "displayValue"),
				Value:         stringField(rawLeader, "value"),
				AthleteID:     athleteID,
				AthleteName:   athleteName,
				AthleteRef:    athleteRef,
				TeamRef:       refFromField(rawLeader, "team"),
				StatisticsRef: refFromField(rawLeader, "statistics"),
				Runs:          stringField(rawLeader, "runs"),
				Wickets:       stringField(rawLeader, "wickets"),
				Overs:         stringField(rawLeader, "overs"),
				Maidens:       stringField(rawLeader, "maidens"),
				EconomyRate:   stringField(rawLeader, "economyRate"),
				Balls:         stringField(rawLeader, "balls"),
				Fours:         stringField(rawLeader, "fours"),
				Sixes:         stringField(rawLeader, "sixes"),
				Extensions: extensionsFromMap(rawLeader,
					"order", "displayValue", "value", "athlete", "team", "statistics", "runs", "wickets", "overs", "maidens", "economyRate", "balls", "fours", "sixes",
				),
			}
			category.Leaders = append(category.Leaders, entry)
		}

		leaders.Categories = append(leaders.Categories, category)
	}

	return leaders, nil
}

// NormalizeTeamRecordCategories maps team records pages into stat-category-like entries.
func NormalizeTeamRecordCategories(data []byte) ([]StatCategory, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	items := mapSliceField(payload, "items")
	if len(items) == 0 {
		return []StatCategory{}, nil
	}

	categories := make([]StatCategory, 0, len(items))
	for _, item := range items {
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
			DisplayName:  nonEmpty(stringField(item, "displayName"), stringField(item, "name")),
			ShortName:    stringField(item, "shortDisplayName"),
			Abbreviation: stringField(item, "abbreviation"),
			Summary:      stringField(item, "summary"),
			Stats:        stats,
			Extensions: extensionsFromMap(item,
				"$ref", "id", "name", "displayName", "shortDisplayName", "abbreviation", "summary", "stats",
			),
		})
	}

	return categories, nil
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
	return normalizeInningsFromMap(payload), nil
}

func normalizeInningsFromMap(payload map[string]any) *Innings {
	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	inningsNumber := parseInt(ids["inningsId"])
	period := intField(payload, "period")
	if period == 0 {
		period = parseInt(ids["periodId"])
	}

	innings := &Innings{
		Ref:             ref,
		ID:              ids["inningsId"],
		LeagueID:        ids["leagueId"],
		EventID:         ids["eventId"],
		CompetitionID:   ids["competitionId"],
		MatchID:         ids["competitionId"],
		TeamID:          ids["competitorId"],
		InningsNumber:   inningsNumber,
		Period:          period,
		Runs:            intField(payload, "runs"),
		Wickets:         intField(payload, "wickets"),
		Overs:           floatField(payload, "overs"),
		Score:           stringField(payload, "score"),
		Description:     stringField(payload, "description"),
		Target:          intField(payload, "target"),
		IsBatting:       truthyField(payload, "isBatting"),
		IsCurrent:       truthyField(payload, "isCurrent"),
		Fours:           intField(payload, "fours"),
		Sixes:           intField(payload, "sixes"),
		StatisticsRef:   refFromField(payload, "statistics"),
		LeadersRef:      refFromField(payload, "leaders"),
		PartnershipsRef: refFromField(payload, "partnerships"),
		FallOfWicketRef: refFromField(payload, "fow"),
		Extensions: extensionsFromMap(payload,
			"$ref", "period", "runs", "wickets", "overs", "score", "description", "target",
			"isBatting", "isCurrent", "fours", "sixes", "value", "displayValue", "source", "followOn",
			"statistics", "leaders", "partnerships", "fow",
		),
	}

	return innings
}

// NormalizeInningsPeriodStatistics maps period statistics payloads into over and wicket timelines.
func NormalizeInningsPeriodStatistics(data []byte) ([]InningsOver, []InningsWicket, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, nil, err
	}

	splits := mapField(payload, "splits")
	if splits == nil {
		return []InningsOver{}, []InningsWicket{}, nil
	}

	overs := normalizeOverTimeline(splits)
	wickets := make([]InningsWicket, 0)
	for _, over := range overs {
		wickets = append(wickets, over.Wickets...)
	}

	return overs, wickets, nil
}

func normalizeOverTimeline(splits map[string]any) []InningsOver {
	if splits == nil {
		return []InningsOver{}
	}

	rawOvers, ok := splits["overs"]
	if !ok || rawOvers == nil {
		return []InningsOver{}
	}

	rows := make([]map[string]any, 0)
	switch typed := rawOvers.(type) {
	case []any:
		for _, entry := range typed {
			switch rowOrSlice := entry.(type) {
			case map[string]any:
				rows = append(rows, rowOrSlice)
			case []any:
				for _, nested := range rowOrSlice {
					row, ok := nested.(map[string]any)
					if !ok {
						continue
					}
					rows = append(rows, row)
				}
			}
		}
	}

	overs := make([]InningsOver, 0, len(rows))
	for _, row := range rows {
		wickets := normalizeTimelineWickets(row)
		overs = append(overs, InningsOver{
			Number:      intField(row, "number"),
			Runs:        intField(row, "runs"),
			WicketCount: len(wickets),
			Wickets:     wickets,
			Extensions:  extensionsFromMap(row, "number", "runs", "wicket"),
		})
	}

	return overs
}

func normalizeTimelineWickets(overRow map[string]any) []InningsWicket {
	if overRow == nil {
		return []InningsWicket{}
	}
	rawWickets, ok := overRow["wicket"]
	if !ok || rawWickets == nil {
		return []InningsWicket{}
	}

	entries, ok := rawWickets.([]any)
	if !ok {
		return []InningsWicket{}
	}

	wickets := make([]InningsWicket, 0, len(entries))
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		details := mapField(entry, "details")
		wickets = append(wickets, InningsWicket{
			Number:          intField(entry, "number"),
			FOW:             stringField(entry, "fow"),
			Over:            stringField(entry, "over"),
			FOWType:         stringField(entry, "fowType"),
			Runs:            intField(entry, "runs"),
			BallsFaced:      intField(entry, "ballsFaced"),
			StrikeRate:      floatField(entry, "strikeRate"),
			DismissalCard:   stringField(entry, "dismissalCard"),
			ShortText:       stringField(entry, "shortText"),
			DetailRef:       stringField(details, "$ref"),
			DetailShortText: stringField(details, "shortText"),
			DetailText:      stringField(details, "text"),
			Extensions: extensionsFromMap(entry,
				"number", "fow", "over", "fowType", "runs", "ballsFaced", "dismissalCard", "shortText", "details",
			),
		})
	}

	return wickets
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
	playType := mapField(payload, "playType")
	dismissal := mapField(payload, "dismissal")
	batsman := mapField(payload, "batsman")
	bowler := mapField(payload, "bowler")
	fielder := mapField(dismissal, "fielder")
	batsmanRef := nonEmpty(nestedRef(payload, "batsman", "athlete"), refFromField(payload, "batsman"))
	bowlerRef := nonEmpty(nestedRef(payload, "bowler", "athlete"), refFromField(payload, "bowler"))
	fielderRef := nonEmpty(nestedRef(payload, "dismissal", "fielder", "athlete"), nestedRef(payload, "dismissal", "fielder"))
	batsmanID := nonEmpty(stringField(batsman, "playerId"), stringField(batsman, "id"), refIDs(batsmanRef)["athleteId"])
	bowlerID := nonEmpty(stringField(bowler, "playerId"), stringField(bowler, "id"), refIDs(bowlerRef)["athleteId"])
	fielderID := nonEmpty(stringField(fielder, "playerId"), stringField(fielder, "id"), refIDs(fielderRef)["athleteId"])
	teamRef := refFromField(payload, "team")
	teamIDs := refIDs(teamRef)
	athletePlayerIDs := extractAthletePlayerIDs(payload)
	xCoordinate := nullableFloatField(payload, "xCoordinate")
	yCoordinate := nullableFloatField(payload, "yCoordinate")
	bbbTimestamp := int64Field(payload, "bbbTimestamp")

	event := &DeliveryEvent{
		Ref:              ref,
		ID:               nonEmpty(stringField(payload, "id"), ids["detailId"]),
		LeagueID:         ids["leagueId"],
		EventID:          ids["eventId"],
		CompetitionID:    ids["competitionId"],
		MatchID:          ids["competitionId"],
		TeamID:           nonEmpty(teamIDs["teamId"], teamIDs["competitorId"]),
		Period:           intField(payload, "period"),
		PeriodText:       stringField(payload, "periodText"),
		OverNumber:       intField(over, "number"),
		BallNumber:       intField(over, "ball"),
		ScoreValue:       intField(payload, "scoreValue"),
		ShortText:        stringField(payload, "shortText"),
		Text:             stringField(payload, "text"),
		HomeScore:        stringField(payload, "homeScore"),
		AwayScore:        stringField(payload, "awayScore"),
		BatsmanRef:       batsmanRef,
		BowlerRef:        bowlerRef,
		BatsmanPlayerID:  batsmanID,
		BowlerPlayerID:   bowlerID,
		FielderPlayerID:  fielderID,
		AthletePlayerIDs: athletePlayerIDs,
		PlayType:         playType,
		Dismissal:        dismissal,
		DismissalType:    stringField(dismissal, "type"),
		DismissalName:    nonEmpty(stringField(dismissal, "name"), stringField(dismissal, "type")),
		DismissalCard:    stringField(dismissal, "dismissalCard"),
		DismissalText:    stringField(dismissal, "text"),
		SpeedKPH:         floatField(payload, "speedKPH"),
		XCoordinate:      xCoordinate,
		YCoordinate:      yCoordinate,
		BBBTimestamp:     bbbTimestamp,
		CoordinateX:      xCoordinate,
		CoordinateY:      yCoordinate,
		Timestamp:        bbbTimestamp,
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "period", "periodText", "over", "scoreValue", "shortText", "text", "homeScore", "awayScore",
			"batsman", "bowler", "playType", "dismissal", "speedKPH", "xCoordinate", "yCoordinate", "bbbTimestamp",
		),
	}

	return event, nil
}

func extractAthletePlayerIDs(payload map[string]any) []string {
	if payload == nil {
		return nil
	}
	rawItems, ok := payload["athletesInvolved"].([]any)
	if !ok || len(rawItems) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(rawItems))
	for _, raw := range rawItems {
		ref := refValue(raw)
		if ref == "" {
			if mapped, ok := raw.(map[string]any); ok {
				ref = nonEmpty(refFromField(mapped, "athlete"), nestedRef(mapped, "athlete"))
			}
		}
		playerID := strings.TrimSpace(refIDs(ref)["athleteId"])
		if playerID == "" {
			continue
		}
		if _, ok := seen[playerID]; ok {
			continue
		}
		seen[playerID] = struct{}{}
		out = append(out, playerID)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// NormalizeMatchScorecard maps a matchcards payload into batting, bowling, and partnerships views.
func NormalizeMatchScorecard(data []byte, match Match) (*MatchScorecard, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	scorecard := &MatchScorecard{
		Ref:              nonEmpty(stringField(payload, "$ref"), matchSubresourceRef(match, "matchcards", "matchcards")),
		LeagueID:         match.LeagueID,
		EventID:          match.EventID,
		CompetitionID:    match.CompetitionID,
		MatchID:          match.ID,
		BattingCards:     []BattingCard{},
		BowlingCards:     []BowlingCard{},
		PartnershipCards: []PartnershipCard{},
		Extensions: extensionsFromMap(payload,
			"$ref", "count", "items", "pageCount", "pageIndex", "pageSize",
		),
	}

	for _, item := range mapSliceField(payload, "items") {
		headline := strings.ToLower(strings.TrimSpace(stringField(item, "headline")))
		typeID := strings.TrimSpace(stringField(item, "typeID"))

		switch {
		case headline == "batting" || typeID == "11":
			scorecard.BattingCards = append(scorecard.BattingCards, normalizeBattingCard(item))
		case headline == "bowling" || typeID == "12":
			scorecard.BowlingCards = append(scorecard.BowlingCards, normalizeBowlingCard(item))
		case headline == "partnerships" || typeID == "13":
			scorecard.PartnershipCards = append(scorecard.PartnershipCards, normalizePartnershipCard(item))
		default:
			// Preserve unclassified card payloads for --all-fields without failing command execution.
			if scorecard.Extensions == nil {
				scorecard.Extensions = map[string]any{}
			}
			unknown, _ := scorecard.Extensions["unknownCards"].([]any)
			scorecard.Extensions["unknownCards"] = append(unknown, item)
		}
	}

	return scorecard, nil
}

// NormalizeMatchSituation maps a situation payload into a stable shape that tolerates sparse data.
func NormalizeMatchSituation(data []byte, match Match) (*MatchSituation, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	situation := &MatchSituation{
		Ref:           nonEmpty(stringField(payload, "$ref"), matchSubresourceRef(match, "situation", "situation")),
		LeagueID:      match.LeagueID,
		EventID:       match.EventID,
		CompetitionID: match.CompetitionID,
		MatchID:       match.ID,
		OddsRef:       refFromField(payload, "odds"),
		Data:          extensionsFromMap(payload, "$ref", "odds"),
	}

	return situation, nil
}

func normalizeBattingCard(payload map[string]any) BattingCard {
	card := BattingCard{
		InningsNumber: parseInt(stringField(payload, "inningsNumber")),
		TeamName:      stringField(payload, "teamName"),
		Runs:          stringField(payload, "runs"),
		Total:         stringField(payload, "total"),
		Extras:        stringField(payload, "extras"),
		Players:       []BattingCardEntry{},
	}

	for _, row := range mapSliceField(payload, "playerDetails") {
		card.Players = append(card.Players, BattingCardEntry{
			PlayerID:   stringField(row, "playerID"),
			PlayerName: stringField(row, "playerName"),
			Dismissal:  stringField(row, "dismissal"),
			Runs:       stringField(row, "runs"),
			BallsFaced: stringField(row, "ballsFaced"),
			Fours:      stringField(row, "fours"),
			Sixes:      stringField(row, "sixes"),
			Href:       stringField(row, "href"),
		})
	}

	return card
}

func normalizeBowlingCard(payload map[string]any) BowlingCard {
	card := BowlingCard{
		InningsNumber: parseInt(stringField(payload, "inningsNumber")),
		TeamName:      stringField(payload, "teamName"),
		Players:       []BowlingCardEntry{},
	}

	for _, row := range mapSliceField(payload, "playerDetails") {
		card.Players = append(card.Players, BowlingCardEntry{
			PlayerID:    stringField(row, "playerID"),
			PlayerName:  stringField(row, "playerName"),
			Overs:       stringField(row, "overs"),
			Maidens:     stringField(row, "maidens"),
			Conceded:    stringField(row, "conceded"),
			Wickets:     stringField(row, "wickets"),
			EconomyRate: stringField(row, "economyRate"),
			NBW:         stringField(row, "nbw"),
			Href:        stringField(row, "href"),
		})
	}

	return card
}

func normalizePartnershipCard(payload map[string]any) PartnershipCard {
	card := PartnershipCard{
		InningsNumber: parseInt(stringField(payload, "inningsNumber")),
		TeamName:      stringField(payload, "teamName"),
		Players:       []PartnershipCardEntry{},
	}

	for _, row := range mapSliceField(payload, "playerDetails") {
		card.Players = append(card.Players, PartnershipCardEntry{
			PartnershipRuns:       stringField(row, "partnershipRuns"),
			PartnershipOvers:      stringField(row, "partnershipOvers"),
			PartnershipWicketName: stringField(row, "partnershipWicketName"),
			FOWType:               stringField(row, "fowType"),
			Player1Name:           stringField(row, "player1Name"),
			Player1Runs:           stringField(row, "player1Runs"),
			Player2Name:           stringField(row, "player2Name"),
			Player2Runs:           stringField(row, "player2Runs"),
		})
	}

	return card
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
			Ref:          ref,
			ID:           ids["partnershipId"],
			TeamID:       ids["competitorId"],
			MatchID:      ids["competitionId"],
			InningsID:    ids["inningsId"],
			Period:       ids["periodId"],
			Order:        parseInt(ids["partnershipId"]),
			WicketNumber: parseInt(ids["partnershipId"]),
			Extensions: extensionsFromMap(item,
				"$ref",
			),
		})
	}

	return partnerships, nil
}

// NormalizePartnership maps a single partnership payload into a detailed normalized object.
func NormalizePartnership(data []byte) (*Partnership, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	start := mapField(payload, "start")
	end := mapField(payload, "end")

	batsmen := make([]PartnershipBatsman, 0)
	for _, batsman := range mapSliceField(payload, "batsmen") {
		athleteRef := strings.TrimSpace(stringField(batsman, "athlete"))
		if athleteRef == "" {
			athleteRef = refFromField(batsman, "athlete")
		}
		batsmen = append(batsmen, PartnershipBatsman{
			AthleteRef: athleteRef,
			Balls:      intField(batsman, "balls"),
			Runs:       intField(batsman, "runs"),
		})
	}

	partnership := &Partnership{
		Ref:          ref,
		ID:           ids["partnershipId"],
		TeamID:       ids["competitorId"],
		MatchID:      ids["competitionId"],
		InningsID:    ids["inningsId"],
		Period:       ids["periodId"],
		Order:        parseInt(ids["partnershipId"]),
		WicketNumber: intField(payload, "wicketNumber"),
		WicketName:   stringField(payload, "wicketName"),
		FOWType:      stringField(payload, "fowType"),
		Overs:        floatField(payload, "overs"),
		Runs:         intField(payload, "runs"),
		RunRate:      floatField(payload, "runRate"),
		Start: PartnershipSnapshot{
			Overs:   floatField(start, "overs"),
			Runs:    intField(start, "runs"),
			Wickets: intField(start, "wickets"),
		},
		End: PartnershipSnapshot{
			Overs:   floatField(end, "overs"),
			Runs:    intField(end, "runs"),
			Wickets: intField(end, "wickets"),
		},
		Batsmen: batsmen,
		Extensions: extensionsFromMap(payload,
			"$ref", "wicketNumber", "wicketName", "fowType", "overs", "runs", "runRate", "start", "end", "batsmen",
		),
	}

	if partnership.WicketNumber == 0 {
		partnership.WicketNumber = parseInt(ids["partnershipId"])
	}
	if partnership.Runs == 0 && partnership.End.Runs > partnership.Start.Runs {
		partnership.Runs = partnership.End.Runs - partnership.Start.Runs
	}
	if partnership.Overs == 0 && partnership.End.Overs > partnership.Start.Overs {
		partnership.Overs = partnership.End.Overs - partnership.Start.Overs
	}

	return partnership, nil
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
			MatchID:      ids["competitionId"],
			TeamID:       ids["competitorId"],
			InningsID:    ids["inningsId"],
			Period:       ids["periodId"],
			WicketNumber: parseInt(ids["fowId"]),
			Extensions: extensionsFromMap(item,
				"$ref",
			),
		})
	}

	return wickets, nil
}

// NormalizeFallOfWicket maps a single fall-of-wicket payload into a detailed normalized object.
func NormalizeFallOfWicket(data []byte) (*FallOfWicket, error) {
	payload, err := decodePayloadMap(data)
	if err != nil {
		return nil, err
	}

	ref := stringField(payload, "$ref")
	ids := refIDs(ref)

	fow := &FallOfWicket{
		Ref:          ref,
		ID:           ids["fowId"],
		MatchID:      ids["competitionId"],
		TeamID:       ids["competitorId"],
		InningsID:    ids["inningsId"],
		Period:       ids["periodId"],
		WicketNumber: intField(payload, "wicketNumber"),
		WicketOver:   floatField(payload, "wicketOver"),
		FOWType:      stringField(payload, "fowType"),
		Runs:         intField(payload, "runs"),
		RunsScored:   intField(payload, "runsScored"),
		BallsFaced:   intField(payload, "ballsFaced"),
		AthleteRef:   refFromField(payload, "athlete"),
		Extensions: extensionsFromMap(payload,
			"$ref", "wicketNumber", "wicketOver", "fowType", "runs", "runsScored", "ballsFaced", "athlete",
		),
	}

	if fow.WicketNumber == 0 {
		fow.WicketNumber = parseInt(ids["fowId"])
	}
	if fow.Runs == 0 && fow.RunsScored > 0 {
		fow.Runs = fow.RunsScored
	}
	if fow.RunsScored == 0 && fow.Runs > 0 {
		fow.RunsScored = fow.Runs
	}

	return fow, nil
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
	teamRef := refFromField(payload, "team")
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
	score := mapField(payload, "score")
	scoreSummary := nonEmpty(stringField(score, "displayValue"), stringField(score, "value"))

	return Team{
		Ref:           nonEmpty(teamRef, ref),
		ID:            id,
		UID:           stringField(payload, "uid"),
		Name:          name,
		ShortName:     shortName,
		Abbreviation:  stringField(payload, "abbreviation"),
		ScoreSummary:  scoreSummary,
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

type eventMatchContext struct {
	ref              string
	leagueID         string
	eventID          string
	date             string
	endDate          string
	description      string
	shortDescription string
	venueName        string
	venueSummary     string
}

func buildEventMatchContext(payload map[string]any) eventMatchContext {
	ref := stringField(payload, "$ref")
	ids := refIDs(ref)
	leagueID := ids["leagueId"]

	if leagueID == "" {
		for _, league := range mapSliceField(payload, "leagues") {
			leagueID = nonEmpty(
				stringField(league, "id"),
				refIDs(stringField(league, "$ref"))["leagueId"],
			)
			if leagueID != "" {
				break
			}
		}
	}

	venueName, venueSummary := eventVenueSummary(payload)
	return eventMatchContext{
		ref:              ref,
		leagueID:         leagueID,
		eventID:          nonEmpty(stringField(payload, "id"), ids["eventId"]),
		date:             stringField(payload, "date"),
		endDate:          stringField(payload, "endDate"),
		description:      nonEmpty(stringField(payload, "description"), stringField(payload, "name")),
		shortDescription: nonEmpty(stringField(payload, "shortDescription"), stringField(payload, "shortName")),
		venueName:        venueName,
		venueSummary:     venueSummary,
	}
}

func normalizeMatchFromCompetitionMap(payload map[string]any, context eventMatchContext) Match {
	ref := stringField(payload, "$ref")
	ids := refIDs(ref)

	venue := mapField(payload, "venue")
	venueName := nonEmpty(stringField(venue, "fullName"), context.venueName)
	venueSummary := nonEmpty(venueAddressSummary(venue), context.venueSummary)

	teams := make([]Team, 0)
	for _, item := range mapSliceField(payload, "competitors") {
		teams = append(teams, normalizeTeamMap(item))
	}

	scoreSummary := matchScoreSummary(teams)
	matchState := nonEmpty(
		stringField(payload, "state"),
		stringField(payload, "summary"),
		stringField(payload, "statusSummary"),
		stringField(mapField(payload, "status"), "summary"),
		stringField(mapField(mapField(payload, "status"), "type"), "detail"),
		stringField(mapField(mapField(payload, "status"), "type"), "description"),
	)

	return Match{
		Ref:              nonEmpty(ref, context.ref),
		ID:               nonEmpty(stringField(payload, "id"), ids["competitionId"]),
		UID:              stringField(payload, "uid"),
		LeagueID:         nonEmpty(ids["leagueId"], context.leagueID),
		EventID:          nonEmpty(ids["eventId"], context.eventID),
		CompetitionID:    nonEmpty(ids["competitionId"], stringField(payload, "id")),
		Description:      nonEmpty(stringField(payload, "description"), context.description),
		ShortDescription: nonEmpty(stringField(payload, "shortDescription"), context.shortDescription),
		Note:             stringField(payload, "note"),
		MatchState:       matchState,
		Date:             nonEmpty(stringField(payload, "date"), context.date),
		EndDate:          nonEmpty(stringField(payload, "endDate"), context.endDate),
		VenueName:        venueName,
		VenueSummary:     venueSummary,
		ScoreSummary:     scoreSummary,
		StatusRef:        refFromField(payload, "status"),
		DetailsRef:       refFromField(payload, "details"),
		Teams:            teams,
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "uid", "description", "shortDescription", "note", "state", "summary", "statusSummary",
			"date", "endDate", "status", "details", "competitors",
		),
	}
}

func eventVenueSummary(payload map[string]any) (string, string) {
	venues := mapSliceField(payload, "venues")
	if len(venues) == 0 {
		return "", ""
	}
	return nonEmpty(
		stringField(venues[0], "fullName"),
		stringField(venues[0], "shortName"),
	), venueAddressSummary(venues[0])
}

func venueAddressSummary(venue map[string]any) string {
	if venue == nil {
		return ""
	}
	address := mapField(venue, "address")
	if address == nil {
		return ""
	}
	return nonEmpty(
		stringField(address, "summary"),
		strings.Join(compactValues(
			stringField(address, "city"),
			stringField(address, "state"),
			stringField(address, "country"),
		), ", "),
	)
}

func matchScoreSummary(teams []Team) string {
	parts := make([]string, 0, len(teams))
	for _, team := range teams {
		if team.ScoreSummary == "" {
			continue
		}
		label := nonEmpty(team.ShortName, team.Name, team.ID)
		if label == "" {
			parts = append(parts, team.ScoreSummary)
			continue
		}
		parts = append(parts, label+" "+team.ScoreSummary)
	}
	return strings.Join(parts, " | ")
}

func compactValues(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
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
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		if typed == float32(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return strings.TrimSpace(typed.String())
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

func truthyField(payload map[string]any, key string) bool {
	if boolField(payload, key) {
		return true
	}
	if payload == nil {
		return false
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case float64:
		return typed != 0
	case float32:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return false
		}
		return parsed != 0
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return false
		}
		if parsed, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return parsed != 0
		}
		parsed, err := strconv.ParseBool(trimmed)
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
	if payload == nil {
		return ""
	}
	return refValue(payload[key])
}

func nestedRef(payload map[string]any, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}

	var current any = payload
	for idx, key := range keys {
		mapped, ok := current.(map[string]any)
		if !ok || mapped == nil {
			return ""
		}
		next, ok := mapped[key]
		if !ok || next == nil {
			return ""
		}
		if idx == len(keys)-1 {
			return refValue(next)
		}
		current = next
	}
	return ""
}

func refValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		return strings.TrimSpace(stringField(typed, "$ref"))
	default:
		return ""
	}
}

func normalizePlayerStyles(payload map[string]any) []PlayerStyle {
	rawStyles := append(mapSliceField(payload, "style"), mapSliceField(payload, "styles")...)
	if len(rawStyles) == 0 {
		return nil
	}

	out := make([]PlayerStyle, 0, len(rawStyles))
	seen := map[string]struct{}{}
	for _, raw := range rawStyles {
		style := PlayerStyle{
			Type:             stringField(raw, "type"),
			Description:      stringField(raw, "description"),
			ShortDescription: stringField(raw, "shortDescription"),
		}
		key := strings.Join([]string{style.Type, style.Description, style.ShortDescription}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, style)
	}
	return out
}

func normalizePlayerAffiliations(items []map[string]any) []PlayerAffiliation {
	if len(items) == 0 {
		return nil
	}
	out := make([]PlayerAffiliation, 0, len(items))
	for _, item := range items {
		if affiliation := normalizePlayerAffiliation(item); affiliation != nil {
			out = append(out, *affiliation)
		}
	}
	return out
}

func normalizePlayerAffiliation(item map[string]any) *PlayerAffiliation {
	if len(item) == 0 {
		return nil
	}
	ref := stringField(item, "$ref")
	ids := refIDs(ref)
	return &PlayerAffiliation{
		ID:   nonEmpty(stringField(item, "id"), ids["teamId"]),
		Ref:  ref,
		Name: nonEmpty(stringField(item, "displayName"), stringField(item, "name"), stringField(item, "shortName")),
	}
}

func normalizePlayerDebuts(items []map[string]any) []PlayerDebut {
	if len(items) == 0 {
		return nil
	}
	out := make([]PlayerDebut, 0, len(items))
	for _, item := range items {
		ref := stringField(item, "$ref")
		ids := refIDs(ref)
		out = append(out, PlayerDebut{
			ID:   nonEmpty(stringField(item, "id"), ids["competitionId"], ids["eventId"]),
			Ref:  ref,
			Name: nonEmpty(stringField(item, "displayName"), stringField(item, "name"), stringField(item, "shortName")),
		})
	}
	return out
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
