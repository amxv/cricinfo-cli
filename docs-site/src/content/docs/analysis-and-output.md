---
title: Analysis and output modes
description: Run scoped batting, bowling, dismissal, and partnership analysis with text, JSON, and JSONL output.
summary: How to compose analysis scopes and choose the right output format for humans or agents.
order: 6
category: Automation
---

# Analysis and output modes

`cricinfo analysis` runs derived cricket rankings over hydrated live data. Hydration happens inside the current command execution; no persistent analysis cache is written.

## Analysis scopes

Use `match:<id>` for one match or `season:<year>` with `--league` for season traversal.

```bash
cricinfo analysis bowling --metric economy --scope match:1529474
cricinfo analysis batting --metric strike-rate --scope season:2025 --league 19138
cricinfo analysis partnerships --scope season:2025 --league 19138
cricinfo analysis dismissals --league 19138 --seasons 2024-2025
```

Optional bounds and traversal controls:

```bash
cricinfo analysis batting --metric runs --scope season:2025 --league 19138 --date-from 2025-03-01 --date-to 2025-05-31
cricinfo analysis bowling --metric wickets --scope season:2025 --league 19138 --type 2 --group 1 --match-limit 20
```

## Text for humans

Text is the default and is meant for quick terminal reading.

```bash
cricinfo analysis dismissals --league 19138 --seasons 2025
cricinfo teams leaders rr --match 1529474 --league 11132
```

The CLI avoids leaking raw upstream URLs in text views for common analysis and team leader workflows.

## JSON for single objects

Use JSON when another program needs the complete normalized payload.

```bash
cricinfo matches show 1529474 --format json
cricinfo players profile 1361257 --format json
cricinfo analysis bowling --metric economy --scope match:1529474 --format json
```

Add `--all-fields` when you want extended fields retained in structured output.

```bash
cricinfo matches show 1529474 --format json --all-fields
```

## JSONL for lists

JSONL is best for list-oriented automation because each row can be streamed or piped independently.

```bash
cricinfo matches list --format jsonl
cricinfo leagues events 19138 --format jsonl
cricinfo search players "Kohli" --format jsonl
```

## Agent-friendly pattern

A reliable agent workflow is:

1. Use `search` or a list command with JSONL.
2. Select an ID from the result.
3. Call the precise `show`, `profile`, `scorecard`, or `analysis` command with JSON.
4. Use text only for final user-facing summaries.

```bash
cricinfo search players "Virat Kohli" --format jsonl
cricinfo players profile 1361257 --format json
```
