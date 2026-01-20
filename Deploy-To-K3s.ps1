# Lurus API 自动部署脚本 (PowerShell)
# Wait for GitHub Actions build and deploy to K3s cluster

param(
    [switch]$SkipBuildCheck = $false
)

$ErrorActionPreference = "Stop"

# Configuration
$REPO = "hanmahong5-arch/lurus-api"
$K3S_MASTER = "root@100.98.57.55"
$NAMESPACE = "lurus-system"
$DEPLOYMENT = "lurus-api"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  Lurus API Auto Deploy Script" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# Function: Check GitHub Actions status
function Get-BuildStatus {
    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$REPO/actions/runs?per_page=1" -Method Get
        $run = $response.workflow_runs[0]
        return @{
            Status = $run.status
            Conclusion = $run.conclusion
            HtmlUrl = $run.html_url
            CreatedAt = $run.created_at
        }
    }
    catch {
        Write-Host "Warning: Failed to fetch build status" -ForegroundColor Yellow
        return $null
    }
}

# Step 1: Wait for GitHub Actions build
if (-not $SkipBuildCheck) {
    Write-Host "[1/4] Waiting for GitHub Actions build..." -ForegroundColor Green
    Write-Host "      View details: https://github.com/$REPO/actions" -ForegroundColor Gray
    Write-Host ""

    $maxWait = 600  # 10 minutes
    $waitTime = 0
    $interval = 15

    while ($waitTime -lt $maxWait) {
        $buildInfo = Get-BuildStatus

        if ($buildInfo) {
            if ($buildInfo.Status -eq "completed") {
                if ($buildInfo.Conclusion -eq "success") {
                    Write-Host "✅ Build succeeded!" -ForegroundColor Green
                    break
                }
                else {
                    Write-Host "❌ Build failed: $($buildInfo.Conclusion)" -ForegroundColor Red
                    Write-Host "   Check: $($buildInfo.HtmlUrl)" -ForegroundColor Yellow
                    exit 1
                }
            }
            else {
                Write-Host "⏳ Building... (waited ${waitTime}s, status: $($buildInfo.Status))" -ForegroundColor Yellow
            }
        }

        Start-Sleep -Seconds $interval
        $waitTime += $interval
    }

    if ($waitTime -ge $maxWait) {
        Write-Host "⏱️  Timeout, but you can continue deployment manually" -ForegroundColor Yellow
        $continue = Read-Host "Continue deployment? (y/n)"
        if ($continue -ne "y") {
            exit 1
        }
    }

    Write-Host ""
}
else {
    Write-Host "[1/4] Skipped build check (use -SkipBuildCheck)" -ForegroundColor Yellow
    Write-Host ""
}

# Step 2: Check K3s cluster connection
Write-Host "[2/4] Checking K3s cluster connection..." -ForegroundColor Green
try {
    $null = ssh -o ConnectTimeout=5 $K3S_MASTER "kubectl version --client"
    Write-Host "✅ Cluster connection OK" -ForegroundColor Green
}
catch {
    Write-Host "❌ Cannot connect to K3s cluster" -ForegroundColor Red
    exit 1
}

Write-Host ""

# Step 3: Show current deployment status
Write-Host "[3/4] Current deployment status:" -ForegroundColor Green
ssh $K3S_MASTER "kubectl get deployment $DEPLOYMENT -n $NAMESPACE -o wide"
Write-Host ""
ssh $K3S_MASTER "kubectl get pods -n $NAMESPACE -l app=$DEPLOYMENT -o wide"
Write-Host ""

# Step 4: Restart Deployment
Write-Host "[4/4] Restarting deployment to pull new image..." -ForegroundColor Green
ssh $K3S_MASTER "kubectl rollout restart deployment/$DEPLOYMENT -n $NAMESPACE"

Write-Host ""
Write-Host "⏳ Waiting for rollout to complete..." -ForegroundColor Yellow
ssh $K3S_MASTER "kubectl rollout status deployment/$DEPLOYMENT -n $NAMESPACE"

Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  ✅ Deployment Complete!" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# Show new pod status
Write-Host "New pod status:" -ForegroundColor Green
ssh $K3S_MASTER "kubectl get pods -n $NAMESPACE -l app=$DEPLOYMENT -o wide"
Write-Host ""

# Show recent logs
Write-Host "Recent logs (last 20 lines):" -ForegroundColor Green
$podName = (ssh $K3S_MASTER "kubectl get pods -n $NAMESPACE -l app=$DEPLOYMENT -o jsonpath='{.items[0].metadata.name}'").Trim()
ssh $K3S_MASTER "kubectl logs -n $NAMESPACE $podName --tail=20"

Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  Access URL: https://api.lurus.cn" -ForegroundColor White
Write-Host "  Monitoring: https://grafana.lurus.cn" -ForegroundColor White
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# Verify image version
Write-Host "Current image version:" -ForegroundColor Green
ssh $K3S_MASTER "kubectl describe deployment $DEPLOYMENT -n $NAMESPACE | grep 'Image:'"
