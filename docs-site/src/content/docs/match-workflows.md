---
title: Match workflows
description: Move from live match discovery to scorecards, innings, phases, partnerships, and delivery-level views.
summary: Practical match commands for live, scorecard, innings, and delivery inspection.
order: 3
category: Commands
---

# Match workflows

Use match commands when you want the current cricket slate or a detailed view of one fixture.

## Discover live and recent matches

```bash
cricinfo matches list
cricinfo matches live
cricinfo matches list --format jsonl
```

Use JSONL when you want a list that can be piped into other tools.

## Inspect one match

```bash
cricinfo matches show 1529474
cricinfo matches status 1529474
cricinfo matches live-view 1529474
```

`live-view` is the fan-first snapshot: batters, bowlers, figures, recent balls, and the most useful current-match state.

## Scorecard and innings detail

```bash
cricinfo matches scorecard 1529474
cricinfo matches innings 1529474
cricinfo matches lineup 1529474
```

Use these when you need batting, bowling, partnerships, innings summaries, and starting lineups.

## Delivery and phase views

```bash
cricinfo matches plays 1529474
cricinfo matches details 1529474
cricinfo matches deliveries 1529474
cricinfo matches phases 1529474
cricinfo matches pitch-map 1529474
```

These commands are useful when you need ball-by-ball inspection, coordinate-aware views, or powerplay/middle/death-over splits.

## Partnership and wicket views

```bash
cricinfo matches partnerships 1529474
cricinfo matches fow 1529474
```

Use them to summarize stand-building and fall-of-wicket flow without manually parsing a full scorecard.

## Batter versus bowler views

```bash
cricinfo matches duel 1529474
cricinfo matches matchup-history 1529474
```

`duel` focuses on a single-match matchup summary. `matchup-history` aggregates a historical batter-vs-bowler view when the required data can be hydrated.
