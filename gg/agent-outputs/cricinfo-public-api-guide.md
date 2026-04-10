# Cricinfo Public API Guide (Working Endpoints, Expanded Crawl)

Last verified: 2026-04-10 (Asia/Kolkata)
Base URL: `http://core.espnuk.org/v2/sports/cricket`

This guide contains only endpoints that returned `200` in live probe runs.

## Verification Dataset (current)

Consolidated from targeted probes + recursive `$ref` crawling:
- Working URL samples: `526`
- Normalized working endpoint templates: `56`
- Unique scalar field paths (data-point catalog): `2536`

Generated artifacts:
- [cricinfo-working-endpoints.tsv](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-working-endpoints.tsv)
- [cricinfo-working-templates.tsv](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-working-templates.tsv)
- [cricinfo-field-path-catalog.txt](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-field-path-catalog.txt)

## Probe Context IDs (examples used)

- `leagueId = 19138`
- `eventId = 1529474`
- `competitionId = 1529474`
- `teamId = 789643`
- `playerId = 1361257`
- `detailId = 110`

Additional crawled coverage also included league/event trees like `leagueId=1098952` and `eventId=1475396`, which revealed more player/roster/detail routes.

## 1) Core Response Patterns

## 1.1 Reference object
```json
{ "$ref": "http://core.espnuk.org/v2/sports/cricket/..." }
```

## 1.2 Paginated envelope
```json
{
  "count": 123,
  "items": [ ... ],
  "pageCount": 7,
  "pageIndex": 1,
  "pageSize": 20
}
```

## 1.3 Common high-value key groups
- Match/competition: `status`, `details`, `matchcards`, `officials`, `tickets`, `broadcasts`, `situation`, `odds`
- Ball-by-ball detail item: `text`, `over`, `period`, `batsman`, `bowler`, `scoreValue`, `speedKPH`, `xCoordinate`, `yCoordinate`, `dismissal`
- Athlete profile: `displayName`, `battingName`, `style`, `majorTeams`, `debuts`, `news`
- Stats payloads: typically include `splits` (often object-shaped)

## 2) Working Endpoint Families (Normalized)

Most important templates found working:

## 2.1 Discovery and roots
- `/`
- `/events`
- `/events/{id}`
- `/events/{id}/teams/{id}`
- `/leagues`
- `/leagues/{id}`
- `/teams/{id}`
- `/athletes`
- `/athletes/{id}`
- `/athletes/{id}/news`
- `/athletes/{id}/statistics`

## 2.2 League/event/competition tree
- `/leagues/{id}/events`
- `/leagues/{id}/events/{id}`
- `/leagues/{id}/events/{id}/competitions/{id}`
- `/leagues/{id}/events/{id}/competitions/{id}/status`
- `/leagues/{id}/events/{id}/competitions/{id}/details`
- `/leagues/{id}/events/{id}/competitions/{id}/details/{id}`
- `/leagues/{id}/events/{id}/competitions/{id}/plays`
- `/leagues/{id}/events/{id}/competitions/{id}/matchcards`
- `/leagues/{id}/events/{id}/competitions/{id}/officials`
- `/leagues/{id}/events/{id}/competitions/{id}/broadcasts`
- `/leagues/{id}/events/{id}/competitions/{id}/tickets`
- `/leagues/{id}/events/{id}/competitions/{id}/odds`
- `/leagues/{id}/events/{id}/competitions/{id}/situation`
- `/leagues/{id}/events/{id}/competitions/{id}/situation/odds`

## 2.3 Competitor/team match resources
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/scores`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/leaders`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/statistics`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/records`

## 2.4 Innings/period granular resources
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/leaders`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/statistics/{n}`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/fow`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/fow/{n}`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/partnerships`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/linescores/{n}/{n}/partnerships/{n}`

## 2.5 Player-in-match stat routes
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/linescores`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/linescores/{n}/{n}/statistics/{n}`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/statistics/{n}`

## 2.6 League-level athlete and season resources
- `/leagues/{id}/athletes/{id}`
- `/leagues/{id}/athletes/{n}` (observed `.../athletes/0` style index endpoint)
- `/leagues/{id}/calendar`
- `/leagues/{id}/calendar/ondays`
- `/leagues/{id}/calendar/offdays`
- `/leagues/{id}/standings`
- `/leagues/{id}/seasons`
- `/leagues/{id}/seasons/{id}`
- `/leagues/{id}/seasons/{id}/types`
- `/leagues/{id}/seasons/{id}/types/{n}`
- `/leagues/{id}/seasons/{id}/types/{n}/groups`

For usage frequency and sample URLs per template, see [cricinfo-working-templates.tsv](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-working-templates.tsv).

## 3) Data-Point Coverage

The field catalog currently includes `2536` unique scalar JSON paths from real `200` responses.

Examples:
- `categories.0.leaders.0.athlete.$ref`
- `batsman.runs`
- `bowler.wickets`
- `dismissal`
- `xCoordinate`, `yCoordinate`
- `runRate`
- `wicketOver`
- `styles.0.$ref`
- `majorTeams.$ref`

Full list: [cricinfo-field-path-catalog.txt](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-field-path-catalog.txt)

## 3.1 Player Statistics Deep Map (Focused Pass)

Dedicated player-stat exploration artifacts:
- [cricinfo-player-stat-endpoints.tsv](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-player-stat-endpoints.tsv)
- [cricinfo-player-stat-templates.tsv](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-player-stat-templates.tsv)
- [cricinfo-player-top-keys-frequency.tsv](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-player-top-keys-frequency.tsv)
- [cricinfo-player-item-keys-frequency.tsv](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-player-item-keys-frequency.tsv)
- [cricinfo-player-stat-field-paths.txt](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-player-stat-field-paths.txt)
- [cricinfo-player-stat-field-frequency.tsv](/Users/ashray/code/amxv/cricinfo-cli/gg/agent-outputs/cricinfo-player-stat-field-frequency.tsv)

Focused-pass snapshot:
- Player endpoint samples: `171`
- Player endpoint status mix: `167` x `200`, `1` x `404`, `3` x `503`
- Player-specific normalized templates: `8`

Top player endpoint templates observed:
- `/athletes/{id}`
- `/athletes/{id}/statistics`
- `/athletes/{id}/news`
- `/leagues/{id}/athletes/{id}`
- `/leagues/{id}/events/{id}/competitions/{id}/competitors/{id}/roster/{id}/statistics/{n}`

Top recurring player profile/stat keys observed:
- Identity/profile: `id`, `uid`, `guid`, `displayName`, `fullName`, `shortName`, `gender`, `age`
- Cricket metadata: `battingName`, `fieldingName`, `style`, `styles`, `position`
- Team/career links: `team`, `majorTeams`, `debuts`, `relations`, `news`
- Stats envelope: `$ref`, `athlete`, `splits`

Important nuance:
- `athletes/{id}` is rich profile data; `athletes/{id}/statistics` is condensed and split-oriented.
- `roster/{playerId}/statistics/{index}` is a high-value match-context stat route for CLI player-match commands.

## 4) Practical Go CLI Design Guidance

## 4.1 Architecture
- Primary client should be hypermedia-following (`$ref` first-class), not hardcoded route-only.
- Keep endpoint wrappers for common commands, but always retain a generic `GET <ref>` path follower.

## 4.2 Type strategy
- Stable structs:
  - `Ref`, `Page[T]`, `Athlete`, `Team`, `CompetitionStatus`
- Semi-stable structs:
  - `DetailItem`, `Linescore`, `Partnership`, `FoW`
- Flexible parsing:
  - Keep `splits` and some category payloads as `json.RawMessage` initially.

## 4.3 Suggested command surface
- `matches list` -> `/events`
- `matches status` -> `/.../status`
- `matches plays` -> `/.../plays`
- `matches details` -> `/.../details` and `/.../details/{detailId}`
- `matches scorecard` -> `/.../matchcards`
- `teams roster` -> `/.../competitors/{teamId}/roster`
- `teams innings` -> `/.../linescores/{inningsId}/{periodId}`
- `players profile` -> `/athletes/{playerId}`
- `players news` -> `/athletes/{playerId}/news`
- `players stats-live` -> `/.../roster/{playerId}/statistics/0` and/or `/.../linescores/.../statistics/0`

## 4.4 Reliability
- Expect occasional transient 5xx on some resources.
- Use retry with backoff + jitter for `>=500`.
- Set per-request timeout (5-10s) and lightweight caching for polled routes.

## 5) Next Expansion (to approach “every data point”)

1. Multi-league crawl across more active leagues from `/leagues` (not just one chain).
2. Completed-match sampling vs live-match sampling for schema deltas.
3. Generate per-template JSON schema snapshots (automated) from multiple sample URLs.
4. Build compatibility tests for your Go structs against saved fixture responses.
