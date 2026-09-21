# Architecture

NTE Optimizer separates privileged packet capture, account decoding, local
state, optimization, and presentation so each responsibility can be tested and
changed independently.

## Runtime components

```text
NTE login traffic
       │
       ▼
nte-scan.exe (elevated only while scanning)
       │  validated export v3
       ▼
account_snapshot.json (atomic local snapshot)
       │
       ├──────────────► inventory and character pages
       │
static game data ─────┤
recommendations ──────┤
local preferences ────┤
       │               ▼
       └────────► OptimizerService ─► placement solver ─► React results
```

The desktop executable embeds the compiled React application with `go:embed`.
Wails exposes a small Go facade to TypeScript; production does not require a
separate web server.

## Repository layout

```text
assets/                    game images used by the interface
cmd/nte-scan/              scanner CLI and domain decoders
data/game/                 generated facts extracted from the game
data/optimizer/            optimizer-owned normalization and configuration
data/presentation/         application interface text
data/recommendations/      character build recommendations
docs/                      public and contributor documentation
frontend/                  React and TypeScript application
internal/app/              application services and local persistence
internal/decoded/          scanner export contracts and import normalization
internal/optimizer/        grid and search engine
internal/scanner/          PCAP, Unreal protocol, and transactional publisher
internal/scannerlauncher/  UAC boundary and scanner lifecycle
scripts/                   quality, build, packaging, and smoke tests
```

## Scanner pipeline

1. `pktmon` records a bounded login capture.
2. ETL is converted to PCAPNG after capture stops.
3. the scanner selects relevant flows and reassembles TCP and UDP payloads;
4. Unreal frames, bunches, exports, RPCs, and Hotta containers are decoded;
5. domain decoders produce characters, equipment, Arcs, resources, and module
   placement;
6. relations and JSON contracts are validated in a staging directory;
7. the complete export is published transactionally, with `manifest.json`
   written last;
8. the desktop importer creates one atomic account snapshot.

An incomplete scan never replaces the last valid account import.

## Desktop service boundary

The Wails facade exposes operations for:

- profiles, recommendations, and editable strategy settings;
- build workspace, character priority, and saved builds;
- localized inventory and character state;
- guided scan status, launch, and cancellation;
- optimization launch, progress, cancellation, and results.

Only one optimization runs at a time. Starting another cancels the previous
search. Application shutdown cancels both active optimization and active scan.

## Persistence

Persistent user state lives under `%LOCALAPPDATA%\NTE Optimizer\workspace`.
Writes that replace account or configuration state use atomic temporary-file
replacement. Multi-file scanner exports use neighboring staging and backup
directories so the previous generation can be restored after a publication
failure.

The application still reads the complete legacy `inventory.json` plus
`decoded_state.json` pair for migration. New successful imports use
`account_snapshot.json`.

## Optimizer packages

- `internal/scoring` computes normalized stat contributions;
- `internal/optimizer` owns geometry, placement, candidate reduction, set
  evaluation, exact search, approximate search, and stat aggregation;
- `internal/target` loads recommendation targets and compares results;
- `internal/damage` evaluates supported direct, reaction, and damage-over-time
  formulas;
- `internal/app` composes catalogs, account context, search requests, persistence,
  and presentation-ready result contracts.

Intermediate search structures remain private to their packages. Interfaces are
kept only where the implementation is genuinely substituted in production or
tests.
