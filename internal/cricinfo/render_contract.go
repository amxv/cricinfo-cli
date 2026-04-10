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
	EntityMatchScorecard EntityKind = "match_scorecard"
	EntityMatchSituation EntityKind = "match_situation"
	EntityPlayer         EntityKind = "player"
	EntityTeam           EntityKind = "team"
	EntityTeamRoster     EntityKind = "team_roster"
	EntityTeamScore      EntityKind = "team_score"
	EntityTeamLeaders    EntityKind = "team_leaders"
	EntityTeamStatistics EntityKind = "team_statistics"
	EntityTeamRecords    EntityKind = "team_records"
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

// MatchScorecard is the normalized scorecard view grouped by batting, bowling, and partnerships cards.
type MatchScorecard struct {
	Ref              string            `json:"ref,omitempty"`
	LeagueID         string            `json:"leagueId,omitempty"`
	EventID          string            `json:"eventId,omitempty"`
	CompetitionID    string            `json:"competitionId,omitempty"`
	MatchID          string            `json:"matchId,omitempty"`
	BattingCards     []BattingCard     `json:"battingCards,omitempty"`
	BowlingCards     []BowlingCard     `json:"bowlingCards,omitempty"`
	PartnershipCards []PartnershipCard `json:"partnershipCards,omitempty"`
	Extensions       map[string]any    `json:"extensions,omitempty"`
}

// BattingCard is a normalized batting card section from matchcards.
type BattingCard struct {
	InningsNumber int                `json:"inningsNumber,omitempty"`
	TeamName      string             `json:"teamName,omitempty"`
	Runs          string             `json:"runs,omitempty"`
	Total         string             `json:"total,omitempty"`
	Extras        string             `json:"extras,omitempty"`
	Players       []BattingCardEntry `json:"players,omitempty"`
}

// BattingCardEntry is a player row in a batting card.
type BattingCardEntry struct {
	PlayerID   string `json:"playerId,omitempty"`
	PlayerName string `json:"playerName,omitempty"`
	Dismissal  string `json:"dismissal,omitempty"`
	Runs       string `json:"runs,omitempty"`
	BallsFaced string `json:"ballsFaced,omitempty"`
	Fours      string `json:"fours,omitempty"`
	Sixes      string `json:"sixes,omitempty"`
	Href       string `json:"href,omitempty"`
}

// BowlingCard is a normalized bowling card section from matchcards.
type BowlingCard struct {
	InningsNumber int                `json:"inningsNumber,omitempty"`
	TeamName      string             `json:"teamName,omitempty"`
	Players       []BowlingCardEntry `json:"players,omitempty"`
}

// BowlingCardEntry is a bowler row in a bowling card.
type BowlingCardEntry struct {
	PlayerID    string `json:"playerId,omitempty"`
	PlayerName  string `json:"playerName,omitempty"`
	Overs       string `json:"overs,omitempty"`
	Maidens     string `json:"maidens,omitempty"`
	Conceded    string `json:"conceded,omitempty"`
	Wickets     string `json:"wickets,omitempty"`
	EconomyRate string `json:"economyRate,omitempty"`
	NBW         string `json:"nbw,omitempty"`
	Href        string `json:"href,omitempty"`
}

// PartnershipCard is a normalized partnerships card section from matchcards.
type PartnershipCard struct {
	InningsNumber int                    `json:"inningsNumber,omitempty"`
	TeamName      string                 `json:"teamName,omitempty"`
	Players       []PartnershipCardEntry `json:"players,omitempty"`
}

// PartnershipCardEntry is a row in a partnerships card.
type PartnershipCardEntry struct {
	PartnershipRuns       string `json:"partnershipRuns,omitempty"`
	PartnershipOvers      string `json:"partnershipOvers,omitempty"`
	PartnershipWicketName string `json:"partnershipWicketName,omitempty"`
	FOWType               string `json:"fowType,omitempty"`
	Player1Name           string `json:"player1Name,omitempty"`
	Player1Runs           string `json:"player1Runs,omitempty"`
	Player2Name           string `json:"player2Name,omitempty"`
	Player2Runs           string `json:"player2Runs,omitempty"`
}

// MatchSituation is the normalized match situation payload.
type MatchSituation struct {
	Ref           string         `json:"ref,omitempty"`
	LeagueID      string         `json:"leagueId,omitempty"`
	EventID       string         `json:"eventId,omitempty"`
	CompetitionID string         `json:"competitionId,omitempty"`
	MatchID       string         `json:"matchId,omitempty"`
	OddsRef       string         `json:"oddsRef,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
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

// TeamScope indicates whether a team resource comes from global team endpoints or match-scoped competitor endpoints.
type TeamScope string

const (
	TeamScopeGlobal TeamScope = "global"
	TeamScopeMatch  TeamScope = "match"
)

// TeamRosterEntry is a normalized roster player entry with player-command bridge references.
type TeamRosterEntry struct {
	PlayerID      string         `json:"playerId,omitempty"`
	PlayerRef     string         `json:"playerRef,omitempty"`
	DisplayName   string         `json:"displayName,omitempty"`
	TeamID        string         `json:"teamId,omitempty"`
	TeamRef       string         `json:"teamRef,omitempty"`
	MatchID       string         `json:"matchId,omitempty"`
	Scope         TeamScope      `json:"scope,omitempty"`
	Captain       bool           `json:"captain"`
	Starter       bool           `json:"starter"`
	Active        bool           `json:"active"`
	ActiveName    string         `json:"activeName,omitempty"`
	PositionRef   string         `json:"positionRef,omitempty"`
	LinescoresRef string         `json:"linescoresRef,omitempty"`
	StatisticsRef string         `json:"statisticsRef,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
}

// TeamScore is the normalized team score response.
type TeamScore struct {
	Ref          string         `json:"ref,omitempty"`
	TeamID       string         `json:"teamId,omitempty"`
	MatchID      string         `json:"matchId,omitempty"`
	Scope        TeamScope      `json:"scope,omitempty"`
	DisplayValue string         `json:"displayValue,omitempty"`
	Value        string         `json:"value,omitempty"`
	Place        string         `json:"place,omitempty"`
	Source       string         `json:"source,omitempty"`
	Winner       bool           `json:"winner"`
	Extensions   map[string]any `json:"extensions,omitempty"`
}

// TeamLeaders groups category-based leaderboards for one team.
type TeamLeaders struct {
	Ref        string               `json:"ref,omitempty"`
	TeamID     string               `json:"teamId,omitempty"`
	MatchID    string               `json:"matchId,omitempty"`
	Scope      TeamScope            `json:"scope,omitempty"`
	Name       string               `json:"name,omitempty"`
	Categories []TeamLeaderCategory `json:"categories,omitempty"`
	Extensions map[string]any       `json:"extensions,omitempty"`
}

// TeamLeaderCategory is one leaderboard category (for example runs or wickets).
type TeamLeaderCategory struct {
	Name         string            `json:"name,omitempty"`
	DisplayName  string            `json:"displayName,omitempty"`
	ShortName    string            `json:"shortName,omitempty"`
	Abbreviation string            `json:"abbreviation,omitempty"`
	Leaders      []TeamLeaderEntry `json:"leaders,omitempty"`
	Extensions   map[string]any    `json:"extensions,omitempty"`
}

// TeamLeaderEntry is one player row within a team leaderboard category.
type TeamLeaderEntry struct {
	Order         int            `json:"order,omitempty"`
	DisplayValue  string         `json:"displayValue,omitempty"`
	Value         string         `json:"value,omitempty"`
	AthleteID     string         `json:"athleteId,omitempty"`
	AthleteName   string         `json:"athleteName,omitempty"`
	AthleteRef    string         `json:"athleteRef,omitempty"`
	TeamRef       string         `json:"teamRef,omitempty"`
	StatisticsRef string         `json:"statisticsRef,omitempty"`
	Runs          string         `json:"runs,omitempty"`
	Wickets       string         `json:"wickets,omitempty"`
	Overs         string         `json:"overs,omitempty"`
	Maidens       string         `json:"maidens,omitempty"`
	EconomyRate   string         `json:"economyRate,omitempty"`
	Balls         string         `json:"balls,omitempty"`
	Fours         string         `json:"fours,omitempty"`
	Sixes         string         `json:"sixes,omitempty"`
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
	Ref             string          `json:"ref,omitempty"`
	ID              string          `json:"id,omitempty"`
	LeagueID        string          `json:"leagueId,omitempty"`
	EventID         string          `json:"eventId,omitempty"`
	CompetitionID   string          `json:"competitionId,omitempty"`
	MatchID         string          `json:"matchId,omitempty"`
	TeamID          string          `json:"teamId,omitempty"`
	TeamName        string          `json:"teamName,omitempty"`
	InningsNumber   int             `json:"inningsNumber,omitempty"`
	Period          int             `json:"period,omitempty"`
	Runs            int             `json:"runs,omitempty"`
	Wickets         int             `json:"wickets,omitempty"`
	Overs           float64         `json:"overs,omitempty"`
	Score           string          `json:"score,omitempty"`
	Description     string          `json:"description,omitempty"`
	Target          int             `json:"target,omitempty"`
	IsBatting       bool            `json:"isBatting"`
	IsCurrent       bool            `json:"isCurrent"`
	Fours           int             `json:"fours,omitempty"`
	Sixes           int             `json:"sixes,omitempty"`
	StatisticsRef   string          `json:"statisticsRef,omitempty"`
	LeadersRef      string          `json:"leadersRef,omitempty"`
	PartnershipsRef string          `json:"partnershipsRef,omitempty"`
	FallOfWicketRef string          `json:"fallOfWicketRef,omitempty"`
	OverTimeline    []InningsOver   `json:"overTimeline,omitempty"`
	WicketTimeline  []InningsWicket `json:"wicketTimeline,omitempty"`
	Extensions      map[string]any  `json:"extensions,omitempty"`
}

// InningsOver is one over summary in an innings timeline.
type InningsOver struct {
	Number      int             `json:"number,omitempty"`
	Runs        int             `json:"runs,omitempty"`
	WicketCount int             `json:"wicketCount,omitempty"`
	Wickets     []InningsWicket `json:"wickets,omitempty"`
	Extensions  map[string]any  `json:"extensions,omitempty"`
}

// InningsWicket is a normalized wicket event from period statistics or FOW resources.
type InningsWicket struct {
	Number          int            `json:"number,omitempty"`
	FOW             string         `json:"fow,omitempty"`
	Over            string         `json:"over,omitempty"`
	WicketOver      float64        `json:"wicketOver,omitempty"`
	FOWType         string         `json:"fowType,omitempty"`
	Runs            int            `json:"runs,omitempty"`
	RunsScored      int            `json:"runsScored,omitempty"`
	BallsFaced      int            `json:"ballsFaced,omitempty"`
	DismissalCard   string         `json:"dismissalCard,omitempty"`
	ShortText       string         `json:"shortText,omitempty"`
	DetailRef       string         `json:"detailRef,omitempty"`
	DetailShortText string         `json:"detailShortText,omitempty"`
	DetailText      string         `json:"detailText,omitempty"`
	AthleteRef      string         `json:"athleteRef,omitempty"`
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
	PlayType      map[string]any `json:"playType,omitempty"`
	Dismissal     map[string]any `json:"dismissal,omitempty"`
	DismissalType string         `json:"dismissalType,omitempty"`
	DismissalText string         `json:"dismissalText,omitempty"`
	SpeedKPH      float64        `json:"speedKPH,omitempty"`
	XCoordinate   *float64       `json:"xCoordinate"`
	YCoordinate   *float64       `json:"yCoordinate"`
	BBBTimestamp  int64          `json:"bbbTimestamp"`
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
	Ref          string               `json:"ref,omitempty"`
	ID           string               `json:"id,omitempty"`
	MatchID      string               `json:"matchId,omitempty"`
	TeamID       string               `json:"teamId,omitempty"`
	TeamName     string               `json:"teamName,omitempty"`
	InningsID    string               `json:"inningsId,omitempty"`
	Period       string               `json:"period,omitempty"`
	Order        int                  `json:"order,omitempty"`
	WicketNumber int                  `json:"wicketNumber,omitempty"`
	WicketName   string               `json:"wicketName,omitempty"`
	FOWType      string               `json:"fowType,omitempty"`
	Overs        float64              `json:"overs,omitempty"`
	Runs         int                  `json:"runs,omitempty"`
	RunRate      float64              `json:"runRate,omitempty"`
	Start        PartnershipSnapshot  `json:"start,omitempty"`
	End          PartnershipSnapshot  `json:"end,omitempty"`
	Batsmen      []PartnershipBatsman `json:"batsmen,omitempty"`
	Extensions   map[string]any       `json:"extensions,omitempty"`
}

// PartnershipSnapshot captures start/end score markers for a partnership.
type PartnershipSnapshot struct {
	Overs   float64 `json:"overs,omitempty"`
	Runs    int     `json:"runs,omitempty"`
	Wickets int     `json:"wickets,omitempty"`
}

// PartnershipBatsman captures an individual batter contribution in a partnership.
type PartnershipBatsman struct {
	AthleteRef string `json:"athleteRef,omitempty"`
	Balls      int    `json:"balls,omitempty"`
	Runs       int    `json:"runs,omitempty"`
}

// FallOfWicket is the normalized wicket-fall shape.
type FallOfWicket struct {
	Ref          string         `json:"ref,omitempty"`
	ID           string         `json:"id,omitempty"`
	MatchID      string         `json:"matchId,omitempty"`
	TeamID       string         `json:"teamId,omitempty"`
	TeamName     string         `json:"teamName,omitempty"`
	InningsID    string         `json:"inningsId,omitempty"`
	Period       string         `json:"period,omitempty"`
	WicketNumber int            `json:"wicketNumber,omitempty"`
	WicketOver   float64        `json:"wicketOver,omitempty"`
	FOWType      string         `json:"fowType,omitempty"`
	Runs         int            `json:"runs,omitempty"`
	RunsScored   int            `json:"runsScored,omitempty"`
	BallsFaced   int            `json:"ballsFaced,omitempty"`
	AthleteRef   string         `json:"athleteRef,omitempty"`
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
	case EntityMatchScorecard:
		return "match scorecards"
	case EntityMatchSituation:
		return "match situations"
	case EntityPlayer:
		return "players"
	case EntityTeam:
		return "teams"
	case EntityTeamRoster:
		return "team roster entries"
	case EntityTeamScore:
		return "team scores"
	case EntityTeamLeaders:
		return "team leaderboards"
	case EntityTeamStatistics:
		return "team statistics categories"
	case EntityTeamRecords:
		return "team record categories"
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
