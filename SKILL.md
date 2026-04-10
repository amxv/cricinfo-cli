# Cricinfo CLI Usage Skill

Use `cricinfo` when you need Cricinfo data from the terminal without calling raw API endpoints yourself.

## What It Can Do

- Current and recent match listings
- Live scores and match status
- Scorecards, commentary-style plays, details, and situations
- Player profiles and player match statistics
- Team rosters, records, leaders, and statistics
- League, season, standings, and competition navigation
- Search flows for finding leagues, teams, players, and matches
- Derived cricket analysis such as dismissals, bowling metrics, batting metrics, and partnerships

## Start Here

Always begin with help:

```bash
cricinfo --help
```

Then drill into the command family you need:

```bash
cricinfo matches --help
cricinfo players --help
cricinfo teams --help
cricinfo leagues --help
cricinfo competitions --help
cricinfo search --help
cricinfo analysis --help
```

## Common Commands

```bash
# Live and recent matches
cricinfo matches list
cricinfo matches live
cricinfo matches show 1529474
cricinfo matches scorecard 1529474
cricinfo matches plays 1529474

# Players
cricinfo players profile 1361257
cricinfo players stats 1361257
cricinfo players match-stats 1361257 --match 1529474

# Teams
cricinfo teams show 789643
cricinfo teams roster 789643 --match 1529474
cricinfo teams statistics 789643 --match 1529474

# Leagues and seasons
cricinfo leagues show 19138
cricinfo leagues seasons 19138
cricinfo standings show 19138 --season 2025

# Search
cricinfo search players "Virat Kohli"
cricinfo search teams "Royal Challengers Bengaluru"

# Analysis
cricinfo analysis dismissals --league 19138 --seasons 2025
cricinfo analysis bowling --metric economy --scope match:1529474
```

## Output And Flags

Use structured output when another tool or agent will read the result:

```bash
cricinfo matches show 1529474 --format json
cricinfo matches list --format jsonl
```

Useful flags:

- `--format text|json|jsonl`
- `--verbose`
- `--all-fields`

## How To Work Efficiently

- If the CLI is not installed yet, install it with `npm i -g cricinfo-cli-go`, then run `cricinfo --help`.
- If you know an exact match, player, team, league, or season ID, use it directly.
- If you do not know the ID, start with `search` or a higher-level listing command and then drill down.
- Prefer `--format json` for single objects and `--format jsonl` for lists you want to pipe into other tools.
- Use `--help` on the exact subcommand whenever you are unsure about arguments or flags.
