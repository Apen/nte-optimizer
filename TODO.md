# Code quality roadmap

Audit date: 2026-11-02. Baseline: `5bf7032` plus the existing, uncommitted
character-panel documentation clarification. This roadmap records findings;
it does not change application behavior.

## Summary and scope

The Go/Wails/React application has useful boundaries between passive capture,
decoding, import, persistence, scoring, placement search, damage, and display.
Atomic account imports, explicit approximate-search reporting, deterministic
synthetic search fixtures, and reference-versus-optimized evaluator tests are
worth preserving. The recent level-zero rule is centralized in
`nte.Module.ActiveSubStats` and reaches the important search/stat consumers.

The highest-value next work is correctness and removal of superseded paths,
followed by eliminating repeated preparation outside the solver. A general
architecture rewrite or replacement of the placement engine is not justified.

Reviewed: repository instructions, README, architecture/development/optimization
notes, manifests, CI/build scripts, Wails operations and locking, account
observations and bonuses, import/persistence, character reconstruction, Arc and
module candidate preparation, search orchestration, representative blueprint,
projection/cache/bound logic, damage/reporting, locale loading, React state,
inventory and character pages, bridge declarations, and relevant regression
tests. The scanner's capture/reassembly/decoder paths and damage formulas were
sampled; this was not a proof of every protocol field or combat formula.

No private account, screenshot, or packet data was used in audit probes. Four
behavioral defects were reproduced through temporary Go test overlays against
synthetic fixtures, without modifying tracked application source. The probes
were removed after the audit; proposed permanent regressions are listed below.

Classification: **defect** means an observed violation of the current contract;
**debt** means a justified design or maintenance improvement; **investigation**
means the impact still needs measurement or runtime reproduction.

Priority: **P1** correctness or meaningful user-state risk; **P2** worthwhile
performance, robustness, or maintainability work; **P3** cleanup after higher
priorities. No demonstrated critical-impact P0 issue was found within this
scope. Effort S/M/L is relative implementation effort, not urgency.

## Priority overview

| ID | Action | Class | Priority | Effort | Regression risk | Confidence | Prerequisites |
| --- | --- | --- | --- | --- | --- | --- | --- |
| CQ-001 | Respect reserved Arc identities | Defect | P1 | S | Medium | High, reproduced | None |
| CQ-002 | Report missing calculation inputs explicitly | Defect | P1 | M | Medium | High, base-tier probe | None |
| CQ-003 | Restore meaningful active-substat test fixtures | Defect in tests | P1 | S | Low | High, fixture inspection | None |
| CQ-004 | Save a detailed build with one atomic replacement | Defect | P1 | S–M | Medium | High, reproduced | None |
| CQ-005 | Make the account-bonuses page informational end to end | Defect + debt | P1 | M | Medium | High, probe + benchmark | None |
| CQ-006 | Reuse one immutable calculation context per operation | Debt | P2 | M | Medium | High, repeated call paths | CQ-002, CQ-005 |
| CQ-007 | Prepare damage/reporting inputs once per result batch | Debt | P2 | M | Medium | High, repeated I/O | CQ-003 |
| CQ-008 | Return build summaries instead of complete saved results | Debt | P2 | S–M | Low–Medium | High, serialized DTO | None |
| CQ-009 | Isolate frontend profile drafts and asynchronous operations | Defect path + debt | P1 | M–L | High | High for timer loss; medium for races | Characterization first |
| CQ-010 | Enforce typed Go/TypeScript boundary contracts | Debt | P2 | M | Medium | High, bridge declarations | Coordinate with CQ-008 |
| CQ-011 | Associate runtime panels with character identity | Defect risk | P1 | M–L | High | High for ambiguity; live incidence unknown | Synthetic collision regression |
| CQ-012 | Bound and cancel decoding; add targeted fuzz tests | Debt | P2 | M–L | Medium | High for missing guards | Preserve current valid fixtures |
| CQ-013 | Remove duplicate game-definition tables from application code | Debt | P2 | M | Medium | High, duplicate catalogs | Verify data contract first |
| CQ-014 | Separate localized user errors from technical diagnostics | Defect + debt | P2 | M | Low–Medium | High, visible error path | Coordinate with CQ-010 |
| CQ-015 | Reconcile obsolete labels and completeness metadata | Debt | P3 | S | Low | High, consumer inspection | CQ-005 |

## Small, bounded corrections

- [x] **CQ-001 — Respect reserved Arc identities during copy selection.**
  - **Priority / category:** P1; confirmed correctness defect.
  - **Evidence:** `internal/app/arc_candidates.go:71` selects the strongest
    matching owned copy without consulting `OptimizerService.ReservedArcIDs`.
    `internal/app/build_store.go:240` computes those reservations, and the
    desktop/CLI populate them. Modules and cartridges already filter theirs.
  - **Scenario / impact:** a lower-priority character receives an Arc reserved
    for a higher-priority character, even when an unreserved copy is available.
    A synthetic probe with reserved `4:2` and free `4:1` selected `4:2`.
  - **Approach / tradeoffs:** filter reserved identities before the existing
    strongest-copy comparison. Preserve explicit reuse when reservations are
    intentionally omitted; define the no-available-copy outcome consistently
    with explicit versus implicit Arc selection. Do not invent a second ranking.
  - **Effort / risk / confidence:** S / medium / high; directly reproduced.
  - **Prerequisites:** none.
  - **Acceptance:** free copies remain eligible; reserved copies do not; all
    copies reserved produces the documented unavailable result; reuse mode still
    selects the strongest owned copy. Cover current-equipment and local-build
    reservations through the public optimizer path, not only a helper.
  - **Verification:** focused tests in `arc_candidates_test.go` and
    `build_store_test.go`, followed by `go test ./...`.

- [x] **CQ-003 — Repair fixtures whose implicit level zero now disables their test data.**
  - **Priority / category:** P1; confirmed test-quality regression.
  - **Evidence:** `internal/optimizer/benchmark_test.go:16` creates substats
    without `Level`; `prepared_evaluator_test.go` reuses that fixture.
    `internal/optimizer/exact_test.go:25` likewise generates randomized CRIT
    substats on implicit level-zero modules. `ActiveSubStats` correctly ignores
    them, so comparisons still pass while losing module-stat variation.
  - **Impact / practice:** passing optimized-versus-reference comparisons no
    longer exercise the intended fractional sums, caps, and module objectives.
    The fixed-stat fixture was updated to level 20 and remains useful.
  - **Approach / tradeoffs:** explicitly mark intended active modules, retain
    intentional level-zero cases, and assert that fixture substitutions actually
    change effective stats and feasibility. Include mixed active/inactive cases.
    Do not mechanically change every zero to 20.
  - **Effort / risk / confidence:** S / low / high; verified fixture semantics.
  - **Prerequisites:** none; complete before sensitive solver refactoring.
  - **Acceptance:** reference/compiled/cache/bound/oracle tests cover nonzero
    fractional module contributions, maximum-only goals, strict floors, and
    identity ties. Benchmarks document whether their modules are active.
  - **Verification:** `go test ./internal/optimizer ./internal/scoring`, then
    `go test ./...`; rerun affected benchmarks after fixture correction.

- [x] **CQ-004 — Persist complete build results with a single atomic write.**
  - **Priority / category:** P1; confirmed persistence defect.
  - **Evidence:** `internal/app/build_store.go:28` calls `SaveLocalBuild`, which
    writes `local_builds.json`, then adds `Result` and writes the same file again.
    The second failure occurs after the first replacement has already committed.
  - **Scenario / impact:** disk/write or detailed-result serialization failure
    returns an error while leaving a changed, summary-only saved build. An audit
    probe supplied a non-serializable solution score with serializable derived
    stats; saving failed but a new build without `Result` remained on disk.
  - **Approach / tradeoffs:** extract an in-memory validation/update function;
    let each public save operation perform one final `writeJSON`. Keep the
    existing atomic-file implementation and on-disk schema.
  - **Effort / risk / confidence:** S–M / medium / high; directly reproduced.
  - **Prerequisites:** none.
  - **Acceptance:** serialization or replacement failure preserves the previous
    complete build; success saves summary and result together. The existing
    summary-only operation still works intentionally. Add both no-previous-build
    and replacement-failure regressions.
  - **Verification:** `go test ./internal/app ./internal/atomicfile`, then
    `go test ./...`; use deterministic fault injection rather than filling a disk.

## Focused module refactoring and performance

- [x] **CQ-002 — Fail explicitly when imported progression or objective data cannot be resolved.**
  - **Priority / category:** P1; confirmed missing-input handling defect.
  - **Evidence:** `internal/app/project_data.go:113` returns `ok=false` for a
    missing base-stat tier. `optimizer_service.go:300` and
    `character_state.go:224` then retain recommendation `BaseStats` silently.
    Separately, `optimizer_service.go:311` discards `Target` errors, allowing
    objective preparation to continue without the intended target.
  - **Scenario / impact:** an incomplete catalog or invalid imported progression
    produces plausible but incorrect stats instead of identifying the missing
    contract. A synthetic unavailable-tier probe returned nonzero recommendation
    base stats without an error. This is not evidence of a currently missing
    valid tier in the published catalog.
  - **Approach / tradeoffs:** distinguish imported-character reconstruction from
    intentional account-free profile evaluation. Require the exact imported
    character/level/breakthrough record, validate progression at import, and
    propagate target failures where objectives require a target. Name the exact
    file and key; route user presentation through CQ-014. Do not add fallback maps.
  - **Effort / risk / confidence:** M / medium / high; base fallback reproduced,
    discarded target error verified in callers.
  - **Prerequisites:** none.
  - **Acceptance:** a missing required tier or target cannot silently change the
    calculation mode or base values; account-free APIs retain explicitly tested
    behavior. Character reconstruction must not require an arbitrary strategy's
    default base stats merely because that strategy is first in a map.
  - **Verification:** add public-path tests with a copied catalog missing one
    required tier and a target-load failure; run `go test ./...`.

- [x] **CQ-005 — Remove unused reconstruction and inference from the informational bonus page.**
  - **Priority / category:** P1; reproduced failure plus measured unnecessary work.
  - **Evidence:** `internal/app/account_buffs.go:230` loads full equipment,
    computes comparisons/sources for every character, and calls
    `inferHousingEffects`. `frontend/src/pages/account-buffs-page.tsx` consumes
    furniture selection and bond display fields, not comparisons, suggestions,
    calibration flags, or source breakdowns. The desktop holds `buildMu` during
    this complete operation (`wails_app.go:131`).
  - **Scenario / impact:** a per-character calibration conflict is initially
    recorded as an entry flag, but final housing inference recalculates the same
    loaded character and returns that conflict as a page-wide error. The
    informational list becomes unavailable. This was reproduced synthetically.
  - **Approach / tradeoffs:** reduce this endpoint to catalog resolution, imported
    selection, and localized display. Keep the same backend eligibility rules
    used in calculations. Retain useful panel diagnostics behind an explicit
    diagnostic operation; remove a field only after updating all consumers/tests.
    Keep character-state diagnostics that still have actual UI consumers.
  - **Effort / risk / confidence:** M / medium / high; reproduced and benchmarked.
  - **Prerequisites:** none.
  - **Acceptance:** the page remains readable when one character's calculation
    has a calibration conflict; optimization still rejects overlapping bonuses.
    Reading the page performs no full stat reconstruction or equipment catalog
    loading. The displayed active bonuses still equal calculation eligibility.
  - **Verification:** extend `account_buffs_test.go` with a loaded-panel conflict;
    update `account-buffs-page.test.tsx` and bridge types; compare synthetic
    one-/twelve-character allocations against the audit baseline below.

- [ ] **CQ-006 — Prepare shared calculation inputs once per operation.**
  - **Priority / category:** P2; demonstrated repeated work and state ownership debt.
  - **Evidence:** `character_state.go:208` constructs a new service on every
    reconstruction, discarding service catalog/base caches. `loadCharacters`
    rereads every target; `resolveAccountContext` reloads locale and Arc catalogs;
    `scannedAccountBuffSources` reloads and indexes furniture/bond catalogs on
    every character. Housing inference repeats reconstruction. Character-state
    loading also reloads the account through `LoadEquipmentCatalog`.
  - **Impact / practice:** work scales with characters times catalogs rather than
    one preparation plus character-specific sums. Repeated snapshot reads also
    make operation consistency harder to reason about.
  - **Approach / tradeoffs:** use an operation-owned, typed context containing one
    account snapshot, equipment-by-owner indexes, validated immutable catalogs,
    and selected account bonuses. Separate character facts from recommendation
    ranking inputs. Make display localization return copies rather than mutate
    imported character/weapon pointers in `resolveAccountContext` and
    `prepareArcSelection`. First reuse within a call; add service-lifetime caches
    only with an explicit data-directory/version lifecycle and invalidation.
  - **Effort / risk / confidence:** M / medium / high for repeated I/O; actual
    user-visible latency improvement must be measured after CQ-005.
  - **Prerequisites:** CQ-002, CQ-005; characterize source totals before extraction.
  - **Acceptance:** one logical operation uses one account generation and loads
    each shared catalog at most once; locale changes cannot mutate cached account
    facts; no global mutable cache or scanner-to-optimizer coupling is introduced.
  - **Verification:** compare derived stats and source attribution across
    equipped/new/saved/experiment paths; benchmark character-state reconstruction
    with realistic synthetic inventory sizes; run `go test ./...`.

- [ ] **CQ-007 — Compile damage/reporting context once for all retained alternatives.**
  - **Priority / category:** P2; confirmed repeated I/O, unmeasured end-to-end cost.
  - **Evidence:** `optimizer_service.go:440` calls `localizedDamageAnalysis` per
    alternative. `damage_analysis.go:134` rereads the entire damage catalog;
    `damage_analysis.go:16` loads presentation on each call, and
    `internal/locale/catalog.go:161` also loads Arc effect parameters. The current
    damage and Arc JSON files are approximately 567 KB and 850 KB respectively.
    The service retains up to 100 solutions (`optimizer_search.go:13`).
  - **Impact / practice:** catalog parsing, label lookup, and current-build damage
    repeat for each reported result even though those inputs are invariant.
    These are repeated bytes parsed, not measured peak memory or latency claims.
  - **Approach / tradeoffs:** prepare catalog, labels, character skill/rule context,
    and current-build damage once; evaluate only candidate-dependent stats per
    alternative. Reuse existing phase timings and establish a reporting benchmark
    before considering lazy UI details or changing the number of retained builds.
  - **Effort / risk / confidence:** M / medium / high for repeated preparation;
    speedup unknown until measured on representative characters.
  - **Prerequisites:** CQ-003; preserve conditional/current damage semantics.
  - **Acceptance:** a batch of N alternatives does not load catalogs N times;
    group order, rounding, current/build comparisons, and missing-input handling
    remain equivalent. Catalog errors remain observable rather than becoming
    indistinguishable silent fallbacks.
  - **Verification:** add 1/10/100-result reporting benchmarks and equivalence
    tests; use Go CPU/allocation profiles; `go test ./internal/app ./internal/damage`
    and full Go tests. See [Go diagnostics](https://go.dev/doc/diagnostics).

- [x] **CQ-008 — Stop serializing full saved optimization trees in workspace summaries.**
  - **Priority / category:** P2; confirmed payload and coupling debt.
  - **Evidence:** `build_store.go:25` embeds `Result` in `LocalBuild`;
    `WorkspaceCharacter.Build` at line 83 exposes that persistence type directly,
    and `LoadBuildWorkspace` copies it into the response. The frontend's
    `SavedBuild` type does not consume `result`; `SavedBuildResult` loads details
    separately. Every workspace refresh can therefore serialize all saved
    alternatives and damage details for every character.
  - **Approach / tradeoffs:** introduce a small workspace build-summary DTO;
    keep detailed persistence and the explicit detail endpoint. Do not migrate
    historical snapshots or remove details from disk merely to reduce UI payload.
  - **Effort / risk / confidence:** S–M / low–medium / high; traceable serialization.
  - **Prerequisites:** none; coordinate bridge updates with CQ-010.
  - **Acceptance:** workspace responses contain IDs, summary stats, and timestamps
    only; fetching a saved result still returns the full original details. Measure
    serialized size for multiple synthetic saved builds with alternatives.
  - **Verification:** contract tests assert the workspace JSON has no nested
    detailed result; saved-result round-trip tests and frontend tests/build pass.

## Frontend contracts and lifecycle

- [ ] **CQ-009 — Isolate profile drafts, pending saves, and result request identity.**
  - **Priority / category:** P1; concrete loss path plus asynchronous lifecycle debt.
  - **Evidence:** `frontend/src/App.tsx:128` uses one debounce timer for all
    profiles. Profile changes do not flush that pending draft. Another edit,
    reset, or Arc change clears the same timer. The target effect depends on the
    entire `profiles` collection at line 174; saving any profile can reinitialize
    the current form. Optimization and saved-result handlers set results without
    checking the currently selected character/request at line 204 and below.
  - **Scenario / impact:** edit A, switch to B within 450 ms, then edit B: the
    second edit cancels A's only pending save. Separately, a delayed save/target
    response can reset newer draft edits; a result can arrive after selection
    changes. These paths have no full App integration regression tests today.
  - **Approach / tradeoffs:** first add fake-timer/deferred-promise tests; then
    extract a profile-keyed draft/persistence hook and a search lifecycle hook.
    Capture a complete immutable save snapshot, explicitly flush or retain drafts
    on profile changes, and gate late responses by request/selection identity.
    Split the builds view out of App after behavior is protected. Avoid a global
    state library unless this smaller ownership change proves insufficient.
  - **Effort / risk / confidence:** M–L / high / high for timer loss by control
    flow; remaining timing races require frontend reproduction.
  - **Prerequisites:** characterization tests before moving state.
  - **Acceptance:** A/B rapid edits both persist; older acknowledgments never
    overwrite newer drafts; selected-character changes cannot display an
    unrelated late result; stop/save/loading/error states remain accessible.
    Define and test draft handling on app unmount instead of dropping a timer.
  - **Verification:** add App or extracted-hook integration tests with fake timers
    and reversed response ordering, then `npm test -- --run` and `npm run build`.
    Follow [React effect cleanup guidance](https://react.dev/learn/synchronizing-with-effects)
    when cancelling or ignoring obsolete asynchronous work.

- [ ] **CQ-010 — Type the complete bridge instead of casting `Promise<any>` results.**
  - **Priority / category:** P2; confirmed cross-layer contract debt.
  - **Evidence:** `frontend/wailsjs/go/main/DesktopApp.d.ts:3` and most following
    methods return `any`; App supplies assertions from separately maintained
    `frontend/src/types.ts`. The settings declaration omits fields already sent
    by the UI, including custom goal metadata and explicit strict minima. The
    newest AccountBuffs binding already imports its application type explicitly.
  - **Approach / tradeoffs:** choose one authoritative DTO typing workflow. If
    normal Wails generation is still unavailable under the documented toolchain,
    keep the explicit bridge but use shared named types and add Go JSON contract
    fixtures consumed by TypeScript tests. Do not add a second generation system
    solely to conceal the existing generator limitation.
  - **Effort / risk / confidence:** M / medium / high; declarations inspected.
  - **Prerequisites:** coordinate with CQ-005/CQ-008 DTO changes.
  - **Acceptance:** public calls return named result types without call-site
    assertions; request fields match Go; activation flags, optional/null lists,
    strict floors, and saved-result fields have contract tests. Bridge limitations
    are documented accurately and regeneration does not silently discard fixes.
  - **Verification:** TypeScript production build, frontend contract tests,
    Go DTO serialization tests, and `go test ./...`.

- [ ] **CQ-014 — Return stable error codes and localize user-facing failures.**
  - **Priority / category:** P2; confirmed presentation contract violation.
  - **Evidence:** `frontend/src/pages/character-state-page.tsx:23` renders
    `String(value)` from Go directly. `internal/app/json.go:23` wraps file paths
    into errors, so this can also expose unnecessary local path details. The
    import page instead classifies failures by English substrings; the launcher
    already demonstrates a typed diagnostic-code boundary.
  - **Approach / tradeoffs:** reuse a small typed error classification for
    missing data, calibration conflicts, unavailable equipment, and import
    failures. Preserve wrapped causes for technical diagnostics; map codes to
    both supported locale catalogs at the display boundary. Keep detailed
    developer diagnostics in the dedicated expandable diagnostic view.
  - **Effort / risk / confidence:** M / low–medium / high; direct visible path.
  - **Prerequisites:** coordinate with CQ-010; no broad logging framework needed.
  - **Acceptance:** the character page shows localized errors with alert semantics;
    scan behavior does not depend on matching English prose; missing-contract
    errors still identify the relevant game-data file/key in diagnostics.
  - **Verification:** EN/FR failure-path tests plus launcher/error-code tests,
    frontend tests/build, and Go tests.

## Scanner identity and data boundaries

- [ ] **CQ-011 — Associate decoded panels with an entity, not just equal HP.**
  - **Priority / category:** P1; confirmed identity ambiguity, production incidence unverified.
  - **Evidence:** `cmd/nte-scan/domain_decode.go:106` scans all completed messages
    for each character and accepts the first panel matching that character's
    `MaxHP`. `characters.go:78` receives only bytes, bit length, and HP; it cannot
    distinguish two entities with equal HP. The message is not consumed after a
    match and there is no owner/actor equality guard.
  - **Scenario / impact:** two characters with the same saved HP can receive the
    same first valid panel even if their CRIT/ATK differ. More scans cannot make
    HP into a unique identifier. Sparse-field validation protects panel structure,
    not ownership. No claim is made that a private capture exhibited this case.
  - **Approach / tradeoffs:** add a two-owner/same-HP regression, then carry
    verified owner/actor association from replication into panel extraction.
    If ownership cannot be established, preserve the panel as unassigned or
    omit the association explicitly. Do not guess ownership from catalog stats,
    invent protocol handles, or expect non-deployed final panels to exist.
  - **Effort / risk / confidence:** M–L / high / high for ambiguity; the exact
    ownership contract still needs protocol evidence.
  - **Prerequisites:** synthetic identity-collision test; characterize current
    successful sparse and omitted-field cases.
  - **Acceptance:** equal-HP entities cannot share a panel by first-match accident;
    incomplete ownership produces unavailable evidence, not an authoritative
    observed panel. Missing final panels still use application reconstruction.
  - **Verification:** synthetic owner-collision and repeated-message tests,
    `go test ./cmd/nte-scan ./internal/scanner/...`, followed by opt-in private
    capture validation outside source control when available.

- [ ] **CQ-012 — Apply resource budgets and cancellation through the decoder boundary.**
  - **Priority / category:** P2; robustness debt with concrete reachable gaps.
  - **Evidence:** furniture uses `DecodeLZ4BlockLimited`, but
    `cmd/nte-scan/main.go:222` still uses the unlimited decoder; partial bunches
    accumulate in `udp_messages.go:50` without a byte budget. Domain decoding
    receives no context, and main checks cancellation only before/after the
    complete stage. Panel association repeatedly bit-scans every message for each
    character; named-property extraction realigns every message eight times.
    PCAP parsing reads the entire file and copies packet payloads. No `FuzzXxx`
    tests exist in the inspected scanner packages. Import validation checks
    furniture bounds, but module validation does not validate equipment levels.
  - **Impact / practice:** large or malformed local inputs can cause avoidable
    allocation growth and delayed cancellation. This is a local resource-risk
    finding, not a demonstrated remote execution vulnerability.
  - **Approach / tradeoffs:** establish protocol/workload-derived caps for capture,
    decompression, partial reassembly, and record counts; propagate context through
    costly loops; validate progression at import even if the scanner already
    validates it. Parse reusable message metadata once where measurement supports
    it. Add fuzz targets for PCAP/LZ4, sparse panels, and named properties using
    synthetic seeds. Avoid arbitrary caps that reject normal login captures.
  - **Effort / risk / confidence:** M–L / medium / high for absent guards;
    realistic limits and hotspot gains remain to be measured.
  - **Prerequisites:** retain valid decode fixtures and agree on overflow outcomes.
  - **Acceptance:** bounded failure for oversized/unterminated inputs, no panics
    for malformed seeds, cancellation during long decode loops, invalid equipment
    levels rejected before persistence, and no replacement of a valid snapshot
    by a rejected import.
  - **Verification:** Go regression tests; proposed bounded fuzz runs such as
    `go test ./internal/scanner/unreal -fuzz=FuzzDecodeLZ4 -fuzztime=30s` after
    creating that target; benchmark bytes/allocations on synthetic captures.
    Follow [Go fuzzing guidance](https://go.dev/doc/security/fuzz/).

- [ ] **CQ-013 — Make imported equipment definitions follow the published data contract.**
  - **Priority / category:** P2; confirmed duplicated game facts.
  - **Evidence:** `internal/decoded/importer.go:60` maintains module item/geometry
    definitions, `internal/decoded/inventory.go:9` maintains supported geometries,
    and `internal/nte/module.go:8` maintains cartridge-prefix/set mappings. The
    scanner already consumes generated equipment definitions, and shapes/sets
    exist under `data/game`. An added game geometry can decode successfully yet
    be rejected downstream until these Go tables are edited.
  - **Approach / tradeoffs:** first compare every current mapping to the generated
    catalogs and identify exact missing contract fields. Pass typed validated
    definitions to import/normalization rather than having domain models perform
    hidden file I/O. Preserve the documented legacy normalization behavior with
    explicit migration tests. Request missing generated fields upstream instead
    of creating another fallback mapping.
  - **Effort / risk / confidence:** M / medium / high; tables and consumers verified.
  - **Prerequisites:** establish the minimum current data contract before deleting
    any table; no schema change without coordinated consumers/tests.
  - **Acceptance:** equipment/geometry support changes are driven by published
    catalogs; unknown records identify their missing file/key; supported legacy
    inventories normalize deterministically without translated-name heuristics.
  - **Verification:** production catalog contract tests and importer migration
    tests, including a synthetic new definition; full Go tests.

## Cleanup after behavior is protected

- [ ] **CQ-015 — Reconcile obsolete confirmation vocabulary and source metadata.**
  - **Priority / category:** P3; confirmed stale presentation and metadata debt.
  - **Evidence:** `data/presentation/en.json:154` and the French equivalent still
    contain the retired manual-confirmation workflow, while the informational
    page uses `buff_information_help`. Stat-source labels at line 103 still call
    automatically assumed bonuses "Confirmed". `internal/optimizer/stats.go:25`
    always lists account-specific affinity/bonuses as missing and only removes
    the weapon marker, even when account sources are supplied.
  - **Approach / tradeoffs:** verify dynamic and static consumers before removing
    obsolete locale keys/DTO fields. Describe scan-derived bonuses as assumed
    active, preserving the explicit policy documented in `docs/optimization.md`.
    Derive missing-source metadata from known inputs while retaining genuinely
    unmodeled awakening/passive/account effects; do not mark every calculation
    complete merely because furniture and bond sources exist.
  - **Effort / risk / confidence:** S / low / high; callers inspected.
  - **Prerequisites:** CQ-005 establishes which diagnostic fields remain useful.
  - **Acceptance:** no manual-save instructions remain in the active bonus flow;
    EN/FR source labels agree with policy; missing-source lists neither falsely
    omit unknown effects nor claim a known applied source is wholly absent.
  - **Verification:** locale parity/consumer checks, stat metadata tests, frontend
    tests/build, and full Go tests.

## Investigations and deliberately deferred work

- **Solver preparation allocations:** `prepared_evaluator.go:31` constructs a
  projection for prepared evaluation; reduction/cache/bounds also construct
  related state. Source-weighted cache lookup allocates maps per lookup
  (`stat_cache.go:56`), and each prepared cache has 1,024 entries. Profile a
  repaired, representative workload before sharing projections or replacing
  maps with arrays. Preserve source ordering, fractional float semantics,
  identity alternatives, strict constraints, and worker-local scratch ownership.
  The current fixed-stat benchmark demonstrates a substantial benefit from the
  existing reduction/cache; removing them for superficial simplicity is unjustified.
- **Frontend rendering:** inventory pages render every visible card and perform
  repeated owner lookups. Measure large synthetic inventories in WebView2 before
  adding virtualization, memoization, or a new dependency. Current bundle output
  alone does not establish a rendering problem.
- **Search/log identity:** desktop `runSearch` uses a search ID to guard cleanup,
  but progress emission and `finishOptimizationLog` update shared log state
  without that ID. Cancelling an old run does not wait for it to finish. Add an
  overlapping-run regression before deciding whether old progress/completion can
  overwrite the new run; coordinate with CQ-009 request identity.
- **Historical saved results:** documentation deliberately treats them as historical
  snapshots. Consider an account/data revision marker only if stale results are
  being mistaken for current calculations. Do not silently recompute or rewrite
  them during a cleanup refactor.
- **CI/tooling:** CI runs tests/build/vet/tidy; the stronger pinned quality script
  is opt-in. Read-only formatting and a targeted React hook check could catch
  future regressions, but first protect CQ-009 with behavior tests. Windows race
  testing needs a CGO/compiler environment; the current limitation is documented.
  No dependency upgrade is proposed solely because a newer release exists.
- **Documentation consistency:** README says there is no remote API feature,
  while `updatecheck.Check` performs a GitHub release lookup for release builds.
  Clarify the wording in a future documentation pass: no account-data upload is
  implemented, but update metadata requests do exist. This audit did not change
  the update policy or perform an application update check.
- **Not recommended:** generic repository/factory/DDD layers, merging scanner
  and optimizer models into one subsystem, a new global state/cache framework,
  changing ranking/pruning heuristics during structural cleanup, or a universal
  coverage target. None is supported by this audit's evidence.

## Recommended order and verification

1. Repair test fixtures (CQ-003) and add permanent regressions for the four
   reproduced defects. The temporary audit probes are observations, not a new
   permanent test suite.
2. Correct Arc reservations, missing inputs, and atomic saves (CQ-001/002/004).
   Characterize frontend draft loss and scanner owner collisions in parallel
   only if implementation capacity permits; both affect trusted user results.
3. Simplify the bonus endpoint (CQ-005), then slim workspace responses (CQ-008).
   Update typed boundary contracts with those changes (CQ-010).
4. Share operation context and reporting preparation (CQ-006/007); compare exact
   source totals, ranking, feasibility, and allocations against the baseline.
5. Finish frontend lifecycle isolation (CQ-009), panel ownership (CQ-011), and
   bounded decoder/data-contract work (CQ-012/013) as separately reviewable tasks.
6. Reconcile error presentation and stale metadata (CQ-014/015). Profile remaining
   solver/rendering hypotheses only after the proven repeated work is removed.

Keep domain formulas unchanged during structural refactors. For larger context
changes, migrate one caller at a time behind the existing public operations,
compare old/new outputs in characterization tests, then remove the old helper.
Retain on-disk schemas unless a separate versioned migration is necessary. Each
step should be independently revertible without modifying imported user data.

### Checks executed

| Check | Result |
| --- | --- |
| `go test ./...` | Passed across all packages |
| `go vet ./...` | Passed |
| `go mod tidy -diff` | Passed with a workspace-local Go cache after the default cache denied writes; no module-file changes |
| `gofmt -l` over tracked Go files | No files reported; no write-mode formatter run |
| `npm test -- --run` in `frontend` | Passed: 12 files, 45 tests |
| `npm run build` in `frontend` | Passed: TypeScript and Vite; JS about 402.3 KB / 123.2 KB gzip |
| Four synthetic Go overlay probes | Confirmed reserved Arc selection, missing-tier fallback, page-wide conflict propagation, and partial saved-build mutation |
| Existing synthetic benchmarks | Passed; commands and observations below |
| `git diff --check` | Passed after roadmap creation |

No package release, elevated packet capture, real-account benchmark, browser
visual audit, race run, staticcheck, or vulnerability scan was performed.
CGO is disabled and no gcc/clang/staticcheck/govulncheck executable was found.
The pinned quality script can download tools; it was not run to install tools
as part of this read-only code audit. Passing tests/vet do not establish protocol
completeness, combat accuracy, or the absence of security issues.

### Performance observations

Windows amd64, Go 1.25 module, Ryzen 5 9600X, 12 logical workers reported.
Three single-iteration samples per case; these are exploratory measurements,
not statistically stable performance guarantees or real-account latency claims.

| Synthetic workload | Observed time/op | Allocated bytes/op | Interpretation |
| --- | --- | --- | --- |
| Fixed-stat search, reduction off | 7.14–8.16 ms | about 6.01 MB | Existing comparison workload |
| Fixed-stat search, reduction on | 0.56–0.87 ms | about 0.964 MB | 55 cache hits; preserve this optimization |
| Sanitized fast search | 0.29–0.52 ms | about 0.427 MB | 9 eligible / 7 selected / 35 visited |
| Sanitized beta search | 0.28–0.42 ms | about 0.448 MB | 9 eligible / 8 selected / 56 visited |
| Bonus page, 1 synthetic character | 7.42–12.96 ms | 4.29–4.38 MB | Empty equipment; no observed panel |
| Bonus page, 12 synthetic characters | 56.47–61.81 ms | 29.35–29.43 MB | About 207,000 allocations to display an informational list |

Existing benchmark commands:

```powershell
go test ./internal/optimizer -run '^$' -bench '^BenchmarkFixedStatSearch$' -benchmem -benchtime=1x -count=3
go test ./internal/app -run '^$' -bench '^BenchmarkSanitizedSearchModes$' -benchmem -benchtime=1x -count=3
```

The bonus-page benchmark was an audit-only overlay: write an empty synthetic
account with 1 or 12 distinct cataloged characters at level 80/breakthrough 6,
without panels/equipment, then measure `LoadAccountBuffPage`. Keep a permanent,
deterministic benchmark with fixed character IDs when implementing CQ-005.
Allocated bytes/op measure allocation volume, not retained heap or peak RSS.
Do not use `BenchmarkBlueprintComparison` numbers as a meaningful active-stat
baseline until CQ-003 repairs its fixture.

Only this roadmap was added by the audit. The pre-existing character-panel
documentation edit remains untouched. No application fixes, commits, pushes,
dependency installs, or user-data changes were made.
