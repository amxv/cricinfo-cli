package cricinfo

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestTemplateCoverageLedgerIncludesAllResearchedTemplates(t *testing.T) {
	t.Parallel()

	ledger := TemplateCoverageLedger()
	if len(ledger) == 0 {
		t.Fatalf("template coverage ledger is empty")
	}

	templates := researchedTemplatesFromTSV(t)
	if len(templates) == 0 {
		t.Fatalf("expected researched templates from TSV")
	}

	missing := make([]string, 0)
	for _, template := range templates {
		entry, ok := ledger[template]
		if !ok {
			missing = append(missing, template)
			continue
		}
		if strings.TrimSpace(entry.Template) == "" || entry.Template != template {
			t.Fatalf("template ledger entry mismatch for %q: %+v", template, entry)
		}
		if strings.TrimSpace(entry.CommandFamily) == "" {
			t.Fatalf("template %q has empty command family", template)
		}
		if strings.TrimSpace(entry.Command) == "" {
			t.Fatalf("template %q has empty command mapping", template)
		}
		if strings.TrimSpace(entry.View) == "" {
			t.Fatalf("template %q has empty result view mapping", template)
		}
	}

	extras := make([]string, 0)
	expected := make(map[string]struct{}, len(templates))
	for _, template := range templates {
		expected[template] = struct{}{}
	}
	for template := range ledger {
		if _, ok := expected[template]; !ok {
			extras = append(extras, template)
		}
	}

	sort.Strings(missing)
	sort.Strings(extras)
	if len(missing) > 0 || len(extras) > 0 {
		t.Fatalf("template coverage drift: missing=%v extras=%v", missing, extras)
	}
}

func TestFieldPathFamilyCoverageLedgerKnownFamiliesMapped(t *testing.T) {
	t.Parallel()

	ledger := FieldPathFamilyCoverageLedger()
	if len(ledger) == 0 {
		t.Fatalf("field-path family coverage ledger is empty")
	}

	catalogFamilies := researchedFieldPathFamilies(t)
	if len(catalogFamilies) == 0 {
		t.Fatalf("expected field-path families from catalog")
	}

	expectedFamilies := []string{
		"athlete",
		"athletesInvolved",
		"batsman",
		"bowler",
		"broadcasts",
		"competitions",
		"competitors",
		"details",
		"dismissal",
		"entries",
		"fow",
		"innings",
		"items",
		"leagues",
		"matchcards",
		"odds",
		"officials",
		"over",
		"partnerships",
		"seasons",
		"situation",
		"splits",
		"status",
		"teams",
		"tickets",
	}

	missingLedger := make([]string, 0)
	missingCatalog := make([]string, 0)
	for _, family := range expectedFamilies {
		entry, ok := ledger[family]
		if !ok {
			missingLedger = append(missingLedger, family)
			continue
		}
		if strings.TrimSpace(entry.Family) == "" || entry.Family != family {
			t.Fatalf("field-path ledger entry mismatch for %q: %+v", family, entry)
		}
		if strings.TrimSpace(entry.CommandFamily) == "" {
			t.Fatalf("field-path family %q has empty command family", family)
		}
		if strings.TrimSpace(entry.Command) == "" {
			t.Fatalf("field-path family %q has empty command mapping", family)
		}
		if strings.TrimSpace(entry.View) == "" {
			t.Fatalf("field-path family %q has empty result view mapping", family)
		}
		if _, ok := catalogFamilies[family]; !ok {
			missingCatalog = append(missingCatalog, family)
		}
	}

	sort.Strings(missingLedger)
	sort.Strings(missingCatalog)
	if len(missingLedger) > 0 || len(missingCatalog) > 0 {
		t.Fatalf("field-path family coverage drift: missing ledger=%v missing catalog=%v", missingLedger, missingCatalog)
	}
}

func researchedTemplatesFromTSV(t *testing.T) []string {
	t.Helper()

	path := filepath.Join(repoRoot(t), "gg", "agent-outputs", "cricinfo-working-templates.tsv")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open working templates TSV %q: %v", path, err)
	}
	defer file.Close()

	seen := map[string]struct{}{}
	templates := make([]string, 0, 64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			t.Fatalf("unexpected TSV row %q", line)
		}
		template := strings.TrimSpace(parts[1])
		if template == "" {
			continue
		}
		if _, ok := seen[template]; ok {
			continue
		}
		seen[template] = struct{}{}
		templates = append(templates, template)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan working templates TSV %q: %v", path, err)
	}

	sort.Strings(templates)
	return templates
}

func researchedFieldPathFamilies(t *testing.T) map[string]struct{} {
	t.Helper()

	path := filepath.Join(repoRoot(t), "gg", "agent-outputs", "cricinfo-field-path-catalog.txt")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open field-path catalog %q: %v", path, err)
	}
	defer file.Close()

	families := map[string]struct{}{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path == "" {
			continue
		}
		family := firstFieldPathFamily(path)
		if family == "" {
			continue
		}
		families[family] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan field-path catalog %q: %v", path, err)
	}

	return families
}

func firstFieldPathFamily(path string) string {
	parts := strings.Split(path, ".")
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" || token == "$ref" {
			continue
		}
		if isNumericToken(token) {
			continue
		}
		return token
	}
	return ""
}

func isNumericToken(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("unable to locate repository root from %q", dir)
		}
		dir = parent
	}
}
