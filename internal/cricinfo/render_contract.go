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
	EntityMatch           EntityKind = "match"
	EntityMatchScorecard  EntityKind = "match_scorecard"
	EntityMatchSituation  EntityKind = "match_situation"
	EntityMatchPhases     EntityKind = "match_phases"
	EntityCompetition     EntityKind = "competition"
	EntityCompOfficial    EntityKind = "competition_official"
	EntityCompBroadcast   EntityKind = "competition_broadcast"
	EntityCompTicket      EntityKind = "competition_ticket"
	EntityCompOdds        EntityKind = "competition_odds"
	EntityCompMetadata    EntityKind = "competition_metadata"
	EntityPlayer          EntityKind = "player"
	EntityPlayerStats     EntityKind = "player_statistics"
	EntityPlayerMatch     EntityKind = "player_match"
	EntityPlayerInnings   EntityKind = "player_innings"
	EntityPlayerDismissal EntityKind = "player_dismissal"
	EntityPlayerDelivery  EntityKind = "player_delivery"
	EntityNewsArticle     EntityKind = "news_article"
	EntityTeam            EntityKind = "team"
	EntityTeamRoster      EntityKind = "team_roster"
	EntityTeamScore       EntityKind = "team_score"
	EntityTeamLeaders     EntityKind = "team_leaders"
	EntityTeamStatistics  EntityKind = "team_statistics"
	EntityTeamRecords     EntityKind = "team_records"
	EntityLeague          EntityKind = "league"
	EntitySeason          EntityKind = "season"
	EntityCalendarDay     EntityKind = "calendar_day"
	EntitySeasonType      EntityKind = "season_type"
	EntitySeasonGroup     EntityKind = "season_group"
	EntityStandingsGroup  EntityKind = "standings_group"
	EntityInnings         EntityKind = "innings"
	EntityDeliveryEvent   EntityKind = "delivery_event"
	EntityStatCategory    EntityKind = "stat_category"
	EntityPartnership     EntityKind = "partnership"
	EntityFallOfWicket    EntityKind = "fall_of_wicket"
	EntityAnalysisDismiss EntityKind = "analysis_dismissal"
	EntityAnalysisBowl    EntityKind = "analysis_bowling"
	EntityAnalysisBat     EntityKind = "analysis_batting"
	EntityAnalysisPart    EntityKind = "analysis_partnership"
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

// MatchPhases is a fan-oriented phase/momentum breakdown for each innings.
type MatchPhases struct {
	MatchID       string             `json:"matchId,omitempty"`
	LeagueID      string             `json:"leagueId,omitempty"`
	EventID       string             `json:"eventId,omitempty"`
	CompetitionID string             `json:"competitionId,omitempty"`
	Fixture       string             `json:"fixture,omitempty"`
	Result        string             `json:"result,omitempty"`
	Innings       []MatchPhaseInning `json:"innings,omitempty"`
}

// MatchPhaseInning captures phase splits and momentum points for one innings.
type MatchPhaseInning struct {
	TeamID              string       `json:"teamId,omitempty"`
	TeamName            string       `json:"teamName,omitempty"`
	InningsNumber       int          `json:"inningsNumber,omitempty"`
	Period              int          `json:"period,omitempty"`
	Score               string       `json:"score,omitempty"`
	Target              int          `json:"target,omitempty"`
	Powerplay           PhaseSummary `json:"powerplay,omitempty"`
	Middle              PhaseSummary `json:"middle,omitempty"`
	Death               PhaseSummary `json:"death,omitempty"`
	BestScoringOver     int          `json:"bestScoringOver,omitempty"`
	BestScoringOverRuns int          `json:"bestScoringOverRuns,omitempty"`
	CollapseOver        int          `json:"collapseOver,omitempty"`
	CollapseWickets     int          `json:"collapseWickets,omitempty"`
}

// PhaseSummary is a normalized run/wicket split across a phase bucket.
type PhaseSummary struct {
	Name    string  `json:"name,omitempty"`
	Runs    int     `json:"runs,omitempty"`
	Wickets int     `json:"wickets,omitempty"`
	Overs   float64 `json:"overs,omitempty"`
	RunRate float64 `json:"runRate,omitempty"`
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
	Live          *MatchLiveView `json:"live,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
}

// MatchLiveView is a synthesized fan-first live snapshot when upstream situation payload is sparse.
type MatchLiveView struct {
	Fixture      string           `json:"fixture,omitempty"`
	Status       string           `json:"status,omitempty"`
	Score        string           `json:"score,omitempty"`
	Overs        string           `json:"overs,omitempty"`
	CurrentOver  int              `json:"currentOver,omitempty"`
	BallInOver   int              `json:"ballInOver,omitempty"`
	BattingTeam  string           `json:"battingTeam,omitempty"`
	BowlingTeam  string           `json:"bowlingTeam,omitempty"`
	Batters      []LiveBatterView `json:"batters,omitempty"`
	Bowlers      []LiveBowlerView `json:"bowlers,omitempty"`
	RecentBalls  []DeliveryEvent  `json:"recentBalls,omitempty"`
	CurrentBalls []DeliveryEvent  `json:"currentOverBalls,omitempty"`
}

// LiveBatterView captures in-progress batter figures.
type LiveBatterView struct {
	PlayerID   string  `json:"playerId,omitempty"`
	PlayerName string  `json:"playerName,omitempty"`
	Runs       int     `json:"runs,omitempty"`
	Balls      int     `json:"balls,omitempty"`
	Fours      int     `json:"fours,omitempty"`
	Sixes      int     `json:"sixes,omitempty"`
	StrikeRate float64 `json:"strikeRate,omitempty"`
	OnStrike   bool    `json:"onStrike"`
}

// LiveBowlerView captures in-progress bowler figures.
type LiveBowlerView struct {
	PlayerID   string  `json:"playerId,omitempty"`
	PlayerName string  `json:"playerName,omitempty"`
	Overs      float64 `json:"overs,omitempty"`
	Balls      int     `json:"balls,omitempty"`
	Maidens    int     `json:"maidens,omitempty"`
	Conceded   int     `json:"conceded,omitempty"`
	Wickets    int     `json:"wickets,omitempty"`
	Economy    float64 `json:"economy,omitempty"`
}

// Competition is the normalized competition metadata root view.
type Competition struct {
	Ref              string         `json:"ref,omitempty"`
	ID               string         `json:"id,omitempty"`
	LeagueID         string         `json:"leagueId,omitempty"`
	EventID          string         `json:"eventId,omitempty"`
	CompetitionID    string         `json:"competitionId,omitempty"`
	Description      string         `json:"description,omitempty"`
	ShortDescription string         `json:"shortDescription,omitempty"`
	Date             string         `json:"date,omitempty"`
	EndDate          string         `json:"endDate,omitempty"`
	MatchState       string         `json:"matchState,omitempty"`
	VenueName        string         `json:"venueName,omitempty"`
	VenueSummary     string         `json:"venueSummary,omitempty"`
	ScoreSummary     string         `json:"scoreSummary,omitempty"`
	StatusRef        string         `json:"statusRef,omitempty"`
	DetailsRef       string         `json:"detailsRef,omitempty"`
	MatchcardsRef    string         `json:"matchcardsRef,omitempty"`
	SituationRef     string         `json:"situationRef,omitempty"`
	OfficialsRef     string         `json:"officialsRef,omitempty"`
	BroadcastsRef    string         `json:"broadcastsRef,omitempty"`
	TicketsRef       string         `json:"ticketsRef,omitempty"`
	OddsRef          string         `json:"oddsRef,omitempty"`
	Teams            []Team         `json:"teams,omitempty"`
	Extensions       map[string]any `json:"extensions,omitempty"`
}

// CompetitionMetadataEntry is a normalized row from officials/broadcasts/tickets/odds resources.
type CompetitionMetadataEntry struct {
	Ref         string         `json:"ref,omitempty"`
	ID          string         `json:"id,omitempty"`
	DisplayName string         `json:"displayName,omitempty"`
	Name        string         `json:"name,omitempty"`
	Role        string         `json:"role,omitempty"`
	Type        string         `json:"type,omitempty"`
	Order       int            `json:"order,omitempty"`
	Text        string         `json:"text,omitempty"`
	Value       string         `json:"value,omitempty"`
	Href        string         `json:"href,omitempty"`
	Extensions  map[string]any `json:"extensions,omitempty"`
}

// CompetitionMetadataSummary aggregates auxiliary competition metadata routes.
type CompetitionMetadataSummary struct {
	Competition Competition                `json:"competition"`
	Officials   []CompetitionMetadataEntry `json:"officials,omitempty"`
	Broadcasts  []CompetitionMetadataEntry `json:"broadcasts,omitempty"`
	Tickets     []CompetitionMetadataEntry `json:"tickets,omitempty"`
	Odds        []CompetitionMetadataEntry `json:"odds,omitempty"`
}

// Player is the normalized core player shape.
type Player struct {
	Ref                  string              `json:"ref,omitempty"`
	ID                   string              `json:"id,omitempty"`
	UID                  string              `json:"uid,omitempty"`
	GUID                 string              `json:"guid,omitempty"`
	Type                 string              `json:"type,omitempty"`
	Name                 string              `json:"name,omitempty"`
	FirstName            string              `json:"firstName,omitempty"`
	MiddleName           string              `json:"middleName,omitempty"`
	LastName             string              `json:"lastName,omitempty"`
	DisplayName          string              `json:"displayName,omitempty"`
	FullName             string              `json:"fullName,omitempty"`
	ShortName            string              `json:"shortName,omitempty"`
	BattingName          string              `json:"battingName,omitempty"`
	FieldingName         string              `json:"fieldingName,omitempty"`
	Gender               string              `json:"gender,omitempty"`
	Age                  int                 `json:"age,omitempty"`
	DateOfBirth          string              `json:"dateOfBirth,omitempty"`
	DateOfBirthDisplay   string              `json:"dateOfBirthDisplay,omitempty"`
	Active               bool                `json:"active"`
	Position             string              `json:"position,omitempty"`
	PositionRef          string              `json:"positionRef,omitempty"`
	PositionAbbreviation string              `json:"positionAbbreviation,omitempty"`
	Styles               []PlayerStyle       `json:"styles,omitempty"`
	Team                 *PlayerAffiliation  `json:"team,omitempty"`
	MajorTeams           []PlayerAffiliation `json:"majorTeams,omitempty"`
	Debuts               []PlayerDebut       `json:"debuts,omitempty"`
	NewsRef              string              `json:"newsRef,omitempty"`
	Extensions           map[string]any      `json:"extensions,omitempty"`
}

// PlayerStyle captures batting/bowling handedness or discipline metadata.
type PlayerStyle struct {
	Type             string `json:"type,omitempty"`
	Description      string `json:"description,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
}

// PlayerAffiliation captures a player-team relationship from profile payloads.
type PlayerAffiliation struct {
	ID   string `json:"id,omitempty"`
	Ref  string `json:"ref,omitempty"`
	Name string `json:"name,omitempty"`
}

// PlayerDebut captures a debut reference exposed by the athlete profile.
type PlayerDebut struct {
	ID   string `json:"id,omitempty"`
	Ref  string `json:"ref,omitempty"`
	Name string `json:"name,omitempty"`
}

// PlayerStatistics keeps the upstream grouped split/category structure intact.
type PlayerStatistics struct {
	Ref          string         `json:"ref,omitempty"`
	PlayerID     string         `json:"playerId,omitempty"`
	PlayerRef    string         `json:"playerRef,omitempty"`
	SplitID      string         `json:"splitId,omitempty"`
	Name         string         `json:"name,omitempty"`
	Abbreviation string         `json:"abbreviation,omitempty"`
	Categories   []StatCategory `json:"categories,omitempty"`
	Extensions   map[string]any `json:"extensions,omitempty"`
}

// PlayerMatch summarizes player-in-match context from roster stats and innings/detail routes.
type PlayerMatch struct {
	PlayerID      string             `json:"playerId,omitempty"`
	PlayerRef     string             `json:"playerRef,omitempty"`
	PlayerName    string             `json:"playerName,omitempty"`
	MatchID       string             `json:"matchId,omitempty"`
	CompetitionID string             `json:"competitionId,omitempty"`
	EventID       string             `json:"eventId,omitempty"`
	LeagueID      string             `json:"leagueId,omitempty"`
	TeamID        string             `json:"teamId,omitempty"`
	TeamName      string             `json:"teamName,omitempty"`
	StatisticsRef string             `json:"statisticsRef,omitempty"`
	LinescoresRef string             `json:"linescoresRef,omitempty"`
	Batting       []StatCategory     `json:"batting,omitempty"`
	Bowling       []StatCategory     `json:"bowling,omitempty"`
	Fielding      []StatCategory     `json:"fielding,omitempty"`
	Summary       PlayerMatchSummary `json:"summary,omitempty"`
	Extensions    map[string]any     `json:"extensions,omitempty"`
}

// PlayerMatchSummary exposes high-value batting/bowling and dismissal fields for agent reasoning.
type PlayerMatchSummary struct {
	DismissalName   string  `json:"dismissalName,omitempty"`
	DismissalCard   string  `json:"dismissalCard,omitempty"`
	BallsFaced      int     `json:"ballsFaced,omitempty"`
	StrikeRate      float64 `json:"strikeRate,omitempty"`
	Dots            int     `json:"dots,omitempty"`
	EconomyRate     float64 `json:"economyRate,omitempty"`
	Maidens         int     `json:"maidens,omitempty"`
	FoursConceded   int     `json:"foursConceded,omitempty"`
	SixesConceded   int     `json:"sixesConceded,omitempty"`
	Wides           int     `json:"wides,omitempty"`
	Noballs         int     `json:"noballs,omitempty"`
	BowlerPlayerID  string  `json:"bowlerPlayerId,omitempty"`
	FielderPlayerID string  `json:"fielderPlayerId,omitempty"`
}

// PlayerInnings is a normalized player-specific innings split row.
type PlayerInnings struct {
	Ref           string             `json:"ref,omitempty"`
	PlayerID      string             `json:"playerId,omitempty"`
	PlayerName    string             `json:"playerName,omitempty"`
	MatchID       string             `json:"matchId,omitempty"`
	CompetitionID string             `json:"competitionId,omitempty"`
	EventID       string             `json:"eventId,omitempty"`
	LeagueID      string             `json:"leagueId,omitempty"`
	TeamID        string             `json:"teamId,omitempty"`
	TeamName      string             `json:"teamName,omitempty"`
	InningsNumber int                `json:"inningsNumber,omitempty"`
	Period        int                `json:"period,omitempty"`
	Order         int                `json:"order,omitempty"`
	IsBatting     bool               `json:"isBatting"`
	StatisticsRef string             `json:"statisticsRef,omitempty"`
	Batting       []StatCategory     `json:"batting,omitempty"`
	Bowling       []StatCategory     `json:"bowling,omitempty"`
	Fielding      []StatCategory     `json:"fielding,omitempty"`
	Summary       PlayerMatchSummary `json:"summary,omitempty"`
	Extensions    map[string]any     `json:"extensions,omitempty"`
}

// PlayerDismissal is a dismissal-focused first-class output view.
type PlayerDismissal struct {
	PlayerID        string  `json:"playerId,omitempty"`
	PlayerName      string  `json:"playerName,omitempty"`
	MatchID         string  `json:"matchId,omitempty"`
	CompetitionID   string  `json:"competitionId,omitempty"`
	EventID         string  `json:"eventId,omitempty"`
	LeagueID        string  `json:"leagueId,omitempty"`
	TeamID          string  `json:"teamId,omitempty"`
	TeamName        string  `json:"teamName,omitempty"`
	InningsNumber   int     `json:"inningsNumber,omitempty"`
	Period          int     `json:"period,omitempty"`
	WicketNumber    int     `json:"wicketNumber,omitempty"`
	FOW             string  `json:"fow,omitempty"`
	Over            string  `json:"over,omitempty"`
	DetailRef       string  `json:"detailRef,omitempty"`
	DetailShortText string  `json:"detailShortText,omitempty"`
	DetailText      string  `json:"detailText,omitempty"`
	DismissalName   string  `json:"dismissalName,omitempty"`
	DismissalCard   string  `json:"dismissalCard,omitempty"`
	DismissalType   string  `json:"dismissalType,omitempty"`
	DismissalText   string  `json:"dismissalText,omitempty"`
	BallsFaced      int     `json:"ballsFaced,omitempty"`
	StrikeRate      float64 `json:"strikeRate,omitempty"`
	BatsmanPlayerID string  `json:"batsmanPlayerId,omitempty"`
	BowlerPlayerID  string  `json:"bowlerPlayerId,omitempty"`
	FielderPlayerID string  `json:"fielderPlayerId,omitempty"`
}

// NewsArticle is a normalized Cricinfo article/story payload.
type NewsArticle struct {
	Ref          string         `json:"ref,omitempty"`
	ID           string         `json:"id,omitempty"`
	UID          string         `json:"uid,omitempty"`
	Type         string         `json:"type,omitempty"`
	Headline     string         `json:"headline,omitempty"`
	Title        string         `json:"title,omitempty"`
	LinkText     string         `json:"linkText,omitempty"`
	Byline       string         `json:"byline,omitempty"`
	Description  string         `json:"description,omitempty"`
	Published    string         `json:"published,omitempty"`
	LastModified string         `json:"lastModified,omitempty"`
	WebURL       string         `json:"webUrl,omitempty"`
	APIURL       string         `json:"apiUrl,omitempty"`
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
	TeamName      string         `json:"teamName,omitempty"`
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
	TeamName   string               `json:"teamName,omitempty"`
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

// CalendarDay is the normalized league calendar day shape.
type CalendarDay struct {
	Ref        string         `json:"ref,omitempty"`
	LeagueID   string         `json:"leagueId,omitempty"`
	Date       string         `json:"date,omitempty"`
	DayType    string         `json:"dayType,omitempty"`
	StartDate  string         `json:"startDate,omitempty"`
	EndDate    string         `json:"endDate,omitempty"`
	Sections   []string       `json:"sections,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// SeasonType is the normalized season type shape.
type SeasonType struct {
	Ref          string         `json:"ref,omitempty"`
	ID           string         `json:"id,omitempty"`
	LeagueID     string         `json:"leagueId,omitempty"`
	SeasonID     string         `json:"seasonId,omitempty"`
	Name         string         `json:"name,omitempty"`
	Abbreviation string         `json:"abbreviation,omitempty"`
	StartDate    string         `json:"startDate,omitempty"`
	EndDate      string         `json:"endDate,omitempty"`
	HasGroups    bool           `json:"hasGroups"`
	HasStandings bool           `json:"hasStandings"`
	GroupsRef    string         `json:"groupsRef,omitempty"`
	Extensions   map[string]any `json:"extensions,omitempty"`
}

// SeasonGroup is the normalized season group shape.
type SeasonGroup struct {
	Ref          string         `json:"ref,omitempty"`
	ID           string         `json:"id,omitempty"`
	LeagueID     string         `json:"leagueId,omitempty"`
	SeasonID     string         `json:"seasonId,omitempty"`
	TypeID       string         `json:"typeId,omitempty"`
	Name         string         `json:"name,omitempty"`
	Abbreviation string         `json:"abbreviation,omitempty"`
	StandingsRef string         `json:"standingsRef,omitempty"`
	Extensions   map[string]any `json:"extensions,omitempty"`
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
	StrikeRate      float64        `json:"strikeRate,omitempty"`
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
	Ref                 string         `json:"ref,omitempty"`
	ID                  string         `json:"id,omitempty"`
	LeagueID            string         `json:"leagueId,omitempty"`
	EventID             string         `json:"eventId,omitempty"`
	CompetitionID       string         `json:"competitionId,omitempty"`
	MatchID             string         `json:"matchId,omitempty"`
	TeamID              string         `json:"teamId,omitempty"`
	Period              int            `json:"period,omitempty"`
	PeriodText          string         `json:"periodText,omitempty"`
	OverNumber          int            `json:"overNumber,omitempty"`
	BallNumber          int            `json:"ballNumber,omitempty"`
	ScoreValue          int            `json:"scoreValue,omitempty"`
	ShortText           string         `json:"shortText,omitempty"`
	Text                string         `json:"text,omitempty"`
	HomeScore           string         `json:"homeScore,omitempty"`
	AwayScore           string         `json:"awayScore,omitempty"`
	BatsmanRef          string         `json:"batsmanRef,omitempty"`
	BowlerRef           string         `json:"bowlerRef,omitempty"`
	OtherBatsmanRef     string         `json:"otherBatsmanRef,omitempty"`
	OtherBowlerRef      string         `json:"otherBowlerRef,omitempty"`
	BatsmanPlayerID     string         `json:"batsmanPlayerId,omitempty"`
	BowlerPlayerID      string         `json:"bowlerPlayerId,omitempty"`
	OtherBatsmanID      string         `json:"otherBatsmanPlayerId,omitempty"`
	OtherBowlerID       string         `json:"otherBowlerPlayerId,omitempty"`
	FielderPlayerID     string         `json:"fielderPlayerId,omitempty"`
	AthletePlayerIDs    []string       `json:"athletePlayerIds,omitempty"`
	Involvement         []string       `json:"involvement,omitempty"`
	BatsmanRuns         int            `json:"batsmanRuns,omitempty"`
	BatsmanTotalRuns    int            `json:"batsmanTotalRuns,omitempty"`
	BatsmanBalls        int            `json:"batsmanBalls,omitempty"`
	BatsmanFours        int            `json:"batsmanFours,omitempty"`
	BatsmanSixes        int            `json:"batsmanSixes,omitempty"`
	OtherBatterRuns     int            `json:"otherBatterRuns,omitempty"`
	OtherBatterBalls    int            `json:"otherBatterBalls,omitempty"`
	OtherBatterFours    int            `json:"otherBatterFours,omitempty"`
	OtherBatterSixes    int            `json:"otherBatterSixes,omitempty"`
	BowlerOvers         float64        `json:"bowlerOvers,omitempty"`
	BowlerBalls         int            `json:"bowlerBalls,omitempty"`
	BowlerMaidens       int            `json:"bowlerMaidens,omitempty"`
	BowlerConceded      int            `json:"bowlerConceded,omitempty"`
	BowlerWickets       int            `json:"bowlerWickets,omitempty"`
	OtherBowlerOvers    float64        `json:"otherBowlerOvers,omitempty"`
	OtherBowlerBalls    int            `json:"otherBowlerBalls,omitempty"`
	OtherBowlerMaidens  int            `json:"otherBowlerMaidens,omitempty"`
	OtherBowlerConceded int            `json:"otherBowlerConceded,omitempty"`
	OtherBowlerWickets  int            `json:"otherBowlerWickets,omitempty"`
	Sequence            int            `json:"sequence,omitempty"`
	PlayType            map[string]any `json:"playType,omitempty"`
	Dismissal           map[string]any `json:"dismissal,omitempty"`
	DismissalType       string         `json:"dismissalType,omitempty"`
	DismissalName       string         `json:"dismissalName,omitempty"`
	DismissalCard       string         `json:"dismissalCard,omitempty"`
	DismissalText       string         `json:"dismissalText,omitempty"`
	SpeedKPH            float64        `json:"speedKPH,omitempty"`
	XCoordinate         *float64       `json:"xCoordinate"`
	YCoordinate         *float64       `json:"yCoordinate"`
	BBBTimestamp        int64          `json:"bbbTimestamp"`
	CoordinateX         *float64       `json:"coordinateX,omitempty"`
	CoordinateY         *float64       `json:"coordinateY,omitempty"`
	Timestamp           int64          `json:"timestamp,omitempty"`
	Extensions          map[string]any `json:"extensions,omitempty"`
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

// AnalysisScope captures the resolved analysis traversal scope.
type AnalysisScope struct {
	Mode              string           `json:"mode"`
	RequestedLeagueID string           `json:"requestedLeagueId,omitempty"`
	LeagueID          string           `json:"leagueId,omitempty"`
	LeagueName        string           `json:"leagueName,omitempty"`
	Seasons           []string         `json:"seasons,omitempty"`
	MatchIDs          []string         `json:"matchIds,omitempty"`
	MatchCount        int              `json:"matchCount"`
	DateFrom          string           `json:"dateFrom,omitempty"`
	DateTo            string           `json:"dateTo,omitempty"`
	TypeQuery         string           `json:"type,omitempty"`
	GroupQuery        string           `json:"group,omitempty"`
	HydrationMetric   HydrationMetrics `json:"hydrationMetrics,omitempty"`
}

// AnalysisFilters captures user-level row filters.
type AnalysisFilters struct {
	TeamQuery     string `json:"team,omitempty"`
	PlayerQuery   string `json:"player,omitempty"`
	DismissalType string `json:"dismissalType,omitempty"`
	Innings       int    `json:"innings,omitempty"`
	Period        int    `json:"period,omitempty"`
}

// AnalysisRow is one ranked row in an analysis response.
type AnalysisRow struct {
	Rank          int            `json:"rank"`
	Key           string         `json:"key"`
	Metric        string         `json:"metric,omitempty"`
	Value         float64        `json:"value"`
	Count         int            `json:"count,omitempty"`
	Matches       int            `json:"matches,omitempty"`
	PlayerID      string         `json:"playerId,omitempty"`
	PlayerName    string         `json:"playerName,omitempty"`
	TeamID        string         `json:"teamId,omitempty"`
	TeamName      string         `json:"teamName,omitempty"`
	LeagueID      string         `json:"leagueId,omitempty"`
	SeasonID      string         `json:"seasonId,omitempty"`
	DismissalType string         `json:"dismissalType,omitempty"`
	InningsNumber int            `json:"inningsNumber,omitempty"`
	Period        int            `json:"period,omitempty"`
	Extras        map[string]any `json:"extras,omitempty"`
}

// AnalysisView is the stable agent-friendly analysis payload.
type AnalysisView struct {
	Command string          `json:"command"`
	Metric  string          `json:"metric,omitempty"`
	Scope   AnalysisScope   `json:"scope"`
	GroupBy []string        `json:"groupBy"`
	Filters AnalysisFilters `json:"filters,omitempty"`
	Rows    []AnalysisRow   `json:"rows"`
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
	case EntityMatchPhases:
		return "match phase reports"
	case EntityCompetition:
		return "competitions"
	case EntityCompOfficial:
		return "competition officials"
	case EntityCompBroadcast:
		return "competition broadcasts"
	case EntityCompTicket:
		return "competition tickets"
	case EntityCompOdds:
		return "competition odds"
	case EntityCompMetadata:
		return "competition metadata views"
	case EntityPlayer:
		return "players"
	case EntityPlayerStats:
		return "player statistics"
	case EntityPlayerMatch:
		return "player match views"
	case EntityPlayerInnings:
		return "player innings"
	case EntityPlayerDismissal:
		return "player dismissals"
	case EntityPlayerDelivery:
		return "player deliveries"
	case EntityNewsArticle:
		return "news articles"
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
	case EntityCalendarDay:
		return "calendar days"
	case EntitySeasonType:
		return "season types"
	case EntitySeasonGroup:
		return "season groups"
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
	case EntityAnalysisDismiss:
		return "analysis dismissals"
	case EntityAnalysisBowl:
		return "analysis bowling rows"
	case EntityAnalysisBat:
		return "analysis batting rows"
	case EntityAnalysisPart:
		return "analysis partnerships"
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
		case "types":
			ids["typeId"] = value
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
