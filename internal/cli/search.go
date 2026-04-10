package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type searchRuntimeOptions struct {
	limit     int
	leagueID  string
	matchID   string
	leagueRef string
	seasonRef string
}

var newSearchResolver = func() (*cricinfo.Resolver, error) {
	return cricinfo.NewResolver(cricinfo.ResolverConfig{})
}

func newSearchCommand(global *globalOptions) *cobra.Command {
	opts := &searchRuntimeOptions{}

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Cross-entity discovery for matches, players, teams, and leagues.",
		Long: strings.Join([]string{
			"Search domain entities using numeric IDs, refs, or cached aliases.",
			"The resolver is seeded incrementally from current events, league traversal, and roster data.",
			"",
			"Next steps:",
			"  cricinfo search players <query>",
			"  cricinfo search teams <query>",
			"  cricinfo search leagues <query>",
			"  cricinfo search matches <query>",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().IntVar(&opts.limit, "limit", 10, "Maximum number of results to return")
	cmd.PersistentFlags().StringVar(&opts.leagueID, "league", "", "Preferred league ID for context-aware resolution")
	cmd.PersistentFlags().StringVar(&opts.matchID, "match", "", "Preferred match/competition ID for context-aware resolution")
	cmd.PersistentFlags().StringVar(&opts.leagueRef, "league-ref", "", "Known league ref to seed search context")
	cmd.PersistentFlags().StringVar(&opts.seasonRef, "season-ref", "", "Known season ref to seed league traversal context")

	cmd.AddCommand(newSearchEntityCommand("players", cricinfo.EntityPlayer, global, opts))
	cmd.AddCommand(newSearchEntityCommand("teams", cricinfo.EntityTeam, global, opts))
	cmd.AddCommand(newSearchEntityCommand("leagues", cricinfo.EntityLeague, global, opts))
	cmd.AddCommand(newSearchEntityCommand("matches", cricinfo.EntityMatch, global, opts))

	return cmd
}

func newSearchEntityCommand(name string, kind cricinfo.EntityKind, global *globalOptions, searchOpts *searchRuntimeOptions) *cobra.Command {
	usage := fmt.Sprintf("%s [query]", name)
	command := &cobra.Command{
		Use:   usage,
		Short: fmt.Sprintf("Search %s by id, ref, or alias", name),
		Long: strings.Join([]string{
			fmt.Sprintf("Search %s by numeric ID, known ref, or cached alias.", name),
			"",
			"Examples:",
			fmt.Sprintf("  cricinfo search %s 1361257", name),
			fmt.Sprintf("  cricinfo search %s \"fazal haq\"", name),
		}, "\n"),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolver, err := newSearchResolver()
			if err != nil {
				return err
			}
			defer func() {
				_ = resolver.Close()
			}()

			query := strings.TrimSpace(strings.Join(args, " "))
			searchResult, err := resolver.Search(context.Background(), kind, query, cricinfo.ResolveOptions{
				Limit:     searchOpts.limit,
				LeagueID:  searchOpts.leagueID,
				MatchID:   searchOpts.matchID,
				LeagueRef: searchOpts.leagueRef,
				SeasonRef: searchOpts.seasonRef,
			})
			if err != nil {
				return err
			}

			items := make([]any, 0, len(searchResult.Entities))
			for _, entity := range searchResult.Entities {
				items = append(items, entity.ToRenderable())
			}

			result := cricinfo.NewListResult(kind, items)
			if len(searchResult.Warnings) > 0 {
				if result.Status == cricinfo.ResultStatusEmpty {
					result.Status = cricinfo.ResultStatusPartial
					result.Message = "partial data returned"
				} else {
					result = cricinfo.NewPartialListResult(kind, items, searchResult.Warnings...)
				}
				result.Warnings = searchResult.Warnings
			}

			return cricinfo.Render(cmd.OutOrStdout(), result, cricinfo.RenderOptions{
				Format:    global.format,
				Verbose:   global.verbose,
				AllFields: global.allFields,
			})
		},
	}

	return command
}
