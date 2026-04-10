package cricinfo

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPhase15FixtureDeterministicRankingAndGrouping(t *testing.T) {
	t.Parallel()

	players := []PlayerMatch{
		{
			PlayerID:   "1001",
			PlayerName: "Bowler A",
			TeamID:     "789643",
			TeamName:   "BOOST",
			MatchID:    "1529474",
			LeagueID:   "19138",
			Bowling: []StatCategory{{
				Name: "bowling",
				Stats: []StatValue{
					{Name: "dots", Value: 24, DisplayValue: "24"},
					{Name: "sixesConceded", Value: 1, DisplayValue: "1"},
					{Name: "balls", Value: 24, DisplayValue: "24"},
					{Name: "conceded", Value: 14, DisplayValue: "14"},
				},
			}},
			Batting: []StatCategory{{
				Name: "batting",
				Stats: []StatValue{
					{Name: "runs", Value: 40, DisplayValue: "40"},
					{Name: "ballsFaced", Value: 30, DisplayValue: "30"},
					{Name: "fours", Value: 5, DisplayValue: "5"},
					{Name: "sixes", Value: 1, DisplayValue: "1"},
				},
			}},
		},
		{
			PlayerID:   "1002",
			PlayerName: "Bowler B",
			TeamID:     "789644",
			TeamName:   "SGH",
			MatchID:    "1529474",
			LeagueID:   "19138",
			Bowling: []StatCategory{{
				Name: "bowling",
				Stats: []StatValue{
					{Name: "dots", Value: 12, DisplayValue: "12"},
					{Name: "sixesConceded", Value: 3, DisplayValue: "3"},
					{Name: "balls", Value: 24, DisplayValue: "24"},
					{Name: "conceded", Value: 28, DisplayValue: "28"},
				},
			}},
			Batting: []StatCategory{{
				Name: "batting",
				Stats: []StatValue{
					{Name: "runs", Value: 30, DisplayValue: "30"},
					{Name: "ballsFaced", Value: 20, DisplayValue: "20"},
					{Name: "fours", Value: 3, DisplayValue: "3"},
					{Name: "sixes", Value: 2, DisplayValue: "2"},
				},
			}},
		},
	}

	bowlingAgg := map[string]*analysisAggregate{}
	for _, player := range players {
		totals := extractBowlingTotals(player)
		row := analysisSourceRow{
			MatchID:       player.MatchID,
			LeagueID:      player.LeagueID,
			TeamID:        player.TeamID,
			TeamName:      player.TeamName,
			PlayerID:      player.PlayerID,
			PlayerName:    player.PlayerName,
			Dots:          totals.dots,
			SixesConceded: totals.sixesConceded,
			Balls:         totals.balls,
			RunsConceded:  totals.conceded,
		}
		key, dims := buildAnalysisGroup(row, []string{"player"})
		entry := bowlingAgg[key]
		if entry == nil {
			entry = &analysisAggregate{row: dims, matchIDs: map[string]struct{}{}}
			bowlingAgg[key] = entry
		}
		entry.matchIDs[row.MatchID] = struct{}{}
		entry.dots += row.Dots
		entry.sixesConceded += row.SixesConceded
		entry.balls += row.Balls
		entry.runsConceded += row.RunsConceded
	}

	economyRows := make([]AnalysisRow, 0, len(bowlingAgg))
	dotRows := make([]AnalysisRow, 0, len(bowlingAgg))
	sixRows := make([]AnalysisRow, 0, len(bowlingAgg))
	for key, entry := range bowlingAgg {
		economyRows = append(economyRows, AnalysisRow{Key: key, Value: economyFromAggregate(entry)})
		dotRows = append(dotRows, AnalysisRow{Key: key, Value: float64(entry.dots), Count: entry.dots})
		sixRows = append(sixRows, AnalysisRow{Key: key, Value: float64(entry.sixesConceded), Count: entry.sixesConceded})
	}

	economyRows = rankAnalysisRows(economyRows, true)
	dotRows = rankAnalysisRows(dotRows, false)
	sixRows = rankAnalysisRows(sixRows, false)

	if !strings.Contains(economyRows[0].Key, "Bowler A") {
		t.Fatalf("expected Bowler A to rank first by economy, rows=%+v", economyRows)
	}
	if !strings.Contains(dotRows[0].Key, "Bowler A") {
		t.Fatalf("expected Bowler A to rank first by dots, rows=%+v", dotRows)
	}
	if !strings.Contains(sixRows[0].Key, "Bowler B") {
		t.Fatalf("expected Bowler B to rank first by sixes conceded, rows=%+v", sixRows)
	}

	battingAgg := map[string]*analysisAggregate{}
	for _, player := range players {
		totals := extractBattingTotals(player)
		row := analysisSourceRow{
			MatchID:      player.MatchID,
			LeagueID:     player.LeagueID,
			TeamID:       player.TeamID,
			TeamName:     player.TeamName,
			PlayerID:     player.PlayerID,
			PlayerName:   player.PlayerName,
			Fours:        totals.fours,
			BattingSixes: totals.sixes,
			RunsScored:   totals.runs,
			BallsFaced:   totals.balls,
		}
		key, dims := buildAnalysisGroup(row, []string{"player"})
		entry := battingAgg[key]
		if entry == nil {
			entry = &analysisAggregate{row: dims, matchIDs: map[string]struct{}{}}
			battingAgg[key] = entry
		}
		entry.matchIDs[row.MatchID] = struct{}{}
		entry.fours += row.Fours
		entry.battingSixes += row.BattingSixes
		entry.runsScored += row.RunsScored
		entry.ballsFaced += row.BallsFaced
	}

	strikeRows := make([]AnalysisRow, 0, len(battingAgg))
	for key, entry := range battingAgg {
		strikeRows = append(strikeRows, AnalysisRow{Key: key, Value: strikeRateFromAggregate(entry)})
	}
	strikeRows = rankAnalysisRows(strikeRows, false)
	if !strings.Contains(strikeRows[0].Key, "Bowler B") {
		t.Fatalf("expected Bowler B to rank first by strike rate (30/20), rows=%+v", strikeRows)
	}

	dismissRows := []AnalysisRow{
		{Key: "dismissal=bowled | team=BOOST", Value: 3, Count: 3},
		{Key: "dismissal=caught | team=BOOST", Value: 3, Count: 3},
		{Key: "dismissal=lbw | team=SGH", Value: 2, Count: 2},
	}
	dismissRows = rankAnalysisRows(dismissRows, false)
	if dismissRows[0].Key > dismissRows[1].Key {
		t.Fatalf("expected deterministic tie-break ordering by key, rows=%+v", dismissRows)
	}
}

func TestPhase15BowlingActivityFilterSkipsNonBowlers(t *testing.T) {
	t.Parallel()

	nonBowler := &analysisAggregate{
		row: AnalysisRow{
			Key:        "player=Pure Batter",
			PlayerName: "Pure Batter",
		},
		matchIDs: map[string]struct{}{"1527689": {}},
	}
	bowler := &analysisAggregate{
		row: AnalysisRow{
			Key:        "player=Bowler",
			PlayerName: "Bowler",
		},
		matchIDs:     map[string]struct{}{"1527689": {}},
		balls:        24,
		runsConceded: 18,
	}

	if hasBowlingActivity(nonBowler) {
		t.Fatalf("expected non-bowling aggregate to be filtered out")
	}
	if !hasBowlingActivity(bowler) {
		t.Fatalf("expected bowling aggregate to be retained")
	}

	rows := []AnalysisRow{
		{
			Key:   bowler.row.Key,
			Value: economyFromAggregate(bowler),
		},
	}
	rows = rankAnalysisRows(rows, true)
	if len(rows) != 1 || !strings.Contains(rows[0].Key, "Bowler") {
		t.Fatalf("expected only active bowler to remain after filtering, rows=%+v", rows)
	}
}

func TestLivePhase15SmallHistoricalScope(t *testing.T) {
	t.Parallel()
	requireLiveMatrix(t)

	service, err := NewAnalysisService(AnalysisServiceConfig{})
	if err != nil {
		t.Fatalf("NewAnalysisService error: %v", err)
	}
	defer func() {
		_ = service.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	bowlingResult, err := service.Bowling(ctx, AnalysisMetricOptions{
		Metric:      "economy",
		Scope:       "season:2025",
		LeagueQuery: "19138",
		Top:         5,
	})
	if err != nil {
		t.Fatalf("Bowling live error: %v", err)
	}
	if bowlingResult.Status == ResultStatusError {
		if bowlingResult.Error != nil && bowlingResult.Error.StatusCode == 503 {
			t.Skipf("skipping after transient 503: %+v", bowlingResult.Error)
		}
		t.Fatalf("unexpected bowling live status error: %+v", bowlingResult)
	}

	dismissResult, err := service.Dismissals(ctx, AnalysisDismissalOptions{
		LeagueQuery: "19138",
		Seasons:     "2024-2025",
		Top:         5,
	})
	if err != nil {
		t.Fatalf("Dismissals live error: %v", err)
	}
	if dismissResult.Status == ResultStatusError {
		if dismissResult.Error != nil && dismissResult.Error.StatusCode == 503 {
			t.Skipf("skipping after transient 503: %+v", dismissResult.Error)
		}
		t.Fatalf("unexpected dismissals live status error: %+v", dismissResult)
	}

	dotsResult, err := service.Bowling(ctx, AnalysisMetricOptions{
		Metric:      "dots",
		Scope:       "match:1529474",
		LeagueQuery: "19138",
		Top:         5,
	})
	if err != nil {
		t.Fatalf("Bowling dots live error: %v", err)
	}
	if dotsResult.Status == ResultStatusError {
		if dotsResult.Error != nil && dotsResult.Error.StatusCode == 503 {
			t.Skipf("skipping after transient 503: %+v", dotsResult.Error)
		}
		t.Fatalf("unexpected bowling dots status error: %+v", dotsResult)
	}
}
