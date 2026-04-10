# Cricinfo API Research Handoff (For Next Agent)

Last updated: 2026-04-10 (Asia/Kolkata)
Repo: `/Users/ashray/code/amxv/cricinfo-cli`

## 1) Objective Completed

This workspace now contains a live-validated exploration of Cricinfo public APIs, focused on building a future Go CLI for cricket statistics.

The research prioritized:
- Real endpoint reachability checks (only counting endpoints that returned `200`).
- Extraction of normalized endpoint templates.
- Extraction of field-level data points (scalar JSON paths).
- Consolidation into reusable inventory files.

## 2) Primary Source of Truth Files

These files are in `gg/agent-outputs` and should be treated as canonical outputs from this pass:

1. `/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-public-api-guide.md`
What it is:
- Human-readable guide of working endpoint families, key shapes, and implementation guidance.

2. `/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-working-endpoints.tsv`
Columns:
- `status_code`
- `full_url`
- `top_level_keys`
- `item_keys` (best effort)
What it is:
- Deduplicated list of sampled URLs that returned `200`.

3. `/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-working-templates.tsv`
Columns:
- `count`
- `normalized_template`
- `sample_url`
What it is:
- Aggregated endpoint templates with usage frequency from observed samples.

4. `/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-field-path-catalog.txt`
What it is:
- Unique scalar JSON paths discovered from successful responses.
- Use this to understand what data points exist.

5. `/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-field-path-frequency.tsv`
Columns:
- `count`
- `field_path`
What it is:
- Frequency distribution of discovered field paths.

6. `/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-player-stat-endpoints.tsv`
Columns:
- `status_code`
- `full_url`
- `top_level_keys`
- `item_keys`
What it is:
- Focused player-stat probe dataset.

7. `/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-player-stat-templates.tsv`
Columns:
- `count`
- `normalized_template`
- `sample_url`
What it is:
- Focused template map for player-centric endpoints.

8. `/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-player-top-keys-frequency.tsv`
What it is:
- Frequency of top-level response keys across focused player probes.

## 3) Coverage Snapshot

Current consolidated totals (from the generated artifacts):
- Working URL samples: `526`
- Normalized working endpoint templates: `56`
- Unique scalar field paths: `2536`

Important: this is high coverage but not mathematically exhaustive across all possible leagues/seasons/match states.

Focused player-stat pass summary:
- Player endpoint samples: `171`
- Status mix: `167` x `200`, `1` x `404`, `3` x `503`
- Player templates: `8`

## 4) Endpoint Families Confirmed Working

High-level confirmed route families:
- Root/discovery:
  - `/`
  - `/events`
  - `/leagues`
  - `/teams/{id}`
  - `/athletes/{id}`
  - `/athletes/{id}/news`
  - `/athletes/{id}/statistics`
- League/event/competition:
  - `/leagues/{id}/events/{id}/competitions/{id}` and subresources (`status`, `details`, `plays`, `matchcards`, `officials`, `broadcasts`, `tickets`, `odds`, `situation`, `situation/odds`)
- Competitor:
  - `/competitors/{id}`
  - `/competitors/{id}/scores`
  - `/competitors/{id}/roster`
  - `/competitors/{id}/leaders`
  - `/competitors/{id}/statistics`
  - `/competitors/{id}/records`
- Innings/period depth:
  - `/linescores/{inningsId}/{periodId}`
  - `/.../leaders`
  - `/.../statistics/{index}`
  - `/.../fow`, `/.../fow/{n}`
  - `/.../partnerships`, `/.../partnerships/{n}`
- Player-in-match depth:
  - `/roster/{playerId}/statistics/{index}`
  - `/roster/{playerId}/linescores`
  - `/roster/{playerId}/linescores/{inningsId}/{periodId}/statistics/{index}`
- League athlete routes:
  - `/leagues/{id}/athletes/{id}`

Player-heavy templates confirmed:
- `/athletes/{id}`
- `/athletes/{id}/statistics`
- `/athletes/{id}/news`
- `/leagues/{id}/athletes/{id}`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/statistics/{n}`

## 5) Known Caveats

1. Transient `503` can occur on otherwise valid routes.
- Use retries with backoff in CLI client.

2. Some responses are highly link-driven (`$ref`) and sparse unless followed.
- CLI should support generic link-following and not rely only on hardcoded routes.

3. `splits` shape is not always stable as a strict array/object contract.
- Keep flexible typing (`json.RawMessage`) for unstable structures.

4. A failed sweep attempt produced malformed temporary rows in `tmp/league_sweep.tsv`.
- Consolidated outputs in `gg/agent-outputs` were rebuilt from trusted probe files and are clean.

## 6) Other Relevant Locations Used During Research

Reference projects scanned:
- `/Users/ashray/code/amxv/zue-cricket`
- `/Users/ashray/code/ashrayxyz/yorkerjs`
- `/Users/ashray/code/ashrayxyz/blue`

Notable finding:
- `hs-consumer-api.espncricinfo.com` and `www.espncricinfo.com/.../engine/...json` endpoints returned `403` from this environment and were not included in working inventories.

Temporary research files (can be inspected or deleted later):
- `/Users/ashray/code/amxv/cricinfo-cli/tmp/endpoint_probe.tsv`
- `/Users/ashray/code/amxv/cricinfo-cli/tmp/more_probe.tsv`
- `/Users/ashray/code/amxv/cricinfo-cli/tmp/crawl_results.tsv`
- `/Users/ashray/code/amxv/cricinfo-cli/tmp/crawl2_results.tsv`
- plus other `tmp/crawl*` intermediates

## 7) Recommended Next Agent Plan

1. Build typed Go client package (`internal/cricinfo`):
- Generic GET + timeout + retry + backoff.
- `$ref` follower utility.
- Generic paginated envelope model.

2. Implement first command set using confirmed templates:
- `matches list`
- `matches status`
- `matches details`
- `matches plays`
- `teams roster`
- `players profile`
- `players news`

3. Add fixture-based contract tests:
- Save sample JSON for top 10 templates.
- Decode tests for core structs and tolerant decode for unstable parts.

4. Expand schema extraction:
- For each template in `cricinfo-working-templates.tsv`, sample N URLs and compute union schema.

5. Player-stat expansion:
- Add command group for player profile/stats/news/match-context stats.
- Build struct set that separates:
  - `AthleteProfile`
  - `AthleteStatistics`
  - `RosterPlayerStatistics`
- Keep `splits` as flexible payload until shape contracts are stabilized by fixture tests.

## 8) Quick Validation Commands for Next Agent

Run from repo root:

```bash
# Inspect top endpoint templates
sed -n '1,60p' gg/agent-outputs/cricinfo-working-templates.tsv

# Inspect most common field paths
sed -n '1,80p' gg/agent-outputs/cricinfo-field-path-frequency.tsv

# Inspect full guide
sed -n '1,220p' gg/agent-outputs/cricinfo-public-api-guide.md
```
