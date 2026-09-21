# Development

## Prerequisites

- Windows 10/11;
- Go 1.25;
- Node.js 22 and npm;
- Git;
- Wails CLI v2.16 only for integrated development and binding generation.

Install the pinned Wails CLI when needed:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
```

## Initial setup

From the repository root:

```powershell
cd frontend
npm ci
cd ..
go test ./...
```

`npm ci` installs the exact frontend dependency versions from
`package-lock.json`. Use `npm install` only when intentionally updating that
lockfile.

## Development commands

Run the Wails development environment:

```powershell
wails dev
```

Run only the frontend development server from `frontend`:

```powershell
npm run dev
```

After changing a public `DesktopApp` method, regenerate Wails bindings before
committing if the normal Wails workflow has not already done so.

## Tests and quality

The full local quality gate is:

```powershell
.\scripts\build-app.ps1 -Quality -SmokeTest
```

It runs:

- `go vet ./...`;
- `go mod tidy -diff`;
- `staticcheck v0.8.1`;
- `govulncheck v1.8.0`;
- all Go tests and production-data contract tests;
- Vitest frontend tests;
- the TypeScript and Vite production build;
- scanner and desktop compilation;
- portable archive creation and an extracted-archive smoke test.

Useful focused commands are:

```powershell
go test ./...
go vet ./...
cd frontend
npm test
npm run build
```

Generate a coverage profile in PowerShell with quoted Go flags:

```powershell
go test ./... '-coverprofile=coverage.out'
go tool cover '-func=coverage.out'
```

Race detection requires CGO and a supported C compiler on Windows. The current
local environment does not provide that toolchain, so `go test -race ./...` is
not part of the local gate.

## Code organization rules

- keep privileged capture separate from the non-elevated desktop application;
- keep scanner protocol parsing, domain decoding, and export publication
  separate;
- keep game facts, optimizer rules, recommendations, and presentation text in
  their dedicated data areas;
- do not hardcode translated game names in Go or React;
- avoid interfaces for single implementations without a real substitution or
  testing need;
- preserve transactional account import and cancellation behavior;
- add focused tests for behavior changes instead of targeting arbitrary 100%
  coverage.

All documentation and source-code comments must be written in English. Runtime
translations belong in the locale files and may use their target language.

## CI

The Windows workflow installs locked npm dependencies, checks Go module files,
runs vet, executes the build script, and verifies the portable archive and its
checksum. Local release creation does not depend on CI.
