# Lurus Hub 业务验收测试（UAT）

> 面向**业务 / QA / 客户成功**的黑盒验收测试。不是 Go 单测（开发侧见 `TESTING.md`），
> 而是"产品对外能不能用"的端到端确认。每条用例给：目的 / 前置 / 步骤 / 预期 / 通过标准。
>
> **目标环境**：STAGE `https://test-newhub.lurus.cn`（R6）。PROD 切换前用同一套用例回归。
> **路由真源**：`internal/adapter/handler/router/*.go`（本文用例与之逐条对齐）。

---

## 0. 前置与约定

| 项 | 说明 |
|----|------|
| 工具 | `curl` / Postman / 浏览器（控制台类用例走浏览器） |
| Base URL | `export BASE=https://test-newhub.lurus.cn` |
| 通过定义 | 一条 TC 的"预期"**全部**满足才算 PASS；任一不满足记 FAIL + 截图/响应体 |
| 不在范围 | 上游模型**回答质量**、压测/容量（单列性能测试）、前端像素级 UI |

### 0.1 需准备的凭证（开测前由管理员发放）

| 代号 | 凭证 | 获取方式 |
|------|------|----------|
| `ROOT_JWT` | 平台 root 的 Zitadel JWT | 平台管理员登录后取 access_token |
| `TENANT` | 租户 slug（如 `acme`） | 管理员创建租户后得到 |
| `TOKEN` | 中转 API Key（`sk-...`） | 租户控制台 → Token 管理 → 新建 |
| `USER_SESSION` | 普通用户登录态 Cookie | 浏览器登录 V2 控制台 |
| `REDEEM_CODE` | 激活码 / 兑换码 | 管理员生成 |

设置：`export TOKEN=sk-xxxx TENANT=acme`

### 0.2 通用断言

- HTTP 2xx + 响应体 `success:true`（V1/V2 管理 API）；Relay 走 OpenAI 兼容结构。
- 错误体满足"三要素"：发生了什么 / 期望 / 调用方能做什么（如 402 附 `actionable` 提示）。

---

## 1. 测试流程（6 阶段门禁，逐阶段 gate）

```
阶段1 冒烟门禁 ──gate──> 阶段2 租户开通 ──gate──> 阶段3 Token & 中转
        │                                                      │
        └──────────── FAIL 任一 → 停测、报开发 ────────────────┘
阶段4 计费联动 ──gate──> 阶段5 治理 & 安全 ──gate──> 阶段6 验收签字
```

| 阶段 | 目标 | 入口 gate（必须先 PASS） | 用例组 |
|------|------|--------------------------|--------|
| 1 冒烟 | 服务在线、关键路由可达 | — | TC-S* |
| 2 租户开通 | 能建租户、用户能登录、隔离生效 | 阶段1 全绿 | TC-A* |
| 3 Token & 中转 | 发 Key、过网关、拿到模型响应 | 阶段2 全绿 | TC-T*、TC-R* |
| 4 计费 | 用量计费、余额、充值、兑换 | 阶段3 至少 1 条 Relay PASS | TC-B* |
| 5 治理 & 安全 | 限流/配额/权限/审计 | 阶段3 全绿 | TC-G*、TC-SEC* |
| 6 验收 | 可观测 + 签字 | 1~5 全绿 | TC-O*、签字表 |

---

## 2. 测试用例

### 2.1 冒烟（TC-S）— 无需认证

| ID | 目的 | 步骤 | 预期 | 通过标准 |
|----|------|------|------|----------|
| **TC-S1** | 网关存活 | `curl -s -o /dev/null -w "%{http_code} %{time_total}s" $BASE/api/status` | `200`，耗时 < 1s | 200 + 亚秒 |
| **TC-S2** | 配置体可读 | `curl $BASE/api/status` | JSON，`success:true` | 字段完整 |
| **TC-S3** | Switch 版本清单（Hub 专属路由） | `curl $BASE/api/v2/switch/tools/versions` | `200`，`data` 含 `claude/codex/gemini/...` 版本号 | 版本号非空 |
| **TC-S4** | Switch 预设库 | `curl $BASE/api/v2/switch/presets` | `200`，`data` 为数组 | 200 + 数组 |
| **TC-S5** | 指标暴露 | `curl $BASE/metrics` | Prometheus 文本，含 `relay_*` / `http_*` | 含网关指标 |
| **TC-S6** | 公开费率卡 | `curl $BASE/api/v2/switch/pricing` | `200`，含模型单价 | 200 + 含价目 |

> **冒烟基线（已实测 2026-06-05）**：TC-S1=200/0.19s ✅；TC-S3 返回 `claude 2.1.163 / codex 0.137.0 / gemini 0.45.1 / openclaw 2026.6.1`；TC-S4=`[]` ✅。

### 2.2 认证 & 多租户（TC-A）

| ID | 目的 | 步骤 | 预期 |
|----|------|------|------|
| **TC-A1** | 创建租户（admin） | `POST $BASE/api/v2/admin/tenants` + `ROOT_JWT`，body `{"slug":"acme","name":"ACME"}` | `200`，返回租户 id；`success:true` |
| **TC-A2** | 列租户 | `GET $BASE/api/v2/admin/tenants` + `ROOT_JWT` | 列表含 `acme` |
| **TC-A3** | 用户登录跳转 | 浏览器开 `$BASE/api/v2/acme/auth/login` | 302 跳 Zitadel，回调后建立会话 |
| **TC-A4** | 取当前用户 | `GET $BASE/api/v2/acme/user/me`（带会话） | `200`，返回本租户用户档案 |
| **TC-A5** | 跨租户隔离 | 用 `acme` 会话访问 `$BASE/api/v2/other/user/me` | `403 TENANT_MISMATCH`（**必须拒绝**） |
| **TC-A6** | 启停租户 | `POST .../admin/tenants/{id}/disable` 后该租户 Relay | 禁用后请求被拒；`enable` 后恢复 |

### 2.3 Token 管理（TC-T）— 租户控制台

| ID | 目的 | 步骤 | 预期 |
|----|------|------|------|
| **TC-T1** | 新建 Token | `POST $BASE/api/v2/acme/tokens`（会话），body 名称+额度+允许模型 | `200`，返回 `sk-...` |
| **TC-T2** | 列 Token | `GET $BASE/api/v2/acme/tokens` | 含刚建的 Token，额度正确 |
| **TC-T3** | 改额度/模型白名单 | `PUT $BASE/api/v2/acme/tokens/{id}` | 改动生效 |
| **TC-T4** | 轮换密钥 | `POST $BASE/api/v2/acme/tokens/{id}/rotate` | 返回新 Key，旧 Key 失效 |
| **TC-T5** | 删除 Token | `DELETE $BASE/api/v2/acme/tokens/{id}` | 删除后该 Key Relay 返回 401 |

### 2.4 中转 / Relay（TC-R）— 核心业务路径，用 `TOKEN`

> 网关门禁链：`TokenAuth → PoolBalanceCheck → CostSpikeLimit → EntitlementCheck → ModelRequestRateLimit → Distribute → Relay`。
> 示例模型按租户实际可用渠道替换（`$MODEL`）。

| ID | 目的 | 步骤 | 预期 |
|----|------|------|------|
| **TC-R1** | Chat 补全（OpenAI 兼容） | `POST $BASE/v1/chat/completions` + `Bearer TOKEN`，body `{"model":"$MODEL","messages":[{"role":"user","content":"ping"}],"max_tokens":16}` | `200`，`choices[0].message.content` 非空，`usage.total_tokens>0` |
| **TC-R2** | 流式 | TC-R1 加 `"stream":true` | `text/event-stream`，多个 `data:` chunk，末尾 `[DONE]` |
| **TC-R3** | Claude 原生格式 | `POST $BASE/v1/messages` + `Bearer TOKEN`，`anthropic-version: 2023-06-01`，body Claude messages | `200`，`content[0].text` 非空 |
| **TC-R4** | Embeddings | `POST $BASE/v1/embeddings`，body `{"model":"$EMB","input":"hello"}` | `200`，`data[0].embedding` 数组长度>0 |
| **TC-R5** | 列模型 | `GET $BASE/v1/models` + `Bearer TOKEN` | `200`，`data` 含该 Token 可用模型 |
| **TC-R6** | 图像生成 | `POST $BASE/v1/images/generations` | `200`，返回图片 url/b64（若该租户开通） |
| **TC-R7** | 格式互转 | 用 OpenAI 格式打一个仅 Claude 渠道的模型 | 网关自动转换，正常返回 |
| **TC-R8** | 自动重试/负载均衡 | 同模型连发 10 次（多渠道） | 全部 200，日志显示分散到多个渠道 |

### 2.5 计费联动（TC-B）

| ID | 目的 | 步骤 | 预期 |
|----|------|------|------|
| **TC-B1** | 自助查余额 | `GET $BASE/v1/billing/balance` + `Bearer TOKEN` | `200`，返回当前余额 |
| **TC-B2** | 自助查用量 | `GET $BASE/v1/billing/usage` + `Bearer TOKEN` | 返回用量明细 |
| **TC-B3** | 用量→扣费闭环 | 记 TC-R1 前余额 → 打 1 次 Relay → 再查余额 | 余额减少 = 该次 token 计费；用量日志新增 1 条 |
| **TC-B4** | 激活码充值（匿名） | `POST $BASE/api/v2/switch/redeem`，body `{"code":"REDEEM_CODE"}` | `200`，额度入账 |
| **TC-B5** | 兑换码（租户内） | `POST $BASE/api/v2/acme/redeem`（会话） | `200`，余额增加，码标记已用 |
| **TC-B6** | 在线充值下单 | `POST $BASE/api/v2/user/billing/checkout`（平台 JWT） | 返回支付链接/订单号 |
| **TC-B7** | 订单状态查询 | `GET $BASE/api/v2/user/billing/checkout/{order_no}/status` | 返回订单状态机字段 |
| **TC-B8** | Reseller 额度池 | admin 建池 + topup + 查 usage（`/admin/tenants/{id}/credit-pool*`） | 池余额随子用量递减 |

> **已知 STAGE 现状（2026-06-05 实测，写给业务别误判）**：统一货币（LUC↔LUT）服务在该 stage **未配置**（`currency_info` 返回 503 `currency_unconfigured`）；平台聚合端点 `account_overview` 当前 500。TC-B6/B7 依赖支付通道开通；若 stage 未接支付，标 **N/A** 而非 FAIL。

### 2.6 治理 & 安全（TC-G / TC-SEC）

| ID | 目的 | 步骤 | 预期 |
|----|------|------|------|
| **TC-SEC1** | 无 Token 拒绝 | `POST $BASE/v1/chat/completions` 不带 Authorization | `401` |
| **TC-SEC2** | 错 Token 拒绝 | 带 `Bearer sk-invalid` | `401` |
| **TC-SEC3** | 模型白名单 | 用只允许模型 A 的 Token 打模型 B | `403`/权限错误 |
| **TC-SEC4** | 配额耗尽 | Token 额度打满后再请求 | `402`/余额不足，附可操作提示 |
| **TC-SEC5** | IP 白名单 | 若 Token 设了 IP 白名单，从名单外 IP 请求 | `403` |
| **TC-SEC6** | 频控 | 短时间内超过 `ModelRequestRateLimit` 阈值连发 | 触发 `429` |
| **TC-SEC7** | 成本尖峰 | 单 Token 异常高频/高额连发 | `CostSpikeLimit` 短路保护生效 |
| **TC-SEC8** | 额度池闸 | Reseller 子用户在池余额耗尽时请求 | `PoolBalanceCheck` 拒绝 |
| **TC-G1** | 审计可查 | admin `GET $BASE/api/v2/admin/audit/events` | 含上述操作的审计记录（who/what/result） |
| **TC-G2** | 治理看板 | admin `GET .../admin/governance/{channels,latency,efficiency}` | 返回聚合统计 |
| **TC-G3** | 日志检索 | 租户 `GET $BASE/api/v2/acme/logs?...`（Meilisearch） | 命中相关日志，响应快 |

### 2.7 可观测性（TC-O）

| ID | 目的 | 步骤 | 预期 |
|----|------|------|------|
| **TC-O1** | 指标完整 | `curl $BASE/metrics` | 含请求量/错误率/时延/计费相关指标 |
| **TC-O2** | Trace 关联 | 打一次 Relay，看响应头 `X-Trace-Id` / `x-oneapi-request-id` | 有 trace id，可在 Jaeger 检索 |
| **TC-O3** | 结构化日志 | 后台查 pod 日志 | JSON 格式，关键操作含 who/what/result |

---

## 3. 缺陷分级 & 回归

| 级别 | 定义 | 例 | 处理 |
|------|------|----|------|
| **P0 阻断** | 核心路径不可用 | TC-S1/TC-R1/TC-SEC1 FAIL | 停测、立即报开发 |
| **P1 严重** | 主功能受损有绕过 | 计费偏差、某 provider 不通 | 当日修 |
| **P2 一般** | 体验问题 | 日志检索慢、文案 | 排期 |
| **P3 建议** | 优化项 | — | 记录 |

**回归规则**：任何 P0/P1 修复后，至少回归该用例组 + 阶段1 冒烟全套。PROD 上线前，全套用例在 STAGE 重跑一遍。

---

## 4. 验收签字

| 阶段 | 用例数 | PASS | FAIL | N/A | 结论 | 签字 / 日期 |
|------|--------|------|------|-----|------|-------------|
| 1 冒烟 | 6 | | | | | |
| 2 租户 | 6 | | | | | |
| 3 Token+Relay | 13 | | | | | |
| 4 计费 | 8 | | | | | |
| 5 治理+安全 | 11 | | | | | |
| 6 可观测 | 3 | | | | | |
| **合计** | **47** | | | | □通过 □有条件通过 □不通过 | |

**业务负责人**：________  **QA**：________  **开发对接**：________

---

_维护：本文用例对齐 `router/*.go`，路由变更后同步更新。最近核对 2026-06-05。_
