---
title: Command map
description: Understand the command families and when to use each one.
summary: A high-level map of matches, players, teams, leagues, search, and analysis commands.
order: 2
category: Commands
---

# Command map

The CLI is organized around cricket domains. Start broad, resolve an ID or alias, then drill into the specific object or scoped workflow.

## Top-level families

| Family | What it is for | Common starts |
| --- | --- | --- |
| `matches` | Current matches, live views, scorecards, innings, lineups, plays, phases, and matchup views. | `cricinfo matches live`, `cricinfo matches show <match>` |
| `players` | Player search, profile, news, career stats, match stats, innings, dismissals, deliveries, batting, and bowling splits. | `cricinfo players search <query>`, `cricinfo players profile <player>` |
| `teams` | Team summary, roster, leaders, records, scores, and match-scoped statistics. | `cricinfo teams show <team>`, `cricinfo teams roster <team> --match <match>` |
| `leagues` | League list, events, calendar, athletes, standings, and seasons. | `cricinfo leagues list`, `cricinfo leagues seasons <league>` |
| `seasons` | Season, season type, and group traversal for a league. | `cricinfo seasons show <league> --season 2025` |
| `standings` | Standings traversal and IPL Orange Cap leaderboard. | `cricinfo standings show <league>`, `cricinfo standings orange-cap` |
| `competitions` | Match competition metadata such as officials, broadcasts, tickets, and odds. | `cricinfo competitions metadata <match>` |
| `search` | Cross-entity discovery for matches, players, teams, and leagues. | `cricinfo search players "Virat Kohli"` |
| `analysis` | Derived rankings over hydrated match or season scopes. | `cricinfo analysis batting --metric strike-rate --scope season:2025 --league 19138` |

## Match commands

`matches` is the fastest way to discover the current cricket surface and inspect a known match.

```bash
cricinfo matches list
cricinfo matches live
cricinfo matches show 1529474
cricinfo matches status 1529474
cricinfo matches live-view 1529474
cricinfo matches scorecard 1529474
cricinfo matches innings 1529474
cricinfo matches lineup 1529474
cricinfo matches plays 1529474
cricinfo matches deliveries 1529474
cricinfo matches partnerships 1529474
cricinfo matches phases 1529474
cricinfo matches pitch-map 1529474
```

Some match commands accept `--league` to provide resolution context when a match ID or alias is ambiguous.

## Entity commands

Entity commands can resolve IDs, refs, and aliases. Use `search` first when you do not know the exact ID.

```bash
cricinfo search players "Virat Kohli"
cricinfo search teams "Royal Challengers Bengaluru"
cricinfo search leagues "IPL"
cricinfo search matches "India Australia"
```

Then pass the resolved value into a more specific command.

```bash
cricinfo players profile 1361257
cricinfo teams show rr --league 11132
cricinfo leagues events 19138
```

## Analysis commands

Analysis commands rank or group cricket metrics over an explicit scope. A match scope can be used directly; a season scope usually needs a league.

```bash
cricinfo analysis dismissals --league 19138 --seasons 2025
cricinfo analysis bowling --metric economy --scope match:1529474
cricinfo analysis batting --metric strike-rate --scope season:2025 --league 19138
cricinfo analysis partnerships --scope season:2025 --league 19138
```
