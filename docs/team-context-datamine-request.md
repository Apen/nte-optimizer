# Datamine follow-up: certify passive team-effect application

> Historical design notes from before the static Arc rules were implemented. See [the current follow-up](../TODO.md#team-context-feature).

The additive contracts have been received: all 27 characters have canonical
`esper_type`, every Arc has `team_effects`, and both initial effects are explicitly
unsupported. Their curve references and exact-rank parameter values are useful,
but null targets and unknown application behavior still block calculation.

Application extraction has now been completed and published as
`application_evidence.status: extracted_not_certified`. All five rank families,
referenced effects, parents, and relevant condition bytecode were followed. The
candidate USMAP loads, but its build/region is unknown and decoding does not
expose verified modifier attributes, operations, or stacking fields.

The next task is to resolve the mapping/native-class prerequisite, rather than
repeat the same extraction. Establish a version-matched schema for native
`/Script/HTGame.HTGamePlayEffect`, its GameplayEffect base, modifier structs, and
relevant effect/buff-event/stacking enums. Where values are inherited native
defaults, obtain supporting defaults or implementation evidence; their absence
from a sparse Blueprint default is not proof of zero or any particular policy.
The legacy Great Thief melee condition must not be used to certify the current
team-count behavior. Report a precise unresolved dependency if this evidence
cannot be obtained; do not substitute a differently named enum or guessed default.

After that prerequisite is resolved, the following extraction and verification
checklist applies. Its application-family extraction and parent traversal have
already been performed:

1. Extend the ability exporter to include the Buff/Fork application blueprints:
   - `/Game/Blueprints/Abilities/Buff/Fork/Fork_BitGame2/Buff_Fork_BitGame2_Lv<rank>`
   - `/Game/Blueprints/Abilities/Buff/Fork/Fork_Tuansanlang2/Buff_Fork_Tuansanlang2_Lv<rank>`
2. Follow their referenced gameplay effects, class defaults, and relevant
   application/removal bytecode. Resolve the asset mapping prerequisite first.
   If extraction is blocked, report the exact missing mapping or asset contract;
   keep the rules unsupported.
3. Establish the actual target attribute and modifier operation. Description
   metadata such as `attack_bonus` must not become an `AtkUp` mapping without
   application evidence. Publish `target.property_id`, `target.operation`, and
   their provenance only when proven.
4. Verify recipient selection, off-field application/removal, team-count
   evaluation, and whether the wearer is included in counts and recipients.
5. Verify equal-rank and mixed-rank duplicate sources and reapplication/removal
   behavior. Non-stacking text alone does not establish strongest-source
   selection, replacement order, or what happens when a source leaves the team.

Initial rule IDs and parameter references remain:

| Effect ID | Arc | Parameter |
| --- | --- | --- |
| `bitgame_off_field_active_attack` | `fork_BitGame` | `buff_BitGame2_AtkUp1` |
| `great_thief_same_esper_break_intensity` | `fork_tuansanlang` | `buff_tuansanlang_Unbal` |

Exclude BitGame's damage-triggered stacks. Preserve exact-rank parameter lookup,
canonical `CHARACTER_ELEMENT_TYPE_*` values including `NATURE`, existing schema
compatibility, wearer-only statistics, and explicit unsupported reasons.

If only part of the behavior becomes provable, publish that evidence while
keeping unsupported execution paths unavailable. Do not promote either current
rule merely because the scenario has one source: the target and operation are
currently unproven too. A packet scan is not required to export these assets;
controlled gameplay observations, if later requested, would be separate evidence.

Update upstream contracts/tests, generated catalogs, and manifest hashes together.
Report certified effect IDs, verified targets/operations and policies, unresolved
cases, supporting asset fields, and validation results. No optimizer-specific
Go mappings, account calibration, or runtime localized-text parsing should be
needed to consume the publication.
