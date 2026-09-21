[CmdletBinding()]
param(
    [string]$Archive = ""
)

$ErrorActionPreference = "Stop"
$projectDir = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($Archive)) {
    $Archive = Join-Path $projectDir "build\release\nte-optimizer-windows.zip"
}
$Archive = [System.IO.Path]::GetFullPath($Archive)
if (-not (Test-Path -LiteralPath $Archive -PathType Leaf)) {
    throw "Release archive not found: $Archive"
}

$smokeRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("nte-optimizer-release-smoke-" + [guid]::NewGuid().ToString("N"))
$oldLocalAppData = $env:LOCALAPPDATA
$desktop = $null
try {
    New-Item -ItemType Directory -Path $smokeRoot | Out-Null
    Expand-Archive -LiteralPath $Archive -DestinationPath $smokeRoot

    $desktopPath = Join-Path $smokeRoot "nte-optimizer.exe"
    $scannerPath = Join-Path $smokeRoot "nte-scan.exe"
    $required = @(
        $desktopPath,
        $scannerPath,
        (Join-Path $smokeRoot "data\optimizer\config.json"),
        (Join-Path $smokeRoot "data\recommendations\targets")
    )
    foreach ($path in $required) {
        if (-not (Test-Path -LiteralPath $path)) {
            throw "Release is missing required path: $path"
        }
    }

    $scannerVersion = & $scannerPath -version
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace(($scannerVersion | Out-String))) {
        throw "Packaged scanner did not start correctly."
    }

    $env:LOCALAPPDATA = Join-Path $smokeRoot "UserData"
    $desktop = Start-Process -FilePath $desktopPath -WorkingDirectory $smokeRoot -WindowStyle Hidden -PassThru
    Start-Sleep -Seconds 3
    if ($desktop.HasExited) {
        throw "Packaged desktop application exited during startup with code $($desktop.ExitCode)."
    }
    Write-Host "[SMOKE] Packaged scanner and desktop started successfully." -ForegroundColor Green
}
finally {
    if ($null -ne $desktop -and -not $desktop.HasExited) {
        Stop-Process -Id $desktop.Id -Force
        $desktop.WaitForExit()
    }
    $env:LOCALAPPDATA = $oldLocalAppData
    $expectedPrefix = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
    $resolvedSmokeRoot = [System.IO.Path]::GetFullPath($smokeRoot)
    if ($resolvedSmokeRoot.StartsWith($expectedPrefix, [System.StringComparison]::OrdinalIgnoreCase) -and
        [System.IO.Path]::GetFileName($resolvedSmokeRoot).StartsWith("nte-optimizer-release-smoke-")) {
        Remove-Item -LiteralPath $resolvedSmokeRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
