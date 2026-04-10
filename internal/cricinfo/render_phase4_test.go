package cricinfo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeCoreEntitiesFromFixtures(t *testing.T) {
	t.Parallel()

	competition := mustReadFixtureFile(t, "matches-competitions/competition.json")
	match, err := NormalizeMatch(competition)
	if err != nil {
		t.Fatalf("NormalizeMatch error: %v", err)
	}
	assertJSONHasKeys(t, match, "id", "leagueId", "eventId", "competitionId", "teams")
	if len(match.Teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(match.Teams))
	}

	athlete := mustReadFixtureFile(t, "players/athlete-1361257.json")
	player, err := NormalizePlayer(athlete)
	if err != nil {
		t.Fatalf("NormalizePlayer error: %v", err)
	}
	assertJSONHasKeys(t, player, "id", "displayName", "teamRef", "styles")

	competitor := mustReadFixtureFile(t, "team-competitor/competitor-789643.json")
	team, err := NormalizeTeam(competitor)
	if err != nil {
		t.Fatalf("NormalizeTeam error: %v", err)
	}
	assertJSONHasKeys(t, team, "id", "homeAway", "rosterRef")

	root := mustReadFixtureFile(t, "root-discovery/root.json")
	league, err := NormalizeLeague(root)
	if err != nil {
		t.Fatalf("NormalizeLeague error: %v", err)
	}
	assertJSONHasKeys(t, league, "id", "name", "slug")

	seasonsBody := mustReadFixtureFile(t, "leagues-seasons-standings/seasons.json")
	seasons, err := NormalizeSeasonList(seasonsBody)
	if err != nil {
		t.Fatalf("NormalizeSeasonList error: %v", err)
	}
	if len(seasons) == 0 {
		t.Fatal("expected seasons")
	}
	assertJSONHasKeys(t, seasons[0], "id", "leagueId", "year")

	standingsBody := mustReadFixtureFile(t, "leagues-seasons-standings/standings.json")
	groups, err := NormalizeStandingsGroups(standingsBody)
	if err != nil {
		t.Fatalf("NormalizeStandingsGroups error: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("expected standings groups")
	}
	assertJSONHasKeys(t, groups[0], "id", "seasonId")

	inningsBody := mustReadFixtureFile(t, "innings-fow-partnerships/innings-1-2.json")
	innings, err := NormalizeInnings(inningsBody)
	if err != nil {
		t.Fatalf("NormalizeInnings error: %v", err)
	}
	assertJSONHasKeys(t, innings, "id", "period", "runs", "wickets", "partnershipsRef", "fallOfWicketRef")

	detailBody := mustReadFixtureFile(t, "details-plays/detail-110.json")
	delivery, err := NormalizeDeliveryEvent(detailBody)
	if err != nil {
		t.Fatalf("NormalizeDeliveryEvent error: %v", err)
	}
	assertJSONHasKeys(t, delivery, "id", "period", "overNumber", "ballNumber", "scoreValue", "batsmanRef", "bowlerRef")

	statsBody := mustReadFixtureFile(t, "players/athlete-1361257-statistics.json")
	categories, err := NormalizeStatCategories(statsBody)
	if err != nil {
		t.Fatalf("NormalizeStatCategories error: %v", err)
	}
	if len(categories) == 0 {
		t.Fatal("expected stat categories")
	}
	assertJSONHasKeys(t, categories[0], "name", "displayName", "stats")

	partnershipBody := mustReadFixtureFile(t, "innings-fow-partnerships/partnerships.json")
	partnerships, err := NormalizePartnerships(partnershipBody)
	if err != nil {
		t.Fatalf("NormalizePartnerships error: %v", err)
	}
	if len(partnerships) == 0 {
		t.Fatal("expected partnerships")
	}
	assertJSONHasKeys(t, partnerships[0], "id", "inningsId", "order")

	fowBody := mustReadFixtureFile(t, "innings-fow-partnerships/fow.json")
	wickets, err := NormalizeFallOfWickets(fowBody)
	if err != nil {
		t.Fatalf("NormalizeFallOfWickets error: %v", err)
	}
	if len(wickets) == 0 {
		t.Fatal("expected wicket entries")
	}
	assertJSONHasKeys(t, wickets[0], "id", "inningsId", "wicketNumber")
}

func TestNormalizeExtensionsPreserveLongTailFields(t *testing.T) {
	t.Parallel()

	competition := mustReadFixtureFile(t, "matches-competitions/competition.json")
	match, err := NormalizeMatch(competition)
	if err != nil {
		t.Fatalf("NormalizeMatch error: %v", err)
	}
	if _, ok := match.Extensions["notes"]; !ok {
		t.Fatalf("expected match extensions to preserve notes, got keys: %v", mapKeys(match.Extensions))
	}
	if _, ok := match.Extensions["class"]; !ok {
		t.Fatalf("expected match extensions to preserve class")
	}

	athlete := mustReadFixtureFile(t, "players/athlete-1361257.json")
	player, err := NormalizePlayer(athlete)
	if err != nil {
		t.Fatalf("NormalizePlayer error: %v", err)
	}
	if _, ok := player.Extensions["links"]; !ok {
		t.Fatalf("expected player extensions to preserve links, got keys: %v", mapKeys(player.Extensions))
	}
	if _, ok := player.Extensions["majorTeams"]; !ok {
		t.Fatalf("expected player extensions to preserve majorTeams")
	}

	detailBody := mustReadFixtureFile(t, "details-plays/detail-110.json")
	delivery, err := NormalizeDeliveryEvent(detailBody)
	if err != nil {
		t.Fatalf("NormalizeDeliveryEvent error: %v", err)
	}
	if _, ok := delivery.Extensions["athletesInvolved"]; !ok {
		t.Fatalf("expected delivery extensions to preserve athletesInvolved, got keys: %v", mapKeys(delivery.Extensions))
	}
	if _, ok := delivery.Extensions["innings"]; !ok {
		t.Fatalf("expected delivery extensions to preserve innings")
	}
}

func TestRenderTextGoldenSnapshots(t *testing.T) {
	t.Parallel()

	competition := mustReadFixtureFile(t, "matches-competitions/competition.json")
	match, err := NormalizeMatch(competition)
	if err != nil {
		t.Fatalf("NormalizeMatch error: %v", err)
	}

	tests := []struct {
		name   string
		file   string
		result NormalizedResult
	}{
		{
			name:   "match-list",
			file:   "match-list.golden",
			result: NewListResult(EntityMatch, []any{match}),
		},
		{
			name:   "match-empty",
			file:   "match-empty.golden",
			result: NewListResult(EntityMatch, nil),
		},
		{
			name:   "match-partial",
			file:   "match-partial.golden",
			result: NewPartialListResult(EntityMatch, []any{match}, "plays endpoint returned pointer-only payload"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := Render(&buf, tc.result, RenderOptions{Format: "text"}); err != nil {
				t.Fatalf("Render text error: %v", err)
			}
			assertGolden(t, tc.file, buf.String())
		})
	}
}

func TestRenderJSONStructureAndAllFieldsToggle(t *testing.T) {
	t.Parallel()

	competition := mustReadFixtureFile(t, "matches-competitions/competition.json")
	match, err := NormalizeMatch(competition)
	if err != nil {
		t.Fatalf("NormalizeMatch error: %v", err)
	}

	result := NewDataResult(EntityMatch, match)

	var compact bytes.Buffer
	if err := Render(&compact, result, RenderOptions{Format: "json"}); err != nil {
		t.Fatalf("Render json error: %v", err)
	}
	compactMap := decodeJSONMap(t, compact.Bytes())
	assertMapHasKey(t, compactMap, "kind")
	assertMapHasKey(t, compactMap, "status")
	assertMapHasKey(t, compactMap, "data")

	dataMap, ok := compactMap["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object in json output")
	}
	if _, ok := dataMap["extensions"]; ok {
		t.Fatalf("expected extensions to be omitted without --all-fields")
	}

	var allFields bytes.Buffer
	if err := Render(&allFields, result, RenderOptions{Format: "json", AllFields: true}); err != nil {
		t.Fatalf("Render json all-fields error: %v", err)
	}
	allFieldsMap := decodeJSONMap(t, allFields.Bytes())
	allDataMap := allFieldsMap["data"].(map[string]any)
	extensions, ok := allDataMap["extensions"].(map[string]any)
	if !ok {
		t.Fatalf("expected extensions with --all-fields")
	}
	if _, ok := extensions["notes"]; !ok {
		t.Fatalf("expected notes to survive in extensions")
	}
}

func TestRenderJSONLBehavior(t *testing.T) {
	t.Parallel()

	seasonsBody := mustReadFixtureFile(t, "leagues-seasons-standings/seasons.json")
	seasons, err := NormalizeSeasonList(seasonsBody)
	if err != nil {
		t.Fatalf("NormalizeSeasonList error: %v", err)
	}

	items := make([]any, 0, len(seasons))
	for i := 0; i < 2 && i < len(seasons); i++ {
		items = append(items, seasons[i])
	}

	result := NewListResult(EntitySeason, items)
	var buf bytes.Buffer
	if err := Render(&buf, result, RenderOptions{Format: "jsonl"}); err != nil {
		t.Fatalf("Render jsonl error: %v", err)
	}
	lines := splitNonEmptyLines(buf.String())
	if len(lines) != len(items) {
		t.Fatalf("expected %d jsonl lines, got %d", len(items), len(lines))
	}

	partial := NewPartialListResult(EntitySeason, items, "season endpoint timed out on page 2")
	buf.Reset()
	if err := Render(&buf, partial, RenderOptions{Format: "jsonl"}); err != nil {
		t.Fatalf("Render jsonl partial error: %v", err)
	}
	partialLines := splitNonEmptyLines(buf.String())
	if len(partialLines) != len(items)+1 {
		t.Fatalf("expected meta + %d items for partial jsonl, got %d", len(items), len(partialLines))
	}
	if !strings.Contains(partialLines[0], "_meta") {
		t.Fatalf("expected first jsonl line to be metadata, got %q", partialLines[0])
	}

	empty := NewListResult(EntitySeason, nil)
	buf.Reset()
	if err := Render(&buf, empty, RenderOptions{Format: "jsonl"}); err != nil {
		t.Fatalf("Render jsonl empty error: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("expected empty jsonl output for empty list, got %q", buf.String())
	}

	single := NewDataResult(EntitySeason, seasons[0])
	if err := Render(&buf, single, RenderOptions{Format: "jsonl"}); err == nil {
		t.Fatal("expected jsonl render error for single-entity result")
	}
}

func TestTransportErrorMessaging(t *testing.T) {
	t.Parallel()

	err := &HTTPStatusError{URL: "http://example.com/events", StatusCode: 503}
	result := NewTransportErrorResult(EntityMatch, "/events", err)
	if result.Status != ResultStatusError {
		t.Fatalf("expected error status, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "status 503") {
		t.Fatalf("expected status code in error message, got %q", result.Message)
	}

	var buf bytes.Buffer
	if err := Render(&buf, result, RenderOptions{Format: "text"}); err != nil {
		t.Fatalf("Render text error result: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Status: 503") {
		t.Fatalf("expected text renderer to include status code, got %q", output)
	}
	if !strings.Contains(output, "Requested: /events") {
		t.Fatalf("expected text renderer to include requested ref, got %q", output)
	}
}

func mustReadFixtureFile(t *testing.T, fixturePath string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "fixtures", fixturePath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q: %v", path, err)
	}
	return data
}

func assertJSONHasKeys(t *testing.T, value any, keys ...string) {
	t.Helper()
	blob, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	mapped := decodeJSONMap(t, blob)
	for _, key := range keys {
		assertMapHasKey(t, mapped, key)
	}
}

func assertMapHasKey(t *testing.T, mapped map[string]any, key string) {
	t.Helper()
	if _, ok := mapped[key]; !ok {
		t.Fatalf("expected json key %q, got keys: %v", key, mapKeys(mapped))
	}
}

func decodeJSONMap(t *testing.T, blob []byte) map[string]any {
	t.Helper()
	var mapped map[string]any
	if err := json.Unmarshal(blob, &mapped); err != nil {
		t.Fatalf("decode json map: %v", err)
	}
	return mapped
}

func splitNonEmptyLines(text string) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func assertGolden(t *testing.T, goldenFile string, actual string) {
	t.Helper()
	goldenPath := filepath.Join("testdata", "golden", goldenFile)

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
	}

	expectedBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file %q: %v", goldenPath, err)
	}
	expected := string(expectedBytes)
	if actual != expected {
		t.Fatalf("golden mismatch for %s\n--- expected ---\n%s\n--- actual ---\n%s", goldenFile, expected, actual)
	}
}
