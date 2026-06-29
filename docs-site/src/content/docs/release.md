---
title: Release
description: Understand the tag-driven GitHub release and npm publish contract.
summary: How cricinfo versions become GitHub release binaries and the npm package.
order: 8
category: Operations
---

# Release

Releases are tag-driven. Push a `vX.Y.Z` tag and GitHub Actions handles validation, binary builds, GitHub release assets, and npm publishing.

## Release flow

1. Run Go and Node quality checks.
2. Build release binaries for supported platforms.
3. Publish GitHub release assets.
4. Publish the npm package at the tag version.

## Artifact contract

| Item | Convention |
| --- | --- |
| Binary name | `cricinfo` |
| npm package | `cricinfo-cli-go` |
| npm command | `cricinfo` from `bin/cricinfo.js` |
| GitHub release asset | `cricinfo_<goos>_<goarch>[.exe]` |
| Install fallback | `scripts/postinstall.js` builds locally if no matching asset is available. |

Supported release targets are built by `make build-all`:

```text
darwin/amd64
darwin/arm64
linux/amd64
linux/arm64
windows/amd64
```

## Create a tag

```bash
make release-tag VERSION=x.y.z
```

The Makefile creates and pushes `vX.Y.Z`. Make sure the package version and intended tag are aligned before publishing.

## When changing release internals

If you change the CLI name, package name, binary naming, or release asset layout, update these together:

- `package.json`
- `bin/cricinfo.js`
- `scripts/postinstall.js`
- `Makefile`
- `.github/workflows/release.yml`

Keeping those files synchronized preserves the install contract for npm users and automation that expects the `cricinfo` command.
