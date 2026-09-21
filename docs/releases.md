# Release process

## Build a local release

From a clean repository root:

```powershell
.\scripts\build-app.ps1 -Quality -SmokeTest
```

The script injects the Git-derived version, commit, and UTC build date, then
produces:

```text
build/bin/nte-optimizer.exe
build/bin/nte-scan.exe
build/release/nte-optimizer/
build/release/nte-optimizer-windows.zip
build/release/nte-optimizer-windows.zip.sha256
```

The portable directory and ZIP include both executables and the complete static
`data` directory.

## Build options

Use an explicit release version:

```powershell
.\scripts\build-app.ps1 -Version v1.0.0
```

Build to another executable name:

```powershell
.\scripts\build-app.ps1 -Name nte-optimizer-dev.exe
```

Start the application after a successful build:

```powershell
.\scripts\build-app.ps1 -Launch
```

Options can be combined.

## Smoke test

`-SmokeTest` extracts the generated ZIP into a temporary directory, validates
its files, runs the packaged scanner with `-version`, and starts the packaged
desktop application with an isolated `LOCALAPPDATA`. It does not read or modify
the developer's current imported account.

## Release checklist

1. confirm `git status` is clean;
2. run `build-app.ps1 -Quality -SmokeTest`;
3. confirm the embedded version does not contain `dirty`;
4. verify the ZIP contains both executables and `data`;
5. verify the `.sha256` value against the archive;
6. test installation and guided import on a clean Windows user profile;
7. review game-data and asset redistribution rights;
8. publish release notes describing game-data compatibility and known decoder
   limitations;
9. attach the ZIP and checksum together.

## Update and compatibility

Users update by replacing the extracted application folder. Persistent account
data lives under `%LOCALAPPDATA%\NTE Optimizer`, so it survives updates.

The application copies an older portable `workspace` to the user-data location
on first launch when no destination workspace exists. The original portable
workspace is not deleted automatically.

Scanner export schemas and generated game-data schemas must be reviewed when a
game update changes network records. Never publish a decoder change and a large
protocol refactor in the same release without capture-based validation.

## Signing and reputation

The current build process does not sign Windows executables. Before broad public
distribution, consider code signing and publishing reproducible release
metadata. Until then, Windows reputation warnings may appear even for an
unchanged release; users should verify the checksum and source provenance.
