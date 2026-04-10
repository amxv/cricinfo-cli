# Curated Fixture Strategy

This fixture tree is organized by major Cricinfo API resource families so future phases can validate only the families they touch.

Families:

- `root-discovery`
- `matches-competitions`
- `details-plays`
- `team-competitor`
- `innings-fow-partnerships`
- `players`
- `leagues-seasons-standings`
- `aux-competition-metadata`

Sources:

- Endpoint inventory: `internal/cricinfo/fixture_matrix.go`
- Generated matrix file: `internal/cricinfo/testdata/fixtures/endpoint-matrix.tsv`

Refresh command:

```bash
go run ./internal/cricinfo/cmd/fixture-refresh --write
```

Refresh only selected families:

```bash
go run ./internal/cricinfo/cmd/fixture-refresh --write --families players,details-plays
```

Live integration test entrypoint:

```bash
CRICINFO_LIVE_MATRIX=1 go test ./internal/cricinfo -run TestLive -count=1
```

Limit live test families with:

```bash
CRICINFO_LIVE_MATRIX=1 CRICINFO_LIVE_FAMILIES=players,team-competitor go test ./internal/cricinfo -run TestLiveFixtureMatrixByFamily -count=1
```
