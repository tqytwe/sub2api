# v2.17.7 同步批次账本

> **2026-10-31 修复部署复核（UTC+8）**：PR [#394](https://github.com/tqytwe/sub2api/pull/394)
> 全部 9 项 CI 检查通过，修复提交 `d99f2c8a2` 已合并至
> `play/main@3da3cb0e953fe751b50a2f6f446e8c24a62c80b2`。
> Zeabur deployment `6ae5a1eaea16512cf741c89b` 的实际 SHA 与合并 SHA 一致，
> `finishedAt=2026-10-31T08:13:36.402Z`、当前状态 `RUNNING`；构建日志有镜像导出完成证据。
> 应用、PostgreSQL、Redis 均为 `RUNNING`，首页和 `/health` HTTP 200，
> 未认证访问 `/api/v1/admin/risk-control/users` HTTP 401。
> 用户在本机刷新后确认“加载正常，没有 internal error”；18:17:56–18:18:09 的
> 5 次该接口请求全部 HTTP 200，18:17:40–18:18:43 的应用最近 100 行无 ERROR 或原 SQL 参数错误。
> PostgreSQL 最近 100 行中，部署完成之后的 51 行无 ERROR/FATAL/PANIC；旧参数错误发生在部署前。
> 较早应用采样有 1 条上游转发 `context canceled`，不归为本次 SQL 缺陷，不能据有限窗口宣称全站无异常。
> 本次未新增数据库迁移或执行数据修复。补充 SQL 只读复核最初超时；关闭 psql 分页器并禁止交互密码提示后，
> 在 `BEGIN READ ONLY`、7 秒 statement timeout 和 2 秒 lock timeout 下成功完成实际参数推断的
> PREPARE/EXECUTE：7 种筛选的统计与列表一致（all/enforced 为 2/2，其他筛选均 0/0），
> 不存在用户的定向统计为 0。两条 PREPARE 均已 DEALLOCATE，已取得显式 `ROLLBACK` 回执；CLI 返回 0。
> 此次列表刷新故障已有生产请求与用户本机确认；整版同步仍保持 `fully_migrated=false`，
> 游客/普通用户完整浏览器验收、既有批次其他业务验收和数据库独立跟进项不因此关闭。
> 本段为发布后本地交付补记，待后续累计交付纳入仓库，不单独触发生产部署。

> **2026-10-31 故障定位历史（内容审查用户列表；由上方复核覆盖）**：PR [#393](https://github.com/tqytwe/sub2api/pull/393)
> 已部署到 `play/main@e11c3166945d01012a104b5f5157cb34eb98b611`，Zeabur deployment
> `6ae58008ea16512cf741c5b4` 已完成构建并运行；此前“候选未部署/生产 #392”的描述是历史记录。
> 管理员反馈 Usage 正常；内容审查用户管理刷新返回 500，15:08:31（UTC+8）应用日志定位为
> `count content moderation users: pq: could not determine data type of parameter $1`。
> 原因是总数 SQL 使用 `$3/$4` 却绑定了未使用的 `$1/$2` 时间窗口参数。
> 修复分支 `fix/moderation-users-count-20261031` 仅修正统计参数并复用同一筛选条件；不改数据库、前端、权限或审核处罚规则。
> 原查询已在生产只读事务内通过 PREPARE 复现并显式 ROLLBACK；回归测试先失败、修复后通过。
> 完整测试、构建、PR CI、修复部署和管理员刷新复验仍在进行，**不能宣称生产缺陷已解决**。
> 整版同步仍保持 `fully_migrated=false`；本次修复不改变正文采集/harvest 的既有阻断边界。

## #394 后当前工作队列（2026-10-31）

- 生产基线为上述 #394；#393 的质量/Usage/moderation/凭证能力已上线，不沿用下方“候选未部署”的历史叙述。
- 数据库新证据：生产 ledger **429** 条、当前源码 SQL **422** 个，缺失 **0**、checksum 差异 **0**，历史额外记录 **7**；296/298/301/320/334 与目标表/索引均在；只读事务显式 ROLLBACK。
- 账务独立只读观察：本轮 `settlement_pending=21,326`，其中部署前21,284、部署后42；21,325条无精确billing link。按执行状态为failed18,682、cancelled2,426、interrupted195、succeeded19、inflight4；usage_unknown21,321。`settled`但存在未验证link的计数为0。此为聚合证据，不足以判断欠扣/重复扣费；历史归因继续待办，未补扣、未改状态、未清理，事务ROLLBACK。
- 本地累计分支 `codex/v2177-post394-20261031`：AutoRestore 显式配置/执行、质量历史 unknown、不安全历史 ProbeDetail 读出、structured invalid_grant 分类、Priority quality round 与337稳定配置 revision 已完成；后端完整测试/unit/lint/build、前端全量测试/类型/lint/build、Fork integrity 和文档检查通过，尚未提交/部署，真实 PostgreSQL 集成执行待 PR CI。
- 本地尝试编译并执行 PostgreSQL integration suite 时，Docker 拉取 `postgres` 镜像因 Docker Hub 网络超时未启动容器；该失败不代表测试逻辑通过，必须由 PR CI 的 disposable PostgreSQL 作最终执行证据。
- 优先调度质量信号为本批后端实际缺口：两种reader没有填质量字段；运行器updated_at每次claim/finish变化，不能直接抄上游时间过滤。实现需独立稳定配置时间、quality有效期与完整round聚合，不引入额外provider/区域运行时。
- 已取消重复开发：审核记录行独立启用用户、跨计划历史、真实ownership恢复、最终HTTP/WS准入已存在；moderation相关现有前端/API回归重跑42/42。
- 上述本地修复将统一检查、审查、一个PR和一次生产发布；发布后立即查Zeabur实际SHA、日志、迁移、权限/API，再做受影响管理员本机验收。不能用本段替代尚未执行的发布。
- 后续剩余范围见 [开发清单](./DEVELOPMENT_CHECKLIST.md) 与 [功能矩阵](./feature-matrix.md)。446条范围准确不等于446条行为已迁移；保持`fully_migrated=false`。

以下均为较早批次的历史记录；当与本节及首段冲突时，以上方明确标注的生产证据为准。

状态：`deployed-partial / migration-incomplete`。同步批次 PR #389 已于 2026-10-28 合并并部署，但这只证明本批候选上线，不代表完整 v2.17.7 功能审计或迁移完成。`fully_migrated=false` 继续有效。文末“最新生产复核覆盖”是当前生产数字的权威覆盖，早期采样仅保留审计历史。

本目标是版本功能同步，不能以 v2.17.7 release notes 的末尾修复代替完整差异盘点。生产与上游源码的共同祖先为 `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`（上游 `0.2.5`）；上游从该点至目标 tag 有 2,155 个提交、446 个 first-parent merge，按共同祖先直接比较有 3,809 个路径变化。以上数字只界定审查范围，不表示需要照搬这些提交或文件。Fork 自有实现必须逐项保留/映射；仅纳入相关产品能力和必要安全兼容修复。

## 来源锁定

| 字段 | 值 |
|---|---|
| 来源仓库 | `ranxi2001/sub2api` |
| Release | `v2.17.7` |
| Tag object | `af9a09cf92c1cbfdcf793862122d46a5a29a6da1` |
| Release commit | `9620d315f72a7dbf04fa10fca02970e1e410805e` |
| 上游共同祖先 | `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a` (`0.2.5`) |
| 生产分析基线 | `origin/play/main@21dd9af2239c2c7f8259e41e31e21156131622b4`（PR #392 merge；2026-10-30 核验） |
| fully migrated | `false` |

## 已发布批次与整版状态

| 检查项 | 当前证据 | 结论 |
|---|---|---|
| 合并 | PR [#389](https://github.com/tqytwe/sub2api/pull/389) 是上游同步已部署批次；PR [#392](https://github.com/tqytwe/sub2api/pull/392) 当前生产修复合并 SHA 为 `21dd9af2239c2c7f8259e41e31e21156131622b4`。新累计候选在分支 `codex/v2177-cumulative-integration-20261029` 上有未提交改动 | #392 已合并；新候选尚未提交或创建 PR |
| 部署 | #392 的 GitHub Zeabur check 对应 deployment `6ae303acea16512cf7419a17`，结论 `success` | 已部署 |
| 运行态技术核对 | 2026-10-30 核实 `origin/play/main@21dd9af`；首页和 `/health` 返回 200 | 技术证据通过，不能替代管理员业务验收或数据库复核 |
| PR CI | #392 merge workflow 40842377522 成功，包含 Zeabur 与前后端安全检查；它不覆盖当前未提交候选，也不能代替新 PR 的 CI | 新候选 PR CI 待运行 |
| 管理员闭环 | 没有本机管理员浏览器操作、保存刷新、审计记录和调度效果的现场证据 | 待管理员验收 |
| 数据库生产账本 | Zeabur PostgreSQL 只读查得 416 条；生产缺少候选 `293/296/298/301`，也不存在 moderation user state/hits/actions 与 security penalty 表 | 基础 ledger 已核验；候选迁移待单批部署后复核 |
| 上游完整功能差异 | 446 个 merge SHA 均已唯一映射并完成初步功能域/处置分流（121 reviewed、325 triaged）；生产证据仍全部未验证，且存在 scope、evidence、split、隐私及架构阻断项 | 分流完成，迁移和生产验收未完成 |

本批代码已部署与“上游整版同步完成”是两个不同状态。当前累计候选已补齐质量规则模板/预设、Jev choice judge、质量 ownership 并发保护、管理员质量页面，以及 Priority 的配置化分位数和半开时间窗口修复；这些改动尚未部署。仍未完成或未证实项至少包括 Priority attempt error evidence/quality round 的完整跨协议语义、Quality live regression 与真实 restriction restore/reconcile 验收、凭据失败的上游逐字段等价复审、管理员精确滚动 24 小时用量候选的生产审查、内容审查用户管理迁移的生产核验、完整上游功能差异映射、PostgreSQL 部署窗口/平台重启原因、历史日卡审计语义、管理员真实交互验收及历史 gateway settlement 归因。生产只读确认有一条过期但仍标记 `active` 的旧日卡 entitlement，关联订阅已过期；另有 4 条到期套餐权益保留 `active` 状态，但代码动态过期判断和准入均会拒绝。当前没有证据表明这些记录仍放行用户请求；不改生产数据，历史审计/账务语义由业务负责人确认。v2.17.7 #492 security penalty 功能另有独立隐私阻断：会新增未加密 `full_prompt` 明文副本且无自动 TTL/保留清理，管理员详情可跨用户读取；生产已有相邻 prompt audit 全文能力但无自动 TTL。该项不依赖 #519 recovery，不能原样带入，须先完成原文访问/保留合同评审。按本表完成每项证据后，才可逐步更新状态；`fully_migrated` 只在整份功能矩阵清零且生产验收完成后改为 `true`。

## 必审正式变更

- 质量运维：凭据错误安全展示、账号保存保留服务端隔离状态、质量规则归属与恢复/删除确认。
- GPT-6.1 Sol：长上下文计费阈值、OpenAI OAuth/Setup Token 豁免边界及最终模型映射。
- 用量时间范围：精确滚动 24 小时窗口、筛选快照、清理日期范围和非 UTC 聚合回查。
- 内容审查用户管理：管理员独立解除禁用入口、原因确认、写入前后状态回读，不改变白名单或其他安全限制。

## 关键上游 PR 差异结论

| 上游 PR | 当前候选结论 | 不能误判为已完成的增量 |
|---|---|---|
| #572 优先调度 | 部分实现，当前候选仍有合并阻断 | 历史刷新未满足缓存/并发/账号数上限；错误率权重无运行逻辑；quality round 未接入调度；legacy percentile 忽略配置。详见 [功能域矩阵](feature-matrix.md) |
| #573 渠道运维 | Fork 适配已部署，部分等价 | 上游 309 迁移、metrics component、完整 handler/workflow 未证明等价；候选保留本 Fork 的 282 编号与结构 |
| #574 凭证异常 | 生产代码已有主要闭环，等价复审未完成 | token guard v2、reauth、refresh-failure reader/writer、API/UI 和 334/335 migration 文件都在生产基线源码；本候选补齐已存在 UI 所调用的预算恢复路由。生产 migration ledger 与上游行为仍待重核；tag 外本地预算扩展不算 #574 等价证据 |
| #575/#576 质量运维/Jev | 当前累计候选部分实现 | 已新增质量计划 API、基础判定/ownership、Jev choice judge、模板/预设和管理员页面；仍缺上游 LLM/live regression、分组/模型动作、跨计划历史与真实 restriction 恢复验收 |
| #577 GPT-6.1 Sol | Fork 等价/部分等价 | 仍需逐项上游差异和生产账务样本核对 |
| #579 滚动 24 小时 | 本地候选已完成，生产待发布 | 生产尚未应用候选 UI/API |
| #580 moderation | Fork 适配已完成，生产待发布 | 候选采用更强 CAS、generation、豁免和动作审计合同，不直接 cherry-pick 上游 |
| #581 版本号 | 策略待定 | 当前仍保留 Fork `0.2.14`，不能声称已切换上游 `2.17.7` |
| request capture / support / harvest | 未迁移 | 需先确认产品范围、隐私和保留合同 |
| security penalty | 隐私阻断 | 上游会新增明文全文副本、跨用户管理员读取且无 TTL，禁止原样带入 |

## 后端差异审查

| 上游能力 | 生产基线证据 | 状态 | 必须交付的验证 |
|---|---|---|---|
| 质量运维 | 生产 #392 尚未部署质量运维入口；候选已补管理员质量规则列表、创建/编辑、运行、历史和 ownership 预览/受控操作 API/UI | 当前候选已支持 deterministic expected-answer、Jev choice judge、模板/预设和 observe/disable-scheduling；live regression、分组处罚、跨计划历史、生产实时回归和真实恢复仍缺 | 先完成候选统一门禁和管理员现场验收，再决定是否纳入上游扩展动作 |
| GPT-6.1 Sol 计费 | 已部署代码覆盖 272000 长上下文、OAuth/Setup Token 不超过 500000 豁免、缓存 token、最终发送模型及 API key 边界（`openai_gateway_usage.go`；`openai_gpt61_sol_long_context_test.go`） | 代码/单测等价已证；生产账务样本待核 | 只读抽查部署前后长上下文样本及余额/冻结价格，不对无法归因请求补扣 |
| 管理端滚动 24 小时用量 | 上游 #579 的后端合同已存在；本候选把精确 ISO 窗口接入真实管理员 Usage 页，默认/刷新生成 `[now-24h, now)`，筛选、分页、排序、导出沿用同一筛选快照，错误请求查询保留精确时间，清理拒绝非日历时间范围 | 代码在 `00b50ef83` 完成并由当前候选继承；定向与全量测试、类型检查、ESLint、构建均通过；尚未推送或部署 | 随累计批次发布后做生产权限/数据验收和 DST 证据 |
| 内容审查用户管理 | 已有日志行用户启用流程要求原因、确认并读回状态，但无法从 UID 独立查找；旧 moderation-owned unban 无原因/版本保护。v2.17.7 正式合同另有 generation/count reset、四类 exemption 和 ownership-aware unban | 候选前后端服务/API、迁移 296/298/301、CAS、原因确认、owned unban 与测试已完成；生产缺对应表和 migration ledger | 单批合并部署后核对 migration ledger、表、权限和审计；覆盖历史 296 快照行、并发状态修改和安全副作用边界 |

以上按生产 `21dd9af22` 代码及对应测试审查得出。尚未实现、测试或生产验收的项不计作完成；每项合入前要链接到本仓实现和测试证据。

## 当前集成规则

1. 逐项比较上游实现、生产已有实现和 Fork 定制，不执行整树合并。
2. 已部署迁移不可改写；迁移必须按完整文件名、checksum 和 schema 依赖审查。
3. 普通用户请求台账入口保持移除，任何新页面必须有真实管理员权限和后端合同。
4. 代码、前端、数据库、文档和测试累计为一个生产发布批次；未通过统一审查前不得合并或部署。

## 下一批次执行队列

1. **全量映射和收口**：GitHub 写授权已完成（`tqytwe`，仓库 `push=true`）；继续产出共同祖先至 v2.17.7 的 merge SHA → 功能簇覆盖表，并完成 priority、quality/Jev、credential/token guard 的完整工作包和隔离 PostgreSQL 预检。全量范围、代码、数据和管理员验收合同通过主线程审查后，再把累计候选推送为一个 PR，只进行一次生产部署。
2. **发布后闭环**：核对 deployment SHA、健康接口、迁移 ledger、296/298/301 checksum、moderation 表/索引/行数、管理员权限和审计；由管理员按交接清单完成 Usage、moderation、渠道运维、优先调度和请求台账现场验收。
3. **priority 完整工作包**：先设计 `request_timing_details`、attempt identity、quality rounds、HTTP/WS admission、penalty clock 和新迁移编号，再实现 API/UI/测试；禁止局部 cherry-pick #572。
4. **quality 完整工作包**：补 ownership/version/隔离、Jev judge、live regression、API/UI 和迁移依赖，完成真实管理员验收；旧 Fork 质量基础只作为兼容边界，不当作上游等价。
5. **credential 完整工作包**：补 token guard v2、OAuth reauth/worker、refresh failure writer/reader/API/UI，重新编号迁移并做隔离 PG 预检；禁止单独接入错误分类器或直接使用上游 309。
6. **范围阻断项**：request capture、support/harvest 先完成产品范围和隐私保留合同；security penalty 先完成全文访问、TTL、跨用户权限和审计设计，未通过前不迁入。
7. **财务与历史数据**：对约 2 万条 `settlement_pending` 做只读业务/财务抽样归因；在证据不足前不补扣、不标记 settled、不清理历史数据。

## 全版本差异矩阵

| 工作包 | 当前负责人 | 交付判据 | 状态 |
|---|---|---|---|
| 来源、共同祖先和差异索引 | 集成负责人 | 锁定 release/tag/commit；将相关上游 PR/功能逐项归类，并记录 Fork 等价证据或迁移决策 | 446 个 SHA 已唯一映射；121 reviewed、325 triaged；生产证据未验证，split 与范围决定仍待关闭 |
| 后端/API 与认证/业务规则 | 后端负责人 | 按真实路由、服务、权限、错误与并发合同核对上游行为；保留 Fork 行为合同 | GPT-6.1 Sol 豁免、显式 last24h 查询已有实现/测试；优先调度仍缺完整 attempt evidence/quality round；质量模板/Jev/ownership、凭据异常主要闭环和内容审查候选代码已完成，生产证据待部署后复核 |
| 前端路由、交互与视觉 | 前端负责人 | 真实管理员/用户路由与 API 联动；覆盖权限、加载/空/错误/成功、保存刷新及响应式 | 本批渠道运维/调度页已部署；管理员真实操作待现场验收；quality/account ops、security audit、ticket、request-capture 等差异仍待逐项定范围和映射 |
| 数据库、迁移与计费 | 数据库/计费负责人 | 对完整文件名、trim 后 SHA-256、顺序、schema、锁/耗时和数据结果做隔离与生产只读核验 | 生产 ledger/checksum、282 与 gateway 账本结构及关键不变量已只读核验；发现约 1.95 万条历史结算待归因。#519 296 tag migration SHA256 为 `2c64246f18a084cc7b1b11ee7c349b626d6e433035f7c52f00a14d112b394ac0`；只读模拟快照为 7 行/2 个活跃用户、单用户最多 6 行、低于当前阈值 10，不会由 migration 直接禁用用户，但仍需审阅 cutoff/current_count 影响。其余 v2.17.7 migration、锁耗时、部署全窗日志和业务账务抽样仍未完成 |
| 集成、测试、提交前复核 | 主线程 | 规格与代码双审、定向/全量测试、构建、Fork integrity、设计和文档门禁；准确记录 PR CI | 本地门禁通过；PR 无 GitHub CI checks，故远端 CI 项记为缺证，不覆盖成通过 |
| 一个生产批次 | 发布负责人 | 一个目标 `play/main` PR、CI、合并、一次部署；验证精确 SHA、健康、关键 API 和业务闭环 | PR #392 已合并，Zeabur 对 `21dd9af22` 部署成功；技术健康与迁移结构已核验，管理员操作、完整部署日志和业务数据审计仍待办 |

矩阵需覆盖共同祖先之后直至 `v2.17.7` 的产品功能域，而非只覆盖末段 release-note 项，也不要求逐提交搬运 446 个 upstream merge。每个功能域要留下纳入、Fork 等价、明确排除或阻断结论和证据；未判定项不能静默跳过。GPT-6.1 Sol 豁免已在生产等价；usage range 的本地 UI 候选、moderation quick-enable 与完整用户解禁合同分别核验，不能混为一项。

已确认的主要前端缺口包含质量/账号运维的上游扩展、工单、安全审计和 request-capture 页面；本候选已补管理员 `/admin/account-quality` 的 deterministic 质量规则创建、编辑、运行、历史和 ownership 操作。现有 `/admin/requests` 是 Fork 自有请求台账，不是上游 request-capture。后端 route guard、角色和菜单 feature flag 必须逐项核对，不能把侧栏隐藏当作直达路由授权。

数据库方面，共同祖先至 v2.17.7 有 97 个 SQL migration 路径变化；生产到目标的直接比对有 89 个上游新增、173 个 Fork 自有、2 个同名异内容及 1 个重命名迁移。`226_channel_monitor_quota_mode.sql` 已部署且 checksum 不同，必须保留原文件。质量运维属于功能闭包评估范围：复用 Fork 的 066/070 定时测试、scheduler outbox 和 273 allowed-model schema；缺失 Pelican 配置、质量状态/判定、隔离所有权、模板/预设、admin API 与 `/admin/account-quality` 页面。`v2.10.5..v2.17.7` 的迁移增量是 44 个 SQL 路径（另有 2 个 Go 迁移测试路径），不是 46 个 SQL。渠道运维迁移已核实可采用独立的 `282_channel_operations.sql`（与现有 `channels`/`accounts` 外键及复合归属约束一致），但必须和 API、权限、审计一起交付；`283_account_oauth_refresh_failures.sql` 依赖当前 Fork 不存在的 token-guard 链路，排出本批，直到 writer/reader/API 和完整依赖同时进入凭证异常任务。其余 284–286 仍是暂定编号，只有线上只读确认最高迁移及隔离 PostgreSQL 验证通过后才可采用。展示页、group tests、showcase/cost 和依赖不存在表的 prompt migration 作为独立功能，不随质量运维盲目带入；最终纳入范围需有功能差异矩阵依据。

## 当前团队任务

> **2026-10-31 集成候选**：工作分支 `codex/v2177-cumulative-integration-20261029` 基于生产 `21dd9af2239c2c7f8259e41e31e21156131622b4`，当前改动未提交。候选包含 Priority 配置化分位数/半开窗口修复、Quality ownership 并发/删除保护、Jev choice judge、规则模板/预设、管理员质量规则 API/UI，以及凭据失败、Token Guard、OAuth reauth、内容审查用户管理和滚动 Usage 的累计改动；Go 全量测试、前端全量测试、lint、类型检查、生产构建、i18n、设计治理和 Fork integrity 已通过。候选尚未提交、推送、创建 PR 或部署；生产 #392 deployment `6ae303acea16512cf7419a17` 的 Zeabur check 成功，首页和 `/health` 为 200。

1. **集成范围/版本矩阵**：继续把共同祖先至 v2.17.7 的相关功能域逐项归为 Fork 等价、纳入迁移、明确排除或阻断，并附代码/测试/迁移证据；不搬运整树、不重做已部署功能。
2. **Priority**：本候选修复首发、重试和 WS 准入绕过；仍需完成完整上游 hard-stop、penalty clock、quality round 联动与跨协议回归。
3. **Quality/Jev**：本候选新增 `/admin/account-quality` 管理员入口、质量规则创建/编辑/运行/历史、ownership 预览和 reconcile/restore/delete 操作，并补充 Jev choice judge、规则模板/预设和同步 runner；仍缺 live regression、跨计划历史、分组/模型处罚、生产实时回归与真实恢复验证。
4. **Credential**：继续核清凭证失败 allowlist、安全展示、writer/reader/API/worker 和上游 OAuth reauth 的完整合同；候选测试通过不等于整条上游能力已迁完。
5. **迁移和生产预检**：隔离 PostgreSQL 验证候选迁移、完整文件名/checksum/顺序/锁耗时；部署前用 Zeabur 只读核实迁移账本和兼容影响。当前环境没有 Zeabur CLI，未做本轮生产数据库读取。
6. **统一交付门禁**：`check-fork-integrity.sh`、Go/前端全量测试、lint、类型检查、生产构建、i18n、设计治理和文档检查均已通过；视觉记录明确静态评审板不是生产浏览器验收截图。候选可进入逐文件复核和一次性提交，但生产管理员浏览器验收仍必须由用户本机完成，不能用静态图替代。
7. **部署后验收**：用户本机管理员按本账本核验 Quality、Usage、moderation、渠道运维、优先调度和请求台账权限/读写/刷新/审计；再核对实际部署 SHA、数据库迁移、日志和关键 API。
8. **独立风险跟进，不扩入本批功能开发**：对约 2 万条 `settlement_pending` 做脱敏只读业务/财务抽样；复核部署日志窗口、PostgreSQL crash recovery 原因及历史日卡审计语义。不得补扣、批量改状态或清理历史数据。
9. **明确排除/阻断**：request capture、support/harvest 等正文采集先完成隐私范围合同；security penalty 在全文访问、保留期限、自动 TTL 和审计约束明确前不迁入。

## 2026-10-31 当前状态

- 生产：`play/main@21dd9af2239c2c7f8259e41e31e21156131622b4`；GitHub Zeabur check 成功，deployment `6ae303acea16512cf7419a17`；首页和 `/health` HTTP 200。候选未部署。
- 工作区：`codex/v2177-cumulative-integration-20261029` 基于生产 SHA，候选改动未提交。没有目标分支开放 PR。
- 本地验证：Go 全量测试、前端全量 Vitest、ESLint、Vue 类型检查、前端 production build、i18n 检查、设计治理、Fork integrity、文档链接和交接检查通过。候选仍未执行隔离 PostgreSQL 迁移预检，也没有远端 PR CI 或生产部署证据。
- 上游矩阵：446 个 merge SHA 均已唯一映射并初步分流（121 reviewed、325 triaged）；生产证据、代码等价和管理员验收仍未闭环；`fully_migrated=false`。
- 功能工作包：Quality 的模板/预设/Jev choice judge 已完成代码与本地测试；Priority attempt evidence/quality round、Quality live regression/真实恢复、Credential 上游逐字段等价复审以及所有生产验收仍有缺口，详见上方队列。
- 生产数据：10 月 29 日快照中的约 2 万条 `settlement_pending`、迁移/日志与历史 entitlement 事项均未在本轮重新读取；保留为独立只读核验项，不据旧快照推断新部署后的数据库状态。

以下保留 2026-10-29 及更早的历史状态和采样，不代表当前部署 SHA。

## 历史状态（截至 2026-10-29）

- 来源锁定：已完成。
- 上游完整差异分类：初步分流已完成；446 个 SHA 唯一对应，121 reviewed、325 triaged。仍需关闭 split、product-scope、evidence、隐私/架构阻断，并为纳入项补代码、测试和生产证据。
- 后端/API：本地候选 `f0bf08935` 已通过 Priority/Quality 定向门禁；Credential 的 Python worker 和前端门禁已通过，Go 被 stale Wire provider 装配阻断；生产尚未应用。Quality 的 Jev/live runner/真实恢复仍未完成；request capture、support/harvest 和 security penalty 仍未完成或被阻断。
- 前端/交互：滚动用量和内容审查用户管理已在本地候选合并并通过全量门禁，未推送/未部署；已发布的渠道运维和调度页面仍需管理员本机验证真实操作、保存刷新、审计与普通用户无入口/无权访问。
- 数据库/计费：生产 ledger 有 416 条；409 个当前迁移均已应用且 checksum 匹配，7 个额外历史记录也匹配 Git 中退役前原文件；0 个当前迁移未应用、0 个 checksum 不匹配。282 结构与 gateway 275–281 关键 schema/约束均核验通过；billing link/task 孤儿、settled+unverified link 和 stale inflight 均为 0。需单列高优先级核验：19,538 条 settlement_pending 请求中 19,537 条无精确 billing link，绝大多数 usage unknown；不能据此认定已扣款或再次扣费，需业务/财务只读抽样对照旧 usage、余额账本、订阅扣额和 dedup/archive 证据。另有历史日卡数据（4,136 replay、5,095 holds、21 entitlements），当前代码无对应读写引用；hold/replay 聚合一致性检查通过。唯一 `active` 但到期的日卡记录属于 `redeem_code`，其对应订阅也已 expired，无 payment order link。另有 4 条套餐权益记录状态 `active` 但已过期，均无有效订阅/替代权益；代码动态过期判定和准入 SQL 会拒绝它们。历史状态不自动修复；迁移锁耗时仍待办；283 不纳入本批。
- CI 与本地门禁：此前候选 Go、前端全量 Vitest、ESLint、typecheck、设计治理、Fork integrity、构建均通过；本轮 Priority 定向 requestledger/repository/service 测试和 Quality 定向 service/repository/handler 测试通过。Credential 定向 Go 测试因耗时未完成，尚未合入主候选；尚未创建 PR，远端 CI 未运行。
- PR、合并、生产部署：PR #389 已合并；merge/deploy SHA 为 `5c16bc5b5965a7b0d7c38aa35671639f045f2dea`；Zeabur deployment `6ae1a532ea16512cf741806a` 成功；首页/健康 200；本地候选 `f0bf08935` 尚未推送，尚未达到整版同步或生产业务验收完成。

PR #389 提交前的本地完整 `make test-backend`、前端 lint/typecheck/全量测试/生产构建、设计治理、Fork integrity 和交接检查均通过；该 PR 没有远端 CI checks。部署后验证了生产分支 SHA、首页/health、构建资源及新 admin API 的未认证拒绝；生产迁移账本和 282、gateway 相关结构已完成只读核验。当前生产 audit 的 Zeabur 日志只有滚动最近 100 行，不能代替完整部署窗；另发现 19,538 条历史 settlement_pending 记录待业务归因。日卡/套餐权益已聚合核对，未发现当前有效权益被过期记录放行；历史审计语义也待业务核验。本批已部署，但以上缺口和未分类上游功能阻止将 `fully_migrated` 改成 true。当前 Usage 候选门禁状态见团队任务记录，不等同于 PR #389 当时已通过的治理检查。

## 生产交接验收清单

### 管理员现场检查

使用用户本机浏览器和现有管理员账号，不在交接文档中记录账号或令牌：

| 检查 | 操作与通过证据 | 状态 |
|---|---|---|
| 渠道运维入口/权限 | 管理员打开 `/admin/channels/operations`，页面可加载；普通用户侧栏没有该入口，直达路由及 API 被拒绝 | 等管理员现场确认 |
| 渠道运维读写 | 查询渠道目录和运维快照；对一项无生产影响的配置做经批准的测试变更，保存后刷新仍一致；冲突 revision 返回可理解错误 | 等管理员现场确认；不要用真实流量开关做未经批准的试验 |
| 审计 | 对上述测试变更确认审计记录含操作者、目标 ID、结果和版本，不含凭证、token、原始上游响应 | 等管理员现场确认 |
| 优先调度 | 管理员打开 `/admin/priority-scheduling`；确认读取/保存/刷新一致、状态明确；候选不得绕过硬准入；无历史/质量数据时显示 unknown/unavailable 而非伪造通过 | 等管理员现场确认 |
| 内容审查快捷启用 | 仅对获准的测试对象核对管理员可见、普通用户不可见、启用前确认和状态回读；不得改变白名单或其他安全限制 | 等管理员现场确认 |
| 请求台账边界 | `/admin/requests` 管理员入口仍可用；普通用户导航不出现入口，直达页面/API 仍按服务端权限拒绝 | 管理员入口和 API 认证已有技术证据；普通用户现场确认待办 |

### 日志与数据库跟踪

| 检查 | 必须取得的证据 | 状态 |
|---|---|---|
| Zeabur 应用/数据库日志 | production `sub2api-wortic` 与 PostgreSQL 当前均 RUNNING；部署仍为 `play/main@5c16bc5`、deployment `6ae1a532`，`/health` 200。应用最新 100 条（10:24:35–10:25:40Z）无 ERROR/WARN/FATAL；较早 10:11:47–10:11:59Z 的 100 条有 11 条含 ERROR/WARN marker，内容不足以安全确定根因。PostgreSQL 08:31:42.948–08:31:43.301Z 有 crash recovery consistency window；08:31:43.366Z ready，约 0.42 秒。09:07:52–09:16:10Z 的 8 条 DB ERROR 均为本轮只读审计 SELECT 拼写/语法失败，连接中止或事务回滚，非应用写入/迁移故障。Zeabur 仅返回服务级最近 100 行，无法核验整段部署日志或平台重启原因 | 当前服务/健康已证；应用历史警告根因、部署完整时间窗和 PostgreSQL 恢复外因仍待平台历史日志 |
| 生产迁移账本 | 只读查询 `schema_migrations` 共 416 条；409 个仓库当前 SQL 文件逐一核对 filename 和 trim 后 SHA-256，另 7 个历史文件对照 Git 删除前版本，均匹配；当前迁移无漏应用或 checksum 差异，`282_channel_operations.sql` 仅一条 | ledger/checksum 已核验 |
| 282 schema 结果 | 两张目标表、`channels.operations_revision NOT NULL DEFAULT 0`、索引、唯一/检查约束及外键均存在；upstream/member 行数为 0/0，revision 空值/负数和成员孤儿均为 0。只读检查未改业务数据；迁移锁等待/耗时未从完整启动日志证实 | 表结构/不变量已核验；运行时锁/耗时待日志 |
| 历史日卡权益/账务 | 日卡相关历史表：replay 4,136、hold 5,095（captured 4,135 / released 960）、entitlement 21（active 1 / exhausted 5 / expired 15）。captured hold 缺 replay=0、completed replay 缺完成时间=0、pending/forwarding/retryable=0；2,547 replay 有 usage log 对应。唯一 `active` 但已过期的记录来源为 `redeem_code`，关联订阅已 expired，无 payment order link；未发现同一账户组的当前有效套餐权益，当前代码树也未读取旧表 | 未见当前有效权益或请求放行风险；历史兑换审计语义由业务负责人确认，不改状态、不清理 |
| 当前套餐权益状态卫生 | 生产有 4 条 `subscription_package_entitlements.status=active` 但 `expires_at` 已过期的记录；均无当前有效订阅、其他有效套餐权益或有效历史日卡权益 | 代码按时间动态判为 expired，准入 SQL 同时要求 `expires_at > now()`；未见当前权益放行风险，不自动修复状态 |
| 请求/错误日志 | Zeabur runtime API 只返回最近 100 行，当前应用样本无错误级别记录，较早样本出现 11 条 ERROR/WARN marker；完整部署前后 5xx、关键 admin API、DB、scheduler 和用户请求影响不可由此证明 | 部分采样已核验；完整窗口和业务影响待办 |
| Gateway 结算与用量归因 | 10:23 UTC 只读聚合：19,538 条 `settlement_pending`；19,252 >1h、19,044 >24h、14,344 >7d、never reconciled=2。19,537 条没有 billing link；状态组合主要为 failed/usage_unknown 17,073、cancelled/usage_unknown 2,250、interrupted/usage_unknown 190、succeeded/usage_unknown 18，另有 succeeded/known 1 和新鲜 inflight/pending 5。剩余一条 link 未验证、未证明扣费；subscription outbox 81 全 completed、pending receipt=0。`settlement_state` 是请求账本状态，不能代替扣款证明 | 高优先级待业务/财务脱敏抽样，关联 usage log、dedup/archive proof、余额流水和订阅扣额；不得批量修状态或补扣 |
| v2.17.7 #492 security penalty 明文保留审查 | `293_security_penalties.sql` 新增 `security_penalty_events.full_prompt TEXT`；writer 明文保存至多 100,000 runes，无自动 TTL/cleanup；admin detail 可跨用户取全文，用户只能取自己的。当前 Fork 的 `prompt_audit_events` 已存最多 65,536 runes 明文，也只有人工删除、无 TTL，因此原样迁入会额外保存副本并扩大读取范围。#293 DDL/运行时不依赖 #519 recovery，不应把两者捆绑 | 阻断原样迁入；先确认全文是否必须持久化、管理员最小读取范围、保留期限/自动清理和审计合同；只读源码证据，不读取生产 prompt 正文 |
| GPT-6.1 Sol 计费与用量 | 核对部署前后长上下文、余额、冻结价格和订阅样本；避免将 unknown usage 当作实际扣款 | 待业务数据只读抽样 |

生产检查必须由具备相应权限的管理员/数据库维护者执行并把脱敏结果回填本表。禁止将生产 DSN、token、用户凭据或完整请求正文写入 issue、日志摘录或仓库。无证据的条目保持“待办”，不要用 `/health` 200、部署成功或单元测试代替。

## 2026-10-28 最新生产复核覆盖

## 2026-10-29 当前生产复核覆盖

### 2026-10-29 后续只读复核（覆盖本节较早统计）

- 当前本地候选为 `codex/v2.17.7-moderation-release-20261028@f0bf08935`；生产仍运行 `play/main@5c16bc5b5965a7b0d7c38aa35671639f045f2dea`，deployment `6ae1a532ea16512cf741806a`。候选未推送、未创建 PR、未合并、未部署；`fully_migrated=false`。
- 本次应用服务滚动 100 条日志为 INFO 99、WARN 1、ERROR/FATAL 0。唯一 WARN 是 `account_select_failed`（模型 `gpt-5.6-sol`、pool 2、excluded 1、runtime_blocked 1）。这表明当前请求未从该池选出账号；手头证据尚不足以归因两个排除原因，也不能据此断定候选代码有问题。需从账号池状态、排除策略和 runtime block 指标继续只读追查。服务 API 仅提供最近滚动日志，不能证明整个部署窗口无异常。
- 生产 PostgreSQL `schema_migrations=416`，最新为 `282_channel_operations.sql`；内容审查用户状态、命中、动作表及 security penalty 表仍不存在。`/api/v1/requests` 返回 404，未认证 `/api/v1/admin/requests` 返回 401；生产首页与 `/health` 为 200。
- 最新只读聚合为 20,088 条 `gateway_requests.settlement_state=settlement_pending`，全部 `usage_unknown`；另见 4 条 pending 和 1 条 known 用量记录。该状态不证明扣款结果，必须按只读 usage、余额流水、订阅扣额及幂等证据抽样归因；不得批量标记 settled、补扣或清理记录。
- Go `./...` 在当前候选通过。前端全量 Vitest 已通过（513 个文件、3,957 项通过、3 个文件/8 项跳过）；`pnpm` shim 缺失导致原组合脚本中断，随后直接运行本地 ESLint、设计治理、i18n 完整性、`vue-tsc --noEmit`、`vite build` 与 `scripts/check-fork-integrity.sh` 均通过。构建有现存 chunk 导入方式及 Browserslist 数据时效提示，不影响退出状态。完整本地门禁通过；GitHub 远端 PR CI 尚未运行。
- 本轮生产读取保持只读。部分早期只读 SQL 曾因字段名或引号错误失败，没有生产写入；这些是审计命令错误，不是应用故障。

以下两次 10 月 29 日采样记录较早的同日观察；日志计数和数据库聚合以本节开头的“后续只读复核”为准。

- Zeabur 项目 `6a51de14c2881a93656fa4c5`、环境 `6a51de14104975fcb46761c3` 已通过 CLI 读取；应用服务、PostgreSQL 和 Redis 当前为 `RUNNING`。应用 deployment 仍为 `6ae1a532ea16512cf741806a`，对应 `play/main@5c16bc5b5965a7b0d7c38aa35671639f045f2dea`；`/` 与 `/health` 均返回 200。
- 应用服务最近 100 条 runtime 日志为 INFO 84、WARN 16、ERROR/FATAL 0。WARN 已分类为 observe 模式无审核 API key，以及上游账号 403/503 引起的重试、failover 和 transient cooldown；当前没有证据把这些 WARN 归因于本地候选代码。Zeabur CLI 只提供服务级滚动窗口，不能证明完整部署窗口无异常。
- PostgreSQL 只读查询显示 `schema_migrations` 416 条，最新文件仍为 `282_channel_operations.sql`；`content_moderation_user_states`、`content_moderation_user_hits`、`content_moderation_user_actions` 均不存在。候选 moderation 路由和迁移尚未进入生产。
- `gateway_requests` 当前约 2 万条 `settlement_pending`，绝大多数 `usage_unknown` 且没有精确 billing link；该状态不等于已扣款或未扣款，业务/财务必须按只读 usage、余额流水、订阅扣额和幂等证据抽样归因。禁止批量标记 settled、补扣或清理历史记录。

以下结果覆盖本账本中较早的生产采样数字：

本次状态修订：候选 moderation 用户管理与滚动 24 小时用量已在本地合并提交 `00b50ef83`，并通过本地全量门禁；当前生产仍未应用这些迁移和前端候选。

- Zeabur 当前服务 `sub2api-wortic` 与 PostgreSQL 均为 `RUNNING`；Git trigger 为 `refs/heads/play/main`，部署 SHA 为 `5c16bc5b5965a7b0d7c38aa35671639f045f2dea`，首页和 `/health` 返回 200。Deployment API 的 `status=RUNNING` 与 `finishedAt` 同时出现，不能单凭 status 字段宣称终态成功。
- 应用最新滚动 100 条为 INFO 84、WARN 16、ERROR/FATAL 0；WARN 主要是 `content_moderation.skip_no_audit_api_keys`，以及上游账号 403/503 导致的重试、切换和暂时熔断。当前采样没有应用 ERROR/FATAL，不把这些 WARN 直接归因于本地候选代码。
- PostgreSQL `schema_migrations` 为 416 条；`293/296/298/301` 均不存在。`content_moderation_logs` 存在，moderation user state/hits/actions 与 security penalty 表不存在。候选 moderation 路由在生产返回 404，证明候选尚未部署；应用启动迁移路径曾在已部署迁移写入后运行，缺失候选迁移是代码版本差异而非迁移器未运行证据。
- 最新只读聚合约有 2 万条 `settlement_pending`，绝大多数为 `usage_unknown` 且没有精确 billing link。该状态不证明已扣款，必须等待业务/财务只读归因，不得批量改状态或补扣。
- PostgreSQL 最近日志中的 ERROR/FATAL 来自本轮只读审计命令的列名/超时参数错误及管理员终止并行 worker，没有证据表明是应用迁移或业务写入故障；生产日志原文不进入仓库。
