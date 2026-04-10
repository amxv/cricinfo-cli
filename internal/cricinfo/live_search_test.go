package cricinfo

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLiveSearchDiscoveryAcrossEntityFamilies(t *testing.T) {
	t.Parallel()
	requireLiveMatrix(t)

	resolver, err := NewResolver(ResolverConfig{
		EventSeedTTL: time.Minute,
		MaxEventSeed: 24,
		IndexPath:    t.TempDir() + "/live-search-index.json",
	})
	if err != nil {
		t.Fatalf("NewResolver error: %v", err)
	}
	defer func() {
		_ = resolver.Close()
	}()

	tests := []struct {
		name  string
		kind  EntityKind
		query string
		opts  ResolveOptions
		id    string
	}{
		{name: "player", kind: EntityPlayer, query: "1361257", opts: ResolveOptions{Limit: 5}, id: "1361257"},
		{name: "team", kind: EntityTeam, query: "789643", opts: ResolveOptions{Limit: 5}, id: "789643"},
		{name: "league", kind: EntityLeague, query: "19138", opts: ResolveOptions{Limit: 5}, id: "19138"},
		{name: "match", kind: EntityMatch, query: "1529474", opts: ResolveOptions{Limit: 5, LeagueID: "19138"}, id: "1529474"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			result, err := resolver.Search(ctx, tc.kind, tc.query, tc.opts)
			if err != nil {
				if isLiveSearchTransient503(err.Error()) {
					t.Skipf("skipping %s search after transient 503: %v", tc.name, err)
				}
				t.Fatalf("Search error for %s: %v", tc.name, err)
			}

			if len(result.Entities) == 0 {
				if hasLive503Warning(result.Warnings) {
					t.Skipf("skipping %s search after transient 503 warnings: %v", tc.name, result.Warnings)
				}
				t.Fatalf("expected at least one %s result for query %q", tc.name, tc.query)
			}

			found := false
			for _, entity := range result.Entities {
				if entity.ID == tc.id {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected %s result set to contain id %s, got %+v", tc.name, tc.id, result.Entities)
			}
		})
	}
}

func hasLive503Warning(warnings []string) bool {
	for _, warning := range warnings {
		if isLiveSearchTransient503(warning) {
			return true
		}
	}
	return false
}

func isLiveSearchTransient503(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	return strings.Contains(raw, "status 503") || strings.Contains(raw, "503 service unavailable")
}
