package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type leagueCommandService interface {
	Close() error
	List(ctx context.Context, opts cricinfo.LeagueListOptions) (cricinfo.NormalizedResult, error)
	Show(ctx context.Context, leagueQuery string) (cricinfo.NormalizedResult, error)
	Events(ctx context.Context, leagueQuery string, opts cricinfo.LeagueListOptions) (cricinfo.NormalizedResult, error)
	Calendar(ctx context.Context, leagueQuery string) (cricinfo.NormalizedResult, error)
	Athletes(ctx context.Context, leagueQuery string, opts cricinfo.LeagueListOptions) (cricinfo.NormalizedResult, error)
	Standings(ctx context.Context, leagueQuery string) (cricinfo.NormalizedResult, error)
	Seasons(ctx context.Context, leagueQuery string) (cricinfo.NormalizedResult, error)
	SeasonShow(ctx context.Context, leagueQuery string, opts cricinfo.SeasonLookupOptions) (cricinfo.NormalizedResult, error)
	SeasonTypes(ctx context.Context, leagueQuery string, opts cricinfo.SeasonLookupOptions) (cricinfo.NormalizedResult, error)
	SeasonGroups(ctx context.Context, leagueQuery string, opts cricinfo.SeasonLookupOptions) (cricinfo.NormalizedResult, error)
}

type leagueRuntimeOptions struct {
	limit  int
	season string
	typ    string
}

var newLeagueService = func() (leagueCommandService, error) {
	return cricinfo.NewLeagueService(cricinfo.LeagueServiceConfig{})
}

func newLeaguesCommand(global *globalOptions) *cobra.Command {
	opts := &leagueRuntimeOptions{}

	cmd := &cobra.Command{
		Use:   "leagues",
		Short: "League discovery and navigation across events, calendar, athletes, standings, and seasons.",
		Long: strings.Join([]string{
			"Resolve leagues by ID/ref/alias and drill into events, calendar views, athletes, standings, and season navigation.",
			"",
			"Next steps:",
			"  cricinfo leagues list",
			"  cricinfo leagues show <league>",
			"  cricinfo leagues events <league>",
			"  cricinfo leagues calendar <league>",
			"  cricinfo leagues athletes <league>",
			"  cricinfo leagues standings <league>",
			"  cricinfo leagues seasons <league>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List leagues from /leagues",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.List(ctx, cricinfo.LeagueListOptions{Limit: opts.limit})
			})
		},
	}
	listCmd.Flags().IntVar(&opts.limit, "limit", 20, "Maximum number of leagues to return")

	showCmd := &cobra.Command{
		Use:   "show <league>",
		Short: "Show one league summary",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.Show(ctx, leagueQuery)
			})
		},
	}

	eventsCmd := &cobra.Command{
		Use:   "events <league>",
		Short: "List matches/events for one league",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.Events(ctx, leagueQuery, cricinfo.LeagueListOptions{Limit: opts.limit})
			})
		},
	}
	eventsCmd.Flags().IntVar(&opts.limit, "limit", 20, "Maximum number of league events/matches to return")

	calendarCmd := &cobra.Command{
		Use:   "calendar <league>",
		Short: "Show normalized league calendar day entries",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.Calendar(ctx, leagueQuery)
			})
		},
	}

	athletesCmd := &cobra.Command{
		Use:   "athletes <league>",
		Short: "Show league athlete views, with roster-driven fallback when direct pages are sparse",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.Athletes(ctx, leagueQuery, cricinfo.LeagueListOptions{Limit: opts.limit})
			})
		},
	}
	athletesCmd.Flags().IntVar(&opts.limit, "limit", 20, "Maximum number of athletes to return")

	standingsCmd := &cobra.Command{
		Use:   "standings <league>",
		Short: "Show normalized standings groups for one league",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.Standings(ctx, leagueQuery)
			})
		},
	}

	seasonsCmd := &cobra.Command{
		Use:   "seasons <league>",
		Short: "Show season refs for one league",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.Seasons(ctx, leagueQuery)
			})
		},
	}

	cmd.AddCommand(
		listCmd,
		showCmd,
		eventsCmd,
		calendarCmd,
		athletesCmd,
		standingsCmd,
		seasonsCmd,
	)
	return cmd
}

func newSeasonsCommand(global *globalOptions) *cobra.Command {
	opts := &leagueRuntimeOptions{}

	cmd := &cobra.Command{
		Use:   "seasons",
		Short: "Season navigation across league season, type, and group hierarchies.",
		Long: strings.Join([]string{
			"Resolve seasons, types, and groups by league while hiding pointer-heavy upstream traversal.",
			"",
			"Next steps:",
			"  cricinfo seasons show <league> --season <season>",
			"  cricinfo seasons types <league> --season <season>",
			"  cricinfo seasons groups <league> --season <season> --type <type>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <league>",
		Short: "Show one selected league season",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.season) == "" {
				return fmt.Errorf("--season is required")
			}
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.SeasonShow(ctx, leagueQuery, cricinfo.SeasonLookupOptions{
					SeasonQuery: opts.season,
				})
			})
		},
	}
	showCmd.Flags().StringVar(&opts.season, "season", "", "Required: season ID/ref (for example 2025)")

	typesCmd := &cobra.Command{
		Use:   "types <league>",
		Short: "List normalized season types for a selected league season",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.season) == "" {
				return fmt.Errorf("--season is required")
			}
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.SeasonTypes(ctx, leagueQuery, cricinfo.SeasonLookupOptions{
					SeasonQuery: opts.season,
				})
			})
		},
	}
	typesCmd.Flags().StringVar(&opts.season, "season", "", "Required: season ID/ref (for example 2025)")

	groupsCmd := &cobra.Command{
		Use:   "groups <league>",
		Short: "List normalized season groups for a selected season type",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.season) == "" {
				return fmt.Errorf("--season is required")
			}
			if strings.TrimSpace(opts.typ) == "" {
				return fmt.Errorf("--type is required")
			}
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.SeasonGroups(ctx, leagueQuery, cricinfo.SeasonLookupOptions{
					SeasonQuery: opts.season,
					TypeQuery:   opts.typ,
				})
			})
		},
	}
	groupsCmd.Flags().StringVar(&opts.season, "season", "", "Required: season ID/ref (for example 2025)")
	groupsCmd.Flags().StringVar(&opts.typ, "type", "", "Required: type ID/ref/name (for example 1)")

	cmd.AddCommand(showCmd, typesCmd, groupsCmd)
	return cmd
}

func newStandingsCommand(global *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "standings",
		Short: "Standings traversal by league.",
		Long: strings.Join([]string{
			"Traverse league standings routes and normalize ref-heavy standings resources into group entries.",
			"",
			"Next steps:",
			"  cricinfo standings show <league>",
			"  cricinfo leagues standings <league>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <league>",
		Short: "Show normalized standings groups for one league",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			leagueQuery := strings.TrimSpace(strings.Join(args, " "))
			return runLeagueCommand(cmd, global, func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error) {
				return service.Standings(ctx, leagueQuery)
			})
		},
	}

	cmd.AddCommand(showCmd)
	return cmd
}

func runLeagueCommand(
	cmd *cobra.Command,
	global *globalOptions,
	fn func(ctx context.Context, service leagueCommandService) (cricinfo.NormalizedResult, error),
) error {
	service, err := newLeagueService()
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
