# Optimizer command-line runner

The development CLI runs the same Go optimization service as the desktop build planner. It reads the imported account and saved profile settings from the local user-data directory. It never saves or equips a build.

From the repository root:

```powershell
go run ./cmd/nte-optimize --character 1036 --method fast
go run ./cmd/nte-optimize --character 1036 --method beta
```

`--character` accepts a numeric character ID or a profile ID such as `zankou`. If a character has several profiles, the command picks the first profile in the same sorted list used by the UI's initial selection. Use `--profile PROFILE_ID` to select a specific variant; `--profile` can also be used without `--character`.

The default method is `fast`. The command uses the profile's default cartridge main stats, weights, targets, and tolerances unless saved settings override them. Disabled goals, strict minimums, current equipment, character-priority reservations, and imported account overrides are also applied. Unsaved changes visible in the UI are **not** available to the CLI; save the profile first when comparing the same configuration. Temporary pinned or excluded modules selected in the UI are not persisted and therefore are not reproduced.

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
--json             Emit the complete optimization result as JSON
```

For example, to inspect an exact profile and capture a report:

```powershell
go run ./cmd/nte-optimize --profile zankou --method fast > zankou-report.txt
```

The JSON output and text report can contain private inventory identifiers and account-derived statistics. Keep reports local or anonymize them before sharing. The CLI does not add telemetry or upload anything.
