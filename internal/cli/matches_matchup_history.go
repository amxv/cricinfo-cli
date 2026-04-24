package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type matchupHistoryScope struct {
	Mode  string
	Query string
}

type matchupHistoryAggregate struct {
	Matches []cricinfo.MatchDuel
	Runs    int
	Balls   int
	Dots    int
	Fours   int
	Sixes   int
	Wickets int
}

func runMatchupHistoryCommand(cmd *cobra.Command, opts *matchRuntimeOptions) error {
	scope, err := parseMatchupHistoryScope(opts.scope)
	if err != nil {
		return err
	}

	matchIDs, leagueID, warnings, err := resolveMatchupHistoryScope(cmd, scope, opts)
	if err != nil {
		return err
	}

	service, err := newMatchService()
	if err != nil {
		return err
	}
	defer func() {
		_ = service.Close()
	}()

	agg := matchupHistoryAggregate{
		Matches: make([]cricinfo.MatchDuel, 0),
	}

	for _, matchID := range matchIDs {
		result, derr := service.Duel(cmd.Context(), matchID, cricinfo.MatchDuelOptions{
			LeagueID:    leagueID,
			BatterQuery: strings.TrimSpace(opts.batter),
			BowlerQuery: strings.TrimSpace(opts.bowler),
		})
		if derr != nil {
			warnings = append(warnings, fmt.Sprintf("match %s duel: %v", matchID, derr))
			continue
		}
		warnings = append(warnings, result.Warnings...)

		var duel cricinfo.MatchDuel
		ok := false
		if value, yes := result.Data.(cricinfo.MatchDuel); yes {
			duel = value
			ok = true
		} else if value, yes := result.Data.(*cricinfo.MatchDuel); yes && value != nil {
			duel = *value
			ok = true
		}
		if !ok || duel.Balls == 0 {
			if strings.TrimSpace(result.Message) != "" && strings.Contains(strings.ToLower(result.Message), "no deliveries found") {
				continue
			}
			continue
		}
		agg.Matches = append(agg.Matches, duel)
		agg.Runs += duel.Runs
		agg.Balls += duel.Balls
		agg.Dots += duel.Dots
		agg.Fours += duel.Fours
		agg.Sixes += duel.Sixes
		agg.Wickets += duel.Wickets
	}

	sort.Slice(agg.Matches, func(i, j int) bool {
		return agg.Matches[i].LastUpdateMS > agg.Matches[j].LastUpdateMS
	})

	renderMatchupHistory(cmd.OutOrStdout(), opts, scope, leagueID, len(matchIDs), agg)
	if compact := uniqueStrings(warnings); len(compact) > 0 {
		fmt.Fprintln(cmd.OutOrStdout())
		for _, warning := range compact {
			fmt.Fprintf(cmd.OutOrStdout(), "warning: %s\n", warning)
		}
	}
	return nil
}

func resolveMatchupHistoryScope(cmd *cobra.Command, scope matchupHistoryScope, opts *matchRuntimeOptions) ([]string, string, []string, error) {
	leagueID := strings.TrimSpace(opts.leagueID)
	warnings := make([]string, 0)
	matchIDs := make([]string, 0)

	switch scope.Mode {
	case "match":
		matchIDs = append(matchIDs, scope.Query)
		if leagueID == "" {
			derived, derr := resolveLeagueIDForMatch(cmd.Context(), scope.Query)
			if derr == nil && strings.TrimSpace(derived) != "" {
				leagueID = strings.TrimSpace(derived)
			} else if derr != nil {
				warnings = append(warnings, "league auto-detect failed: "+derr.Error())
			}
		}
	case "season":
		if leagueID == "" {
			return nil, "", nil, fmt.Errorf("--league is required for season scope")
		}
		hydration, err := cricinfo.NewHistoricalHydrationService(cricinfo.HistoricalHydrationServiceConfig{})
		if err != nil {
			return nil, "", nil, err
		}
		defer func() {
			_ = hydration.Close()
		}()

		session, err := hydration.BeginScope(cmd.Context(), cricinfo.HistoricalScopeOptions{
			LeagueQuery: leagueID,
			SeasonQuery: scope.Query,
			MatchLimit:  opts.limit,
		})
		if err != nil {
			return nil, "", nil, err
		}
		summary := session.Scope()
		if strings.TrimSpace(summary.League.ID) != "" {
			leagueID = strings.TrimSpace(summary.League.ID)
		}
		matchIDs = append(matchIDs, summary.MatchIDs...)
		warnings = append(warnings, summary.Warnings...)
	default:
		return nil, "", nil, fmt.Errorf("unsupported scope mode %q", scope.Mode)
	}

	matchIDs = uniqueStrings(matchIDs)
	return matchIDs, leagueID, warnings, nil
}

func parseMatchupHistoryScope(raw string) (matchupHistoryScope, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return matchupHistoryScope{}, fmt.Errorf("--scope is required (match:<match> or season:<season>)")
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return matchupHistoryScope{}, fmt.Errorf("--scope must be match:<match> or season:<season>")
	}
	mode := strings.ToLower(strings.TrimSpace(parts[0]))
	query := strings.TrimSpace(parts[1])
	if query == "" {
		return matchupHistoryScope{}, fmt.Errorf("scope query is required")
	}
	switch mode {
	case "match", "season":
		return matchupHistoryScope{Mode: mode, Query: query}, nil
	default:
		return matchupHistoryScope{}, fmt.Errorf("unsupported --scope mode %q (expected match or season)", mode)
	}
}

func renderMatchupHistory(out io.Writer, opts *matchRuntimeOptions, scope matchupHistoryScope, leagueID string, scopedMatches int, agg matchupHistoryAggregate) {
	fmt.Fprintln(out, "Matchup History")
	fmt.Fprintf(out, "Pair: %s (bat) vs %s (bowl)\n", strings.TrimSpace(opts.batter), strings.TrimSpace(opts.bowler))
	fmt.Fprintf(out, "Scope: %s:%s\n", scope.Mode, scope.Query)
	fmt.Fprintf(out, "League: %s\n", nonEmpty(strings.TrimSpace(leagueID), "(unknown)"))
	fmt.Fprintf(out, "Matches in scope: %d\n", scopedMatches)
	fmt.Fprintf(out, "Matches with matchup: %d", len(agg.Matches))
	if scopedMatches > 0 {
		pct := float64(len(agg.Matches)) * 100 / float64(scopedMatches)
		fmt.Fprintf(out, " (%.1f%%)", pct)
	}
	fmt.Fprintln(out)

	if agg.Balls == 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "No head-to-head deliveries found for this scope.")
		return
	}

	strikeRate := (float64(agg.Runs) * 100) / float64(agg.Balls)
	dotPct := (float64(agg.Dots) * 100) / float64(agg.Balls)
	boundaryRuns := (agg.Fours * 4) + (agg.Sixes * 6)
	boundaryPct := 0.0
	if agg.Runs > 0 {
		boundaryPct = float64(boundaryRuns) * 100 / float64(agg.Runs)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Aggregate")
	fmt.Fprintf(out, "  Runs/Balls: %d/%d\n", agg.Runs, agg.Balls)
	fmt.Fprintf(out, "  Strike Rate: %.2f\n", strikeRate)
	fmt.Fprintf(out, "  Dots: %d (%.1f%%)\n", agg.Dots, dotPct)
	fmt.Fprintf(out, "  Boundaries: 4s %d | 6s %d | boundary runs %d (%.1f%%)\n", agg.Fours, agg.Sixes, boundaryRuns, boundaryPct)
	fmt.Fprintf(out, "  Wickets: %d\n", agg.Wickets)

	top := opts.top
	if top <= 0 {
		top = 8
	}
	if len(agg.Matches) < top {
		top = len(agg.Matches)
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Match Cards (top %d)\n", top)
	for i := 0; i < top; i++ {
		m := agg.Matches[i]
		fmt.Fprintf(out, "  %d) %s | %d off %d | SR %.2f | dots %d | 4s %d | 6s %d | wkts %d\n", i+1, nonEmpty(strings.TrimSpace(m.MatchID), "?"), m.Runs, m.Balls, m.StrikeRate, m.Dots, m.Fours, m.Sixes, m.Wickets)
		if strings.TrimSpace(m.Fixture) != "" {
			fmt.Fprintf(out, "     Fixture: %s\n", strings.TrimSpace(m.Fixture))
		}
		if strings.TrimSpace(m.Score) != "" {
			fmt.Fprintf(out, "     Score: %s\n", strings.TrimSpace(m.Score))
		}
		if seq := matchupSequence(m.RecentBalls); seq != "" {
			fmt.Fprintf(out, "     Sequence: %s\n", seq)
		}
	}
}

func matchupSequence(balls []cricinfo.DeliveryEvent) string {
	if len(balls) == 0 {
		return ""
	}
	tokens := make([]string, 0, len(balls))
	for _, ball := range balls {
		if dismissal, ok := ball.Dismissal["dismissal"].(bool); ok && dismissal {
			tokens = append(tokens, "W")
			continue
		}
		short := strings.ToUpper(strings.TrimSpace(ball.ShortText))
		switch {
		case strings.Contains(short, "SIX"):
			tokens = append(tokens, "6")
		case strings.Contains(short, "FOUR"):
			tokens = append(tokens, "4")
		case strings.Contains(short, "WIDE"):
			tokens = append(tokens, "Wd")
		case strings.Contains(short, "NO BALL"), strings.Contains(short, "NOBALL"):
			tokens = append(tokens, "Nb")
		default:
			if ball.ScoreValue <= 0 {
				tokens = append(tokens, ".")
			} else {
				tokens = append(tokens, strconv.Itoa(ball.ScoreValue))
			}
		}
	}
	return strings.Join(tokens, " ")
}
