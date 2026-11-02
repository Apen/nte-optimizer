[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$projectDir = Split-Path -Parent $PSScriptRoot
Push-Location $projectDir
try {
    $goFiles = @(& git ls-files -- "*.go")
    if ($LASTEXITCODE -ne 0) { throw "Unable to enumerate tracked Go source files." }
    if ($goFiles.Count -eq 0) { throw "No tracked Go source files were found." }
    $unformatted = @(& gofmt -l $goFiles)
    if ($LASTEXITCODE -ne 0) { throw "Go formatting check failed." }
    if ($unformatted.Count -gt 0) {
        throw "Run gofmt on these files: $($unformatted -join ', ')"
    }
}
finally {
    Pop-Location
}
