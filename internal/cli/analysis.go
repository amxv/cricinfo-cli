package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type analysisCommandService interface {
	Close() error
	Dismissals(ctx context.Context, opts cricinfo.AnalysisDismissalOptions) (cricinfo.NormalizedResult, error)
	Bowling(ctx context.Context, opts cricinfo.AnalysisMetricOptions) (cricinfo.NormalizedResult, error)
	Batting(ctx context.Context, opts cricinfo.AnalysisMetricOptions) (cricinfo.NormalizedResult, error)
	Partnerships(ctx context.Context, opts cricinfo.AnalysisMetricOptions) (cricinfo.NormalizedResult, error)
}

type analysisRuntimeOptions struct {
	leagueID   string
	typeQuery  string
	groupQuery string
	dateFrom   string
	dateTo     string
	matchLimit int
}

var newAnalysisService = func() (analysisCommandService, error) {
	return cricinfo.NewAnalysisService(cricinfo.AnalysisServiceConfig{})
}

func newAnalysisCommand(global *globalOptions) *cobra.Command {
	base := &analysisRuntimeOptions{}

	cmd := &cobra.Command{
		Use:   "analysis",
		Short: "Derived cricket analysis over normalized real-time hydrated data.",
		Long: strings.Join([]string{
			"Run ranking and grouping analysis over live scoped Cricinfo data.",
			"Analysis reuses in-process hydration for the active command execution only.",
			"No persistent analysis cache is written.",
			"",
			"Next steps:",
			"  cricinfo analysis dismissals --league 19138 --seasons 2024-2025",
			"  cricinfo analysis bowling --metric economy --scope season:2025 --league 19138",
			"  cricinfo analysis batting --metric strike-rate --scope match:1529474",
			"  cricinfo analysis partnerships --scope season:2025 --league 19138",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVar(&base.leagueID, "league", "", "League ID/ref/alias (required for season scope and dismissals)")
	cmd.PersistentFlags().StringVar(&base.typeQuery, "type", "", "Optional season type scope for season traversal")
	cmd.PersistentFlags().StringVar(&base.groupQuery, "group", "", "Optional season group scope for season traversal")
	cmd.PersistentFlags().StringVar(&base.dateFrom, "date-from", "", "Optional lower date bound (RFC3339 or YYYY-MM-DD)")
	cmd.PersistentFlags().StringVar(&base.dateTo, "date-to", "", "Optional upper date bound (RFC3339 or YYYY-MM-DD)")
	cmd.PersistentFlags().IntVar(&base.matchLimit, "match-limit", 0, "Optional cap on scoped matches before analysis")

	type dismissRuntime struct {
		seasons       string
		groupBy       string
		team          string
		player        string
		dismissalType string
		innings       int
		period        int
		top           int
	}
	dismiss := &dismissRuntime{}
	dismissalsCmd := &cobra.Command{
		Use:   "dismissals",
		Short: "Rank dismissal patterns across league seasons",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(base.leagueID) == "" {
				return fmt.Errorf("--league is required")
			}
			if strings.TrimSpace(dismiss.seasons) == "" {
				return fmt.Errorf("--seasons is required")
			}
			return runAnalysisCommand(cmd, global, func(ctx context.Context, service analysisCommandService) (cricinfo.NormalizedResult, error) {
				return service.Dismissals(ctx, cricinfo.AnalysisDismissalOptions{
					LeagueQuery:   base.leagueID,
					Seasons:       dismiss.seasons,
					TypeQuery:     base.typeQuery,
					GroupQuery:    base.groupQuery,
					DateFrom:      base.dateFrom,
					DateTo:        base.dateTo,
					MatchLimit:    base.matchLimit,
					GroupBy:       dismiss.groupBy,
					TeamQuery:     dismiss.team,
					PlayerQuery:   dismiss.player,
					DismissalType: dismiss.dismissalType,
					Innings:       dismiss.innings,
					Period:        dismiss.period,
					Top:           dismiss.top,
				})
			})
		},
	}
	dismissalsCmd.Flags().StringVar(&dismiss.seasons, "seasons", "", "Required: season range (for example 2023-2025 or 2024,2025)")
	dismissalsCmd.Flags().StringVar(&dismiss.groupBy, "group-by", "dismissal-type", "Grouping fields (comma-separated): player,team,league,season,dismissal-type,innings")
	dismissalsCmd.Flags().StringVar(&dismiss.team, "team", "", "Optional team filter (id or alias)")
	dismissalsCmd.Flags().StringVar(&dismiss.player, "player", "", "Optional player filter (id or alias)")
	dismissalsCmd.Flags().StringVar(&dismiss.dismissalType, "dismissal-type", "", "Optional dismissal type filter (for example caught, bowled)")
	dismissalsCmd.Flags().IntVar(&dismiss.innings, "innings", 0, "Optional innings filter")
	dismissalsCmd.Flags().IntVar(&dismiss.period, "period", 0, "Optional period filter")
	dismissalsCmd.Flags().IntVar(&dismiss.top, "top", 20, "Maximum ranked rows to return")

	type metricRuntime struct {
		scope   string
		metric  string
		groupBy string
		team    string
		player  string
		top     int
	}
	bowling := &metricRuntime{}
	bowlingCmd := &cobra.Command{
		Use:   "bowling",
		Short: "Rank bowling metrics by match or season scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(bowling.scope) == "" {
				return fmt.Errorf("--scope is required")
			}
			if strings.TrimSpace(bowling.metric) == "" {
				return fmt.Errorf("--metric is required")
			}
			return runAnalysisCommand(cmd, global, func(ctx context.Context, service analysisCommandService) (cricinfo.NormalizedResult, error) {
				return service.Bowling(ctx, cricinfo.AnalysisMetricOptions{
					Metric:      bowling.metric,
					Scope:       bowling.scope,
					LeagueQuery: base.leagueID,
					TypeQuery:   base.typeQuery,
					GroupQuery:  base.groupQuery,
					DateFrom:    base.dateFrom,
					DateTo:      base.dateTo,
					MatchLimit:  base.matchLimit,
					GroupBy:     bowling.groupBy,
					TeamQuery:   bowling.team,
					PlayerQuery: bowling.player,
					Top:         bowling.top,
				})
			})
		},
	}
	bowlingCmd.Flags().StringVar(&bowling.scope, "scope", "", "Required: match:<match> or season:<season>")
	bowlingCmd.Flags().StringVar(&bowling.metric, "metric", "", "Required: economy, dots, or sixes-conceded")
	bowlingCmd.Flags().StringVar(&bowling.groupBy, "group-by", "player", "Grouping fields (comma-separated): player,team,league,season")
	bowlingCmd.Flags().StringVar(&bowling.team, "team", "", "Optional team filter")
	bowlingCmd.Flags().StringVar(&bowling.player, "player", "", "Optional player filter")
	bowlingCmd.Flags().IntVar(&bowling.top, "top", 20, "Maximum ranked rows to return")

	batting := &metricRuntime{}
	battingCmd := &cobra.Command{
		Use:   "batting",
		Short: "Rank batting metrics by match or season scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(batting.scope) == "" {
				return fmt.Errorf("--scope is required")
			}
			if strings.TrimSpace(batting.metric) == "" {
				return fmt.Errorf("--metric is required")
			}
			return runAnalysisCommand(cmd, global, func(ctx context.Context, service analysisCommandService) (cricinfo.NormalizedResult, error) {
				return service.Batting(ctx, cricinfo.AnalysisMetricOptions{
					Metric:      batting.metric,
					Scope:       batting.scope,
					LeagueQuery: base.leagueID,
					TypeQuery:   base.typeQuery,
					GroupQuery:  base.groupQuery,
					DateFrom:    base.dateFrom,
					DateTo:      base.dateTo,
					MatchLimit:  base.matchLimit,
					GroupBy:     batting.groupBy,
					TeamQuery:   batting.team,
					PlayerQuery: batting.player,
					Top:         batting.top,
				})
			})
		},
	}
	battingCmd.Flags().StringVar(&batting.scope, "scope", "", "Required: match:<match> or season:<season>")
	battingCmd.Flags().StringVar(&batting.metric, "metric", "", "Required: fours, sixes, or strike-rate")
	battingCmd.Flags().StringVar(&batting.groupBy, "group-by", "player", "Grouping fields (comma-separated): player,team,league,season")
	battingCmd.Flags().StringVar(&batting.team, "team", "", "Optional team filter")
	battingCmd.Flags().StringVar(&batting.player, "player", "", "Optional player filter")
	battingCmd.Flags().IntVar(&batting.top, "top", 20, "Maximum ranked rows to return")

	type partnershipRuntime struct {
		scope   string
		groupBy string
		team    string
		innings int
		top     int
	}
	partners := &partnershipRuntime{}
	partnershipsCmd := &cobra.Command{
		Use:   "partnerships",
		Short: "Rank partnerships by match or season scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(partners.scope) == "" {
				return fmt.Errorf("--scope is required")
			}
			return runAnalysisCommand(cmd, global, func(ctx context.Context, service analysisCommandService) (cricinfo.NormalizedResult, error) {
				return service.Partnerships(ctx, cricinfo.AnalysisMetricOptions{
					Scope:       partners.scope,
					LeagueQuery: base.leagueID,
					TypeQuery:   base.typeQuery,
					GroupQuery:  base.groupQuery,
					DateFrom:    base.dateFrom,
					DateTo:      base.dateTo,
					MatchLimit:  base.matchLimit,
					GroupBy:     partners.groupBy,
					TeamQuery:   partners.team,
					Innings:     partners.innings,
					Top:         partners.top,
				})
			})
		},
	}
	partnershipsCmd.Flags().StringVar(&partners.scope, "scope", "", "Required: match:<match> or season:<season>")
	partnershipsCmd.Flags().StringVar(&partners.groupBy, "group-by", "innings", "Grouping fields (comma-separated): innings,team,league,season")
	partnershipsCmd.Flags().StringVar(&partners.team, "team", "", "Optional team filter")
	partnershipsCmd.Flags().IntVar(&partners.innings, "innings", 0, "Optional innings filter")
	partnershipsCmd.Flags().IntVar(&partners.top, "top", 20, "Maximum ranked rows to return")

	cmd.AddCommand(dismissalsCmd, bowlingCmd, battingCmd, partnershipsCmd)
	return cmd
}

func runAnalysisCommand(
	cmd *cobra.Command,
	global *globalOptions,
	fn func(ctx context.Context, service analysisCommandService) (cricinfo.NormalizedResult, error),
) error {
	service, err := newAnalysisService()
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
