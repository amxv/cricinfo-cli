package cricinfo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

const (
	liveMatrixEnv         = "CRICINFO_LIVE_MATRIX"
	liveMatrixFamiliesEnv = "CRICINFO_LIVE_FAMILIES"
)

func TestLiveFixtureMatrixByFamily(t *testing.T) {
	t.Parallel()
	requireLiveMatrix(t)

	selected, err := ParseFixtureFamilies(os.Getenv(liveMatrixFamiliesEnv))
	if err != nil {
		t.Fatalf("ParseFixtureFamilies error: %v", err)
	}

	matrix := FixtureMatrix()
	matrix = FilterFixtureMatrixByFamily(matrix, selected)
	matrix = LiveProbeMatrix(matrix)

	if len(matrix) == 0 {
		t.Fatalf("no live probes selected from fixture matrix")
	}

	requiredFamilies := map[FixtureFamily]struct{}{}
	for _, spec := range matrix {
		requiredFamilies[spec.Family] = struct{}{}
	}
	for family := range selected {
		if _, ok := requiredFamilies[family]; !ok {
			t.Fatalf("selected family %q has no live probe in matrix", family)
		}
	}

	client, err := NewClient(Config{
		Timeout:    12 * time.Second,
		MaxRetries: 3,
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	for _, spec := range matrix {
		spec := spec
		t.Run(fmt.Sprintf("%s/%s", spec.Family, spec.Name), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			resolved, err := client.ResolveRefChain(ctx, spec.Ref)
			if err != nil {
				if isLive503(err) {
					t.Skipf("skipping after persistent 503 for %s: %v", spec.Ref, err)
				}
				t.Fatalf("ResolveRefChain(%q) error: %v", spec.Ref, err)
			}

			var payload map[string]any
			if err := json.Unmarshal(resolved.Body, &payload); err != nil {
				t.Fatalf("unmarshal %q: %v", spec.Ref, err)
			}
			if len(payload) == 0 {
				t.Fatalf("empty payload from %q", spec.Ref)
			}

			validateLiveFamilyPayload(t, spec.Family, payload)
		})
	}
}

func requireLiveMatrix(t *testing.T) {
	t.Helper()
	if os.Getenv(liveMatrixEnv) != "1" {
		t.Skip("set CRICINFO_LIVE_MATRIX=1 to run live fixture matrix tests")
	}
}

func isLive503(err error) bool {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == 503
	}
	return false
}

func validateLiveFamilyPayload(t *testing.T, family FixtureFamily, payload map[string]any) {
	t.Helper()

	switch family {
	case FixtureFamilyRootDiscovery:
		requireAnyKey(t, payload, "events", "leagues", "items")
	case FixtureFamilyMatchesCompetition:
		requireAnyKey(t, payload, "competitors", "status", "date")
	case FixtureFamilyDetailsPlays:
		requireAnyKey(t, payload, "items", "count")
	case FixtureFamilyTeamCompetitor:
		requireAnyKey(t, payload, "id", "team", "record")
	case FixtureFamilyInningsDepth:
		requireAnyKey(t, payload, "period", "runs", "wickets")
	case FixtureFamilyPlayers:
		requireAnyKey(t, payload, "id", "displayName", "fullName")
	case FixtureFamilyLeagueSeason:
		requireAnyKey(t, payload, "children", "items", "entries")
	case FixtureFamilyAuxCompetitionMeta:
		requireAnyKey(t, payload, "items", "count", "entries")
	default:
		t.Fatalf("unsupported family %q", family)
	}
}
