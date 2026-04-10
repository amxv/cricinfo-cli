package cricinfo

import (
	"fmt"
	"sort"
	"strings"
)

// FixtureFamily groups fixtures and live probes by API resource family.
type FixtureFamily string

const (
	FixtureFamilyRootDiscovery      FixtureFamily = "root-discovery"
	FixtureFamilyMatchesCompetition FixtureFamily = "matches-competitions"
	FixtureFamilyDetailsPlays       FixtureFamily = "details-plays"
	FixtureFamilyTeamCompetitor     FixtureFamily = "team-competitor"
	FixtureFamilyInningsDepth       FixtureFamily = "innings-fow-partnerships"
	FixtureFamilyPlayers            FixtureFamily = "players"
	FixtureFamilyLeagueSeason       FixtureFamily = "leagues-seasons-standings"
	FixtureFamilyAuxCompetitionMeta FixtureFamily = "aux-competition-metadata"
)

// FixtureSpec defines a curated fixture/live endpoint pairing.
type FixtureSpec struct {
	Family      FixtureFamily
	Name        string
	Ref         string
	FixturePath string
	LiveProbe   bool
}

var fixtureSpecs = []FixtureSpec{
	{
		Family:      FixtureFamilyRootDiscovery,
		Name:        "root",
		Ref:         "/v2/sports/cricket",
		FixturePath: "root-discovery/root.json",
		LiveProbe:   true,
	},
	{
		Family:      FixtureFamilyRootDiscovery,
		Name:        "events-page",
		Ref:         "/events",
		FixturePath: "root-discovery/events.json",
		LiveProbe:   false,
	},
	{
		Family:      FixtureFamilyMatchesCompetition,
		Name:        "competition",
		Ref:         "/leagues/19138/events/1529474/competitions/1529474",
		FixturePath: "matches-competitions/competition.json",
		LiveProbe:   true,
	},
	{
		Family:      FixtureFamilyDetailsPlays,
		Name:        "plays-page",
		Ref:         "/leagues/19138/events/1529474/competitions/1529474/plays?limit=1",
		FixturePath: "details-plays/plays.json",
		LiveProbe:   true,
	},
	{
		Family:      FixtureFamilyDetailsPlays,
		Name:        "detail-item",
		Ref:         "/leagues/19138/events/1529474/competitions/1529474/details/110",
		FixturePath: "details-plays/detail-110.json",
		LiveProbe:   false,
	},
	{
		Family:      FixtureFamilyTeamCompetitor,
		Name:        "competitor",
		Ref:         "/leagues/19138/events/1529474/competitions/1529474/competitors/789643",
		FixturePath: "team-competitor/competitor-789643.json",
		LiveProbe:   true,
	},
	{
		Family:      FixtureFamilyTeamCompetitor,
		Name:        "competitor-roster",
		Ref:         "/leagues/11132/events/1475396/competitions/1475396/competitors/1147772/roster",
		FixturePath: "team-competitor/roster-1147772.json",
		LiveProbe:   false,
	},
	{
		Family:      FixtureFamilyInningsDepth,
		Name:        "innings-period",
		Ref:         "/leagues/1098952/events/1475396/competitions/1475396/competitors/1147772/linescores/1/2",
		FixturePath: "innings-fow-partnerships/innings-1-2.json",
		LiveProbe:   true,
	},
	{
		Family:      FixtureFamilyInningsDepth,
		Name:        "fow",
		Ref:         "/leagues/1098952/events/1475396/competitions/1475396/competitors/1147772/linescores/1/2/fow",
		FixturePath: "innings-fow-partnerships/fow.json",
		LiveProbe:   false,
	},
	{
		Family:      FixtureFamilyInningsDepth,
		Name:        "partnerships",
		Ref:         "/leagues/1098952/events/1475396/competitions/1475396/competitors/1147772/linescores/1/2/partnerships",
		FixturePath: "innings-fow-partnerships/partnerships.json",
		LiveProbe:   false,
	},
	{
		Family:      FixtureFamilyPlayers,
		Name:        "athlete-profile",
		Ref:         "/athletes/1361257",
		FixturePath: "players/athlete-1361257.json",
		LiveProbe:   true,
	},
	{
		Family:      FixtureFamilyPlayers,
		Name:        "athlete-statistics",
		Ref:         "/athletes/1361257/statistics",
		FixturePath: "players/athlete-1361257-statistics.json",
		LiveProbe:   false,
	},
	{
		Family:      FixtureFamilyLeagueSeason,
		Name:        "standings",
		Ref:         "/leagues/19138/standings",
		FixturePath: "leagues-seasons-standings/standings.json",
		LiveProbe:   true,
	},
	{
		Family:      FixtureFamilyLeagueSeason,
		Name:        "seasons",
		Ref:         "/leagues/19138/seasons",
		FixturePath: "leagues-seasons-standings/seasons.json",
		LiveProbe:   false,
	},
	{
		Family:      FixtureFamilyAuxCompetitionMeta,
		Name:        "officials",
		Ref:         "/leagues/11132/events/1527944/competitions/1527944/officials",
		FixturePath: "aux-competition-metadata/officials.json",
		LiveProbe:   true,
	},
	{
		Family:      FixtureFamilyAuxCompetitionMeta,
		Name:        "broadcasts",
		Ref:         "/leagues/11132/events/1527944/competitions/1527944/broadcasts",
		FixturePath: "aux-competition-metadata/broadcasts.json",
		LiveProbe:   false,
	},
}

// FixtureMatrix returns a copy of the curated fixture endpoint matrix.
func FixtureMatrix() []FixtureSpec {
	out := make([]FixtureSpec, len(fixtureSpecs))
	copy(out, fixtureSpecs)
	return out
}

// FixtureFamilies returns sorted family names represented in the matrix.
func FixtureFamilies() []FixtureFamily {
	seen := map[FixtureFamily]struct{}{}
	for _, spec := range fixtureSpecs {
		seen[spec.Family] = struct{}{}
	}

	families := make([]FixtureFamily, 0, len(seen))
	for family := range seen {
		families = append(families, family)
	}
	sort.Slice(families, func(i, j int) bool { return families[i] < families[j] })

	return families
}

// ParseFixtureFamilies parses a comma-separated family filter.
func ParseFixtureFamilies(raw string) (map[FixtureFamily]struct{}, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	allowed := map[FixtureFamily]struct{}{}
	for _, family := range FixtureFamilies() {
		allowed[family] = struct{}{}
	}

	selected := map[FixtureFamily]struct{}{}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		family := FixtureFamily(strings.TrimSpace(part))
		if family == "" {
			continue
		}
		if _, ok := allowed[family]; !ok {
			return nil, fmt.Errorf("unknown fixture family %q", family)
		}
		selected[family] = struct{}{}
	}

	if len(selected) == 0 {
		return nil, nil
	}

	return selected, nil
}

// FilterFixtureMatrixByFamily filters the matrix by selected families.
func FilterFixtureMatrixByFamily(matrix []FixtureSpec, selected map[FixtureFamily]struct{}) []FixtureSpec {
	if len(selected) == 0 {
		out := make([]FixtureSpec, len(matrix))
		copy(out, matrix)
		return out
	}

	out := make([]FixtureSpec, 0, len(matrix))
	for _, spec := range matrix {
		if _, ok := selected[spec.Family]; ok {
			out = append(out, spec)
		}
	}

	return out
}

// LiveProbeMatrix returns one or more live probe entries from the matrix.
func LiveProbeMatrix(matrix []FixtureSpec) []FixtureSpec {
	out := make([]FixtureSpec, 0, len(matrix))
	for _, spec := range matrix {
		if spec.LiveProbe {
			out = append(out, spec)
		}
	}
	return out
}
