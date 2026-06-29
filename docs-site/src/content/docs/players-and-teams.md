---
title: Players and teams
description: Resolve players and teams, inspect profiles, rosters, match stats, dismissals, deliveries, leaders, and records.
summary: Player and team commands for entity lookup and match-scoped cricket data.
order: 4
category: Commands
---

# Players and teams

Player and team commands accept IDs, refs, and aliases. When a route is match-specific, include `--match`; when resolution needs context, include `--league`.

## Find a player

```bash
cricinfo players search "Virat Kohli"
cricinfo search players "Virat Kohli" --league 19138
```

Once you know the player value, use it directly.

```bash
cricinfo players profile 1361257
cricinfo players news 1361257
cricinfo players stats 1361257
cricinfo players career 1361257
```

## Player match workflows

```bash
cricinfo players match-stats 1361257 --match 1529474
cricinfo players innings 1361257 --match 1529474
cricinfo players dismissals 1361257 --match 1529474
cricinfo players deliveries 1361257 --match 1529474
cricinfo players batting 1361257 --match 1529474
cricinfo players bowling 1361257 --match 1529474
```

These commands help you move from a player profile to the exact match-level batting, bowling, fielding, dismissal, and delivery views.

## Historical player maps

```bash
cricinfo players map-history 1361257 --scope season:2025 --league 8048
```

Use `map-history` for aggregated batting and bowling map data over a selected scope.

## Team discovery

```bash
cricinfo search teams "Royal Challengers Bengaluru"
cricinfo teams show rr --league 11132
```

## Team match workflows

```bash
cricinfo teams roster rr --match 1529474 --league 11132
cricinfo teams leaders rr --match 1529474 --league 11132
cricinfo teams records rr --match 1529474 --league 11132
cricinfo teams scores rr --match 1529474 --league 11132
cricinfo teams statistics rr --match 1529474 --league 11132
```

Team routes are useful for roster-driven workflows, competitor score summaries, and match-scoped leaders or record categories.
