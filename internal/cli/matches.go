package cli

import (
	"context"
	"fmt"
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
	Scorecard(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Details(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Plays(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Situation(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Innings(ctx context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error)
	Partnerships(ctx context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error)
	FallOfWicket(ctx context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error)
	Deliveries(ctx context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error)
}

type matchRuntimeOptions struct {
	limit    int
	leagueID string
	team     string
	innings  int
	period   int
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
				return service.List(ctx, cricinfo.MatchListOptions{
					Limit:    opts.limit,
					LeagueID: opts.leagueID,
				})
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
				return service.Live(ctx, cricinfo.MatchListOptions{
					Limit:    opts.limit,
					LeagueID: opts.leagueID,
				})
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

	scorecardCmd := &cobra.Command{
		Use:   "scorecard <match>",
		Short: "Show batting, bowling, and partnerships scorecards for one match",
		Long: strings.Join([]string{
			"Resolve a match and render normalized batting, bowling, and partnerships scorecard views.",
			"",
			"Next steps:",
			"  cricinfo matches details <match>",
			"  cricinfo matches plays <match>",
			"  cricinfo matches situation <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Scorecard(ctx, query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	detailsCmd := &cobra.Command{
		Use:   "details <match>",
		Short: "Show normalized delivery events from match details",
		Long: strings.Join([]string{
			"Resolve a match and render normalized detail delivery events with batsman/bowler refs, score value, dismissal, and over context.",
			"",
			"Next steps:",
			"  cricinfo matches plays <match>",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches situation <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Details(ctx, query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	playsCmd := &cobra.Command{
		Use:   "plays <match>",
		Short: "Show normalized delivery events from match plays",
		Long: strings.Join([]string{
			"Resolve a match and render normalized play delivery events.",
			"",
			"Next steps:",
			"  cricinfo matches details <match>",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches situation <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Plays(ctx, query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	situationCmd := &cobra.Command{
		Use:   "situation <match>",
		Short: "Show match situation data when available",
		Long: strings.Join([]string{
			"Resolve a match and render normalized situation data. Sparse situation payloads are treated as valid empty results.",
			"",
			"Next steps:",
			"  cricinfo matches status <match>",
			"  cricinfo matches details <match>",
			"  cricinfo matches scorecard <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Situation(ctx, query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	inningsCmd := &cobra.Command{
		Use:   "innings <match>",
		Short: "Show innings summaries with over and wicket timelines",
		Long: strings.Join([]string{
			"Resolve a match and return normalized innings summaries.",
			"Use --team to focus on one team competitor.",
			"",
			"Next steps:",
			"  cricinfo matches partnerships <match> --team <team> --innings <n> --period <n>",
			"  cricinfo matches fow <match> --team <team> --innings <n> --period <n>",
			"  cricinfo matches deliveries <match> --team <team> --innings <n> --period <n>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Innings(ctx, query, cricinfo.MatchInningsOptions{
					LeagueID:  opts.leagueID,
					TeamQuery: opts.team,
				})
			})
		},
	}
	inningsCmd.Flags().StringVar(&opts.team, "team", "", "Optional: team ID/ref/alias to scope innings")

	partnershipsCmd := &cobra.Command{
		Use:   "partnerships <match>",
		Short: "Show partnerships for a selected team innings period",
		Long: strings.Join([]string{
			"Resolve one match and team, select innings/period, and render detailed partnership objects.",
			"",
			"Required flags:",
			"  --team <team>",
			"  --innings <n>",
			"  --period <n>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.team) == "" {
				return fmt.Errorf("--team is required")
			}
			if opts.innings <= 0 {
				return fmt.Errorf("--innings is required")
			}
			if opts.period <= 0 {
				return fmt.Errorf("--period is required")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Partnerships(ctx, query, cricinfo.MatchInningsOptions{
					LeagueID:  opts.leagueID,
					TeamQuery: opts.team,
					Innings:   opts.innings,
					Period:    opts.period,
				})
			})
		},
	}
	partnershipsCmd.Flags().StringVar(&opts.team, "team", "", "Required: team ID/ref/alias")
	partnershipsCmd.Flags().IntVar(&opts.innings, "innings", 0, "Required: innings number")
	partnershipsCmd.Flags().IntVar(&opts.period, "period", 0, "Required: period number")

	fowCmd := &cobra.Command{
		Use:   "fow <match>",
		Short: "Show fall-of-wicket entries for a selected team innings period",
		Long: strings.Join([]string{
			"Resolve one match and team, select innings/period, and render detailed fall-of-wicket objects.",
			"",
			"Required flags:",
			"  --team <team>",
			"  --innings <n>",
			"  --period <n>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.team) == "" {
				return fmt.Errorf("--team is required")
			}
			if opts.innings <= 0 {
				return fmt.Errorf("--innings is required")
			}
			if opts.period <= 0 {
				return fmt.Errorf("--period is required")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.FallOfWicket(ctx, query, cricinfo.MatchInningsOptions{
					LeagueID:  opts.leagueID,
					TeamQuery: opts.team,
					Innings:   opts.innings,
					Period:    opts.period,
				})
			})
		},
	}
	fowCmd.Flags().StringVar(&opts.team, "team", "", "Required: team ID/ref/alias")
	fowCmd.Flags().IntVar(&opts.innings, "innings", 0, "Required: innings number")
	fowCmd.Flags().IntVar(&opts.period, "period", 0, "Required: period number")

	deliveriesCmd := &cobra.Command{
		Use:   "deliveries <match>",
		Short: "Show over-by-over and wicket timelines for a selected innings period",
		Long: strings.Join([]string{
			"Resolve one match and team, select innings/period, and render normalized over and wicket timelines from period statistics.",
			"",
			"Required flags:",
			"  --team <team>",
			"  --innings <n>",
			"  --period <n>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.team) == "" {
				return fmt.Errorf("--team is required")
			}
			if opts.innings <= 0 {
				return fmt.Errorf("--innings is required")
			}
			if opts.period <= 0 {
				return fmt.Errorf("--period is required")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Deliveries(ctx, query, cricinfo.MatchInningsOptions{
					LeagueID:  opts.leagueID,
					TeamQuery: opts.team,
					Innings:   opts.innings,
					Period:    opts.period,
				})
			})
		},
	}
	deliveriesCmd.Flags().StringVar(&opts.team, "team", "", "Required: team ID/ref/alias")
	deliveriesCmd.Flags().IntVar(&opts.innings, "innings", 0, "Required: innings number")
	deliveriesCmd.Flags().IntVar(&opts.period, "period", 0, "Required: period number")

	cmd.AddCommand(
		liveCmd,
		listCmd,
		showCmd,
		statusCmd,
		scorecardCmd,
		detailsCmd,
		playsCmd,
		situationCmd,
		inningsCmd,
		partnershipsCmd,
		fowCmd,
		deliveriesCmd,
	)
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
