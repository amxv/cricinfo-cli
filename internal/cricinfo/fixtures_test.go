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
