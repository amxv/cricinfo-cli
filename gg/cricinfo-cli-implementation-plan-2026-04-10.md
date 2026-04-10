# Cricinfo CLI Implementation Plan

Date: 2026-04-10  
Repo: `/Users/ashray/code/amxv/cricinfo-cli`

This plan assumes the user-approved product direction:

- The CLI is a domain-driven exploration tool for the public Cricinfo API.
- The CLI must not expose raw endpoint commands as part of its public UX.
- Every publicly accessible piece of Cricinfo data that is reachable through the validated API inventories should be represented somewhere in the CLI through discoverable commands and help.
- The CLI must be easy for both humans and agents to use step by step.
- Every phase must include validation, and implementation agents should verify against live API data in addition to fixtures.

## State of Current System

- The repository is currently a shipping scaffold, not a Cricinfo product.
- The native entrypoint in `cmd/cricinfo/main.go` only calls `app.Run`, prints `Error:`, and exits non-zero on failure.
- The entire command surface in `internal/app/app.go` is a manual parser with root help, `--version`, and a single `hello` command.
- `internal/app/app_test.go` only validates root help, version, `hello`, and unknown-command failure.
- The npm shim, postinstall downloader, Makefile, and release workflow are already in place and already matter:
  - `package.json`
  - `bin/cricinfo.js`
  - `scripts/postinstall.js`
  - `Makefile`
  - `.github/workflows/release.yml`
- The packaging contract already expects:
  - npm executable name `cricinfo`
  - release assets named like `cricinfo_<goos>_<goarch>[.exe]`
  - installed binary names `bin/cricinfo-bin` or `bin/cricinfo.exe`

Current codebase gaps:

- No HTTP transport layer.
- No retry/backoff policy for transient `503`.
- No `$ref` traversal layer.
- No paginator abstraction.
- No resolver or search/indexing layer.
- No stable domain models for matches, players, teams, leagues, seasons, innings, or statistics.
- No rendering boundary for human-readable output versus agent-friendly JSON.
- No fixture corpus for representative Cricinfo payloads.
- No live integration test harness for the validated endpoint families.

Research and live-probe facts that shape the plan:

- The validated inventories already cover `526` working URLs, `56` normalized templates, and `2536` scalar field paths.
- Confirmed public families include:
  - discovery roots
  - leagues, events, competitions
  - competitors, rosters, team leaders, team statistics, team records
  - match status, details, plays, matchcards, officials, broadcasts, tickets, odds, situation
  - innings and period depth
  - fall-of-wicket and partnerships
  - player profile, news, global statistics, and match-context statistics
  - calendar, seasons, season types, groups, standings
- The API is mixed-shape:
  - some routes are paginated `items[]` envelopes
  - some routes are object-shaped collections such as roster payloads
  - some routes return only `$ref` hops that must be followed
  - some useful routes work even when the parent payload omits or nulls them
- The API is rich enough to support direct cricket reasoning:
  - ball-level detail records include `dismissal`, `scoreValue`, `over`, `period`, `playType`, `bbbTimestamp`, `speedKPH`, `xCoordinate`, `yCoordinate`
  - roster-player match statistics include `dismissalName`, `dismissalCard`, `dots`, `economyRate`, `foursConceded`, `sixesConceded`, `maidens`, `wides`, `noballs`
  - innings statistics include over-by-over splits and wicket events with detail refs
  - matchcards expose batting, bowling, and partnerships in a concise card format
  - team leaders expose batting and bowling leaders with linked player statistics
- Discovery is uneven:
  - `/events` is small and practical for live use
  - `/athletes` is very large at `106137` records across `4246` pages
  - some navigation routes such as standings and group trees are multi-hop ref chains
- The current release workflow builds binaries without injecting the version ldflag, so released binaries may report `dev` even when local builds do not.
- The current npm shim and postinstall script have inconsistent fallback defaults for the CLI name if `package.json` config is missing.

## State of Ideal System

The finished CLI should behave like a guided cricket data graph, not a thin API wrapper.

Core product properties:

- All public Cricinfo data families are mapped into a clean command tree.
- Agents can start from almost no API knowledge and discover deeper layers with `help`.
- Commands are named around cricket concepts rather than transport paths.
- The CLI hides `$ref` traversal, path canonicalization, pagination quirks, and collection-shape differences.
- Output is stable and normalized:
  - readable summary text by default
  - structured JSON for agents
  - full-detail modes for long-tail data without exposing raw endpoints
- Every command points to logical next steps in help text.
- Commands accept names, IDs, and cached aliases where practical.
- Historical and live questions can both be answered:
  - direct entity inspection for current or single-match questions
  - scoped aggregation for league and season questions such as dismissal patterns across the last three IPL seasons

Recommended public command tree:

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

Expected public capabilities by command family:

- `matches`
  - live, list, show, status, scorecard, details, plays, situation, innings, deliveries, partnerships, fow, leaders
- `players`
  - search, profile, news, stats, career, match-stats, innings, dismissals, deliveries, bowling, batting
- `teams`
  - search, show, roster, scores, leaders, statistics, records, innings
- `leagues`
  - list, show, events, calendar, athletes, standings, seasons
- `seasons`
  - show, types, groups
- `competitions`
  - show, officials, broadcasts, tickets, odds, metadata
- `search`
  - cross-entity discovery for players, teams, leagues, and matches
- `analysis`
  - derived answers across one or many matches, leagues, or seasons using normalized CLI data

Expected agent-friendly output model:

- Default text view:
  - concise summaries
  - drill-down hints
  - clearly labeled sections
- `--format json`
  - normalized domain objects, not raw upstream payloads
- `--format jsonl`
  - for list or streaming use cases where appropriate
- `--verbose`
  - more detail within the normalized model
- `--all-fields`
  - long-tail data that is not shown in the default view but is still part of the mapped domain model

Expected internal architecture:

- CLI boundary
  - command tree, help, flags, examples, next-step guidance
- transport boundary
  - HTTP fetch, retry, timeout, headers, live client configuration
- hypermedia boundary
  - `$ref` following, multi-hop traversal, canonicalization handling
- resolver boundary
  - names, IDs, aliases, cached entity lookup, context-sensitive matching
- schema boundary
  - stable normalized models plus extension blocks for long-tail fields
- rendering boundary
  - text, JSON, JSONL
- verification boundary
  - fixtures, live tests, coverage ledger against researched templates
- analysis boundary
  - scoped aggregation over normalized match, innings, player, and delivery data

Cricket-specific product requirement:

- The CLI must expose enough structured batting, bowling, innings, and dismissal data for agents to answer questions such as:
  - who was the most economical bowler in a match or season scope
  - who bowled the most dot balls
  - which bowlers conceded the most sixes
  - which dismissal types are most common in a league over a chosen season range
- Those answers should come from normalized CLI data and CLI-level aggregation, not from agents reconstructing raw endpoints by hand.

## Cross-provider Requirements

- Upstream data provider:
  - the public Cricinfo API under `http://core.espnuk.org/v2/sports/cricket`
- Public UX requirement:
  - do not expose raw endpoint commands as a supported user-facing surface
- Distribution surfaces:
  - native Go binary
  - npm-installed shim
  - GitHub release assets
- Cross-platform build requirement:
  - prefer pure-Go dependencies and avoid CGO-only storage or database choices that would complicate multi-platform release binaries
- Runtime reliability requirement:
  - normalize transient `503` behavior with retry/backoff and clear error messages
- Testing requirement:
  - every phase ships with fixture coverage and live validation for the new endpoint families
- Output compatibility requirement:
  - default text and JSON views must stay semantically aligned so agents can switch formats without losing access to mapped data
- Coverage requirement:
  - every working template family from the inventories must map to at least one public command or subview by the end of the plan

## Plan Phases

### Phase 1: CLI Framework And Root Contract

Files to read before starting:

- `cmd/cricinfo/main.go`
- `internal/app/app.go`
- `internal/app/app_test.go`
- `internal/buildinfo/buildinfo.go`
- `go.mod`
- `package.json`
- `Makefile`
- `.github/workflows/release.yml`
- `gg/cricinfo-cli-foundation-research-report-2026-04-10.md`

What to do:

- Replace the manual single-file parser with a real command framework.
- Default recommendation: use Cobra for subcommands, help generation, and shell completion support.
- Keep `cmd/cricinfo/main.go` as the process boundary, but move parsing/help logic into a new CLI package.
- Establish the root UX contract:
  - root help
  - `--version`
  - global `--format`
  - global `--verbose`
  - global `--all-fields`
  - consistent unknown-command handling
- Create placeholder top-level groups for the final public nouns:
  - `matches`
  - `players`
  - `teams`
  - `leagues`
  - `seasons`
  - `standings`
  - `competitions`
  - `search`
  - `analysis`
- Add “next step” help wording so commands are obviously drillable.
- Preserve binary name, version behavior, and existing packaging assumptions.

Validation strategy:

- Extend command tests to cover root help, version, unknown commands, placeholder subcommands, and global flags.
- Run `make test`, `make build`, and npm script checks.
- Verify help output for root and at least two placeholder groups.

Risks / fallbacks:

- Risk: the CLI framework can sprawl into business logic.
- Fallback: keep command definitions thin and push all real behavior into internal packages from the start.

### Phase 2: Transport, Retry, Ref Traversal, And Pagination Foundation

Files to read before starting:

- Phase 1 command package
- `gg/agent-outputs/cricinfo-public-api-guide.md`
- `gg/agent-outputs/cricinfo-api-research-handoff.md`
- `gg/agent-outputs/cricinfo-working-templates.tsv`
- `gg/agent-outputs/cricinfo-working-endpoints.tsv`
- `gg/cricinfo-cli-foundation-research-report-2026-04-10.md`

What to do:

- Create a dedicated Cricinfo client package.
- Implement:
  - base URL configuration
  - request timeout
  - retry with backoff and jitter for `>=500`
  - request context propagation
  - stable user-agent
- Add core decode types:
  - `Ref`
  - paginated envelope
  - minimal root discovery objects
- Implement helpers for:
  - following `$ref`
  - auto-following multi-hop ref chains when a route is effectively a pointer
  - preserving both requested and canonical refs when they differ
  - dealing with routes that work even when parent objects omit or null them
- Build shape-aware decode helpers for:
  - page envelopes
  - object-shaped collections
  - single-object stats payloads

Validation strategy:

- Unit tests with `httptest` for retry, timeout, backoff, and ref-following behavior.
- Live smoke tests for:
  - `/events`
  - `/athletes/{id}`
  - `/athletes/{id}/statistics`
  - `/leagues/{id}/events/{id}/competitions/{id}/status`
  - `/leagues/{id}/events/{id}/competitions/{id}/details/{id}`
  - `/leagues/{id}/standings`
- Verify correct behavior on both success and transient `503`.

Risks / fallbacks:

- Risk: over-eager ref following can hide meaningful intermediate objects.
- Fallback: store both the raw transport object and the resolved target in internal result types so the renderer can choose the right view.

### Phase 3: Fixture Corpus And Live Validation Harness

Files to read before starting:

- Phase 2 client package
- `internal/app/app_test.go`
- `Makefile`
- `gg/agent-outputs/cricinfo-working-templates.tsv`
- `gg/agent-outputs/cricinfo-player-stat-templates.tsv`
- `gg/agent-outputs/cricinfo-field-path-frequency.tsv`

What to do:

- Create a curated fixture strategy instead of ad hoc JSON samples.
- Add a `testdata/fixtures/` tree organized by resource family:
  - root and discovery
  - matches and competitions
  - details and plays
  - team and competitor
  - innings, fow, partnerships
  - players
  - leagues, seasons, standings
  - auxiliary competition metadata
- Add refresh tooling to pull approved representative live payloads into fixtures.
- Add live integration test commands behind an explicit flag or environment variable.
- Add a curated endpoint matrix so future phases can validate only the families they touch.

Validation strategy:

- Confirm fixture refresh works end to end.
- Confirm live tests are opt-in and resilient to transient `503`.
- Add at least one fixture and one live test for each major family before later phases build on them.

Risks / fallbacks:

- Risk: live tests become flaky and block progress.
- Fallback: keep live tests scoped and retried, and let fixture tests remain the primary fast feedback loop.

### Phase 4: Rendering Contract And Normalized Result Shapes

Files to read before starting:

- Phase 1 CLI package
- Phase 2 client package
- Phase 3 fixtures
- `gg/agent-outputs/cricinfo-field-path-catalog.txt`
- `gg/agent-outputs/cricinfo-player-stat-field-paths.txt`

What to do:

- Create a rendering boundary that all later commands must use.
- Define normalized result shapes for the core entities:
  - match
  - player
  - team
  - league
  - season
  - standings group
  - innings
  - delivery event
  - stat category
  - partnership
  - fall-of-wicket
- Define extension blocks for long-tail data that should remain accessible via `--all-fields`.
- Implement:
  - default text renderer
  - JSON renderer
  - JSONL renderer where appropriate for list outputs
- Standardize empty-result behavior, partial-data behavior, and transport-error messaging.

Validation strategy:

- Snapshot or golden tests for text rendering.
- JSON schema-style assertions for normalized output.
- Fixture-based tests for long-tail field preservation through extension blocks.

Risks / fallbacks:

- Risk: over-normalization can erase upstream detail.
- Fallback: keep normalized core plus explicit extension payloads and never drop researched fields without a command-level reason.

### Phase 5: Resolver, Search, And Cache Foundation

Files to read before starting:

- Phase 2 client package
- Phase 4 normalized result types
- `gg/agent-outputs/cricinfo-working-endpoints.tsv`
- `gg/cricinfo-cli-foundation-research-report-2026-04-10.md`

What to do:

- Build the entity resolution layer that the upstream API does not provide cleanly.
- Support:
  - explicit numeric IDs
  - known refs
  - context-aware resolution from current matches and leagues
  - cached alias matching for players, teams, leagues, and matches
- Implement a cache/index store using a pure-Go approach suitable for cross-platform binaries.
- Do not scan all `106137` athletes on every query.
- Seed the resolver incrementally from:
  - live `/events`
  - leagues and seasons traversed by the user
  - roster entries from fetched matches
  - cached entities from previous commands
- Add public discovery commands:
  - `search players`
  - `search teams`
  - `search leagues`
  - `search matches`

Validation strategy:

- Unit tests for exact and fuzzy resolution.
- Live tests proving search works for at least one player, one team, one league, and one live/current match.
- Cache tests proving reused lookups avoid unnecessary transport churn.

Risks / fallbacks:

- Risk: full-universe player search is too expensive without indexing.
- Fallback: start with incremental cache-backed search and add scoped hydration later for season-scale analysis.

### Phase 6: Match Discovery, Summary, And Status Commands

Files to read before starting:

- Phase 4 result types and renderers
- Phase 5 resolver package
- `gg/agent-outputs/cricinfo-public-api-guide.md`
- `gg/agent-outputs/cricinfo-working-templates.tsv`

What to do:

- Implement the first real user-facing match commands:
  - `matches live`
  - `matches list`
  - `matches show <match>`
  - `matches status <match>`
- Make `matches live` and `matches list` work well from `/events`.
- Normalize the event and competition layers into a single public “match” concept.
- Include useful default fields:
  - teams
  - match state
  - date
  - venue summary when present
  - score summary
  - IDs for drill-down use
- Help text for each command should point to next logical commands such as `matches scorecard` and `matches innings`.

Validation strategy:

- Live tests for current-event traversal from `/events`.
- Fixture tests for event and competition decoding.
- Command tests for text and JSON output.

Risks / fallbacks:

- Risk: event IDs, competition IDs, and league-linked paths are easy to conflate.
- Fallback: keep an internal match context object that always carries the resolved league, event, and competition identifiers together.

### Phase 7: Match Scorecard, Details, Plays, And Situation

Files to read before starting:

- Phase 6 match commands
- Phase 3 match fixtures
- `gg/agent-outputs/cricinfo-working-templates.tsv`
- `gg/agent-outputs/cricinfo-field-path-catalog.txt`

What to do:

- Implement match drill-down commands for the high-value match surfaces:
  - `matches scorecard <match>`
  - `matches details <match>`
  - `matches plays <match>`
  - `matches situation <match>`
- Render matchcards as clean batting, bowling, and partnerships views rather than raw card payloads.
- Normalize detail records into delivery-event objects with:
  - batsman and bowler refs
  - score value
  - dismissal information
  - over context
  - timestamps
  - coordinates and speed when available
- Ensure `matches plays` works even when the parent competition object reports `plays: null`.
- Treat sparse `situation` payloads as valid empty data, not command failure.

Validation strategy:

- Live tests for detail, plays, matchcards, and situation routes.
- Fixture tests that assert batting card, bowling card, partnerships card, and delivery-event rendering.
- Confirm JSON output preserves detail-level fields such as `dismissal`, `playType`, `bbbTimestamp`, and coordinates.

Risks / fallbacks:

- Risk: detail payload richness varies by match.
- Fallback: standardize an output shape with nullable advanced fields rather than hiding the command when a match has sparse detail.

### Phase 8: Team And Competitor Surfaces

Files to read before starting:

- Phase 5 resolver package
- Phase 6 and Phase 7 match context types
- `gg/agent-outputs/cricinfo-working-templates.tsv`
- `gg/agent-outputs/cricinfo-field-path-catalog.txt`

What to do:

- Implement team-centric commands with both global and match-scoped behavior:
  - `teams show <team>`
  - `teams roster <team>`
  - `teams roster <team> --match <match>`
  - `teams scores <team> --match <match>`
  - `teams leaders <team> --match <match>`
  - `teams statistics <team> --match <match>`
  - `teams records <team> --match <match>`
- Normalize the difference between:
  - global team resources
  - competition competitor resources
- Use roster entries to bridge team commands into player commands cleanly.
- Present team leaders in text views as readable batting and bowling leaderboards.

Validation strategy:

- Live tests for roster, leaders, statistics, records, and scores.
- Fixture tests for object-shaped roster payloads and category-based leader payloads.
- Command tests for both team-ID input and resolved team-name input.

Risks / fallbacks:

- Risk: team commands can become ambiguous between global team data and match-scoped competitor data.
- Fallback: make match scope explicit in help and JSON output and surface context labels in the text renderer.

### Phase 9: Innings, Over, Partnership, And Fall-Of-Wicket Surfaces

Files to read before starting:

- Phase 7 detail-event models
- Phase 8 team and competitor commands
- `gg/agent-outputs/cricinfo-working-templates.tsv`
- `gg/agent-outputs/cricinfo-field-path-catalog.txt`

What to do:

- Implement the innings-depth commands:
  - `matches innings <match>`
  - `matches innings <match> --team <team>`
  - `matches partnerships <match> --team <team> --innings <n> --period <n>`
  - `matches fow <match> --team <team> --innings <n> --period <n>`
  - `matches deliveries <match> --team <team> --innings <n> --period <n>`
- Normalize linescore and period resources into:
  - innings summary
  - over timeline
  - wicket timeline
  - partnership objects
  - fall-of-wicket objects
- Surface over-split wicket data from innings statistics so agents can answer innings-flow questions without scraping raw nested arrays.
- Include direct links from wicket timeline entries back to delivery details where available.

Validation strategy:

- Live tests for linescores, period statistics, partnerships, and fow.
- Fixture tests for over-by-over wicket splits and partnership payloads.
- Command tests for innings selectors and error handling when a requested innings or period is missing.

Risks / fallbacks:

- Risk: innings and period identifiers are not intuitive for users.
- Fallback: default to the current or first sensible period when possible and always show available innings and periods in help or error hints.

### Phase 10: Player Discovery, Profile, News, And Global Statistics

Files to read before starting:

- Phase 5 resolver package
- Phase 4 normalized player types
- `gg/agent-outputs/cricinfo-player-stat-templates.tsv`
- `gg/agent-outputs/cricinfo-player-top-keys-frequency.tsv`
- `gg/agent-outputs/cricinfo-player-stat-field-paths.txt`

What to do:

- Implement global player commands:
  - `players search <query>`
  - `players profile <player>`
  - `players news <player>`
  - `players stats <player>`
  - `players career <player>`
- Keep profile and statistics as separate resource families in both code and output.
- Normalize player profile fields such as:
  - identity
  - names
  - position
  - styles
  - team
  - major teams
  - debuts
  - related news
- Normalize player statistics into category-and-stat structures rather than flattening away the upstream grouping.

Validation strategy:

- Live tests for `athletes/{id}`, `athletes/{id}/news`, and `athletes/{id}/statistics`.
- Fixture tests for profile and stats payloads with category arrays.
- Command tests for name-based resolution and stable JSON output.

Risks / fallbacks:

- Risk: general athlete statistics can be sparse or partially empty for some players.
- Fallback: preserve categories and stat names even when display values are empty so the schema remains stable.

### Phase 11: Player Match-Context, Dismissals, Deliveries, And Live Cricket Detail

Files to read before starting:

- Phase 7 match-detail commands
- Phase 8 team/roster commands
- Phase 9 innings models
- `gg/agent-outputs/cricinfo-player-stat-templates.tsv`
- `gg/agent-outputs/cricinfo-field-path-catalog.txt`

What to do:

- Implement the player-in-match commands that matter most for agent reasoning:
  - `players match-stats <player> --match <match>`
  - `players innings <player> --match <match>`
  - `players dismissals <player> --match <match>`
  - `players deliveries <player> --match <match>`
  - `players bowling <player> --match <match>`
  - `players batting <player> --match <match>`
- Pull from both:
  - roster-player match statistics
  - detail-event and innings-split data
- Ensure the normalized model exposes fields such as:
  - `dismissalName`
  - `dismissalCard`
  - `ballsFaced`
  - `strikeRate`
  - `dots`
  - `economyRate`
  - `maidens`
  - `foursConceded`
  - `sixesConceded`
  - `wides`
  - `noballs`
  - `bowlerPlayerId`
  - `fielderPlayerId`
- When detail coordinates exist, expose them as a player delivery or shot-map view in normalized form.
- Make dismissal and wicket views explicit first-class outputs rather than hidden long-tail fields.

Validation strategy:

- Live tests for roster-player statistics and roster-player linescore resources.
- Fixture tests for batting and bowling stat category extraction.
- Fixture or live tests proving dismissal and delivery views preserve detail refs and dismissal metadata.

Risks / fallbacks:

- Risk: coordinate and speed data may be present only for some matches or some deliveries.
- Fallback: keep coordinate-aware views optional but stable and clearly indicate when no coordinates are available for the selected scope.

### Phase 12: League, Calendar, Season, Type, Group, And Standings Navigation

Files to read before starting:

- Phase 5 resolver package
- Phase 6 match discovery commands
- `gg/agent-outputs/cricinfo-working-templates.tsv`
- `gg/cricinfo-cli-foundation-research-report-2026-04-10.md`

What to do:

- Implement league and season navigation commands:
  - `leagues list`
  - `leagues show <league>`
  - `leagues events <league>`
  - `leagues calendar <league>`
  - `leagues athletes <league>`
  - `leagues standings <league>`
  - `leagues seasons <league>`
  - `seasons show <league> --season <season>`
  - `seasons types <league> --season <season>`
  - `seasons groups <league> --season <season> --type <type>`
  - `standings show <league>`
- Hide the upstream multi-hop traversal behind a single command flow.
- Normalize calendar day payloads even though they are section-shaped rather than item-shaped.
- Normalize standing resources even though some routes only return refs and require additional follow steps.
- Support league-athlete views when they provide more context than global athlete views.

Validation strategy:

- Live tests for leagues, calendar, seasons, types, groups, and standings.
- Fixture tests for section-shaped calendar routes and ref-only standings chains.
- Command tests for league-name resolution and season/type/group selection.

Risks / fallbacks:

- Risk: calendar and standings resources are more pointer-heavy and less self-describing than match routes.
- Fallback: centralize multi-hop traversal in one navigator package so individual commands stay simple.

### Phase 13: Competition Metadata And Long-Tail Coverage Gap Closure

Files to read before starting:

- `gg/agent-outputs/cricinfo-working-templates.tsv`
- `gg/agent-outputs/cricinfo-field-path-catalog.txt`
- All earlier phase command packages and normalized result types

What to do:

- Implement the remaining competition metadata surfaces:
  - `competitions show <match>`
  - `competitions officials <match>`
  - `competitions broadcasts <match>`
  - `competitions tickets <match>`
  - `competitions odds <match>`
  - `competitions metadata <match>`
- Handle empty-but-valid collections as clean zero-result views rather than warnings.
- Add a coverage ledger that maps researched endpoint templates to public commands and result views.
- Use the coverage ledger to close any gaps between:
  - working templates
  - known field-path families
  - public command coverage
- Ensure no validated family is left reachable only through internal code.

Validation strategy:

- Live tests for officials, broadcasts, tickets, odds, and metadata aggregation.
- Coverage test that asserts every researched template maps to a command family or documented subview.
- Manual command verification for empty collection cases.

Risks / fallbacks:

- Risk: some auxiliary routes are frequently empty, making them easy to neglect.
- Fallback: treat “empty but supported” as a first-class success state in both tests and renderers.

### Phase 14: Historical Scope Traversal And Real-Time Hydration

Files to read before starting:

- Phase 5 resolver packages
- Phase 12 league and season navigation commands
- Phase 13 coverage ledger
- `gg/agent-outputs/cricinfo-working-templates.tsv`

What to do:

- Build the traversal and hydration layer needed for cross-match and cross-season reasoning.
- Support scoped collection of matches by:
  - league
  - season
  - season type
  - group
  - date range
- Hydrate match, innings, player-match, and delivery summaries on demand in real time.
- Reuse normalized data only within a single command execution when it avoids duplicate fetches, but do not add a persistent cache, local store, or warm-cache workflow.
- Keep hydration domain-driven, not endpoint-driven.
- Make sure later analytics commands can reuse the in-process hydrated data for the active run without re-fetching the same scoped resources repeatedly.
- Do not add a public warm-cache or persistence command in this phase.

Validation strategy:

- Integration tests for traversing a season or group into concrete match sets.
- Live tests for reusing in-process hydrated data within a repeated scoped analysis flow in the same run.
- Performance checks on a limited multi-match sample to ensure the approach is practical.

Risks / fallbacks:

- Risk: historical hydration can grow too slow or too broad if it tries to scan the whole universe.
- Fallback: keep hydration scoped by explicit league and season boundaries and use only narrowly scoped in-process memoization during a single command execution.

### Phase 15: Analysis Commands For Agent Reasoning

Files to read before starting:

- Phase 9 innings commands
- Phase 10 player global commands
- Phase 11 player match-context commands
- Phase 14 hydration layer
- `gg/agent-outputs/cricinfo-field-path-catalog.txt`

What to do:

- Add a dedicated `analysis` command family for derived cricket reasoning over normalized CLI data.
- Initial analysis commands should cover the user’s concrete question types:
  - `analysis dismissals --league <league> --seasons <range>`
  - `analysis bowling --metric economy --scope <match-or-season>`
  - `analysis bowling --metric dots --scope <match-or-season>`
  - `analysis bowling --metric sixes-conceded --scope <match-or-season>`
- `analysis batting --metric fours|sixes|strike-rate --scope <match-or-season>`
- `analysis partnerships --scope <match-or-season>`
- Make the analysis layer reusable so agents can combine CLI output with their own reasoning instead of relying on one-off ad hoc scripts.
- Keep analysis results real-time by deriving them from scoped live traversal and in-process hydration, not from persistent cached datasets.
- Ensure JSON output is stable enough for agents to sort, filter, and compare results safely.
- Add ranking, grouping, and filter semantics that fit cricket usage:
  - by player
  - by team
  - by league
  - by season
  - by dismissal type
  - by innings

Validation strategy:

- Fixture tests on normalized match data for deterministic ranking and grouping.
- Live tests on a small curated historical scope.
- Manual spot checks against known match outputs for economy, dot balls, dismissal counts, and sixes conceded.

Risks / fallbacks:

- Risk: deriving historical answers directly from live traversal can be too slow for repeated agent use.
- Fallback: keep traversal tightly scoped, reuse normalized data only within the current command execution, and surface scope metadata in the output when useful.

### Phase 16: Packaging, Release, Docs, And Final Acceptance

Files to read before starting:

- `package.json`
- `bin/cricinfo.js`
- `scripts/postinstall.js`
- `Makefile`
- `.github/workflows/release.yml`
- All command help output from earlier phases
- Coverage ledger from Phase 13

What to do:

- Fix release-version injection so GitHub release binaries report the tag version instead of `dev`.
- Resolve the CLI-name fallback mismatch between `bin/cricinfo.js` and `scripts/postinstall.js`.
- Add missing make targets for live tests, fixture refresh, and acceptance checks.
- Update README and command help examples to reflect the real product.
- Run an end-to-end acceptance pass covering:
  - install path
  - root help discoverability
  - match exploration
  - player exploration
  - team exploration
  - league and season traversal
  - analysis commands
  - JSON output
- Confirm that the coverage ledger now accounts for every validated endpoint family and that no public API family is only accessible through undocumented internal behavior.

Validation strategy:

- Run `make check`, build binaries, and npm install smoke checks.
- Run live acceptance commands from a clean environment.
- Manually verify the released help text is sufficient for an agent to drill from `cricinfo --help` into match, player, league, and analysis workflows.

Risks / fallbacks:

- Risk: packaging or release fixes get deferred because they are not feature work.
- Fallback: treat distribution hardening as part of the product definition, not post-launch cleanup.
