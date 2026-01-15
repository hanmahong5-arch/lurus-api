# DeepSeek Connection Diagnosis Script
# DeepSeek 连接诊断脚本
# Usage: .\diagnose-deepseek.ps1

$ErrorActionPreference = "Continue"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  DeepSeek Connection Diagnosis Tool" -ForegroundColor Cyan
Write-Host "  DeepSeek 连接诊断工具" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. Check api.lurus.cn connectivity
Write-Host "[1/5] Checking api.lurus.cn connectivity..." -ForegroundColor Yellow
Write-Host "      检查 api.lurus.cn 连通性..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "http://api.lurus.cn/api/status" -TimeoutSec 10 -UseBasicParsing
    if ($response.StatusCode -eq 200) {
        Write-Host "      ✅ api.lurus.cn is reachable (HTTP $($response.StatusCode))" -ForegroundColor Green
    } else {
        Write-Host "      ⚠️  api.lurus.cn returned HTTP $($response.StatusCode)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "      ❌ Cannot connect to api.lurus.cn: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# 2. Check DeepSeek API connectivity
Write-Host "[2/5] Checking DeepSeek API connectivity..." -ForegroundColor Yellow
Write-Host "      检查 DeepSeek API 连通性..." -ForegroundColor Yellow
try {
    $startTime = Get-Date
    $response = Invoke-WebRequest -Uri "https://api.deepseek.com/v1/models" -TimeoutSec 30 -UseBasicParsing -ErrorAction SilentlyContinue
    $endTime = Get-Date
    $duration = ($endTime - $startTime).TotalSeconds
    Write-Host "      ✅ DeepSeek API reachable (${duration}s)" -ForegroundColor Green
} catch {
    if ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "      ✅ DeepSeek API reachable (401 = needs auth, but network OK)" -ForegroundColor Green
    } else {
        Write-Host "      ❌ DeepSeek API error: $($_.Exception.Message)" -ForegroundColor Red
    }
}
Write-Host ""

# 3. DNS resolution check
Write-Host "[3/5] Checking DNS resolution..." -ForegroundColor Yellow
Write-Host "      检查 DNS 解析..." -ForegroundColor Yellow
try {
    $dns1 = Resolve-DnsName -Name "api.lurus.cn" -ErrorAction SilentlyContinue
    $dns2 = Resolve-DnsName -Name "api.deepseek.com" -ErrorAction SilentlyContinue

    if ($dns1) {
        Write-Host "      ✅ api.lurus.cn -> $($dns1.IPAddress -join ', ')" -ForegroundColor Green
    } else {
        Write-Host "      ❌ Cannot resolve api.lurus.cn" -ForegroundColor Red
    }

    if ($dns2) {
        Write-Host "      ✅ api.deepseek.com -> $($dns2.IPAddress -join ', ')" -ForegroundColor Green
    } else {
        Write-Host "      ⚠️  Cannot resolve api.deepseek.com (may need proxy)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "      ⚠️  DNS check failed: $($_.Exception.Message)" -ForegroundColor Yellow
}
Write-Host ""

# 4. Test API endpoint
Write-Host "[4/5] Testing chat completions endpoint..." -ForegroundColor Yellow
Write-Host "      测试聊天补全端点..." -ForegroundColor Yellow
Write-Host "      Please provide your API key (or press Enter to skip):" -ForegroundColor Cyan
$apiKey = Read-Host "      API Key"

if ($apiKey) {
    $body = @{
        model = "deepseek-chat"
        messages = @(
            @{
                role = "user"
                content = "Say 'test ok' in 2 words"
            }
        )
        max_tokens = 10
    } | ConvertTo-Json -Depth 3

    try {
        $headers = @{
            "Authorization" = "Bearer $apiKey"
            "Content-Type" = "application/json"
        }

        $startTime = Get-Date
        $response = Invoke-WebRequest -Uri "http://api.lurus.cn/v1/chat/completions" `
            -Method POST `
            -Headers $headers `
            -Body $body `
            -TimeoutSec 60 `
            -UseBasicParsing
        $endTime = Get-Date
        $duration = ($endTime - $startTime).TotalSeconds

        Write-Host "      ✅ API call successful (${duration}s)" -ForegroundColor Green
        Write-Host "      Response: $($response.Content.Substring(0, [Math]::Min(200, $response.Content.Length)))..." -ForegroundColor Gray
    } catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        Write-Host "      ❌ API call failed (HTTP $statusCode): $($_.Exception.Message)" -ForegroundColor Red

        if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $responseBody = $reader.ReadToEnd()
            Write-Host "      Error details: $responseBody" -ForegroundColor Red
        }
    }
} else {
    Write-Host "      ⏭️  Skipped (no API key provided)" -ForegroundColor Gray
}
Write-Host ""

# 5. Summary and recommendations
Write-Host "[5/5] Diagnosis Summary / 诊断摘要" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "If you're experiencing connection timeout, check:" -ForegroundColor White
Write-Host "如果遇到连接超时，请检查：" -ForegroundColor White
Write-Host ""
Write-Host "  1. Channel Configuration (渠道配置):" -ForegroundColor Cyan
Write-Host "     - Login to http://api.lurus.cn/admin" -ForegroundColor Gray
Write-Host "     - Check if DeepSeek channel is enabled" -ForegroundColor Gray
Write-Host "     - Verify API Key is valid" -ForegroundColor Gray
Write-Host "     - Ensure 'deepseek-chat' is in model list" -ForegroundColor Gray
Write-Host ""
Write-Host "  2. Request Format (请求格式):" -ForegroundColor Cyan
Write-Host "     - Use correct endpoint: /v1/chat/completions" -ForegroundColor Gray
Write-Host "     - Include Authorization header" -ForegroundColor Gray
Write-Host "     - Model name: deepseek-chat" -ForegroundColor Gray
Write-Host ""
Write-Host "  3. Environment Variables (环境变量):" -ForegroundColor Cyan
Write-Host "     - RELAY_TIMEOUT=120 (seconds)" -ForegroundColor Gray
Write-Host "     - STREAMING_TIMEOUT=300 (seconds)" -ForegroundColor Gray
Write-Host ""
Write-Host "  4. Proxy Settings (代理设置):" -ForegroundColor Cyan
Write-Host "     - If server cannot reach api.deepseek.com directly" -ForegroundColor Gray
Write-Host "     - Configure proxy in channel settings" -ForegroundColor Gray
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
