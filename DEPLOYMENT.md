# Lurus API 部署指南
# Lurus API Deployment Guide

## ✅ 已完成步骤 / Completed Steps

### 1. 代码更新 / Code Updates
- ✅ 所有代码从 new-api 重命名为 lurus-api (327+ 文件)
- ✅ 提交到 Git: commit `e1e1b7cf`
- ✅ 推送到 GitHub: https://github.com/hanmahong5-arch/lurus-api

### 2. GitHub Actions 自动构建 / Automatic Build
- ✅ GitHub Actions 工作流已配置 (`.github/workflows/build.yaml`)
- 🔄 **正在自动构建 Docker 镜像中...**
- 📦 镜像将推送到: `ghcr.io/hanmahong5-arch/lurus-api:latest`

---

## 📋 接下来的步骤 / Next Steps

### 步骤 1: 检查 GitHub Actions 构建状态

访问: https://github.com/hanmahong5-arch/lurus-api/actions

等待构建完成（通常需要 5-10 分钟）。确认：
- ✅ "Build and Push Docker Image" 工作流成功
- ✅ 镜像已推送到 ghcr.io

### 步骤 2: 连接到 K3s 集群

如果使用远程 K3s 集群，在**集群节点**上执行以下命令：

```bash
# SSH 到 K3s 集群节点
ssh user@your-k3s-server

# 验证 kubectl 可用
kubectl version

# 检查命名空间
kubectl get namespace lurus-system
```

### 步骤 3: 更新 K8s 部署

有两种方式更新部署：

#### 方式 A: 使用 ArgoCD 同步 (推荐)

如果使用 ArgoCD 管理部署：

```bash
# 通过 ArgoCD UI 或 CLI 触发同步
argocd app sync lurus-api

# 或者通过 ArgoCD UI 手动点击 "Sync" 按钮
# 访问: https://your-argocd-url/applications/lurus-api
```

#### 方式 B: 手动重启 Pod

强制 Pod 拉取最新镜像：

```bash
# 进入项目目录
cd /path/to/lurus-api

# 方式 1: 使用 kubectl rollout restart
kubectl rollout restart deployment/lurus-api -n lurus-system

# 方式 2: 删除 Pod 让它自动重建
kubectl delete pod -l app=lurus-api -n lurus-system

# 方式 3: 使用 kustomize 应用配置
kubectl apply -k deploy/k8s/
```

### 步骤 4: 验证部署

```bash
# 查看 Pod 状态
kubectl get pods -n lurus-system -l app=lurus-api

# 查看 Pod 日志
kubectl logs -n lurus-system -l app=lurus-api --tail=100 -f

# 检查部署状态
kubectl rollout status deployment/lurus-api -n lurus-system

# 验证新镜像
kubectl describe pod -n lurus-system -l app=lurus-api | grep Image:
# 应该显示: ghcr.io/hanmahong5-arch/lurus-api:latest
```

### 步骤 5: 测试服务

```bash
# 获取服务端点
kubectl get svc -n lurus-system lurus-api
kubectl get ingress -n lurus-system

# 测试 API 端点
curl https://your-domain/api/status

# 测试 Meilisearch 搜索功能
curl -X GET "https://your-domain/api/log/search?keyword=test" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## 🔧 故障排查 / Troubleshooting

### 问题 1: Pod 无法启动

```bash
# 检查 Pod 事件
kubectl describe pod -n lurus-system -l app=lurus-api

# 常见问题：
# - ImagePullBackOff: 检查镜像是否构建成功
# - CrashLoopBackOff: 检查日志和环境变量配置
```

### 问题 2: 镜像拉取失败

```bash
# 验证镜像存在
docker pull ghcr.io/hanmahong5-arch/lurus-api:latest

# 检查 imagePullSecrets（如果仓库是私有的）
kubectl get secret -n lurus-system
```

### 问题 3: 数据库连接失败

```bash
# 检查 Secret 配置
kubectl get secret lurus-api-secrets -n lurus-system -o yaml

# 验证 SQL_DSN 配置正确
kubectl describe deployment lurus-api -n lurus-system
```

---

## 📊 监控和验证 / Monitoring

### 查看部署信息

```bash
# 完整部署信息
kubectl get all -n lurus-system -l app=lurus-api

# 查看资源使用情况
kubectl top pod -n lurus-system -l app=lurus-api
```

### 验证 Meilisearch 集成

```bash
# 进入 Pod
kubectl exec -it -n lurus-system deployment/lurus-api -- /bin/sh

# 检查环境变量
env | grep MEILISEARCH

# 测试 Meilisearch 连接（如果部署了 Meilisearch）
curl http://meilisearch:7700/health
```

---

## 🚀 快速部署命令 / Quick Deploy Commands

如果一切配置正确，在 K3s 集群节点上执行：

```bash
# 一键更新部署
cd /path/to/lurus-api && \
kubectl rollout restart deployment/lurus-api -n lurus-system && \
kubectl rollout status deployment/lurus-api -n lurus-system && \
kubectl logs -n lurus-system -l app=lurus-api --tail=50 -f
```

---

## 📝 版本信息 / Version Info

- **Commit**: e1e1b7cf
- **版本**: v1.1.0 (with Meilisearch integration)
- **镜像**: ghcr.io/hanmahong5-arch/lurus-api:latest
- **K8s 命名空间**: lurus-system
- **部署名称**: lurus-api

---

## 🔗 相关链接 / Related Links

- GitHub 仓库: https://github.com/hanmahong5-arch/lurus-api
- GitHub Actions: https://github.com/hanmahong5-arch/lurus-api/actions
- 容器镜像: https://github.com/hanmahong5-arch/lurus-api/pkgs/container/lurus-api

---

**部署时间**: 2026-01-20
**部署者**: Administrator
