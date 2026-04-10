package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
)

const defaultOutputRoot = "internal/cricinfo/testdata/fixtures"

func main() {
	var (
		familiesRaw string
		outputRoot  string
		timeout     time.Duration
		maxRetries  int
		write       bool
	)

	flag.StringVar(&familiesRaw, "families", "", "comma-separated fixture families to refresh")
	flag.StringVar(&outputRoot, "output", defaultOutputRoot, "fixture output root directory")
	flag.DurationVar(&timeout, "timeout", 12*time.Second, "per-request timeout")
	flag.IntVar(&maxRetries, "max-retries", 3, "max retries per request")
	flag.BoolVar(&write, "write", false, "write fixtures to disk (required for refresh)")
	flag.Parse()

	if !write {
		fmt.Println("dry-run: pass --write to refresh fixture files")
		return
	}

	selected, err := cricinfo.ParseFixtureFamilies(familiesRaw)
	if err != nil {
		fatalf("parse families: %v", err)
	}

	matrix := cricinfo.FixtureMatrix()
	matrix = cricinfo.FilterFixtureMatrixByFamily(matrix, selected)
	if len(matrix) == 0 {
		fatalf("no fixture specs selected")
	}

	client, err := cricinfo.NewClient(cricinfo.Config{
		Timeout:    timeout,
		MaxRetries: maxRetries,
	})
	if err != nil {
		fatalf("new client: %v", err)
	}

	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		fatalf("create output root %q: %v", outputRoot, err)
	}

	refreshed := 0
	keptExisting := 0
	for _, spec := range matrix {
		targetPath := filepath.Join(outputRoot, filepath.FromSlash(spec.FixturePath))
		if err := ensurePathUnderRoot(outputRoot, targetPath); err != nil {
			fatalf("invalid fixture path for %s/%s: %v", spec.Family, spec.Name, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		resolved, fetchErr := client.ResolveRefChain(ctx, spec.Ref)
		cancel()

		if fetchErr != nil {
			var statusErr *cricinfo.HTTPStatusError
			if errors.As(fetchErr, &statusErr) && statusErr.StatusCode == 503 {
				if _, statErr := os.Stat(targetPath); statErr == nil {
					fmt.Printf("! kept existing fixture after persistent 503: %s (%s)\n", spec.Name, spec.Ref)
					keptExisting++
					continue
				}
				fatalf("persistent 503 and no existing fixture for %s (%s)", spec.Name, spec.Ref)
			}
			fatalf("refresh %s (%s): %v", spec.Name, spec.Ref, fetchErr)
		}

		formatted := formatJSONOrOriginal(resolved.Body)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			fatalf("create fixture directory for %q: %v", targetPath, err)
		}
		if err := os.WriteFile(targetPath, formatted, 0o644); err != nil {
			fatalf("write fixture %q: %v", targetPath, err)
		}

		fmt.Printf("+ refreshed %s -> %s\n", spec.Ref, targetPath)
		refreshed++
	}

	if err := writeMatrixTSV(outputRoot, matrix); err != nil {
		fatalf("write endpoint matrix: %v", err)
	}

	fmt.Printf("refresh complete: refreshed=%d kept-existing=%d total=%d\n", refreshed, keptExisting, len(matrix))
}

func formatJSONOrOriginal(raw []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return raw
	}
	out.WriteByte('\n')
	return out.Bytes()
}

func writeMatrixTSV(outputRoot string, matrix []cricinfo.FixtureSpec) error {
	var b strings.Builder
	b.WriteString("family\tname\tref\tfixture_path\tlive_probe\n")
	for _, spec := range matrix {
		line := fmt.Sprintf("%s\t%s\t%s\t%s\t%t\n", spec.Family, spec.Name, spec.Ref, spec.FixturePath, spec.LiveProbe)
		b.WriteString(line)
	}

	path := filepath.Join(outputRoot, "endpoint-matrix.tsv")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func ensurePathUnderRoot(root, path string) error {
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	if cleanPath == cleanRoot {
		return nil
	}
	prefix := cleanRoot + string(os.PathSeparator)
	if !strings.HasPrefix(cleanPath, prefix) {
		return fmt.Errorf("path %q escapes root %q", path, root)
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
