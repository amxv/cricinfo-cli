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
	playType := mapField(payload, "playType")
	dismissal := mapField(payload, "dismissal")
	xCoordinate := nullableFloatField(payload, "xCoordinate")
	yCoordinate := nullableFloatField(payload, "yCoordinate")
	bbbTimestamp := int64Field(payload, "bbbTimestamp")

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
		PlayType:      playType,
		Dismissal:     dismissal,
		DismissalType: stringField(dismissal, "type"),
		DismissalText: stringField(dismissal, "text"),
		SpeedKPH:      floatField(payload, "speedKPH"),
		XCoordinate:   xCoordinate,
		YCoordinate:   yCoordinate,
		BBBTimestamp:  bbbTimestamp,
		CoordinateX:   xCoordinate,
		CoordinateY:   yCoordinate,
		Timestamp:     bbbTimestamp,
		Extensions: extensionsFromMap(payload,
			"$ref", "id", "period", "periodText", "over", "scoreValue", "shortText", "text", "homeScore", "awayScore",
			"batsman", "bowler", "playType", "dismissal", "speedKPH", "xCoordinate", "yCoordinate", "bbbTimestamp",
		),
	}

	return event, nil
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
