---
title: Quickstart
description: Install cricinfo, verify the command, and run the first match and JSON workflows.
summary: Install the npm package and run your first Cricinfo terminal queries.
order: 1
category: Start
---

# Quickstart

`cricinfo` is published on npm as `cricinfo-cli-go` and installs the `cricinfo` command. The package ships a Node shim that launches a Go binary. During install it uses a platform-specific release binary when available and can fall back to a local Go build.

## Install from npm

```bash
npm i -g cricinfo-cli-go
cricinfo --version
cricinfo --help
```

The root help lists every command family and the global output flags. Most workflows start by opening the exact help page for the area you need.

```bash
cricinfo matches --help
cricinfo players --help
cricinfo teams --help
cricinfo leagues --help
cricinfo search --help
cricinfo analysis --help
```

## First useful commands

```bash
# Current and live match discovery
cricinfo matches list
cricinfo matches live

# Inspect one match once you know its ID
cricinfo matches show 1529474
cricinfo matches scorecard 1529474

# Resolve entities
cricinfo players profile 1361257
cricinfo teams roster 789643 --match 1529474
cricinfo leagues seasons 19138
```

## Use structured output

Use text output for reading and structured output for scripts or agents.

```bash
cricinfo matches show 1529474 --format json
cricinfo matches list --format jsonl
cricinfo players profile 1361257 --format json
```

Global flags available across the CLI:

| Flag | Use |
| --- | --- |
| `--format text\|json\|jsonl` | Pick human-readable text, a single JSON payload, or list-friendly JSONL. |
| `--verbose` | Include richer summaries when a command supports them. |
| `--all-fields` | Keep long-tail fields in structured output instead of the compact default. |

## Work from source

Use the checked-in Makefile when developing locally.

```bash
make build
make install-local
cricinfo --help
```
