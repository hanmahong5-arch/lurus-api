#!/usr/bin/env pwsh
# Test Coverage Script for lurus-api
# Runs all tests with coverage profiling and checks thresholds

param(
    [switch]$Html,
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

Write-Host "=== Running tests with coverage ===" -ForegroundColor Cyan

$coverArgs = @(
    "test"
    "-coverprofile=coverage.out"
    "-covermode=atomic"
    "-count=1"
)

if ($Verbose) {
    $coverArgs += "-v"
}

$coverArgs += @("./internal/data/model/...", "./internal/server/controller/...", "./internal/server/middleware/...", "./internal/biz/service/...")

& go @coverArgs
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Tests failed!" -ForegroundColor Red
    exit 1
}

Write-Host "`n=== Coverage Summary ===" -ForegroundColor Cyan
& go tool cover -func=coverage.out

if ($Html) {
    Write-Host "`n=== Generating HTML report ===" -ForegroundColor Cyan
    & go tool cover -html=coverage.out -o coverage.html
    Write-Host "HTML report: coverage.html" -ForegroundColor Green
}

# Check thresholds per package
Write-Host "`n=== Checking Coverage Thresholds ===" -ForegroundColor Cyan

$output = & go tool cover -func=coverage.out
$failed = $false

foreach ($line in $output) {
    if ($line -match "^(.*?)\s+total:\s+\(statements\)\s+([\d.]+)%") {
        # This is the total line for a file, skip
        continue
    }
}

# Extract per-package totals
$packages = @{
    "internal/server/controller/" = 50
    "internal/data/model/"        = 60
    "internal/server/middleware/"  = 50
    "internal/biz/service/"       = 40
}

foreach ($pkg in $packages.Keys) {
    $threshold = $packages[$pkg]
    $pkgOutput = & go test -coverprofile=NUL -covermode=atomic "./$pkg..." 2>&1 | Select-String "coverage:"
    if ($pkgOutput -match "([\d.]+)%") {
        $coverage = [double]$Matches[1]
        if ($coverage -lt $threshold) {
            Write-Host "FAIL: $pkg coverage ${coverage}% < ${threshold}% threshold" -ForegroundColor Red
            $failed = $true
        } else {
            Write-Host "PASS: $pkg coverage ${coverage}% >= ${threshold}% threshold" -ForegroundColor Green
        }
    } else {
        Write-Host "WARN: Could not determine coverage for $pkg" -ForegroundColor Yellow
    }
}

if ($failed) {
    Write-Host "`nCoverage thresholds not met!" -ForegroundColor Red
    exit 1
} else {
    Write-Host "`nAll coverage thresholds met!" -ForegroundColor Green
}
