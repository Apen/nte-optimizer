# Remaining engineering work

Updated 2026-11-02 after implementation of the code-quality roadmap. Completed
refactorings and validation are summarized in [engineering notes](docs/engineering-notes.md).
The original evidence and CQ identifiers remain available in Git commit
`b973993`. This file keeps unresolved work rather than a completed-task log.

## CQ-011 — Verify runtime panel ownership

- [ ] Establish the actor/channel-to-roster ownership contract before describing
  a runtime panel association as identity-proven.
  - **Priority / category:** P1; protocol correctness investigation, partially
    implemented. Equal-HP and conflicting-panel cases are already rejected.
  - **Evidence:** `cmd/nte-scan/panel_association.go` now rejects multiple roster
    characters with the same HP and inconsistent panels. The remaining unique-HP
    path still has no verified actor GUID to roster `netId` (slot/serial) mapping.
  - **Missing contract:** a demonstrated link between the replicated attribute
    block's owner and the imported character identity. Neither HP equality nor a
    loaded blueprint establishes that link. No verified field offset or catalog
    record currently supplies it; inventing one would make observations less safe.
  - **Approach:** obtain a sanitized ownership fixture with two distinct owners,
    including equal-HP characters, and carry the proven owner through decoding.
    Keep collision rejection and unavailable-panel reconstruction until then.
    Non-deployed characters' missing final panels must still be recalculated;
    searching for another RPC is not a substitute for ownership evidence.
  - **Effort / risk / confidence:** M–L / high / high that ownership evidence is
    missing; implementation effort depends on the verified protocol structure.
  - **Prerequisites:** evidence for the actor/channel/roster link, not a guessed
    Unreal handle or character-specific mapping.
  - **Acceptance:** repeated or equal-HP entities cannot acquire another owner's
    panel; unassigned panels remain unavailable; known owners bind deterministically.
  - **Verification:** extend `panel_association_test.go` with a sanitized owner
    fixture and run `go test ./cmd/nte-scan ./internal/scanner/...` and the full
    Go suite. Any private capture verification remains opt-in and outside Git.

## Measured follow-up before further changes

These are investigations, not evidence that another rewrite is required.

- **Solver preparation:** profile representative active-substat workloads before
  sharing projections or replacing source-weighted maps. Keep fractional source
  order, identity alternatives, strict feasibility, and worker-local scratch.
  Fixed-stat search with reduction still uses about 0.96 MB/op and 0.53–0.66 ms
  in exploratory samples versus about 6.01 MB/op and 7.53–8.26 ms without it.
- **Large inventories in WebView2:** measure card rendering, filtering, and owner
  lookups on a large synthetic inventory before adding virtualization or another
  dependency. No visual/interaction benchmark was performed in this pass.
- **Historical build freshness:** saved results remain historical by design.
  Introduce data/account revision metadata only if actual use shows confusion;
  do not silently rewrite saved results during unrelated cleanup.
- **Legacy stat corrections:** reconstruction now applies recommendation
  `current_stats` explicitly and rejects contradictory corrections across a
  character's strategies. Resolve their provenance upstream before moving or
  removing them. Do not replace them with hardcoded Go corrections.
- **Race verification:** local CGO is disabled and no supported C compiler is
  installed. Run a meaningful race suite in a provisioned environment before
  making stronger concurrency claims. Cancellation and stale-response/log
  regressions already run in the normal suites.

## Verification boundaries

The implementation did not change ranking or damage formulas, upload user data,
perform an elevated capture, launch gameplay automation, or publish a release.
Unit tests, synthetic fuzzing/benchmarks, frontend build, source quality checks,
and Windows desktop/scanner compilation are recorded in the engineering notes.
Those checks do not prove live protocol ownership or every conditional combat
rule. Preserve the existing passive scanner and independent package boundaries.
