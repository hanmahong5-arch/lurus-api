#!/bin/bash
# Lurus API 自动部署脚本
# 等待 GitHub Actions 构建完成后自动部署到 K3s 集群

set -e

# 配置
REPO="hanmahong5-arch/lurus-api"
K3S_MASTER="root@100.98.57.55"
NAMESPACE="lurus-system"
DEPLOYMENT="lurus-api"

echo "==========================================="
echo "  Lurus API 自动部署脚本"
echo "==========================================="
echo ""

# 函数：检查 GitHub Actions 状态
check_build_status() {
    local status=$(curl -s "https://api.github.com/repos/${REPO}/actions/runs?per_page=1" | grep -m1 '"status"' | cut -d'"' -f4)
    local conclusion=$(curl -s "https://api.github.com/repos/${REPO}/actions/runs?per_page=1" | grep -m1 '"conclusion"' | cut -d'"' -f4)

    echo "$status|$conclusion"
}

# 步骤 1: 等待 GitHub Actions 构建完成
echo "[1/4] 等待 GitHub Actions 构建完成..."
echo "      查看详情: https://github.com/${REPO}/actions"
echo ""

MAX_WAIT=600  # 最长等待 10 分钟
WAIT_TIME=0
INTERVAL=15

while [ $WAIT_TIME -lt $MAX_WAIT ]; do
    STATUS_INFO=$(check_build_status)
    STATUS=$(echo $STATUS_INFO | cut -d'|' -f1)
    CONCLUSION=$(echo $STATUS_INFO | cut -d'|' -f2)

    if [ "$STATUS" == "completed" ]; then
        if [ "$CONCLUSION" == "success" ]; then
            echo "✅ 构建成功！"
            break
        else
            echo "❌ 构建失败: $CONCLUSION"
            echo "   请检查: https://github.com/${REPO}/actions"
            exit 1
        fi
    else
        echo "⏳ 构建中... (已等待 ${WAIT_TIME}s)"
        sleep $INTERVAL
        WAIT_TIME=$((WAIT_TIME + INTERVAL))
    fi
done

if [ $WAIT_TIME -ge $MAX_WAIT ]; then
    echo "⏱️  等待超时，但您可以手动继续部署"
    read -p "是否继续部署? (y/n) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo ""

# 步骤 2: 检查 K3s 集群连接
echo "[2/4] 检查 K3s 集群连接..."
if ssh -o ConnectTimeout=5 $K3S_MASTER "kubectl version --client" > /dev/null 2>&1; then
    echo "✅ 集群连接正常"
else
    echo "❌ 无法连接到 K3s 集群"
    exit 1
fi

echo ""

# 步骤 3: 查看当前部署状态
echo "[3/4] 当前部署状态:"
ssh $K3S_MASTER "kubectl get deployment $DEPLOYMENT -n $NAMESPACE -o wide"
echo ""
ssh $K3S_MASTER "kubectl get pods -n $NAMESPACE -l app=$DEPLOYMENT -o wide"
echo ""

# 步骤 4: 重启 Deployment
echo "[4/4] 重启 Deployment 拉取新镜像..."
ssh $K3S_MASTER "kubectl rollout restart deployment/$DEPLOYMENT -n $NAMESPACE"

echo ""
echo "⏳ 等待滚动更新完成..."
ssh $K3S_MASTER "kubectl rollout status deployment/$DEPLOYMENT -n $NAMESPACE"

echo ""
echo "==========================================="
echo "  ✅ 部署完成！"
echo "==========================================="
echo ""

# 显示新 Pod 状态
echo "新 Pod 状态:"
ssh $K3S_MASTER "kubectl get pods -n $NAMESPACE -l app=$DEPLOYMENT -o wide"
echo ""

# 显示最近日志
echo "最近日志 (最后 20 行):"
POD_NAME=$(ssh $K3S_MASTER "kubectl get pods -n $NAMESPACE -l app=$DEPLOYMENT -o jsonpath='{.items[0].metadata.name}'")
ssh $K3S_MASTER "kubectl logs -n $NAMESPACE $POD_NAME --tail=20"

echo ""
echo "==========================================="
echo "  访问地址: https://api.lurus.cn"
echo "  监控地址: https://grafana.lurus.cn"
echo "==========================================="
echo ""

# 验证镜像版本
echo "当前镜像版本:"
ssh $K3S_MASTER "kubectl describe deployment $DEPLOYMENT -n $NAMESPACE | grep Image:"
