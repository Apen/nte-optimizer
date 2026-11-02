# Data and localization

The `data` directory is organized by responsibility and source of truth.

```text
data/
├── game/                    generated facts from the game
│   ├── characters/          base stats and character decode projection
│   ├── combat/              structured damage rules
│   ├── equipment/           Arcs, traits, grids, sets, shapes, decode data
│   ├── forks/               Arc network decode projection
│   ├── housing/             extracted special-furniture modifiers
│   ├── locales/             one generated file per language
│   └── resources/           resource network decode projection
├── optimizer/               optimizer normalization and configuration
├── presentation/            application-owned interface text
└── recommendations/targets/ structured character build recommendations
```

## Stable identifiers

Relationships use stable game identifiers such as `Suit4`, numeric character
IDs, and `fork_DemonBlade`, never translated display names. When no suitable
game identifier exists, the canonical key is derived from the English name.

Percentages are stored as decimals: `0.24` represents 24%.

## Generated game data

Files under `data/game` are generated and deployed from the datamine pipeline.
They are the source of truth for game labels and game-derived calculations in
this repository. Do not patch them manually: the next data deployment would
overwrite the change.

`data/game/manifest.json` records each generated file's schema version, SHA-256
hash, generation sources, status, and warnings.

The scanner `decode.json` files are intentionally minimal projections. They
contain only identifiers, limits, curves, geometry, and other fields consumed
by network decoding. Unused descriptions, duplicate translations, and source
metadata stay in canonical generated catalogs instead.

## Localization

The interface supports English and French through two independent sources:

- `data/presentation/<language>.json` contains text owned by the application;
- `data/game/locales/<language>.json` contains names extracted from the game.

The generated locale contract currently consumes:

- `tables.characters`;
- `tables.forks`;
- `tables.resources`;
- `tables.sets`;
- `tables.console_trait_effects`, keyed by character ID;
- `tables.ST_furniture`, keyed by the extracted furniture progression label ID;
- selected `*_name` entries from skill-description StringTables.

`tables.items` and a separate `locales/items` catalog are not used. Game names
must not be hardcoded in Go or React as translation fallbacks. If a label is
missing, fix and republish the generated locale source.

Console trait descriptions are display text only. Their numeric stat values
remain sourced from `data/game/equipment/console_traits.json` for optimization.

Equipment import resolves item kind, quality, maximum level, and cartridge set
from `data/game/equipment/decode.json`. Runtime module shapes are matched to
`equipment/shapes.json` by translated cell coordinates without rotating them.
There are no application-owned item-prefix, geometry, or set lookup tables.
Legacy cartridge set IDs are repaired at data-aware boundaries using these same
definitions; inventory models receive definitions explicitly and perform no I/O.

Special-furniture progression levels and per-character bond levels are imported
account observations. The furniture modifier catalog includes `progression_id`
and `level`, resolved by the datamine generator through its progression
configuration. The application maps scanned levels to those catalog records
without hardcoded furniture-to-stat exceptions.

The builder treats all supported furniture bonuses displayed on Account Buffs
as active for deployed characters, and applies each displayed level-10 bond
bonus to its corresponding character. The informational page and calculations
share the same backend eligibility rules. Manual `account_buffs.json`
confirmations are obsolete. The scanner still does not independently establish
furniture placement, activation flags, or bond reward claiming; assuming the
selected bonuses active is an explicit application policy, not decoded evidence.

## Observed character panels

In the captures analyzed so far, the game sends final runtime attribute panels
for deployed characters, not a complete final-stat table for the entire roster.
Opening a non-deployed character's menu did not reveal an equivalent complete
panel in the decoded traffic. For this scan workflow, the final stats of
non-deployed characters are not present in the captured packets: opening their
menus does not provide those missing final-stat panels. Do not expect to recover
them by searching the same packets for another API call or RPC. This documents
the observed behavior; a future protocol change would require new evidence.

For the remaining characters, the application must reconstruct deployed-build
stats from their progression, equipped modules, cartridges, Arc, set effects,
console trait, and applicable account bonuses using `data/game`. It also
recalculates stats for new builds. Received panels serve as comparison evidence;
they are not reusable base stats and must not be added to equipment bonuses.

Detailed panels come from replicated character attribute blocks, separately from
the roster. Panel association uses the roster NetID slot/serial from the
replicated actor properties, not HP equality or blueprint names. Characters
with equal or stale saved HP can therefore receive distinct owned panels.
Duplicate roster identities, conflicting actors or panels, and unsupported
framing leave the panel unavailable rather than assigning a tentative match.

A subsequent private login capture supplied ownership evidence for four distinct
deployed characters. Each actor's replicated property sequence contained the
roster NetID slot and serial, with observed encoded handles 198 and 200. The
`AttributeSet` package export's outer GUID matched the spawned actor GUID in
each case. These are actor property handles, separate from identically numbered
attribute-set handles; they must not be interpreted without their owning layout.
The decoder enforces this observed identity contract with bounded parsing and
synthetic regression fixtures, including equal-HP owners, conflicting identities,
wrong outer GUIDs, wrong attribute references, and truncated messages.

That capture also contained successive package-export sections ahead of a
must-map GUID list and actor creation data. Parsing only the first export section
does not locate the actor payload. The decoder reads these sections and locates
the first complete actor/subobject framing within a bounded spawn-header region,
without assuming a fixed transform length. It requires declared outer GUIDs for
every subobject and exact consumption of the message. It never retries a later
framing after that actor's domain identity is rejected.

Attribute-set base values and the final-value sequence are in separate blocks.
The decoder reads the former inside the owned `AttributeSet` and the latter
inside its actor's `AbilitySystemComponent`, requiring an explicit reference to
that same attribute-set GUID. The observed final-value handles are 36/38/40 for
the consistent HP triple, 46 for attack, and 56 for endurance. It does not scan
an entire channel for matching numbers. The raw capture and private analysis
are not source-controlled fixtures.

Attribute values are decoded separately from
the saved roster HP values. The scanner reads the observed encoded field handles
instead of assuming fixed positions: omitted cycle values, independently omitted
base/current members, and additional elemental fields can change the layout.
It requires a consistent owned HP triple and validated required fields before
attaching a panel. Missing cycle fields and ambiguous elemental bonuses are omitted from
the export rather than presented as measured zero values.

A character blueprint observed during login is not proof that every panel field
was captured, nor an authoritative team-selection record. Characters without a
recognized replicated panel retain their saved HP only. Those values must not be
treated as a complete final-stat reference.

The non-deployed character menu can omit Arc attributes, Arc passive effects,
and cartridge set effects even when the account record links that equipment to
the character. Keep that menu state separate from a projected deployed build;
do not remove equipped Arc or set contributions from optimization merely to
match an inactive menu. Compare values at the precision actually displayed by
the game, and verify conditional effects separately. Agreement on the visible
attributes does not establish correctness for unobserved attributes or combat
states.

### Reconciling extracted effect curves

An extracted curve, a display description, and a runtime effect can disagree
across game versions. Resolve discrepancies in the datamine source and republish
the generated catalogs and manifest; never add a character-specific correction
to Go or React.

The published Prism catalog uses `CT_ForkBuff.buff_Prism_CritUp` values of
15/18/21/24/27%, matching the public extracted data at the pinned
[NTE_Assets revision](https://github.com/Waifus-Grace/NTE_Assets/blob/1816e237ffe9edf687dd6942d028e89cce121381/DataTable/Fork/CT_ForkBuff.json)
and [NTEData revision](https://github.com/555me/NTEData/blob/cf2508dc39687aab2e08ec62d6eb86fe957a9962/DataTable/Fork/CT_ForkBuff.json)
revisions. The rank-one value of 15% agrees with the observed panel comparison.
The published `buff_Prism_LakshanaUp` values are 20/25/30/35/40%. Earlier local
data used 16/20/24/28/32% for both curves. Keep calculated passives and displayed
parameters consistent across all five ranks, and prevent source overrides from
restoring the earlier values during regeneration.

## Recommendations

Each file under `data/recommendations/targets` owns one character's common
context, strategies, set variants, goals, Arc choices, and source metadata.
These are optimizer inputs rather than raw game facts and may be maintained in
this repository or replaced by another recommendation source without changing
the runtime layout.

To add or update a strategy:

1. update the character document in `data/recommendations/targets`;
2. keep common character context at the document root;
3. add strategies and compact set variants without duplicating shared data;
4. verify the character grid in `data/game/equipment/grids.json`;
5. use `data/presentation` only for application-owned text;
6. run all Go tests and the frontend build.

## Visual assets

Runtime game images are stored below `assets/game_ui`:

- `characters/<characterID>.png`;
- `forks/<forkID>.png`;
- `equipment/module/<item>.png`;
- `equipment/core/<set and quality>.png`;
- `resources/<itemID>.png`.

Vite exposes this directory to the embedded frontend. The build script
normalizes the multiplication sign in generated filenames because Go embed
rejects that character in file names.

## Rights and redistribution

Game images, names, and extracted data remain the property of their respective
rights holders. Their inclusion does not grant a general redistribution
license. Before a public release, confirm that distributing every bundled asset
and generated dataset is permitted.

Third-party Go and npm packages retain the licenses declared by their upstream
projects and lockfiles. The repository currently has no root source-code
license; add one deliberately before accepting external contributions or
redistribution.
