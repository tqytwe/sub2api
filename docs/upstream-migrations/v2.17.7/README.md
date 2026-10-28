# v2.17.7 同步批次账本

状态：`analysis`。本账本记录从生产基线 `526f09d36fa4d0a69af823fec2fc5a9ed495193f` 对齐上游正式 `v2.17.7` 的审查范围；不代表已合并或已部署。

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

## 必审正式变更

- 质量运维：凭据错误安全展示、账号保存保留服务端隔离状态、质量规则归属与恢复/删除确认。
- GPT-6.1 Sol：长上下文计费阈值、OpenAI OAuth/Setup Token 豁免边界及最终模型映射。
- 用量时间范围：精确滚动 24 小时窗口、筛选快照、清理日期范围和非 UTC 聚合回查。
- 内容审查用户管理：管理员独立解除禁用入口、原因确认、写入前后状态回读，不改变白名单或其他安全限制。

## 后端差异审查

| 上游能力 | 生产基线证据 | 状态 | 必须交付的验证 |
|---|---|---|---|
| 质量运维 | 生产缺少上游新增 quality ownership/rule/preview/restore/delete 的完整后端合同。credential-failure 草稿在另一 worktree，writer、reader、API 和 #283 schema 尚未集成；错误分类目前基于错误文本匹配，需证明 allowlist 与安全输出 | 缺实现；渠道运维候选不可代替 | 按上游 #576 检查保存并发和隔离状态、规则 ownership/version；凭据错误 mock HTTP 和错误码 allowlist；writer/readback/API/权限测试 |
| GPT-6.1 Sol 计费 | 生产已有 272000 长上下文计费基础；未找到 OAuth/Setup Token 500000 豁免、最终发送模型判定、自定义区间价优先级的完整合同证据 | 部分实现，待逐项核验 | 增补或迁入上游边界测试，覆盖 token 缓存、账号类型、最终模型映射及自定义价格优先级 |
| 管理端滚动 24 小时用量 | 生产有时间范围、时区和清理基础；未发现精确 `[now-24h, now)` 默认值、跨筛选/翻页/排序/导出共享时间快照、非 UTC/不完整聚合桶回查的证据 | 待实现/差异核验 | 对照上游 #579 API/UI；冻结应用层 now；覆盖时区边界、精确清理拒绝和原始日志回查 |
| 内容审查用户管理 | 生产 `UnbanUser` 直接设置 active 并清认证缓存；现有合同不收 reason，前端也无独立管理动作及状态读回 | 明确缺失 | 单独管理员动作要求原因与确认；写入前确认仍为 disabled、写入后回读 active；不宣称 PUT 有 version 前置条件；证明不修改白名单、计数和其他安全限制 |

以上按生产 `526f09d36` 静态代码审查得出。尚未实现、测试或生产验收的项不计作完成；每项合入前要链接到本仓实现和测试证据。

## 当前集成规则

1. 逐项比较上游实现、生产已有实现和 Fork 定制，不执行整树合并。
2. 已部署迁移不可改写；迁移必须按完整文件名、checksum 和 schema 依赖审查。
3. 普通用户请求台账入口保持移除，任何新页面必须有真实管理员权限和后端合同。
4. 代码、前端、数据库、文档和测试累计为一个生产发布批次；未通过统一审查前不得合并或部署。

## 全版本差异矩阵

| 工作包 | 当前负责人 | 交付判据 | 状态 |
|---|---|---|---|
| 来源、共同祖先和差异索引 | 集成负责人 | 锁定 release/tag/commit；按上游 merge/PR 和功能域建立清单，逐项标为 Fork 已有、需要迁入/适配、重复/不适用或阻断 | 进行中；共同祖先和差异规模已核实 |
| 后端/API 与认证/业务规则 | 后端负责人 | 对真实路由、服务、权限、错误与并发行为给出差异和测试证据；保留 Fork 行为合同 | usage/GPT-6.1 候选测试通过；渠道运维审计字段已补；priority scheduling 仍只有配置 CRUD，运行时与有效 snapshot 未完成 |
| 前端路由、交互与视觉 | 前端负责人 | 只改真实生产页面和 API；验证权限、按钮/导航、加载/空/错误/成功状态及响应式；不以静态原型代替功能 | quick-enable 定向检查通过；全版本路由盘点进行中 |
| 数据库、迁移与计费 | 数据库/计费负责人 | 按文件名、checksum、执行顺序、schema 依赖及计费合同审查；冲突以兼容向前迁移解决并完成隔离 PostgreSQL 验证 | 282 可随渠道运维纳入；283 排出本批；线上 ledger 与隔离 PostgreSQL 尚未验证 |
| 集成、测试、提交前复核 | 主线程 | 规格、代码质量、前后端/API/DB 交叉审查；定向及全量测试、构建、Fork integrity、设计与文档门禁全部通过 | 未开始 |
| 一个生产批次 | 发布负责人 | 一个目标 `play/main` 的 PR/merge/deploy 批次；记录精确部署 SHA、健康和受影响业务验收 | 未开始 |

矩阵需覆盖共同祖先之后直至 `v2.17.7` 的相关上游功能，而非只覆盖下面的四个 release-note 项。逐条清单不要求复制所有上游提交；每项必须留下范围结论和生产等价实现/适配证据，未判定项不能静默跳过。现有 `usage range`、GPT-6.1 Sol 和 moderation quick-enable 候选属于最后一段增量工作包，不能标记整版同步完成。

已确认的主要前端缺口包含质量/账号运维、优先调度、渠道运维、工单、安全审计和 request-capture 页面；现有 `/admin/requests` 是 Fork 自有请求台账，不是上游 request-capture。后端 route guard、角色和菜单 feature flag 必须逐项核对，不能把侧栏隐藏当作直达路由授权。

数据库方面，共同祖先至 v2.17.7 有 97 个 SQL migration 路径变化；生产到目标的直接比对有 89 个上游新增、173 个 Fork 自有、2 个同名异内容及 1 个重命名迁移。`226_channel_monitor_quota_mode.sql` 已部署且 checksum 不同，必须保留原文件。质量运维属于功能闭包评估范围：复用 Fork 的 066/070 定时测试、scheduler outbox 和 273 allowed-model schema；缺失 Pelican 配置、质量状态/判定、隔离所有权、模板/预设、admin API 与 `/admin/account-quality` 页面。`v2.10.5..v2.17.7` 的迁移增量是 44 个 SQL 路径（另有 2 个 Go 迁移测试路径），不是 46 个 SQL。渠道运维迁移已核实可采用独立的 `282_channel_operations.sql`（与现有 `channels`/`accounts` 外键及复合归属约束一致），但必须和 API、权限、审计一起交付；`283_account_oauth_refresh_failures.sql` 依赖当前 Fork 不存在的 token-guard 链路，排出本批，直到 writer/reader/API 和完整依赖同时进入凭证异常任务。其余 284–286 仍是暂定编号，只有线上只读确认最高迁移及隔离 PostgreSQL 验证通过后才可采用。展示页、group tests、showcase/cost 和依赖不存在表的 prompt migration 作为独立功能，不随质量运维盲目带入；最终纳入范围需有功能差异矩阵依据。

## 当前团队任务

- **集成/项目管理（主线程）**：已在线复核 v2.17.7 latest Release 和精确 tag；锁定生产基线、共同祖先、当前 `VERSION` 与双边差异规模；维护全版本功能矩阵并检查重复实现和工作包依赖。
- **后端/API**：usage range 与 GPT-6.1 Sol 候选已通过 admin handler、repository、service unit packages；gofmt 和 diff check 通过。当前负责渠道运维/优先调度候选，核对 #572/#573 的后端合同和配置生效。
- **前端/交互**：moderation quick-enable 的 33 项 Vitest、typecheck、定向 ESLint 通过；全版本路由盘点已确认多组管理台缺口。当前负责渠道运维/优先调度真实 admin UI 与后端合同联调。
- **数据库/计费**：已完成 migration 差异与质量运维闭包审计；当前检查渠道运维 migration。线上 migration ledger 与隔离 PostgreSQL 尚未验证，Docker context 未确认安全前不启动数据库。

## 当前状态

- 来源锁定：已完成。
- 上游完整差异分类：进行中；共同祖先与规模已核验。不得只依据 release 摘要或编号差异跳过 v2.17.7 之前的上游已发布功能。
- 后端/API：已验证 usage/GPT 末段候选；渠道运维/调度工作包进行中。完整版本差异仍需按功能矩阵逐项补齐。
- 前端/交互：quick-enable 测试已通过；已识别上游专用管理台与 Fork 路由的差异，渠道运维 UI 工作包进行中。
- 数据库/计费：97 个上游 migration 路径变化及生产直接差异已审查；质量运维闭包和新编号方案已列出；渠道运维迁移、隔离 PostgreSQL 和线上 migration ledger 仍待核验。
- 集成/来源：GitHub Release API 与精确 tag 在线核验通过；release 检查器定向单测 4 项通过；完整功能差异矩阵仍在进行。
- PR、合并、生产部署：未完成。

后端候选定向验证：`GOTOOLCHAIN=local go test -tags=unit ./internal/handler/admin -count=1`、`./internal/repository`、`./internal/service` 全部通过；gofmt 与 `git diff --check` 通过。前端候选 33 项 Vitest、`vue-tsc --noEmit` 和定向 ESLint 通过；设计治理检查仍失败，因为视觉审查缺少真实截图 manifest。数据库迁移尚未在隔离 PostgreSQL 上验证，线上 `schema_migrations` 未核实。以上仅为候选包验证，不代表全量检查、提交或生产完成。
