# AGENTS.md

Guidance for coding agents working in `cricinfo-cli`.

## Purpose

This repository ships the `cricinfo` Go CLI through npm. Treat the npm package, the Go binary, and the release workflow as one contract.

## Architecture

- `cmd/cricinfo/main.go`: process entrypoint and exit handling.
- `internal/app/app.go`: command parsing and top-level dispatch.
- `internal/cli/`: command families, help text, and flag wiring.
- `internal/cricinfo/`: transport, normalization, rendering, and analysis logic.
- `bin/cricinfo.js`: npm shim that launches the installed native binary.
- `scripts/postinstall.js`: downloads the tagged binary or falls back to `go build`.
- `Makefile`: local build, test, smoke, and release helpers.
- `.github/workflows/release.yml`: tag-driven build and publish pipeline.

## Local Commands

Prefer the checked-in `Makefile` targets:

- `make fmt`
- `make test`
- `make vet`
- `make lint`
- `make check`
- `make build`
- `make build-all`
- `make npm-smoke`
- `make acceptance`
- `make test-live`
- `make test-live-smoke`
- `make fixtures-refresh`
- `make install-local`

Direct commands that are also safe to use:

- `go test ./...`
- `go vet ./...`
- `npm run test`
- `npm run lint`

When running Node or npm commands through background helpers on this machine, prefer `zsh -lc '<command>'` so the login shell initializes the expected `PATH`.

## Release Contract

Releases trigger on `v*` tags and expect:

- `NPM_TOKEN` to be configured in GitHub Actions.
- `package.json` to keep the published npm package name and `config.cliBinaryName` aligned with the CLI install contract.
- release assets to follow `<cli>_<goos>_<goarch>[.exe]`.
- `scripts/postinstall.js` to be able to fetch the matching GitHub release asset or build from source.

If you touch release artifacts or the binary name, update `package.json`, `bin/cricinfo.js`, `scripts/postinstall.js`, `Makefile`, and `.github/workflows/release.yml` in the same change.

## Guardrails

- Prefer additive changes and keep the existing CLI naming contract intact.
- Do not rewrite user changes in unrelated files.
- Keep help output concrete and command-local so `<command> --help` explains the next step.
- If you add dependencies, commit the updated `go.sum` and verify the release workflow still builds cleanly.
