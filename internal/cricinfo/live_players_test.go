package cricinfo

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestLivePlayerRoutes(t *testing.T) {
	t.Parallel()
	requireLiveMatrix(t)

	client, err := NewClient(Config{
		Timeout:    12 * time.Second,
		MaxRetries: 3,
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	routes := []struct {
		name string
		ref  string
		keys []string
	}{
		{
			name: "athlete-profile",
			ref:  "/athletes/1361257",
			keys: []string{"id", "displayName", "styles", "team"},
		},
		{
			name: "athlete-news",
			ref:  "/athletes/253802/news",
			keys: []string{"items", "count"},
		},
		{
			name: "athlete-statistics",
			ref:  "/athletes/1361257/statistics",
			keys: []string{"splits", "athlete"},
		},
	}

	for _, tc := range routes {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			resolved, err := client.ResolveRefChain(ctx, tc.ref)
			if err != nil {
				if isLive503(err) {
					t.Skipf("skipping %s after transient 503: %v", tc.name, err)
				}
				t.Fatalf("ResolveRefChain(%q) error: %v", tc.ref, err)
			}

			var payload map[string]any
			if err := json.Unmarshal(resolved.Body, &payload); err != nil {
				t.Fatalf("unmarshal %s payload: %v", tc.name, err)
			}
			requireAnyKey(t, payload, tc.keys...)
		})
	}
}
