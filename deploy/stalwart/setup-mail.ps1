# Stalwart Mail Server Setup Script for Windows
# Lurus Switch Mail Infrastructure
#
# Usage:
#   .\setup-mail.ps1                    # Development mode
#   .\setup-mail.ps1 -Production        # Production with HTTPS
#   .\setup-mail.ps1 -Domain "yourdomain.com" -Production

param(
    [string]$Domain = "lurus.local",
    [switch]$Production,
    [switch]$Stop,
    [switch]$Status,
    [switch]$Logs
)

$ErrorActionPreference = "Stop"

# Colors
function Write-Info { Write-Host "[INFO] $args" -ForegroundColor Cyan }
function Write-Success { Write-Host "[OK] $args" -ForegroundColor Green }
function Write-Warn { Write-Host "[WARN] $args" -ForegroundColor Yellow }
function Write-Err { Write-Host "[ERROR] $args" -ForegroundColor Red }

# Navigate to deploy directory
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

if ($Status) {
    Write-Info "Checking Stalwart Mail Server status..."
    docker ps --filter "name=stalwart" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    exit 0
}

if ($Logs) {
    Write-Info "Showing Stalwart Mail Server logs..."
    docker logs -f stalwart-mail
    exit 0
}

if ($Stop) {
    Write-Info "Stopping Stalwart Mail Server..."
    if ($Production) {
        docker-compose -f docker-compose.mail.ssl.yml down
    } else {
        docker-compose -f docker-compose.mail.yml down
    }
    Write-Success "Stalwart Mail Server stopped."
    exit 0
}

# Set environment
$env:MAIL_DOMAIN = $Domain

Write-Info "============================================"
Write-Info "Stalwart Mail Server Setup"
Write-Info "============================================"
Write-Info "Domain: $Domain"
Write-Info "Mode: $(if ($Production) { 'Production (HTTPS)' } else { 'Development' })"
Write-Info "============================================"

# Create data directories
Write-Info "Creating data directories..."
New-Item -ItemType Directory -Force -Path ".\stalwart\data" | Out-Null
New-Item -ItemType Directory -Force -Path ".\stalwart\certs" | Out-Null

# Update config with domain
Write-Info "Updating configuration..."
$configPath = ".\stalwart\config.toml"
$config = Get-Content $configPath -Raw
$config = $config -replace 'hostname = "mail\.lurus\.local"', "hostname = `"mail.$Domain`""
$config = $config -replace 'next-hop = false', "next-hop = false`nhostname = `"mail.$Domain`""
Set-Content -Path $configPath -Value $config

if ($Production) {
    Write-Warn "Production mode requires proper DNS configuration:"
    Write-Host ""
    Write-Host "  Required DNS Records:" -ForegroundColor Yellow
    Write-Host "  ----------------------"
    Write-Host "  A     mail.$Domain      -> <YOUR_SERVER_IP>"
    Write-Host "  MX    $Domain           -> mail.$Domain (priority 10)"
    Write-Host "  TXT   $Domain           -> v=spf1 mx ~all"
    Write-Host "  TXT   _dmarc.$Domain    -> v=DMARC1; p=quarantine; rua=mailto:admin@$Domain"
    Write-Host ""

    $confirm = Read-Host "Have you configured DNS records? (y/N)"
    if ($confirm -ne "y" -and $confirm -ne "Y") {
        Write-Warn "Please configure DNS records first, then run this script again."
        exit 1
    }

    Write-Info "Starting Stalwart Mail Server (Production)..."
    docker-compose -f docker-compose.mail.ssl.yml up -d
} else {
    Write-Info "Starting Stalwart Mail Server (Development)..."
    docker-compose -f docker-compose.mail.yml up -d
}

# Wait for startup
Write-Info "Waiting for Stalwart to start..."
Start-Sleep -Seconds 10

# Health check
$maxRetries = 30
$retryCount = 0
while ($retryCount -lt $maxRetries) {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/healthz" -UseBasicParsing -TimeoutSec 5 -ErrorAction SilentlyContinue
        if ($response.StatusCode -eq 200) {
            Write-Success "Stalwart Mail Server is running!"
            break
        }
    } catch {
        $retryCount++
        Write-Host "." -NoNewline
        Start-Sleep -Seconds 2
    }
}

if ($retryCount -eq $maxRetries) {
    Write-Err "Stalwart failed to start. Check logs with: docker logs stalwart-mail"
    exit 1
}

Write-Host ""
Write-Success "============================================"
Write-Success "Stalwart Mail Server is ready!"
Write-Success "============================================"
Write-Host ""
Write-Host "  Web Admin:    http://localhost:8080" -ForegroundColor Cyan
Write-Host "  Admin Login:  admin / changeme" -ForegroundColor Cyan
Write-Host ""
Write-Host "  SMTP:         localhost:25 (MTA)"
Write-Host "                localhost:587 (Submission)"
Write-Host "                localhost:465 (SMTPS)"
Write-Host ""
Write-Host "  IMAP:         localhost:143 (STARTTLS)"
Write-Host "                localhost:993 (IMAPS)"
Write-Host ""
Write-Warn "IMPORTANT: Change the admin password immediately!"
Write-Host ""
