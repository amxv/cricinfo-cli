package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/amxv/cricinfo-cli/internal/cricinfo"
	"github.com/spf13/cobra"
)

type playerHistoryScope struct {
	Mode  string
	Query string
}

type playerHistoryMode struct {
	Label   string
	Batting bool
	Bowling bool
}

func runPlayerMapHistoryCommand(cmd *cobra.Command, opts *playerRuntimeOptions, playerQuery string) error {
	scope, err := parsePlayerHistoryScope(opts.scope)
	if err != nil {
		return err
	}
	mode, err := normalizePlayerHistoryMode(opts.mapMode)
	if err != nil {
		return err
	}

	leagueID := strings.TrimSpace(opts.leagueID)
	matchIDs := make([]string, 0)
	warnings := make([]string, 0)

	switch scope.Mode {
	case "match":
		matchIDs = append(matchIDs, scope.Query)
		if leagueID == "" {
			derivedLeagueID, derr := resolveLeagueIDForMatch(cmd.Context(), scope.Query)
			if derr != nil {
				warnings = append(warnings, "league auto-detect failed: "+derr.Error())
			}
			leagueID = strings.TrimSpace(nonEmpty(derivedLeagueID, leagueID))
		}
		if leagueID == "" {
			return fmt.Errorf("--league is required for map history when match league cannot be auto-resolved")
		}
	case "season":
		if leagueID == "" {
			return fmt.Errorf("--league is required for season scope")
		}
		hydration, herr := cricinfo.NewHistoricalHydrationService(cricinfo.HistoricalHydrationServiceConfig{})
		if herr != nil {
			return herr
		}
		defer func() {
			_ = hydration.Close()
		}()

		session, serr := hydration.BeginScope(cmd.Context(), cricinfo.HistoricalScopeOptions{
			LeagueQuery: leagueID,
			SeasonQuery: scope.Query,
			MatchLimit:  opts.mapLimit,
		})
		if serr != nil {
			return serr
		}
		summary := session.Scope()
		if strings.TrimSpace(summary.League.ID) != "" {
			leagueID = strings.TrimSpace(summary.League.ID)
		}
		matchIDs = append(matchIDs, summary.MatchIDs...)
		warnings = append(warnings, summary.Warnings...)
	default:
		return fmt.Errorf("unsupported scope mode %q", scope.Mode)
	}

	matchIDs = uniqueStrings(matchIDs)
	renderHistoricalMapHeader(cmd.OutOrStdout(), playerQuery, scope, mode, leagueID, len(matchIDs))
	if len(matchIDs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No matches found for selected scope.")
		renderHistoryWarnings(cmd.OutOrStdout(), warnings)
		return nil
	}

	battingBundle := siteBattingMapBundle{PlayerName: strings.TrimSpace(playerQuery)}
	bowlingBundle := sitePitchMapBundle{PlayerName: strings.TrimSpace(playerQuery)}
	battingMatchesUsed := 0
	bowlingMatchesUsed := 0

	for _, matchID := range matchIDs {
		if mode.Batting {
			if bundle, berr := fetchSiteAPIBattingWagonMap(cmd.Context(), leagueID, matchID, playerQuery); berr == nil && len(bundle.ZoneRuns) > 0 {
				battingBundle.PlayerName = nonEmpty(strings.TrimSpace(bundle.PlayerName), battingBundle.PlayerName)
				battingBundle.ZoneRuns = sumIntSlices(battingBundle.ZoneRuns, bundle.ZoneRuns)
				battingBundle.ZoneShots = sumIntSlices(battingBundle.ZoneShots, bundle.ZoneShots)
				battingBundle.TotalRuns += bundle.TotalRuns
				battingBundle.TotalShots += bundle.TotalShots
				battingMatchesUsed++
			}
		}
		if mode.Bowling {
			if bundle, berr := fetchSiteAPIPitchMapGrid(cmd.Context(), leagueID, matchID, playerQuery); berr == nil && (countPitchMapGrid(bundle.RHB) > 0 || countPitchMapGrid(bundle.LHB) > 0) {
				bowlingBundle.PlayerName = nonEmpty(strings.TrimSpace(bundle.PlayerName), bowlingBundle.PlayerName)
				bowlingBundle.RHB = combinePitchMapGrids(bowlingBundle.RHB, bundle.RHB)
				bowlingBundle.LHB = combinePitchMapGrids(bowlingBundle.LHB, bundle.LHB)
				bowlingMatchesUsed++
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Matches used for batting map: %d\n", battingMatchesUsed)
	fmt.Fprintf(cmd.OutOrStdout(), "Matches used for bowling map: %d\n", bowlingMatchesUsed)

	if mode.Batting {
		if battingMatchesUsed > 0 && len(battingBundle.ZoneRuns) > 0 {
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "Historical Batting Distribution (aggregated over %d matches)\n", battingMatchesUsed)
			renderSiteAPIBattingWagonMap(cmd.OutOrStdout(), battingBundle)
		} else {
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Historical Batting Distribution")
			fmt.Fprintln(cmd.OutOrStdout(), "No batting wagon data found in selected scope.")
		}
	}

	if mode.Bowling {
		if bowlingMatchesUsed > 0 && (countPitchMapGrid(bowlingBundle.RHB) > 0 || countPitchMapGrid(bowlingBundle.LHB) > 0) {
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "Historical Bowling Pitch Map (aggregated over %d matches)\n", bowlingMatchesUsed)
			renderSiteAPIPitchMapGrid(cmd.OutOrStdout(), bowlingBundle)
		} else {
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Historical Bowling Pitch Map")
			fmt.Fprintln(cmd.OutOrStdout(), "No bowling pitch-map data found in selected scope.")
		}
	}

	renderHistoryWarnings(cmd.OutOrStdout(), warnings)
	return nil
}

func parsePlayerHistoryScope(raw string) (playerHistoryScope, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return playerHistoryScope{}, fmt.Errorf("--scope is required (match:<match> or season:<season>)")
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return playerHistoryScope{}, fmt.Errorf("--scope must be match:<match> or season:<season>")
	}
	mode := strings.ToLower(strings.TrimSpace(parts[0]))
	query := strings.TrimSpace(parts[1])
	if query == "" {
		return playerHistoryScope{}, fmt.Errorf("scope query is required")
	}
	switch mode {
	case "match", "season":
		return playerHistoryScope{Mode: mode, Query: query}, nil
	default:
		return playerHistoryScope{}, fmt.Errorf("unsupported --scope mode %q (expected match or season)", mode)
	}
}

func normalizePlayerHistoryMode(raw string) (playerHistoryMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "both":
		return playerHistoryMode{Label: "both", Batting: true, Bowling: true}, nil
	case "batting":
		return playerHistoryMode{Label: "batting", Batting: true, Bowling: false}, nil
	case "bowling":
		return playerHistoryMode{Label: "bowling", Batting: false, Bowling: true}, nil
	default:
		return playerHistoryMode{}, fmt.Errorf("--mode must be one of: batting, bowling, both")
	}
}

func resolveLeagueIDForMatch(ctx context.Context, matchQuery string) (string, error) {
	service, err := newMatchService()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = service.Close()
	}()

	result, err := service.Show(ctx, strings.TrimSpace(matchQuery), cricinfo.MatchLookupOptions{})
	if err == nil {
		if match, ok := result.Data.(cricinfo.Match); ok {
			if strings.TrimSpace(match.LeagueID) != "" {
				return strings.TrimSpace(match.LeagueID), nil
			}
		}
		if match, ok := result.Data.(*cricinfo.Match); ok && match != nil {
			if strings.TrimSpace(match.LeagueID) != "" {
				return strings.TrimSpace(match.LeagueID), nil
			}
		}
	}

	details, derr := service.Details(ctx, strings.TrimSpace(matchQuery), cricinfo.MatchLookupOptions{})
	if derr != nil {
		if err != nil {
			return "", err
		}
		return "", derr
	}
	for _, item := range details.Items {
		if delivery, ok := item.(cricinfo.DeliveryEvent); ok {
			if strings.TrimSpace(delivery.LeagueID) != "" {
				return strings.TrimSpace(delivery.LeagueID), nil
			}
		}
		if delivery, ok := item.(*cricinfo.DeliveryEvent); ok && delivery != nil {
			if strings.TrimSpace(delivery.LeagueID) != "" {
				return strings.TrimSpace(delivery.LeagueID), nil
			}
		}
	}
	return "", fmt.Errorf("unable to resolve league for match %q", matchQuery)
}

func sumIntSlices(left, right []int) []int {
	size := len(left)
	if len(right) > size {
		size = len(right)
	}
	if size == 0 {
		return nil
	}
	out := make([]int, size)
	copy(out, left)
	for i, v := range right {
		out[i] += v
	}
	return out
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func renderHistoricalMapHeader(out io.Writer, playerQuery string, scope playerHistoryScope, mode playerHistoryMode, leagueID string, matchCount int) {
	fmt.Fprintln(out, "Historical Player Maps")
	fmt.Fprintf(out, "Player: %s\n", strings.TrimSpace(playerQuery))
	fmt.Fprintf(out, "Scope: %s:%s\n", scope.Mode, scope.Query)
	fmt.Fprintf(out, "Mode: %s\n", mode.Label)
	fmt.Fprintf(out, "League: %s\n", nonEmpty(strings.TrimSpace(leagueID), "(unknown)"))
	fmt.Fprintf(out, "Matches in scope: %d\n", matchCount)
}

func renderHistoryWarnings(out io.Writer, warnings []string) {
	unique := uniqueStrings(warnings)
	if len(unique) == 0 {
		return
	}
	fmt.Fprintln(out)
	for _, warning := range unique {
		fmt.Fprintf(out, "warning: %s\n", warning)
	}
}
