# v2.17.7 同步批次账本

状态：`deployed-partial / migration-incomplete`。同步批次 PR #389 已于 2026-10-28 合并并部署，但这只证明本批候选上线，不代表完整 v2.17.7 功能审计或迁移完成。`fully_migrated=false` 继续有效。

本目标是版本功能同步，不能以 v2.17.7 release notes 的末尾修复代替完整差异盘点。生产与上游源码的共同祖先为 `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`（上游 `0.2.5`）；上游从该点至目标 tag 有 2,155 个提交、446 个 first-parent merge，按共同祖先直接比较有 3,809 个路径变化。以上数字只界定审查范围，不表示需要照搬这些提交或文件。Fork 自有实现必须逐项保留/映射；仅纳入相关产品能力和必要安全兼容修复。

## 来源锁定

| 字段 | 值 |
|---|---|
| 来源仓库 | `ranxi2001/sub2api` |
| Release | `v2.17.7` |
| Tag object | `af9a09cf92c1cbfdcf793862122d46a5a29a6da1` |
| Release commit | `9620d315f72a7dbf04fa10fca02970e1e410805e` |
| 上游共同祖先 | `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a` (`0.2.5`) |
| 生产分析基线 | `origin/play/main@526f09d36fa4d0a69af823fec2fc5a9ed495193f` |
| fully migrated | `false` |

## 已发布批次与整版状态

| 检查项 | 当前证据 | 结论 |
|---|---|---|
| 合并 | PR [#389](https://github.com/tqytwe/sub2api/pull/389)，merge commit `5c16bc5b5965a7b0d7c38aa35671639f045f2dea`，父提交为生产基线 `526f09d36` 与候选 head `82578be73` | 已合并 |
| 部署 | Zeabur deployment `6ae1a532ea16512cf741806a`，GitHub Zeabur check `122690278483` 对 merge SHA 报告 `success` | 已部署 |
| 运行态技术核对 | `origin/play/main` 为 merge SHA；首页和 `/health` 返回 200；部署 index 引用渠道运维/优先调度资源；两条新 admin snapshot API 未认证返回 401 | 技术证据通过，不能替代管理员业务验收 |
| PR CI | PR head 没有 GitHub CI check run；手动 dispatch 返回 `Actions has been disabled for this repository` | 缺少远端 CI 证据；本地门禁不能伪写成 PR CI 通过 |
| 管理员闭环 | 没有本机管理员浏览器操作、保存刷新、审计记录和调度效果的现场证据 | 待管理员验收 |
| 数据库生产账本 | Zeabur PostgreSQL 只读查得 416 条；仓库当前 409 个迁移文件均已登记且 trim 后 SHA-256 一致，另 7 条已退役文件与 Git 历史原文件一致；当前文件无未应用项或 checksum 差异 | ledger/checksum 已核验；迁移锁耗时和全量数据业务不变量尚未全部核验 |
| 上游完整功能差异 | 共同祖先至 tag 的 446 个上游 PR merge 尚未逐项形成 Fork 等价实现/适配/排除结论 | 未完成，不能宣称整版迁移 |

本批代码已部署与“上游整版同步完成”是两个不同状态。已知未完成或未证实项至少包括质量运维/Jev 与凭据失败安全分类、管理员精确滚动 24 小时用量候选的全批审查/部署、内容审查用户管理后端闭包、完整上游功能差异映射、PostgreSQL 更早部署窗口/平台重启原因、历史日卡审计语义、管理员真实交互验收及历史 gateway settlement 归因。生产只读确认有一条过期但仍标记 `active` 的旧日卡 entitlement，关联订阅已过期；另有 4 条到期套餐权益保留 `active` 状态，但代码动态过期判断和准入均会拒绝。当前没有证据表明这些记录仍放行用户请求；不改生产数据，历史审计/账务语义由业务负责人确认。v2.17.7 #492 security penalty 功能另有独立隐私阻断：会新增未加密 `full_prompt` 明文副本且无自动 TTL/保留清理，管理员详情可跨用户读取；生产已有相邻 prompt audit 全文能力但无自动 TTL。该项不依赖 #519 recovery，不能原样带入，须先完成原文访问/保留合同评审。按本表完成每项证据后，才可逐步更新状态；`fully_migrated` 只在整份功能矩阵清零且生产验收完成后改为 `true`。

## 必审正式变更

- 质量运维：凭据错误安全展示、账号保存保留服务端隔离状态、质量规则归属与恢复/删除确认。
- GPT-6.1 Sol：长上下文计费阈值、OpenAI OAuth/Setup Token 豁免边界及最终模型映射。
- 用量时间范围：精确滚动 24 小时窗口、筛选快照、清理日期范围和非 UTC 聚合回查。
- 内容审查用户管理：管理员独立解除禁用入口、原因确认、写入前后状态回读，不改变白名单或其他安全限制。

## 后端差异审查

| 上游能力 | 生产基线证据 | 状态 | 必须交付的验证 |
|---|---|---|---|
| 质量运维 | 生产缺少上游新增 quality ownership/rule/preview/restore/delete 的完整后端合同。credential-failure 草稿在另一 worktree，writer、reader、API 和 #283 schema 尚未集成；错误分类目前基于错误文本匹配，需证明 allowlist 与安全输出 | 缺实现；渠道运维候选不可代替 | 按上游 #576 检查保存并发和隔离状态、规则 ownership/version；凭据错误 mock HTTP 和错误码 allowlist；writer/readback/API/权限测试 |
| GPT-6.1 Sol 计费 | 已部署代码覆盖 272000 长上下文、OAuth/Setup Token 不超过 500000 豁免、缓存 token、最终发送模型及 API key 边界（`openai_gateway_usage.go`；`openai_gpt61_sol_long_context_test.go`） | 代码/单测等价已证；生产账务样本待核 | 只读抽查部署前后长上下文样本及余额/冻结价格，不对无法归因请求补扣 |
| 管理端滚动 24 小时用量 | 上游 #579 的后端合同已存在；本候选把精确 ISO 窗口接入真实管理员 Usage 页，默认/刷新生成 `[now-24h, now)`，筛选、分页、排序、导出沿用同一筛选快照，错误请求查询保留精确时间，清理拒绝非日历时间范围 | 本地候选实现；定向测试通过，尚未完成全批检查、合并或部署 | 补验 DST 行为并完成统一集成审查；累计到同一生产批次 |
| 内容审查用户管理 | 已有日志行用户启用流程要求原因、确认并读回状态，但无法从 UID 独立查找；旧 moderation-owned unban 无原因/版本保护。v2.17.7 正式合同另有 generation/count reset、四类 exemption 和 ownership-aware unban | 前端 UID 列表/详情/动作候选已完成并测试；后端服务、API、迁移及前后端联调进行中；未合并/未部署 | 按 tag 实现 list/detail/action 状态、四类 exemption、generation/reset、自动 hit 幂等与 ownership/version guarded unban；旧日志启用流程保留，旧无合同 unban 按钮不再使用；覆盖历史 296 快照行、并发状态修改和安全副作用边界 |

以上按生产 `5c16bc5b` 代码及对应测试审查得出。尚未实现、测试或生产验收的项不计作完成；每项合入前要链接到本仓实现和测试证据。

## 当前集成规则

1. 逐项比较上游实现、生产已有实现和 Fork 定制，不执行整树合并。
2. 已部署迁移不可改写；迁移必须按完整文件名、checksum 和 schema 依赖审查。
3. 普通用户请求台账入口保持移除，任何新页面必须有真实管理员权限和后端合同。
4. 代码、前端、数据库、文档和测试累计为一个生产发布批次；未通过统一审查前不得合并或部署。

## 全版本差异矩阵

| 工作包 | 当前负责人 | 交付判据 | 状态 |
|---|---|---|---|
| 来源、共同祖先和差异索引 | 集成负责人 | 锁定 release/tag/commit；将相关上游 PR/功能逐项归类，并记录 Fork 等价证据或迁移决策 | 来源已锁定；446 个 merge 尚未全部分类，整版范围评估未完成 |
| 后端/API 与认证/业务规则 | 后端负责人 | 按真实路由、服务、权限、错误与并发合同核对上游行为；保留 Fork 行为合同 | GPT-6.1 Sol 豁免、显式 last24h 查询已有实现/测试；优先调度仅部分等价；质量运维/Jev、凭据异常闭环、内容审查用户独立解禁合同缺失 |
| 前端路由、交互与视觉 | 前端负责人 | 真实管理员/用户路由与 API 联动；覆盖权限、加载/空/错误/成功、保存刷新及响应式 | 本批渠道运维/调度页已部署；管理员真实操作待现场验收；quality/account ops、security audit、ticket、request-capture 等差异仍待逐项定范围和映射 |
| 数据库、迁移与计费 | 数据库/计费负责人 | 对完整文件名、trim 后 SHA-256、顺序、schema、锁/耗时和数据结果做隔离与生产只读核验 | 生产 ledger/checksum、282 与 gateway 账本结构及关键不变量已只读核验；发现约 1.95 万条历史结算待归因。#519 296 tag migration SHA256 为 `2c64246f18a084cc7b1b11ee7c349b626d6e433035f7c52f00a14d112b394ac0`；只读模拟快照为 7 行/2 个活跃用户、单用户最多 6 行、低于当前阈值 10，不会由 migration 直接禁用用户，但仍需审阅 cutoff/current_count 影响。其余 v2.17.7 migration、锁耗时、部署全窗日志和业务账务抽样仍未完成 |
| 集成、测试、提交前复核 | 主线程 | 规格与代码双审、定向/全量测试、构建、Fork integrity、设计和文档门禁；准确记录 PR CI | 本地门禁通过；PR 无 GitHub CI checks，故远端 CI 项记为缺证，不覆盖成通过 |
| 一个生产批次 | 发布负责人 | 一个目标 `play/main` PR、CI、合并、一次部署；验证精确 SHA、健康、关键 API 和业务闭环 | PR #389 已合并，Zeabur 对 `5c16bc5b` 部署成功；技术健康与迁移结构已核验，管理员操作、完整部署日志和业务数据审计仍待办 |

矩阵需覆盖共同祖先之后直至 `v2.17.7` 的产品功能域，而非只覆盖末段 release-note 项，也不要求逐提交搬运 446 个 upstream merge。每个功能域要留下纳入、Fork 等价、明确排除或阻断结论和证据；未判定项不能静默跳过。GPT-6.1 Sol 豁免已在生产等价；usage range 的本地 UI 候选、moderation quick-enable 与完整用户解禁合同分别核验，不能混为一项。

已确认的主要前端缺口包含质量/账号运维、优先调度、渠道运维、工单、安全审计和 request-capture 页面；现有 `/admin/requests` 是 Fork 自有请求台账，不是上游 request-capture。后端 route guard、角色和菜单 feature flag 必须逐项核对，不能把侧栏隐藏当作直达路由授权。

数据库方面，共同祖先至 v2.17.7 有 97 个 SQL migration 路径变化；生产到目标的直接比对有 89 个上游新增、173 个 Fork 自有、2 个同名异内容及 1 个重命名迁移。`226_channel_monitor_quota_mode.sql` 已部署且 checksum 不同，必须保留原文件。质量运维属于功能闭包评估范围：复用 Fork 的 066/070 定时测试、scheduler outbox 和 273 allowed-model schema；缺失 Pelican 配置、质量状态/判定、隔离所有权、模板/预设、admin API 与 `/admin/account-quality` 页面。`v2.10.5..v2.17.7` 的迁移增量是 44 个 SQL 路径（另有 2 个 Go 迁移测试路径），不是 46 个 SQL。渠道运维迁移已核实可采用独立的 `282_channel_operations.sql`（与现有 `channels`/`accounts` 外键及复合归属约束一致），但必须和 API、权限、审计一起交付；`283_account_oauth_refresh_failures.sql` 依赖当前 Fork 不存在的 token-guard 链路，排出本批，直到 writer/reader/API 和完整依赖同时进入凭证异常任务。其余 284–286 仍是暂定编号，只有线上只读确认最高迁移及隔离 PostgreSQL 验证通过后才可采用。展示页、group tests、showcase/cost 和依赖不存在表的 prompt migration 作为独立功能，不随质量运维盲目带入；最终纳入范围需有功能差异矩阵依据。

## 当前团队任务

- **集成/项目管理（主线程）**：已在线复核 v2.17.7 latest Release 和精确 tag；锁定生产基线、共同祖先、当前 `VERSION` 与双边差异规模；维护全版本功能矩阵并检查重复实现和工作包依赖。
- **后端/API**：生产已覆盖 GPT-6.1 Sol 豁免和显式 last24h 后端合同。优先调度仅部分等价；内容审查用户管理后端正在实现 v2.17.7 的状态版本、generation/hit 与动作审计合同。#293 security penalty 源码已确认无 #519 DDL 依赖，但其全文明文访问/保留设计列为单独隐私阻断，不在本闭包原样迁入。质量运维/Jev 与凭据安全分类仍需继续。
- **Usage UI 候选**：管理员滚动 24 小时窗口的 ISO 范围、刷新快照、筛选/分页/排序/导出一致性、错误请求半开日期边界与清理日历范围保护已实现；DateRangePicker 和 UsageView 定向测试 26 项通过，router/i18n/请求台账权限回归 122 项通过；自定义日期路由刷新保持日历区间。TypeScript、生产构建和针对性 ESLint 通过。Fork integrity 的设计治理子项因要求受影响真实页面的结构化渲染截图而未通过；不能用原型或假 API 替代生产页面证据，当前无管理员认证的可核验截图。DST 行为和证据门禁仍待解决；未提交/未部署。
- **前端/交互**：生产已有从日志行执行的管理员启用流程（原因/确认、前后状态回读），本轮不重复开发。当前分支已加入真实 `/admin/risk-control` UID/状态筛选、分页、详情、四类 exemption、generation 计数 reset 和仅 moderation-owned disabled 状态的版本化 unban；旧无版本快速 unban 按钮已移除。前端定向 Vitest 37 项、typecheck 通过；尚未获得对应后端合同，不算联调完成；ESLint 工具当前未安装，不新增依赖。
- **数据库/计费**：生产 `schema_migrations` 有 416 条；409 个当前文件逐项 checksum 匹配，7 个退役文件有历史匹配证据。282 相关表、索引与约束存在，唯一 NOT VALID 约束为迁移明确设计的 bounds 检查，异常行数为 0；gateway 275–281 关键表/索引存在，settled+unverified link、billing link 孤儿、task 孤儿和超 10 分钟 inflight 均为 0。另有 19,538 条 `gateway_requests.settlement_state=settlement_pending`，19,537 条无精确 billing link，主要为 failed/cancelled/interrupted 且 usage unknown；1 条有未验证 link，其 usage log 未标记 billing settled 且无 dedup proof。状态不证明已扣款，需管理员/财务从只读 usage、余额账本、订阅和幂等证据抽样归责，严禁批量标记 settled 或补扣。296 migration 若部署会按 tag 逻辑快照 7 条旧命中；迁移本身不改用户状态，需在上线前确认当前阈值与统计边界。历史日卡与套餐权益陈旧状态也保持只读，不修数、不清理。

## 当前状态

- 来源锁定：已完成。
- 上游完整差异分类：未完成；446 个上游 PR merge 尚未逐项完成 Fork 等价/迁移/排除判定。
- 后端/API：生产已部署部分同步能力；质量运维/Jev、凭据错误安全闭环和内容审查用户全栈闭包仍未完成。本地后端实现隔离在 `/data/projects/sub2api-moderation-independent-unban`，尚无可验证通过的后端测试交付。
- 前端/交互：滚动用量和内容审查用户管理都只有本地候选，未合入/未部署；RiskControl 候选等待后端联调。已发布的渠道运维和调度页面仍需管理员本机验证真实操作、保存刷新、审计与普通用户无入口/无权访问。
- 数据库/计费：生产 ledger 有 416 条；409 个当前迁移均已应用且 checksum 匹配，7 个额外历史记录也匹配 Git 中退役前原文件；0 个当前迁移未应用、0 个 checksum 不匹配。282 结构与 gateway 275–281 关键 schema/约束均核验通过；billing link/task 孤儿、settled+unverified link 和 stale inflight 均为 0。需单列高优先级核验：19,538 条 settlement_pending 请求中 19,537 条无精确 billing link，绝大多数 usage unknown；不能据此认定已扣款或再次扣费，需业务/财务只读抽样对照旧 usage、余额账本、订阅扣额和 dedup/archive 证据。另有历史日卡数据（4,136 replay、5,095 holds、21 entitlements），当前代码无对应读写引用；hold/replay 聚合一致性检查通过。唯一 `active` 但到期的日卡记录属于 `redeem_code`，其对应订阅也已 expired，无 payment order link。另有 4 条套餐权益记录状态 `active` 但已过期，均无有效订阅/替代权益；代码动态过期判定和准入 SQL 会拒绝它们。历史状态不自动修复；迁移锁耗时仍待办；283 不纳入本批。
- CI 与本地门禁：本地全量后端/前端测试、lint、构建及治理检查通过；PR head 无远端 CI check run，缺口单独记录。
- PR、合并、生产部署：PR #389 已合并；merge/deploy SHA 为 `5c16bc5b5965a7b0d7c38aa35671639f045f2dea`；Zeabur deployment `6ae1a532ea16512cf741806a` 成功；首页/健康 200；尚未达到整版同步或生产业务验收完成。

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
