# Damage catalog validation

The existing Damage tab remains the build-comparison interface. The Buffs tab
summarizes teammate contributions. There is no separate skill-sheet tab or manual
scenario editor. Detailed dataminer inputs also support automated verification.

## Published snapshot

The synchronized snapshot contains these numerical catalogs under `data/game/combat`:

| Catalog | Validation role |
| --- | --- |
| `skill_values.json` | 29 character IDs, 162 skill definitions and 788 sheet lines; independent coefficient/scaling cross-checks |
| `skill_buffs.json` | 143 main definitions and two provisional attribute transfers; units, unlocks, recipients and exclusion contracts |
| `modifier_values.json` | 454 supplemental records; these are not automatically active buffs |
| `reaction_values.json` | Six damage scenarios, two Zero recovery scenarios and summon evidence; selected arithmetic remains separate from standard damage totals |
| `damage.json` | 766 effects, preview exclusions, action splits, critical behavior and explicit hit counts |
| `action_evidence.json` | 14 compact action records and unresolved effect identities |

The snapshot also updates `team_effect_previews.json`, both game locales and the
manifest. All 22 catalog hashes match the supplied export. No runtime dataminer
dependency, localized-text formula parsing or account-specific calibration is added.

## What is verified

The roster audit calls the same damage batch that supplies the Damage tab, using
synthetic levels 1, 8 and 13 and distinct ATK/HP/DEF values. It verifies:

- Correct coefficient selection through the published parent/level bindings.
- Correct ATK, HP or DEF scaling and fixed critical rates.
- Explicit repeated-hit totals without duplicate effect identities.
- Preservation of action splits, conditional variants and preview exclusions.
- Agreement between 638 sheet damage parameters and their damage-catalog arrays
  and scaling. The explicitly display-only Akane coefficient is kept outside
  executable damage effects.

762 effects have a published binding in this audit. Four constant-coefficient
entries have no parent ability or imported-level binding and remain outside the
standard damage output:

| Character ID | Effect ID |
| --- | --- |
| 1004 | `GE_Player_Lacrimosa_AnHunZhouTwo_Damage` |
| 1008 | `GE_Player_Skia_GuangXiangFive_Damage` |
| 1025 | `GE_Player_Hathor_GuangXiangFive_Damage` |
| 1054 | `GE_Player_Daffodill_ExtraUnbalance_Damage` |

A constant array establishes a numerical coefficient, not automatic activation,
an action grouping or extra hits. The audit does not invent those relationships.
Existing Shinku/Blackbird hit-count, Zero exclusion, Linko variant and Zankou
regressions continue to check the standard action totals.

The support-contract tests also verify recipients, unlocks, exact units, selected
stacks/counters, replacement groups, Zero actor priority, wrapper zeros and mutually
exclusive Chaos branches. These calculations do not automatically activate effects
or alter the Damage tab, ranking, saved builds or imported account data. Summon
pool limits and projectile counts are diagnostic evidence, not guaranteed hits.

## Remaining source uncertainties

Catalog agreement and passing engine tests verify numerical plumbing. They do not
independently certify extracted values against the game. The dataminer's 64
maintained level assumptions from 1.4, plus six new assumed 1.5 mappings, remain
explicit.
Native lifecycle, mixed-source stacking, automatic timing and final healing
execution are not certified by these tests.

The Akane review identified changed curves and a sheet-only row:

1. The supplied damage snapshot changes **11 Akane coefficient arrays** compared
   with the previous consumer snapshot, despite the handoff's preservation claim.
   The new sheet curves agree with the new damage arrays. The consumer keeps the
   supplied generated data unchanged; that agreement does not validate the change
   against the game. Request the source revision and justification for these rows:

   | Effect ID | Previous level 1 | Supplied level 1 |
   | --- | ---: | ---: |
   | `GE_Player_Akane_Melee1_Damage` | 0.276 | 0.149 |
   | `GE_Player_Akane_Melee2_Damage` | 0.405 | 0.219 |
   | `GE_Player_Akane_Melee3_1_Damage` | 0.214 | 0.135 |
   | `GE_Player_Akane_Melee3_Damage` | 0.165 | 0.103 |
   | `GE_Player_Akane_Melee4_Damage` | 3.814 | 4.247 |
   | `GE_Player_Akane_PerfectEvadeAttack_Damage` | 0.25 | 1.375 |
   | `GE_Player_Akane_Skill2_Damage` | 2.25 | 1.62 |
   | `GE_Player_Akane_Skill_Damage` | 5 | 3.6 |
   | `GE_Player_Akane_UltraSkill1_Damage` | 0.9 | 0.648 |
   | `GE_Player_Akane_UltraSkill2_Damage` | 12 | 9.18 |
   | `GE_Player_Akane_UpgradeSkill_Damage` | 11.25 | 10.125 |

2. The publisher now classifies `GE_Player_Akane_UpgradeSkill_DisplayGE` as
   `display_damage_coefficient`, with
   `damage_calculation.status: display_only_no_confirmed_execution_binding`,
   `supported: false` and a null executable effect ID. Its coefficient remains
   available for display/validation; no alias or additional hit is invented.
   The typed contract preserves its reason and source references.

   The executable `GE_Player_Akane_UpgradeSkill_Damage` keeps its base curve
   separate from the conditional `CoefAdd` records in `modifier_values.json`.
   The composed display coefficient is not substituted for that base, and no
   modifier is added to the display row. Focused regressions pin all 13 display
   and base values, check the selected short/long-state modifier branches and
   verify that the Damage output never activates these additions automatically.

The French locale also updates 69 pre-existing values. All existing English labels
are preserved, and neither language loses existing keys. These translations are
display data, not formula inputs.

## Version 1.5 boundaries

Exe (1074) and Elyms (1090) have imported identities, exact base stats, console
traits, skill labels, Arc compatibility, grids and portraits. Their 18 and 35
own damage effects remain distinct from sheet-reference availability. The six
new level projections cover 15 effects and retain `assumed` status, source
fingerprints and `native_execution_certified: false`. Damage diagnostics expose
the selected level source and review; they do not certify native propagation.

Exe's raw sheet has 45 damage parameters referencing Shinku effects. Numeric
values remain available, but `Parameter.ResolveDamageEffect` rejects every
foreign-character binding. The coverage audit pins the complete set by sheet
line, parameter index and source ID; it still fails on new unmatched associations
or coefficient/scaling disagreements. No alias, extra hit or cross-character
damage substitution is introduced.

Four added passive projections retain manual conditions, recipient scope and
breakthrough unlocks: Elyms's caster CRIT DMG and per-stack team Cosmos DMG,
Exe's caster damage stacks triggered by allied summons, and Exe's clone-only
DEF ignore. The last value is deliberately zero. Exe's caster damage bonus is
not restricted to summon damage merely because summons trigger it. Its infinite
duration is not replaced by the serialized ten-second magnitude. Per-contributor
counts and native mixed-source accumulation remain unverified.

`ForkCatalog.ResolveConditionalEffect` resolves modeled Arc effects at the exact
rank and checks recipients, damage filters and stack limits. The builder projects
confirmed panel-wide modifiers into the ranking and buffed preview. It applies
hit-specific modifiers to matching damage instances. For Echoes on Hold, the
builder removes the old wearer DEF-ignore contribution from `conditional_by_star`
before applying its confirmed value.
[Arc effects in a combat scenario](data-and-localization.md#arc-effects-in-a-combat-scenario)

Linko's modifier consumes the explicit `linko_eligible_other_qte_count` native-query
result, not a count inferred from living teammates. Its arithmetic retains the
three-player baseline. The reaction extension's extracted `ExtraDamageRatio`
is 0.10 and remains diagnostic rather than automatically active.

`data/recommendations/targets/elyms.json` supplies the Lost Radiance and Speedy
Hedgehog search profiles, six recommended Arcs and their published stat targets.
Integration tests exercise both profiles with every recommended Arc against
synthetic inventory, including imported levels and the Damage output.

The export still does not include `data/recommendations/targets/exe.json` while
its source recommendations are unavailable. Imported character views and stat
calculation work independently of a strategy profile. Exe build searches still
require authored recommendations; weights, preferred sets and execution/variant
assumptions are not fabricated from sheet values.

## Compact teammate summaries

The Buffs tab lists the selected teammates' team-recipient effects from
`skill_buffs.json`: source, flat/percentage value, condition and duration. Caster-only
effects and enemy debuffs do not appear in this team-buff summary. Published
awakening and passive unlock thresholds are checked against the imported teammate.
Amounts use exact skill levels and the ally's resolved Arc/base stats. Missing
inputs remain unavailable rather than becoming zero. Re-run optimization to refresh
older saved results that do not contain this summary.

These rows are informational. They never append stats or change damage/ranking.
An effect is marked "In buffed preview" only when an existing skill projection
matches its source character, ability, property, units and amount. The existing
row is then hidden in this view to avoid displaying the contribution twice. Other
effects are marked "Not included"; they do not become automatically active.
Projection status is stored separately for each build alternative.

Conditions use localized presentation labels for the published semantic conditions.
`conditions.description_key` is a source-evidence pointer, not a guaranteed label
in the published game locales. When only a native predicate/source pointer exists,
the summary says that the passive's specific conditions apply. It does not invent a
trigger or infer an awakening threshold from an asset name.

## Verification

Run `go test ./...` for catalog parsing, manifest hashes, all referenced EN/FR
labels, exact level/character/scenario axes, support-contract arithmetic and the
existing damage/scanner/equipment regressions. The roster audit exercises the
actual Damage output; it does not validate a second display calculation against
itself. Run the frontend suite and production build after interface changes.
