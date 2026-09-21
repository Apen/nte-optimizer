# Data and localization

The `data` directory is organized by responsibility and source of truth.

```text
data/
├── game/                    generated facts from the game
│   ├── characters/          base stats and character decode projection
│   ├── combat/              structured damage rules
│   ├── equipment/           Arcs, traits, grids, sets, shapes, decode data
│   ├── forks/               Arc network decode projection
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
- selected `*_name` entries from skill-description StringTables.

`tables.items` and a separate `locales/items` catalog are not used. Game names
must not be hardcoded in Go or React as translation fallbacks. If a label is
missing, fix and republish the generated locale source.

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
