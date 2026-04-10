package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type teamCommandService interface {
	Close() error
	Show(ctx context.Context, query string, opts cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error)
	Roster(ctx context.Context, query string, opts cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error)
	Scores(ctx context.Context, query string, opts cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error)
	Leaders(ctx context.Context, query string, opts cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error)
	Statistics(ctx context.Context, query string, opts cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error)
	Records(ctx context.Context, query string, opts cricinfo.TeamLookupOptions) (cricinfo.NormalizedResult, error)
}

type teamRuntimeOptions struct {
	leagueID string
	match    string
}

var newTeamService = func() (teamCommandService, error) {
	return cricinfo.NewTeamService(cricinfo.TeamServiceConfig{})
}

func newTeamsCommand(global *globalOptions) *cobra.Command {
	opts := &teamRuntimeOptions{}

	cmd := &cobra.Command{
		Use:   "teams",
		Short: "Team and competitor views with roster, leaders, scores, statistics, and records.",
		Long: strings.Join([]string{
			"Resolve teams by ID/ref/alias and drill into global team resources or match-scoped competitor resources.",
			"Use --match to force competitor scope when a route is match-specific.",
			"",
			"Next steps:",
			"  cricinfo teams show <team>",
			"  cricinfo teams roster <team>",
			"  cricinfo teams roster <team> --match <match>",
			"  cricinfo teams leaders <team> --match <match>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVar(&opts.leagueID, "league", "", "Preferred league ID for resolver context")

	showCmd := &cobra.Command{
		Use:   "show <team>",
		Short: "Show one team summary",
		Long: strings.Join([]string{
			"Resolve a team by ID/ref/alias and show normalized identity fields.",
			"",
			"Next steps:",
			"  cricinfo teams roster <team>",
			"  cricinfo teams leaders <team> --match <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runTeamCommand(cmd, global, func(ctx context.Context, service teamCommandService) (cricinfo.NormalizedResult, error) {
				return service.Show(ctx, query, cricinfo.TeamLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	rosterCmd := &cobra.Command{
		Use:   "roster <team>",
		Short: "Show team roster (global athletes or match-scoped competitor roster)",
		Long: strings.Join([]string{
			"Without --match, roster resolves global team athletes.",
			"With --match, roster resolves the match competitor roster and bridges entries to player refs.",
			"",
			"Next steps:",
			"  cricinfo teams leaders <team> --match <match>",
			"  cricinfo teams statistics <team> --match <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runTeamCommand(cmd, global, func(ctx context.Context, service teamCommandService) (cricinfo.NormalizedResult, error) {
				return service.Roster(ctx, query, cricinfo.TeamLookupOptions{LeagueID: opts.leagueID, MatchQuery: opts.match})
			})
		},
	}
	rosterCmd.Flags().StringVar(&opts.match, "match", "", "Match ID/ref/alias for match-scoped competitor roster")

	scoresCmd := &cobra.Command{
		Use:   "scores <team>",
		Short: "Show team score from a specific match competitor",
		Long: strings.Join([]string{
			"Resolve a team and match, then show the competitor score payload for that match.",
			"",
			"Next steps:",
			"  cricinfo teams leaders <team> --match <match>",
			"  cricinfo teams records <team> --match <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			return runTeamCommand(cmd, global, func(ctx context.Context, service teamCommandService) (cricinfo.NormalizedResult, error) {
				return service.Scores(ctx, query, cricinfo.TeamLookupOptions{LeagueID: opts.leagueID, MatchQuery: opts.match})
			})
		},
	}
	scoresCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias for competitor score route")

	leadersCmd := &cobra.Command{
		Use:   "leaders <team>",
		Short: "Show team batting and bowling leaders for a specific match",
		Long: strings.Join([]string{
			"Resolve a team and match, then render batting and bowling leaderboards from the competitor leaders route.",
			"",
			"Next steps:",
			"  cricinfo teams statistics <team> --match <match>",
			"  cricinfo teams records <team> --match <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			return runTeamCommand(cmd, global, func(ctx context.Context, service teamCommandService) (cricinfo.NormalizedResult, error) {
				return service.Leaders(ctx, query, cricinfo.TeamLookupOptions{LeagueID: opts.leagueID, MatchQuery: opts.match})
			})
		},
	}
	leadersCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias for competitor leaders route")

	statisticsCmd := &cobra.Command{
		Use:   "statistics <team>",
		Short: "Show team statistics categories for a specific match",
		Long: strings.Join([]string{
			"Resolve a team and match, then render competitor statistics categories.",
			"",
			"Next steps:",
			"  cricinfo teams records <team> --match <match>",
			"  cricinfo teams leaders <team> --match <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			return runTeamCommand(cmd, global, func(ctx context.Context, service teamCommandService) (cricinfo.NormalizedResult, error) {
				return service.Statistics(ctx, query, cricinfo.TeamLookupOptions{LeagueID: opts.leagueID, MatchQuery: opts.match})
			})
		},
	}
	statisticsCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias for competitor statistics route")

	recordsCmd := &cobra.Command{
		Use:   "records <team>",
		Short: "Show team records categories for a specific match",
		Long: strings.Join([]string{
			"Resolve a team and match, then render competitor records categories.",
			"",
			"Next steps:",
			"  cricinfo teams scores <team> --match <match>",
			"  cricinfo teams leaders <team> --match <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			return runTeamCommand(cmd, global, func(ctx context.Context, service teamCommandService) (cricinfo.NormalizedResult, error) {
				return service.Records(ctx, query, cricinfo.TeamLookupOptions{LeagueID: opts.leagueID, MatchQuery: opts.match})
			})
		},
	}
	recordsCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias for competitor records route")

	cmd.AddCommand(showCmd, rosterCmd, scoresCmd, leadersCmd, statisticsCmd, recordsCmd)
	return cmd
}

func runTeamCommand(
	cmd *cobra.Command,
	global *globalOptions,
	fn func(ctx context.Context, service teamCommandService) (cricinfo.NormalizedResult, error),
) error {
	service, err := newTeamService()
	if err != nil {
		return err
	}
	defer func() {
		_ = service.Close()
	}()

	result, err := fn(cmd.Context(), service)
	if err != nil {
		return err
	}

	return cricinfo.Render(cmd.OutOrStdout(), result, cricinfo.RenderOptions{
		Format:    global.format,
		Verbose:   global.verbose,
		AllFields: global.allFields,
	})
}
