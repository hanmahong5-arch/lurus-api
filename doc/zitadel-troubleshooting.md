# Zitadel 故障排查与修复指南
# Zitadel Troubleshooting and Fix Guide

**问题 / Issue**: 使用 admin 账户登录时提示"找不到用户"
**Problem**: "User not found" when trying to login with admin account

---

## 诊断步骤 / Diagnostic Steps

### 1. 检查 Zitadel Pod 状态 / Check Zitadel Pod Status

```bash
# 在 K3s 服务器上执行 / Execute on K3s server
kubectl get pods -n lurus-identity

# 预期输出 / Expected output:
# NAME                       READY   STATUS    RESTARTS   AGE
# zitadel-xxxxxxxxxx-xxxxx   1/1     Running   0          Xd
```

**如果 Pod 不是 Running 状态 / If Pod is not Running:**
```bash
# 查看 Pod 详情
kubectl describe pod -n lurus-identity <pod-name>

# 查看 Pod 日志
kubectl logs -n lurus-identity <pod-name>
```

### 2. 检查 Zitadel 日志 / Check Zitadel Logs

```bash
# 查看最近 100 行日志
kubectl logs -n lurus-identity -l app=zitadel --tail=100

# 查看完整日志
kubectl logs -n lurus-identity -l app=zitadel

# 实时查看日志
kubectl logs -n lurus-identity -l app=zitadel -f
```

**关键信息查找 / Look for:**
- 初始化错误 / Initialization errors
- 数据库连接问题 / Database connection issues
- "default admin created" 或类似的初始化成功消息

### 3. 检查 Zitadel 配置 / Check Zitadel Configuration

```bash
# 查看 Zitadel Deployment 配置
kubectl get deployment -n lurus-identity zitadel -o yaml

# 查看 ConfigMap
kubectl get configmap -n lurus-identity

# 查看 Secret
kubectl get secret -n lurus-identity
```

### 4. 检查数据库连接 / Check Database Connection

```bash
# 查看 Zitadel 环境变量（包含数据库配置）
kubectl get deployment -n lurus-identity zitadel -o jsonpath='{.spec.template.spec.containers[0].env}' | jq
```

---

## 解决方案 / Solutions

### 方案 1: 检查默认管理员账户 / Check Default Admin Account

Zitadel 首次启动时应该自动创建默认管理员账户。默认账户信息可能在：

**可能的默认账户 / Possible default accounts:**
1. Username: `admin` / Email: `admin@zitadel.localhost`
2. Username: `zitadel-admin@zitadel.localhost`
3. 在初始化时配置的自定义账户

**查找默认账户信息 / Find default account info:**
```bash
# 查看 Zitadel 日志中的初始化信息
kubectl logs -n lurus-identity -l app=zitadel | grep -i "admin\|password\|created"

# 查看 Secret 中是否有初始密码
kubectl get secret -n lurus-identity -o yaml | grep -i password
```

### 方案 2: 重置 Zitadel 数据库（谨慎操作）/ Reset Zitadel Database (Use with caution)

**⚠️ 警告 / WARNING**: 此操作会删除所有 Zitadel 数据，仅在测试环境使用！
**⚠️ WARNING**: This will delete all Zitadel data, only use in test environment!

```bash
# 1. 缩容 Zitadel 到 0
kubectl scale deployment -n lurus-identity zitadel --replicas=0

# 2. 查看 Zitadel 使用的数据库
kubectl get deployment -n lurus-identity zitadel -o yaml | grep -i database

# 3. 如果使用独立的 PostgreSQL，连接并清空数据库
# (具体命令取决于数据库配置)

# 4. 重新扩容 Zitadel
kubectl scale deployment -n lurus-identity zitadel --replicas=1

# 5. 查看初始化日志
kubectl logs -n lurus-identity -l app=zitadel -f
```

### 方案 3: 使用 Zitadel CLI 创建管理员账户 / Create Admin with Zitadel CLI

```bash
# 进入 Zitadel Pod
kubectl exec -it -n lurus-identity <zitadel-pod-name> -- /bin/sh

# 在 Pod 内执行 Zitadel CLI 命令创建管理员
# (具体命令取决于 Zitadel 版本，请参考官方文档)
```

### 方案 4: 检查 Zitadel 版本和兼容性 / Check Zitadel Version

```bash
# 查看当前 Zitadel 版本
kubectl get deployment -n lurus-identity zitadel -o jsonpath='{.spec.template.spec.containers[0].image}'

# 输出示例: ghcr.io/zitadel/zitadel:v2.54.0
```

**如果版本较旧，考虑升级 / If version is old, consider upgrading:**
```bash
# 更新到最新稳定版本
kubectl set image deployment/zitadel -n lurus-identity \
  zitadel=ghcr.io/zitadel/zitadel:v2.67.1
```

### 方案 5: 访问 Zitadel 控制台并使用 Setup 流程 / Access Zitadel Console and Use Setup

1. **访问 Zitadel URL / Access Zitadel URL:**
   ```
   https://auth.lurus.cn
   ```

2. **查看是否有首次设置向导 / Look for first-time setup wizard:**
   - Zitadel 首次访问时可能显示设置向导
   - 按照向导创建第一个管理员账户

3. **使用 "Forgot Password" 功能 / Use "Forgot Password":**
   - 如果知道管理员邮箱，可以尝试重置密码
   - 需要配置 SMTP 才能收到重置邮件

---

## 推荐的操作流程 / Recommended Procedure

### Step 1: 检查 Zitadel 是否正常运行

```bash
# 在 K3s 服务器执行
kubectl get pods -n lurus-identity
kubectl logs -n lurus-identity -l app=zitadel --tail=50
```

### Step 2: 查找初始管理员信息

```bash
# 查找日志中的管理员创建信息
kubectl logs -n lurus-identity -l app=zitadel | grep -A 5 -B 5 -i "admin\|setup\|initial"

# 查找 Secret 中的初始密码
kubectl get secret -n lurus-identity -o yaml
```

### Step 3: 访问 Zitadel 控制台

1. 浏览器访问：https://auth.lurus.cn
2. 查看是否有首次设置页面
3. 如果有设置页面，按照指引创建管理员账户

### Step 4: 如果以上都不行，考虑重新部署 Zitadel

**创建新的 Zitadel 配置清单：**

```yaml
# zitadel-setup.yaml
apiVersion: v1
kind: Secret
metadata:
  name: zitadel-admin-sa
  namespace: lurus-identity
type: Opaque
stringData:
  zitadel-admin-sa.json: |
    {
      "type": "serviceaccount",
      "keyId": "initial-admin-key",
      "userId": "initial-admin-user"
    }
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: zitadel-config
  namespace: lurus-identity
data:
  config.yaml: |
    Log:
      Level: info

    FirstInstance:
      Org:
        Human:
          UserName: admin
          Password: Lurus@ops2026
        Machine:
          UserName: zitadel-admin-sa
          MachineKey:
            ExpirationDate: "2030-01-01T00:00:00Z"

    Database:
      # 根据实际数据库配置填写
      postgres:
        Host: your-db-host
        Port: 5432
        Database: zitadel
        User:
          Username: zitadel
          Password: your-password
        Admin:
          Username: postgres
          Password: admin-password
```

---

## 快速修复脚本 / Quick Fix Script

将以下脚本保存为 `fix-zitadel.sh` 并在 K3s 服务器上执行：

```bash
#!/bin/bash
# Zitadel 快速诊断和修复脚本

echo "========================================="
echo "Zitadel 诊断脚本 / Zitadel Diagnostic Script"
echo "========================================="

# 检查 Zitadel Pod 状态
echo -e "\n[1/5] 检查 Pod 状态..."
kubectl get pods -n lurus-identity

# 查看最近日志
echo -e "\n[2/5] 查看最近日志..."
kubectl logs -n lurus-identity -l app=zitadel --tail=20

# 查找初始管理员信息
echo -e "\n[3/5] 查找管理员信息..."
kubectl logs -n lurus-identity -l app=zitadel | grep -i "admin\|setup" | tail -10

# 检查数据库连接
echo -e "\n[4/5] 检查配置..."
kubectl get deployment -n lurus-identity zitadel -o jsonpath='{.spec.template.spec.containers[0].env}' | jq '.'

# 访问测试
echo -e "\n[5/5] 测试访问..."
curl -I https://auth.lurus.cn

echo -e "\n========================================="
echo "诊断完成 / Diagnostic Complete"
echo "========================================="
```

执行脚本：
```bash
chmod +x fix-zitadel.sh
./fix-zitadel.sh
```

---

## 常见问题 / FAQ

### Q1: 为什么找不到管理员用户？
**A1**: 可能原因：
- Zitadel 数据库未正确初始化
- 初始管理员账户创建失败
- 使用了错误的用户名或邮箱

### Q2: 可以手动创建管理员吗？
**A2**: 可以，但需要：
- 访问 Zitadel 数据库
- 或使用 Zitadel CLI
- 或重新部署 Zitadel 并配置 FirstInstance

### Q3: 重置 Zitadel 会影响其他服务吗？
**A3**: 如果 Zitadel 使用独立数据库，只会影响认证系统。但已登录的用户 session 会失效。

### Q4: 如何查看 Zitadel 的完整配置？
**A4**:
```bash
kubectl get deployment -n lurus-identity zitadel -o yaml > zitadel-deployment.yaml
```

---

## 联系支持 / Support

如果以上方法都无法解决问题，请提供以下信息：

1. Zitadel Pod 状态：`kubectl get pods -n lurus-identity`
2. Zitadel 日志：`kubectl logs -n lurus-identity -l app=zitadel --tail=100`
3. Zitadel 版本：`kubectl get deployment -n lurus-identity zitadel -o jsonpath='{.spec.template.spec.containers[0].image}'`
4. 访问 https://auth.lurus.cn 时看到的页面截图

---

**创建日期 / Created**: 2026-01-25
**最后更新 / Last Updated**: 2026-01-25
