# Zitadel 管理员账户快速修复指南
# Quick Fix Guide for Zitadel Admin Account

**问题**: 使用 admin 账户登录提示"找不到用户"
**Problem**: "User not found" when logging in with admin account

---

## 🚀 快速修复步骤 / Quick Fix Steps

### 方法 A: 一键修复脚本（推荐）

**步骤1: 上传修复脚本到服务器**

在您的本地 Windows 机器上执行（PowerShell 或 Git Bash）：

```bash
# 使用 SCP 上传脚本到 K3s Master 节点
scp C:\Users\Administrator\Desktop\lurus\lurus-api\doc\zitadel-fix-script.sh root@cloud-ubuntu-1-16c32g:/root/

# 或者使用 Tailscale IP
scp C:\Users\Administrator\Desktop\lurus\lurus-api\doc\zitadel-fix-script.sh root@100.98.57.55:/root/
```

**步骤2: SSH 连接到服务器并执行脚本**

```bash
# SSH 连接
ssh root@cloud-ubuntu-1-16c32g
# 密码: Lurus@ops

# 赋予执行权限
chmod +x /root/zitadel-fix-script.sh

# 运行脚本
/root/zitadel-fix-script.sh
```

**步骤3: 根据菜单选择操作**

脚本会显示菜单，**推荐选择选项 3**（重新部署）:
```
选择操作:
  1) 直接进入 Pod Shell（手动操作）
  2) 使用 zitadel CLI 创建管理员（自动）
  3) 重新部署 Zitadel 并配置初始管理员（方案4）★ 推荐
  4) 查看完整日志
  5) 退出

请选择 [1-5]: 3
```

**步骤4: 确认重新部署**

当提示确认时，输入 `yes`:
```
警告: 此操作将删除现有 Zitadel 数据!
确认继续? (yes/no): yes
```

**步骤5: 等待部署完成**

脚本会自动执行：
- ✅ 缩容 Zitadel
- ✅ 创建初始管理员配置
- ✅ 重新扩容 Zitadel
- ✅ 等待 Pod 就绪
- ✅ 显示初始化日志

**步骤6: 登录测试**

访问 https://auth.lurus.cn 并使用以下账户登录：
- **用户名/邮箱**: `admin@lurus.cn`
- **密码**: `Lurus@ops2026`

---

### 方法 B: 手动修复（如果脚本失败）

如果自动脚本失败，可以手动执行以下命令：

```bash
# SSH 连接到 K3s Master
ssh root@cloud-ubuntu-1-16c32g

# 1. 查看当前 Zitadel 状态
kubectl get pods -n lurus-identity

# 2. 查看日志查找问题
kubectl logs -n lurus-identity -l app=zitadel --tail=100

# 3. 缩容 Zitadel
kubectl scale deployment -n lurus-identity zitadel --replicas=0

# 4. 创建初始配置
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: zitadel-init-config
  namespace: lurus-identity
data:
  init-config.yaml: |
    FirstInstance:
      Org:
        Human:
          UserName: admin
          Email: admin@lurus.cn
          FirstName: Admin
          LastName: User
          Password: Lurus@ops2026
        Name: Lurus Platform
EOF

# 5. 重新扩容
kubectl scale deployment -n lurus-identity zitadel --replicas=1

# 6. 等待 Pod 就绪
kubectl wait --for=condition=ready pod -n lurus-identity -l app=zitadel --timeout=120s

# 7. 查看初始化日志
kubectl logs -n lurus-identity -l app=zitadel -f
```

---

## 📋 创建的管理员账户信息

修复完成后，使用以下账户登录 Zitadel:

| 字段 / Field | 值 / Value |
|--------------|-----------|
| **登录URL** | https://auth.lurus.cn |
| **用户名** | admin |
| **邮箱** | admin@lurus.cn |
| **密码** | Lurus@ops2026 |
| **组织名称** | Lurus Platform |

**重要提示**:
- 登录时可以使用 `admin` 或 `admin@lurus.cn` 作为用户名
- 首次登录后建议立即修改密码
- 这个账户拥有完整的平台管理权限

---

## 🔍 故障排查 / Troubleshooting

### 问题1: 脚本上传失败

```bash
# 如果 scp 命令失败，可以手动复制内容
# 1. 在本地打开脚本文件
# 2. 复制全部内容
# 3. SSH 到服务器后创建文件
ssh root@cloud-ubuntu-1-16c32g
cat > /root/zitadel-fix-script.sh << 'SCRIPT_END'
[粘贴脚本内容]
SCRIPT_END
```

### 问题2: Pod 一直无法就绪

```bash
# 查看 Pod 详细信息
kubectl describe pod -n lurus-identity -l app=zitadel

# 查看事件
kubectl get events -n lurus-identity --sort-by='.lastTimestamp'

# 检查数据库连接
kubectl logs -n lurus-identity -l app=zitadel | grep -i database
```

### 问题3: 登录仍提示"找不到用户"

可能的原因和解决方案：

**原因1: 初始化尚未完成**
```bash
# 查看日志确认是否完成初始化
kubectl logs -n lurus-identity -l app=zitadel | grep -i "setup complete\|ready"
```

**原因2: 使用了错误的登录格式**

尝试以下登录方式：
- ✅ `admin@lurus.cn` （推荐）
- ✅ `admin`
- ❌ `admin@zitadel.localhost` （旧格式）

**原因3: 数据库未正确清空**

如果之前有旧数据，需要手动清空数据库：
```bash
# 连接到数据库
ssh root@cloud-ubuntu-2-4c8g
sudo -u postgres psql

# 查看 Zitadel 数据库
\l

# 如果有 zitadel 数据库，删除并重建
DROP DATABASE IF EXISTS zitadel;
CREATE DATABASE zitadel;

# 退出
\q

# 然后回到 master 节点重新部署 Zitadel
```

### 问题4: Zitadel Pod 启动失败

```bash
# 检查数据库连接配置
kubectl get deployment -n lurus-identity zitadel -o yaml | grep -A 10 -i database

# 检查是否有必要的 Secret
kubectl get secret -n lurus-identity

# 测试数据库连接
kubectl run test-db --rm -it --image=postgres:15 -- psql "postgres://lurus:Lurus@ops@100.94.177.10:30543/lurus"
```

---

## 📝 验证修复成功

修复完成后，执行以下验证步骤：

### 1. 访问 Zitadel 控制台
```
打开浏览器访问: https://auth.lurus.cn
```

### 2. 登录测试
- 用户名: `admin@lurus.cn`
- 密码: `Lurus@ops2026`

### 3. 验证管理权限
登录后应该能看到：
- ✅ Organizations 菜单
- ✅ Projects 菜单
- ✅ Users 管理
- ✅ Settings 配置

### 4. 创建测试 Organization
按照 `doc/zitadel-setup-guide.md` 继续配置：
1. 创建 Organization "Lurus Platform"
2. 创建 Project "lurus-api"
3. 配置 OIDC Application
4. 获取 Client ID 和 Client Secret

---

## ⚡ 一键命令（复制粘贴）

如果您想快速执行，可以直接复制以下命令块：

```bash
# 在本地 PowerShell/Git Bash 执行（上传脚本）
scp C:\Users\Administrator\Desktop\lurus\lurus-api\doc\zitadel-fix-script.sh root@cloud-ubuntu-1-16c32g:/root/

# SSH 到服务器并执行
ssh root@cloud-ubuntu-1-16c32g "chmod +x /root/zitadel-fix-script.sh && /root/zitadel-fix-script.sh"
```

或者完全手动执行：

```bash
# SSH 连接
ssh root@cloud-ubuntu-1-16c32g

# 一键重新部署（包含初始管理员配置）
kubectl scale deployment -n lurus-identity zitadel --replicas=0 && \
cat <<EOF | kubectl apply -f - && \
apiVersion: v1
kind: ConfigMap
metadata:
  name: zitadel-init-config
  namespace: lurus-identity
data:
  init-config.yaml: |
    FirstInstance:
      Org:
        Human:
          UserName: admin
          Email: admin@lurus.cn
          FirstName: Admin
          LastName: User
          Password: Lurus@ops2026
        Name: Lurus Platform
EOF
sleep 5 && \
kubectl scale deployment -n lurus-identity zitadel --replicas=1 && \
kubectl wait --for=condition=ready pod -n lurus-identity -l app=zitadel --timeout=120s && \
kubectl logs -n lurus-identity -l app=zitadel --tail=50
```

---

## 📞 需要帮助？

如果修复后仍有问题，请提供以下信息：

1. **Pod 状态**:
   ```bash
   kubectl get pods -n lurus-identity
   ```

2. **最近日志**:
   ```bash
   kubectl logs -n lurus-identity -l app=zitadel --tail=100
   ```

3. **浏览器访问截图**: https://auth.lurus.cn 的页面

4. **错误信息**: 登录时的具体错误提示

---

**创建日期**: 2026-01-25
**最后更新**: 2026-01-25
