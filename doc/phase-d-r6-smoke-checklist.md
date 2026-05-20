# Phase D R6 STAGE Smoke Checklist (2026-05-19)

> Switch Phase D push 后的 R6 STAGE 部署 + e2e smoke。read-only audit 完成于 2026-05-19；命令带 `[live]` 标的可直接执行。

## R6 当前状态

| 项 | 值 |
|---|---|
| Hub 部署方式 | **k3s** (namespace `lurus-newhub`, deployment `lurus-newhub`) |
| 当前运行 pod | `lurus-newhub-f66787dbf-j7mn9`(2026-05-13 02:45 UTC 启动, 6d1h 龄) |
| 当前运行 imageID | `ghcr.io/hanmahong5-arch/lurus-newhub@sha256:857bbe679f7d127adaaab9007406a997e5dbd7ce1181bfa91cd25dd7a7267858` |
| 配置的 image tag | `ghcr.io/hanmahong5-arch/lurus-newhub:main`(浮动 tag, `imagePullPolicy: Always`) |
| GHA 最新 build | `Publish Docker image (main branch)` run `26073233739` **success** at 2026-05-19 02:52 UTC(`:main` 已被覆盖到 `3d03eeeb` 产物) |
| 与最新 push `3d03eeeb` 的 delta | **pod 滞后**：image 已 push 到 ghcr，pod 没 rollout(创建于 5-13 早于 5-19 02:52 的 image push) |
| K8s service | `lurus-newhub` NodePort 8850→30850 |
| 容器 port | 3000(`/api/status` health) |
| Nginx site | `test-newhub.lurus.cn` → `127.0.0.1:30850`(端口 443/80) |
| Hub URL(测试用) | `https://test-newhub.lurus.cn` |
| **`LURUS_WHITELABEL_MASTER_SECRET`** | **UNSET**(deployment env 和 secret `lurus-newhub-secrets` 均无此 key) — **会阻塞 hmac-key endpoint** |
| 其他相关 env(已设) | `SESSION_SECRET`,`SQL_DSN`,`IDENTITY_SERVICE_INTERNAL_KEY`,`IDENTITY_SESSION_SECRET`,`IDENTITY_GRPC_ADDR`,`NATS_URL`,`ALLOWED_ORIGINS=https://test-newhub.lurus.cn` |

### 关键阻塞

1. **5 个验证 workflow FAIL on `3d03eeeb`**:
   - `Go CI` / `Web CI` / `Security Tests` / `Deploy to Staging` / `Build and Push Docker Image`(build 11s–32s 即失败)
   - 失败原因(已 grep log):
     - Go CI: `Repository path '/home/runner/work/lurus-newhub/shared/lurus-proto-go' is not under '/home/runner/work/lurus-newhub/lurus-newhub'` — workflow checkout 配置缺 `shared/lurus-proto-go` 兄弟 repo
     - Build and Push Docker Image: `failed to calculate checksum of ref ...: "/zita-sdk-go": not found` — Dockerfile 引用了不存在的 `/zita-sdk-go` 路径
   - **但** `Publish Docker image (main branch)` workflow **success** 5m38s — 这个 workflow 不含上面缺失的 SDK 依赖,产出的镜像可用
2. **`LURUS_WHITELABEL_MASTER_SECRET` 未配** — 没设置之前 hmac-key endpoint 一定 500
3. **Pod 没 rollout** — `:main` tag 已覆盖到 `3d03eeeb`,但现有 pod 仍跑 5-13 老镜像

## 部署 `3d03eeeb` 到 R6

### Step A — 在 secret 中加入 `LURUS_WHITELABEL_MASTER_SECRET`

**生成 32-byte hex secret(本地)**:

```bash
openssl rand -hex 32
# 例:abcd1234...64 字符 hex
```

**写入 k8s secret**(R6 上执行,会触发 secret 更新但**不**自动 rollout):

```bash
SECRET_VAL="<openssl rand -hex 32 的输出>"
ssh root@100.122.83.20 "kubectl patch secret -n lurus-newhub lurus-newhub-secrets --type=json -p='[{\"op\":\"add\",\"path\":\"/data/LURUS_WHITELABEL_MASTER_SECRET\",\"value\":\"'$(echo -n "$SECRET_VAL" | base64 -w0)'\"}]'"
```

**把 env 挂到 deployment**(env 段加一条 `valueFrom: secretKeyRef`):

```bash
ssh root@100.122.83.20 "kubectl patch deployment -n lurus-newhub lurus-newhub --type=json -p='[{\"op\":\"add\",\"path\":\"/spec/template/spec/containers/0/env/-\",\"value\":{\"name\":\"LURUS_WHITELABEL_MASTER_SECRET\",\"valueFrom\":{\"secretKeyRef\":{\"name\":\"lurus-newhub-secrets\",\"key\":\"LURUS_WHITELABEL_MASTER_SECRET\"}}}}]'"
```

> 注意:`kubectl patch deployment` 触发 rollout,会顺便拉新镜像。如果不想立刻 rollout,先用 `--dry-run=client -o yaml` 看 patch diff。

> 三铁律提醒:此处的 `kubectl patch` 仅用于临时验证;**长期方案**是改 GitOps manifest 源(无 ArgoCD app for newhub,需查 deploy 源在哪个 repo / 是否有 `2l-svc-platform/deploy/` 类似目录管 newhub manifest)再 `kubectl apply -f`。

### Step B — 触发 rollout 拉 `:main` 新 image

```bash
ssh root@100.122.83.20 "kubectl rollout restart deployment/lurus-newhub -n lurus-newhub"
ssh root@100.122.83.20 "kubectl rollout status deployment/lurus-newhub -n lurus-newhub --timeout=120s"
ssh root@100.122.83.20 "kubectl get pods -n lurus-newhub --sort-by=.metadata.creationTimestamp -o wide | tail -3"
```

### Step C — 验证新 pod 起的是新 image

```bash
ssh root@100.122.83.20 "kubectl describe pod -n lurus-newhub \$(kubectl get pods -n lurus-newhub -l app=lurus-newhub -o name | tail -1 | cut -d/ -f2) | grep -E '(Image ID:|Started:)'"
# 期望:Started > 2026-05-19 02:52 UTC,Image ID 不再是 sha256:857bbe...
```

### Step D — 看 startup log

```bash
ssh root@100.122.83.20 "kubectl logs -n lurus-newhub deployment/lurus-newhub --tail=80"
# 期望:含 'gin 启动' / 'listening on :3000' / 无 panic
```

### Step E — 验证 GHCR 上 `:main` 当前 digest

```bash
gh api /users/hanmahong5-arch/packages/container/lurus-newhub/versions --jq '.[0:3]' 2>&1
```

## E2E smoke 命令

部署完后按顺序在本地 Git Bash 跑(直接连 R6 域名,不走 Tailscale 直连)。

### 1. 健康检查 + endpoint 存活

```bash
HUB_URL="https://test-newhub.lurus.cn"

curl -sfk "$HUB_URL/api/status" | head -c 200; echo
# 期望:JSON,含 version / start_time

curl -sfk "$HUB_URL/api/v2/switch/tools/versions" | head -c 200; echo
# 期望:成功列出 claude/codex/gemini 等工具版本
```

### 2. Redeem(匿名,核心 endpoint)

```bash
# 用一个**真实未用激活码** — 从 Reseller 控制台先生成一个 test 码
TEST_CODE="<reseller 控制台新生成的激活码>"

curl -sk -X POST "$HUB_URL/api/v2/switch/redeem" \
  -H "Content-Type: application/json" \
  -d "{\"code\":\"$TEST_CODE\",\"fingerprint\":\"smoke-test-$(date +%s)\",\"app_version\":\"smoke\"}" | jq

# 预期成功:{"success":true,"data":{"user_token":"sk-...","user_id":...,"quota":...,"tenant_slug":"..."}}
# 已用:    {"success":false,"message":"该兑换码已被使用"}
# 404:     router 没注册 → 镜像还是 5-13 老的,回 Step B
```

### 3. Heartbeat(用上一步拿到的 user_token)

```bash
USER_TOKEN="<上一步返回的 user_token>"
TENANT_SLUG="<redeem 响应里的 tenant_slug>"

# Multi-tenant 路径
curl -sk -X POST "$HUB_URL/api/v2/$TENANT_SLUG/user/heartbeat" \
  -H "Authorization: $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"fingerprint\":\"smoke-test-$(date +%s)\",\"app_version\":\"smoke\"}" | jq

# Single-tenant fallback(不带 slug)
curl -sk -X POST "$HUB_URL/api/v2/switch/heartbeat" \
  -H "Authorization: $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"fingerprint\":\"smoke-test\",\"app_version\":\"smoke\"}" | jq

# 预期:{"success":true,"data":{"status":"active","quota":...}}
```

### 4. HMAC key(admin auth)

```bash
ADMIN_TOKEN="<Hub admin session token,从 Reseller 控制台浏览器 cookie session= 或 Network 抓 Authorization Bearer>"
TENANT_SLUG="<经销商 slug>"

curl -sfk "$HUB_URL/api/v2/admin/whitelabel/hmac-key?tenant_slug=$TENANT_SLUG" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq

# 预期:{"success":true,"data":{"hmac_key":"<64 字符 hex>"}}
# 500 → LURUS_WHITELABEL_MASTER_SECRET 没设(回 Step A)
# 401 → admin token 错 / 过期
# 404 → router 没注册
```

## Switch wails dev EndUser smoke

部署完 Hub 后,本地:

```bash
cd /c/Users/Anita/Desktop/lurus/2c-gui-switch
wails dev
```

GUI 步骤:

1. AppModeSelect → 选 **EndUser**
2. 首次输入白标包 Hub URL → 填 `https://test-newhub.lurus.cn`(或留 hub.lurus.cn 默认值,看 EndUser 包是否硬编码)
3. ActivationPage:输入测试激活码 → 按 Activate
4. 验证:成功 toast + 跳转 EndUserMainPage,显示 quota / token expires
5. 等 ~60s 看 heartbeat:dev 控制台 stdout 应有 `heartbeat status=active`
6. 关 wails → 重启 → 验证 token 持久化,不需要重新激活

## 失败排查

| 症状 | 可能原因 |
|---|---|
| Redeem 返回 404 | router 没注册 / pod 还是 5-13 老镜像 → kubectl rollout restart |
| Redeem 返回 500 connection refused | nginx 通,但 pod startup probe 没过 → kubectl logs 看 panic |
| Heartbeat 401 | user_token 写错 / token 已被禁用 / Authorization header 漏 `Bearer ` 前缀(rawToken 模式不需要) |
| HMAC key 500 | env `LURUS_WHITELABEL_MASTER_SECRET` 没设(回 Step A) |
| HMAC key 401 | admin token 不对 |
| Switch UI 启动白屏 | bun build 没跑;先 `wails build` 一次或 `cd frontend && bun install && bun run build` |
| wails dev EndUser 兑换 toast 失败但 hub log 显示成功 | 已知 Result.success silent-failure 类问题,看 wails dev 控制台 Result 字段 |

## 已知限制

- **CI 失败遗留**:Go CI / Docker Build workflow 仍 fail(checkout 缺 `shared/lurus-proto-go` 兄弟 repo + Dockerfile 引用不存在的 `/zita-sdk-go`)。`Publish Docker image (main branch)` workflow 不受影响,镜像可用。建议作为下一个 Sprint task 修
- **`*claw` (picoclaw/nullclaw/openclaw/zeroclaw)** 工具的 `buildLaunchArgs` 仅生成 nil args,TODO 待验证
- **claude/codex/gemini `--model` flag** 是常识假设,未实跑各 CLI `--help` 验证
- Switch ~186 个 pre-existing dirty 文件(Wave 0-6 / 41 页迁移产出)留在 working tree,Phase D 与之无关
- R6 `hub.lurus.cn` nginx site **未配**;CLAUDE.md 里 Personal 模式默认指向 `hub.lurus.cn`,实测要用 `test-newhub.lurus.cn`,或 R6 上加 nginx site
- 无 ArgoCD app for newhub(`kubectl get applications -A` 只有 lucrum-web);GitOps 源 manifest 位置未确认,本 checklist 用 `kubectl patch` 临时改 deployment;**长期方案** = 找到 newhub manifest 源 repo,把 `LURUS_WHITELABEL_MASTER_SECRET` 改进 git 后 `kubectl apply -f`
