package cli

import (
	"context"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type competitionCommandService interface {
	Close() error
	Show(ctx context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error)
	Officials(ctx context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error)
	Broadcasts(ctx context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error)
	Tickets(ctx context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error)
	Odds(ctx context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error)
	Metadata(ctx context.Context, query string, opts cricinfo.CompetitionLookupOptions) (cricinfo.NormalizedResult, error)
}

type competitionRuntimeOptions struct {
	leagueID string
}

var newCompetitionService = func() (competitionCommandService, error) {
	return cricinfo.NewCompetitionService(cricinfo.CompetitionServiceConfig{})
}

func newCompetitionsCommand(global *globalOptions) *cobra.Command {
	opts := &competitionRuntimeOptions{}

	cmd := &cobra.Command{
		Use:   "competitions",
		Short: "Competition metadata including officials, broadcasts, tickets, odds, and aggregate views.",
		Long: strings.Join([]string{
			"Resolve a match/competition and drill into auxiliary competition metadata routes.",
			"Empty-but-valid metadata collections are rendered as clean zero-result views.",
			"",
			"Next steps:",
			"  cricinfo competitions show <match>",
			"  cricinfo competitions officials <match>",
			"  cricinfo competitions broadcasts <match>",
			"  cricinfo competitions tickets <match>",
			"  cricinfo competitions odds <match>",
			"  cricinfo competitions metadata <match>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVar(&opts.leagueID, "league", "", "Preferred league ID for match resolution context")

	showCmd := &cobra.Command{
		Use:   "show <match>",
		Short: "Show one competition summary",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runCompetitionCommand(cmd, global, func(ctx context.Context, service competitionCommandService) (cricinfo.NormalizedResult, error) {
				return service.Show(ctx, query, cricinfo.CompetitionLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	officialsCmd := &cobra.Command{
		Use:   "officials <match>",
		Short: "Show competition officials entries",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runCompetitionCommand(cmd, global, func(ctx context.Context, service competitionCommandService) (cricinfo.NormalizedResult, error) {
				return service.Officials(ctx, query, cricinfo.CompetitionLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	broadcastsCmd := &cobra.Command{
		Use:   "broadcasts <match>",
		Short: "Show competition broadcast entries",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runCompetitionCommand(cmd, global, func(ctx context.Context, service competitionCommandService) (cricinfo.NormalizedResult, error) {
				return service.Broadcasts(ctx, query, cricinfo.CompetitionLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	ticketsCmd := &cobra.Command{
		Use:   "tickets <match>",
		Short: "Show competition ticket entries",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runCompetitionCommand(cmd, global, func(ctx context.Context, service competitionCommandService) (cricinfo.NormalizedResult, error) {
				return service.Tickets(ctx, query, cricinfo.CompetitionLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	oddsCmd := &cobra.Command{
		Use:   "odds <match>",
		Short: "Show competition odds entries",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runCompetitionCommand(cmd, global, func(ctx context.Context, service competitionCommandService) (cricinfo.NormalizedResult, error) {
				return service.Odds(ctx, query, cricinfo.CompetitionLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	metadataCmd := &cobra.Command{
		Use:   "metadata <match>",
		Short: "Show aggregated competition metadata across officials, broadcasts, tickets, and odds",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			return runCompetitionCommand(cmd, global, func(ctx context.Context, service competitionCommandService) (cricinfo.NormalizedResult, error) {
				return service.Metadata(ctx, query, cricinfo.CompetitionLookupOptions{LeagueID: opts.leagueID})
			})
		},
	}

	cmd.AddCommand(showCmd, officialsCmd, broadcastsCmd, ticketsCmd, oddsCmd, metadataCmd)
	return cmd
}

func runCompetitionCommand(
	cmd *cobra.Command,
	global *globalOptions,
	fn func(ctx context.Context, service competitionCommandService) (cricinfo.NormalizedResult, error),
) error {
	service, err := newCompetitionService()
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
