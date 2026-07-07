# Newhub 本地部署 + Playwright E2E 全功能验证报告

**日期**: 2026-06-20  **分支**: `fix/pg-baseline-gaps-021-2026-06-18` (HEAD `c9200b4d`) + 未提交的 OIDC 重构工作树
**结论**: 本地可测面 **14/14 通过 + 1 显式 skip**（E4 需真 LLM 流量）；后端 `go test -short` 全绿；前端 `bun run build` 通过。**1 个安全相关真问题**（#33 未并入本分支 → 渠道密钥无安全验证即可读取）。其余"失败"全部归因为测试 drift 或外部依赖，已修复或显式标注。

---

## 1. 怎么跑起来的（环境实况，非计划假设）

- **本机 Docker + WSL 不可同时运行**（owner 确认的机器约束）；Docker Desktop 的 WSL2 后端起不来（dockerd 不启动、wsl.exe 卡死）。
- **绕过方案**：newhub 运行期只硬依赖 **PostgreSQL**（Redis 在 `REDIS_CONN_STRING` 为空时自动禁用 → cookie session）。遂在 **R6 起一个一次性 `postgres:15` 容器**（`/data` 挂载，合规 R6 磁盘铁律），SSH 隧道到本机 `:5499`，本机 Go 服务直连。Go 后端 + 前端 + Playwright 全部本机跑。
- 运行拓扑：
  - Go 后端 `:8012`（host，`go run ./cmd/server`）→ 隧道 → R6 `newhub-e2e-pg`
  - Vite 前端 `:5173`（host，代理 `/api`→`:8012`）
  - Playwright + Chromium（host，已装）

**复跑命令**（栈起好后，在 `web/`）：
```bash
E2E_BASE_URL=http://localhost:5173 \
E2E_BACKEND_URL=http://localhost:8012 \
E2E_BRIDGE_TOKEN=<.env 里的 E2E_BRIDGE_TOKEN> \
E2E_TENANT_SLUG=lurus E2E_USER_ID=1 \
bunx playwright test
```

**Phase-1 校验全过**：`/api/status`→200；`schema_migrations` 最高=**21**；租户 `default | slug=lurus | enabled`（migration 021 §4 种子）；bridge 登录→`/console/v2/dashboard` 可达。

---

## 2. 三分类结果

### A. 本地实测通过（E2E + API，真实证据）

| 能力 | 验证点 | 结果 |
|------|--------|------|
| Bridge 登录 | POST `/api/v2/bridge/exchange?token=&user_id=1` → 200 + session cookie；PrivateRoute 放行 dashboard | ✅ |
| Token CRUD | create/list/delete（API）+ token 页 UI 可见 | ✅ |
| Token 跨重登持久 | create → clearCookies → 重登 → token 仍在（story-11-2 DoD） | ✅ |
| Channel CRUD | create/get/update/list/delete（v2，AdminAuth） | ✅ |
| Redemption CRUD | create/list/delete（v2） | ✅ |
| Credit-pool | create/get/usage（singleton；topup 见 C 类） | ✅ |
| 只读投影 | models / pricing / logs / admin tenants / admin users 形状校验 | ✅ |
| Settings | PUT `/user/me` 改 display_name → GET 回读一致 | ✅ |
| RBAC（正向） | root session 同时可达 UserAuth + RootJWTAuth 路由（RootJWTAuth 也认 session cookie） | ✅ |
| Token scope 拒绝 | chat-scoped token 调 `/v1/embeddings` → **403** + `auth.scope_rejected` 审计行 | ✅ |
| Audit taxonomy/export | `/api/v2/admin/audit/actions`（51 个动作）+ CSV/JSON 导出 round-trip + 非法 format→400 | ✅ |
| Relay 网关 | 无 token / 错 token → **401** | ✅ |
| Credit-pool topup 边界 | 显式暴露"无 platform wallet"边界（不 mock 假成功） | ✅ |

后端：`go test -short ./...` → **exit 0**（全 ok）。前端：`bun run build` → **exit 0**。

### B. 仅 STAGE / 真 key 可验（本地显式 skip 或标注，未造假）

| 能力 | 为什么本地测不了 |
|------|------------------|
| LLM relay 200 happy-path | 需真 provider key + 出网 + 已配 channel（本地无 channel → 400） |
| billing.quota_threshold 审计行 | 需真 LLM 流量跨越配额档位（story-e4 显式 skip，仅断言 taxonomy 存在） |
| credit-pool **topup** | ~~需 platform wallet~~ → ✅ **STAGE PASS 2026-06-25**（见下方 §B-STAGE） |
| 真 OIDC 登录 UI | 走 platform 身份（本地 ZitaClient=nil，zita 路由不注册） |
| billing/topup 支付、SEAM S1 付费→pool→relay 全链 | ✅ **STAGE PASS 2026-06-25**（internal-credit 充钱包 + 真 DeepSeek relay；沙盒支付腿仍缺 provider，见下方）|
| LLM relay 200 happy-path | ✅ **STAGE PASS 2026-06-25**（DeepSeek channel，真 200 + 扣费）|
| Midjourney / Flows / Meilisearch 搜索 | 需外部服务 |
| RBAC 反向（非 root 被拒 admin） | 需第二个非 root 用户；本 build admin 建用户为 deferred |

#### §B-STAGE — SEAM S1 整链 R6 STAGE 真实跑绿（2026-06-25,无 mock/skip）

环境：R6 自有 k3s(`ssh root@100.122.83.20`),newhub ns `lurus-newhub`(image `:main` 含 PR #34 idem 修复,digest sha256:2633078),platform-core ns `lurus-platform`(NodePort 30104/30105)。actor=root(id=1,`lurus_account_id=38`),tenant `lurus-default`,channel=DeepSeek(真 key),relay token `seam-s1-relay`。真源 worklog=`doc/seam-s1-stage-worklog-2026-06-21.md`。

**① topup → platform wallet 扣 + credit-pool 充**(修前 = 402 `WALLET_DEBIT_FAILED`,根因 newhub 漏发 `Idempotency-Key`,PR #34 修):
```
POST /api/v2/admin/tenants/lurus-default/credit-pool/topup {"amount":100000}
→ {"data":{"new_balance":100000,"max_balance":10000000,"tenant_id":"lurus-default"},"success":true} [http=200]
platform wallet(account 38): 999.0000 → 899.0000   (real billing.wallet_transactions id=64: pool_topup -100.0000, balance_after 899, product_id=newhub)
credit-pool(lurus-default): current_balance 0 → 100000
```

**② relay /v1/chat/completions → 真 DeepSeek + credit-pool 扣费**(token tenant_id 对齐 `lurus-default` 后,pool 真实抽取):
```
POST /v1/chat/completions {"model":"deepseek-chat","messages":[{"role":"user","content":"Reply with exactly: SEAM S1 OK"}]}
→ {"choices":[{"message":{"content":"SEAM S1 OK","role":"assistant"}}],"usage":{"total_tokens":18,"x_lurus":{"cost_lb":0.000004,...}}} [http=200]   (真 DeepSeek 响应)
后续 713-token 调用: credit-pool 100000 → 99904 (−96 units)
账单(newhub logs id=4): type=consume / deepseek-chat / prompt 13 + completion 700 / quota 96 / token=seam-s1-relay  ← 与 pool 抽取 96 吻合
```

**残留(非本链阻塞)**:(a) 沙盒支付→钱包腿仍缺 R6 payment provider(本次钱包用 platform internal-credit 真账本充值,owner 批准);(b) **tenant-id 漂移真 bug**:tenants.id=`lurus-default` 但 user/token 默认 `tenant_id='default'`(孤儿引用,无此 tenant)→ relay 按 `default` 查 pool 落空、静默走本地 quota 不抽 pool(本次手动对齐 user/token→`lurus-default` 后修复)。⚠️ **生产影响**:默认租户用户的 relay 不会扣 credit-pool=现金路径漏洞,需 owner 在 seed/provision 侧统一 tenant id。

### C. 发现的真问题（含 repro）

**[HIGH / 安全] 渠道密钥无安全验证即可读取（#33 未并入本分支）— 已于 2026-06-21 应用 #33 并实测修复（待 owner commit）**
- 原状（修前 repro）：root session 直接 `GET /api/channel/:id/key` → 200 + 明文 key（无任何验证）；`POST /api/verify` → **404**。`SecureVerificationRequired` 中间件 + `UniversalVerify` handler 存在但未挂任何路由。
- 根因：本分支 `fix/pg-baseline-gaps-021`（HEAD c9200b4d）早于 #33（2026-06-20 才并入 main），未含其接线。
- **处置**：`git cherry-pick -n 934bb523`（#33 的修复 commit）应用到工作树（**已 staged、未 commit**，与 OIDC 重构 122 文件零重叠）。#33 把 reveal 由 GET 改为 `POST /api/channel/:id/key` + 加 `SecureVerificationRequired`，并注册 `POST /api/verify` + `GET /api/verify/status`（均 UserAuth）。
- **实测验证（2026-06-21，本地栈）**：未验证 `POST key`→**403 `VERIFICATION_REQUIRED`**；旧 `GET key`→**404**；`POST /api/verify`→**200 verified:true**；`/api/verify/status`→**verified:true**；验证后 `POST key`→**200 + 明文 key**。`go test -short`（middleware/router/handler 三包）全绿；E2E 14/14 通过（spec 已翻转为断言 403→verify→200，见 `tests/e2e/matrix-channel-key-reveal.spec.ts`）。
- 注：会话用 cookie store（无 Redis），`/api/verify` 会回写更新后的 session cookie；客户端须持有更新后的 cookie（浏览器/Playwright 自动持有；裸 curl 须 `-c` 持久化）。
- 剩余：owner 决定是否把这 6 文件（2 源改 + 1 中间件注释 + 3 测试）commit 进本线（或直接走 main 的 #33）。

**[LOW / 体感] bridge 响应的 `tenant_slug` 是 "default" 而非可路由的 `lurus`**
- `resolveTenantSlug("default")` 提前返回 `"default"`；但租户真 slug=`lurus`，`/api/v2/:slug/*` 必须用 `lurus`。
- 前端靠 `DEFAULT_TENANT='lurus'` 兜底所以能用，但裸用 bridge 返回值的消费者会 404。建议让 resolveTenantSlug 对默认租户也返回真 slug。

**[INFO] `DELETE /credit-pool` 是"抽干"不是"删除"**
- 返回 `"Credit pool drained"`，余额归零但 singleton 行保留 → 再 create 报 `POOL_ALREADY_EXISTS`。非 bug，但命名易误解（建议文档或改名 drain）。

### D. 计划/文档 drift（已在 harness 修正）

1. **bridge 契约**：旧 spec 用 GET 无 user_id；实际 **POST + 必填 user_id**。
2. **audit 路由**：旧 spec 用租户级 `/api/v2/:slug/admin/audit/*`（404）；实际 **root 级 `/api/v2/admin/audit/*`**。
3. **CSV 表头**：旧 spec 断言 `id,ts,actor,...`；实际 `id,tenant_id,timestamp,actor_type,...`。
4. **PORT**：`.env.dev` 写 `PORT=3000`，但 vite 代理指 `:8012` → 本地须 `PORT=8012`。
5. **typecheck**：`web/` 无 `tsconfig`/`typecheck` 脚本，且项目 TS 4.4.2 过老（不识别 `import {type X}`）→ 无法独立 tsc；**`bun run build` 才是真门控**（已过）。e2e .ts 由 Playwright/esbuild 转译并已成功运行。
6. **relay 不走 vite 代理**：vite 只代理 `/api,/mj,/pg`，**不代理 `/v1`** → relay 调用须直连后端（harness 用 `E2E_BACKEND_URL`）。

---

## 3. Harness 产物（可重复）

- `web/tests/e2e/helpers/auth.ts` — `loginViaBridge` / `gotoDashboard` / `v2()` / `backend()` + 常量；storageState 复用。
- `web/tests/e2e/global.setup.ts` + `playwright.config.ts` — **一次性 bridge 登录**存 storageState（bridge 限流 5/60s，逐测试登录会 429）。
- 4 个改写的 DoD spec（story-11-2 / e2 / e3 / e4）+ 4 个新 matrix spec（crud / channel-key-reveal / relay-and-access / smoke）。

---

## 4. 范围外 / 诚实声明

- Playwright MCP 未接入 → 用 Bun/Playwright harness（E2E 等价）。
- 外部依赖 happy-path 不本地 mock 凑全绿（反 shortcut）。
- 本次是 OIDC 重构工作树的**首次功能级验证**；E2E 绿 ≠ 过 CI 5 门控（-race / pg-integration / coverage / lint / Trivy 仍须走 CI；本机 Docker 未起 → pg-integration 未本地补跑）。
- 未替 owner commit/stash 任何改动；OIDC 重构去向仍由 owner 决定。
- 测试数据写入 R6 一次性容器 `newhub-e2e-pg`（隔离，可整体销毁），未触碰任何现有库。

### 收尾（用完销毁本地栈）
```bash
# 本机：停 go run / vite / ssh 隧道（后台任务）
# R6：销毁一次性 PG 容器 + 数据目录
ssh root@100.122.83.20 'docker rm -f newhub-e2e-pg && rm -rf /data/newhub-e2e-pg'
```
