# setup.ps1 — build the s1temap CLI binary (Windows / PowerShell)
#Requires -Version 5.1
#
# Safe to run from anywhere: it resolves the Go module root (repo root) relative to
# this script, so it works whether the skill lives in the repo or is copied into
# an agent's skills directory.

$RequiredMajor = 1
$RequiredMinor = 26
$Binary = "s1temap.exe"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ModuleDir = Split-Path -Parent $ScriptDir   # repo root — the directory containing go.mod
Set-Location $ModuleDir

# ── Check Go ──────────────────────────────────────────────────────────────────
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

# ── Build (skip if binary already exists) ────────────────────────────────────
if (Test-Path ".\$Binary") {
    Write-Host "Binary $ModuleDir\$Binary already exists — skipping build." -ForegroundColor Cyan
    Write-Host "Delete it or run 'go build -o $Binary ./cmd/cli' to rebuild."
} else {
    Write-Host "Building $Binary..."
    go build -o $Binary ./cmd/cli

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed." -ForegroundColor Red
        exit 1
    }

    Write-Host ""
    Write-Host "Build complete: $ModuleDir\$Binary" -ForegroundColor Green
}

Write-Host ""
Write-Host "Quick start:"
Write-Host "  $ModuleDir\$Binary start https://example.com/sitemap.xml"
Write-Host "  $ModuleDir\$Binary start https://example.com/sitemap.xml --filter-status=!200"
Write-Host "  $ModuleDir\$Binary list .\urls.txt"
Write-Host ""
Write-Host "Optional HTTP API server: go build -o s1temap-api.exe ./cmd/api"
Write-Host "See SKILL.md for the full command reference and examples."
