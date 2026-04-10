package cricinfo

// TemplateCoverageEntry maps a researched endpoint template to public command coverage.
type TemplateCoverageEntry struct {
	Template      string
	CommandFamily string
	Command       string
	View          string
}

// FieldPathCoverageEntry maps a field-path family to public command coverage.
type FieldPathCoverageEntry struct {
	Family        string
	CommandFamily string
	Command       string
	View          string
}

var templateCoverageLedger = map[string]TemplateCoverageEntry{
	"/":                                           {Template: "/", CommandFamily: "leagues", Command: "leagues list", View: "root discovery seed"},
	"/athletes":                                   {Template: "/athletes", CommandFamily: "search", Command: "search players", View: "global athlete discovery seed"},
	"/athletes/{id}":                              {Template: "/athletes/{id}", CommandFamily: "players", Command: "players profile", View: "player profile"},
	"/athletes/{id}/news":                         {Template: "/athletes/{id}/news", CommandFamily: "players", Command: "players news", View: "player news list"},
	"/athletes/{id}/statistics":                   {Template: "/athletes/{id}/statistics", CommandFamily: "players", Command: "players stats", View: "player statistics categories"},
	"/events":                                     {Template: "/events", CommandFamily: "matches", Command: "matches list", View: "event discovery"},
	"/events/{id}":                                {Template: "/events/{id}", CommandFamily: "matches", Command: "matches show", View: "event competition expansion"},
	"/events/{id}/competitions/{id}":              {Template: "/events/{id}/competitions/{id}", CommandFamily: "competitions", Command: "competitions show", View: "competition summary"},
	"/events/{id}/teams/{id}":                     {Template: "/events/{id}/teams/{id}", CommandFamily: "teams", Command: "teams show", View: "event-team identity subview"},
	"/leagues":                                    {Template: "/leagues", CommandFamily: "leagues", Command: "leagues list", View: "league discovery"},
	"/leagues/{id}":                               {Template: "/leagues/{id}", CommandFamily: "leagues", Command: "leagues show", View: "league summary"},
	"/leagues/{id}/athletes/{id}":                 {Template: "/leagues/{id}/athletes/{id}", CommandFamily: "leagues", Command: "leagues athletes", View: "league athlete profile"},
	"/leagues/{id}/athletes/{n}":                  {Template: "/leagues/{id}/athletes/{n}", CommandFamily: "leagues", Command: "leagues athletes", View: "league athlete index page"},
	"/leagues/{id}/calendar":                      {Template: "/leagues/{id}/calendar", CommandFamily: "leagues", Command: "leagues calendar", View: "calendar root"},
	"/leagues/{id}/calendar/offdays":              {Template: "/leagues/{id}/calendar/offdays", CommandFamily: "leagues", Command: "leagues calendar", View: "calendar offdays section"},
	"/leagues/{id}/calendar/ondays":               {Template: "/leagues/{id}/calendar/ondays", CommandFamily: "leagues", Command: "leagues calendar", View: "calendar ondays section"},
	"/leagues/{id}/events":                        {Template: "/leagues/{id}/events", CommandFamily: "leagues", Command: "leagues events", View: "league event list"},
	"/leagues/{id}/events/{id}":                   {Template: "/leagues/{id}/events/{id}", CommandFamily: "leagues", Command: "leagues events", View: "event expansion"},
	"/leagues/{id}/events/{id}/competitions/{id}": {Template: "/leagues/{id}/events/{id}/competitions/{id}", CommandFamily: "competitions", Command: "competitions show", View: "competition summary"},
	"/leagues/{id}/events/{id}/competitions/{id}/broadcasts":                                                     {Template: "/leagues/{id}/events/{id}/competitions/{id}/broadcasts", CommandFamily: "competitions", Command: "competitions broadcasts", View: "competition broadcasts"},
	"/leagues/{id}/events/{id}/competitions/{id}/details":                                                        {Template: "/leagues/{id}/events/{id}/competitions/{id}/details", CommandFamily: "matches", Command: "matches details", View: "delivery ref page"},
	"/leagues/{id}/events/{id}/competitions/{id}/details/{id}":                                                   {Template: "/leagues/{id}/events/{id}/competitions/{id}/details/{id}", CommandFamily: "matches", Command: "matches details", View: "delivery event detail"},
	"/leagues/{id}/events/{id}/competitions/{id}/details/{n}":                                                    {Template: "/leagues/{id}/events/{id}/competitions/{id}/details/{n}", CommandFamily: "matches", Command: "matches details", View: "delivery event detail"},
	"/leagues/{id}/events/{id}/competitions/{id}/matchcards":                                                     {Template: "/leagues/{id}/events/{id}/competitions/{id}/matchcards", CommandFamily: "matches", Command: "matches scorecard", View: "batting/bowling/partnership cards"},
	"/leagues/{id}/events/{id}/competitions/{id}/odds":                                                           {Template: "/leagues/{id}/events/{id}/competitions/{id}/odds", CommandFamily: "competitions", Command: "competitions odds", View: "competition odds"},
	"/leagues/{id}/events/{id}/competitions/{id}/officials":                                                      {Template: "/leagues/{id}/events/{id}/competitions/{id}/officials", CommandFamily: "competitions", Command: "competitions officials", View: "competition officials"},
	"/leagues/{id}/events/{id}/competitions/{id}/plays":                                                          {Template: "/leagues/{id}/events/{id}/competitions/{id}/plays", CommandFamily: "matches", Command: "matches plays", View: "play-derived delivery events"},
	"/leagues/{id}/events/{id}/competitions/{id}/situation":                                                      {Template: "/leagues/{id}/events/{id}/competitions/{id}/situation", CommandFamily: "matches", Command: "matches situation", View: "match situation"},
	"/leagues/{id}/events/{id}/competitions/{id}/situation/odds":                                                 {Template: "/leagues/{id}/events/{id}/competitions/{id}/situation/odds", CommandFamily: "matches", Command: "matches situation", View: "situation odds subview"},
	"/leagues/{id}/events/{id}/competitions/{id}/status":                                                         {Template: "/leagues/{id}/events/{id}/competitions/{id}/status", CommandFamily: "matches", Command: "matches status", View: "match status"},
	"/leagues/{id}/events/{id}/competitions/{id}/tickets":                                                        {Template: "/leagues/{id}/events/{id}/competitions/{id}/tickets", CommandFamily: "competitions", Command: "competitions tickets", View: "competition tickets"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}":                                               {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}", CommandFamily: "teams", Command: "teams show --match <match>", View: "competitor summary"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/leaders":                                       {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/leaders", CommandFamily: "teams", Command: "teams leaders --match <match>", View: "team leaders"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores":                                    {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores", CommandFamily: "matches", Command: "matches innings", View: "team innings list"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}":                                {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}", CommandFamily: "matches", Command: "matches innings", View: "innings pointer"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}":                            {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}", CommandFamily: "matches", Command: "matches deliveries", View: "period innings detail"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/fow":                        {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/fow", CommandFamily: "matches", Command: "matches fow", View: "fall-of-wicket list"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/fow/{n}":                    {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/fow/{n}", CommandFamily: "matches", Command: "matches fow", View: "fall-of-wicket detail"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/leaders":                    {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/leaders", CommandFamily: "matches", Command: "matches deliveries", View: "period leaders subview"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/partnerships":               {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/partnerships", CommandFamily: "matches", Command: "matches partnerships", View: "partnership list"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/partnerships/{n}":           {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/partnerships/{n}", CommandFamily: "matches", Command: "matches partnerships", View: "partnership detail"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/statistics/{n}":             {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/statistics/{n}", CommandFamily: "matches", Command: "matches deliveries", View: "period statistics timelines"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/records":                                       {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/records", CommandFamily: "teams", Command: "teams records --match <match>", View: "team records categories"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster":                                        {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster", CommandFamily: "teams", Command: "teams roster --match <match>", View: "match roster entries"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/linescores":                        {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/linescores", CommandFamily: "players", Command: "players innings --match <match>", View: "player innings list"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/linescores/{n}/{n}/statistics/{n}": {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/linescores/{n}/{n}/statistics/{n}", CommandFamily: "players", Command: "players innings --match <match>", View: "player innings period statistics"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/statistics/{n}":                    {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/statistics/{n}", CommandFamily: "players", Command: "players match-stats --match <match>", View: "player match statistics"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/scores":                                        {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/scores", CommandFamily: "teams", Command: "teams scores --match <match>", View: "team score"},
	"/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/statistics":                                    {Template: "/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/statistics", CommandFamily: "teams", Command: "teams statistics --match <match>", View: "team statistics categories"},
	"/leagues/{id}/seasons":                       {Template: "/leagues/{id}/seasons", CommandFamily: "leagues", Command: "leagues seasons", View: "season list"},
	"/leagues/{id}/seasons/{id}":                  {Template: "/leagues/{id}/seasons/{id}", CommandFamily: "seasons", Command: "seasons show", View: "season detail"},
	"/leagues/{id}/seasons/{id}/types":            {Template: "/leagues/{id}/seasons/{id}/types", CommandFamily: "seasons", Command: "seasons types", View: "season type list"},
	"/leagues/{id}/seasons/{id}/types/{n}":        {Template: "/leagues/{id}/seasons/{id}/types/{n}", CommandFamily: "seasons", Command: "seasons types", View: "season type detail"},
	"/leagues/{id}/seasons/{id}/types/{n}/groups": {Template: "/leagues/{id}/seasons/{id}/types/{n}/groups", CommandFamily: "seasons", Command: "seasons groups", View: "season group list"},
	"/leagues/{id}/standings":                     {Template: "/leagues/{id}/standings", CommandFamily: "standings", Command: "standings show", View: "standings groups"},
	"/teams/{id}":                                 {Template: "/teams/{id}", CommandFamily: "teams", Command: "teams show", View: "global team profile"},
}

var fieldPathFamilyCoverageLedger = map[string]FieldPathCoverageEntry{
	"athlete":          {Family: "athlete", CommandFamily: "players", Command: "players profile", View: "player identity"},
	"athletesInvolved": {Family: "athletesInvolved", CommandFamily: "matches", Command: "matches details", View: "delivery athlete involvement"},
	"batsman":          {Family: "batsman", CommandFamily: "matches", Command: "matches details", View: "delivery batting context"},
	"bowler":           {Family: "bowler", CommandFamily: "matches", Command: "matches details", View: "delivery bowling context"},
	"broadcasts":       {Family: "broadcasts", CommandFamily: "competitions", Command: "competitions broadcasts", View: "competition broadcasts"},
	"competitions":     {Family: "competitions", CommandFamily: "competitions", Command: "competitions metadata", View: "competition metadata root"},
	"competitors":      {Family: "competitors", CommandFamily: "teams", Command: "teams show --match <match>", View: "match competitor view"},
	"details":          {Family: "details", CommandFamily: "matches", Command: "matches details", View: "delivery detail refs"},
	"dismissal":        {Family: "dismissal", CommandFamily: "players", Command: "players dismissals --match <match>", View: "dismissal metadata"},
	"entries":          {Family: "entries", CommandFamily: "teams", Command: "teams roster --match <match>", View: "roster entries"},
	"fow":              {Family: "fow", CommandFamily: "matches", Command: "matches fow", View: "fall-of-wicket timeline"},
	"innings":          {Family: "innings", CommandFamily: "matches", Command: "matches innings", View: "innings summary"},
	"items":            {Family: "items", CommandFamily: "matches", Command: "matches list", View: "page envelope items"},
	"leagues":          {Family: "leagues", CommandFamily: "leagues", Command: "leagues show", View: "league hierarchy"},
	"matchcards":       {Family: "matchcards", CommandFamily: "matches", Command: "matches scorecard", View: "scorecard cards"},
	"odds":             {Family: "odds", CommandFamily: "competitions", Command: "competitions odds", View: "competition odds"},
	"officials":        {Family: "officials", CommandFamily: "competitions", Command: "competitions officials", View: "competition officials"},
	"over":             {Family: "over", CommandFamily: "matches", Command: "matches deliveries", View: "over timeline"},
	"partnerships":     {Family: "partnerships", CommandFamily: "matches", Command: "matches partnerships", View: "partnership timeline"},
	"seasons":          {Family: "seasons", CommandFamily: "seasons", Command: "seasons groups", View: "season hierarchy"},
	"situation":        {Family: "situation", CommandFamily: "matches", Command: "matches situation", View: "situation snapshot"},
	"splits":           {Family: "splits", CommandFamily: "players", Command: "players stats", View: "statistics splits and categories"},
	"status":           {Family: "status", CommandFamily: "matches", Command: "matches status", View: "status summary"},
	"teams":            {Family: "teams", CommandFamily: "teams", Command: "teams show", View: "team identity"},
	"tickets":          {Family: "tickets", CommandFamily: "competitions", Command: "competitions tickets", View: "competition tickets"},
}

// TemplateCoverageLedger returns a copy of the researched-template coverage ledger.
func TemplateCoverageLedger() map[string]TemplateCoverageEntry {
	out := make(map[string]TemplateCoverageEntry, len(templateCoverageLedger))
	for template, entry := range templateCoverageLedger {
		out[template] = entry
	}
	return out
}

// FieldPathFamilyCoverageLedger returns a copy of the field-family coverage ledger.
func FieldPathFamilyCoverageLedger() map[string]FieldPathCoverageEntry {
	out := make(map[string]FieldPathCoverageEntry, len(fieldPathFamilyCoverageLedger))
	for family, entry := range fieldPathFamilyCoverageLedger {
		out[family] = entry
	}
	return out
}
