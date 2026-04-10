package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRootHelp(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer

	err := Run([]string{"--help"}, &out, &errBuf)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "Usage:") {
		t.Fatalf("expected help output, got: %q", out.String())
	}
	if !strings.Contains(help, "--version") {
		t.Fatalf("expected --version in help output, got: %q", out.String())
	}
	if !strings.Contains(help, "matches") || !strings.Contains(help, "players") {
		t.Fatalf("expected placeholder groups in root help, got: %q", help)
	}
}

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer

	err := Run([]string{"--version"}, &out, &errBuf)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(out.String(), "cricinfo ") {
		t.Fatalf("unexpected version output: %q", out.String())
	}
}

func TestRunPlaceholderGroupHelp(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer

	err := Run([]string{"matches", "--help"}, &out, &errBuf)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "Next steps:") {
		t.Fatalf("expected next-step guidance in help output, got: %q", help)
	}
	if !strings.Contains(help, "cricinfo matches --help") {
		t.Fatalf("expected drill-down command in help output, got: %q", help)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer

	err := Run([]string{"unknown"}, &out, &errBuf)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), `unknown command "unknown"`) {
		t.Fatalf("expected unknown command error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "run `cricinfo --help`") {
		t.Fatalf("expected unknown command help hint, got: %v", err)
	}
}

func TestRunGlobalFlagsPropagateToSubcommands(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer

	err := Run([]string{"players", "--format", "json", "--verbose", "--all-fields", "--help"}, &out, &errBuf)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "--format") || !strings.Contains(help, "--verbose") || !strings.Contains(help, "--all-fields") {
		t.Fatalf("expected global flags in subcommand help output, got: %q", help)
	}
}

func TestRunRejectsUnknownFormat(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer

	err := Run([]string{"players", "--format", "yaml"}, &out, &errBuf)
	if err == nil {
		t.Fatal("expected error for invalid --format value")
	}
	if !strings.Contains(err.Error(), `invalid value "yaml" for --format`) {
		t.Fatalf("unexpected invalid format error: %v", err)
	}
}

func TestRunSearchGroupHelp(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer

	err := Run([]string{"search", "--help"}, &out, &errBuf)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	help := out.String()
	if !strings.Contains(help, "players") || !strings.Contains(help, "teams") || !strings.Contains(help, "leagues") || !strings.Contains(help, "matches") {
		t.Fatalf("expected entity search subcommands in help output, got: %q", help)
	}
	if !strings.Contains(help, "--league") || !strings.Contains(help, "--match") {
		t.Fatalf("expected search context flags in help output, got: %q", help)
	}
}
