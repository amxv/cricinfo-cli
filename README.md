# cricinfo

Domain-driven Cricinfo CLI for real-time match, player, team, league, season, and analysis workflows.

The CLI uses normalized public Cricinfo API data and supports both:
- human-readable text output
- stable machine-friendly JSON output (`--format json`)

## Install

```bash
npm i -g @amxv/cricinfo
cricinfo --help
cricinfo --version
```

## Command Families

```text
cricinfo matches ...
cricinfo players ...
cricinfo teams ...
cricinfo leagues ...
cricinfo seasons ...
cricinfo standings ...
cricinfo competitions ...
cricinfo search ...
cricinfo analysis ...
```

Use `cricinfo <family> --help` to drill deeper.

## Quick Start

```bash
# Discover current matches
cricinfo matches list
cricinfo matches show 1529474
cricinfo matches scorecard 1529474

# Explore players and teams
cricinfo players profile 1361257
cricinfo players match-stats 1361257 --match 1529474
cricinfo teams roster 789643 --match 1529474

# Traverse leagues and seasons
cricinfo leagues show 19138
cricinfo leagues seasons 19138
cricinfo seasons show 19138 --season 2025

# Derived analysis
cricinfo analysis dismissals --league 19138 --seasons 2024-2025
cricinfo analysis bowling --metric economy --scope match:1529474
cricinfo analysis batting --metric strike-rate --scope season:2025 --league 19138
cricinfo analysis partnerships --scope season:2025 --league 19138
```

## JSON Output (Agent Friendly)

```bash
cricinfo matches show 1529474 --format json
cricinfo players profile 1361257 --format json
cricinfo analysis dismissals --league 19138 --seasons 2025 --format json
```

Global flags:
- `--format text|json|jsonl`
- `--verbose`
- `--all-fields`

## Development

```bash
make fmt
make test
make check
make build
make build-all
make test-live
make test-live-smoke
make fixtures-refresh
make npm-smoke
make acceptance
```

`make acceptance` runs an end-to-end live traversal smoke pass across install/help, matches, players, teams, leagues/seasons, analysis, and JSON rendering.

## Release

Tag-based GitHub Actions release:
- push tag `vX.Y.Z`
- workflow runs quality checks
- workflow builds cross-platform binaries with embedded tag version
- workflow publishes GitHub release assets
- workflow publishes npm package

Core files:
- `cmd/cricinfo/main.go`: process entrypoint
- `internal/cli/`: command tree + help UX
- `internal/cricinfo/`: transport, normalization, analysis, rendering
- `scripts/postinstall.js`: npm install binary downloader with local Go fallback
- `bin/cricinfo.js`: npm executable shim
- `.github/workflows/release.yml`: release pipeline
