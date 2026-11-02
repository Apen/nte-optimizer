# Remaining engineering work

Updated 2026-11-02 after implementation of the code-quality roadmap. Completed
refactorings and validation are summarized in [engineering notes](docs/engineering-notes.md).
The original evidence and CQ identifiers remain available in Git commit
`b973993`. This file keeps unresolved work rather than a completed-task log.

CQ-011 is implemented: runtime panels now require roster NetID and declared
actor/subobject ownership. Tests and private scan verification are recorded in
[engineering notes](docs/engineering-notes.md). The items below require measurement
or an external test environment before further changes are justified.

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
