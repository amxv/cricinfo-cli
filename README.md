# cricinfo

`cricinfo` is a Go-powered Cricinfo CLI published on npm as `cricinfo-cli-go`. It is designed for both human workflows and agent workflows, with readable text output by default and structured JSON or JSONL for automation.

## Install

Install the latest published package globally:

```bash
npm i -g cricinfo-cli-go
cricinfo --version
cricinfo --help
```

If you are working from source, the repo also ships local build targets:

```bash
make build
make install-local
```

The npm package installs a platform-specific binary when available and falls back to a local Go build during `postinstall` if needed.

## Usage

Start at the root help, then drill into the family you need:

```bash
cricinfo --help
cricinfo matches --help
cricinfo players --help
cricinfo teams --help
cricinfo leagues --help
cricinfo analysis --help
```

Common command families include:

```text
matches
players
teams
leagues
seasons
standings
competitions
search
analysis
```

Typical workflows:

```bash
# Discover current or recent match data
cricinfo matches list
cricinfo matches live
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
cricinfo analysis dismissals --league 19138 --seasons 2025
cricinfo analysis bowling --metric economy --scope match:1529474
cricinfo analysis batting --metric strike-rate --scope season:2025 --league 19138
cricinfo analysis partnerships --scope season:2025 --league 19138
```

## Output Modes

Use `--format text|json|jsonl` to match the consumer:

```bash
cricinfo matches show 1529474 --format json
cricinfo matches list --format jsonl
cricinfo players profile 1361257 --format json
```

Helpful global flags:

- `--verbose` for richer summaries
- `--all-fields` to retain extended entity data in structured output
- `--format jsonl` for list-oriented machine processing

## Development

Use the checked-in `Makefile` targets whenever possible:

```bash
make fmt
make test
make vet
make lint
make check
make build
make build-all
make npm-smoke
make acceptance
make test-live
make test-live-smoke
make fixtures-refresh
```

`make check` runs format, tests, vet, and lint. `make acceptance` performs a live end-to-end traversal across install/help, matches, players, teams, leagues, analysis, and JSON rendering.

## Release

Releases are tag-driven. Push a `vX.Y.Z` tag and GitHub Actions will:

1. run Go and Node quality checks
2. build release binaries for the supported platforms
3. publish the GitHub release assets
4. publish the npm package at the tag version

Release artifact conventions:

- binary name: `cricinfo`
- GitHub release assets: `cricinfo_<goos>_<goarch>[.exe]`
- npm shim: `bin/cricinfo.js`
- install fallback: `scripts/postinstall.js`

If you change the CLI name or release artifact layout, update `package.json`, `bin/cricinfo.js`, `scripts/postinstall.js`, `Makefile`, and `.github/workflows/release.yml` together.
