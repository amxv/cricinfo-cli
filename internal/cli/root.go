package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/buildinfo"
	"github.com/spf13/cobra"
)

const commandName = "cricinfo"

type globalOptions struct {
	format    string
	verbose   bool
	allFields bool
	version   bool
}

func Run(args []string, stdout, stderr io.Writer) error {
	root := newRootCommand(stdout, stderr)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		return normalizeCommandError(err)
	}

	return nil
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	opts := &globalOptions{}

	root := &cobra.Command{
		Use:                commandName,
		Short:              "Explore Cricinfo cricket data from the command line.",
		Long:               rootLongDescription(),
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableAutoGenTag:  true,
		DisableSuggestions: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return validateFormat(opts.format)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.version {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", commandName, buildinfo.CurrentVersion())
				return nil
			}

			return cmd.Help()
		},
	}

	root.SetOut(stdout)
	root.SetErr(stderr)

	root.Flags().BoolVar(&opts.version, "version", false, "Print version information")
	root.PersistentFlags().StringVar(&opts.format, "format", "text", "Output format: text, json, or jsonl")
	root.PersistentFlags().BoolVar(&opts.verbose, "verbose", false, "Show verbose output when available")
	root.PersistentFlags().BoolVar(&opts.allFields, "all-fields", false, "Include long-tail fields in output")

	root.AddCommand(newPlaceholderGroupCommand(
		"matches",
		"Live and historical match discovery, scorecards, and ball-by-ball views.",
		[]string{
			"cricinfo matches --help",
			"cricinfo search --help",
		},
	))
	root.AddCommand(newPlaceholderGroupCommand(
		"players",
		"Player discovery, profiles, and match-context statistics.",
		[]string{
			"cricinfo players --help",
			"cricinfo search --help",
		},
	))
	root.AddCommand(newPlaceholderGroupCommand(
		"teams",
		"Team and competitor views including roster, leaders, and records.",
		[]string{
			"cricinfo teams --help",
			"cricinfo matches --help",
		},
	))
	root.AddCommand(newPlaceholderGroupCommand(
		"leagues",
		"League-level discovery for events, seasons, and calendars.",
		[]string{
			"cricinfo leagues --help",
			"cricinfo standings --help",
		},
	))
	root.AddCommand(newPlaceholderGroupCommand(
		"seasons",
		"Season navigation across league season, type, and group hierarchies.",
		[]string{
			"cricinfo seasons --help",
			"cricinfo leagues --help",
		},
	))
	root.AddCommand(newPlaceholderGroupCommand(
		"standings",
		"Standings and table exploration across league competition structures.",
		[]string{
			"cricinfo standings --help",
			"cricinfo leagues --help",
		},
	))
	root.AddCommand(newPlaceholderGroupCommand(
		"competitions",
		"Competition metadata including officials, broadcasts, and odds.",
		[]string{
			"cricinfo competitions --help",
			"cricinfo matches --help",
		},
	))
	root.AddCommand(newPlaceholderGroupCommand(
		"search",
		"Cross-entity discovery for matches, players, teams, and leagues.",
		[]string{
			"cricinfo search --help",
			"cricinfo players --help",
		},
	))
	root.AddCommand(newPlaceholderGroupCommand(
		"analysis",
		"Derived cricket analysis over normalized command output.",
		[]string{
			"cricinfo analysis --help",
			"cricinfo matches --help",
		},
	))

	return root
}

func newPlaceholderGroupCommand(name, description string, nextSteps []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: description,
		Long: fmt.Sprintf("%s\n\n%s",
			description,
			formatNextSteps(nextSteps),
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	return cmd
}

func validateFormat(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text", "json", "jsonl":
		return nil
	default:
		return fmt.Errorf("invalid value %q for --format (expected: text, json, jsonl)", value)
	}
}

func normalizeCommandError(err error) error {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return err
	}

	firstLine := strings.SplitN(message, "\n", 2)[0]

	if strings.HasPrefix(firstLine, "unknown command ") || strings.HasPrefix(firstLine, "unknown flag:") {
		return fmt.Errorf("%s (run `%s --help`)", firstLine, commandName)
	}

	return fmt.Errorf("%s", firstLine)
}

func rootLongDescription() string {
	return strings.Join([]string{
		"Domain-driven Cricinfo CLI for exploring matches, players, teams, and leagues.",
		"",
		"Next steps:",
		"  cricinfo matches --help",
		"  cricinfo players --help",
		"  cricinfo search --help",
	}, "\n")
}

func formatNextSteps(commands []string) string {
	lines := []string{"Next steps:"}
	for _, command := range commands {
		lines = append(lines, "  "+command)
	}
	return strings.Join(lines, "\n")
}
