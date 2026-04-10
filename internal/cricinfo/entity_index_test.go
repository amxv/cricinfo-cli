package cricinfo

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEntityIndexSearchExactAndFuzzy(t *testing.T) {
	t.Parallel()

	idx, err := OpenEntityIndex(filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatalf("OpenEntityIndex error: %v", err)
	}

	now := time.Now().UTC()
	if err := idx.Upsert(IndexedEntity{
		Kind:      EntityPlayer,
		ID:        "1361257",
		Ref:       "http://core.espnuk.org/v2/sports/cricket/athletes/1361257",
		Name:      "Fazal Haq Shaheen",
		ShortName: "Fazal",
		Aliases:   []string{"Fazal Haq", "Shaheen"},
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Upsert player 1 error: %v", err)
	}

	if err := idx.Upsert(IndexedEntity{
		Kind:      EntityPlayer,
		ID:        "999",
		Name:      "John Doe",
		Aliases:   []string{"Johnny"},
		UpdatedAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("Upsert player 2 error: %v", err)
	}

	exact := idx.Search(EntityPlayer, "1361257", 5, SearchContext{})
	if len(exact) == 0 || exact[0].ID != "1361257" {
		t.Fatalf("expected exact id match for 1361257, got %+v", exact)
	}

	fuzzy := idx.Search(EntityPlayer, "faz sha", 5, SearchContext{})
	if len(fuzzy) == 0 || fuzzy[0].ID != "1361257" {
		t.Fatalf("expected fuzzy alias match for 'faz sha', got %+v", fuzzy)
	}
}

func TestEntityIndexSearchContextBoost(t *testing.T) {
	t.Parallel()

	idx, err := OpenEntityIndex(filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatalf("OpenEntityIndex error: %v", err)
	}

	if err := idx.Upsert(IndexedEntity{
		Kind:      EntityMatch,
		ID:        "1529474",
		Name:      "3rd Match",
		LeagueID:  "19138",
		MatchID:   "1529474",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Upsert preferred match error: %v", err)
	}

	if err := idx.Upsert(IndexedEntity{
		Kind:      EntityMatch,
		ID:        "1529999",
		Name:      "3rd Match",
		LeagueID:  "11132",
		MatchID:   "1529999",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Upsert non-preferred match error: %v", err)
	}

	results := idx.Search(EntityMatch, "3rd match", 5, SearchContext{PreferredLeagueID: "19138"})
	if len(results) == 0 {
		t.Fatalf("expected search results")
	}
	if results[0].ID != "1529474" {
		t.Fatalf("expected context-preferred match first, got %+v", results)
	}
}

func TestEntityIndexSearchPrefersExactMatchContext(t *testing.T) {
	t.Parallel()

	idx, err := OpenEntityIndex(filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatalf("OpenEntityIndex error: %v", err)
	}

	now := time.Now().UTC()
	entities := []IndexedEntity{
		{
			Kind:      EntityPlayer,
			ID:        "253802",
			Name:      "Virat Kohli",
			LeagueID:  "11132",
			MatchID:   "1527689",
			UpdatedAt: now,
		},
		{
			Kind:      EntityPlayer,
			ID:        "999999",
			Name:      "Virat Kohli",
			LeagueID:  "19138",
			MatchID:   "1529474",
			UpdatedAt: now.Add(time.Minute),
		},
	}
	if err := idx.UpsertMany(entities); err != nil {
		t.Fatalf("UpsertMany error: %v", err)
	}

	results := idx.Search(EntityPlayer, "virat kohli", 5, SearchContext{
		PreferredLeagueID: "11132",
		PreferredMatchID:  "1527689",
	})
	if len(results) == 0 {
		t.Fatalf("expected contextual player search results")
	}
	if results[0].ID != "253802" {
		t.Fatalf("expected preferred match-context player first, got %+v", results)
	}
	for _, result := range results {
		if result.MatchID != "1527689" {
			t.Fatalf("expected results to be narrowed to preferred match context, got %+v", results)
		}
	}
}

func TestEntityIndexSearchDoesNotMatchInitialForFullNameQuery(t *testing.T) {
	t.Parallel()

	idx, err := OpenEntityIndex(filepath.Join(t.TempDir(), "index.json"))
	if err != nil {
		t.Fatalf("OpenEntityIndex error: %v", err)
	}

	now := time.Now().UTC()
	if err := idx.UpsertMany([]IndexedEntity{
		{
			Kind:      EntityPlayer,
			ID:        "253802",
			Name:      "Virat Kohli",
			ShortName: "Kohli",
			Aliases:   []string{"Virat Kohli", "V Kohli"},
			UpdatedAt: now,
		},
		{
			Kind:      EntityPlayer,
			ID:        "1408688",
			Name:      "Vaibhav Sooryavanshi",
			ShortName: "Sooryavanshi",
			Aliases:   []string{"Vaibhav Sooryavanshi", "V Sooryavanshi"},
			UpdatedAt: now.Add(time.Minute),
		},
	}); err != nil {
		t.Fatalf("UpsertMany error: %v", err)
	}

	results := idx.Search(EntityPlayer, "Virat Kohli", 5, SearchContext{})
	if len(results) == 0 {
		t.Fatalf("expected player search results")
	}
	if results[0].ID != "253802" {
		t.Fatalf("expected exact full-name match first, got %+v", results)
	}
}
