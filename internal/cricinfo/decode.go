package cricinfo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DecodePage decodes a paginated envelope payload.
func DecodePage[T any](data []byte) (*Page[T], error) {
	var page Page[T]
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("decode paginated payload: %w", err)
	}

	if page.Items == nil {
		page.Items = []T{}
	}

	return &page, nil
}

// DecodeObjectCollection decodes array-shaped or object-shaped collection fields.
func DecodeObjectCollection[T any](data []byte, field string) ([]T, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return nil, fmt.Errorf("collection field is required")
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}

	raw, ok := envelope[field]
	if !ok {
		return nil, fmt.Errorf("collection field %q not found", field)
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return []T{}, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var items []T
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("decode array collection field %q: %w", field, err)
		}
		return items, nil
	}

	if strings.HasPrefix(trimmed, "{") {
		var keyed map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keyed); err != nil {
			return nil, fmt.Errorf("decode object collection field %q: %w", field, err)
		}

		keys := make([]string, 0, len(keyed))
		for k := range keyed {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		items := make([]T, 0, len(keys))
		for _, key := range keys {
			var item T
			if err := json.Unmarshal(keyed[key], &item); err != nil {
				return nil, fmt.Errorf("decode object collection field %q key %q: %w", field, key, err)
			}
			items = append(items, item)
		}
		return items, nil
	}

	return nil, fmt.Errorf("collection field %q is neither array nor object", field)
}

// DecodeStatsObject decodes a single-object stats payload.
func DecodeStatsObject(data []byte) (*StatsObject, error) {
	var stats StatsObject
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("decode stats payload: %w", err)
	}
	return &stats, nil
}

// ExtractOptionalRef gets a nested {"$ref":"..."} field when present.
func ExtractOptionalRef(data []byte, field string) (*Ref, bool, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return nil, false, fmt.Errorf("field is required")
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, false, fmt.Errorf("decode envelope: %w", err)
	}

	raw, ok := envelope[field]
	if !ok || string(raw) == "null" {
		return nil, false, nil
	}

	var ref Ref
	if err := json.Unmarshal(raw, &ref); err != nil {
		return nil, false, fmt.Errorf("decode ref field %q: %w", field, err)
	}

	if strings.TrimSpace(ref.URL) == "" {
		return nil, false, nil
	}

	return &ref, true, nil
}
