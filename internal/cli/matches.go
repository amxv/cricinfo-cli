package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type matchCommandService interface {
	Close() error
	List(ctx context.Context, opts cricinfo.MatchListOptions) (cricinfo.NormalizedResult, error)
	Live(ctx context.Context, opts cricinfo.MatchListOptions) (cricinfo.NormalizedResult, error)
	Lineups(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Show(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Status(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Scorecard(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Details(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Plays(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Situation(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	LiveView(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Duel(ctx context.Context, query string, opts cricinfo.MatchDuelOptions) (cricinfo.NormalizedResult, error)
	Phases(ctx context.Context, query string, opts cricinfo.MatchLookupOptions) (cricinfo.NormalizedResult, error)
	Innings(ctx context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error)
	Partnerships(ctx context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error)
	FallOfWicket(ctx context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error)
	Deliveries(ctx context.Context, query string, opts cricinfo.MatchInningsOptions) (cricinfo.NormalizedResult, error)
}

type matchRuntimeOptions struct {
	limit    int
	leagueID string
	team     string
	batter   string
	bowler   string
	player   string
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
			"  cricinfo matches live-view <match>",
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
			"  cricinfo matches live-view <match>",
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
			"  cricinfo matches live-view <match>",
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

	lineupCmd := &cobra.Command{
		Use:   "lineup <match>",
		Short: "Show starting lineups for both teams in one match",
		Long: strings.Join([]string{
			"Resolve one match and return the match-scoped roster entries for both teams.",
			"",
			"Next steps:",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches status <match>",
			"  cricinfo matches deliveries <match> --team <team> --innings <n> --period <n>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Lineups(ctx, query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <match>",
		Short: "Show one match summary",
		Long: strings.Join([]string{
			"Resolve a match by ID/ref/alias and show the normalized summary with teams, state, date, venue, and scores.",
			"",
			"Next steps:",
			"  cricinfo matches status <match>",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches live-view <match>",
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
			"  cricinfo matches live-view <match>",
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
			"  cricinfo matches live-view <match>",
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
			"  cricinfo matches live-view <match>",
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

	pitchMapCmd := &cobra.Command{
		Use:   "pitch-map <match>",
		Short: "Render a visual ASCII pitch map from delivery coordinates",
		Long: strings.Join([]string{
			"Resolve a match and render a text pitch map using delivery x/y coordinates.",
			"Use --player to focus on one player by ID, ref fragment, or name snippet from shortText.",
			"",
			"Notes:",
			"  - Balls without coordinates are counted as unplottable.",
			"  - If all balls are missing coordinates, the map is shown empty with diagnostics.",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !strings.EqualFold(global.format, "text") {
				return fmt.Errorf("matches pitch-map currently supports --format text only")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchPitchMapCommand(cmd, global, opts, query)
		},
	}
	pitchMapCmd.Flags().StringVar(&opts.player, "player", "", "Optional: player id/ref/name filter")

	situationCmd := &cobra.Command{
		Use:   "situation <match>",
		Short: "Show match situation data when available",
		Long: strings.Join([]string{
			"Resolve a match and render normalized situation data. Sparse situation payloads are treated as valid empty results.",
			"",
			"Next steps:",
			"  cricinfo matches status <match>",
			"  cricinfo matches details <match>",
			"  cricinfo matches live-view <match>",
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

	liveViewCmd := &cobra.Command{
		Use:   "live-view <match>",
		Short: "Show fan-first live snapshot (batters, bowlers, figures, recent balls)",
		Long: strings.Join([]string{
			"Resolve a match and render a synthesized live snapshot from delivery details and roster metadata.",
			"",
			"Shows:",
			"  - current batters with runs/balls/strike-rate/boundaries",
			"  - current bowlers with figures and economy",
			"  - recent balls with over.ball and running score",
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
				return service.LiveView(ctx, query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	duelCmd := &cobra.Command{
		Use:   "duel <match>",
		Short: "Show batter-vs-bowler matchup summary in one match",
		Long: strings.Join([]string{
			"Resolve a match and summarize the head-to-head duel between one batter and one bowler.",
			"",
			"Required flags:",
			"  --batter <player>",
			"  --bowler <player>",
			"",
			"Next steps:",
			"  cricinfo matches plays <match>",
			"  cricinfo players deliveries <player> --match <match>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.batter) == "" {
				return fmt.Errorf("--batter is required")
			}
			if strings.TrimSpace(opts.bowler) == "" {
				return fmt.Errorf("--bowler is required")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Duel(ctx, query, cricinfo.MatchDuelOptions{
					LeagueID:    opts.leagueID,
					BatterQuery: opts.batter,
					BowlerQuery: opts.bowler,
				})
			})
		},
	}
	duelCmd.Flags().StringVar(&opts.batter, "batter", "", "Required: batter ID/ref/alias")
	duelCmd.Flags().StringVar(&opts.bowler, "bowler", "", "Required: bowler ID/ref/alias")

	phasesCmd := &cobra.Command{
		Use:   "phases <match>",
		Short: "Show powerplay, middle, and death-over phase splits for each innings",
		Long: strings.Join([]string{
			"Resolve a match and show fan-friendly phase splits (powerplay/middle/death) with momentum markers.",
			"",
			"Next steps:",
			"  cricinfo matches scorecard <match>",
			"  cricinfo matches deliveries <match> --team <team> --innings <n> --period <n>",
			"  cricinfo matches fow <match> --team <team> --innings <n> --period <n>",
		}, "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runMatchCommand(cmd, global, func(ctx context.Context, service matchCommandService) (cricinfo.NormalizedResult, error) {
				return service.Phases(ctx, query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
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
		lineupCmd,
		showCmd,
		statusCmd,
		scorecardCmd,
		detailsCmd,
		playsCmd,
		pitchMapCmd,
		situationCmd,
		liveViewCmd,
		duelCmd,
		phasesCmd,
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

func runMatchPitchMapCommand(cmd *cobra.Command, global *globalOptions, opts *matchRuntimeOptions, query string) error {
	service, err := newMatchService()
	if err != nil {
		return err
	}
	defer func() {
		_ = service.Close()
	}()

	result, err := service.Details(cmd.Context(), query, cricinfo.MatchLookupOptions{LeagueID: opts.leagueID})
	if err != nil {
		return err
	}

	if result.Status == cricinfo.ResultStatusError {
		return cricinfo.Render(cmd.OutOrStdout(), result, cricinfo.RenderOptions{
			Format:    global.format,
			Verbose:   global.verbose,
			AllFields: global.allFields,
		})
	}

	deliveries := make([]cricinfo.DeliveryEvent, 0, len(result.Items))
	for _, item := range result.Items {
		if delivery, ok := item.(cricinfo.DeliveryEvent); ok {
			deliveries = append(deliveries, delivery)
			continue
		}
		if delivery, ok := item.(*cricinfo.DeliveryEvent); ok && delivery != nil {
			deliveries = append(deliveries, *delivery)
		}
	}

	filtered := filterDeliveriesByPlayer(deliveries, opts.player)
	plotted := 0
	for _, d := range filtered {
		if d.XCoordinate != nil && d.YCoordinate != nil {
			plotted++
		}
	}

	renderPitchMapText(
		cmd.OutOrStdout(),
		query,
		opts.player,
		filtered,
		result.Warnings,
	)

	if plotted == 0 && strings.TrimSpace(opts.player) != "" {
		leagueID := strings.TrimSpace(opts.leagueID)
		if leagueID == "" && len(deliveries) > 0 {
			leagueID = strings.TrimSpace(deliveries[0].LeagueID)
		}
		matchID := strings.TrimSpace(query)
		if matchID != "" && leagueID != "" {
			if bundle, err := fetchSiteAPIPitchMapGrid(cmd.Context(), leagueID, matchID, opts.player); err == nil && (len(bundle.RHB) > 0 || len(bundle.LHB) > 0) {
				renderSiteAPIPitchMapGrid(cmd.OutOrStdout(), bundle)
			} else if batting, berr := fetchSiteAPIBattingWagonMap(cmd.Context(), leagueID, matchID, opts.player); berr == nil && len(batting.ZoneRuns) > 0 {
				renderSiteAPIBattingWagonMap(cmd.OutOrStdout(), batting)
			}
		}
	}
	return nil
}

func renderPitchMapText(out io.Writer, matchQuery, playerFilter string, filtered []cricinfo.DeliveryEvent, warnings []string) {
	type point struct {
		x       float64
		y       float64
		wicket  bool
		labelID string
	}

	points := make([]point, 0, len(filtered))
	unplottable := 0
	for _, d := range filtered {
		if d.XCoordinate == nil || d.YCoordinate == nil {
			unplottable++
			continue
		}
		isWicket := false
		if raw, ok := d.Dismissal["dismissal"]; ok {
			if dismiss, ok := raw.(bool); ok && dismiss {
				isWicket = true
			}
		}
		points = append(points, point{
			x:       *d.XCoordinate,
			y:       *d.YCoordinate,
			wicket:  isWicket,
			labelID: d.ID,
		})
	}

	const rows = 13
	const cols = 29
	grid := make([][]rune, rows)
	for r := 0; r < rows; r++ {
		grid[r] = make([]rune, cols)
		for c := 0; c < cols; c++ {
			grid[r][c] = ' '
		}
	}

	minX, maxX := 0.0, 1.0
	minY, maxY := 0.0, 1.0
	if len(points) > 0 {
		minX, maxX = points[0].x, points[0].x
		minY, maxY = points[0].y, points[0].y
		for _, p := range points[1:] {
			if p.x < minX {
				minX = p.x
			}
			if p.x > maxX {
				maxX = p.x
			}
			if p.y < minY {
				minY = p.y
			}
			if p.y > maxY {
				maxY = p.y
			}
		}
		if minX == maxX {
			minX -= 0.5
			maxX += 0.5
		}
		if minY == maxY {
			minY -= 0.5
			maxY += 0.5
		}
	}

	for _, p := range points {
		cf := (p.x - minX) / (maxX - minX)
		rf := (p.y - minY) / (maxY - minY)
		c := int(math.Round(cf * float64(cols-1)))
		r := int(math.Round(rf * float64(rows-1)))
		if c < 0 {
			c = 0
		}
		if c >= cols {
			c = cols - 1
		}
		if r < 0 {
			r = 0
		}
		if r >= rows {
			r = rows - 1
		}
		mark := 'o'
		if p.wicket {
			mark = 'X'
		}
		if grid[r][c] != ' ' && grid[r][c] != mark {
			mark = '*'
		}
		grid[r][c] = mark
	}

	fmt.Fprintf(out, "Pitch Map\n")
	fmt.Fprintf(out, "Match: %s\n", strings.TrimSpace(matchQuery))
	if strings.TrimSpace(playerFilter) != "" {
		fmt.Fprintf(out, "Player filter: %s\n", strings.TrimSpace(playerFilter))
	}
	fmt.Fprintf(out, "Balls matched: %d\n", len(filtered))
	fmt.Fprintf(out, "Plotted balls: %d\n", len(points))
	fmt.Fprintf(out, "Unplottable balls: %d\n", unplottable)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Legend: o ball, X wicket, * overlap")
	fmt.Fprintln(out, "      OFF SIDE")
	fmt.Fprintf(out, "   +%s+\n", strings.Repeat("-", cols))
	for _, row := range grid {
		fmt.Fprintf(out, "   |%s|\n", string(row))
	}
	fmt.Fprintf(out, "   +%s+\n", strings.Repeat("-", cols))
	fmt.Fprintln(out, "      LEG SIDE")

	if len(warnings) > 0 {
		fmt.Fprintln(out)
		sort.Strings(warnings)
		for _, warning := range warnings {
			fmt.Fprintf(out, "warning: %s\n", warning)
		}
	}
}

func filterDeliveriesByPlayer(deliveries []cricinfo.DeliveryEvent, playerFilter string) []cricinfo.DeliveryEvent {
	filter := strings.ToLower(strings.TrimSpace(playerFilter))
	if filter == "" {
		return append([]cricinfo.DeliveryEvent(nil), deliveries...)
	}
	filtered := make([]cricinfo.DeliveryEvent, 0, len(deliveries))
	for _, d := range deliveries {
		if deliveryMatchesPlayerFilter(d, filter) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

type sitePitchMapBundle struct {
	PlayerName string
	RHB        [][]int
	LHB        [][]int
}

type siteBattingMapBundle struct {
	PlayerName string
	ZoneRuns   []int
	ZoneShots  []int
	TotalRuns  int
	TotalShots int
}

func fetchSiteAPIPitchMapGrid(ctx context.Context, leagueID, matchID, playerFilter string) (sitePitchMapBundle, error) {
	u := "https://site.api.espn.com/apis/site/v2/sports/cricket/" + url.PathEscape(strings.TrimSpace(leagueID)) + "/summary?event=" + url.QueryEscape(strings.TrimSpace(matchID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return sitePitchMapBundle{}, err
	}
	req.Header.Set("User-Agent", "cricinfo-cli/pitch-map")
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return sitePitchMapBundle{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sitePitchMapBundle{}, fmt.Errorf("site api status %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return sitePitchMapBundle{}, err
	}

	filter := strings.ToLower(strings.TrimSpace(playerFilter))
	rosters := mapSliceValue(payload, "rosters")
	for _, rosterRaw := range rosters {
		roster := mapValue(rosterRaw)
		players := mapSliceValue(roster, "roster")
		for _, playerRaw := range players {
			player := mapValue(playerRaw)
			athlete := mapValue(player["athlete"])
			playerID := strings.TrimSpace(valueString(athlete, "id"))
			displayName := strings.TrimSpace(valueString(athlete, "displayName"))
			fullName := strings.TrimSpace(valueString(athlete, "fullName"))
			if !playerFilterMatches(filter, playerID, displayName, fullName) {
				continue
			}

			linescores := mapSliceValue(player, "linescores")
			for _, lsRaw := range linescores {
				ls := mapValue(lsRaw)
				innings := mapSliceValue(ls, "linescores")
				for _, innRaw := range innings {
					inn := mapValue(innRaw)
					stats := mapValue(inn["statistics"])
					bowling := mapValue(stats["bowling"])
					rhb := decodePitchMapCountGrid(bowling["pitchMapRhb"])
					lhb := decodePitchMapCountGrid(bowling["pitchMapLhb"])
					combined := combinePitchMapGrids(rhb, lhb)
					if countPitchMapGrid(combined) > 0 {
						return sitePitchMapBundle{
							PlayerName: nonEmpty(displayName, fullName, playerID),
							RHB:        rhb,
							LHB:        lhb,
						}, nil
					}
				}
			}
		}
	}

	return sitePitchMapBundle{}, fmt.Errorf("no site-api pitch map for %q", playerFilter)
}

func playerFilterMatches(filter, playerID, displayName, fullName string) bool {
	if filter == "" {
		return true
	}
	candidates := []string{
		strings.ToLower(strings.TrimSpace(playerID)),
		strings.ToLower(strings.TrimSpace(displayName)),
		strings.ToLower(strings.TrimSpace(fullName)),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if c == filter || strings.Contains(c, filter) {
			return true
		}
	}
	if _, err := strconv.Atoi(filter); err == nil {
		return strings.TrimSpace(playerID) == filter
	}
	return false
}

func decodePitchMapCountGrid(raw any) [][]int {
	rows, ok := raw.([]any)
	if !ok || len(rows) == 0 {
		return nil
	}
	grid := make([][]int, 0, len(rows))
	for _, rowRaw := range rows {
		colsRaw, ok := rowRaw.([]any)
		if !ok || len(colsRaw) == 0 {
			continue
		}
		row := make([]int, 0, len(colsRaw))
		for _, cellRaw := range colsRaw {
			triple, ok := cellRaw.([]any)
			if !ok || len(triple) == 0 {
				row = append(row, 0)
				continue
			}
			row = append(row, intValue(triple[0]))
		}
		grid = append(grid, row)
	}
	return grid
}

func combinePitchMapGrids(a, b [][]int) [][]int {
	rows := len(a)
	if len(b) > rows {
		rows = len(b)
	}
	if rows == 0 {
		return nil
	}
	out := make([][]int, rows)
	for r := 0; r < rows; r++ {
		cols := 0
		if r < len(a) && len(a[r]) > cols {
			cols = len(a[r])
		}
		if r < len(b) && len(b[r]) > cols {
			cols = len(b[r])
		}
		if cols == 0 {
			continue
		}
		out[r] = make([]int, cols)
		for c := 0; c < cols; c++ {
			if r < len(a) && c < len(a[r]) {
				out[r][c] += a[r][c]
			}
			if r < len(b) && c < len(b[r]) {
				out[r][c] += b[r][c]
			}
		}
	}
	return out
}

func countPitchMapGrid(grid [][]int) int {
	total := 0
	for _, row := range grid {
		for _, v := range row {
			total += v
		}
	}
	return total
}

func renderSiteAPIPitchMapGrid(out io.Writer, bundle sitePitchMapBundle) {
	combined := combinePitchMapGrids(bundle.RHB, bundle.LHB)
	if len(combined) == 0 {
		return
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Site API Pitch Map Fallback (%s)\n", strings.TrimSpace(bundle.PlayerName))
	fmt.Fprintln(out, "Guide:")
	fmt.Fprintln(out, "  Rows (top->bottom): Short, Back of length, Good, Fuller, Full, Yorker")
	fmt.Fprintln(out, "  Cols (left->right): Wide Off, Off, Channel, Leg, Wide Leg")
	fmt.Fprintln(out, "  Density legend: .(1-2) o(3-5) O(6-9) #(10-14) @(15+)")
	fmt.Fprintln(out)
	renderSingleLabeledPitchGrid(out, "Combined", combined)
	if countPitchMapGrid(bundle.RHB) > 0 {
		fmt.Fprintln(out)
		renderSingleLabeledPitchGrid(out, "vs RHB", bundle.RHB)
	}
	if countPitchMapGrid(bundle.LHB) > 0 {
		fmt.Fprintln(out)
		renderSingleLabeledPitchGrid(out, "vs LHB", bundle.LHB)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Hotspots (top cells):")
	for _, row := range topPitchMapHotspots(combined, 3) {
		fmt.Fprintf(out, "  %s\n", row)
	}
}

func renderSingleLabeledPitchGrid(out io.Writer, title string, grid [][]int) {
	if len(grid) == 0 {
		return
	}
	fmt.Fprintf(out, "%s\n", strings.TrimSpace(title))
	cols := 0
	for _, row := range grid {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return
	}
	colLabels := []string{"WO", "OF", "CH", "LG", "WL"}
	rowLabels := []string{"SHT", "BOL", "GDL", "FUL", "FLR", "YRK"}
	fmt.Fprint(out, "       ")
	for c := 0; c < cols; c++ {
		label := "C" + strconv.Itoa(c+1)
		if c < len(colLabels) {
			label = colLabels[c]
		}
		fmt.Fprintf(out, "%s ", label)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "   +%s+\n", strings.Repeat("--", cols))
	for r, row := range grid {
		label := "R" + strconv.Itoa(r+1)
		if r < len(rowLabels) {
			label = rowLabels[r]
		}
		fmt.Fprintf(out, " %3s|", label)
		for c := 0; c < cols; c++ {
			v := 0
			if c < len(row) {
				v = row[c]
			}
			fmt.Fprintf(out, "%c ", pitchDensityGlyph(v))
		}
		fmt.Fprintln(out, "|")
	}
	fmt.Fprintf(out, "   +%s+\n", strings.Repeat("--", cols))
	fmt.Fprintf(out, "   balls represented: %d\n", countPitchMapGrid(grid))
}

func pitchDensityGlyph(v int) rune {
	switch {
	case v >= 15:
		return '@'
	case v >= 10:
		return '#'
	case v >= 6:
		return 'O'
	case v >= 3:
		return 'o'
	case v >= 1:
		return '.'
	default:
		return ' '
	}
}

func topPitchMapHotspots(grid [][]int, limit int) []string {
	type cell struct {
		r int
		c int
		v int
	}
	cells := make([]cell, 0)
	for r, row := range grid {
		for c, v := range row {
			if v > 0 {
				cells = append(cells, cell{r: r, c: c, v: v})
			}
		}
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].v != cells[j].v {
			return cells[i].v > cells[j].v
		}
		if cells[i].r != cells[j].r {
			return cells[i].r < cells[j].r
		}
		return cells[i].c < cells[j].c
	})
	if len(cells) > limit {
		cells = cells[:limit]
	}
	rowLabels := []string{"Short", "BackLen", "GoodLen", "Full", "Fuller", "Yorker"}
	colLabels := []string{"WideOff", "Off", "Channel", "Leg", "WideLeg"}
	out := make([]string, 0, len(cells))
	for _, c := range cells {
		row := "Row" + strconv.Itoa(c.r+1)
		col := "Col" + strconv.Itoa(c.c+1)
		if c.r < len(rowLabels) {
			row = rowLabels[c.r]
		}
		if c.c < len(colLabels) {
			col = colLabels[c.c]
		}
		out = append(out, fmt.Sprintf("%s / %s -> %d balls", row, col, c.v))
	}
	if len(out) == 0 {
		out = append(out, "No hotspot cells available")
	}
	return out
}

func fetchSiteAPIBattingWagonMap(ctx context.Context, leagueID, matchID, playerFilter string) (siteBattingMapBundle, error) {
	u := "https://site.api.espn.com/apis/site/v2/sports/cricket/" + url.PathEscape(strings.TrimSpace(leagueID)) + "/summary?event=" + url.QueryEscape(strings.TrimSpace(matchID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return siteBattingMapBundle{}, err
	}
	req.Header.Set("User-Agent", "cricinfo-cli/batting-map")
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return siteBattingMapBundle{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return siteBattingMapBundle{}, fmt.Errorf("site api status %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return siteBattingMapBundle{}, err
	}

	filter := strings.ToLower(strings.TrimSpace(playerFilter))
	rosters := mapSliceValue(payload, "rosters")
	for _, rosterRaw := range rosters {
		roster := mapValue(rosterRaw)
		players := mapSliceValue(roster, "roster")
		for _, playerRaw := range players {
			player := mapValue(playerRaw)
			athlete := mapValue(player["athlete"])
			playerID := strings.TrimSpace(valueString(athlete, "id"))
			displayName := strings.TrimSpace(valueString(athlete, "displayName"))
			fullName := strings.TrimSpace(valueString(athlete, "fullName"))
			if !playerFilterMatches(filter, playerID, displayName, fullName) {
				continue
			}

			linescores := mapSliceValue(player, "linescores")
			for _, lsRaw := range linescores {
				ls := mapValue(lsRaw)
				innings := mapSliceValue(ls, "linescores")
				for _, innRaw := range innings {
					inn := mapValue(innRaw)
					stats := mapValue(inn["statistics"])
					batting := mapValue(stats["batting"])
					wagonRaw := mapSliceValue(batting, "wagonZone")
					if len(wagonRaw) == 0 {
						continue
					}
					runs := make([]int, 0, len(wagonRaw))
					shots := make([]int, 0, len(wagonRaw))
					totalRuns := 0
					totalShots := 0
					for _, zoneRaw := range wagonRaw {
						zone := mapValue(zoneRaw)
						r := intValue(zone["runs"])
						s := intValue(zone["scoringShots"])
						runs = append(runs, r)
						shots = append(shots, s)
						totalRuns += r
						totalShots += s
					}
					if totalRuns > 0 || totalShots > 0 {
						return siteBattingMapBundle{
							PlayerName: nonEmpty(displayName, fullName, playerID),
							ZoneRuns:   runs,
							ZoneShots:  shots,
							TotalRuns:  totalRuns,
							TotalShots: totalShots,
						}, nil
					}
				}
			}
		}
	}
	return siteBattingMapBundle{}, fmt.Errorf("no site-api batting wagon map for %q", playerFilter)
}

func renderSiteAPIBattingWagonMap(out io.Writer, bundle siteBattingMapBundle) {
	if len(bundle.ZoneRuns) == 0 {
		return
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Site API Batting Wagon Fallback (%s)\n", strings.TrimSpace(bundle.PlayerName))
	fmt.Fprintln(out, "Guide:")
	fmt.Fprintln(out, "  Zones use provider order Z1..Z8 (direction labels are provider-defined).")
	fmt.Fprintln(out, "  Density legend by runs: .(1-4) o(5-9) O(10-14) #(15-24) @(25+)")
	fmt.Fprintln(out)

	get := func(i int) int {
		if i < 0 || i >= len(bundle.ZoneRuns) {
			return 0
		}
		return bundle.ZoneRuns[i]
	}
	g := func(i int) string {
		v := get(i)
		return fmt.Sprintf("%s%02d", string(battingDensityGlyph(v)), v)
	}

	fmt.Fprintf(out, "              [%s]\n", g(0))
	fmt.Fprintf(out, "        [%s]         [%s]\n", g(7), g(1))
	fmt.Fprintf(out, "        [%s]    []    [%s]\n", g(6), g(2))
	fmt.Fprintf(out, "        [%s]         [%s]\n", g(5), g(3))
	fmt.Fprintf(out, "              [%s]\n", g(4))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Totals: %d runs from %d scoring shots\n", bundle.TotalRuns, bundle.TotalShots)
	fmt.Fprintln(out, "Zone Breakdown:")
	for i := 0; i < len(bundle.ZoneRuns); i++ {
		shots := 0
		if i < len(bundle.ZoneShots) {
			shots = bundle.ZoneShots[i]
		}
		fmt.Fprintf(out, "  Z%d -> %d runs, %d shots\n", i+1, bundle.ZoneRuns[i], shots)
	}
}

func battingDensityGlyph(v int) rune {
	switch {
	case v >= 25:
		return '@'
	case v >= 15:
		return '#'
	case v >= 10:
		return 'O'
	case v >= 5:
		return 'o'
	case v >= 1:
		return '.'
	default:
		return ' '
	}
}

func mapValue(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	if mapped, ok := raw.(map[string]any); ok {
		return mapped
	}
	return nil
}

func mapSliceValue(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	return items
}

func valueString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", raw))
	}
}

func intValue(raw any) int {
	switch v := raw.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return n
		}
	}
	return 0
}

func nonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func deliveryMatchesPlayerFilter(d cricinfo.DeliveryEvent, filter string) bool {
	if filter == "" {
		return true
	}
	candidates := []string{
		d.BatsmanPlayerID,
		d.BowlerPlayerID,
		d.OtherBatsmanID,
		d.OtherBowlerID,
		d.FielderPlayerID,
		d.BatsmanRef,
		d.BowlerRef,
		d.OtherBatsmanRef,
		d.OtherBowlerRef,
		d.ShortText,
		d.Text,
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), filter) {
			return true
		}
	}
	return false
}
