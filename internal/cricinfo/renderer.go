package cricinfo

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
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
			warningLine = warningLine + ": " + strings.Join(sanitizeWarningsForText(result.Warnings), "; ")
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
		if result.Kind == EntityMatchSituation {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Match Situation")
			lines = append(lines, formatMatchSituation(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityPlayerStats {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Player Statistics")
			lines = append(lines, formatPlayerStatistics(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityAnalysisDismiss || result.Kind == EntityAnalysisBowl || result.Kind == EntityAnalysisBat || result.Kind == EntityAnalysisPart {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Analysis")
			lines = append(lines, formatAnalysisView(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityPlayer {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Player")
			lines = append(lines, formatPlayerProfile(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityPlayerMatch {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Player Match")
			lines = append(lines, formatPlayerMatchView(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityCompMetadata {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Competition Metadata")
			lines = append(lines, formatCompetitionMetadata(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityMatch || result.Kind == EntityCompetition {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, kindTitle)
			lines = append(lines, formatMatchView(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityMatchPhases {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Match Phases")
			lines = append(lines, formatMatchPhases(itemMap)...)
			return writeTextLines(w, lines)
		}
		if result.Kind == EntityMatchDuel {
			itemMap, err := toMap(result.Data, opts.AllFields)
			if err != nil {
				return err
			}
			lines = append(lines, "Match Duel")
			lines = append(lines, formatMatchDuel(itemMap)...)
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

	if result.Kind == EntityTeamStatistics || result.Kind == EntityTeamRecords {
		title := "Team Statistics"
		if result.Kind == EntityTeamRecords {
			title = "Team Records"
		}
		lines = append(lines, fmt.Sprintf("%s Categories (%d)", title, len(result.Items)))
		lines = append(lines, formatStatCategoryList(result.Items)...)
		return writeTextLines(w, lines)
	}

	if result.Kind == EntityStandingsGroup {
		lines = append(lines, formatStandingsGroupList(result.Items)...)
		return writeTextLines(w, lines)
	}

	if result.Kind == EntityDeliveryEvent || result.Kind == EntityPlayerDelivery {
		lines = append(lines, fmt.Sprintf("%s (%d)", titleize(kindPlural(result.Kind)), len(result.Items)))
		if result.Kind == EntityDeliveryEvent {
			source := ""
			switch {
			case strings.Contains(result.RequestedRef, "/details"):
				source = "details"
			case strings.Contains(result.RequestedRef, "/plays"):
				source = "plays"
			}
			if source != "" {
				lines = append(lines, "Source: "+source)
			}
		}
		for i, item := range result.Items {
			summary := summarizeDeliveryListItem(item, result.Kind)
			if strings.TrimSpace(summary) == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, summary))
		}
		return writeTextLines(w, lines)
	}

	summaries := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		itemMap, err := toMap(item, opts.AllFields)
		if err != nil {
			return err
		}
		summary := summarizeEntity(itemMap, result.Kind, opts.Verbose)
		if strings.TrimSpace(summary) == "" {
			continue
		}
		summaries = append(summaries, summary)
	}
	if len(summaries) == 0 {
		message := result.Message
		if message == "" {
			message = fmt.Sprintf("No %s found.", kindPlural(result.Kind))
		}
		lines = append(lines, sentenceCase(message))
		return writeTextLines(w, lines)
	}
	lines = append(lines, fmt.Sprintf("%s (%d)", titleize(kindPlural(result.Kind)), len(summaries)))
	for i, summary := range summaries {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, summary))
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

func summarizeDeliveryListItem(item any, kind EntityKind) string {
	switch typed := item.(type) {
	case DeliveryEvent:
		short := firstNonEmpty(strings.TrimSpace(typed.ShortText), strings.TrimSpace(typed.Text))
		if short == "" {
			short = joinParts("over "+intString(typed.OverNumber), "ball "+intString(typed.BallNumber))
		}
		lead := firstNonEmpty(overBallString(typed.OverNumber, typed.BallNumber), "")
		score := firstNonEmpty(scoreLabel(typed.HomeScore), scoreLabel(typed.AwayScore))
		if kind == EntityPlayerDelivery {
			return joinParts(lead, short, score, strings.Join(typed.Involvement, ","))
		}
		return joinParts(lead, short, score)
	case map[string]any:
		short := firstNonEmpty(valueString(typed, "shortText"), valueString(typed, "text"))
		if short == "" {
			short = joinParts("over "+valueString(typed, "overNumber"), "ball "+valueString(typed, "ballNumber"))
		}
		if strings.TrimSpace(short) == "/" || strings.TrimSpace(short) == "-" {
			return ""
		}
		lead := overBallLabel(typed)
		score := firstNonEmpty(scoreLabel(valueString(typed, "homeScore")), scoreLabel(valueString(typed, "awayScore")))
		if kind == EntityPlayerDelivery {
			return joinParts(lead, short, score, involvementLabel(typed))
		}
		return joinParts(lead, short, score)
	default:
		return ""
	}
}

func overBallString(over, ball int) string {
	if over <= 0 || ball <= 0 {
		return ""
	}
	return fmt.Sprintf("%d.%d", over, ball)
}

func intString(value int) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func defaultNumeric(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "0"
	}
	return raw
}

func scoreLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "/") {
		return raw
	}
	return ""
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

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}

	blob, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var mapped map[string]any
	if err := json.Unmarshal(blob, &mapped); err != nil {
		return nil
	}
	return mapped
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
	case EntityMatchDuel:
		duelLabel := strings.TrimSpace(fmt.Sprintf("%s vs %s",
			firstNonEmpty(valueString(entity, "batterName"), valueString(entity, "batterId")),
			firstNonEmpty(valueString(entity, "bowlerName"), valueString(entity, "bowlerId")),
		))
		return joinParts(
			duelLabel,
			fmt.Sprintf("%s off %s", valueString(entity, "runs"), valueString(entity, "balls")),
		)
	case EntityMatchPhases:
		return joinParts(
			"match "+firstNonEmpty(valueString(entity, "matchId"), valueString(entity, "competitionId")),
			fmt.Sprintf("%d innings", len(sliceValue(entity, "innings"))),
		)
	case EntityCompetition:
		return joinParts(
			firstNonEmpty(valueString(entity, "shortDescription"), valueString(entity, "description"), valueString(entity, "id")),
			matchTeamsLabel(entity),
			valueString(entity, "matchState"),
		)
	case EntityCompOfficial, EntityCompBroadcast, EntityCompTicket, EntityCompOdds:
		return joinParts(
			firstNonEmpty(valueString(entity, "displayName"), valueString(entity, "name"), valueString(entity, "text"), valueString(entity, "value"), valueString(entity, "id")),
			valueString(entity, "role"),
			valueString(entity, "type"),
		)
	case EntityCompMetadata:
		return joinParts(
			"officials "+fmt.Sprintf("%d", len(sliceValue(entity, "officials"))),
			"broadcasts "+fmt.Sprintf("%d", len(sliceValue(entity, "broadcasts"))),
			"tickets "+fmt.Sprintf("%d", len(sliceValue(entity, "tickets"))),
			"odds "+fmt.Sprintf("%d", len(sliceValue(entity, "odds"))),
		)
	case EntityPlayer:
		return joinParts(firstNonEmpty(valueString(entity, "displayName"), valueString(entity, "fullName"), valueString(entity, "name")), bracket(valueString(entity, "id")))
	case EntityPlayerStats:
		return joinParts(firstNonEmpty(valueString(entity, "name"), "statistics"), fmt.Sprintf("%d categories", len(sliceValue(entity, "categories"))))
	case EntityPlayerMatch:
		return joinParts(
			firstNonEmpty(valueString(entity, "playerName"), valueString(entity, "playerId")),
			"match "+valueString(entity, "matchId"),
			"bat "+fmt.Sprintf("%d", len(sliceValue(entity, "batting"))),
			"bowl "+fmt.Sprintf("%d", len(sliceValue(entity, "bowling"))),
		)
	case EntityPlayerInnings:
		return joinParts(
			firstNonEmpty(valueString(entity, "playerName"), valueString(entity, "playerId")),
			"innings "+valueString(entity, "inningsNumber")+"/"+valueString(entity, "period"),
			firstNonEmpty(valueString(entity, "teamName"), valueString(entity, "teamId")),
		)
	case EntityPlayerDismissal:
		return joinParts(
			firstNonEmpty(valueString(entity, "dismissalName"), valueString(entity, "dismissalType")),
			firstNonEmpty(valueString(entity, "dismissalCard"), valueString(entity, "fow")),
			valueString(entity, "detailRef"),
		)
	case EntityPlayerDelivery:
		short := firstNonEmpty(valueString(entity, "shortText"), valueString(entity, "text"))
		return joinParts(short, involvementLabel(entity), overBallLabel(entity))
	case EntityNewsArticle:
		return joinParts(firstNonEmpty(valueString(entity, "headline"), valueString(entity, "title"), valueString(entity, "id")), valueString(entity, "published"), valueString(entity, "byline"))
	case EntityTeam:
		name := firstNonEmpty(valueString(entity, "name"), valueString(entity, "shortName"), valueString(entity, "id"))
		return joinParts(name, bracket(valueString(entity, "homeAway")))
	case EntityTeamRoster:
		name := strings.TrimSpace(valueString(entity, "displayName"))
		if name == "" {
			playerID := firstNonEmpty(valueString(entity, "playerId"), valueString(entity, "athleteId"))
			if playerID != "" {
				name = "Unknown player (" + playerID + ")"
			} else {
				name = "Unknown player"
			}
		}
		badges := []string{}
		if valueString(entity, "captain") == "true" {
			badges = append(badges, "captain")
		}
		if valueString(entity, "active") == "true" {
			badges = append(badges, "active")
		}
		return joinParts(name, firstNonEmpty(valueString(entity, "teamName"), valueString(entity, "teamId")), strings.Join(badges, ", "))
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
	case EntityCalendarDay:
		return joinParts(
			valueString(entity, "date"),
			valueString(entity, "dayType"),
			strings.Join(stringSliceValue(entity, "sections"), ", "),
		)
	case EntitySeasonType:
		return joinParts(
			firstNonEmpty(valueString(entity, "name"), "type "+valueString(entity, "id")),
			"season "+valueString(entity, "seasonId"),
		)
	case EntitySeasonGroup:
		return joinParts(
			firstNonEmpty(valueString(entity, "name"), "group "+valueString(entity, "id")),
			"type "+valueString(entity, "typeId"),
			"season "+valueString(entity, "seasonId"),
		)
	case EntityStandingsGroup:
		return joinParts(valueString(entity, "id"), "season "+valueString(entity, "seasonId"))
	case EntityInnings:
		if valueString(entity, "score") == "" &&
			valueString(entity, "runs") == "" &&
			valueString(entity, "wickets") == "" &&
			valueString(entity, "target") == "" &&
			valueString(entity, "isBatting") == "false" {
			return ""
		}
		score := valueString(entity, "score")
		if score == "" {
			score = joinParts(valueString(entity, "runs")+"/"+valueString(entity, "wickets"), valueString(entity, "overs")+" ov")
		}
		parts := []string{
			firstNonEmpty(valueString(entity, "teamName"), valueString(entity, "teamId")),
			"innings " + valueString(entity, "inningsNumber") + "/" + valueString(entity, "period"),
			score,
		}
		if wc := len(sliceValue(entity, "wicketTimeline")); wc > 0 {
			parts = append(parts, fmt.Sprintf("%d wickets", wc))
		}
		return joinParts(parts...)
	case EntityDeliveryEvent:
		short := firstNonEmpty(valueString(entity, "shortText"), valueString(entity, "text"))
		if short == "" {
			short = joinParts("over "+valueString(entity, "overNumber"), "ball "+valueString(entity, "ballNumber"))
		}
		return short
	case EntityStatCategory:
		return joinParts(firstNonEmpty(valueString(entity, "displayName"), valueString(entity, "name")), fmt.Sprintf("%d stats", len(sliceValue(entity, "stats"))))
	case EntityPartnership:
		runsText := ""
		if runs := valueString(entity, "runs"); runs != "" {
			runsText = runs + " runs"
		} else if valueString(entity, "overs") != "" {
			runsText = "0 runs"
		}
		oversText := ""
		if overs := valueString(entity, "overs"); overs != "" {
			oversText = overs + " ov"
		}
		return joinParts(
			firstNonEmpty(valueString(entity, "wicketName"), "partnership "+valueString(entity, "id")),
			runsText,
			oversText,
			"innings "+valueString(entity, "inningsId")+"/"+valueString(entity, "period"),
		)
	case EntityFallOfWicket:
		scoreText := ""
		if runs := valueString(entity, "runs"); runs != "" {
			scoreText = runs + "/" + valueString(entity, "wicketNumber")
		} else if valueString(entity, "wicketNumber") == "1" {
			scoreText = "0/1"
		}
		return joinParts(
			"wicket "+valueString(entity, "wicketNumber"),
			scoreText,
			valueString(entity, "wicketOver")+" ov",
			"innings "+valueString(entity, "inningsId")+"/"+valueString(entity, "period"),
		)
	case EntityAnalysisDismiss, EntityAnalysisBowl, EntityAnalysisBat, EntityAnalysisPart:
		return joinParts(
			valueString(entity, "key"),
			valueString(entity, "metric"),
			valueString(entity, "value"),
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
			"teams",
		}
	case EntityCompetition:
		order = []string{
			"id", "competitionId", "eventId", "leagueId",
			"description", "shortDescription", "matchState",
			"date", "endDate", "venueName", "venueSummary", "scoreSummary",
			"teams",
		}
	case EntityCompOfficial, EntityCompBroadcast, EntityCompTicket, EntityCompOdds:
		order = []string{
			"id", "displayName", "name", "role", "type", "order", "text", "value", "href",
		}
	case EntityCompMetadata:
		order = []string{
			"competition", "officials", "broadcasts", "tickets", "odds",
		}
	case EntityPlayer:
		order = []string{
			"id", "displayName", "fullName", "name", "firstName", "middleName", "lastName",
			"battingName", "fieldingName", "gender", "age", "dateOfBirthDisplay",
			"position", "team", "majorTeams", "debuts", "newsRef",
		}
	case EntityPlayerStats:
		order = []string{"playerId", "name", "abbreviation", "splitId", "categories"}
	case EntityPlayerMatch:
		order = []string{
			"playerId", "playerName", "matchId", "teamId", "teamName",
			"summary", "batting", "bowling", "fielding",
		}
	case EntityPlayerInnings:
		order = []string{
			"playerId", "playerName", "matchId", "teamId", "teamName",
			"inningsNumber", "period", "order", "isBatting", "summary",
			"batting", "bowling", "fielding",
		}
	case EntityPlayerDismissal:
		order = []string{
			"playerId", "playerName", "matchId", "teamId", "teamName",
			"inningsNumber", "period", "wicketNumber", "fow", "over",
			"dismissalName", "dismissalCard", "dismissalType", "dismissalText",
			"ballsFaced", "strikeRate", "batsmanPlayerId", "bowlerPlayerId", "fielderPlayerId",
			"detailRef", "detailShortText",
		}
	case EntityPlayerDelivery:
		order = []string{
			"id", "matchId", "teamId", "period", "overNumber", "ballNumber", "scoreValue",
			"shortText", "dismissalType", "dismissalName", "dismissalCard",
			"batsmanPlayerId", "bowlerPlayerId", "fielderPlayerId",
			"xCoordinate", "yCoordinate", "involvement",
		}
	case EntityNewsArticle:
		order = []string{"id", "headline", "title", "byline", "published", "description", "webUrl"}
	case EntityTeam:
		order = []string{"id", "name", "shortName", "homeAway"}
	case EntityTeamScore:
		order = []string{"teamId", "matchId", "scope", "displayValue", "value", "winner", "source"}
	case EntityLeague:
		order = []string{"id", "name", "slug"}
	case EntitySeason:
		order = []string{"id", "year", "leagueId"}
	case EntityCalendarDay:
		order = []string{"date", "dayType", "sections", "startDate", "endDate", "leagueId"}
	case EntitySeasonType:
		order = []string{"id", "name", "abbreviation", "seasonId", "leagueId", "startDate", "endDate", "hasGroups", "hasStandings", "groupsRef"}
	case EntitySeasonGroup:
		order = []string{"id", "name", "abbreviation", "typeId", "seasonId", "leagueId", "standingsRef"}
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
	case EntityMatchPhases:
		order = []string{"matchId", "competitionId", "eventId", "leagueId", "fixture", "result", "innings"}
	case EntityStatCategory:
		order = []string{"name", "displayName", "abbreviation"}
	case EntityPartnership:
		order = []string{"teamName", "teamId", "inningsId", "period", "wicketNumber", "wicketName", "runs", "overs", "runRate", "batsmen"}
	case EntityFallOfWicket:
		order = []string{"teamName", "teamId", "inningsId", "period", "wicketNumber", "wicketOver", "runs", "runsScored", "ballsFaced", "athleteRef"}
	case EntityAnalysisDismiss, EntityAnalysisBowl, EntityAnalysisBat, EntityAnalysisPart:
		order = []string{
			"command", "metric", "scope", "groupBy", "filters", "rows",
		}
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
				wicketCountLabel(valueString(over, "wicketCount")),
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

func wicketCountLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return ""
	}
	return raw + " wkts"
}

func formatPlayerProfile(entity map[string]any) []string {
	lines := make([]string, 0, 16)
	if name := firstNonEmpty(valueString(entity, "displayName"), valueString(entity, "fullName"), valueString(entity, "name")); name != "" {
		lines = append(lines, "Name: "+name)
	}
	if playerID := valueString(entity, "id"); playerID != "" {
		lines = append(lines, "Player ID: "+playerID)
	}
	if role := valueString(entity, "position"); role != "" {
		lines = append(lines, "Role: "+role)
	}
	if team := namedValue(entity["team"]); team != "" && !looksLikeRawIdentifier(team) {
		lines = append(lines, "Team: "+team)
	}
	if styles := styleSummary(entity); styles != "" {
		lines = append(lines, "Styles: "+styles)
	}
	if majorTeams := sliceSummary(entity, "majorTeams", 4); majorTeams != "" {
		lines = append(lines, "Major Teams: "+majorTeams)
	}
	if debuts := sliceSummary(entity, "debuts", 4); debuts != "" {
		if strings.HasSuffix(debuts, " items") {
			lines = append(lines, fmt.Sprintf("Debuts: %d matches", len(sliceValue(entity, "debuts"))))
		} else {
			lines = append(lines, "Debuts: "+debuts)
		}
	} else if debuts := sliceValue(entity, "debuts"); len(debuts) > 0 {
		lines = append(lines, fmt.Sprintf("Debuts: %d matches", len(debuts)))
	}
	if born := valueString(entity, "dateOfBirthDisplay"); born != "" {
		lines = append(lines, "Born: "+born)
	}
	return lines
}

func formatPlayerMatchView(entity map[string]any) []string {
	lines := make([]string, 0, 48)
	if player := valueString(entity, "playerName"); player != "" {
		lines = append(lines, "Player: "+player)
	}
	if matchID := valueString(entity, "matchId"); matchID != "" {
		lines = append(lines, "Match: "+matchID)
	}
	if team := valueString(entity, "teamName"); team != "" {
		lines = append(lines, "Team: "+team)
	}
	if summary, ok := entity["summary"].(map[string]any); ok {
		if text := summarizePlayerMatchSummary(summary); text != "" {
			lines = append(lines, text)
		}
	}
	if batting := sliceValue(entity, "batting"); len(batting) > 0 {
		lines = append(lines, "Batting")
		lines = append(lines, formatStatCategoryList(batting)...)
	}
	if bowling := sliceValue(entity, "bowling"); len(bowling) > 0 {
		lines = append(lines, "Bowling")
		lines = append(lines, formatStatCategoryList(bowling)...)
	}
	if fielding := sliceValue(entity, "fielding"); len(fielding) > 0 {
		lines = append(lines, "Fielding")
		lines = append(lines, formatStatCategoryList(fielding)...)
	}
	if len(sliceValue(entity, "batting")) == 0 &&
		len(sliceValue(entity, "bowling")) == 0 &&
		len(sliceValue(entity, "fielding")) == 0 {
		lines = append(lines, "No match statistics categories available.")
	}
	return lines
}

func formatCompetitionMetadata(entity map[string]any) []string {
	lines := make([]string, 0, 12)
	if competition, ok := entity["competition"].(map[string]any); ok {
		lines = append(lines, "Competition: "+joinParts(
			firstNonEmpty(valueString(competition, "shortDescription"), valueString(competition, "description"), valueString(competition, "id")),
			matchTeamsLabel(competition),
			valueString(competition, "matchState"),
		))
	}
	if officials := summarizeNamedItems(sliceValue(entity, "officials"), 5); officials != "" {
		lines = append(lines, "Officials: "+officials)
	}
	if broadcasts := summarizeNamedItems(sliceValue(entity, "broadcasts"), 4); broadcasts != "" {
		lines = append(lines, "Broadcasts: "+broadcasts)
	}
	if odds := summarizeNamedItems(sliceValue(entity, "odds"), 3); odds != "" {
		lines = append(lines, "Odds: "+odds)
	}
	if tickets := sliceValue(entity, "tickets"); len(tickets) > 0 {
		lines = append(lines, fmt.Sprintf("Tickets: %d options", len(tickets)))
	}
	return lines
}

func formatMatchView(entity map[string]any) []string {
	lines := make([]string, 0, 16)
	if id := valueString(entity, "id"); id != "" {
		lines = append(lines, "Match: "+id)
	}
	if desc := firstNonEmpty(valueString(entity, "shortDescription"), valueString(entity, "description")); desc != "" {
		lines = append(lines, "Fixture: "+desc)
	}
	if state := valueString(entity, "matchState"); state != "" {
		lines = append(lines, "Status: "+state)
	}
	if score := valueString(entity, "scoreSummary"); score != "" {
		lines = append(lines, "Score: "+score)
	}
	if venue := firstNonEmpty(valueString(entity, "venueName"), valueString(entity, "venueSummary")); venue != "" {
		lines = append(lines, "Venue: "+venue)
	}
	if date := valueString(entity, "date"); date != "" {
		lines = append(lines, "Date: "+date)
	}
	if teams := summarizeNamedItems(sliceValue(entity, "teams"), 4); teams != "" {
		lines = append(lines, "Teams: "+teams)
	}
	return lines
}

func formatMatchSituation(entity map[string]any) []string {
	lines := make([]string, 0, 64)
	if matchID := firstNonEmpty(valueString(entity, "matchId"), valueString(entity, "competitionId")); matchID != "" {
		lines = append(lines, "Match: "+matchID)
	}

	live, _ := entity["live"].(map[string]any)
	if live == nil {
		if oddsRef := valueString(entity, "oddsRef"); oddsRef != "" {
			lines = append(lines, "Odds Ref: "+oddsRef)
		}
		if data, ok := entity["data"].(map[string]any); ok && len(data) > 0 {
			lines = append(lines, "Data: "+printableValue(data))
		}
		if len(lines) <= 1 {
			lines = append(lines, "No situation data available for this match.")
		}
		return lines
	}

	if fixture := valueString(live, "fixture"); fixture != "" {
		lines = append(lines, "Fixture: "+fixture)
	}
	if status := valueString(live, "status"); status != "" {
		lines = append(lines, "Status: "+status)
	}
	scoreLine := joinParts(valueString(live, "score"), valueString(live, "overs"))
	if scoreLine != "" {
		lines = append(lines, "Score: "+scoreLine)
	}
	teamsLine := joinParts(
		"Batting "+valueString(live, "battingTeam"),
		"Bowling "+valueString(live, "bowlingTeam"),
	)
	if teamsLine != "" {
		lines = append(lines, teamsLine)
	}
	if snapshotAt := valueString(live, "snapshotAt"); snapshotAt != "" {
		lines = append(lines, "Snapshot: "+snapshotAt)
	}
	if stale := valueString(live, "stale"); stale == "true" {
		lines = append(lines, "Stale: true")
		if reason := valueString(live, "staleReason"); reason != "" {
			lines = append(lines, "Stale Reason: "+reason)
		}
	}

	batters := sliceValue(live, "batters")
	if len(batters) > 0 {
		lines = append(lines, "Batters")
		for i, raw := range batters {
			batter, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			score := fmt.Sprintf("%s(%s)", defaultNumeric(valueString(batter, "runs")), defaultNumeric(valueString(batter, "balls")))
			boundaries := joinParts("4s "+defaultNumeric(valueString(batter, "fours")), "6s "+defaultNumeric(valueString(batter, "sixes")))
			row := joinParts(
				firstNonEmpty(valueString(batter, "playerName"), valueString(batter, "playerId")),
				score,
				"SR "+defaultNumeric(valueString(batter, "strikeRate")),
				boundaries,
			)
			if strings.EqualFold(valueString(batter, "onStrike"), "true") {
				row = joinParts(row, "*")
			}
			lines = append(lines, fmt.Sprintf("  %d. %s", i+1, row))
		}
	}

	bowlers := sliceValue(live, "bowlers")
	if len(bowlers) > 0 {
		lines = append(lines, "Bowlers")
		for i, raw := range bowlers {
			bowler, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			figures := fmt.Sprintf("%s-%s-%s-%s",
				oversLabelFromFields(valueString(bowler, "overs"), valueString(bowler, "balls")),
				defaultNumeric(valueString(bowler, "maidens")),
				defaultNumeric(valueString(bowler, "conceded")),
				defaultNumeric(valueString(bowler, "wickets")),
			)
			row := joinParts(
				firstNonEmpty(valueString(bowler, "playerName"), valueString(bowler, "playerId")),
				figures,
				"Econ "+defaultNumeric(valueString(bowler, "economy")),
			)
			lines = append(lines, fmt.Sprintf("  %d. %s", i+1, row))
		}
	}

	balls := sliceValue(live, "recentBalls")
	if len(balls) == 0 {
		balls = sliceValue(live, "currentOverBalls")
	}
	if len(balls) > 0 {
		lines = append(lines, "Recent Balls")
		for i, raw := range balls {
			lines = append(lines, fmt.Sprintf("  %d. %s", i+1, summarizeDeliveryListItem(raw, EntityDeliveryEvent)))
		}
	}
	return lines
}

func formatMatchDuel(entity map[string]any) []string {
	lines := make([]string, 0, 48)
	if matchID := valueString(entity, "matchId"); matchID != "" {
		lines = append(lines, "Match: "+matchID)
	}
	if fixture := valueString(entity, "fixture"); fixture != "" {
		lines = append(lines, "Fixture: "+fixture)
	}
	if score := valueString(entity, "score"); score != "" {
		lines = append(lines, "Score: "+score)
	}
	lines = append(lines, "Duel: "+firstNonEmpty(valueString(entity, "batterName"), valueString(entity, "batterId"))+" vs "+firstNonEmpty(valueString(entity, "bowlerName"), valueString(entity, "bowlerId")))
	summary := joinParts(
		fmt.Sprintf("%s off %s", defaultNumeric(valueString(entity, "runs")), defaultNumeric(valueString(entity, "balls"))),
		"SR "+defaultNumeric(valueString(entity, "strikeRate")),
		"dots "+defaultNumeric(valueString(entity, "dots")),
		"4s "+defaultNumeric(valueString(entity, "fours")),
		"6s "+defaultNumeric(valueString(entity, "sixes")),
		"wkts "+defaultNumeric(valueString(entity, "wickets")),
	)
	if strings.TrimSpace(summary) != "" {
		lines = append(lines, "Summary: "+summary)
	}
	if snapshot := valueString(entity, "snapshotAt"); snapshot != "" {
		lines = append(lines, "Snapshot: "+snapshot)
	}
	balls := sliceValue(entity, "recentBalls")
	if len(balls) > 0 {
		lines = append(lines, "Recent Duel Balls")
		for i, raw := range balls {
			lines = append(lines, fmt.Sprintf("  %d. %s", i+1, summarizeDeliveryListItem(raw, EntityDeliveryEvent)))
		}
	}
	return lines
}

func oversLabelFromFields(overs string, balls string) string {
	overs = strings.TrimSpace(overs)
	if overs != "" && overs != "0" {
		return overs
	}
	balls = strings.TrimSpace(balls)
	if balls == "" || balls == "0" {
		return "0.0"
	}
	b, err := strconv.Atoi(balls)
	if err != nil || b < 0 {
		return "0.0"
	}
	return fmt.Sprintf("%d.%d", b/6, b%6)
}

func formatMatchPhases(entity map[string]any) []string {
	lines := make([]string, 0, 64)
	if matchID := firstNonEmpty(valueString(entity, "matchId"), valueString(entity, "competitionId")); matchID != "" {
		lines = append(lines, "Match: "+matchID)
	}
	if fixture := valueString(entity, "fixture"); fixture != "" {
		lines = append(lines, "Fixture: "+fixture)
	}
	if result := valueString(entity, "result"); result != "" {
		lines = append(lines, "Result: "+result)
	}

	for _, raw := range sliceValue(entity, "innings") {
		inn, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		lines = append(lines, "")
		lines = append(lines, joinParts(
			firstNonEmpty(valueString(inn, "teamName"), valueString(inn, "teamId")),
			"innings "+valueString(inn, "inningsNumber")+"/"+valueString(inn, "period"),
			valueString(inn, "score"),
		))

		lines = append(lines, "  Phases")
		for _, key := range []string{"powerplay", "middle", "death"} {
			phaseMap, ok := inn[key].(map[string]any)
			if !ok {
				continue
			}
			name := firstNonEmpty(valueString(phaseMap, "name"), strings.Title(key))
			runs := defaultNumeric(valueString(phaseMap, "runs"))
			wickets := defaultNumeric(valueString(phaseMap, "wickets"))
			overs := defaultNumeric(valueString(phaseMap, "overs"))
			runRate := defaultNumeric(valueString(phaseMap, "runRate"))
			phaseLine := joinParts(
				name,
				"runs "+runs,
				"wkts "+wickets,
				"ov "+overs,
				"rr "+runRate,
			)
			lines = append(lines, "  - "+phaseLine)
		}

		bestOver := valueString(inn, "bestScoringOver")
		bestRuns := valueString(inn, "bestScoringOverRuns")
		if bestOver != "" && bestRuns != "" {
			lines = append(lines, "  Best Over: over "+bestOver+" ("+bestRuns+" runs)")
		}
		collapseOver := valueString(inn, "collapseOver")
		collapseWickets := valueString(inn, "collapseWickets")
		if collapseOver != "" && collapseWickets != "" && collapseWickets != "0" {
			lines = append(lines, "  Pressure Over: over "+collapseOver+" ("+collapseWickets+" wickets)")
		}
	}

	return lines
}

func formatPlayerStatistics(entity map[string]any) []string {
	lines := make([]string, 0, 64)

	if playerID := valueString(entity, "playerId"); playerID != "" {
		lines = append(lines, "Player: "+playerID)
	}
	if header := joinParts(firstNonEmpty(valueString(entity, "name"), "Statistics"), bracket(valueString(entity, "abbreviation"))); header != "" {
		lines = append(lines, header)
	}

	categories := sliceValue(entity, "categories")
	if len(categories) == 0 {
		lines = append(lines, "No statistics categories available.")
		return lines
	}

	for _, rawCategory := range categories {
		category, ok := rawCategory.(map[string]any)
		if !ok {
			continue
		}
		categoryName := firstNonEmpty(valueString(category, "displayName"), valueString(category, "name"))
		if categoryName == "" {
			categoryName = "Category"
		}
		lines = append(lines, categoryName)

		for idx, rawStat := range sliceValue(category, "stats") {
			stat, ok := rawStat.(map[string]any)
			if !ok {
				continue
			}
			row := joinParts(
				firstNonEmpty(valueString(stat, "displayName"), valueString(stat, "name")),
				firstNonEmpty(valueString(stat, "displayValue"), valueString(stat, "value")),
				bracket(valueString(stat, "abbreviation")),
			)
			if row == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %d. %s", idx+1, row))
		}
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

	if teamName := firstNonEmpty(valueString(entity, "teamName"), valueString(entity, "name")); teamName != "" {
		lines = append(lines, "Team: "+teamName)
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

func formatStatCategoryList(items []any) []string {
	lines := make([]string, 0, len(items)*4)
	for i, rawCategory := range items {
		category := mapFromAny(rawCategory)
		if category == nil {
			continue
		}

		categoryName := firstNonEmpty(valueString(category, "displayName"), valueString(category, "name"), "Category")
		lines = append(lines, fmt.Sprintf("  %d. %s", i+1, categoryName))

		stats := sliceValue(category, "stats")
		if len(stats) == 0 {
			continue
		}

		limit := len(stats)
		if limit > 16 {
			limit = 16
		}
		for j := 0; j < limit; j++ {
			statMap, ok := stats[j].(map[string]any)
			if !ok {
				continue
			}
			label := firstNonEmpty(valueString(statMap, "displayName"), valueString(statMap, "name"), valueString(statMap, "abbreviation"))
			value := firstNonEmpty(valueString(statMap, "displayValue"), valueString(statMap, "value"))
			row := joinParts(label, value, bracket(valueString(statMap, "abbreviation")))
			if row == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("     - %s", row))
		}
		if len(stats) > limit {
			lines = append(lines, fmt.Sprintf("     - ... %d more", len(stats)-limit))
		}
	}
	return lines
}

func formatStandingsGroupList(items []any) []string {
	lines := make([]string, 0, len(items)*8+1)
	lines = append(lines, fmt.Sprintf("Standings Groups (%d)", len(items)))

	for i, rawGroup := range items {
		group := mapFromAny(rawGroup)
		if group == nil {
			continue
		}

		groupID := firstNonEmpty(valueString(group, "groupId"), valueString(group, "id"), fmt.Sprintf("%d", i+1))
		seasonID := valueString(group, "seasonId")
		header := "Group " + groupID
		if seasonID != "" {
			header = joinParts(header, "Season "+seasonID)
		}
		lines = append(lines, header)

		entries := sliceValue(group, "entries")
		if len(entries) == 0 {
			lines = append(lines, "  No standings entries available.")
			continue
		}

		for entryIndex, rawEntry := range entries {
			team := mapFromAny(rawEntry)
			if team == nil {
				continue
			}

			rank := standingsStatValueFromTeam(team, "rank", "position")
			if rank == "" {
				rank = fmt.Sprintf("%d", entryIndex+1)
			}
			teamName := firstNonEmpty(valueString(team, "shortName"), valueString(team, "name"), valueString(team, "id"))
			played := standingsStatValueFromTeam(team, "matchesplayed", "played", "matches")
			won := standingsStatValueFromTeam(team, "wins", "won")
			lost := standingsStatValueFromTeam(team, "losses", "lost")
			points := standingsStatValueFromTeam(team, "matchpoints", "points", "pts")
			nrr := standingsStatValueFromTeam(team, "netrunrate", "nrr", "runrate")

			row := joinParts(
				fmt.Sprintf("#%s %s", rank, teamName),
				nonEmptyLabel("P", played),
				nonEmptyLabel("W", won),
				nonEmptyLabel("L", lost),
				nonEmptyLabel("Pts", points),
				nonEmptyLabel("NRR", nrr),
			)
			if strings.TrimSpace(row) == "" {
				row = joinParts(fmt.Sprintf("#%s %s", rank, teamName), valueString(team, "scoreSummary"))
			}
			lines = append(lines, "  "+row)
		}
	}
	return lines
}

func standingsStatValueFromTeam(team map[string]any, names ...string) string {
	if len(names) == 0 || team == nil {
		return ""
	}

	targets := map[string]struct{}{}
	for _, name := range names {
		key := normalizeStatName(name)
		if key != "" {
			targets[key] = struct{}{}
		}
	}

	extensions, ok := team["extensions"].(map[string]any)
	if !ok || extensions == nil {
		return ""
	}
	records, ok := extensions["records"].([]any)
	if !ok || len(records) == 0 {
		return ""
	}

	for _, rawRecord := range records {
		record := mapFromAny(rawRecord)
		if record == nil {
			continue
		}
		for _, rawStat := range sliceValue(record, "stats") {
			stat := mapFromAny(rawStat)
			if stat == nil {
				continue
			}
			nameKey := normalizeStatName(firstNonEmpty(valueString(stat, "name"), valueString(stat, "type")))
			if _, ok := targets[nameKey]; !ok {
				continue
			}
			value := firstNonEmpty(valueString(stat, "displayValue"), valueString(stat, "value"))
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func formatAnalysisView(entity map[string]any) []string {
	lines := make([]string, 0, 64)
	if command := valueString(entity, "command"); command != "" {
		lines = append(lines, "Command: "+command)
	}
	if metric := valueString(entity, "metric"); metric != "" {
		lines = append(lines, "Metric: "+metric)
	}

	scopeMap, _ := entity["scope"].(map[string]any)
	if scopeMap != nil {
		mode := valueString(scopeMap, "mode")
		league := firstNonEmpty(valueString(scopeMap, "requestedLeagueId"), valueString(scopeMap, "leagueName"), valueString(scopeMap, "leagueId"))
		matchCount := valueString(scopeMap, "matchCount")
		lines = append(lines, "Scope: "+joinParts(mode, league, "matches "+matchCount))
		if seasons := sliceValue(scopeMap, "seasons"); len(seasons) > 0 {
			seasonParts := make([]string, 0, len(seasons))
			for _, season := range seasons {
				if asString, ok := season.(string); ok && strings.TrimSpace(asString) != "" {
					seasonParts = append(seasonParts, strings.TrimSpace(asString))
				}
			}
			if len(seasonParts) > 0 {
				lines = append(lines, "Seasons: "+strings.Join(seasonParts, ", "))
			}
		}
	}

	if groupBy := stringSliceValue(entity, "groupBy"); len(groupBy) > 0 {
		lines = append(lines, "Group By: "+strings.Join(groupBy, ", "))
	}

	rows := sliceValue(entity, "rows")
	if len(rows) == 0 {
		lines = append(lines, "No ranked rows found.")
		return lines
	}

	lines = append(lines, "Rows")
	for i, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		label := analysisRowLabel(row)
		line := joinParts(
			"#"+valueString(row, "rank"),
			label,
			valueString(row, "value"),
		)
		if count := valueString(row, "count"); count != "" {
			line = joinParts(line, "count "+count)
		}
		if matches := valueString(row, "matches"); matches != "" {
			line = joinParts(line, "matches "+matches)
		}
		if line == "" {
			continue
		}
		if valueString(row, "value") == "0" {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %d. %s", i+1, line))
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

		player := firstNonEmpty(valueString(leader, "athleteName"), valueString(leader, "name"), "Unknown player")
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

func analysisRowLabel(row map[string]any) string {
	if row == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if player := valueString(row, "playerName"); player != "" {
		parts = append(parts, player)
	}
	if team := valueString(row, "teamName"); team != "" {
		parts = append(parts, team)
	}
	if dismissal := valueString(row, "dismissalType"); dismissal != "" {
		parts = append(parts, dismissal)
	}
	innings := valueString(row, "inningsNumber")
	period := valueString(row, "period")
	if innings != "" && innings != "0" {
		if period != "" && period != "0" {
			parts = append(parts, innings+"/"+period)
		} else {
			parts = append(parts, innings)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " | ")
	}
	if key := valueString(row, "key"); key != "" {
		return sanitizeAnalysisKey(key)
	}
	return "row"
}

func sanitizeAnalysisKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	segments := strings.Split(key, "|")
	values := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		_, value, ok := strings.Cut(segment, "=")
		if !ok {
			continue
		}
		value = sanitizeWarningText(value)
		if value == "" || looksLikeRawIdentifier(value) {
			continue
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return "row"
	}
	return strings.Join(values, " | ")
}

func looksLikeRawIdentifier(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return true
	}
	if strings.Contains(value, "/") || strings.Contains(value, "http://") || strings.Contains(value, "https://") {
		return true
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var (
	urlLikeTokenPattern  = regexp.MustCompile(`https?://\S+`)
	htmlTagPattern       = regexp.MustCompile(`<[^>]*>`)
	internalPathPattern  = regexp.MustCompile(`(?:https?://[^\s]+)?/v2/sports/cricket[^\s]*`)
	spaceCollapsePattern = regexp.MustCompile(`\s+`)
)

func sanitizeWarningsForText(warnings []string) []string {
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		cleaned := sanitizeWarningText(warning)
		if cleaned == "" {
			continue
		}
		out = append(out, cleaned)
	}
	if len(out) == 0 {
		return []string{"partial data"}
	}
	return out
}

func sanitizeWarningText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	cleaned := htmlTagPattern.ReplaceAllString(raw, " ")
	cleaned = internalPathPattern.ReplaceAllString(cleaned, " ")
	cleaned = urlLikeTokenPattern.ReplaceAllString(cleaned, " ")
	cleaned = strings.ReplaceAll(cleaned, `""`, "")
	lowered := strings.ToLower(cleaned)
	if strings.Contains(lowered, "backend fetch failed") || strings.Contains(lowered, "context deadline exceeded") {
		return "upstream request failed"
	}
	cleaned = spaceCollapsePattern.ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

func overBallLabel(entity map[string]any) string {
	over := valueString(entity, "overNumber")
	ball := valueString(entity, "ballNumber")
	if over == "" {
		return ""
	}
	if ball == "" {
		return over
	}
	return over + "." + ball
}

func involvementLabel(entity map[string]any) string {
	roles := stringSliceValue(entity, "involvement")
	if len(roles) == 0 {
		return ""
	}
	if len(roles) > 1 {
		filtered := make([]string, 0, len(roles))
		for _, role := range roles {
			if role == "involved" {
				continue
			}
			filtered = append(filtered, role)
		}
		if len(filtered) > 0 {
			roles = filtered
		}
	}
	return strings.Join(roles, ", ")
}

func sliceSummary(entity map[string]any, key string, limit int) string {
	return summarizeNamedItems(sliceValue(entity, key), limit)
}

func summarizeNamedItems(items []any, limit int) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, minInt(len(items), limit))
	for _, raw := range items {
		mapped, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := firstNonEmpty(
			valueString(mapped, "displayName"),
			valueString(mapped, "fullName"),
			valueString(mapped, "shortName"),
			valueString(mapped, "name"),
			valueString(mapped, "headline"),
			valueString(mapped, "title"),
		)
		if name == "" {
			name = namedValue(mapped)
		}
		if name == "" {
			continue
		}
		parts = append(parts, name)
		if len(parts) >= limit {
			break
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d items", len(items))
	}
	if len(items) > len(parts) {
		return strings.Join(parts, ", ") + fmt.Sprintf(" (+%d more)", len(items)-len(parts))
	}
	return strings.Join(parts, ", ")
}

func namedValue(raw any) string {
	mapped, ok := raw.(map[string]any)
	if !ok || mapped == nil {
		return ""
	}
	name := firstNonEmpty(
		valueString(mapped, "displayName"),
		valueString(mapped, "fullName"),
		valueString(mapped, "shortName"),
		valueString(mapped, "name"),
	)
	if name != "" {
		return name
	}
	id := valueString(mapped, "id")
	if looksLikeRawIdentifier(id) {
		return ""
	}
	return id
}

func styleSummary(entity map[string]any) string {
	items := sliceValue(entity, "styles")
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, raw := range items {
		mapped, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := firstNonEmpty(valueString(mapped, "description"), valueString(mapped, "shortDescription"), valueString(mapped, "type"))
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d styles", len(items))
	}
	return strings.Join(parts, ", ")
}

func summarizePlayerMatchSummary(summary map[string]any) string {
	runs := valueString(summary, "runs")
	balls := valueString(summary, "ballsFaced")
	dismissal := valueString(summary, "dismissalName")
	strikeRate := valueString(summary, "strikeRate")
	parts := []string{
		nonEmptyRunsBalls(runs, balls),
		nonEmptyLabel("dismissal", dismissal),
		nonEmptyLabel("SR", strikeRate),
		nonEmptyLabel("econ", valueString(summary, "economyRate")),
		nonEmptyLabel("dots", valueString(summary, "dots")),
	}
	text := joinParts(parts...)
	if text == "" {
		return ""
	}
	return "Summary: " + text
}

func nonEmptyRunsBalls(runs, balls string) string {
	if runs == "" && balls == "" {
		return ""
	}
	if runs != "" && balls != "" {
		return runs + " runs off " + balls
	}
	if runs != "" {
		return runs + " runs"
	}
	return balls + " balls"
}

func nonEmptyLabel(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return label + " " + value
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
	case map[string]any:
		return firstNonEmpty(
			valueString(typed, "displayName"),
			valueString(typed, "fullName"),
			valueString(typed, "shortName"),
			valueString(typed, "name"),
			valueString(typed, "headline"),
			valueString(typed, "title"),
			valueString(typed, "id"),
		)
	case []any:
		return fmt.Sprintf("%d items", len(typed))
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

func stringSliceValue(m map[string]any, key string) []string {
	raw := sliceValue(m, key)
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		asString, ok := item.(string)
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
