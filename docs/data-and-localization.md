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

Detailed panels come from replicated character attribute blocks, separately from
the saved roster HP values. The scanner reads the observed encoded field handles
instead of assuming fixed positions: omitted cycle values, independently omitted
base/current members, and additional elemental fields can change the layout.
It requires a consistent HP anchor and validated required fields before attaching
a panel. Missing cycle fields and ambiguous elemental bonuses are omitted from
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
