# Engineering history and limitations

This page preserves the outcome of the repository quality and release-readiness
work without keeping a completed internal checklist at the repository root.

## Completed work

- portable Windows package containing the desktop app, scanner, and static data;
- stable user data under `%LOCALAPPDATA%\NTE Optimizer` with legacy workspace
  migration;
- transactional scanner publication and atomic account snapshots;
- scan cancellation across the Wails and UAC process boundary;
- guaranteed `pktmon stop` cleanup on success, failure, cancellation, and app
  shutdown;
- separated PCAP, Unreal protocol, domain decoding, and export packages;
- decomposed optimizer preparation, worker execution, reduction, and result
  reporting;
- Go, frontend, production-data, cancellation, and release smoke tests;
- pinned local static analysis and vulnerability checks;
- Wails v2.16 upgrade and reproducible ZIP/checksum generation.

## Validation baseline

The current local release gate verifies:

- all Go tests;
- frontend Vitest tests;
- TypeScript and Vite production build;
- `go vet` and clean module metadata;
- `staticcheck v0.8.1`;
- `govulncheck v1.8.0`;
- Windows scanner and desktop compilation;
- portable archive contents, checksum, scanner startup, and desktop startup.

The scanner package has focused tests for capture lifecycle, flow selection,
transactional output, character state, binary primitives, and inspection
helpers. Full decoder paths still require representative real captures because
their correctness depends on the live game protocol.

## Known limitations

- the guided scanner is Windows-only because it uses `pktmon`;
- protocol and data catalogs may need updates after an NTE release;
- not every conditional Arc, awakening, affinity, skill, or combat effect is
  guaranteed to be decoded or modeled;
- exact module positions depend on records present in the login capture;
- damage projections include only structured and verified rules available in
  the deployed catalog;
- Windows race tests are not currently part of the local gate because the
  development machine lacks a compatible CGO C compiler;
- the application binaries are not currently code-signed;
- the repository does not yet declare a project-wide source license or private
  security-reporting policy.

## Deliberate non-goals

Unless a concrete need appears, the project does not plan to:

- introduce a generic Clean Architecture or DDD layer;
- create repositories, factories, or interfaces around single implementations;
- replace straightforward CLI output with a logging framework;
- chase 100% test coverage;
- make the scanner cross-platform while `pktmon` remains the capture mechanism;
- automate game input, equip items, or modify an NTE account.
