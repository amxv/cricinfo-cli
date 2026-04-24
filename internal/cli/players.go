package cli

import (
	"context"
	"fmt"
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
	MatchStats(ctx context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
	Innings(ctx context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
	Dismissals(ctx context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
	Deliveries(ctx context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
	Bowling(ctx context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
	Batting(ctx context.Context, playerQuery, matchQuery string, opts cricinfo.PlayerLookupOptions) (cricinfo.NormalizedResult, error)
}

type playerRuntimeOptions struct {
	leagueID string
	limit    int
	match    string
	scope    string
	mapMode  string
	mapLimit int
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
			"  cricinfo players match-stats <player> --match <match>",
			"  cricinfo players innings <player> --match <match>",
			"  cricinfo players dismissals <player> --match <match>",
			"  cricinfo players deliveries <player> --match <match>",
			"  cricinfo players bowling <player> --match <match>",
			"  cricinfo players batting <player> --match <match>",
			"  cricinfo players map-history <player> --scope season:2025 --league 8048",
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

	matchStatsCmd := &cobra.Command{
		Use:   "match-stats <player>",
		Short: "Show player-in-match batting/bowling/fielding statistics",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			playerQuery := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.MatchStats(ctx, playerQuery, opts.match, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}
	matchStatsCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias")

	inningsCmd := &cobra.Command{
		Use:   "innings <player>",
		Short: "Show player innings splits from roster-player linescores",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			playerQuery := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.Innings(ctx, playerQuery, opts.match, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}
	inningsCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias")

	dismissalsCmd := &cobra.Command{
		Use:   "dismissals <player>",
		Short: "Show dismissal and wicket views for a player in one match",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			playerQuery := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.Dismissals(ctx, playerQuery, opts.match, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}
	dismissalsCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias")

	deliveriesCmd := &cobra.Command{
		Use:   "deliveries <player>",
		Short: "Show player delivery events (including coordinate-aware shots/balls) for one match",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			playerQuery := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.Deliveries(ctx, playerQuery, opts.match, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}
	deliveriesCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias")

	bowlingCmd := &cobra.Command{
		Use:   "bowling <player>",
		Short: "Show player-in-match bowling split categories",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			playerQuery := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.Bowling(ctx, playerQuery, opts.match, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}
	bowlingCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias")

	battingCmd := &cobra.Command{
		Use:   "batting <player>",
		Short: "Show player-in-match batting split categories",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.match) == "" {
				return fmt.Errorf("--match is required")
			}
			playerQuery := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerCommand(cmd, global, func(ctx context.Context, service playerCommandService) (cricinfo.NormalizedResult, error) {
				return service.Batting(ctx, playerQuery, opts.match, cricinfo.PlayerLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}
	battingCmd.Flags().StringVar(&opts.match, "match", "", "Required: match ID/ref/alias")

	mapHistoryCmd := &cobra.Command{
		Use:   "map-history <player>",
		Short: "Show historical aggregated batting and bowling maps for a player",
		Long: strings.Join([]string{
			"Aggregate player maps across a historical scope and render visual text output.",
			"Use --scope match:<match> for one match or --scope season:<season> for season-wide maps.",
			"",
			"Examples:",
			"  cricinfo players map-history Virat Kohli --scope season:2025 --league 8048",
			"  cricinfo players map-history Jason Holder --scope match:1529277 --league 8048 --mode bowling",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !strings.EqualFold(global.format, "text") {
				return fmt.Errorf("players map-history currently supports --format text only")
			}
			if strings.TrimSpace(opts.scope) == "" {
				return fmt.Errorf("--scope is required (match:<match> or season:<season>)")
			}
			playerQuery := strings.TrimSpace(strings.Join(args, " "))
			return runPlayerMapHistoryCommand(cmd, opts, playerQuery)
		},
	}
	mapHistoryCmd.Flags().StringVar(&opts.scope, "scope", "", "Required: match:<match> or season:<season>")
	mapHistoryCmd.Flags().StringVar(&opts.mapMode, "mode", "both", "Map mode: batting, bowling, or both")
	mapHistoryCmd.Flags().IntVar(&opts.mapLimit, "match-limit", 0, "Optional cap on scoped matches before aggregation")

	cmd.AddCommand(
		searchCmd,
		profileCmd,
		newsCmd,
		statsCmd,
		careerCmd,
		matchStatsCmd,
		inningsCmd,
		dismissalsCmd,
		deliveriesCmd,
		bowlingCmd,
		battingCmd,
		mapHistoryCmd,
	)
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
