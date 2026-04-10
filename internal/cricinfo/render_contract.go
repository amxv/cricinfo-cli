package cricinfo

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// EntityKind identifies normalized Cricinfo entity families.
type EntityKind string

const (
	EntityMatch          EntityKind = "match"
	EntityPlayer         EntityKind = "player"
	EntityTeam           EntityKind = "team"
	EntityLeague         EntityKind = "league"
	EntitySeason         EntityKind = "season"
	EntityStandingsGroup EntityKind = "standings_group"
	EntityInnings        EntityKind = "innings"
	EntityDeliveryEvent  EntityKind = "delivery_event"
	EntityStatCategory   EntityKind = "stat_category"
	EntityPartnership    EntityKind = "partnership"
	EntityFallOfWicket   EntityKind = "fall_of_wicket"
)

// ResultStatus standardizes output states across render formats.
type ResultStatus string

const (
	ResultStatusOK      ResultStatus = "ok"
	ResultStatusEmpty   ResultStatus = "empty"
	ResultStatusPartial ResultStatus = "partial"
	ResultStatusError   ResultStatus = "error"
)

// TransportError is the normalized transport failure payload.
type TransportError struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode,omitempty"`
	URL        string `json:"url,omitempty"`
}

// NormalizedResult is the render boundary contract used by commands.
type NormalizedResult struct {
	Kind         EntityKind      `json:"kind"`
	Status       ResultStatus    `json:"status"`
	RequestedRef string          `json:"requestedRef,omitempty"`
	CanonicalRef string          `json:"canonicalRef,omitempty"`
	Message      string          `json:"message,omitempty"`
	Warnings     []string        `json:"warnings,omitempty"`
	Error        *TransportError `json:"error,omitempty"`
	Data         any             `json:"data,omitempty"`
	Items        []any           `json:"items,omitempty"`
}

// Match is the normalized core match shape.
type Match struct {
	Ref              string         `json:"ref,omitempty"`
	ID               string         `json:"id,omitempty"`
	UID              string         `json:"uid,omitempty"`
	LeagueID         string         `json:"leagueId,omitempty"`
	EventID          string         `json:"eventId,omitempty"`
	CompetitionID    string         `json:"competitionId,omitempty"`
	Description      string         `json:"description,omitempty"`
	ShortDescription string         `json:"shortDescription,omitempty"`
	Note             string         `json:"note,omitempty"`
	MatchState       string         `json:"matchState,omitempty"`
	Date             string         `json:"date,omitempty"`
	EndDate          string         `json:"endDate,omitempty"`
	VenueName        string         `json:"venueName,omitempty"`
	VenueSummary     string         `json:"venueSummary,omitempty"`
	ScoreSummary     string         `json:"scoreSummary,omitempty"`
	StatusRef        string         `json:"statusRef,omitempty"`
	DetailsRef       string         `json:"detailsRef,omitempty"`
	Teams            []Team         `json:"teams,omitempty"`
	Extensions       map[string]any `json:"extensions,omitempty"`
}

// Player is the normalized core player shape.
type Player struct {
	Ref          string         `json:"ref,omitempty"`
	ID           string         `json:"id,omitempty"`
	UID          string         `json:"uid,omitempty"`
	DisplayName  string         `json:"displayName,omitempty"`
	FullName     string         `json:"fullName,omitempty"`
	ShortName    string         `json:"shortName,omitempty"`
	BattingName  string         `json:"battingName,omitempty"`
	FieldingName string         `json:"fieldingName,omitempty"`
	Gender       string         `json:"gender,omitempty"`
	Age          int            `json:"age,omitempty"`
	TeamRef      string         `json:"teamRef,omitempty"`
	Position     string         `json:"position,omitempty"`
	Styles       []string       `json:"styles,omitempty"`
	NewsRef      string         `json:"newsRef,omitempty"`
	Extensions   map[string]any `json:"extensions,omitempty"`
}

// Team is the normalized core team or competitor shape.
type Team struct {
	Ref           string         `json:"ref,omitempty"`
	ID            string         `json:"id,omitempty"`
	UID           string         `json:"uid,omitempty"`
	Name          string         `json:"name,omitempty"`
	ShortName     string         `json:"shortName,omitempty"`
	Abbreviation  string         `json:"abbreviation,omitempty"`
	ScoreSummary  string         `json:"scoreSummary,omitempty"`
	Type          string         `json:"type,omitempty"`
	HomeAway      string         `json:"homeAway,omitempty"`
	Order         int            `json:"order,omitempty"`
	Winner        bool           `json:"winner"`
	ScoreRef      string         `json:"scoreRef,omitempty"`
	RosterRef     string         `json:"rosterRef,omitempty"`
	LeadersRef    string         `json:"leadersRef,omitempty"`
	StatisticsRef string         `json:"statisticsRef,omitempty"`
	RecordRef     string         `json:"recordRef,omitempty"`
	LinescoresRef string         `json:"linescoresRef,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
}

// League is the normalized core league shape.
type League struct {
	Ref        string         `json:"ref,omitempty"`
	ID         string         `json:"id,omitempty"`
	UID        string         `json:"uid,omitempty"`
	Name       string         `json:"name,omitempty"`
	Slug       string         `json:"slug,omitempty"`
	SeasonRef  string         `json:"seasonRef,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// Season is the normalized core season shape.
type Season struct {
	Ref        string         `json:"ref,omitempty"`
	ID         string         `json:"id,omitempty"`
	LeagueID   string         `json:"leagueId,omitempty"`
	Year       int            `json:"year,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// StandingsGroup is the normalized standings-group shape.
type StandingsGroup struct {
	Ref        string         `json:"ref,omitempty"`
	ID         string         `json:"id,omitempty"`
	LeagueID   string         `json:"leagueId,omitempty"`
	SeasonID   string         `json:"seasonId,omitempty"`
	GroupID    string         `json:"groupId,omitempty"`
	Entries    []Team         `json:"entries,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// Innings is the normalized innings shape.
type Innings struct {
	Ref             string         `json:"ref,omitempty"`
	ID              string         `json:"id,omitempty"`
	Period          int            `json:"period,omitempty"`
	Runs            int            `json:"runs,omitempty"`
	Wickets         int            `json:"wickets,omitempty"`
	Overs           float64        `json:"overs,omitempty"`
	Score           string         `json:"score,omitempty"`
	Description     string         `json:"description,omitempty"`
	Target          int            `json:"target,omitempty"`
	StatisticsRef   string         `json:"statisticsRef,omitempty"`
	LeadersRef      string         `json:"leadersRef,omitempty"`
	PartnershipsRef string         `json:"partnershipsRef,omitempty"`
	FallOfWicketRef string         `json:"fallOfWicketRef,omitempty"`
	Extensions      map[string]any `json:"extensions,omitempty"`
}

// DeliveryEvent is the normalized ball-level event shape.
type DeliveryEvent struct {
	Ref           string         `json:"ref,omitempty"`
	ID            string         `json:"id,omitempty"`
	Period        int            `json:"period,omitempty"`
	PeriodText    string         `json:"periodText,omitempty"`
	OverNumber    int            `json:"overNumber,omitempty"`
	BallNumber    int            `json:"ballNumber,omitempty"`
	ScoreValue    int            `json:"scoreValue,omitempty"`
	ShortText     string         `json:"shortText,omitempty"`
	Text          string         `json:"text,omitempty"`
	HomeScore     string         `json:"homeScore,omitempty"`
	AwayScore     string         `json:"awayScore,omitempty"`
	BatsmanRef    string         `json:"batsmanRef,omitempty"`
	BowlerRef     string         `json:"bowlerRef,omitempty"`
	DismissalType string         `json:"dismissalType,omitempty"`
	DismissalText string         `json:"dismissalText,omitempty"`
	SpeedKPH      float64        `json:"speedKPH,omitempty"`
	CoordinateX   *float64       `json:"coordinateX,omitempty"`
	CoordinateY   *float64       `json:"coordinateY,omitempty"`
	Timestamp     int64          `json:"timestamp,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
}

// StatCategory is the normalized grouped statistics shape.
type StatCategory struct {
	Name         string         `json:"name,omitempty"`
	DisplayName  string         `json:"displayName,omitempty"`
	ShortName    string         `json:"shortName,omitempty"`
	Abbreviation string         `json:"abbreviation,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	Stats        []StatValue    `json:"stats,omitempty"`
	Extensions   map[string]any `json:"extensions,omitempty"`
}

// StatValue is a normalized statistic entry.
type StatValue struct {
	Name         string         `json:"name,omitempty"`
	DisplayName  string         `json:"displayName,omitempty"`
	ShortName    string         `json:"shortName,omitempty"`
	Description  string         `json:"description,omitempty"`
	Abbreviation string         `json:"abbreviation,omitempty"`
	DisplayValue string         `json:"displayValue,omitempty"`
	Value        any            `json:"value,omitempty"`
	Type         string         `json:"type,omitempty"`
	Extensions   map[string]any `json:"extensions,omitempty"`
}

// Partnership is the normalized partnership shape.
type Partnership struct {
	Ref        string         `json:"ref,omitempty"`
	ID         string         `json:"id,omitempty"`
	InningsID  string         `json:"inningsId,omitempty"`
	Period     string         `json:"period,omitempty"`
	Order      int            `json:"order,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// FallOfWicket is the normalized wicket-fall shape.
type FallOfWicket struct {
	Ref          string         `json:"ref,omitempty"`
	ID           string         `json:"id,omitempty"`
	InningsID    string         `json:"inningsId,omitempty"`
	WicketNumber int            `json:"wicketNumber,omitempty"`
	Extensions   map[string]any `json:"extensions,omitempty"`
}

// NewDataResult creates a successful single-entity result.
func NewDataResult(kind EntityKind, data any) NormalizedResult {
	return NormalizedResult{
		Kind:   kind,
		Status: ResultStatusOK,
		Data:   data,
	}
}

// NewListResult creates a list result and auto-normalizes empty state.
func NewListResult(kind EntityKind, items []any) NormalizedResult {
	if len(items) == 0 {
		return NormalizedResult{
			Kind:    kind,
			Status:  ResultStatusEmpty,
			Message: fmt.Sprintf("no %s found", kindPlural(kind)),
			Items:   []any{},
		}
	}

	return NormalizedResult{
		Kind:   kind,
		Status: ResultStatusOK,
		Items:  items,
	}
}

// NewPartialResult creates a partial-data result for single entity responses.
func NewPartialResult(kind EntityKind, data any, warnings ...string) NormalizedResult {
	return NormalizedResult{
		Kind:     kind,
		Status:   ResultStatusPartial,
		Data:     data,
		Warnings: compactWarnings(warnings),
		Message:  "partial data returned",
	}
}

// NewPartialListResult creates a partial-data list result.
func NewPartialListResult(kind EntityKind, items []any, warnings ...string) NormalizedResult {
	if len(items) == 0 {
		result := NewListResult(kind, items)
		result.Status = ResultStatusPartial
		result.Warnings = compactWarnings(warnings)
		if result.Message == "" {
			result.Message = "partial data returned"
		}
		return result
	}

	return NormalizedResult{
		Kind:     kind,
		Status:   ResultStatusPartial,
		Items:    items,
		Warnings: compactWarnings(warnings),
		Message:  "partial data returned",
	}
}

// NewTransportErrorResult standardizes transport failure payloads.
func NewTransportErrorResult(kind EntityKind, requestedRef string, err error) NormalizedResult {
	if err == nil {
		err = errors.New("unknown transport error")
	}

	transportErr := &TransportError{Message: "transport error"}

	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		transportErr.StatusCode = statusErr.StatusCode
		transportErr.URL = statusErr.URL
		transportErr.Message = fmt.Sprintf("transport error: status %d", statusErr.StatusCode)
	} else {
		transportErr.Message = fmt.Sprintf("transport error: %v", err)
	}

	if requestedRef == "" {
		requestedRef = transportErr.URL
	}

	return NormalizedResult{
		Kind:         kind,
		Status:       ResultStatusError,
		RequestedRef: requestedRef,
		Message:      transportErr.Message,
		Error:        transportErr,
	}
}

func kindPlural(kind EntityKind) string {
	switch kind {
	case EntityMatch:
		return "matches"
	case EntityPlayer:
		return "players"
	case EntityTeam:
		return "teams"
	case EntityLeague:
		return "leagues"
	case EntitySeason:
		return "seasons"
	case EntityStandingsGroup:
		return "standings groups"
	case EntityInnings:
		return "innings"
	case EntityDeliveryEvent:
		return "delivery events"
	case EntityStatCategory:
		return "stat categories"
	case EntityPartnership:
		return "partnerships"
	case EntityFallOfWicket:
		return "fall-of-wicket entries"
	default:
		return "results"
	}
}

func compactWarnings(warnings []string) []string {
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		out = append(out, warning)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func refIDs(raw string) map[string]string {
	ids := map[string]string{}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ids
	}
	segments := strings.Split(strings.Trim(path.Clean(u.Path), "/"), "/")
	for i := 0; i+1 < len(segments); i++ {
		key := segments[i]
		value := segments[i+1]
		switch key {
		case "leagues":
			ids["leagueId"] = value
		case "events":
			ids["eventId"] = value
		case "competitions":
			ids["competitionId"] = value
		case "competitors":
			ids["competitorId"] = value
		case "teams":
			ids["teamId"] = value
		case "athletes":
			ids["athleteId"] = value
		case "seasons":
			ids["seasonId"] = value
		case "groups":
			ids["groupId"] = value
		case "standings":
			ids["standingsId"] = value
		case "linescores":
			if _, ok := ids["inningsId"]; !ok {
				ids["inningsId"] = value
			} else {
				ids["periodId"] = value
			}
		case "details":
			ids["detailId"] = value
		case "partnerships":
			ids["partnershipId"] = value
		case "fow":
			ids["fowId"] = value
		}
	}
	return ids
}

func parseYear(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	year, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	if year < 1800 || year > 3000 {
		return 0
	}
	return year
}

func parseInt(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}
