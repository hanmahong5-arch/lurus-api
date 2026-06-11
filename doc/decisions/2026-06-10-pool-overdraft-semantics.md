# ADR: 池扣减 overdraft 语义（P0-3 收钱链审计 HIGH 项）

- **日期**: 2026-06-10
- **状态**: Accepted
- **上下文**: 收钱链审计 2026-05-30 遗留 HIGH — post-consume 池扣减失败只 log 不落账

## 问题

post-consume 阶段（`PostConsumeQuota` Phase 2.5）池扣减 `DebitPool` 返回
`ErrPoolExhausted` 时，旧行为是 `SysLog` 一行后丢弃扣减。此时上游 token 已烧、
用户 quota 已扣，池账本却少记一笔 → 守恒律 `seed − Σdraws == balance` 静默破坏，
池余额与实际消耗漂移且无任何账面痕迹。

## 决策

**不做共享事务，改 overdraft 化**：`ErrPoolExhausted` 时无条件扣成负余额并落
`relay_overdraft` draw 账行（`repo.OverdraftDebitPool`）。

为什么不是共享事务（回滚用户扣减 + 池扣减绑一个 tx）：

1. **语义错误**：上游成本已经真实发生（token 烧掉了），池空时正确动作是
   "把债记下来"，不是回滚用户侧扣减让公司吞掉成本。
2. **技术不可行**：用户 quota 写路径有 `BatchUpdateEnabled` / Redis 异步缓冲
   分支，根本没有可加入的 DB 事务。

overdraft 化后：

- 守恒律无条件成立（每笔消耗必有 draw 行）。
- 负余额本身就是欠账记录；relay gate `IsExhausted() ⇒ balance <= 0`
  持续拦截新请求，直到 topup 把债补平 — 自然补偿，无需 reconciliation ticker。
- 指标：`credit_pool_overdraft_total{tenant_id}`（每笔 overdraft）+
  结构化 JSON log（who/what/result）。

## 残留缺口（诚实边界）

1. **DB 硬错误仍会丢扣减**：`DebitPool` / `OverdraftDebitPool` 遇非 exhaustion
   的硬错误（连接断、约束炸）时无处落账，只能 CRITICAL log +
   `credit_pool_debit_lost_total` counter。该 counter 任何增长 = 已知守恒律
   违约，需人工对账。接受理由：DB 不可用时任何落账方案都同样失败。
2. **BatchUpdateEnabled 模式下用户 quota 写延迟**：批量模式下用户侧扣减
   有固有有界延迟（批刷周期内），与池账本短暂不一致属设计内漂移，刷批后收敛。

## 验证

- `TestOverdraftDebitPool_NegativeBalanceAndDrawRow`（空池 / 余 3 扣 10→−7 / 已负）
- `TestPoolConservation_OverdraftFallbackRace`（-race 门：seed 53，20 并发×10，
  5 正常 + 15 overdraft 全落账，终值 −147，debit draw 行恰 20，Σ=200）
- `TestDebitTenantPool_ExhaustedWritesOverdraft` / `_UnlimitedPoolSkips` / `_NoPoolRowSkips`
