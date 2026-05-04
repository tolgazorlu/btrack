# btrack installer for Windows
# Usage: irm https://raw.githubusercontent.com/tolgazorlu/btrack/main/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$repo    = "tolgazorlu/btrack"
$binName = "btrack.exe"
$installDir = "$env:LOCALAPPDATA\btrack"

function Write-Step($msg) {
    Write-Host "  " -NoNewline
    Write-Host $msg -ForegroundColor Cyan
}

function Write-Ok($msg) {
    Write-Host "  " -NoNewline
    Write-Host "✓ " -ForegroundColor Green -NoNewline
    Write-Host $msg
}

function Write-Fail($msg) {
    Write-Host "  " -NoNewline
    Write-Host "✗ " -ForegroundColor Red -NoNewline
    Write-Host $msg
    exit 1
}

Write-Host ""
Write-Host "  btrack installer" -ForegroundColor Blue
Write-Host "  ─────────────────────────────────" -ForegroundColor DarkGray
Write-Host ""

# 1. Detect architecture
$arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else { Write-Fail "32-bit Windows is not supported" }
Write-Step "Detected: windows-$arch"

# 2. Get latest release from GitHub
Write-Step "Fetching latest release..."
try {
    $release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
} catch {
    Write-Fail "Could not reach GitHub API. Check your internet connection."
}

$version = $release.tag_name
$assetName = "btrack-windows-$arch.zip"
$asset = $release.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1

if (-not $asset) {
    Write-Fail "Release asset '$assetName' not found in $version"
}

Write-Ok "Found btrack $version"

# 3. Download zip
$tmpDir  = Join-Path $env:TEMP "btrack-install-$([System.IO.Path]::GetRandomFileName())"
$zipPath = Join-Path $tmpDir "btrack.zip"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

Write-Step "Downloading $assetName..."
try {
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zipPath -UseBasicParsing
} catch {
    Write-Fail "Download failed: $_"
}
Write-Ok "Downloaded"

# 4. Extract
Write-Step "Extracting..."
Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force

$exePath = Join-Path $tmpDir $binName
if (-not (Test-Path $exePath)) {
    Write-Fail "btrack.exe not found in archive"
}

# 5. Install to LOCALAPPDATA\btrack
Write-Step "Installing to $installDir..."
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Copy-Item $exePath (Join-Path $installDir $binName) -Force
Write-Ok "Installed btrack.exe"

# 6. Add to PATH (user-level, no admin needed)
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$userPath;$installDir", "User")
    Write-Ok "Added to PATH"
} else {
    Write-Ok "Already in PATH"
}

# 7. Cleanup
Remove-Item $tmpDir -Recurse -Force

# 8. Done
Write-Host ""
Write-Host "  ─────────────────────────────────" -ForegroundColor DarkGray
Write-Host "  btrack $version ready!" -ForegroundColor Green
Write-Host ""
Write-Host "  Restart your terminal, then:" -ForegroundColor DarkGray
Write-Host "  btrack version" -ForegroundColor White
Write-Host "  btrack s `"fix login bug`"" -ForegroundColor White
Write-Host ""
