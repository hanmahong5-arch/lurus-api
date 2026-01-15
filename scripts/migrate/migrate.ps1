# MySQL to PostgreSQL Migration Script for new-api
# Windows PowerShell Script

param(
    [Parameter(Mandatory=$true)]
    [string]$MysqlDsn,

    [Parameter(Mandatory=$true)]
    [string]$PostgresDsn,

    [int]$BatchSize = 1000,

    [switch]$Truncate,

    [switch]$DryRun,

    [string]$Tables = ""
)

Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  MySQL to PostgreSQL Migration Tool" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

# Check if Go is installed
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Get script directory
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Build the migration tool
Write-Host "Building migration tool..." -ForegroundColor Yellow
Push-Location $ScriptDir
go build -o migrate.exe .
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to build migration tool" -ForegroundColor Red
    Pop-Location
    exit 1
}
Pop-Location

# Build arguments
$args = @(
    "-mysql", $MysqlDsn,
    "-pg", $PostgresDsn,
    "-batch", $BatchSize
)

if ($Truncate) {
    $args += "-truncate"
}

if ($DryRun) {
    $args += "-dry-run"
}

if ($Tables -ne "") {
    $args += "-tables"
    $args += $Tables
}

# Run migration
Write-Host ""
Write-Host "Starting migration..." -ForegroundColor Green
Write-Host "MySQL: $MysqlDsn" -ForegroundColor Gray
Write-Host "PostgreSQL: $PostgresDsn" -ForegroundColor Gray
Write-Host ""

& "$ScriptDir\migrate.exe" @args

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "======================================" -ForegroundColor Green
    Write-Host "  Migration completed successfully!" -ForegroundColor Green
    Write-Host "======================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Yellow
    Write-Host "1. Update your .env file with PostgreSQL DSN:" -ForegroundColor White
    Write-Host "   SQL_DSN=$PostgresDsn" -ForegroundColor Gray
    Write-Host ""
    Write-Host "2. Reset sequences (run init_postgres.sql):" -ForegroundColor White
    Write-Host "   psql `"$PostgresDsn`" -f init_postgres.sql" -ForegroundColor Gray
    Write-Host ""
    Write-Host "3. Start new-api and verify:" -ForegroundColor White
    Write-Host "   ./new-api" -ForegroundColor Gray
} else {
    Write-Host ""
    Write-Host "Migration failed with errors" -ForegroundColor Red
    exit 1
}
