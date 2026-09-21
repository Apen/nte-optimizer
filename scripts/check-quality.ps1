[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$projectDir = Split-Path -Parent $PSScriptRoot
$staticcheckVersion = "v0.8.1"
$govulncheckVersion = "v1.8.0"

Push-Location $projectDir
try {
    Write-Host "[QUALITY] go vet" -ForegroundColor Cyan
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet failed with exit code $LASTEXITCODE." }

    Write-Host "[QUALITY] go mod tidy -diff" -ForegroundColor Cyan
    & go mod tidy -diff
    if ($LASTEXITCODE -ne 0) { throw "go.mod or go.sum is not tidy." }

    Write-Host "[QUALITY] staticcheck $staticcheckVersion" -ForegroundColor Cyan
    & go run "honnef.co/go/tools/cmd/staticcheck@$staticcheckVersion" ./...
    if ($LASTEXITCODE -ne 0) { throw "staticcheck failed with exit code $LASTEXITCODE." }

    Write-Host "[SECURITY] govulncheck $govulncheckVersion" -ForegroundColor Cyan
    & go run "golang.org/x/vuln/cmd/govulncheck@$govulncheckVersion" ./...
    if ($LASTEXITCODE -ne 0) { throw "govulncheck failed with exit code $LASTEXITCODE." }
}
finally {
    Pop-Location
}
