package cli

import (
	"context"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type playerCommandService interface {
	Close() error
	Search(ctx context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
	Profile(ctx context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
	News(ctx context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
	Stats(ctx context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
	Career(ctx context.Context, query string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
}

type playerRuntimeOptions struct {
	leagueID string
	limit    int
}

var newPlayerService = func() (playerCommandService, error) {
	return cricinfo.NewPlayerService(cricinfo.PlayerServiceConfig{})
}

func newPlayersCommand(global *globalOptions) *cobra.Command {
	opts := &playerRuntimeOptions{}

	cmd := &cobra.Command{
		Use:   "players",
		Short: "Player discovery with profile, news, and grouped career statistics.",
		Long: strings.Join([]string{
			"Resolve players by ID/ref/alias and inspect normalized profile, related news, and grouped statistics.",
			"",
			"Next steps:",
			"  cricinfo players search <query>",
			"  cricinfo players profile <player>",
			"  cricinfo players news <player>",
			"  cricinfo players stats <player>",
			"  cricinfo players career <player>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVar(&opts.leagueID, "league", "", "Preferred league ID for resolver context")
	cmd.PersistentFlags().IntVar(&opts.limit, "limit", 10, "Maximum number of results to return for search and news")

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search players by ID, ref, or alias",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.Search(ctx, query, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID, Limit: opts.limit})
			})
		},
	}

	profileCmd := &cobra.Command{
		Use:   "profile <player>",
		Short: "Show one normalized player profile",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.Profile(ctx, query, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	newsCmd := &cobra.Command{
		Use:   "news <player>",
		Short: "Show normalized related news articles for one player",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.News(ctx, query, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID, Limit: opts.limit})
			})
		},
	}

	statsCmd := &cobra.Command{
		Use:   "stats <player>",
		Short: "Show grouped global player statistics",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.Stats(ctx, query, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	careerCmd := &cobra.Command{
		Use:   "career <player>",
		Short: "Show grouped career statistics for one player",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.Career(ctx, query, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	cmd.AddCommand(searchCmd, profileCmd, newsCmd, statsCmd, careerCmd)
	return cmd
}

func runPlayerCommand(
	cmd *cobra.Command,
	global *globalOptions,
	fn func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error),
) error {
	service, err := newPlayerService()
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
