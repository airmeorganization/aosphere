# ==============================================================================
# AO Sphere Multi-Architecture Cross-Compilation Script (PowerShell)
# Builds statically linked binaries for Linux, macOS, and Windows.
# ==============================================================================
param(
    [string]$Version = "1.0.0",
    [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"

Write-Host "==> Starting multi-platform cross-compilation for AO Sphere v$Version..." -ForegroundColor Cyan

if (Test-Path $OutputDir) {
    Remove-Item -Recurse -Force $OutputDir
}
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

$targets = @(
    @{ OS = "linux";   Arch = "amd64"; Ext = "" },
    @{ OS = "linux";   Arch = "arm64"; Ext = "" },
    @{ OS = "darwin";  Arch = "amd64"; Ext = "" },
    @{ OS = "darwin";  Arch = "arm64"; Ext = "" },
    @{ OS = "windows"; Arch = "amd64"; Ext = ".exe" }
)

$ldflags = "-s -w -X 'main.Version=v$Version'"

foreach ($t in $targets) {
    $binName = "aosphere-v$Version-$($t.OS)-$($t.Arch)$($t.Ext)"
    $outPath = Join-Path $OutputDir $binName
    Write-Host "==> Compiling for $($t.OS)/$($t.Arch) -> $binName..." -ForegroundColor Yellow

    $env:CGO_ENABLED = "0"
    $env:GOOS = $t.OS
    $env:GOARCH = $t.Arch

    go build -trimpath -ldflags $ldflags -o $outPath .
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed compiling for $($t.OS)/$($t.Arch)"
    }

    # Generate SHA-256 checksum
    $hash = (Get-FileHash -Algorithm SHA256 $outPath).Hash.ToLower()
    Set-Content -Path "$outPath.sha256" -Value "$hash  $binName"
    Write-Host "    [SUCCESS] $binName (SHA256: $hash)" -ForegroundColor Green
}

Write-Host "==> All target binaries compiled and checksummed into $OutputDir/" -ForegroundColor Cyan
Get-ChildItem $OutputDir

