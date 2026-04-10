package cricinfo

import "testing"

func TestFixtureMatrixCoversAllMajorFamilies(t *testing.T) {
	t.Parallel()

	required := []FixtureFamily{
		FixtureFamilyRootDiscovery,
		FixtureFamilyMatchesCompetition,
		FixtureFamilyDetailsPlays,
		FixtureFamilyTeamCompetitor,
		FixtureFamilyInningsDepth,
		FixtureFamilyPlayers,
		FixtureFamilyLeagueSeason,
		FixtureFamilyAuxCompetitionMeta,
	}

	matrix := FixtureMatrix()
	found := map[FixtureFamily]int{}
	for _, spec := range matrix {
		found[spec.Family]++
		if spec.Name == "" || spec.Ref == "" || spec.FixturePath == "" {
			t.Fatalf("invalid fixture spec %+v", spec)
		}
	}

	for _, family := range required {
		if found[family] == 0 {
			t.Fatalf("missing fixture coverage for family %q", family)
		}
	}
}

func TestParseFixtureFamilies(t *testing.T) {
	t.Parallel()

	selected, err := ParseFixtureFamilies("players,details-plays")
	if err != nil {
		t.Fatalf("ParseFixtureFamilies error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected families, got %d", len(selected))
	}

	if _, err := ParseFixtureFamilies("players,unknown"); err == nil {
		t.Fatalf("expected parse error for unknown family")
	}
}
