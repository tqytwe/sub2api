# ranxi2001/sub2api v2.10.3 核心迁移账本

状态：固定分析账本与分批范围；不是实时状态或完整迁移/部署完成证明。

## 当前交接从这里开始

本目录只在 [handoff.json](./handoff.json) 维护带核查时间的工作项状态、PR/SHA、
各层验证/部署/生产验收、负责人待分配、风险及后续任务。维护规则见
[工程交接入口](../../PROJECT_HYGIENE.md)。不要把本页下面的初始分析改写成实时看板。

来源锁仅维护来源、分析基线与 `fully_migrated`；旧 `verified_batches` / `pending_batches`
初始计划已移至交接快照，避免 #338/#339 完成后仍有另一份待办状态。
后续变更先重查快照链接中的 PR/head/base/CI，再更新同一记录；不把后端实现写成 UI 闭环已完成。

### 路由审查范围

快照 `route_review` 保存固定来源/目标 SHA 链接及 17 个源独有路由的逐条分类和理由。
81 个源路由、105 个目标路由、64 个共有路由只代表注册路径集合；其中 17 个源独有、41 个目标独有。
源 `/admin/astra-gateway` 与 `/admin/smart-ops` 是重定向别名，其余按后续独立评估登记；
本次没有据此批准引入、弃用或删除任何功能。目标路由多不意味着完成度更高。

以下各节保留 **PR #338 开始前、生产 #337** 的分析语境；“当前”“未修复”“草稿恢复失败”
仅描述该历史时点。后续进展/修复/部署以快照时间和所链接 GitHub 证据为准。

## 固定来源与生产基线

- 直接功能上游：`https://github.com/ranxi2001/sub2api`，只跟踪正式 release。
- GitHub `releases/latest`：v2.10.3，非 draft、非 prerelease，发布时间 `2026-10-09T15:57:59Z`。
- tag 对象：`95479ee5222e25765b1ab55baa513714db3ca184`；解引用 commit：`fd1b5ee4eeb20961fbb783fa6f136a1704271e90`；版本文件为 `2.10.3`。
- 生产 `origin/play/main`：`7508cb0cd8a9c38a33805795922ec13662d7753e`（PR #337）。已用 `git ls-remote` 实时核实。
- 共同祖先：`efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`。环境缓存 `7b58f1e` 不是开发基线。
- 隔离工作分支：`codex/core-v2103-batch1-20261009`；worktree：`/workspace/sub2api-core-v2103`。
- Wei-Shaw 的既有历史、Go 模块路径和来源归属保留；后续不再直接同步该仓库。

## 证据范围

`file-differences.tsv` 是两个固定 commit 的完整后端/前端文件差异清单（禁用 rename 推断），不是“缺失功能数”。A 表示上游独有，D 表示 Fork 独有，M 表示内容不同。任何 D 都不授权删除。

后端核心：793 A、720 M、749 D；后端其他：125 A、66 M、22 D；前端：282 A、423 M、371 D。生成代码和测试也在清单中，不能拿文件数代替功能验收。

`production-migration-files.txt`、`upstream-migration-files.txt` 和 `upstream-only-migrations.txt` 分别记录 399/331 个 SQL 文件及 40 个上游独有文件。每个新增迁移必须审查完整文件名、已部署校验和、表列定义和前置迁移。

## 核心功能差异与迁入策略

| 范围 | 真实源码差异/现状 | 迁入和验收决策 |
| --- | --- | --- |
| 账号权威读取和最终准入 | 已知草稿包含权威读取；生产 `openai_gateway_*`、`gateway_service*`、handler 与上游均有大量差异 | 草稿恢复后逐个入口审查，不整文件覆盖；启停、删除、分组变化后不能靠缓存放行 |
| 分组模型范围 | 上游 `account_group.go` 有 AllowedModels/Normalize/Validate/IsModelAllowedInGroup，生产不存在；草稿已实现存储 | 必须覆盖列表发现、普通/粘性/复合路由、最终准入，限制只收窄；保留父分组权限与模型别名 |
| 凭证刷新 | 上游账号/网关有 profile、protocol 与 token guard 扩展；草稿 Responses 最终准入刷新后可能继续使用旧对象 | 测试实际 HTTP 请求使用新凭证；不能只测准入函数。允许迁入授权功能代码，不自动建立真实 OAuth 授权、不保存真实密码/TOTP、不扩大授权 |
| 限流/调度 | 生产已具备“快照陈旧但未来重置已知时仍暂停”（`openAIQuotaWindowResetPending`）；上游另有 `openai_quota_recovery.go`、CAS 清除旧限流、RPM/并发租约/优先级调度 | 已有功能不重复宣称迁入。新恢复要验证旧探测不清除新 429、影子账号隔离、人工禁用/模型冷却保留 |
| 路由协议 | 上游 `provider_profile*`、`resolveUpstreamProtocol`、Command Code/Cline/Prism/BPS 等新增 | 优先现有 OpenAI Responses/Chat、共享 CC、Anthropic/Gemini 兼容；新供应商依赖另批评估，不能以新页面代替实际转发 |
| 失败切换/重试 | 草稿 passthrough `result+error` 有丢失部分用量风险 | 验证已响应不重试、未响应可切换且用量入账一次；覆盖断流、取消、超时、429、5xx |
| 结算恢复 | 草稿涉及结算恢复与简单计费错误路径 | 简单模式不错误扣费；余额/套餐/冻结价格/附加费/钱包流水保持原合同；禁止历史补扣 |
| 使用记录 | 上游有请求生命周期耗时详情、管理员 timing dialog、观察员视图、逐条 TPS | TPS 先做独立无迁移批次；生命周期采集后续单独设计保留期限与权限；不导入扩大授权的观察员角色 |
| 逐条 TPS | 上游 `frontend/src/utils/usageTps.ts`、`components/admin/usage/UsageTable.vue`；目标共享表无 TPS。用户页复用同一表 | 后端 DTO 新增可空 `output_tps`，两端统一显示；总耗时口径包括首 token 等待，不伪称模型纯生成速度；缺失/异常数据不显示零 |
| 统计/计费 | 上游账号累计成本、统计查询、长上下文等改动；直接 diff 会把 Fork `usageLogChargedCostExpr` 换成 `actual_cost` | 严禁替换：会丢附加费。用户实扣/账号成本/标准价分列测试，保留钱包与套餐口径；统计草稿待恢复 |
| 账号成本配置 | 上游 `account_cost.go`、`UpstreamBillingRateCell` 和自动同步成本倍率 | 另批接入现有样式，验证手动覆盖与探测刷新不会改历史价格 |
| 账号质量/运营 | 上游 `AccountOpsView`、`AccountQualityView`、定时探测、告警等 | 排在核心之后，先理清新增表、权限、网络请求和通知默认值，不自动启用 |

## 前端页面、入口与按钮清单

| 上游增量 | 目标处理 |
| --- | --- |
| AccountsView + AccountGroupModelLimits / groupAllowedModels / Create/Edit/BulkEdit | 接入账号 API 和调度后迁入按组模型编辑；复用现有 Modal、Select、表格和 Icon，测试加载失败、保存失败、并发变更 |
| AccountTodayStatsCell 累计 Token/成本、UpstreamBillingRateCell 同步按钮 | 统计后端口径通过后接入，不能以零冒充请求失败 |
| AccountConcurrencyProgress / AccountRpmSettings | 后端并发/RPM合同完成后再做入口，当前不导入无效按钮 |
| UsageTable TPS（用户和管理员共享）、UsageTimingDialog | TPS 独立批次；timing 详情另批，必须管理员权限与空值兼容 |
| PrioritySchedulingView | 对应后台实现通过现有协议回归后迁入 |
| AccountOpsView / AccountQualityView / PelicanTestsView / ControlledExperimentsView | 核心之后分批，先做路由、权限、启停与异常状态测试 |
| RequestCaptureView / TokenGuardView / TokenGuardV2View | 记录数据和凭据风险需专门界定；不与核心批次捆绑 |
| AutoConfigView / HarvestFlowView / BPS / Prism / Astra | 自动配置、授权和对外调用边界需要另行拆分；不得自动建立 OAuth |
| SupportTicketsView / SupportTicketDetailView（用户与管理员）、ChannelStatusV3View / PelicanShowcaseView | 非首批；存在明确后端和权限合同后可引入现有样式 |
| CredentialEncryptionSetup / OpenAITOTPDialog / OpenAITwoFAImport / 双 OAuth | 功能代码列入后续批次评估，兼容现有账号；不自动建立/扩大真实授权，不保存真实密码/TOTP |
| Mobile / Canvas | 用户明确暂不处理；保留现有源码 |

现有 `WalletView`、`AdminFundsView`、`AdminWithdrawalsView`、ModelCatalog、Play/福利、ImageStudio、StarFrame、品牌与 i18n 全部保留。前端实现前必须有真实可解码原型图片和基线证据；静态审查板不可冒充生产浏览器证据。

## 个性化保留与回归契约

| 不变量 | 主要代码边界/验证 |
| --- | --- |
| PR #337 私有结算身份 | ctxkey、client_request_id middleware、gateway routes/handler、gateway_usage_billing；重放客户端 request id 不能复用私有去重键；保留 PR 的全部测试 |
| Go 1.27.2、安全依赖、完整 CI | go.mod/go.sum、Dockerfiles、`.github/workflows/`；禁止用上游旧版本替换 |
| 钱包流水、附加费和计费幂等 | `usage_billing_repo*`、`usage_log_cost_expr.go`、`usage_log_repo_surcharge.go`；用量/去重/钱包一致，无历史补扣 |
| 套餐、冻结价格、父分组权限 | billing hold / subscription / parent-group 服务与相关测试；最终准入必须使用最新状态但保留约定价格 |
| 福利、会员、签到及 Play | FORK-PLAY-003、REWARDS-015、MEMBERSHIP-016；不删现有接口/导航 |
| 图片别名计数、异步图像、StarFrame | FORK-IMAGE-004/011；媒体不能误算文本 TPS，计费模型与展示别名不互相覆盖 |
| 品牌、i18n、设计治理 | FORK-BRAND-001/NAV-002/UI-012；现有组件风格、键完整性与可访问交互 |
| 生产发布约束 | origin/play/main → Zeabur → jisudeng.com；仅 PR 分支，主线程协调部署，不绕过保护 |

完整自研登记继续以 `docs/FORK_CUSTOMIZATIONS.md` 为准；本批不把上游来源字段误标为“整版已迁完”。

## 迁移冲突

- 上游 `270_drop_platform_check_constraints.sql`：依赖平台清单、API 校验、Ent 校验完整迁入后才可考虑；不能只放宽数据库。
- Fork 已有 `270_payment_manual_confirmation_reference.sql`、`270_fund_operation_records.sql`，不得修改或覆盖。
- 上游 `271_openai_excel_dual_oauth.sql`：涉及新的授权凭据表，后续批次审查兼容性与必要性；本批不引入，不能覆盖 Fork 的 271。功能代码本身并非用户禁止项。
- Fork `271_subscription_package_grant_sources.sql` 保留。
- 新功能迁移采用未占用的新编号和唯一完整文件名；不得修改任一已部署 SQL 的字节。新增表列必须与 Ent、DTO、回滚/前向兼容方案一起审查。

## 分批验收计划

上游维护伴随批次：固定来源 manifest 与只读差异/迁移冲突检查先落地；二进制更新入口、安全缓存隔离、release.yml 标签发布和 VERSION 推送约束另做小批次。`UpdateService`/`VersionBadge` 的 Wei-Shaw 成品下载不能直接改为 ranxi 成品下载；源码来源是 ranxi，生产只能部署经过审查和测试的 tqytwe 构建。当前二进制更新、镜像/安装来源和标签发布问题尚未修复，不能宣称维护迁移完成。保留 Go module、LICENSE、法律确认、历史来源、model-price-repo 与供应商版本同步。

1. B1 使用记录逐条 TPS：先 RED（DTO JSON、异常时间、媒体、流式与非流式），再实现；共享表交互、用户/管理员页面、历史空值、导出字段、中文/英文和桌面主题；不改数据库、不改历史费用。
2. B2 账号权威读取、分组模型限制与最终准入：由主线程另行分配核心任务；逐项核实启停/删除、父分组权限、映射模型、刷新凭证在请求中生效、Chat/Responses/共享 CC 全入口；列表结果不能代替最终准入证据。
3. B3 结算恢复与部分流式用量：先修简单计费错误路径，再验证 result+error 经 retry/failover 仍结算一次，覆盖取消与 DB 写失败；保留 PR #337、钱包、附加费、套餐和冻结价格。
4. B4 账号统计与路由配置 UI：账号累计/成本、模型限制、限流状态/调度入口；后端口径和实际页面交互一并验收，错误/空/加载态不可省略。
5. B5 其他明确缺失功能：账号质量/运营、timing、支持工单、双 OAuth 代码等分别评估，兼容现有账号；移动端/Canvas 不进入，不自动建立实际授权。

每批：规格审查 → 代码质量审查 → 定向测试 → `make test` → `make test-backend-unit` → 相关 integration → `make build` → fork integrity → 草稿 PR（play/main）→ 核对远端 SHA → 全部 CI。独立评审未完成必须记录为阻碍。报告主线程协调合并部署；部署后核对 commit/健康，再由用户本机进行游客/用户/管理员验收。

## B1 TPS 适配口径

上游逐条 TPS 来源是 `frontend/src/utils/usageTps.ts`；上游没有逐条 DTO 字段，后端运营聚合 `output_tps` 不是本功能。新增 DTO 字段是本 Fork 的适配，统一用户/管理员列表、详情和导出口径。公式为 `output_tokens / (duration_ms / 1000)`，不扣 TTFT、不额外叠加 reasoning token；流式与非流式一致。

沿用上游输出少于 2、无有效正耗时、图片计数/图片输出 token、视频计费模式排除规则。本 Fork 另外排除 `billing_mode=image`、`media_type=image/video`、`video_count>0`：已有异步媒体和历史记录可能只保留这些媒体标识，不能误展示文本 TPS；每项均有独立测试。有效数值小于 100 保留一位，小于 0.1 显示 `<0.1 t/s` 防止正值舍入成 0，大于等于 100 取整。

`output_tps` 字段存在时以服务端为准，null 明确表示不可用。只有旧接口缺失字段时前端才按相同规则兼容计算。读取派生数值，不改 SQL、不回填、不改历史费用；完整迁移状态仍为 false。

## 草稿恢复状态与五个已知未完项

Library `libfile_c3b8f96febdc819199fe0ff93f12f529`，版本 0，文件 `core-migration-draft-20261009-7b58f1e.tar.gz`，预期 726320 字节，SHA-256 `79e5b15d7cf7cccf2ae77a3ebe46a47d5bfada6306b8690e7499f271d04b414f`。

本环境官方 materialize 首次下载与重新准备后唯一重试均 HTTP 403，未生成文件，未验证哈希，未读取清单，未应用草稿。不能使用父线程路径冒充已接收。草稿旧版本文档须在成功恢复后更新为本账本基线。主线程已确认本任务不再尝试下载、不等待草稿，仅交付 B1 TPS 和来源锁定文档/检查脚本；其余核心任务另行分配。

- [ ] 简单模式错误路径与结算恢复互相影响：恢复后先 RED。
- [ ] allowed_models 已存储但未接路由/最终准入：不得仅以 DTO 测试通过验收。
- [ ] Chat/共享 CC 准入只有测试：逐入口实现并断言真实上游未被越权调用。
- [ ] Responses 新账号凭证未进入实际请求：抓取本地测试上游 Authorization（只用合成值）断言。
- [ ] passthrough result+error 用量被重试分支丢弃：端到端测试结算一次与失败切换次数。

生产核查仅沿用用户给出的结论：上线后 21 条普通 OpenAI SSE 的 usage/dedup/wallet 相符；入口日志不完整，不能证明全部请求均有记录。ID147（0.11）在该窗口无用量。本任务不读取生产秘密或改余额。
