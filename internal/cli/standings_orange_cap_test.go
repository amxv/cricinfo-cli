package cli

import (
	"strings"
	"testing"
)

func TestParseOrangeCapHTML(t *testing.T) {
	html := `<div class="aBcVY">Current IPL 2026 - Orange Cap Holder</div>
<div class="ouFXz"><a href="/p/1">Virat Kohli</a></div><div class="n0dXX">Royal Challengers Bengaluru</div>
<div class="sgZYs">328</div>
<div class="r_XG0"><div>Matches:</div><div class="urmk6">7</div></div><div class="r_XG0"><div>Innings:</div><div class="urmk6">7</div></div><div class="r_XG0"><div>SR:</div><div class="urmk6">163.18</div></div><div class="r_XG0"><div>Avg:</div><div class="urmk6">54.67</div></div>
<ul class="kjRPJ OfwC3">
<li><span class="g9cUV"><span>2</span></span><span class="o0TK8 jHHE0 ouFXz"><a href="/p/2">Abhishek Sharma</a><span class="up7ay ouFXz"><span class="B19Yr NWcSY"></span> <!-- -->SRH</span></span><span class="_11Uoh">323</span><span class="mgOhH">7</span><span class="w8Osy">7</span><span class="uuWQH">215</span><span class="vtnCN">53.83</span></li>
</ul>`

	board, err := parseOrangeCapHTML(html)
	if err != nil {
		t.Fatalf("parseOrangeCapHTML error: %v", err)
	}
	if board.Season != "2026" {
		t.Fatalf("expected season 2026, got %q", board.Season)
	}
	if len(board.Top) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(board.Top))
	}
	if board.Top[0].Name != "Virat Kohli" || board.Top[0].Runs != 328 {
		t.Fatalf("unexpected holder row: %+v", board.Top[0])
	}
	if board.Top[1].Name != "Abhishek Sharma" || board.Top[1].Team != "SRH" || board.Top[1].Runs != 323 {
		t.Fatalf("unexpected second row: %+v", board.Top[1])
	}
}

func TestRenderOrangeCapBoard(t *testing.T) {
	board := orangeCapBoard{
		Season: "2026",
		Source: "https://example.com",
		Top: []orangeCapEntry{
			{Rank: 1, Name: "Virat Kohli", Team: "RCB", Runs: 328, Matches: "7", Innings: "7", StrikeRate: "163.18", Average: "54.67"},
			{Rank: 2, Name: "Abhishek Sharma", Team: "SRH", Runs: 323, Matches: "7", Innings: "7", StrikeRate: "215", Average: "53.83"},
		},
	}
	var b strings.Builder
	renderOrangeCapBoard(&b, board)
	out := b.String()
	for _, want := range []string{"IPL Orange Cap (2026)", "Current Holder", "Virat Kohli", "Top Batters"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got %q", want, out)
		}
	}
}
