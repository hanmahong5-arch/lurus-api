#!/bin/bash
# 自动配置 HTTPS 脚本 (Let's Encrypt 免费证书)
# 无需 Docker，直接安装 Caddy
#
# Usage: sudo bash setup-https.sh

set -e

DOMAIN="api.lurus.cn"

echo "========================================"
echo "  HTTPS Setup for $DOMAIN"
echo "  使用 Let's Encrypt 免费 SSL 证书"
echo "========================================"

# Detect OS
if [ -f /etc/debian_version ]; then
    OS="debian"
elif [ -f /etc/redhat-release ]; then
    OS="rhel"
else
    echo "Unsupported OS"
    exit 1
fi

echo ""
echo "[1/5] 安装 Caddy..."

if [ "$OS" = "debian" ]; then
    apt update
    apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
    apt update
    apt install -y caddy
else
    yum install -y yum-plugin-copr
    yum copr enable @caddy/caddy -y
    yum install -y caddy
fi

echo ""
echo "[2/5] 创建 Caddyfile..."

cat > /etc/caddy/Caddyfile << 'EOF'
api.lurus.cn {
    reverse_proxy localhost:3000
    encode gzip

    header {
        # CORS headers
        Access-Control-Allow-Origin *
        Access-Control-Allow-Methods "GET, POST, PUT, DELETE, OPTIONS"
        Access-Control-Allow-Headers "Authorization, Content-Type"
    }
}
EOF

echo ""
echo "[3/5] 开放防火墙端口..."

if command -v ufw &> /dev/null; then
    ufw allow 80/tcp
    ufw allow 443/tcp
    echo "UFW: 已开放 80/443 端口"
elif command -v firewall-cmd &> /dev/null; then
    firewall-cmd --permanent --add-service=http
    firewall-cmd --permanent --add-service=https
    firewall-cmd --reload
    echo "Firewalld: 已开放 80/443 端口"
else
    echo "未检测到防火墙，请手动确认端口开放"
fi

echo ""
echo "[4/5] 启动 Caddy 服务..."

systemctl daemon-reload
systemctl enable caddy
systemctl restart caddy

echo ""
echo "[5/5] 验证服务状态..."

sleep 3
systemctl status caddy --no-pager

echo ""
echo "========================================"
echo "  ✅ 配置完成！"
echo ""
echo "  Caddy 会自动从 Let's Encrypt 获取证书"
echo "  证书每 90 天自动续期"
echo ""
echo "  测试命令："
echo "  curl https://$DOMAIN/api/status"
echo ""
echo "  查看日志："
echo "  journalctl -u caddy -f"
echo "========================================"
