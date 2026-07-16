# setup.ps1 — build & install the s1temap CLI binary (Windows / PowerShell)
#Requires -Version 5.1
#
# The binary is ALWAYS installed to one fixed, session-independent location:
#     $env:S1TEMAP_HOME\s1temap.exe   (default: %USERPROFILE%\.s1temap\bin\s1temap.exe)
#
# This is the key to reliability: every future agent session — in Claude Code,
# Codex, Cursor, etc. — looks in the same fixed path, regardless of the current
# working directory or where the skill folder happens to live.
#
# Safe to run from anywhere: it walks up from its own location to find the Go
# module root (the directory containing go.mod) only when a build is needed.
#
# The LAST line printed to stdout is the absolute path to the binary, so an agent
# can capture and remember it.

$RequiredMajor = 1
$RequiredMinor = 26
$Binary = "s1temap.exe"

# ── Fixed install location (override with S1TEMAP_HOME) ───────────────────────
if ($env:S1TEMAP_HOME) {
    $InstallDir = $env:S1TEMAP_HOME
} else {
    $InstallDir = Join-Path $env:USERPROFILE ".s1temap\bin"
}
$InstallPath = Join-Path $InstallDir $Binary

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# ── 1) Already installed? Use it, print path, done. ──────────────────────────
if (Test-Path $InstallPath) {
    Write-Host "Binary already installed — skipping build." -ForegroundColor Cyan
    Write-Host "To rebuild: delete '$InstallPath' and re-run this script."
    Write-Host ""
    Write-Host "Quick start:"
    Write-Host "  $InstallPath start https://example.com/sitemap.xml"
    Write-Host "  $InstallPath start https://example.com/sitemap.xml --filter-status=!200"
    Write-Host "  $InstallPath list .\urls.txt"
    Write-Host ""
    Write-Output $InstallPath
    exit 0
}

# ── 2) Need to build: locate the Go module root (walk up to find go.mod) ─────
$ModuleDir = $ScriptDir
while ($ModuleDir -and -not (Test-Path (Join-Path $ModuleDir "go.mod"))) {
    $parent = Split-Path -Parent $ModuleDir
    if ($parent -eq $ModuleDir) { $ModuleDir = $null; break }
    $ModuleDir = $parent
}

if (-not $ModuleDir -or -not (Test-Path (Join-Path $ModuleDir "go.mod"))) {
    Write-Host "Error: could not find go.mod above '$ScriptDir'." -ForegroundColor Red
    Write-Host ""
    Write-Host "The binary is not installed yet and the Go source is required to build it."
    Write-Host "Clone the repository and run this script from inside it:"
    Write-Host "  git clone https://github.com/preditrix/s1temap"
    Write-Host "  cd s1temap"
    Write-Host "  .\skill\setup.ps1"
    exit 1
}

# ── 3) Check Go ───────────────────────────────────────────────────────────────
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Go is not installed or not in PATH." -ForegroundColor Red
    Write-Host ""
    Write-Host "Install Go $RequiredMajor.$RequiredMinor or later:"
    Write-Host "  winget:   winget install GoLang.Go"
    Write-Host "  Download: https://go.dev/dl/"
    Write-Host ""
    Write-Host "After installation, open a new PowerShell window and re-run this script."
    exit 1
}

$goVersionString = (go version) -replace "go version go([0-9]+\.[0-9]+).*", '$1'
$parts = $goVersionString.Split(".")
$major = [int]$parts[0]
$minor = [int]$parts[1]

if ($major -lt $RequiredMajor -or ($major -eq $RequiredMajor -and $minor -lt $RequiredMinor)) {
    Write-Host "Error: Go $RequiredMajor.$RequiredMinor+ is required (found go$goVersionString)." -ForegroundColor Red
    Write-Host ""
    Write-Host "Upgrade Go:"
    Write-Host "  winget:   winget upgrade GoLang.Go"
    Write-Host "  Download: https://go.dev/dl/"
    Write-Host ""
    Write-Host "After upgrading, open a new PowerShell window and re-run this script."
    exit 1
}

Write-Host "Go $(go version | Select-String -Pattern 'go\S+' | ForEach-Object { $_.Matches[0].Value }) — OK" -ForegroundColor Green

# ── 4) Build straight into the fixed install location ────────────────────────
Write-Host "Building $Binary -> $InstallPath ..."
Push-Location $ModuleDir
try {
    go build -trimpath -ldflags="-s -w" -o $InstallPath ./cmd/cli
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed." -ForegroundColor Red
        exit 1
    }
} finally {
    Pop-Location
}

Write-Host "Installed: $InstallPath" -ForegroundColor Green
Write-Host ""
Write-Host "Quick start:"
Write-Host "  $InstallPath start https://example.com/sitemap.xml"
Write-Host "  $InstallPath start https://example.com/sitemap.xml --filter-status=!200"
Write-Host "  $InstallPath list .\urls.txt"
Write-Host ""
Write-Host "Optional HTTP API server: Push-Location '$ModuleDir'; go build -o '$InstallDir\s1temap-api.exe' ./cmd/api; Pop-Location"
Write-Host "See SKILL.md for the full command reference and examples."
Write-Host ""

# LAST line = absolute binary path (agents capture this)
Write-Output $InstallPath
