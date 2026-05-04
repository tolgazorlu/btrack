# btrack installer for Windows
# Run with: irm https://raw.githubusercontent.com/tolgaozgun/btrack/main/scripts/install.ps1 | iex

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\btrack"
)

$ErrorActionPreference = "Stop"
$Repo = "tolgaozgun/btrack"
$Binary = "btrack.exe"

Write-Host "→ Fetching latest btrack release..." -ForegroundColor Cyan

$Latest = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name
$DownloadUrl = "https://github.com/$Repo/releases/download/$Latest/btrack-windows-amd64.exe"

Write-Host "→ Downloading btrack $Latest..." -ForegroundColor Cyan

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$OutPath = Join-Path $InstallDir $Binary
Invoke-WebRequest -Uri $DownloadUrl -OutFile $OutPath

# Add to PATH if not already present
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host "→ Added $InstallDir to PATH" -ForegroundColor Cyan
    Write-Host "  Restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "✓ btrack $Latest installed to $OutPath" -ForegroundColor Green
Write-Host ""
Write-Host "  Get started:"
Write-Host "    btrack start ""my first task"""
Write-Host "    btrack log ""making progress"""
Write-Host "    btrack stop -m ""completed the task #feature"""
Write-Host ""
Write-Host "  Config: $env:USERPROFILE\.config\btrack\config.yaml"
