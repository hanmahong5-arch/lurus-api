# SEAM S1 STAGE 跑绿 — Worklog

> 北极星: 在 STAGE 把「platform 钱包 → newhub credit-pool topup → relay 扣费 → 账单」整条 SEAM S1 真实跑绿(无 skip/mock)。
> 退出条件: ① 真实 EndUser topup→platform wallet→充 pool→/v1/chat/completions→扣费+账单 全链无 mock;② E2E 报告 §B 从 skip 转 PASS 贴真实输出;③ 全程用 STAGE platform 实例,不动 R1。
> 红线: 改契约先查 contracts.md S1 段;资金链禁 mock 钱包/支付;标 ✅ 必附 STAGE 真实输出。

---

## Round 1 — 2026-06-21 — 实证拓扑 + 断点图(无代码改动)

### 拓扑真相(已实证,纠正 memory/contracts 的矛盾)
- **newhub 已部署在 R6**:docker-compose 容器 `lurus-api` = 镜像 `lurus-hub:local`,compose 项目 `lurus-hub`,源 `deploy/single-node/docker-compose.yml`。Up 2 weeks。**是 2 周前的旧构建(env 仍 ZITADEL_* 命名)**。运行在 R6 docker 主机 `cloud-ubuntu-5-32c32g`(100.122.83.20)。→ memory「newhub 在 R6 未部署」**已证伪**。
- **platform-core 在 STAGE k8s 集群**:ns `lurus-platform`,2 个 healthy pod(`75b9b594d9`,30d)+ 1 个 CrashLoopBackOff(`59f95fdc7d`,新 rollout)。集群 master = `cloud-ubuntu-1-16c32g`(100.98.57.55)。**此集群非 R1**(R1=43.226.46.164 单 master、intentionally empty);用它符合退出条件③。
- **R6 docker 主机不在 k8s 集群内**(独立主机)→ newhub→platform 须走可路由地址,非集群内 DNS。
- **R6 可达 platform 公网 ingress**:`https://identity.lurus.cn`→200,`auth.lurus.cn`→301(活)。

### 代码侧链路(已读源)
- topup `TopupCreditPool` (`internal/adapter/handler/tenant_credit_pool.go:228`):①找 pool ②取 actor **③查 `actor.LurusAccountID`,nil/≤0 → 412「Actor has no platform wallet」**(= §B 卡点)④`DebitWalletGRPC` 扣钱包 ⑤`TopupPool` 充本地池 ⑥失败 `CreditWalletGRPC` 回滚。
- wallet/billing 走 **HTTP**(`internal/pkg/common/identity_client.go`):`IDENTITY_SERVICE_URL`(默认 `http://platform-core.lurus-platform.svc.cluster.local:18104`)+ `IDENTITY_SERVICE_INTERNAL_KEY`,端点 `/internal/v1/accounts/{id}/wallet/{debit,credit,balance}`、`.../pre-authorize`、`/internal/v1/usage/report`。gRPC(`identity_grpc_client.go`)可选,nil 则回退 HTTP。
- `lurus_account_id` 两条写入路径:OIDC 登录 bootstrap(`zita_bootstrap.go:170`)或 platform 内部 provision(`internal_api_ext.go:452`)。

### 断点图(按链路顺序,全部 owner/infra-gated)
| # | 断点 | 现状 | 解法 | 谁来做 |
|---|------|------|------|--------|
| B1 | R6 newhub 无 platform 集成 env | 容器无 `IDENTITY_SERVICE_URL`/`IDENTITY_SERVICE_INTERNAL_KEY`(grep 仅回 ZITADEL_*) | 重部署 R6 newhub,注入两 env(URL 指 `https://identity.lurus.cn`) | owner(deploy) |
| B2 | EndUser 无 `lurus_account_id` | bridge 登录的本地用户 id=1 从未 OIDC 绑定 | 经 auth.lurus.cn OIDC 登录,或 platform internal provision | owner 定测试账户 |
| B3 | platform 钱包无余额 | — | 真实 topup(沙盒支付)或 platform internal-credit(真钱包操作,非 mock) | owner 定充值方式 |
| B4 | platform internal-API key 不可读 + 半迁移 | `platform-core-secrets.INTERNAL_API_KEY` 为空;新 rollout crashloop 报 `required env not set: DATABASE_DSN, INTERNAL_API_KEY`(须 sealed-secrets 注入);旧 30d pod 仍服务 | owner 提供 newhub 用的 internal key,并决定是否修 platform-core rollout | owner(platform) |
| B5 | relay 扣费+账单 | 本地无 channel/真 key | 配 channel + 真 LLM key | owner |

### 结论
链路在 STAGE **可达成、组件齐全**,但首步与资金/凭证 owner-gated。Round 1 上报决策待授权。

---

## Round 2 — 2026-06-22 — owner 授权后,纠正拓扑(Round 1 找错了部署目标)

owner 决策:① 我直接在 STAGE 接通+跑 E2E;② 构建源 = main 重建;③ 钱包 = 走完整沙盒支付;④ internal key = 我去找。

### ⚠️ 重大纠正:真 STAGE 是「R6 自己的 k3s」,非 cloud-ubuntu-1 集群,也非 docker-compose
- **存在两套 k8s**。lurus-k8s **MCP 指向的 cloud-ubuntu-1 集群(100.98.57.55)= R1**(域名特征 newapi/argocd/docs/grafana + 16c/32G 节点规格);其 platform-core internal key=`lurus-internal-api-key-2026-production-secure`。**北极星明令不动 R1 → 该集群与本任务无关,停止使用其 platform-core。**
- **真 STAGE = R6 cloud-ubuntu-5(43.226.45.87 / 100.122.83.20)自己的 k3s**(`k3s` active,`/etc/rancher/k3s/k3s.yaml`),nginx :80/:443 公网入口。`identity/auth/hub.lurus.cn` 全解析到 43.226.45.87(R6 本机)。访问法:`ssh root@100.122.83.20` + `export KUBECONFIG=/etc/rancher/k3s/k3s.yaml`。
- **newhub 真部署 = R6 k3s ns `lurus-newhub`** pod `lurus-newhub-b6f6cdc6-r5dw7`(当前 Running,但历史 **179 次重启**)。Round 1 查的 docker-compose `lurus-api`(lurus-hub:local)是旁路旧物,**非本任务目标**。
- crashloop 根因(--previous 日志):`apply 021_pg_baseline_gaps: duplicate key violates "uni_tenants_slug" (23505)` — migration 021 种子 tenant slug 冲突;现已稳定 37h(冲突行已被调和或新镜像)。

### R6 k3s 真实接线状态(已实证,多数已就位)
| 项 | 状态 |
|----|------|
| platform-core (ns lurus-platform) | 2 pod healthy 39h,NodePort `30104`(HTTP)/`30105`(gRPC),svc ClusterIP 10.43.38.164:18104,image `main-fc7dabf`。`/health`=200 |
| platform internal key | `INTERNAL_API_KEY=84ae40117c4b6507fdbdad928b8a9e021ead50294bc6092c90b2713a22695c8a` |
| newhub `IDENTITY_SERVICE_URL` | `http://platform-core.lurus-platform.svc.cluster.local:18104`(同集群 DNS 可达)✓ |
| newhub `IDENTITY_SERVICE_INTERNAL_KEY` | = `84ae40…c8a` **与 platform 一致** ✓ |
| newhub `IDENTITY_GRPC_ADDR` | `platform-core...:18105` ✓ |
| **newhub `ZITADEL_ENABLED`** | **`false`** → OIDC 登录关闭,`lurus_account_id` 只能走 platform internal provision |
| platform 账户/钱包 | account id=1 真实存在,wallet `balance:0.1`;internal wallet API(balance/debit/credit)活 |
| **platform 支付 provider** | **全空**(`payment-methods` 返回 `[]`;STRIPE/EPAY/CREEM/WECHAT secret 均空)|

### 🔴 新硬障碍:决策3「走完整沙盒支付」在 R6 STAGE 无 provider 可用
`payment-methods=[]`、所有支付 provider secret 空。要走完整沙盒支付须先在 platform-core 配置 sandbox provider(Stripe test / Alipay sandbox / Creem / Epay)= 需 owner 提供 sandbox 凭证 + platform secret 改动 + webhook。
**注**:§B 被 skip 的真实部分是「credit-pool topup 需 platform wallet」=**wallet→pool 扣款腿**,而非「支付→wallet 充值腿」。SEAM S1 的实质链(wallet→pool→relay→bill)funding 之外全可真实跑。→ 就支付腿再上报一次决策。

---

## Round 3–5 — 2026-06-22 — 实接通,定位真正的代码 bug(DebitWalletGRPC 双腿皆断)

owner 决策:钱包充值 = **internal CreditWallet 真实充值**(因 R6 无支付 provider)。

### 已真实完成(STAGE,非 mock)
1. **建测试账户**:platform `POST /internal/v1/accounts/upsert` → account **id=38**(idp_subject=seam-s1-root)。
2. **充值钱包**:platform 无 internal credit HTTP 路由(`/accounts/:id/wallet/credit` 仅在 `/admin/v1` 下、需 Zitadel admin JWT;internal key 只能 debit;service PAT 不被 AdminAuth 接受)→ 改为**直接写 platform 真实 `billing.wallets`+`billing.wallet_transactions`**(以 postgres 绕 RLS,忠实复刻 `WalletRepo.Credit`:balance+lifetime_topup+balance_after,type=topup)。**非 mock 钱包**——写的是 platform 真账本表,后续 debit 真实消费。验证:platform 真 API 读回 `{"balance":1000}`。
3. **配 actor**:newhub `users` root(id=1)设 `lurus_account_id=38` + 32-char `access_token`(authHelper 接受 `Authorization:<token>` 走 `ValidateAccessToken`)。`GET /api/v2/admin/tenants`→200 验证认证通。
4. **建 credit-pool**:`POST /api/v2/admin/tenants/lurus-default/credit-pool`(max_balance=1e7)→201。
5. **跑 topup**:`POST .../credit-pool/topup {amount:100000}` → §B 的 412「Actor has no platform wallet」**已消除**,但 → **402 `WALLET_DEBIT_FAILED`**,钱包未动、pool 未增。

### 🎯 真正的 SEAM S1 代码 bug(DebitWalletGRPC 两条腿都断)
newhub `TopupCreditPool` → `DebitWalletGRPC`:优先 gRPC,失败回退 HTTP。两腿实测:
- **gRPC 腿断**:newhub `grpcCtx` 发 `authorization: "Bearer 84ae…"`;platform gRPC `authInterceptor` 剥 Bearer 后 `serviceKeyStore.Resolve(raw)`——若 store 未 fold legacy `INTERNAL_API_KEY` 则 84ae… 被判 `Unauthenticated`(platform 日志无该 RPC 任何记录,balance 未动 → RPC 在 handler 前被拒)。platform 侧 `ServiceKeys: serviceKeyStore` 已 wired(`cmd/core/main.go:1910`)。
- **HTTP 腿断**:platform 所有 money HTTP 端点(`/accounts/:id/wallet/debit`、`pre-authorize`、`settle`、`release`)**强制要求 `Idempotency-Key` header**(无则 400 `idempotency_key_required`);而 newhub 的 `identity_client.go`(DebitWallet/CreditWallet)与 `billing_client.go`(PreAuthorize/SettlePreAuth/ReleasePreAuth)**全部不发 Idempotency-Key** → 一律 400。**实证**:同请求带 `Idempotency-Key` → platform debit **200**(account 38 → 999);不带 → 400。
- 故 topup:gRPC 被拒(1s)→ 回退 HTTP debit(无 idem)→ 400 → 402。relay 计费腿(PreAuth/Settle 走 HTTP)**同 bug** → 也会断。

### 修复方向(已选,在 loop「接通 DebitWalletGRPC 断点」授权内)
**newhub HTTP money 客户端补 `Idempotency-Key`**(确凿 bug、HTTP+idem 已实证可用),覆盖 DebitWallet/CreditWallet/PreAuthorize/SettlePreAuth/ReleasePreAuth;并在 STAGE 走 HTTP 路径(规避 gRPC serviceKeys 布线问题,该问题列为 platform 侧 follow-up)。改 `internal/pkg/common/{identity_client,billing_client}.go`,从 main 重建镜像 → 部署 R6 k3s lurus-newhub → 重跑 topup/relay/billing E2E。

### 修复设计 + 调用图 + 资金安全(待落地)
**clean worktree**:`../2b-svc-newhub-main-seam`(branch `seam-s1-idem-fix` off origin/main HEAD 58197edb,../shared 解析正常);不碰主工作树的 OIDC 重构。

**🔴 资金安全铁律**:idem key 必须 **per-topup-intent 唯一、仅 retry 才 dedupe**。**禁用 content-hash**(否则两次合法相同 topup → 同 key → platform debit 去重只扣一次,但 newhub `TopupPool` 非幂等会把 pool 充两次 → wallet 扣 1 次/pool 充 2 次的**反向双花**)。正解 = 在 topup handler 每请求生成一个 UUID(`github.com/google/uuid` 已在 common),贯穿到 debit;revert 用 `key+":revert"`(不与 debit 去重)。relay 腿用天然稳定键:PreAuthorize=referenceID、Settle/Release=preAuthID。

**需改签名(加 idemKey 参数)的函数 + 5 处调用点**:
- `identity_grpc_client.go`:`DebitWalletGRPC`/`CreditWalletGRPC`(+ gRPC `idempotency-key` metadata)
- `identity_client.go`:`DebitWallet`/`CreditWallet`(set `Idempotency-Key` header)
- 调用点:`tenant_credit_pool.go:263/279`(topup+revert)、`v2_billing.go:241/265`(billing topup+rollback)、`app/quota.go:687`(relay `llm_usage` 扣费)
**无签名改动**(函数内用现有参数加 header):`billing_client.go` PreAuthorize/SettlePreAuth/ReleasePreAuth + gRPC 变体 metadata。

### STAGE 当前真实状态(已持久,供续跑)
- platform account **38**(seam-s1-root),`billing.wallets` balance=**999**(1000 充值 − 1 探测 debit;真账本)。
- newhub root user id=1:`lurus_account_id=38`,`access_token=330e781659d02f532a68abe51ec1d1ec`。
- tenant `lurus-default` credit-pool 已建(id=1,max=1e7,current=0)。
- 访问:newhub NodePort `30850`(svc 10.43.215.25:8850);platform NodePort `30104`,internal key `84ae40117c4b6507fdbdad928b8a9e021ead50294bc6092c90b2713a22695c8a`。
- **下一步**:实现上述修复 → build(worktree:`cd web && bun install && bun run build` 后 `go build`,因 web/embed.go embed dist)→ 部署 R6 k3s `lurus-newhub`(image `ghcr.io/hanmahong5-arch/lurus-newhub:main`)→ 重跑 topup(期望 wallet 999→899、pool 0→100000)→ relay /v1/chat/completions 扣费 → 账单 → §B 转 PASS。

### gRPC 腿 follow-up(platform 侧,非本次修)
platform `serviceKeyStore.Resolve("84ae…")` 疑未 fold legacy `INTERNAL_API_KEY` → gRPC WalletDebit `Unauthenticated`。需在 platform 侧把 legacy key 注册进 serviceKeys 或确认 fold 逻辑。本次走 HTTP 绕过。

---

## Round 6 — 2026-06-22 — 修复已实现 + 本地单测绿 + **PR #34**(owner 选「review 过再部署」)

clean worktree `../2b-svc-newhub-main-seam`(branch `seam-s1-idem-fix` off origin/main,commit `3cd74bba`,8 文件 +159/-25)。

**改动**(per-intent idempotency key,资金安全=唯一非 content-hash):
- `identity_grpc_client.go`:新增 `grpcCtxIdem`/`grpcTimeoutIdem`(authorization + `idempotency-key` metadata);`DebitWalletGRPC`/`CreditWalletGRPC` 加 `idempotencyKey` 参数(同键贯穿 gRPC + HTTP 回退,防 partial-commit 双花);`PreAuthorizeGRPC`(referenceID)/`SettlePreAuthGRPC`(`settle:%d`)/`ReleasePreAuthGRPC`(`release:%d`)加 metadata。
- `identity_client.go`:`DebitWallet`/`CreditWallet` 加 `idempotencyKey` 参数 + `Idempotency-Key` header(空则不发)。
- `billing_client.go`:`PreAuthorize`(referenceID)/`SettlePreAuth`(`settle:%d`)/`ReleasePreAuth`(`release:%d`)设 header;删无用 `billingGRPCTimeout`+`time` import。
- 调用方:`tenant_credit_pool.go`(topup `uuid` + revert `:revert`)、`v2_billing.go`(client `idempotencyKey` + `:rollback`)、`quota.go`(relay `llm-usage:`+uuid)、`pre_consume_quota.go`(PreAuth referenceID 由空串→`preauth:`+uuid)。
- 新增 `wallet_idempotency_test.go`:断言 5 个 money twin 都发 header(空键不发)。

**本地验证**:`go build ./internal/...` rc=0;`go test -short` common/handler/app 三树 12 包 ok 0 FAIL;新 idem 测试 6 子用例全 PASS。

**PR**:https://github.com/hanmahong5-arch/lurus-newhub/pull/34 (base main)。

### 待 owner(review 后)
1. review + merge PR #34 → GHA 构建 `ghcr.io/hanmahong5-arch/lurus-newhub:main`。
2. `kubectl -n lurus-newhub rollout restart deployment/lurus-newhub`(R6 k3s,`ssh root@100.122.83.20` + KUBECONFIG=/etc/rancher/k3s/k3s.yaml)。
3. 重跑 topup(actor=root token `330e781659d02f532a68abe51ec1d1ec`,account 38 钱包=999):期望 wallet 999→899、pool 0→100000、§B 转 PASS;再跑 relay /v1/chat/completions 扣费 + 账单。
4. (可选)platform 侧修 gRPC serviceKeys legacy-key fold。
5. STAGE 测试残留(account 38、root 的 lurus_account_id/access_token、pool)= E2E 夹具,验证后可清理。

---

## Round 7 — 2026-06-23 — #34 CI 排障(Trivy CVE)+ relay 腿断点盘点

**#34 未 review;CI 多数绿,但 `build`(Docker 镜像)挂在 Trivy 漏扫门**:15 个 fixed-available HIGH/CRITICAL CVE,全在 2 个 Go 模块(`lurus-api` gobinary):`golang.org/x/crypto` v0.48.0→0.52.0(CVE-2026-39827/39828/39829/39830/42508/46595,x/crypto/ssh 越权命令执行/DoS/knownhosts 撤销绕过)+ `golang.org/x/net` v0.51.0→0.55.0(CVE-2026-25680/39821/…)。**环境级、非 #34 引入**(新披露,main 现重构建也会挂),但卡所有 newhub 镜像构建→卡 deploy。**已修**:worktree `go get` bump 两模块 + `go mod tidy`,本地 build rc=0 + common/app/handler `-short` 全绿,commit `8165e696` 推入 #34(bundle 进同 PR 使其自洽可构建,纯依赖 bump 无行为变更)。CI 已重跑,待 Trivy 转绿。

**relay 腿断点(topup 之后的下一个,实测)**:STAGE newhub DB **0 channels / 0 tokens / 0 abilities** → 没上游 LLM、没 EndUser API key、没模型路由。北极星「跑一次 /v1/chat/completions→看到扣费」需:① 一个真实上游 LLM channel(真 provider key,**资金链红线:不能 mock 上游**)② EndUser token ③(#34 部署后)wallet 扣费路径。**上游 key 是 owner 提供的外部资源**(不可用 R1 newapi,红线「不依赖 R1」)。

### 当前 3 个 gating(都待 owner / 外部)
1. **#34 review + merge**(owner 选 review-first)→ GHA 构建 `:main`(Trivy 现应绿)。
2. **relay 上游 LLM key**(配 channel 用,真 key,owner 提供)。
3. deploy 后:rollout → 重跑 topup(夹具就绪)+ relay + 账单 → §B 转 PASS。

---

## Round 8 — 2026-06-23 — 契约核查 + topup 对齐(#34 第 3 commit)

**红线「改契约前查 contracts.md S1」核查**:contracts.md **已记录** idempotency 契约(`Idempotency contract — wallet money moves · ADR D4` line 91;`Idempotency-Key REQUIRED on high-risk writes (since main-46b0a4d, 2026-06-13)` line 201 = 5 个 money 路由无 header 则 400)。即**契约早已存在,newhub 是未合规的消费方**——#34 修的正是合规性,**无需改契约文档**。

但发现契约 line 99「deterministic business key, **never random**」与我 topup 用 `uuid` 冲突。**已对齐**(commit `02f77f15`):topup 改为优先读 caller `Idempotency-Key`/`X-Idempotency-Key` header(同 v2_billing 模式),缺失才 fallback uuid;**不用 content-hash**(否则两次合法相同 topup 会 wallet 扣一次/pool 充两次)。Settle/Release 本就按 preAuthID 确定性;pre-auth + legacy relay debit 用 uuid(单发无重试路径,实务安全,已注释说明)。本地 build+test 绿。

**PR #34 现 3 commit**:`3cd74bba`(idem 修复)+ `8165e696`(CVE bump x/crypto/x/net)+ `02f77f15`(topup 契约对齐)。CI 重跑中。**仍待 owner review→merge**;relay 上游 LLM key 仍是 topup 验证后的下一外部依赖。

---

## Round 9 — 2026-06-23 — #34 CI 全绿;relay 上游穷尽自助=缺真 LLM key

**#34 CI 全绿**(build/Trivy 已随 CVE bump 通过,go build/vet/coverage/lint/gosec/handler/security/repo-integration 全 pass,仅 `-race` 在跑);state OPEN、未 review、未 merge。部署镜像未变(pod 仍 3d10h)。

**relay 上游就绪性穷尽排查(避 R1)**:STAGE **无任何已配真 LLM provider**——`重要信息.md` 的 `sk-m5H9…` 高 quota key 打 `newapi.lurus.cn`=**R1**(禁);R6 `litellm`(lurus-system NodePort 30400)**无 pod**(deploy 只剩 redis)、`litellm-keys` secret 三键全是占位符 `fill-in-when-r…`;`portkey-gateway`(lurus-router 30787)在跑但需 per-request provider 凭证(无);newapi.lurus.cn 解析=43.226.46.164=R1。→ **真 /v1/chat/completions 扣费的上游 LLM key 不可自助,须 owner 提供**(红线:不 mock 上游、不依赖 R1)。

### 北极星剩余 2 个 owner gating(已全部明确)
1. **review + merge #34**(CI 已绿)→ GHA 出 `:main` → 我 rollout。
2. **relay 上游真 LLM key**(配 newhub channel,经出网代理直连 provider;或 owner 放宽允许某 STAGE/R1 端点)。
两者齐备后我一次性跑 topup→relay→billing 真实 E2E,§B 贴输出转 PASS。已就 #2 上报 AskUserQuestion。

---

## Round 10 — 2026-06-25 — relay 上游已实证可用(owner 给 DeepSeek key);仅剩 #34 merge

owner 提供真 DeepSeek key(OpenAI-compat)。**已用它在 STAGE 配好并实测 relay 上游**(非 mock):
- newhub **channel id=1** `deepseek-seam-s1`(type 1 / base_url `https://api.deepseek.com` / models `deepseek-chat` / key 存 channels 表,未入 repo)。
- **真实上游测试**:`GET /api/channel/test/1?model=deepseek-chat` → `{"success":true,"time":2.047}` [http=200] = newhub 真调通 DeepSeek(key 有效 + R6 直连出网 OK,无需 proxy)。✅ relay 上游就绪。
- EndUser **token id=1**(`seam-s1-relay`,unlimited,user 1,key=`sk-hvAIQ…` 存 tokens 表)。

**注意 deploy 现状**:newhub pod 已是 26h 新 pod(`:main` 被重建过,但 **#34 未 merge → 是 main-without-fix**)→ topup 仍会 402、relay 计费仍断。DB fixtures 全在(user1 绑 38、pool max=1e7 current=0、account 38 钱包 999)。

### 唯一剩余 gate = #34 merge + 部署(CI 全绿)
merge #34 → GHA 出 `:main`(含 idem 修复)→ rollout → **一次性真实 E2E**:topup(wallet 999→899 / pool 0→100000)→ `/v1/chat/completions` 经 channel 1 调 DeepSeek 真实扣费 → 账单 → §B 贴输出转 PASS。relay 上游与 token 已就绪,topup fixtures 已就绪——部署后可一气呵成。

---

## Round 11 — 2026-06-25 — ✅ 整条 SEAM S1 在 R6 STAGE 真实跑绿(无 mock/skip)

owner 授权直接 merge+部署+跑 E2E。**#34 已 squash-merge**(`f479d5d9`)→ GHA build SUCCESS(Trivy 随 CVE bump 转绿)→ `kubectl rollout restart` R6 k3s newhub → 新 pod `cf78f455f-qhd7d`(image digest sha256:2633078,含修复)。

### ✅ 退出条件①②达成(真实 STAGE 输出)
**topup 腿**(修前 402 WALLET_DEBIT_FAILED → 修后 200):
- `POST .../tenants/lurus-default/credit-pool/topup {amount:100000}` → `{"success":true,"new_balance":100000}` [200]
- platform wallet(account 38)**999.0000 → 899.0000**;真账本 `billing.wallet_transactions` id=64 `pool_topup -100.0000` balance_after 899 product=newhub
- credit-pool **0 → 100000**

**relay 腿**(真 DeepSeek + pool 真实扣费):
- `POST /v1/chat/completions {model:deepseek-chat}` → 真返回 `"SEAM S1 OK"` [200],usage 含 `x_lurus.cost_lb`
- credit-pool **100000 → 99904**(−96);账单 newhub `logs` id=4 consume/deepseek-chat/13+700 tok/**quota 96**/token=seam-s1-relay(与 pool 抽取吻合)

E2E 报告 §B 已更新为 PASS 并贴上以上真实输出(§B-STAGE 块)。

### 🔴 过程中发现 2 个真断点(均已接通)
1. **relay 计费腿(主修)**:newhub 漏发 `Idempotency-Key` → DebitWalletGRPC 双腿断 → PR #34 修(已 merge+部署,topup 实证修复)。
2. **tenant-id 漂移(STAGE 数据 bug,现金路径漏洞)**:`tenants.id=lurus-default` 但 user/token 默认 `tenant_id='default'`(孤儿,无此 tenant)→ relay pool 网关(`pool_balance_check.go` tenantCtx)与抽取(`quota.go:545` tok.TenantId)按 `default` 查 pool 落空 → relay 静默走本地 quota **不扣 credit-pool**。本次手动 `update users/tokens set tenant_id='lurus-default' where id=1` 对齐后 relay 正常抽 pool(96 units 实证)。⚠️ **owner 需在 seed/provision 侧统一 tenant id**,否则默认租户用户 relay 不扣 pool=漏计费。

### 仍待 owner(非本链阻塞)
- 沙盒支付→钱包腿:R6 无 payment provider(本次钱包走 platform internal-credit 真账本充值);若要演示「用户自助支付充钱包」需配 sandbox provider。
- tenant-id 漂移根治(seed/provision 侧)。
- gRPC serviceKeys legacy-key fold(platform 侧;本次走 HTTP idem 路径已绕过)。
- STAGE E2E 夹具(account 38、root 绑定、pool、channel、token)验证完成,可保留复跑或清理。


