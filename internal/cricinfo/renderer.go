package cricinfo

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// RenderOptions controls output behavior for the rendering boundary.
type RenderOptions struct {
	Format    string
	Verbose   bool
	AllFields bool
}

// Renderer defines the rendering boundary all commands should use.
type Renderer interface {
	Render(w io.Writer, result NormalizedResult, opts RenderOptions) error
}

type defaultRenderer struct{}

// NewRenderer returns the default rendering implementation.
func NewRenderer() Renderer {
	return &defaultRenderer{}
}

// Render writes a normalized result using the requested output format.
func Render(w io.Writer, result NormalizedResult, opts RenderOptions) error {
	return NewRenderer().Render(w, result, opts)
}

func (r *defaultRenderer) Render(w io.Writer, result NormalizedResult, opts RenderOptions) error {
	format := strings.ToLower(strings.TrimSpace(opts.Format))
	if format == "" {
		format = "text"
	}

	switch format {
	case "text":
		return renderText(w, result, opts)
	case "json":
		return renderJSON(w, result, opts)
	case "jsonl":
		return renderJSONL(w, result, opts)
	default:
		return fmt.Errorf("unsupported render format %q", opts.Format)
	}
}

func renderJSON(w io.Writer, result NormalizedResult, opts RenderOptions) error {
	sanitized, err := sanitizeValue(result, opts.AllFields)
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json output: %w", err)
	}

	if _, err := fmt.Fprintln(w, string(encoded)); err != nil {
		return fmt.Errorf("write json output: %w", err)
	}

	return nil
}

func renderJSONL(w io.Writer, result NormalizedResult, opts RenderOptions) error {
	if len(result.Items) == 0 {
		switch result.Status {
		case ResultStatusEmpty:
			return nil
		case ResultStatusError:
			meta := map[string]any{
				"_meta": map[string]any{
					"kind":   result.Kind,
					"status": result.Status,
				},
			}
			if result.Message != "" {
				meta["_meta"].(map[string]any)["message"] = result.Message
			}
			if result.Error != nil {
				meta["_meta"].(map[string]any)["error"] = result.Error
			}
			return writeJSONLLine(w, meta, opts.AllFields)
		default:
			if result.Data != nil {
				return fmt.Errorf("jsonl format requires list results")
			}
			return nil
		}
	}

	if result.Data != nil {
		return fmt.Errorf("jsonl format requires list results")
	}

	if result.Status == ResultStatusPartial || len(result.Warnings) > 0 || result.Message != "" {
		meta := map[string]any{
			"_meta": map[string]any{
				"kind":   result.Kind,
				"status": result.Status,
			},
		}
		if len(result.Warnings) > 0 {
			meta["_meta"].(map[string]any)["warnings"] = result.Warnings
		}
		if result.Message != "" {
			meta["_meta"].(map[string]any)["message"] = result.Message
		}
		if err := writeJSONLLine(w, meta, opts.AllFields); err != nil {
			return err
		}
	}

	for _, item := range result.Items {
		if err := writeJSONLLine(w, item, opts.AllFields); err != nil {
			return err
		}
	}

	return nil
}

func writeJSONLLine(w io.Writer, value any, allFields bool) error {
	sanitized, err := sanitizeValue(value, allFields)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return fmt.Errorf("encode jsonl line: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(encoded)); err != nil {
		return fmt.Errorf("write jsonl line: %w", err)
	}
	return nil
}

func renderText(w io.Writer, result NormalizedResult, opts RenderOptions) error {
	lines := make([]string, 0, 16)
	kindTitle := titleize(string(result.Kind))

	switch result.Status {
	case ResultStatusError:
		message := result.Message
		if message == "" {
			message = "transport error"
		}
		lines = append(lines, fmt.Sprintf("Error (%s): %s", kindTitle, message))
		if result.Error != nil {
			if result.Error.URL != "" {
				lines = append(lines, "URL: "+result.Error.URL)
			}
			if result.Error.StatusCode > 0 {
				lines = append(lines, fmt.Sprintf("Status: %d", result.Error.StatusCode))
			}
		}
		if result.RequestedRef != "" {
			lines = append(lines, "Requested: "+result.RequestedRef)
		}
		return writeTextLines(w, lines)
	case ResultStatusPartial:
		warningLine := "Partial data returned"
		if len(result.Warnings) > 0 {
			warningLine = warningLine + ": " + strings.Join(result.Warnings, "; ")
		}
		lines = append(lines, warningLine)
	}

	if result.Data != nil {
		if result.Kind == EntityMatchScorecard {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Match Scorecard")
			lines = append(lines, formatMatchScorecard(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityTeamLeaders {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Team Leaders")
			lines = append(lines, formatTeamLeaders(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityInnings {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Innings")
			lines = append(lines, formatInningsTimelines(itemMap)...)
			return writeTextLines(w, lines)
		}

		itemMap, err := toMap(result.Data, opts.AllFields)
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%s", kindTitle))
		lines = append(lines, formatSingleEntity(itemMap, result.Kind, opts)...)
		return writeTextLines(w, lines)
	}

	if len(result.Items) == 0 {
		message := result.Message
		if message == "" {
			message = fmt.Sprintf("No %s found.", kindPlural(result.Kind))
		}
		lines = append(lines, sentenceCase(message))
		return writeTextLines(w, lines)
	}

	lines = append(lines, fmt.Sprintf("%s (%d)", titleize(kindPlural(result.Kind)), len(result.Items)))
	for i, item := range result.Items {
		itemMap, err := toMap(item, opts.AllFields)
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, summarizeEntity(itemMap, result.Kind, opts.Verbose)))
	}

	return writeTextLines(w, lines)
}

func writeTextLines(w io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("write text output: %w", err)
		}
	}
	return nil
}

func sanitizeValue(value any, allFields bool) (any, error) {
	blob, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal value: %w", err)
	}

	var out any
	if err := json.Unmarshal(blob, &out); err != nil {
		return nil, fmt.Errorf("unmarshal value: %w", err)
	}

	if !allFields {
		removeExtensions(out)
	}

	return out, nil
}

func removeExtensions(value any) {
	switch typed := value.(type) {
	case map[string]any:
		delete(typed, "extensions")
		for _, child := range typed {
			removeExtensions(child)
		}
	case []any:
		for _, child := range typed {
			removeExtensions(child)
		}
	}
}

func toMap(value any, allFields bool) (map[string]any, error) {
	sanitized, err := sanitizeValue(value, allFields)
	if err != nil {
		return nil, err
	}
	mapped, ok := sanitized.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("render item is not an object")
	}
	return mapped, nil
}

func summarizeEntity(entity map[string]any, kind EntityKind, verbose bool) string {
	switch kind {
	case EntityMatch:
		id := valueString(entity, "id")
		desc := firstNonEmpty(valueString(entity, "shortDescription"), valueString(entity, "description"))
		if desc == "" {
			desc = valueString(entity, "note")
		}
		if desc == "" {
			desc = valueString(entity, "date")
		}
		teams := matchTeamsLabel(entity)
		state := valueString(entity, "matchState")
		score := valueString(entity, "scoreSummary")
		venue := firstNonEmpty(valueString(entity, "venueName"), valueString(entity, "venueSummary"))
		date := valueString(entity, "date")
		if verbose {
			return joinParts(
				id,
				desc,
				teams,
				state,
				score,
				date,
				venue,
				"league "+valueString(entity, "leagueId"),
				"event "+valueString(entity, "eventId"),
			)
		}
		return joinParts(id, desc, teams, state, score, date)
	case EntityMatchScorecard:
		return joinParts(
			fmt.Sprintf("batting %d", len(sliceValue(entity, "battingCards"))),
			fmt.Sprintf("bowling %d", len(sliceValue(entity, "bowlingCards"))),
			fmt.Sprintf("partnerships %d", len(sliceValue(entity, "partnershipCards"))),
		)
	case EntityMatchSituation:
		if data := valueString(entity, "data"); data != "" {
			return joinParts("situation", data)
		}
		return joinParts("situation", valueString(entity, "competitionId"))
	case EntityPlayer:
		return joinParts(valueString(entity, "displayName"), bracket(valueString(entity, "id")))
	case EntityTeam:
		name := firstNonEmpty(valueString(entity, "name"), valueString(entity, "shortName"), valueString(entity, "id"))
		return joinParts(name, bracket(valueString(entity, "homeAway")))
	case EntityTeamRoster:
		name := firstNonEmpty(valueString(entity, "displayName"), valueString(entity, "playerId"), valueString(entity, "athleteId"))
		badges := []string{}
		if valueString(entity, "captain") == "true" {
			badges = append(badges, "captain")
		}
		if valueString(entity, "active") == "true" {
			badges = append(badges, "active")
		}
		return joinParts(name, valueString(entity, "playerRef"), strings.Join(badges, ", "))
	case EntityTeamScore:
		return joinParts(valueString(entity, "displayValue"), valueString(entity, "value"), bracket(valueString(entity, "source")))
	case EntityTeamLeaders:
		return joinParts(valueString(entity, "name"), fmt.Sprintf("%d categories", len(sliceValue(entity, "categories"))))
	case EntityTeamStatistics, EntityTeamRecords:
		return joinParts(firstNonEmpty(valueString(entity, "displayName"), valueString(entity, "name")), fmt.Sprintf("%d stats", len(sliceValue(entity, "stats"))))
	case EntityLeague:
		return joinParts(firstNonEmpty(valueString(entity, "name"), valueString(entity, "id")), bracket(valueString(entity, "slug")))
	case EntitySeason:
		return joinParts(valueString(entity, "id"), valueString(entity, "leagueId"))
	case EntityStandingsGroup:
		return joinParts(valueString(entity, "id"), "season "+valueString(entity, "seasonId"))
	case EntityInnings:
		score := valueString(entity, "score")
		if score == "" {
			score = joinParts(valueString(entity, "runs")+"/"+valueString(entity, "wickets"), valueString(entity, "overs")+" ov")
		}
		return joinParts(
			firstNonEmpty(valueString(entity, "teamName"), valueString(entity, "teamId")),
			"innings "+valueString(entity, "inningsNumber")+"/"+valueString(entity, "period"),
			score,
			fmt.Sprintf("%d wickets", len(sliceValue(entity, "wicketTimeline"))),
		)
	case EntityDeliveryEvent:
		short := firstNonEmpty(valueString(entity, "shortText"), valueString(entity, "text"))
		if short == "" {
			short = joinParts("over "+valueString(entity, "overNumber"), "ball "+valueString(entity, "ballNumber"))
		}
		return short
	case EntityStatCategory:
		return joinParts(firstNonEmpty(valueString(entity, "displayName"), valueString(entity, "name")), fmt.Sprintf("%d stats", len(sliceValue(entity, "stats"))))
	case EntityPartnership:
		return joinParts(
			firstNonEmpty(valueString(entity, "wicketName"), "partnership "+valueString(entity, "id")),
			valueString(entity, "runs")+" runs",
			valueString(entity, "overs")+" ov",
			"innings "+valueString(entity, "inningsId")+"/"+valueString(entity, "period"),
		)
	case EntityFallOfWicket:
		return joinParts(
			"wicket "+valueString(entity, "wicketNumber"),
			valueString(entity, "runs")+"/"+valueString(entity, "wicketNumber"),
			valueString(entity, "wicketOver")+" ov",
			"innings "+valueString(entity, "inningsId")+"/"+valueString(entity, "period"),
		)
	default:
		if summary := valueString(entity, "id"); summary != "" {
			return summary
		}
		return "item"
	}
}

func formatSingleEntity(entity map[string]any, kind EntityKind, opts RenderOptions) []string {
	order := []string{}
	switch kind {
	case EntityMatch:
		order = []string{
			"id", "competitionId", "eventId", "leagueId",
			"description", "shortDescription", "matchState",
			"date", "endDate", "venueName", "venueSummary", "scoreSummary",
			"statusRef", "detailsRef", "teams",
		}
	case EntityPlayer:
		order = []string{"id", "displayName", "fullName", "battingName", "fieldingName", "teamRef", "newsRef"}
	case EntityTeam:
		order = []string{"id", "name", "shortName", "homeAway", "scoreRef", "rosterRef", "leadersRef"}
	case EntityTeamScore:
		order = []string{"teamId", "matchId", "scope", "displayValue", "value", "winner", "source"}
	case EntityLeague:
		order = []string{"id", "name", "slug"}
	case EntitySeason:
		order = []string{"id", "year", "leagueId"}
	case EntityStandingsGroup:
		order = []string{"id", "seasonId", "groupId"}
	case EntityInnings:
		order = []string{
			"teamName", "teamId", "matchId", "inningsNumber", "period",
			"runs", "wickets", "overs", "score", "description",
			"statisticsRef", "partnershipsRef", "fallOfWicketRef",
			"overTimeline", "wicketTimeline",
		}
	case EntityDeliveryEvent:
		order = []string{
			"id", "period", "overNumber", "ballNumber", "scoreValue", "shortText",
			"playType", "dismissal", "dismissalType", "bbbTimestamp", "xCoordinate", "yCoordinate",
		}
	case EntityMatchScorecard:
		order = []string{"matchId", "competitionId", "eventId", "leagueId", "battingCards", "bowlingCards", "partnershipCards"}
	case EntityMatchSituation:
		order = []string{"matchId", "competitionId", "eventId", "leagueId", "oddsRef", "data"}
	case EntityStatCategory:
		order = []string{"name", "displayName", "abbreviation"}
	case EntityPartnership:
		order = []string{"teamName", "teamId", "inningsId", "period", "wicketNumber", "wicketName", "runs", "overs", "runRate", "batsmen"}
	case EntityFallOfWicket:
		order = []string{"teamName", "teamId", "inningsId", "period", "wicketNumber", "wicketOver", "runs", "runsScored", "ballsFaced", "athleteRef"}
	}

	lines := make([]string, 0, len(order)+2)
	for _, key := range order {
		value := entity[key]
		if isEmptyValue(value) {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", key, printableValue(value)))
	}

	if opts.AllFields {
		if extMap, ok := entity["extensions"].(map[string]any); ok && len(extMap) > 0 {
			keys := mapsKeys(extMap)
			sort.Strings(keys)
			lines = append(lines, "extension fields: "+strings.Join(keys, ", "))
		}
	}

	if opts.Verbose {
		if ref := valueString(entity, "ref"); ref != "" {
			lines = append(lines, "ref: "+ref)
		}
	}

	return lines
}

func formatMatchScorecard(entity map[string]any) []string {
	lines := make([]string, 0, 64)

	if matchID := firstNonEmpty(valueString(entity, "matchId"), valueString(entity, "competitionId")); matchID != "" {
		lines = append(lines, "Match: "+matchID)
	}

	batting := sliceValue(entity, "battingCards")
	if len(batting) > 0 {
		lines = append(lines, "Batting")
		lines = append(lines, formatBattingCards(batting)...)
	}

	bowling := sliceValue(entity, "bowlingCards")
	if len(bowling) > 0 {
		lines = append(lines, "Bowling")
		lines = append(lines, formatBowlingCards(bowling)...)
	}

	partnerships := sliceValue(entity, "partnershipCards")
	if len(partnerships) > 0 {
		lines = append(lines, "Partnerships")
		lines = append(lines, formatPartnershipCards(partnerships)...)
	}

	if len(batting) == 0 && len(bowling) == 0 && len(partnerships) == 0 {
		lines = append(lines, "No scorecard sections available.")
	}

	return lines
}

func formatInningsTimelines(entity map[string]any) []string {
	lines := make([]string, 0, 64)

	if teamName := firstNonEmpty(valueString(entity, "teamName"), valueString(entity, "teamId")); teamName != "" {
		lines = append(lines, "Team: "+teamName)
	}
	if matchID := firstNonEmpty(valueString(entity, "matchId"), valueString(entity, "competitionId")); matchID != "" {
		lines = append(lines, "Match: "+matchID)
	}

	header := joinParts(
		"Innings "+valueString(entity, "inningsNumber")+"/"+valueString(entity, "period"),
		valueString(entity, "score"),
	)
	if strings.TrimSpace(header) == "" {
		header = joinParts(
			"Innings "+valueString(entity, "inningsNumber")+"/"+valueString(entity, "period"),
			valueString(entity, "runs")+"/"+valueString(entity, "wickets"),
			valueString(entity, "overs")+" ov",
		)
	}
	if header != "" {
		lines = append(lines, header)
	}

	overs := sliceValue(entity, "overTimeline")
	if len(overs) > 0 {
		lines = append(lines, "Over Timeline")
		for _, rawOver := range overs {
			over, ok := rawOver.(map[string]any)
			if !ok {
				continue
			}
			row := joinParts(
				"Over "+valueString(over, "number"),
				valueString(over, "runs")+" runs",
				valueString(over, "wicketCount")+" wkts",
			)
			if row != "" {
				lines = append(lines, "  "+row)
			}
		}
	}

	wickets := sliceValue(entity, "wicketTimeline")
	if len(wickets) > 0 {
		lines = append(lines, "Wicket Timeline")
		for idx, rawWicket := range wickets {
			wicket, ok := rawWicket.(map[string]any)
			if !ok {
				continue
			}
			row := joinParts(
				"#"+valueString(wicket, "number"),
				valueString(wicket, "fow"),
				valueString(wicket, "over")+" ov",
				firstNonEmpty(valueString(wicket, "shortText"), valueString(wicket, "detailShortText")),
			)
			if row == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %d. %s", idx+1, row))
			if detailRef := valueString(wicket, "detailRef"); detailRef != "" {
				lines = append(lines, "     detail: "+detailRef)
			}
		}
	}

	if len(overs) == 0 && len(wickets) == 0 {
		lines = append(lines, "No period timeline data available.")
	}

	return lines
}

func formatBattingCards(cards []any) []string {
	lines := make([]string, 0, len(cards)*6)
	for _, rawCard := range cards {
		card, ok := rawCard.(map[string]any)
		if !ok {
			continue
		}
		header := joinParts(
			"Innings "+valueString(card, "inningsNumber"),
			valueString(card, "teamName"),
			valueString(card, "runs"),
			valueString(card, "total"),
		)
		if strings.TrimSpace(header) != "" {
			lines = append(lines, header)
		}
		for idx, rawPlayer := range sliceValue(card, "players") {
			player, ok := rawPlayer.(map[string]any)
			if !ok {
				continue
			}
			score := valueString(player, "runs")
			if balls := valueString(player, "ballsFaced"); balls != "" {
				score = strings.TrimSpace(joinParts(score, "("+balls+" balls)"))
			}
			boundary := joinParts("4s "+valueString(player, "fours"), "6s "+valueString(player, "sixes"))
			row := joinParts(valueString(player, "playerName"), score, boundary, valueString(player, "dismissal"))
			if row == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %d. %s", idx+1, row))
		}
	}
	return lines
}

func formatBowlingCards(cards []any) []string {
	lines := make([]string, 0, len(cards)*6)
	for _, rawCard := range cards {
		card, ok := rawCard.(map[string]any)
		if !ok {
			continue
		}
		header := joinParts("Innings "+valueString(card, "inningsNumber"), valueString(card, "teamName"))
		if strings.TrimSpace(header) != "" {
			lines = append(lines, header)
		}
		for idx, rawPlayer := range sliceValue(card, "players") {
			player, ok := rawPlayer.(map[string]any)
			if !ok {
				continue
			}
			figures := joinParts(
				"overs "+valueString(player, "overs"),
				"maidens "+valueString(player, "maidens"),
				"runs "+valueString(player, "conceded"),
				"wkts "+valueString(player, "wickets"),
				"econ "+valueString(player, "economyRate"),
			)
			row := joinParts(valueString(player, "playerName"), figures, valueString(player, "nbw"))
			if row == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %d. %s", idx+1, row))
		}
	}
	return lines
}

func formatPartnershipCards(cards []any) []string {
	lines := make([]string, 0, len(cards)*6)
	for _, rawCard := range cards {
		card, ok := rawCard.(map[string]any)
		if !ok {
			continue
		}
		header := joinParts("Innings "+valueString(card, "inningsNumber"), valueString(card, "teamName"))
		if strings.TrimSpace(header) != "" {
			lines = append(lines, header)
		}
		for idx, rawPlayer := range sliceValue(card, "players") {
			player, ok := rawPlayer.(map[string]any)
			if !ok {
				continue
			}
			runs := valueString(player, "partnershipRuns")
			runsText := ""
			if runs != "" {
				runsText = runs + " runs"
			}
			overs := valueString(player, "partnershipOvers")
			oversText := ""
			if overs != "" {
				oversText = overs + " overs"
			}
			pair := joinParts(valueString(player, "player1Name"), valueString(player, "player2Name"))
			detail := joinParts(
				valueString(player, "partnershipWicketName"),
				runsText,
				oversText,
			)
			row := joinParts(pair, detail)
			if row == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %d. %s", idx+1, row))
		}
	}
	return lines
}

func formatTeamLeaders(entity map[string]any) []string {
	lines := make([]string, 0, 64)

	if teamID := valueString(entity, "teamId"); teamID != "" {
		lines = append(lines, "Team: "+teamID)
	}
	if matchID := valueString(entity, "matchId"); matchID != "" {
		lines = append(lines, "Match: "+matchID)
	}

	categories := sliceValue(entity, "categories")
	if len(categories) == 0 {
		lines = append(lines, "No leaderboard categories available.")
		return lines
	}

	batting := make([]string, 0)
	bowling := make([]string, 0)
	other := make([]string, 0)

	for _, rawCategory := range categories {
		category, ok := rawCategory.(map[string]any)
		if !ok {
			continue
		}
		role := leaderCategoryRole(category)
		rows := formatTeamLeaderCategory(category, role)
		switch role {
		case "batting":
			batting = append(batting, rows...)
		case "bowling":
			bowling = append(bowling, rows...)
		default:
			other = append(other, rows...)
		}
	}

	if len(batting) > 0 {
		lines = append(lines, "Batting Leaders")
		lines = append(lines, batting...)
	}
	if len(bowling) > 0 {
		lines = append(lines, "Bowling Leaders")
		lines = append(lines, bowling...)
	}
	if len(other) > 0 {
		lines = append(lines, "Other Leaders")
		lines = append(lines, other...)
	}
	if len(batting) == 0 && len(bowling) == 0 && len(other) == 0 {
		lines = append(lines, "No leaderboard categories available.")
	}

	return lines
}

func formatTeamLeaderCategory(category map[string]any, role string) []string {
	lines := make([]string, 0, 24)
	name := firstNonEmpty(valueString(category, "displayName"), valueString(category, "name"))
	if name == "" {
		name = "Leaders"
	}
	lines = append(lines, name)

	leaders := sliceValue(category, "leaders")
	for idx, rawLeader := range leaders {
		leader, ok := rawLeader.(map[string]any)
		if !ok {
			continue
		}

		player := firstNonEmpty(valueString(leader, "athleteName"), valueString(leader, "athleteId"), valueString(leader, "athleteRef"))
		primary := valueString(leader, "displayValue")
		if primary == "" {
			primary = valueString(leader, "value")
		}

		score := ""
		switch role {
		case "batting":
			if primary != "" {
				score = primary + " runs"
			}
		case "bowling":
			if primary != "" {
				score = primary + " wkts"
			}
		default:
			score = primary
		}

		extras := []string{}
		if role == "batting" {
			if balls := valueString(leader, "balls"); balls != "" {
				extras = append(extras, balls+" balls")
			}
			if fours := valueString(leader, "fours"); fours != "" {
				extras = append(extras, fours+"x4")
			}
			if sixes := valueString(leader, "sixes"); sixes != "" {
				extras = append(extras, sixes+"x6")
			}
		}
		if role == "bowling" {
			if overs := valueString(leader, "overs"); overs != "" {
				extras = append(extras, overs+" ov")
			}
			if runs := valueString(leader, "runs"); runs != "" {
				extras = append(extras, runs+" runs")
			}
			if economy := valueString(leader, "economyRate"); economy != "" {
				extras = append(extras, "econ "+economy)
			}
		}

		row := joinParts(player, score, strings.Join(extras, ", "))
		if row == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %d. %s", idx+1, row))
	}

	return lines
}

func leaderCategoryRole(category map[string]any) string {
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(valueString(category, "name"), valueString(category, "displayName"))))
	if strings.Contains(name, "run") || strings.Contains(name, "bat") {
		return "batting"
	}
	if strings.Contains(name, "wicket") || strings.Contains(name, "bowl") {
		return "bowling"
	}
	return "other"
}

func mapsKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func valueString(m map[string]any, key string) string {
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%.2f", typed)
	case bool:
		return fmt.Sprintf("%t", typed)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func sliceValue(m map[string]any, key string) []any {
	value, ok := m[key]
	if !ok || value == nil {
		return nil
	}
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	return raw
}

func printableValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%.2f", typed)
	case []any:
		return fmt.Sprintf("%d items", len(typed))
	case map[string]any:
		keys := mapsKeys(typed)
		sort.Strings(keys)
		if len(keys) > 5 {
			keys = keys[:5]
		}
		return "{" + strings.Join(keys, ", ") + "}"
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func isEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func joinParts(parts ...string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		trimmed = append(trimmed, part)
	}
	return strings.Join(trimmed, " - ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func bracket(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return "(" + value + ")"
}

func titleize(value string) string {
	value = strings.ReplaceAll(value, "_", " ")
	parts := strings.Fields(value)
	for i := range parts {
		parts[i] = strings.Title(parts[i])
	}
	return strings.Join(parts, " ")
}

func sentenceCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimSuffix(value, ".")
	if len(value) == 1 {
		return strings.ToUpper(value) + "."
	}
	return strings.ToUpper(value[:1]) + value[1:] + "."
}

func matchTeamsLabel(entity map[string]any) string {
	teams := sliceValue(entity, "teams")
	if len(teams) == 0 {
		return ""
	}

	parts := make([]string, 0, len(teams))
	for _, raw := range teams {
		team, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := firstNonEmpty(valueString(team, "shortName"), valueString(team, "name"), valueString(team, "id"))
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " vs ")
}
