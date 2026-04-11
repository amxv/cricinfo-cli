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
			if kind == cricinfo.EntityMatch && len(items) == 0 && query != "" {
				fallbackItems, fallbackWarnings := fallbackMatchSearchItems(cmd.Context(), query, searchOpts.leagueID, searchOpts.limit)
				if len(fallbackItems) > 0 {
					items = fallbackItems
				}
				searchResult.Warnings = append(searchResult.Warnings, fallbackWarnings...)
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

func fallbackMatchSearchItems(ctx context.Context, query string, leagueID string, limit int) ([]any, []string) {
	service, err := cricinfo.NewMatchService(cricinfo.MatchServiceConfig{})
	if err != nil {
		return nil, []string{fmt.Sprintf("match fallback init failed: %v", err)}
	}
	defer func() {
		_ = service.Close()
	}()

	listLimit := limit
	if listLimit < 20 {
		listLimit = 20
	}
	if listLimit > 80 {
		listLimit = 80
	}

	result, err := service.Live(ctx, cricinfo.MatchListOptions{Limit: listLimit, LeagueID: strings.TrimSpace(leagueID)})
	if err != nil {
		return nil, []string{fmt.Sprintf("match fallback live list failed: %v", err)}
	}
	if len(result.Items) == 0 {
		result, err = service.List(ctx, cricinfo.MatchListOptions{Limit: listLimit, LeagueID: strings.TrimSpace(leagueID)})
		if err != nil {
			return nil, []string{fmt.Sprintf("match fallback list failed: %v", err)}
		}
	}

	queryNorm := normalizeMatchSearchText(query)
	queryTokens := strings.Fields(queryNorm)
	if queryNorm == "" {
		return nil, nil
	}

	items := make([]any, 0, limitOrDefault(limit, 10))
	for _, raw := range result.Items {
		match, ok := raw.(cricinfo.Match)
		if !ok {
			continue
		}
		if matchSearchScore(match, queryNorm, queryTokens) == 0 {
			continue
		}
		items = append(items, match)
		if len(items) >= limitOrDefault(limit, 10) {
			break
		}
	}

	if len(items) == 0 {
		return nil, nil
	}
	return items, []string{"match search fallback used list/live aliases"}
}

func matchSearchScore(match cricinfo.Match, query string, queryTokens []string) int {
	score := 0
	for _, value := range []string{
		match.ID,
		match.Description,
		match.ShortDescription,
		match.Note,
		match.ScoreSummary,
	} {
		score = maxInt(score, aliasScore(normalizeMatchSearchText(value), query, queryTokens))
	}
	for _, team := range match.Teams {
		for _, value := range []string{
			team.ID,
			team.Name,
			team.ShortName,
			team.Abbreviation,
		} {
			score = maxInt(score, aliasScore(normalizeMatchSearchText(value), query, queryTokens))
		}
	}
	return score
}

func aliasScore(alias, query string, queryTokens []string) int {
	if alias == "" || query == "" {
		return 0
	}
	if alias == query {
		return 1000
	}
	if strings.HasPrefix(alias, query) {
		return 800
	}
	if strings.Contains(alias, query) {
		return 650
	}
	aliasTokens := strings.Fields(alias)
	if len(aliasTokens) == 0 || len(queryTokens) == 0 {
		return 0
	}
	matched := 0
	for _, queryToken := range queryTokens {
		for _, aliasToken := range aliasTokens {
			if aliasToken == queryToken || strings.HasPrefix(aliasToken, queryToken) || (len(aliasToken) >= 2 && strings.HasPrefix(queryToken, aliasToken)) {
				matched++
				break
			}
		}
	}
	if matched == 0 {
		return 0
	}
	return 300 + (matched * 60)
}

func normalizeMatchSearchText(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	var builder strings.Builder
	lastSpace := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func limitOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	if fallback <= 0 {
		return 10
	}
	return fallback
}
