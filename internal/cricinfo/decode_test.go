package cricinfo

import (
	"testing"
)

type testItem struct {
	ID int `json:"id"`
}

func TestDecodePage(t *testing.T) {
	t.Parallel()

	page, err := DecodePage[Ref]([]byte(`{"count":1,"items":[{"$ref":"http://example.com/a"}],"pageCount":1,"pageIndex":1,"pageSize":20}`))
	if err != nil {
		t.Fatalf("DecodePage error: %v", err)
	}

	if page.Count != 1 || page.PageSize != 20 {
		t.Fatalf("unexpected page metadata: %+v", page)
	}
	if len(page.Items) != 1 || page.Items[0].URL != "http://example.com/a" {
		t.Fatalf("unexpected page items: %+v", page.Items)
	}
}

func TestDecodeObjectCollectionArray(t *testing.T) {
	t.Parallel()

	items, err := DecodeObjectCollection[testItem]([]byte(`{"entries":[{"id":1},{"id":2}]}`), "entries")
	if err != nil {
		t.Fatalf("DecodeObjectCollection error: %v", err)
	}
	if len(items) != 2 || items[0].ID != 1 || items[1].ID != 2 {
		t.Fatalf("unexpected array items: %+v", items)
	}
}

func TestDecodeObjectCollectionObject(t *testing.T) {
	t.Parallel()

	items, err := DecodeObjectCollection[testItem]([]byte(`{"entries":{"b":{"id":2},"a":{"id":1}}}`), "entries")
	if err != nil {
		t.Fatalf("DecodeObjectCollection error: %v", err)
	}
	if len(items) != 2 || items[0].ID != 1 || items[1].ID != 2 {
		t.Fatalf("unexpected object items order/content: %+v", items)
	}
}

func TestDecodeObjectCollectionNull(t *testing.T) {
	t.Parallel()

	items, err := DecodeObjectCollection[testItem]([]byte(`{"entries":null}`), "entries")
	if err != nil {
		t.Fatalf("DecodeObjectCollection error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty slice, got %+v", items)
	}
}

func TestDecodeStatsObject(t *testing.T) {
	t.Parallel()

	stats, err := DecodeStatsObject([]byte(`{"$ref":"http://example.com/stats","athlete":{"$ref":"http://example.com/athletes/1"},"splits":{"categories":[]}}`))
	if err != nil {
		t.Fatalf("DecodeStatsObject error: %v", err)
	}

	if stats.Ref != "http://example.com/stats" {
		t.Fatalf("unexpected stats ref: %q", stats.Ref)
	}
	if stats.Athlete == nil || stats.Athlete.URL == "" {
		t.Fatalf("expected athlete ref in stats object")
	}
	if len(stats.Splits) == 0 {
		t.Fatalf("expected splits payload")
	}
}

func TestExtractOptionalRef(t *testing.T) {
	t.Parallel()

	ref, ok, err := ExtractOptionalRef([]byte(`{"status":{"$ref":"http://example.com/status"}}`), "status")
	if err != nil {
		t.Fatalf("ExtractOptionalRef error: %v", err)
	}
	if !ok || ref == nil || ref.URL != "http://example.com/status" {
		t.Fatalf("unexpected optional ref output: ok=%v ref=%+v", ok, ref)
	}

	nilRef, nilOk, nilErr := ExtractOptionalRef([]byte(`{"status":null}`), "status")
	if nilErr != nil {
		t.Fatalf("ExtractOptionalRef null error: %v", nilErr)
	}
	if nilOk || nilRef != nil {
		t.Fatalf("expected null optional ref, got ok=%v ref=%+v", nilOk, nilRef)
	}
}
