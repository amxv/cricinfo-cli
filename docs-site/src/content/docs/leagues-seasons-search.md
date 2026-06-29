---
title: Leagues, seasons, and search
description: Navigate leagues, events, calendars, standings, season types, groups, and cross-entity search.
summary: Traversal workflows for finding the right competition context before deeper commands.
order: 5
category: Commands
---

# Leagues, seasons, and search

League and season commands are the bridge between high-level competition discovery and match or analysis commands.

## League discovery

```bash
cricinfo leagues list
cricinfo leagues show 19138
cricinfo leagues events 19138
cricinfo leagues calendar 19138
cricinfo leagues athletes 19138
cricinfo leagues standings 19138
cricinfo leagues seasons 19138
```

Use league events and calendars when you need match IDs for later match, team, player, or analysis commands.

## Season traversal

```bash
cricinfo seasons show 19138 --season 2025
cricinfo seasons types 19138 --season 2025
cricinfo seasons groups 19138 --season 2025 --type 2
```

Season commands normalize pointer-heavy upstream data into simple terminal views so you can find the right season, type, or group before running a scoped workflow.

## Standings

```bash
cricinfo standings show 19138
cricinfo leagues standings 19138
cricinfo standings orange-cap
```

Use `standings orange-cap` for the current IPL Orange Cap leaderboard.

## Competition metadata

```bash
cricinfo competitions show 1529474
cricinfo competitions officials 1529474
cricinfo competitions broadcasts 1529474
cricinfo competitions tickets 1529474
cricinfo competitions odds 1529474
cricinfo competitions metadata 1529474
```

Empty-but-valid metadata collections render as clean zero-result views, which makes them safe in scripts.

## Search with context

```bash
cricinfo search players "Virat Kohli" --league 19138
cricinfo search teams "Rajasthan Royals" --league 11132
cricinfo search leagues "IPL"
cricinfo search matches "India Australia" --limit 5
```

Search can use known context values to seed the resolver.

```bash
cricinfo search players "Kohli" --match 1529474 --league 19138
cricinfo search matches "final" --league-ref /v2/sports/cricket/leagues/19138
cricinfo search teams "rr" --season-ref /v2/sports/cricket/leagues/11132/seasons/2025
```
