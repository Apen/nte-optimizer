# Team context implementation roadmap

> Historical design notes from before the static Arc rules were implemented. See [the current follow-up](../TODO.md#team-context-feature).

Status: additive data contracts published; executable effect evidence still missing.

The current publication covers all 27 characters' canonical `esper_type` and
adds `team_effects` to every Arc. Both initial rules explicitly remain
`unsupported`; neither may contribute to statistics or ranking, even with one
source. No functional team-buff calculation or builder selection is implemented
by this roadmap update.

The application blueprints, referenced effects, parents, and condition bytecode
have since been extracted. Published `application_evidence.status` is
`extracted_not_certified`. The remaining prerequisite is a verified mapping/native
class contract for `HTGamePlayEffect`, its GameplayEffect base, modifier structs,
and related enums. The candidate mapping's game version is unknown. Repeating
the same export does not establish the missing attributes or lifecycle rules;
absent sparse properties must not be interpreted as native default values.

## Outcome and first release scope

Optimize one character's build in an explicitly selected team. The optimized
character is the active character in this first version; up to three selected
allies are off-field. Their equipment is fixed to selected imported Arc instances.
This is a planner scenario, not an observation of the current in-game team.

Apply verified passive team effects to both projected statistics and optimizer
evaluation. Start with the non-stacking, composition-based effect of The Great
Thief and the off-field ATK contribution of Blow up the Crowd, once their
machine-readable rules are published. Exclude the latter's damage-triggered
stacks. Existing personal Arc bonuses remain in their existing calculation path.

No automatic team recommendation, whole-team equipment search, reaction
simulation, skill/ultimate activation, uptime estimate, or rotation DPS is part
of this release. Projected action damage may consume the same team-adjusted
statistics; it must not be labeled rotation DPS.

## Verified data gap

`data/game/equipment/arcs.json` now has structured team-effect recipients,
conditions, stacking metadata, provenance, and support status alongside the
numeric `effects_by_star` parameters. For both `fork_BitGame` and
`fork_tuansanlang`, `target.property_id` and `target.operation` are null,
stacking/reapplication remain unknown, and the rules publish `unsupported`.
These records describe the evidence gap, not executable effects. The current
`permanent_by_star` and `conditional_by_star` entries supply no usable team
projection. Those lists represent wearer statistics, so populating them with
ally bonuses would be incorrect.

The English descriptions in `data/game/locales/en.json` identify:

| Arc | Parameter | Meaning at rank one |
| --- | --- | --- |
| `fork_BitGame` | `buff_BitGame2_AtkUp1` | 0.10 ATK for the active ally while the wearer is off-field |
| `fork_tuansanlang` | `buff_tuansanlang_Unbal` | 70 break intensity for matching-type members when at least three team members share the wearer's Esper type |

These descriptions are investigation evidence, not executable rules. Go and
React must not recognize Arc IDs or parse localized descriptions to recreate
the missing rules. Values must resolve by parameter name and exact Arc rank;
missing ranks or parameters must not use a neighboring rank or a hardcoded value.

`data/game/characters/decode.json` now publishes `esper_type` using canonical
`CHARACTER_ELEMENT_TYPE_*` identifiers from the character element data. Preserve
`CHARACTER_ELEMENT_TYPE_NATURE`; do not replace it with the localized Anima label.
The existing `character_group_type` remains an independent Arc-compatibility
contract. Damage instances in `data/game/combat/damage.json` must not replace
the published intrinsic Esper type.

## Step 1: publish the declarative data contract

The upstream-generated Arc catalog now has `team_effects` and the character
catalog has `esper_type`; the manifest was regenerated. Complete this step by
resolving the incomplete native-property decoding and certifying an executable
rule, not by changing its status based on a description. Application asset
extraction is complete; native defaults and application semantics remain
unverified. Existing fields must retain their current meaning.

Each supported team effect needs these typed facts:

- stable English effect identifier and canonical Arc identifier;
- output `target.property_id` and `target.operation`, `parameter.name_id`, and
  exact-rank lookup;
- recipient scope: active ally or team members matching the source Esper type;
- explicit inclusion/exclusion of the wearer;
- conditions, initially source off-field and minimum matching-type team count;
- matching-type count semantics, including whether the wearer is counted;
- stacking group, stacking/reapplication policy, and verified resolution for
  multiple sources with different ranks;
- provenance and an explicit supported/unsupported state with a reason.

Execution must first require a supported rule and verified target/operation.
An `unsupported` rule remains unavailable even with a single source. The
`target.described_stat` field is investigation metadata, not a property mapping.
An empty Arc `team_effects` array means no rule was authored; it is not proof
that the Arc has no ally effect.

Do not infer missing stacking behavior. Until verified, multiple qualifying
sources in an unresolved stacking group cannot produce a claimed combined bonus.
Report that effect as unavailable for that scenario. The catalog must distinguish
an unavailable rule from a verified zero effect.

Use canonical source-game Esper identifiers consistently. Do not reuse Arc
compatibility groups, derive types from names, or introduce a character-ID map.
Machine-readable conditions must not depend on presentation-language text.

The upstream task is specified in [the datamine request](team-context-datamine-request.md).

Acceptance: published data and manifest contain at least one certified executable
rule and the character type field, all
parameter references resolve for supported ranks, and missing/unsupported
conditions remain explicit. Neither Arc's wearer-only stats gain an ally bonus.

## Step 2: implement and test team evaluation in Go

Add a focused domain evaluator, separate from scanner and UI code. It consumes
an immutable catalog, selected character IDs, active-character ID, and actual
imported Arc instances with their ranks. It returns applied contributions,
source/recipient identities, and structured unresolved reasons.

Validate team size, duplicate members, imported ownership, Arc compatibility,
duplicate Arc-instance use, absent character types, and exact-rank data. Do not
silently treat missing data as zero or replace an unavailable Arc copy.

Tests must cover empty context, active/off-field changes, two versus three
matching types, wearer inclusion, non-matching recipients, mixed ranks,
duplicate/non-stacking sources, unsupported conditions, missing parameters,
and input-order independence. Use invented synthetic account identities only.

Acceptance: the evaluator produces source-backed stat contributions and explicit
unresolved effects without modifying imported observations or account bonuses.

## Step 3: integrate identical context into search and reporting

Thread a typed optional context through profile settings, Wails contracts,
optimization requests, saved builds, and alternatives. Old settings without a
context mean solo evaluation. Saved results retain their original scenario.

Integrate resolved contributions as a distinct `team` stat source before
candidate preparation and evaluation. Apply the same context to current-build
projections, candidate scores, feasibility, final ranking, alternatives, and
action-damage projections. An optimized character's own Arc can change the team
effect, so resolve that part per Arc candidate rather than freezing it once.

Keep selected allies' Arc instances unavailable to the target's equipment search.
Refresh or reject a saved context when those instances are absent from a new
account import. Profile/experiment cloning and cache identity must preserve the
team context and never share mutable scenario state across searches.

Never add team bonuses to received final-stat panels: those are already final
observations and may already include the effects. Show the modeled current build
with the same team for fair comparisons, while keeping the observed panel
separate. Personal Arc stats and account bonuses must not be counted again.

Acceptance: empty context preserves existing results, equal scenarios produce
equal rankings in all search modes, and focused regressions verify threshold
changes, no double counting, Arc-dependent effects, and saved-build behavior.

## Step 4: expose a focused builder UI

Add an expandable Team context section near build setup, using established UI
components. Select up to three imported allies and their fixed Arc instances;
the build's character is visibly the active member. Show applied effects,
recipient, activation reason, and unavailable effects. Describe this as a
projection, without claiming the scan supplied an authoritative team selection.

Display projected stats with and without the team and the contribution in the
source breakdown. Keep technical identifiers out of user-facing labels. Localize
all interface text and structured failure reasons in English and French. Carry
the context into the result details and saved-build view so scenarios remain
reviewable after the selection changes.

Acceptance: component tests cover selection, clearing, account refresh, persistence,
stale responses, and inaccessible/unsupported effects. Check keyboard operation
and practical desktop widths. No frontend stat formulas are introduced.

## Step 5: validate and document the implemented feature

Run focused domain/search/bridge tests, `go test ./...`, frontend tests and
production build, formatting, and a full application build. Verify local
calculation on an explicitly authorized account without committing its data.
Capture-based validation can corroborate individual effects; it must not be
used as per-account calibration or assumed proof of every stacking interaction.

Document supported effects, conditions, scenario persistence, and limitations.
Only mark this feature complete after the UI and optimizer consume the same
verified rules. Code and tests should be grouped into data-contract/domain,
application/search, and frontend/documentation commits when commits are requested.
