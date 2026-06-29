---
title: Development
description: Build, test, smoke, and validate the Go CLI and npm shim from source.
summary: Local development commands and repository architecture for contributors and agents.
order: 7
category: Operations
---

# Development

The repository ships a Go binary through an npm package. Treat the Go CLI, Node shim, install script, Makefile, and release workflow as one contract.

## Repository layout

| Path | Purpose |
| --- | --- |
| `cmd/cricinfo/main.go` | Process entrypoint and exit handling. |
| `internal/app/app.go` | Command parsing and top-level dispatch. |
| `internal/cli/` | Command families, help text, and flag wiring. |
| `internal/cricinfo/` | Transport, normalization, rendering, resolver, fixtures, and analysis logic. |
| `bin/cricinfo.js` | npm shim that launches the installed native binary. |
| `scripts/postinstall.js` | Downloads tagged release assets or falls back to a local Go build. |
| `Makefile` | Local build, check, smoke, acceptance, and release helpers. |
| `.github/workflows/release.yml` | Tag-driven CI, GitHub release assets, and npm publish pipeline. |

## Preferred local commands

Use the Makefile targets whenever possible.

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
```

`make check` runs format, tests, vet, and Node lint checks. `make npm-smoke` packs the npm package, installs it into a temporary prefix, and verifies `cricinfo --help`.

## Live validation

Live commands intentionally hit Cricinfo data and are opt-in.

```bash
make test-live
make test-live-smoke
make acceptance
```

The acceptance flow builds the binary, runs the npm smoke test, checks major help pages, traverses matches, players, teams, leagues, seasons, and analysis commands, and verifies common text views do not expose raw upstream URLs.

## Local install from source

```bash
make build
make install-local
~/.local/bin/cricinfo --help
```

## Guardrails for contributors

- Keep the CLI binary name `cricinfo` aligned across package metadata, shim, postinstall, Makefile, and release assets.
- Prefer command-local help text so `cricinfo <family> --help` explains the next useful command.
- If you add dependencies, commit the updated lock or checksum files and run the release-relevant checks.
- Do not change release artifact naming without updating all install and publish paths together.
