<!-- 生成自多 agent swarm 评审 (12 Sonnet 子 agent: 1 盘点 + 5 客户画像 + 5 对抗质疑 + 1 综合)。
事实锚点: doc/local-e2e-report-2026-06-20.md (本地 E2E 14/14)。日期: 2026-06-21。
注: "约15/51 audit action 未接线"为 agent 代码审查所报，按否定式 finding 须对 HEAD 复核后才作为 backlog 立项。 -->

# Newhub 付费意愿评估 + 用户测试交付 Go/No-Go 报告

---

## 1. 付费意愿计分板

| 客户类型 | 原始分 | 对抗修正分 | 最合适计费模型 | 一句话理由 |
|---------|--------|-----------|--------------|-----------|
| 个人开发者 / Indie Developer | 1 | 1.0 | 按量抽成（无月费） | OSS newapi 0 成本自托管覆盖 90% 诉求，真差异化模块（OpenRouter 多 key 池 / 渠道路由）E2E 未验证，结构性无付费动力 |
| AI 应用初创 / SaaS Builder | 3 | 2.5 | 按 token 用量抽成 + 每租户席位底价 | 代码完整但三个硬阻塞（STAGE 未部署 / OIDC 未 commit / 计费 seam 断链）让「概念上愿意付」无法转化为实际付款 |
| 中大型企业 IT / 合规驱动 | 3 | 2.5 | 按租户/月阶梯 + 年付对公 | 治理模块代码就绪，但 SSO 演示不可达、审计覆盖率有 15/51 常量未接线、无商业合同/DPA，安全团队第一关即被卡死 |
| 企业内部平台团队 / 成本归因 | 3.5 | 3.0 | 按活跃租户/月 + 用量阶梯封顶 | 付费核心组合（审计 + 成本归因 + 配额门控 + SSO）有三项 stage-only 或未 commit，但架构设计完整度高于同分客户，修复后弹性最大 |
| 转售商 / 代理商 | 3 | 2.0 | 按租户/月或年付买断 | 唯一差异化模块 Switch 激活码 E2E 未验证、信用池充值链路断开、R6 无部署实例，今天给不了任何可赚钱的闭环能力 |

---

## 2. 最值得先攻的付费段

**首发目标：企业内部平台团队（修正分 3.0）**

理由：
- 痛点真实且自己搭代价高——200 人用 OSS newapi 做 chargeback 需要自写 SQL 报表，newhub 的审计导出 + 用量分析是从无到有的跨越，不只是「做得更好」
- 修正分最高，且该客群没有「LiteLLM Enterprise $500/月」的英文替代选项（中国数据不出境 + 中文 UI + 对公）
- 付费核心组合（审计 + 用量分析）已 local-pass，距「可演示」最近，三个阻塞项（OIDC / NATS 告警 / STAGE 部署）集中在同一技术主线上，修一次能解锁多个模块
- 决策周期相对企业 IT 更短（内部立项，不走采购委员会），付钱路径更快

**次优目标：转售商 / 代理商（修正分 2.0，但 Switch 闭环一旦 E2E 通过可跳升至 4+）**

理由：
- Switch 激活码兑换是 OSS newapi 结构性没有的能力，壁垒唯一
- 该客群体量小但决策快，margin 驱动，付钱意愿随产品可用性陡升
- 条件：Switch E2E + R6 部署 + Provisioning API 文档，三件事同时解锁

**暂缓：中大型企业 IT（修正分 2.5）**

采购周期 3-6 个月，进入正式评估前必须先过安全团队的 SSO 演示和审计覆盖率白皮书，当前两项都无法提供，投入产出比不合算。

---

## 3. "跑通流程"现实

### 今天真正端到端可用的付费闭环

| 闭环能力 | 验证状态 | 可演示路径 |
|---------|---------|-----------|
| 多租户 relay 鉴权（401/403/scope 拒绝） | local-pass | 本机 + SSH 隧道 PG，直连后端 |
| 操作审计 51 actions + CSV/JSON 导出 | local-pass（注意：约 15/51 常量未接线）| bridge 登录后访问 /api/v2/admin/audit |
| 使用量计量 + 日志分页查询 + CSV 导出 | local-pass | bridge 登录后访问 /api/v2/admin/logs |
| Credit Pool CRUD（create/get/usage） | local-pass | 本地 API 调用 |
| Reseller Provisioning（Token 跨租户隔离） | local-pass | 本地 API 调用 |
| v2 管理控制台 17 页 UI | local-pass（bridge 登录绕过 SSO） | bun dev 本地访问 |
| 成本节省分析器（无历史数据时返回空） | local-pass（推断） | GET /api/v2/admin/governance/savings |
| PIPL 账户级联删除（newhub 侧） | local-pass | 内部 API 调用 |

### 卡在 STAGE/broken 的能力

| 能力 | 阻塞原因 | 症状 |
|-----|---------|------|
| OIDC SSO 真实登录 | oauth.go + oidc_auth.go 在工作树 122 文件中，未 commit 未过 CI；HEAD 仍是 Zitadel 硬编码 | 本地 E2E 显式 skip，bridge 登录是唯一入口 |
| Credit Pool topup 注资 | platform wallet 未部署（stage-only） | 返回 "Actor has no platform wallet" |
| 配额门限 NATS 告警 | LLM_QUOTA_NATS_ENABLED 默认 false；告警消费侧（钉钉/企微 webhook）在 lurus-platform 不在本 repo | story-e4 显式 skip |
| relay LLM 200 happy-path | 需真 provider key | E2E 列入 stage-only |
| Switch 激活码兑换 E2E | 需 Switch 客户端 + 真激活码 | verificationStatus=unknown |
| newhub in-cluster 任何路径 | R6 newhub 本体已拆除，无运行实例 | 所有 local-pass 均在开发者本机 bridge 模式下 |

---

## 4. 每段的付费拦路虎

### 个人开发者 / Indie Developer
- **结构性壁垒缺失**：OSS newapi 0 成本自托管覆盖 90% 诉求，newhub 无增量价值可定价
- NATS 告警运维成本远超个人受益
- OpenRouter 多 key 池 / 渠道 EMA 路由虽真实接线但个人开发者不会为此付费
- 无托管 SaaS 版本（让「自建 0 成本」变成「托管省时间」是唯一出路）

### AI 应用初创 / SaaS Builder
- **STAGE 零实例**：R6 newhub 未部署，连演示环境都拿不出来
- **OIDC 未 commit**：企业下游客户要求 SSO，本地 skip 无法演示
- **计费闭环断**：topup 无正向路径，「给客户充值配额」不通，商业模式跑不转
- CNY 商务闭环（专票/对公）仍在 platform backlog，当前差异化壁垒无法兑现

### 中大型企业 IT / 合规驱动
- **SSO 演示不可达**：oidc_auth.go 953 行从未 commit，安全团队第一关卡死
- **审计覆盖率有水分**：~15/51 audit action 常量未接线，被安全团队识破即失去信任
- **无 DPA/合同/专票**：有商业实体能签合同是超越 OSS 竞品的核心卖点，但仍在 platform backlog
- **STAGE 无演示环境**：采购方要的是「集群验收通过」不是「本地 bridge 登录」

### 企业内部平台团队 / 成本归因
- **OIDC SSO 无法演示**（与上同，是最硬阻塞）
- **NATS 告警消费侧不在本 repo**（跨服务硬依赖，不是配置问题）
- **Credit Pool SEAM S1 未打通**（chargeback 核心链路：充值→消耗→报表全走不完）
- **无商业合同/DPA**（合规团队不允许在无签约 SaaS 上放内部 LLM 流量）

### 转售商 / 代理商
- **Switch E2E 未验证**（核心商业闭环能否跑通完全未知）
- **充值链路断**（转售商无法给客户充值 = 收不到钱）
- **newhub R6 无实例**（卖给转售商的是「生产可用 API」，本地 pass 无法兑现）
- **Provisioning API 无文档**（经销商需要用脚本批量开 token，黑箱不可接受）

---

## 5. 交付 Go/No-Go

**结论：conditional-go（有条件放行用户测试）**

当前状态：本地 E2E 14/14 通过，但全部基于 bridge 登录 + SSH 隧道 PG + 无真实集群的开发者环境，不等于可交付给外部用户测试。

### 最小必备清单（按 ROI 排序）

| 优先级 | 任务 | 解锁的付费客群 | 难度估计 |
|--------|------|--------------|---------|
| P0 | **newhub 部署到 R6 STAGE**（migrations=021，/healthz 200，pod running） | 全部——没有 STAGE 什么客户都无法验收 | 中（owner 需 commit OIDC 重构或先 deploy HEAD） |
| P0 | **OIDC 重构 122 文件 commit + CI 五门控全绿 + STAGE 验证一次真实 OIDC 登录** | 企业内部平台团队、SaaS Builder、企业 IT | 中高（CI 门控时间 + 回归风险） |
| P1 | **Credit Pool topup 在 STAGE 打通**（platform wallet → newhub fund seam in-cluster E2E） | SaaS Builder、内部平台团队、转售商 | 高（跨服务 seam，platform 侧需联调） |
| P1 | **Provisioning API 接入文档 + 一个 curl 示例**（/internal/v1/provisioning/tenants/:slug/keys） | 转售商 | 低（文档工作） |
| P2 | **NATS quota_threshold 告警在 STAGE 实证一次**（真流量跨 80% 档位 → 钉钉/企微收消息截图） | 企业内部平台团队、企业 IT | 高（跨 repo 依赖 notification 消费侧） |
| P2 | **Switch 激活码兑换 E2E 实证**（需 Switch 客户端联调） | 转售商 | 高（需跨端协调） |
| P3 | **审计 action 覆盖率白皮书**（标明哪 36 个已接线、哪 15 个仅定义） | 企业 IT（安全团队审查） | 低（代码扫描 + 文档） |

**结论**：P0 两项完成 → 可放行「企业内部平台团队」的 POC 用户测试；P0+P1 完成 → 可放行「SaaS Builder + 转售商」；P2+P3 完成 → 可进入「中大型企业 IT」的采购评估。

---

## 6. 免费 OSS 替代的护城河问题

### 真实付费差异点（有代码支撑的）

| 差异化能力 | OSS newapi 能做吗 | LiteLLM 能做吗 | newhub 状态 |
|-----------|-----------------|--------------|------------|
| 51 action 操作审计 + CSV/JSON 导出 | 极弱（仅原始 log） | 有（Enterprise 版） | local-pass，但 15/51 未接线 |
| 51 action 操作审计 + CSV/JSON 导出（续） | | LiteLLM OSS 版无 | 这是对中小客户的真差异化 |
| Reseller Provisioning 跨租户 Token 发行 | 无 | 无 | local-pass，OSS 竞品结构性缺失 |
| Switch 激活码 → relay token 闭环 | 无 | 无 | unknown，唯一的桌面端差异化 |
| 渠道 EMA 评分 + AdjustWeights 真接线 | 仅权重轮询 failover | 有类似能力 | unknown E2E，但代码已接入 relay 路径 |
| PIPL §47 级联删除 | 无 | 无 | local-pass，合规门槛 |

### 不构成护城河的（会被误认为差异化）

- **多租户 relay 本身**：OSS newapi 同样支持 channel/token 隔离，0 成本自托管，无法收费
- **成本节省分析器**：LiteLLM 有 spend tracking，Helicone 免费版有成本视图，差距不够付费
- **v2 控制台 17 页 i18n**：OSS newapi 也有 UI，938 keys 中英双语是体验提升不是结构壁垒
- **OpenRouter 多 key 池冷却**：已标 monetizable=false，不作为收费卖点

### 真正能撑起收费的护城河（两个，且均有限制）

**护城河 A：中国商务闭环（CNY / 对公转账 / 增值税专票 / 数据不出境合同）**
- 这是 LiteLLM / Portkey / OpenRouter 结构性给不了的
- **当前缺口**：专票状态机和对公核销 API 在 platform 侧 backlog，newhub 代码里的 CNY 只是显示格式，不是闭环；今天这个护城河是期权不是现实

**护城河 B：治理溢价（审计 + RBAC + PIPL + Provisioning）的中国本地化组合**
- 相比 OSS 竞品，这是从无到有的跨越（而非做得更好）
- 对「自建代价高」的中型企业内部平台团队，可以撑起 ¥2000-8000/月的订阅定价
- **当前缺口**：OIDC SSO 未 commit 导致整个「治理入口」无法演示；审计 15/51 未接线有信任风险

**结论**：护城河逻辑成立，但两个护城河今天都有关键缺口。最快变现路径是「治理溢价 B」面向企业内部平台团队，而非「中国商务闭环 A」（商务闭环需要 platform 侧配合，不在本 repo 控制范围内）。在 OIDC commit + STAGE 部署完成前，护城河可讲但无法演示，不能支撑任何正式定价。
