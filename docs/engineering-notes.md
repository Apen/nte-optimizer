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

## Calculation and lifecycle hardening (2026-11-02)

The code-quality follow-up implemented the actionable corrections from the
November audit as separate scoped commits:

- reserved Arc identities are filtered before selecting a copy;
- unresolved imported stat tiers and target data fail explicitly;
- detailed builds are validated and saved in one atomic replacement;
- bonus overviews use imported selection without reconstructing the roster;
- character views retain source-agnostic residuals and no longer compute unused
  cross-character furniture suggestions;
- one operation reads one account snapshot; immutable service catalogs are
  reused until a new service is created for a new installed data generation;
- character reconstruction does not require a ranking strategy for that character;
- damage batches share catalog preparation and the equipped baseline;
- workspace responses contain build summaries, with details loaded separately;
- frontend saves retain independent profile drafts and ignore stale responses;
- search progress and diagnostic completion are scoped to their request;
- explicit Wails declarations use shared named DTOs and a sanitized JSON contract;
- import resolves geometry, quality, set, and level limits from published data;
- localized client error codes are separate from technical causes;
- decoder budgets, cancellation checks, fuzz seeds, and a read-only CI formatting
  gate protect the updated boundaries.

A verified runtime actor-to-roster ownership contract remains missing. Panel
association rejects equal-HP roster collisions and conflicting observations;
unique-HP association is supporting evidence rather than proof of ownership.
See [remaining work](../TODO.md) for the exact missing contract.

### Measured checks for this pass

Checks ran on Windows amd64 using Go 1.27.3; the module retains its Go 1.25
language baseline:

- full Go tests, vet, formatting, and clean module metadata passed;
- frontend tests passed: 17 files, 53 tests, plus TypeScript/Vite production build;
- Staticcheck v0.8.2 passed, and Govulncheck v1.8.0 reported no known vulnerabilities;
- scanner and desktop production executables compiled without launching them;
- five-second synthetic fuzz runs passed for PCAP, bounded LZ4, and sparse panels.

Staticcheck v0.8.1 could not read this compiler's export data. The pinned tool
was updated to v0.8.2, whose [upstream release notes](https://github.com/dominikh/go-tools/releases)
document compatibility with the newer Go export format. An initial global tool
cache permission failure was resolved for the verification run. No application
runtime dependency was changed for that tooling update.

Exploratory benchmarks use three single-iteration samples, not a stable latency
promise. The informational bonus page now allocates about 0.59 MB for twelve
cataloged synthetic characters, versus about 29.4 MB during the original audit.
Current samples took about 0.91–5.07 ms/op; filesystem/cache startup contributes
to the variation. The permanent benchmark sorts cataloged character IDs and
contains no private account data. Damage reporting batches allocate about
3.7 MB for one result and 5.3 MB for one hundred results; catalogs are no longer
parsed independently for each alternative. Allocation volume is not peak RSS.

No package publication, extracted-release smoke launch, private-capture check,
manual WebView2 visual review, or race run was performed in this pass. Race
verification still requires a supported CGO/compiler environment.

## Validation baseline

The current local release gate verifies:

- all Go tests;
- frontend Vitest tests;
- TypeScript and Vite production build;
- `go vet` and clean module metadata;
- `staticcheck v0.8.2`;
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
