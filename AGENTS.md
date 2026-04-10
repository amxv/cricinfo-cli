# AGENTS.md

Guidance for coding agents working in `cricinfo`.

## Purpose

This repo is a Cricinfo Go command-line tool distributed through npm.

The sample command is `cricinfo`. Replace it with your actual CLI name and behavior.

## Architecture

- `cmd/cricinfo/main.go`: process entrypoint, error handling, exits non-zero on failure.
- `internal/app/app.go`: command parser + handlers.
- `internal/app/app_test.go`: starter tests.
- `bin/cricinfo.js`: npm shim that invokes packaged native binary.
- `scripts/postinstall.js`: downloads release binary on install, falls back to `go build`.
- `.github/workflows/release.yml`: tag-driven release pipeline.

## Local commands

Use `make` targets:

- `make fmt`
- `make test`
- `make vet`
- `make lint`
- `make check`
- `make build`
- `make build-all`
- `make install-local`

Direct commands:

- `go test ./...`
- `go vet ./...`
- `npm run lint`

When running Node or npm commands through background-process helpers on this machine, prefer `zsh -lc '<command>'` so the login shell initializes the expected `PATH`. Plain non-login `sh` execution may not find `npm`.

## How to customize safely

1. Rename CLI command consistently in all places:
- directory `cmd/cricinfo`
- `package.json` values (`bin`, `config.cliBinaryName`)
- `bin/cricinfo.js`
- workflow env `CLI_BINARY`
- `Makefile` `BIN_NAME`

2. Keep binary naming convention unchanged unless you also update postinstall/workflow:
- release assets: `<cli>_<goos>_<goarch>[.exe]`
- npm-installed binary path: `bin/<cli>-bin` (or `.exe` on Windows)

3. If adding dependencies, commit `go.sum` and optionally enable Go cache in workflow.

4. Keep help output expressive and command-local (`<command> --help` should explain examples).

## Release contract

Release pipeline triggers on `v*` tags and expects:

- `NPM_TOKEN` GitHub secret present.
- npm package name in `package.json` is publishable under your account/org.
- repository URL matches the release origin used by `scripts/postinstall.js`.

## Guardrails

- Prefer additive changes; do not break the release asset naming contract unintentionally.
- If you change release artifacts or CLI binary name, update both workflow and postinstall script in the same PR.
