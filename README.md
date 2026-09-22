# NTE Optimizer

A local Windows desktop application for importing a **Neverness to Everness**
account inventory and finding strong module, cartridge, and set combinations.

NTE Optimizer follows the same broad idea as inventory optimizers such as
Fribbels: import the gear you actually own, define what a character should
prioritize, and let a solver compare valid combinations. It is a native desktop
application, not a website. The optimizer, account data, and calculations stay
on the user's computer.

> [!IMPORTANT]
> NTE Optimizer is an independent community project. It is not affiliated with,
> endorsed by, or sponsored by the developers or publishers of Neverness to
> Everness.

> [!WARNING]
> NTE Optimizer is under active, intensive development. Features, decoded game
> data, recommendations, and optimization behavior may still change as the
> project is tested against more accounts and game updates. Finding a truly
> optimal build is especially challenging in NTE: unlike simpler equipment
> systems, modules can have several geometries, rotations, and valid placements
> within a character-specific grid. These combinations must be evaluated
> together with cartridges, sets, statistics, constraints, and equipment shared
> across characters. The optimizer is designed to search this space carefully,
> but further correctness, performance, and usability improvements are still in
> progress.

![Optimized build with its console grid, cartridge, and modules](docs/images/application-overview.png)

## What it does

- imports characters, modules, cartridges, Arcs, equipment ownership, and
  selected resources from the NTE login traffic;
- displays the imported account in a localized desktop interface;
- ranks equipment using editable stat goals and recommendation profiles;
- solves character-specific module grids, rotations, cartridge selection, and
  set bonuses together;
- respects equipment already assigned to higher-priority characters unless the
  user explicitly allows reuse;
- compares current and projected character statistics;
- saves proposed builds locally without changing anything in the game.

The application does **not** control NTE, click through its interface, modify
game files, equip items, or write to the player's account.

## Privacy and security at a glance

The current codebase contains no telemetry, analytics, account login, remote
API, or upload feature. It does not send imported account data to a project
server.

The scanner uses Windows `pktmon` to passively record network packets during a
short login window. Administrator permission is required only for the separate
`nte-scan.exe` helper because packet capture is a privileged Windows operation.
The main application and optimizer run without administrator rights.

Temporary ETL and PCAPNG files are removed after a successful scan. The decoded
account snapshot, saved builds, and preferences are stored under:

```text
%LOCALAPPDATA%\NTE Optimizer\workspace
```

Read [Privacy and security](docs/privacy-and-security.md) for the exact trust
model, files written to disk, administrator boundary, and current limitations.

## Quick start

1. Download the Windows release archive.
2. Verify its SHA-256 checksum if one is provided with the release.
3. Extract the archive to a writable folder.
4. Start `nte-optimizer.exe`.
5. Open **Import**, leave NTE on its login screen, and select **Start guided
   scan**.
6. Accept the Windows elevation prompt for `nte-scan.exe`.
7. When the scanner console says it is ready, log in to NTE.
8. Return to the application after the import completes.

No installer or background service is created. See the full
[Getting started guide](docs/getting-started.md) for updates, uninstallation,
troubleshooting, and the standalone scanner command.

## Application tour

| Area | Purpose |
| --- | --- |
| **Characters** | Browse imported characters, inspect their current state, set priority, and start a build search. |
| **Builds** | Select a strategy, edit stat goals and constraints, run the optimizer, compare ranked results, and save a proposed build. |
| **Cartridges** | Browse owned cartridges, their main and secondary stats, set, level, lock state, and owner. |
| **Modules** | Browse owned modules, geometry, stats, set, level, lock state, and owner. |
| **Arcs** | Browse imported Arcs, progression, ownership, and the character currently using them. |
| **Resources** | Browse supported account resources with quantities and game icons. |
| **Import** | Run the guided login scan and review the last successful import summary. |

The user interface supports English and French. Game names come from the
localized data extracted from the game; interface text is maintained
separately.

## How optimization works

The solver combines owned equipment with character grids, module geometry,
rotations, set requirements, Arc statistics, console traits, editable targets,
and account ownership rules. It can return the best result found so far when a
search is stopped, while a completed exact search marks its result as proven
within the retained candidate space.

Displayed scores are comparison tools for one strategy. They are not a gear
percentage and are not directly comparable across different characters or
profiles.

The optimizer uses a fast objective-oriented search that reduces the candidate
space before exploring valid placements. This provides a practical balance
between result quality and waiting time despite the large number of geometry,
rotation, set, and stat combinations.

For formulas, target utility, caps, set scoring, and search behavior, read
[Optimization model](docs/optimization.md).

## Documentation

- [Getting started and user guide](docs/getting-started.md)
- [Privacy and security](docs/privacy-and-security.md)
- [Optimization model](docs/optimization.md)
- [Architecture](docs/architecture.md)
- [Data and localization](docs/data-and-localization.md)
- [Development guide](docs/development.md)
- [Release process](docs/releases.md)
- [Engineering history and remaining limitations](docs/engineering-notes.md)

## Platform and project status

NTE Optimizer currently targets Windows 10/11 because its guided scanner uses
Windows `pktmon`. The desktop application is built with Go, Wails v2, React,
TypeScript, and WebView2.

The versioned catalog currently includes 23 character grids, 12 module
geometries, 12 cartridge sets, 22 console traits, 49 Arcs, and English/French
game labels. The exact set of supported game records depends on the published
datamined catalogs.

## Contributing

Before contributing, read [Development](docs/development.md) and
[Data and localization](docs/data-and-localization.md). Generated game data
must be corrected at its source and republished; it should not be patched by
hand in this repository.

## Licensing and game assets

No project-wide source-code license is currently declared at the repository
root. Until an explicit license is added, the repository should not be assumed
to grant redistribution or modification rights.

Game images, names, and extracted game data remain the property of their
respective rights holders. Their presence does not grant a general license to
redistribute those assets. Third-party Go and npm dependencies retain their own
licenses.
