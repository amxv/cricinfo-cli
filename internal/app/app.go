package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/buildinfo"
)

const commandName = "cricinfo"

var version = buildinfo.CurrentVersion()

func Run(args []string, stdout, stderr io.Writer) error {
	_ = stderr

	if len(args) == 0 || isHelpArg(args[0]) {
		printRootHelp(stdout)
		return nil
	}
	if len(args) == 1 && isVersionArg(args[0]) {
		_, _ = fmt.Fprintf(stdout, "%s %s\n", commandName, version)
		return nil
	}

	switch args[0] {
	case "hello":
		if len(args) > 1 && isHelpArg(args[1]) {
			printHelloHelp(stdout)
			return nil
		}
		name := "world"
		if len(args) > 1 {
			name = strings.TrimSpace(args[1])
			if name == "" {
				name = "world"
			}
		}
		_, _ = fmt.Fprintf(stdout, "Hello, %s!\n", name)
		return nil
	default:
		return fmt.Errorf("unknown command %q (run `%s --help`)", args[0], commandName)
	}
}

func isVersionArg(v string) bool {
	return v == "--version"
}

func isHelpArg(v string) bool {
	switch v {
	case "-h", "--help", "help":
		return true
	default:
		return false
	}
}

func printRootHelp(w io.Writer) {
	writeLines(w,
		"cricinfo - Go CLI template",
		"",
		"Usage:",
		"  cricinfo [--version]",
		"  cricinfo <command> [arguments]",
		"",
		"Commands:",
		"  hello [name]    print a greeting",
		"",
		"Examples:",
		"  cricinfo --version",
		"  cricinfo hello",
		"  cricinfo hello agent",
	)
}

func printHelloHelp(w io.Writer) {
	writeLines(w,
		"cricinfo hello - print a greeting",
		"",
		"Usage:",
		"  cricinfo hello [name]",
		"",
		"Examples:",
		"  cricinfo hello",
		"  cricinfo hello Alice",
	)
}

func writeLines(w io.Writer, lines ...string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(w, line)
	}
}
