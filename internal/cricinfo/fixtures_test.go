package cricinfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCuratedFixturesExistAndAreValidJSON(t *testing.T) {
	t.Parallel()

	for _, spec := range FixtureMatrix() {
		path := filepath.Join("testdata", "fixtures", spec.FixturePath)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read fixture %q: %v", path, err)
		}
		if len(data) == 0 {
			t.Fatalf("fixture %q is empty", path)
		}

		var payload any
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("fixture %q invalid JSON: %v", path, err)
		}
	}
}

func TestCuratedFixturesCoverMajorFamilies(t *testing.T) {
	t.Parallel()

	assertFixtureFamilyKeys(t, FixtureFamilyRootDiscovery, "root")
	assertFixtureFamilyKeys(t, FixtureFamilyMatchesCompetition, "competition")
	assertFixtureFamilyKeys(t, FixtureFamilyDetailsPlays, "plays-page")
	assertFixtureFamilyKeys(t, FixtureFamilyTeamCompetitor, "competitor")
	assertFixtureFamilyKeys(t, FixtureFamilyInningsDepth, "innings-period")
	assertFixtureFamilyKeys(t, FixtureFamilyPlayers, "athlete-profile")
	assertFixtureFamilyKeys(t, FixtureFamilyLeagueSeason, "standings")
	assertFixtureFamilyKeys(t, FixtureFamilyAuxCompetitionMeta, "officials")
}

func TestCuratedFixturesDecodeContracts(t *testing.T) {
	t.Parallel()

	eventsBody := mustReadFixtureByName(t, FixtureFamilyRootDiscovery, "events-page")
	page, err := DecodePage[Ref](eventsBody)
	if err != nil {
		t.Fatalf("DecodePage events fixture error: %v", err)
	}
	if page.PageSize == 0 {
		t.Fatalf("expected non-zero page size in events fixture")
	}

	playsBody := mustReadFixtureByName(t, FixtureFamilyDetailsPlays, "plays-page")
	plays, err := DecodePage[Ref](playsBody)
	if err != nil {
		t.Fatalf("DecodePage plays fixture error: %v", err)
	}
	if len(plays.Items) == 0 {
		t.Fatalf("expected at least one play item ref")
	}

	statsBody := mustReadFixtureByName(t, FixtureFamilyPlayers, "athlete-statistics")
	stats, err := DecodeStatsObject(statsBody)
	if err != nil {
		t.Fatalf("DecodeStatsObject player statistics fixture error: %v", err)
	}
	if len(stats.Splits) == 0 {
		t.Fatalf("expected non-empty splits in player statistics fixture")
	}

	rosterBody := mustReadFixtureByName(t, FixtureFamilyTeamCompetitor, "competitor-roster")
	rosterEntries, err := DecodeObjectCollection[map[string]any](rosterBody, "entries")
	if err != nil {
		t.Fatalf("DecodeObjectCollection roster fixture error: %v", err)
	}
	if len(rosterEntries) == 0 {
		t.Fatalf("expected roster entries in roster fixture")
	}

	rosterObjectBody, err := os.ReadFile(filepath.Join("testdata", "fixtures", "team-competitor", "roster-1147772-object.json"))
	if err != nil {
		t.Fatalf("read object-shaped roster fixture error: %v", err)
	}
	rosterObjectEntries, err := DecodeObjectCollection[map[string]any](rosterObjectBody, "entries")
	if err != nil {
		t.Fatalf("DecodeObjectCollection object-shaped roster fixture error: %v", err)
	}
	if len(rosterObjectEntries) == 0 {
		t.Fatalf("expected roster entries in object-shaped roster fixture")
	}

	leadersBody, err := os.ReadFile(filepath.Join("testdata", "fixtures", "team-competitor", "leaders-789643.json"))
	if err != nil {
		t.Fatalf("read leaders fixture error: %v", err)
	}
	leaders, err := NormalizeTeamLeaders(leadersBody, Team{ID: "789643"}, TeamScopeMatch, "1529474")
	if err != nil {
		t.Fatalf("NormalizeTeamLeaders fixture error: %v", err)
	}
	if len(leaders.Categories) == 0 {
		t.Fatalf("expected category-based leaders in leaders fixture")
	}
	if len(leaders.Categories[0].Leaders) == 0 {
		t.Fatalf("expected at least one leader entry in leaders fixture")
	}
}

func TestPhase9FixtureNormalizationForWicketSplitsAndPartnershipPayloads(t *testing.T) {
	t.Parallel()

	statsBody, err := os.ReadFile(filepath.Join("testdata", "fixtures", "team-competitor", "statistics-789643.json"))
	if err != nil {
		t.Fatalf("read statistics fixture error: %v", err)
	}
	overs, wickets, err := NormalizeInningsPeriodStatistics(statsBody)
	if err != nil {
		t.Fatalf("NormalizeInningsPeriodStatistics fixture error: %v", err)
	}
	if len(overs) == 0 {
		t.Fatalf("expected over timeline entries from period statistics fixture")
	}
	if len(wickets) == 0 {
		t.Fatalf("expected wicket timeline entries from period statistics fixture")
	}
	if wickets[0].DetailRef == "" {
		t.Fatalf("expected wicket timeline detail ref from statistics fixture")
	}

	partnershipBody, err := os.ReadFile(filepath.Join("testdata", "fixtures", "innings-fow-partnerships", "partnership-1.json"))
	if err != nil {
		t.Fatalf("read partnership fixture error: %v", err)
	}
	partnership, err := NormalizePartnership(partnershipBody)
	if err != nil {
		t.Fatalf("NormalizePartnership fixture error: %v", err)
	}
	if partnership.WicketNumber == 0 || partnership.Runs == 0 {
		t.Fatalf("expected detailed partnership fields, got %+v", partnership)
	}
	if len(partnership.Batsmen) == 0 {
		t.Fatalf("expected partnership batsmen payload")
	}

	fowBody, err := os.ReadFile(filepath.Join("testdata", "fixtures", "innings-fow-partnerships", "fow-1.json"))
	if err != nil {
		t.Fatalf("read fow fixture error: %v", err)
	}
	fow, err := NormalizeFallOfWicket(fowBody)
	if err != nil {
		t.Fatalf("NormalizeFallOfWicket fixture error: %v", err)
	}
	if fow.WicketNumber == 0 || fow.WicketOver == 0 {
		t.Fatalf("expected detailed fow fields, got %+v", fow)
	}
	if fow.AthleteRef == "" {
		t.Fatalf("expected fow athlete ref in detailed payload")
	}
}

func TestPhase10FixtureNormalizationForPlayerProfileAndStats(t *testing.T) {
	t.Parallel()

	profileBody, err := os.ReadFile(filepath.Join("testdata", "fixtures", "players", "athlete-1361257.json"))
	if err != nil {
		t.Fatalf("read player profile fixture error: %v", err)
	}
	player, err := NormalizePlayer(profileBody)
	if err != nil {
		t.Fatalf("NormalizePlayer fixture error: %v", err)
	}
	if player.Team == nil || len(player.Styles) == 0 {
		t.Fatalf("expected normalized team and styles in player profile, got %+v", player)
	}
	if len(player.MajorTeams) == 0 {
		t.Fatalf("expected normalized major teams in player profile")
	}

	statsBody, err := os.ReadFile(filepath.Join("testdata", "fixtures", "players", "athlete-1361257-statistics.json"))
	if err != nil {
		t.Fatalf("read player statistics fixture error: %v", err)
	}
	playerStats, err := NormalizePlayerStatistics(statsBody)
	if err != nil {
		t.Fatalf("NormalizePlayerStatistics fixture error: %v", err)
	}
	if len(playerStats.Categories) == 0 {
		t.Fatalf("expected grouped categories in player statistics fixture")
	}
	if len(playerStats.Categories[0].Stats) == 0 {
		t.Fatalf("expected grouped stats in player statistics fixture")
	}
}

func assertFixtureFamilyKeys(t *testing.T, family FixtureFamily, name string) {
	t.Helper()

	body := mustReadFixtureByName(t, family, name)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal fixture %q/%q: %v", family, name, err)
	}
	if len(payload) == 0 {
		t.Fatalf("fixture %q/%q is empty object", family, name)
	}

	switch family {
	case FixtureFamilyRootDiscovery:
		requireAnyKey(t, payload, "events", "leagues", "items")
	case FixtureFamilyMatchesCompetition:
		requireAnyKey(t, payload, "competitors", "status", "date")
	case FixtureFamilyDetailsPlays:
		requireAnyKey(t, payload, "items", "count", "pageSize")
	case FixtureFamilyTeamCompetitor:
		requireAnyKey(t, payload, "id", "team", "linescores")
	case FixtureFamilyInningsDepth:
		requireAnyKey(t, payload, "period", "runs", "wickets")
	case FixtureFamilyPlayers:
		requireAnyKey(t, payload, "id", "displayName", "fullName")
	case FixtureFamilyLeagueSeason:
		requireAnyKey(t, payload, "children", "entries", "items", "seasons")
	case FixtureFamilyAuxCompetitionMeta:
		requireAnyKey(t, payload, "items", "entries", "count", "officials")
	default:
		t.Fatalf("unexpected family %q", family)
	}
}

func requireAnyKey(t *testing.T, payload map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := payload[key]; ok {
			return
		}
	}
	t.Fatalf("expected any key in %v, got keys: %v", keys, mapKeys(payload))
}

func mustReadFixtureByName(t *testing.T, family FixtureFamily, name string) []byte {
	t.Helper()

	for _, spec := range FixtureMatrix() {
		if spec.Family == family && spec.Name == name {
			path := filepath.Join("testdata", "fixtures", spec.FixturePath)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture %q: %v", path, err)
			}
			return data
		}
	}

	t.Fatalf("fixture not found in matrix: family=%q name=%q", family, name)
	return nil
}

func mapKeys(payload map[string]any) []string {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	return keys
}
