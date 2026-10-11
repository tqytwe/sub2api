# P3 v2.10.5 完整升级准备（未完成迁移）

可行动：先交付 P3-1 账号测试 SSE 保活；根已明确授权。其余按下面批次认领。
阻断：#364/#363、281 标识诊断、独立 account_usage_service.go 并发任务不能覆盖；
续费 SQL 新编号未分配；全部非等价文件仍需语义审查；全量升级和生产验收均未通过。
本目录是固定准备报告，不是替代 v2.10.3/handoff.json 的实时看板。正式来源锁不修改。

## 固定证据

2026-10-11 独立读取 GitHub release/latest、git ls-remote 和本地精确 Git 对象：

- 来源 ranxi2001/sub2api，正式 release [v2.10.5](https://github.com/ranxi2001/sub2api/releases/tag/v2.10.5)，发布时间 2026-10-10T16:06:55Z，draft/prerelease=false。
- tag object `e7f36d0ed124fccbf55382166c116a89ef79dd83` → commit `d977e7a5e24e5536db6ce8b8478743e0c4b11f77`；VERSION=2.10.5。
- 正式来源锁 v2.10.3 commit `fd1b5ee4eeb20961fbb783fa6f136a1704271e90`，fully_migrated=false。
- 初始 Fork 固定基线与当时远端 play/main：`8364b462feda15f3bf8f3bb2029314d2193b9dba`。不代表本任务确认生产运行版本。
- 独立 worktree `/workspace/sub2api-p3`，分支 `sync/ranxi-v2.10.5-p3-preparation`。
- `/workspace/.agents` 空，仓库无 `.agents/skills`；已读根 AGENTS、DELIVERY_WORKFLOW、PROJECT_HYGIENE、UPSTREAM_SYNC_PLAYBOOK、既有来源锁/账本/交接及 FORK_CUSTOMIZATIONS。本片不改前端。

## 可复算清单与局限

`python3 scripts/check_p3_upgrade.py` 离线重算全部 Git tree/blob、完整双端差异、SQL SHA256和编号冲突；
结果必须与提交的 JSON/TSV 完全一致。首次重建用 `--write`，审查时不得用它掩盖遗漏。
精确 refs 缺失时先按 UPSTREAM_SYNC_PLAYBOOK 抓取 v2.10.3/.4/.5 到只读 remote release refs，不推外部 tag。

- [coverage.json](./coverage.json)：来源基线/Fork/目标三方每个差异文件逐项状态、blob和理由，含老版本尚未吸收差异，未静默缩成209文件。
- [release-diff.tsv](./release-diff.tsv)：v2.10.3→v2.10.5 共209文件（133 backend、54 frontend、22其他）。[release-commits.tsv](./release-commits.tsv)：68 commits，含merge。
- [fork-target-diff.tsv](./fork-target-diff.tsv)：当前Fork→目标完整差异；D不授权删除。
- [features.json](./features.json)：209增量文件各有且仅有一个首要工作包；共享关联字段仍要协调，逐层 pending 不冒充完成。
- [routes.json](./routes.json)：固定路由声明的源/目标/独有集合；别名不冒充功能，Fork独有入口必须保留。
- [migrations.json](./migrations.json)：两端所有SQL文件、完整名称、blob、SHA256及同编号异名冲突；[migration-decisions.json](./migration-decisions.json)逐项记录41个源独有SQL的表/状态/理由，含源244已由Fork273适配吸收、观察员权限当前不适用。
- [baseline-feature-evidence.json](./baseline-feature-evidence.json)：历史已吸收批次线索及是否在固定Fork祖先链，非最新部署证明。
- [protected.json](./protected.json)：GitHub实际 #363/#364完整文件列表；281路径未获根确认前保守保护 requestledger/request_ledger 及所有SQL，另保护额度读取文件。

状态：`absorbed-adapted-schema` 仅指有列定义/迁移测试证据的既有适配；`not-applicable-current-scope` 必须有既有排除理由；SQL `needs-port` 仍须编号与兼容审查。
`absorbed-bytes` 只表示固定Fork字节与目标相同；`needs-port-review` 表示有差异须逐项适配/证明语义已吸收，
不能读成所有文件都缺功能；`not-applicable-preserve` 表示目标不存在该路径，保留Fork/旧入口，绝不自动删除；
`pending-composition` 表示独立在途；`protected-inherited` 表示#363已合入并完整继承，不能再标成在途。不存在从文件相同推导UI/API/DB闭环的规则。
历史已吸收 TPS、最终模型准入、权威账号读取、钱包附加费统计、原版更新阻断等保留回归，
具体证据在历史快照；不重复标记为本轮新增，也不把旧快照状态套成当前部署状态。

## 数据库与共同变更

代码核对 v2.10.4→.5 migrations 无差异；但 .3→.5 新增
`239_payment_order_subscription_renewal_mode.sql` 和其测试。
目标已有 `239_channel_reasoning_effort_multipliers.sql`、`239_membership_financial_fk_guard.sql`，不能复用239。
新增 payment_orders.subscription_renewal_mode VARCHAR(16) NOT NULL DEFAULT extend，CHECK restart/extend。
Ent schema和生成CRUD、订单Service/DTO、支付创建与resume、fulfillment、subscription_service、PaymentView、SubscriptionsView、
paymentFlow/types及测试必须同批适配。新订单默认restart会重置周期用量；旧订单extend须保持。
与 #364 的receipt、冻结价格、撤销与结算原子性组合后，在真实隔离PG验证旧库升级/重复应用/并发续费/退款/撤销/旧写新读，
并审查锁等待和迁移运行器checksum。根分配编号之前不写SQL，不拿无新增迁移代替兼容通过。

旧来源遗留BPS/双OAuth、优先级调度、账号质量、工单等仍可能依赖旧账本的40个源独有迁移。
以本目录全量SQL集合逐一核对，不只核新增239。270/271以及280/281都保留所有已部署字节。

## 分批与验收边界

1. **P3-1 账号测试保活**：仅 account_test_service.go、新增account_test_sse_keepalive.go和对应测试。
   上游#350复用compact内部实现，但Fork采用独立helper，避免修改gateway compact共享代码。
   首字节立即送SSE注释、事件后持续心跳、断连取消、停止后等待协程、恢复原context/writer；不改模型输出与计费。
   RED→GREEN真实隔离HTTP两跳；首包/正文静默、成功/失败/截断终止一次、写失败、race、停止后无写入。
   默认10秒节拍，长等待测试检查实际节拍；全部合成凭据/假上游，不请求生产账号。
2. **P3-2 核心协议/能力路由**：能力拒绝与真实可用性故障分类、transport retry、WS搜索策略；先与#364/281认领handler/forwarder。
   覆盖映射后模型、父组允许范围、停用/删除/凭据刷新、Chat/Responses/共享CC/WS、取消/429/5xx及部分流用量只结算一次。
3. **P3-3 账号请求时区**：settings总开关→API/DTO→账号编辑/创建→请求HTTP/WS全链。默认关闭，非法IANA拒绝，缺字段兼容；只替换已有字段。
4. **P3-4 BPS与账号管理完整基础**：先schema/凭证repo与账号协议/路由，再401恢复/并发隔离、模型冷却、安全诊断UI、可选Redis工具状态。
   fake OAuth、权限、旧credential读取、管理员停用不解除、并发旧401不毁新token；Redis隔离/故障/TTL；不建立真实授权。
5. **P3-5 其余核心与产品入口**：账号RPM/优先级/质量运营、累计成本及配置、timing、支持工单、测试响应证据、Codex配置、Pelican、Grok来源URL。
   各包需schema/config/API/DTO/路由/侧栏/页面/i18n/测试闭环，沿用Fork组件与主题。自动探测/通知默认关闭。
   Mobile/Canvas沿用已明确排除范围，保留现有功能；上游部署二进制/镜像不适用，保留Fork自研源码构建。
6. **P3-6 续费组合**：待#364稳定与新编号分配。金融不变量先于UI，不覆盖已冻结价格、钱包附加费、套餐grant归属及图片计费。
7. **最终组合**：正常merge最新play/main到独立分支；重算清单，语义已吸收逐项加具体测试证据；无理由pending不得宣称完整升级。

每包按仓库TDD、规格/质量审查、定向与race/相关integration、make test、make test-backend-unit、make build、Fork integrity。
同字节已通过证据须列源SHA/文件hash/覆盖范围；不重复启动正在运行的检查。稳定diff先交根审，门禁并行。
草稿PR目标play/main，只在本地适用门禁全通过后提交推送；本任务不合并/部署。

## 个性化保留与回滚

| 保留项 | 必验合同 |
| --- | --- |
| 品牌/首页/主题/i18n/侧栏（FORK-BRAND/NAV/UI） | 复用设计系统，用户/管理员入口，浅深主题及错误/空/加载/权限状态；不得整文件套上游UI |
| 钱包/余额/附加费/幂等（BILLING/LIVE） | 同一请求一次结算；取消/部分流/写失败仍保留证据；usage标准成本不替代用户实扣，无历史补扣 |
| 套餐/冻结价格/会员/福利（REWARDS/MEMBERSHIP/PLAY） | receipt与grant归属、父分组权限、冻结报价、周期counter、优惠券/充值/VIP/签到/Arena/盲盒/答题/Team完整保留 |
| 定价/模型目录（PRICING） | 展示价/渠道成本/用户实扣分离，模型别名与能力/路由一致，手工成本覆盖不改历史 |
| 图片/异步/StarFrame（IMAGE） | 模型别名计数、重试/取消/回收、存储失败恢复、真实成本与扣费，不把媒体算文本TPS |
| 权限/OAuth/公共页/IP风险/管理员（OAUTH/PUBLIC/RISK/ADMIN） | 同域cookie、普通用户/管理员隔离、账号批量操作与既有开关，原版更新继续拒绝 |
| 数据库/部署/移动（MIGRATION/DEPLOY/MOBILE） | 已部署SQL不改字节；Go1.27.2/安全依赖/CI保留；既有移动功能不删；不部署上游产物 |

P3-1可应用级revert，无DB变更；触发条件：心跳污染输出、重复终止、writer race/泄漏、断连继续请求或账号测试回归。
其余批次应用回退必须保留新表/证据/旧字段兼容；不DROP/TRUNCATE/DELETE、不改schema_migrations、不回滚历史SQL。
BPS共享Redis需协调所有实例，回退前关闭可选开关；旧401实现可能删除RT，不能保证恢复语义，须单独根审。
续费restart一旦影响额度/周期不能靠旧应用恢复数据，必须先确定前向修复方案；无授权不清理或补扣。
生产SHA/健康/三身份本机浏览器、财务对账待根安排；本任务未访问生产或凭据，不进行主动漏洞测试。

## 交接校正

初始GitHub核查 #363/#364为OPEN，head分别7cc5c016d409aaccec82ec6b222a4c54b620b6e3 / 88cdf3395288723f305e18fbe4a4352d58670efc。
随后根确认且本任务fetch核实#363已合并caa4805b26114da4b5cb0ddea9343ef6ec9fc528；已普通merge并保留全部文件，见[组合与复用证明](./composition.json)。
当前机器清单已重算至caa基线，最初8364清单保留于8060c27fa提交。17个历史PR已通过GitHub重新核为MERGED，见[PR核查](./baseline-pr-status.json)；不代表已验证生产部署。
历史handoff checked_at=2026-10-10T21:17:26Z，不前移未复核全表时间。
根明确281是独立进行中任务，不能继续按历史“暂停”描述；额度并发独立线程01a12b27-537f-71f2-9b0f-7df4f11d800e。
docs/README.md与旧handoff属于并行热区，索引链接待根组合添加，未自行覆盖。

本片验证结果见 [归档证据](../../archive/completed-tasks/2026-10-11-p3-preparation/README.md)。完整源码差异清单通过不代表完整功能兼容通过。

组合后前端整树等于caa；P3四文件与本地已验证提交8060c27fa完全同字节。后台只继承#363的隔离PG验收fixture，生产后端/迁移/CI/scripts无额外差异；复用#363归档门禁、双审及CI。无业务交集，不重新启动完整后端合同；本任务最终head CI另行一次。

组合后的三方差异清单为6,241项（新增190项来自#363源码/证据），release增量仍为209文件；这些是文件数，不是缺失功能数。
