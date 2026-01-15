# Windows Server HTTPS Setup Script
# 使用 Caddy + Let's Encrypt 免费 SSL 证书
#
# Usage: Run as Administrator
#   powershell -ExecutionPolicy Bypass -File setup-https.ps1

$ErrorActionPreference = "Stop"
$Domain = "api.lurus.cn"
$CaddyDir = "C:\Caddy"
$CaddyExe = "$CaddyDir\caddy.exe"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  HTTPS Setup for $Domain" -ForegroundColor Cyan
Write-Host "  Windows Server 2019 + Caddy" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# 1. Create Caddy directory
Write-Host "`n[1/5] Creating Caddy directory..." -ForegroundColor Yellow
if (!(Test-Path $CaddyDir)) {
    New-Item -ItemType Directory -Path $CaddyDir -Force | Out-Null
}

# 2. Download Caddy
Write-Host "`n[2/5] Downloading Caddy..." -ForegroundColor Yellow
if (!(Test-Path $CaddyExe)) {
    $CaddyUrl = "https://github.com/caddyserver/caddy/releases/download/v2.8.4/caddy_2.8.4_windows_amd64.zip"
    $ZipPath = "$CaddyDir\caddy.zip"

    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $CaddyUrl -OutFile $ZipPath
    Expand-Archive -Path $ZipPath -DestinationPath $CaddyDir -Force
    Remove-Item $ZipPath
    Write-Host "  Caddy downloaded successfully" -ForegroundColor Green
} else {
    Write-Host "  Caddy already exists" -ForegroundColor Green
}

# 3. Create Caddyfile
Write-Host "`n[3/5] Creating Caddyfile..." -ForegroundColor Yellow
$Caddyfile = @"
$Domain {
    reverse_proxy localhost:3000
    encode gzip
}
"@
$Caddyfile | Out-File -FilePath "$CaddyDir\Caddyfile" -Encoding UTF8
Write-Host "  Caddyfile created" -ForegroundColor Green

# 4. Open firewall port
Write-Host "`n[4/5] Configuring Windows Firewall..." -ForegroundColor Yellow
$ruleName = "Caddy HTTPS"
$existingRule = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
if (!$existingRule) {
    New-NetFirewallRule -DisplayName $ruleName -Direction Inbound -Protocol TCP -LocalPort 443 -Action Allow | Out-Null
    New-NetFirewallRule -DisplayName "Caddy HTTP" -Direction Inbound -Protocol TCP -LocalPort 80 -Action Allow | Out-Null
    Write-Host "  Firewall rules created" -ForegroundColor Green
} else {
    Write-Host "  Firewall rules already exist" -ForegroundColor Green
}

# 5. Install as Windows Service
Write-Host "`n[5/5] Installing Caddy as Windows Service..." -ForegroundColor Yellow

# Stop existing service if running
$service = Get-Service -Name "Caddy" -ErrorAction SilentlyContinue
if ($service) {
    Stop-Service -Name "Caddy" -Force -ErrorAction SilentlyContinue
    & sc.exe delete Caddy 2>$null
    Start-Sleep -Seconds 2
}

# Install service using sc.exe
$binPath = "`"$CaddyExe`" run --config `"$CaddyDir\Caddyfile`""
& sc.exe create Caddy binPath= $binPath start= auto
& sc.exe description Caddy "Caddy web server with automatic HTTPS"
Start-Service -Name "Caddy"

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "  Caddy is running and will automatically" -ForegroundColor White
Write-Host "  obtain SSL certificate from Let's Encrypt" -ForegroundColor White
Write-Host ""
Write-Host "  Test: curl https://$Domain/api/status" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Service management:" -ForegroundColor White
Write-Host "    Start:   Start-Service Caddy" -ForegroundColor Gray
Write-Host "    Stop:    Stop-Service Caddy" -ForegroundColor Gray
Write-Host "    Status:  Get-Service Caddy" -ForegroundColor Gray
Write-Host "    Logs:    $CaddyDir\caddy.log" -ForegroundColor Gray
Write-Host "========================================" -ForegroundColor Cyan
