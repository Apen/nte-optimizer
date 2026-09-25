# Optimizer command-line runner

The development CLI runs the same Go optimization service as the desktop build planner. It reads the imported account and saved profile settings from the local user-data directory. It never saves or equips a build.

From the repository root:

```powershell
go run ./cmd/nte-optimize --character 1036 --method fast
go run ./cmd/nte-optimize --character 1036 --method beta
```

`--character` accepts a numeric character ID or a profile ID such as `zankou`. If a character has several profiles, the command picks the first profile in the same sorted list used by the UI's initial selection. Use `--profile PROFILE_ID` to select a specific variant; `--profile` can also be used without `--character`.

The default method is `fast`. The command uses the profile's default cartridge main stats, weights, targets, and tolerances unless saved settings override them. Disabled goals, strict minimums, current equipment, character-priority reservations, and imported account overrides are also applied. Unsaved changes visible in the UI are **not** available to the CLI; save the profile first when comparing the same configuration. Temporary pinned or excluded modules selected in the UI are not persisted and therefore are not reproduced.

Fast and Beta use the shared 60-second search timeout from
`data/optimizer/config.json`. Candidate preparation occurs before this timer
starts. Press Ctrl+C to cancel the CLI run.

The report includes the selected profile and search status, every active goal with its target, weight, strict minimum, final value and points, the ranking breakdown, the selected equipment with stats, and projected action damage. An incomplete search is marked approximate. Ranking values from different methods are on different scales and must not be compared directly; compare builds, final stats, and damage projections instead.

Options:

```text
--character ID     Numeric character ID or profile ID
--profile ID       Exact profile variant
--method METHOD    fast (default) or beta
--lang LANGUAGE    en (default) or fr for game labels
--state-dir PATH   User-data directory containing workspace/; defaults to
                   %LOCALAPPDATA%\NTE Optimizer on Windows
--data PATH        Project data directory; defaults to data
--experiments PATH Run up to 16 named, one-shot Fast/Beta variants from JSON
--json             Emit the complete optimization result as JSON
```

## One-shot experiments

`--experiments` accepts a strict JSON plan with `schema_version: 1` and between
1 and 16 uniquely named variants. Each variant overlays only the fields it
contains on the selected profile's saved settings. If a variant omits `method`,
it uses the command's `--method`. Supported fields are
`method`, `arc_fork_id`, `main_stats`, partial `weights`, partial `goals`, and
`search` limits (`top_per_geometry`, `top_per_set`, `timeout_seconds`). Use
`"arc_fork_id": "none"` to disable the equipped Arc for one variant. A missing
field keeps the saved/default value. The command reads the account snapshot
once for the whole plan, runs each variant against that same inventory and game
catalog, and never writes the temporary settings to disk. Ctrl+C stops the
remaining variants and emits a partial report with canceled statuses.

Weights are the same strategy weights used by the optimizer. Goal patches can
change `target`, `minimum`, `maximum`, `tolerance`, `strict_minimum`,
`disabled`, and display metadata. New custom goals require a `target`. Limits
are per search: `top_per_geometry` is 1–10000, `top_per_set` is 0–10000, and
`timeout_seconds` is 1–86400. Fast and Beta may raise their internal minimum
retention for objective coverage; `selected_candidates` in the report is the
actual retained count.

For the Zankou reference profile, the checked-in example compares Fast and
Beta at saved weights, then varies ATK, CRIT Rate, CRIT DMG, Cycle Intensity,
and Universal DMG one at a time in both modes. It selects Ravenous Blade by its
game Arc ID and expects that Arc to be present in the imported account:

```powershell
go run ./cmd/nte-optimize --character 1036 --method fast `
  --experiments internal/testdata/optimizer/zankou_experiments.example.json `
  --json | Set-Content -LiteralPath "$env:TEMP\zankou-experiments.json" -Encoding utf8
```

The experiment JSON report is sanitized for sharing: it contains the chosen
Arc name and level, equipment stats without local equipment IDs, score
components, final stats, candidate and visited-state counts, duration, and
expected damage for each grouped action against the currently equipped build.
It also reports the Basic DMG index for the candidate and equipped build, with
its percentage delta.
Its machine-readable `interpretation` section marks the damage baseline, that
action damage is not rotation DPS, that no validated rotation is available,
and that Fast/Beta ranking values are not comparable. The text table is
localized by `--lang` and emphasizes per-action damage deltas and
candidate/search cost rather than naming a winner.
The supplied Zankou reference (ranking `33.017212`, roughly `+5.2%` on most
shown actions and `-2.3%` on Scorch) is a comparison landmark only. Regression
tests check score/damage reporting against engine results and synthetic
arithmetic; they do not require a future search to reproduce those values.

Unlike experiment JSON, the ordinary `--json` output without `--experiments`
is the full internal optimization result and can contain local inventory IDs.
Keep that output local or sanitize it before sharing.

Run the deterministic benchmark over the small synthetic search fixture with:

```powershell
go test ./internal/app -run '^$' -bench BenchmarkSanitizedSearchModes -benchmem
```

It measures search modes on synthetic data only; it is useful for relative
regression checks, not as a prediction of full-account runtime.

For example, to inspect an exact profile and capture a report:

```powershell
go run ./cmd/nte-optimize --profile zankou --method fast > zankou-report.txt
```

The ordinary text report and full JSON output can contain local inventory
identifiers and account-derived statistics. The `--experiments --json` report
omits local equipment IDs and file paths, but still contains the profile
settings and account-derived build statistics requested for analysis. The CLI
does not add telemetry or upload anything.
