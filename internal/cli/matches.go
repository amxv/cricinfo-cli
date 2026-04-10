package cli

import (
	"context"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type matchCommandService interface {
	Close() error
	List(ctx context.Context, opts cricinfo.MatchListOptions) (cricinfo.NormalizedResult, error)
	Live(ctx context.Context, opts cricinfo.MatchListOptions) (cricinfo.NormalizedResult, error)
	Show(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Status(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
}

type matchRuntimeOptions struct {
	limit    int
	leagueID string
}

var newMatchService = func() (matchCommandService, error) {
	return cricinfo.NewMatchService(cricinfo.MatchServiceConfig{})
}

func newMatchesCommand(global *globalOptions) *cobra.Command {
	opts := &matchRuntimeOptions{}

	cmd := &cobra.Command{
		Use:   "matches",
		Short: "Live and current match discovery with status and summary views.",
		Long: strings.Join([]string{
			"Discover current matches from Cricinfo events and inspect normalized match summaries.",
			"",
			"Next steps:",
			"  cricinfo matches live",
			"  cricinfo matches list",
			"  cricinfo matches show <match>",
			"  cricinfo matches status <match>",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches innings <match>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVar(&opts.leagueID, "league", "", "Preferred league ID for match resolution context")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List current matches from /events",
		Long: strings.Join([]string{
			"Traverse current events and list normalized matches with teams, state, date, venue, and score summary.",
			"",
			"Next steps:",
			"  cricinfo matches show <match>",
			"  cricinfo matches status <match>",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches innings <match>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.List(ctx, cricinfo.MatchListOptions{Limit: opts.limit})
			})
		},
	}
	listCmd.Flags().IntVar(&opts.limit, "limit", 20, "Maximum number of matches to return")

	liveCmd := &cobra.Command{
		Use:   "live",
		Short: "List current live matches from /events",
		Long: strings.Join([]string{
			"Traverse current events and return only in-progress live matches.",
			"",
			"Next steps:",
			"  cricinfo matches show <match>",
			"  cricinfo matches status <match>",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches innings <match>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Live(ctx, cricinfo.MatchListOptions{Limit: opts.limit})
			})
		},
	}
	liveCmd.Flags().IntVar(&opts.limit, "limit", 20, "Maximum number of live matches to return")

	showCmd := &cobra.Command{
		Use:   "show <match>",
		Short: "Show one match summary",
		Long: strings.Join([]string{
			"Resolve a match by ID/ref/alias and show the normalized summary with teams, state, date, venue, and scores.",
			"",
			"Next steps:",
			"  cricinfo matches status <match>",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches innings <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Show(ctx, query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status <match>",
		Short: "Show one match status",
		Long: strings.Join([]string{
			"Resolve a match by ID/ref/alias and show the current match status summary.",
			"",
			"Next steps:",
			"  cricinfo matches show <match>",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches innings <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Status(ctx, query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	cmd.AddCommand(liveCmd, listCmd, showCmd, statusCmd)
	return cmd
}

func runMatchCommand(
	cmd *cobra.Command,
	global *globalOptions,
	fn func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error),
) error {
	service, err := newMatchService()
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
