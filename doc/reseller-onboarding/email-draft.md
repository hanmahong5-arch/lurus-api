# Reseller Outreach Email Template

> 中文邮件模板，发给潜在外部经销商（G3 候选）。变量用 `{}` 占位，发前替换。
> 配套技术材料：`technical-checklist.md` + `faq.md`。

---

**Subject**: Lurus Newhub —— 给 {对方公司} 的多租户 LLM 中转 + 白标 GUI 经销商方案

---

{对方称呼} 您好，

我是 Lurus 平台的 Anita。我们注意到 {对方公司} 在 {对方 LLM 业务方向} 这块已经积累了一批最终用户。Lurus 在做的 **newhub**（hub.lurus.cn）是一个多租户 LLM 网关 —— 对接了 30+ 上游供应商（OpenAI / Anthropic / Gemini / 国内 Baidu/Zhipu/Tencent 等），目前已经在我们自家产品（Kova、Lucrum、Switch 桌面端）上跑通。我们正打算把这套能力开放给少量外部经销商伙伴，{对方公司} 是首批接触的三家之一。

经销商接入后能拿到的东西：

- **独立租户（multi-tenant）**：贵司用一个 brand slug（例如 `acme`）开租户，下面所有用户、Token、调用日志都和别的租户物理隔离（schema 级 + Zitadel Org 级，参考 `doc/runbook/tenant-onboarding.md`）。
- **Provisioning API**：贵司服务端可以程序化为终端用户创建 / 列出 / 撤销 API Key（`POST/GET/DELETE /internal/v1/provisioning/tenants/{slug}/keys`，详见 `doc/contracts/switch-provisioning-api.md`）。
- **co-branded Switch GUI**：我们的 Switch 桌面客户端可以打包成贵司品牌的安装包，配套 HMAC 签名的 `whitelabel.json` 防止分发链篡改（ADR `2026-05-20-orphan-features-3-whitelabel-hmac.md`）。
- **CreditPool 计费**：预付池模式，每次 relay 自动扣，余额触发 80%（可调）阈值会通过 NATS 告警；订阅 + markup% 模式由 Lurus Platform 钱包统一记账（ADR `2026-05-09-cost-aware-routing.md` §Q1）。
- **明细对账**：每次调用 → 一条 log → 一笔 pool draw，可以 SQL 对账（`tenant_credit_pool_draws` 表，append-only）。

接入流程（建议）：

1. **R6 STAGE 演示**：在 `test-newhub.lurus.cn` 上为贵司开一个测试租户（30 分钟内可完成），跑通"创建 Key → 首次 chat 调用 → 看到 log 与扣费"完整路径。
2. **双周 pilot**：贵司接入 Provisioning API 给 10~50 个真实终端用户发 Key，观察实际调用量、calibrate pool 容量、对账流程。
3. **上 PROD 商业合作**：pilot 数据 OK 后，签订 SLA + markup% 条款，DNS 切 PROD。需说明：newhub 整合 newapi 的工作（D1 决策，Option B）正在进行，统一域名 `newapi.lurus.cn` 已规划，但 PROD 切换需在我们完成 STAGE 演练之后（参考 `doc/decisions/2026-05-20-d1-newapi-newhub-fork-final.md`）。

附件里有两份技术材料 ——

- `technical-checklist.md`：贵司技术负责人在签 pilot 前要 check 的事项（API 端点、HMAC 接入、隔离机制、STAGE 验收清单、待解决的 open questions）。
- `faq.md`：10 个常见经销商问题（计费 / 数据 / 退出 / 故障应对），每条都引代码或 ADR 出处，没编造。

下一步建议安排一个 15 分钟的 demo 通话，由我和 Lurus 技术 lead 一起在 R6 STAGE 上现场跑一遍。{安排时段 1}（{时区}）或 {安排时段 2} 这两个时段，哪个对贵司方便？

诚实补一句：我们在过 industrial-grade 加固期，circuit breaker / pg HA / chaos drill 这些可靠性手段代码层已经 ship，但 STAGE drill 还在排期（参考 `sprint-status.yaml` `wave_a_kickoff_2026_05_20`）。pilot 期间我们会优先把这部分验收数据交付出来。

期待回复。

Best,
{Anita 签名 / Lurus Platform 联系方式}

---

## 变量速查

| 变量 | 说明 |
|------|------|
| `{对方公司}` | 经销商公司名称 |
| `{对方称呼}` | 对方收件人姓名 |
| `{对方 LLM 业务方向}` | 例如 "AI 写作工具 / 客服 bot / 编程助手" |
| `{安排时段 1}` / `{安排时段 2}` | 两个备选会议时间 |
| `{时区}` | 例如 "北京时间 GMT+8" |
| `{Anita 签名 / Lurus Platform 联系方式}` | 邮件落款 |
