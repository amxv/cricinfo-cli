package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

const defaultOrangeCapURL = "https://timesofindia.indiatimes.com/sports/cricket/ipl/ipl-orange-cap-winner"

type orangeCapEntry struct {
	Rank       int
	Name       string
	Team       string
	Runs       int
	Matches    string
	Innings    string
	StrikeRate string
	Average    string
}

type orangeCapBoard struct {
	Season string
	Source string
	Top    []orangeCapEntry
}

func runOrangeCapCommand(cmd *cobra.Command) error {
	board, err := fetchOrangeCapBoard(cmd.Context(), defaultOrangeCapURL)
	if err != nil {
		return err
	}
	renderOrangeCapBoard(cmd.OutOrStdout(), board)
	return nil
}

func fetchOrangeCapBoard(ctx context.Context, sourceURL string) (orangeCapBoard, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return orangeCapBoard{}, err
	}
	req.Header.Set("User-Agent", "cricinfo-cli/orange-cap")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return orangeCapBoard{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return orangeCapBoard{}, fmt.Errorf("orange-cap source returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return orangeCapBoard{}, err
	}

	board, err := parseOrangeCapHTML(string(body))
	if err != nil {
		return orangeCapBoard{}, err
	}
	board.Source = strings.TrimSpace(sourceURL)
	return board, nil
}

func parseOrangeCapHTML(html string) (orangeCapBoard, error) {
	season := "current"
	if m := regexp.MustCompile(`Current IPL ([0-9]{4}) - Orange Cap Holder`).FindStringSubmatch(html); len(m) == 2 {
		season = strings.TrimSpace(m[1])
	}

	holderRE := regexp.MustCompile(`(?s)Current IPL [0-9]{4} - Orange Cap Holder.*?<div class="ouFXz"><a [^>]*>([^<]+)</a></div><div class="n0dXX">([^<]+)</div>.*?<div class="sgZYs">([0-9]+)`)
	holderMatch := holderRE.FindStringSubmatch(html)
	if len(holderMatch) != 4 {
		return orangeCapBoard{}, fmt.Errorf("unable to parse orange-cap holder from source")
	}

	statsRE := regexp.MustCompile(`(?s)<div class="r_XG0"><div>Matches:</div><div class="urmk6">([^<]+)</div></div><div class="r_XG0"><div>Innings:</div><div class="urmk6">([^<]+)</div></div><div class="r_XG0"><div>SR:</div><div class="urmk6">([^<]+)</div></div><div class="r_XG0"><div>Avg:</div><div class="urmk6">([^<]+)</div>`)
	statsMatch := statsRE.FindStringSubmatch(html)
	if len(statsMatch) != 5 {
		return orangeCapBoard{}, fmt.Errorf("unable to parse holder stats from source")
	}

	holderRuns, _ := strconv.Atoi(strings.TrimSpace(holderMatch[3]))
	board := orangeCapBoard{
		Season: season,
		Top: []orangeCapEntry{
			{
				Rank:       1,
				Name:       strings.TrimSpace(holderMatch[1]),
				Team:       strings.TrimSpace(holderMatch[2]),
				Runs:       holderRuns,
				Matches:    strings.TrimSpace(statsMatch[1]),
				Innings:    strings.TrimSpace(statsMatch[2]),
				StrikeRate: strings.TrimSpace(statsMatch[3]),
				Average:    strings.TrimSpace(statsMatch[4]),
			},
		},
	}

	rowRE := regexp.MustCompile(`(?s)<li><span class="g9cUV"><span>([0-9]+)</span></span>.*?<span class="o0TK8 jHHE0 ouFXz"><a [^>]*>([^<]+)</a><span class="up7ay ouFXz"><span class="B19Yr [^"]*"></span> <!-- -->([^<]+)</span></span><span class="_11Uoh">([0-9]+)</span><span class="mgOhH">([^<]+)</span><span class="w8Osy">([^<]+)</span><span class="uuWQH">([^<]+)</span><span class="vtnCN">([^<]+)</span></li>`)
	listBlock := html
	if m := regexp.MustCompile(`(?s)<ul class="kjRPJ OfwC3">(.*?)</ul>`).FindStringSubmatch(html); len(m) == 2 {
		listBlock = m[1]
	}
	rows := rowRE.FindAllStringSubmatch(listBlock, 20)
	for _, row := range rows {
		if len(row) != 9 {
			continue
		}
		rank, _ := strconv.Atoi(strings.TrimSpace(row[1]))
		runs, _ := strconv.Atoi(strings.TrimSpace(row[4]))
		board.Top = append(board.Top, orangeCapEntry{
			Rank:       rank,
			Name:       strings.TrimSpace(row[2]),
			Team:       strings.TrimSpace(row[3]),
			Runs:       runs,
			Matches:    strings.TrimSpace(row[5]),
			Innings:    strings.TrimSpace(row[6]),
			StrikeRate: strings.TrimSpace(row[7]),
			Average:    strings.TrimSpace(row[8]),
		})
	}

	if len(board.Top) == 0 {
		return orangeCapBoard{}, fmt.Errorf("orange-cap table is empty")
	}
	return board, nil
}

func renderOrangeCapBoard(out io.Writer, board orangeCapBoard) {
	fmt.Fprintf(out, "IPL Orange Cap (%s)\n", strings.TrimSpace(board.Season))
	fmt.Fprintf(out, "Source: %s\n", strings.TrimSpace(board.Source))
	fmt.Fprintln(out)

	leader := board.Top[0]
	fmt.Fprintln(out, "Current Holder")
	fmt.Fprintf(out, "  #%d %s (%s)\n", leader.Rank, leader.Name, leader.Team)
	fmt.Fprintf(out, "  %d runs | M %s | Inn %s | SR %s | Avg %s\n", leader.Runs, leader.Matches, leader.Innings, leader.StrikeRate, leader.Average)
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Top Batters")
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Rank\tPlayer\tTeam\tRuns\tM\tInn\tSR\tAvg")
	for _, row := range board.Top {
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n", row.Rank, truncateCell(row.Name, 28), row.Team, row.Runs, row.Matches, row.Innings, row.StrikeRate, row.Average)
	}
	_ = tw.Flush()
}

func truncateCell(v string, max int) string {
	v = strings.TrimSpace(v)
	if len(v) <= max {
		return v
	}
	if max <= 1 {
		return v[:max]
	}
	return v[:max-1] + "…"
}
