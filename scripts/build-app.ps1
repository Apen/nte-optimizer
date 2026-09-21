[CmdletBinding()]
param(
    [ValidateNotNullOrEmpty()]
    [string]$Name = "nte-optimizer.exe",

    [string]$Version = "",

    [switch]$Launch,

    [switch]$SmokeTest,

    [switch]$Quality
)

$ErrorActionPreference = "Stop"
$projectDir = Split-Path -Parent $PSScriptRoot
$frontendDir = Join-Path $projectDir "frontend"
$binDir = Join-Path $projectDir "build\bin"
$outputPath = Join-Path $binDir $Name
$releaseRoot = Join-Path $projectDir "build\release"
$releaseDir = Join-Path $releaseRoot "nte-optimizer"
$archivePath = Join-Path $releaseRoot "nte-optimizer-windows.zip"
$checksumPath = "$archivePath.sha256"

$commit = (& git -C $projectDir rev-parse --short=12 HEAD 2>$null)
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($commit)) {
    $commit = "unknown"
}
$commit = $commit.Trim()
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = (& git -C $projectDir describe --tags --always --dirty 2>$null)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($Version)) {
        $Version = "dev"
    }
}
$Version = $Version.Trim()
$buildDate = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
$metadataLdflags = "-X nte-optimizer/internal/buildinfo.Version=$Version -X nte-optimizer/internal/buildinfo.Commit=$commit -X nte-optimizer/internal/buildinfo.BuildDate=$buildDate"

Write-Host "[BUILD] Version $Version ($commit, $buildDate)" -ForegroundColor Cyan

if ($Quality) {
    & (Join-Path $PSScriptRoot "check-quality.ps1")
    if ($LASTEXITCODE -ne 0) {
        throw "Quality checks failed with exit code $LASTEXITCODE."
    }
}

Write-Host "[TEST] Go and production data" -ForegroundColor Cyan
Push-Location $projectDir
try {
    & go test ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Go tests or production data validation failed with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}

Write-Host "[BUILD] Frontend React" -ForegroundColor Cyan
Push-Location $frontendDir
try {
    & npm test
    if ($LASTEXITCODE -ne 0) {
        throw "Frontend tests failed with exit code $LASTEXITCODE."
    }
    & npm run build
    if ($LASTEXITCODE -ne 0) {
        throw "Frontend build failed with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}

# Go embed rejects the multiplication sign in file names. Datamined assets keep
# their game-facing identifier, so normalize only the generated frontend copy.
$frontendDist = Join-Path $frontendDir "dist"
Get-ChildItem -LiteralPath $frontendDist -Recurse -File | Where-Object { $_.Name.Contains("×") } | ForEach-Object {
    $normalizedName = $_.Name.Replace("×", "x")
    $normalizedPath = Join-Path $_.DirectoryName $normalizedName
    if (Test-Path -LiteralPath $normalizedPath) {
        throw "Cannot normalize embedded asset '$($_.FullName)': '$normalizedPath' already exists."
    }
    Move-Item -LiteralPath $_.FullName -Destination $normalizedPath
}

New-Item -ItemType Directory -Path $binDir -Force | Out-Null

Write-Host "[BUILD] Scanner helper" -ForegroundColor Cyan
Push-Location $projectDir
try {
    & go build -ldflags $metadataLdflags -o (Join-Path $binDir "nte-scan.exe") .\cmd\nte-scan
    if ($LASTEXITCODE -ne 0) {
        throw "Scanner build failed with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}

Write-Host "[BUILD] Windows application" -ForegroundColor Cyan
Push-Location $projectDir
try {
    $desktopLdflags = "-w -s -H windowsgui $metadataLdflags"
    & go build -tags "desktop production" -ldflags $desktopLdflags -o $outputPath .
    if ($LASTEXITCODE -ne 0) {
        throw "Go build failed with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}

$artifact = Get-Item -LiteralPath $outputPath
Write-Host "[BUILD] Ready: $($artifact.FullName) ($([math]::Round($artifact.Length / 1MB, 1)) MB)" -ForegroundColor Green

Write-Host "[PACKAGE] Portable Windows directory" -ForegroundColor Cyan
$expectedReleaseDir = [System.IO.Path]::GetFullPath((Join-Path $projectDir "build\release\nte-optimizer"))
if ([System.IO.Path]::GetFullPath($releaseDir) -ne $expectedReleaseDir) {
    throw "Unexpected release directory: $releaseDir"
}
if (Test-Path -LiteralPath $releaseDir) {
    Remove-Item -LiteralPath $releaseDir -Recurse -Force
}
New-Item -ItemType Directory -Path $releaseDir -Force | Out-Null
Copy-Item -LiteralPath $outputPath -Destination (Join-Path $releaseDir $Name)
Copy-Item -LiteralPath (Join-Path $binDir "nte-scan.exe") -Destination (Join-Path $releaseDir "nte-scan.exe")
Copy-Item -LiteralPath (Join-Path $projectDir "data") -Destination (Join-Path $releaseDir "data") -Recurse
Write-Host "[PACKAGE] Ready: $releaseDir" -ForegroundColor Green

Write-Host "[PACKAGE] Windows archive and checksum" -ForegroundColor Cyan
foreach ($generatedPath in @($archivePath, $checksumPath)) {
    if (Test-Path -LiteralPath $generatedPath) {
        Remove-Item -LiteralPath $generatedPath -Force
    }
}
Compress-Archive -Path (Join-Path $releaseDir "*") -DestinationPath $archivePath -CompressionLevel Optimal
$archiveHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
$checksumLine = "$archiveHash  $([System.IO.Path]::GetFileName($archivePath))"
[System.IO.File]::WriteAllText($checksumPath, "$checksumLine`n", [System.Text.UTF8Encoding]::new($false))
Write-Host "[PACKAGE] Ready: $archivePath" -ForegroundColor Green
Write-Host "[PACKAGE] SHA-256: $archiveHash" -ForegroundColor Green

if ($SmokeTest) {
    Write-Host "[SMOKE] Testing extracted Windows archive" -ForegroundColor Cyan
    & (Join-Path $PSScriptRoot "test-release.ps1") -Archive $archivePath
    if ($LASTEXITCODE -ne 0) {
        throw "Release smoke test failed with exit code $LASTEXITCODE."
    }
}

if ($Launch) {
    Write-Host "[RUN] Starting $($artifact.Name)" -ForegroundColor Cyan
    $releaseExecutable = Join-Path $releaseDir $Name
    $process = Start-Process -FilePath $releaseExecutable -WorkingDirectory $releaseDir -PassThru
    Write-Host "[RUN] Started with PID $($process.Id)" -ForegroundColor Green
}
