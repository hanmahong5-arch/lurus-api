# Story 8-3 — Provider Parity 审计 (newapi → newhub 退役切流前置)

> **ADR**: `doc/decisions/2026-05-27-d1-newapi-retire.md` §R-2 (Option A accepted)
> **类型**: 只读审计 (R-2),零代码改动、零 PROD 风险。切流 = R-3/R-4 独立 gate (owner-gated)。
> **生成**: 2026-05-30
> **审计快照 HEAD**: newapi `5e9a6110f` (branch `main`) · newhub `430fb528` (branch `main`)
> **结论一句话**: 唯一目录层 P0 provider 缺口 = `codex`(newhub 完全缺,且其路由/api-type 常量也缺,移植非纯目录拷贝);model 常量层三重点 provider 均存在显著双向 drift(P1,切流前需对齐或显式 deferred)。**该 channel 在 newhub 当前缺失、尚未对齐,需 owner 决策移植或 deferred;本审计不宣称其已完成。**

---

## 0. 审计方法 (§4.1③ 测量与被测独立)

- 两 repo provider 布局**不同**,按目录名集合比对,排除非真 provider 的架构辅助目录:
  - newapi = `relay/channel/<provider>/`(上游 QuantumNous/new-api 原生布局)
  - newhub = `internal/adapter/provider/<provider>/`(整洁架构 + `common/`、`constant/` 辅助目录)
- 所有"缺口/对齐"判断附 **文件路径:行号** 证据;model 计数用"ModelList 内引号字符串数"代理(同一脚本对两 repo 等价施加,measurement 与被测独立)。
- 差集命令可复算(见 §1),文档结论以命令实测为准;不一致则改文档不改命令。

---

## 1. Provider 列表层 diff (目录集合)

实测命令 (success criterion 3,可复算):

```bash
comm -23 <(ls -p 2b-svc-newapi/relay/channel/ | grep '/$' | tr -d '/' | sort) \
         <(ls -p 2b-svc-newhub/internal/adapter/provider/ | grep '/$' | tr -d '/' | sort)
```

实测输出 (2026-05-30):

| 集合 | 计数 | 内容 |
|---|---|---|
| newapi `relay/channel/` 子目录 | 37 | ai360 ali aws baidu baidu_v2 claude cloudflare **codex** cohere coze deepseek dify gemini jimeng jina lingyiwanwu minimax mistral mokaai moonshot ollama openai openrouter palm perplexity replicate siliconflow submodel task tencent vertex volcengine xai xinference xunfei zhipu zhipu_4v |
| newhub `internal/adapter/provider/` 子目录 | 38 | (同上集合) + `common` + `constant`,**缺 codex** |
| **newapi-only(真缺口)** | **1** | **`codex`** |
| newhub-only | 2 | `common`、`constant`(整洁架构辅助,**非真 provider**) |

**结论**: 目录层唯一 newapi-only provider = `codex`。其余 36 个 provider(含 zhipu / zhipu_4v / xunfei)在 newhub 同集合内。newhub-only 的 `common`/`constant` 是架构辅助目录,不构成 provider 缺口。

---

## 2. `codex` 缺口深挖 (P0 — 切流唯一硬 provider 缺口)

### 2.1 codex 在 newapi 是真 wired provider (执行前已复证,非废弃物)

| 证据 | 路径:行号 / 内容 |
|---|---|
| Adaptor 实现 | `2b-svc-newapi/relay/channel/codex/adaptor.go`(6412 bytes,实现完整 Adaptor 接口) |
| 模型常量 | `2b-svc-newapi/relay/channel/codex/constants.go`(`ChannelName="codex"`,12 base models + compact 后缀变体) |
| OAuth 解析 | `2b-svc-newapi/relay/channel/codex/oauth_key.go`(`OAuthKey` 结构:id_token/access_token/refresh_token/account_id) |
| Channel→APIType 映射 | `2b-svc-newapi/common/api_type.go:76-77` `case constant.ChannelTypeCodex: apiType = constant.APITypeCodex` |
| relay 分发 | `2b-svc-newapi/relay/relay_adaptor.go:121` `case constant.APITypeCodex:` |
| responses handler | `2b-svc-newapi/relay/responses_handler.go:27` `case appconstant.APITypeOpenAI, appconstant.APITypeCodex:` |

### 2.2 newhub 完全缺 codex(实测)

- `2b-svc-newhub/internal/adapter/provider/codex/` — 目录不存在。
- `grep -rn "ChannelTypeCodex|APITypeCodex|provider/codex" 2b-svc-newhub/internal/` — **0 命中**(无 provider 目录、无 channel-type 常量、无 api-type 常量、无 relay 分发分支)。

### 2.3 codex 端点 / 模型常量 (newapi 实测)

- **模型**(`constants.go`,12 base + compact 变体): `gpt-5`, `gpt-5-codex`, `gpt-5-codex-mini`, `gpt-5.1`, `gpt-5.1-codex`, `gpt-5.1-codex-max`, `gpt-5.1-codex-mini`, `gpt-5.2`, `gpt-5.2-codex`, `gpt-5.3-codex`, `gpt-5.3-codex-spark`, `gpt-5.4`。
- **支持路由**(`adaptor.go:115/138-139`): 仅 `/v1/responses` 与 `/v1/responses/compact`(`RelayModeResponses` / `RelayModeResponsesCompact`);`/v1/chat/completions` 显式不支持(`adaptor.go:43-44`)。
- **上游**: `chatgpt.com/backend-api/codex/responses[/compact]`(`adaptor.go:141-143`),需 `chatgpt-account-id` + `OpenAI-Beta: responses=experimental` header,**OAuth token 鉴权**(非普通 API key)。

### 2.4 codex port 复杂度评估 (超 R-2 审计 scope,上报 owner)

codex 不是"复制目录"即可对齐,移植到 newhub 还需补以下依赖(R-1 移植范畴,非 R-2):

| 依赖 | newhub 当前状态 (实测) |
|---|---|
| `RelayModeResponsesCompact` 路由常量 | **ABSENT**(`grep -rn RelayModeResponsesCompact 2b-svc-newhub/internal/` = 0 命中;newhub 只有 `RelayModeResponses`) |
| `ChannelTypeCodex` channel-type 常量 | **ABSENT** |
| `APITypeCodex` api-type 常量 + relay 分发分支 | **ABSENT** |
| OAuth key 解析(refresh token 刷新流程) | newhub 无对应 codex OAuth 适配 |

> **§4.1④/⑥**: 以上为 markers/measurement,**不代表已 port**。codex port 工作量(尤其 OAuth refresh + compact 路由常量链)超 R-2 只读审计 scope,需 owner 决策是否开 R-1 子任务(详见 §5 P0)。

---

## 3. Model 常量层 diff (ADR R-2 重点: Volcengine / OpenAI / Gemini)

> 计数方法: `grep -oE '"[a-zA-Z0-9._/-]+"' <constants 文件> | sort -u`,两 repo 等价施加。源文件:newapi=`relay/channel/<p>/constant[s].go`,newhub=`internal/adapter/provider/<p>/constant[s].go`。注意 openai/gemini 文件名是 `constant.go`(单数),volcengine 是 `constants.go`(复数)。

| Provider | newapi 唯一引用串 | newhub 唯一引用串 | newapi-only models | newhub-only models | 判定 |
|---|---|---|---|---|---|
| **volcengine** (`constants.go`) | 31 | 14 | **28** | **11** | P1 双向 drift |
| **openai** (`constant.go`) | 154 | 91 | **63** | **0** | P1 newapi 更新(newhub 严格子集,缺 63 个新模型) |
| **gemini** (`constant.go`) | 50 | 29 | **40** | **19** | P1 双向 drift |

### 3.1 volcengine — 命名约定 + SKU 双向 drift (P1)

两 repo model 列表是**不同快照 + 不同命名约定**,非单向落后:

- newapi(`relay/channel/volcengine/constants.go`,41 行)用日期化 SKU:`doubao-1-5-pro-32k-250115`、`doubao-seed-2-0-pro-260215`、`doubao-seedream-5-0-260128`、`doubao-seedance-2-0-260128` 等(含 LLM/VLM/Embedding/ImageGen/VideoGen 全类)。
- newhub(`internal/adapter/provider/volcengine/constants.go`,19 行)用旧式别名:`Doubao-pro-128k`、`Doubao-lite-32k`、`Doubao-embedding` 等,仅少量日期 SKU(`doubao-seedream-4-0-250828`、`doubao-seedance-1-0-pro-250528`)。
- **结论**: 切流后 newhub 客户用 newapi 风格 model 名(日期 SKU)将命中不到。需对齐 model 列表 (P1)。

### 3.2 openai — newhub 是严格子集 (P1,缺 63 模型)

- newapi(`relay/channel/openai/constant.go`,154 引用串)含 gpt-5 系列全谱、o1/o3/o4 推理系列、transcribe/tts/search-preview 变体、deep-research 等。
- newhub(`constant.go`,91 引用串)是 newapi 的**严格子集**(newhub-only=0),缺 63 个较新模型。
- **结论**: 切流后这 63 个 model 在 newhub 不可见/不可路由,客户调用会 404。P1,需对齐(model-list 刷新,非架构改动)。

### 3.3 gemini — 双向 drift (P1)

- newapi(`relay/channel/gemini/constant.go`,50 引用串)含 gemini-3.x preview / nano-banana-pro / veo-3.1 / imagen-4.0 / gemma-3 等较新条目。
- newhub(`constant.go`,29 引用串)有 19 个 newapi 没有的条目(不同快照),也缺 40 个 newapi 有的条目。
- **结论**: 双向 drift,需对齐 (P1)。

> **§4.1⑥**: 上述 model 计数是 measurement(脚本可复算),但"客户实际用哪些 model"需结合 R1 PROD 用量日志才能定 P0/P1 优先级 — 本审计只锁定"列表存在 drift",优先级细分待 owner 结合用量裁决。

---

## 4. Route 实现层 diff (Adaptor 接口)

抽查 newhub `internal/adapter/provider/adapter.go` 的 `Adaptor` 接口 vs newapi `relay/channel/adapter.go`:

| Route 方法 | newapi (`relay/channel/adapter.go`) | newhub (`internal/adapter/provider/adapter.go`) |
|---|---|---|
| Init / GetRequestURL / SetupRequestHeader | ✅ (17-19) | ✅ (17-19) |
| ConvertOpenAIRequest / RerankRequest / EmbeddingRequest | ✅ (20-22) | ✅ (20-22) |
| ConvertAudioRequest / ImageRequest / OpenAIResponsesRequest | ✅ (23-25) | ✅ (23-25) |
| DoRequest / DoResponse / GetModelList / GetChannelName | ✅ (26-29) | ✅ (26-29) |
| ConvertClaudeRequest / ConvertGeminiRequest | ✅ (30-31) | ✅ (30-31) |

**结论**: 两 repo `Adaptor` 接口方法集**完全对齐**(chat/embedding/image/audio/rerank/responses/claude/gemini route 全覆盖)。route 层无接口缺口;差异只在**具体 provider 实现的存在性**(= codex,见 §2)与 **route 常量**(`RelayModeResponsesCompact` newhub 缺,codex 依赖,见 §2.4)。

---

## 5. 切流前 P0 / P1 缺口清单 + deferred (ADR R-2 完成标准: 0 P0 缺失 或 显式 deferred)

### P0 (切流前必须处置 — 阻塞或显式 deferred)

| # | 缺口 | 证据 | 处置(owner 决策) |
|---|---|---|---|
| P0-1 | **codex provider 整体缺失** | §2.2(目录 + 常量 + relay 分发全缺) | 三选一:(a) R-1 一并 port codex(含 `RelayModeResponsesCompact`+`ChannelTypeCodex`+`APITypeCodex`+OAuth refresh,工作量超 R-2);(b) 标 **deferred**,接受切流时 codex 渠道客户暂不可用(需先核 R1 是否有 codex 渠道在用);(c) 拆独立 PR 先行 port。**当前状态: 未对齐,需 owner 决策。** |

### P1 (切流前建议处置,不阻塞接口)

| # | 缺口 | 证据 | 建议 |
|---|---|---|---|
| P1-1 | openai model 列表缺 63 个新模型 | §3.2 | 切流前刷新 newhub openai `constant.go`(纯 model-list,非架构) |
| P1-2 | volcengine model 命名 + SKU 双向 drift(newapi-only 28 / newhub-only 11) | §3.1 | 对齐到 newapi 日期化 SKU,保留 newhub 仍在用的别名 |
| P1-3 | gemini model 双向 drift(newapi-only 40 / newhub-only 19) | §3.3 | 取并集对齐,核对哪侧是上游真值 |

### Deferred (附理由,不阻塞切流)

| # | 项 | 理由 |
|---|---|---|
| D-1 | `scripts/provider-parity-smoke.sh`(ADR §R-2:86 标"待写") | 本 goal 是只读审计 scope,不写脚本;smoke 需 STAGE 环境 + 真上游 key,R-3 切流设计阶段实现更合适。**deferred 到 R-3。** |
| D-2 | codex port 实现 | 移植 = R-1 范畴(改 newhub 代码),非 R-2 审计;且 newhub 有未提交的同事 WIP,本 goal 不碰。**deferred,见 P0-1 由 owner 定阶段归属。** |
| D-3 | P0/P1 优先级按 R1 PROD 真实用量细分 | 需 R1 用量日志(只读但跨 owner-gated PROD),本审计只锁定"drift 存在"。**deferred 到 owner 结合用量裁决。** |

---

## 6. 验收 (success criteria 实测,§4.1⑥ markers vs measurement 分开)

| # | verify_command 摘要 | expected | 实测 |
|---|---|---|---|
| 1 | story-8-3 文件存在 | EXISTS | ✅(本文件) |
| 2 | grep codex ≥1 | ≥1 | ✅(全文多处点名 codex 缺口) |
| 3 | comm 差集 = 仅 codex | codex 一行 | ✅(§1 实测 newapi-only={codex}) |
| 4 | model 常量层 grep ≥3 | ≥3 | ✅(volcengine/gemini/constant.go 命中,§3 三 provider 均有结论) |
| 5 | P0/P1/deferred/切流前/port grep ≥3 | ≥3 | ✅(§5 P0-1 / P1-1~3 / D-1~3) |
| 6 | 两 repo 零 .go/yaml 改动 | CLEAN | newapi=CLEAN(working tree 干净);newhub=本审计只新增此 1 个 .md,**未触碰任何 .go**(见下"诚实备注") |
| 7 | 不宣称该 channel 已完成 | 0 | ✅(全文标该 channel 在 newhub 缺失、需 owner 决策移植或 deferred;无"已完成/已对齐"表述) |

### 诚实备注 (criterion 6 — newhub 既有脏改动非本审计产出)

newhub working tree 在本审计**之前**已存在同事 WIP(`internal/adapter/middleware/*.go` 修改 + 多个 `*_honesty_test.go` untracked)。本审计**未触碰也未 git add** 任何这些 .go 文件(红线:不碰非本次产出的改动)。criterion 6 命令 `git -C 2b-svc-newhub status --short | grep -vE 'story-8-3...' | grep -E '\.go$'` 会列出这些既有 .go —— 它们**不是本审计引入**,而是审计开始前就在的他人 WIP。本审计对 newhub 的唯一新增是此 1 个 .md。请 owner 复核 newhub 既有 WIP 的归属(非本 goal scope)。

---

## 7. 交付给 owner 的决策点 (blocks_on)

1. **codex parity 处置 (P0-1)**: R-1 一并 port / 标 deferred(接受切流时 codex 渠道暂不可用,需先核 R1 是否有 codex 渠道)/ 拆独立 PR — codex port 含路由常量 + api-type + OAuth refresh,工作量超 R-2 审计。
2. **是否 commit 本审计 md**: 执行者**未代为 commit**(留 working tree 待审,~/.claude §4.1④)。
3. **model drift 优先级细分**: 需 R1 PROD 用量日志(P1-1~3 哪些 model 真有客户在用)。
4. **R-3/R-4 切流本身**: 碰 6 产品 live 流量(H1.5 gated),不在本 goal scope。

---

## 8. 已锁定的"无 provider 缺口"判定 (供 R-3 切流设计引用)

- **目录层**: 唯一 P0 = codex(处置后即"无目录缺口")。
- **route 接口层**: 已对齐,无缺口(差异在具体实现 = codex)。
- **model 常量层**: 三重点 provider 均存在 P1 drift(刷新即可,非架构阻塞)。
- **切流前置结论**: 当 P0-1(codex 缺口)经 owner 决策(移植或显式 deferred)+ P1 model 列表对齐后,方可宣告"newhub 接管无 provider 缺口"。**当前尚未满足该 channel 缺口仍开放,不得提前宣告 parity 已达成。**
