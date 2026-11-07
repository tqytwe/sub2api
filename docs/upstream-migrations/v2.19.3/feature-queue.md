# 功能差异与验收队列

来源清单覆盖 728 条变更；下表逐一映射全部 41 个增量 PR，另 4 条版本提交不作业务功能。历史 683 条变更保留在完整提交账本，旧结论不直接当作当前等价证明。所有文件路径为线索，未列 PR 表示未实施。

| ID / 上游 PR | 功能与判定 | 当前证据/差异 | 本项完成条件 |
| --- | --- | --- | --- |
| U01 / #585,#593 | 渠道实体/URL 归属与备注：**明确差异** | Fork 有 channel_operations CAS；未有 upstream_channel_entities/显式上游实体绑定的数据链，不能用 URL 相同推断历史归属。<br>`backend/internal/repository/channel_operations.go`、`backend/internal/server/routes/admin.go` | 实体→绑定→revision→目录/API/UI/审计→删除冲突测试；312/313 同号迁移另编号 |
| U02 / #586 | 利润叠加统计：**部分已有** | Fork 已修复管理员附加费统计，未有上游 ProfitOperations 整套 URL/entity 归因。<br>`backend/internal/repository/dashboard_aggregation_repo.go` | 成本/收入/倍率/未知值一致；筛选后与使用流水只读对账，依赖 U01 |
| U03 / #587 | 质量孤立隔离与单模型释放：**明确差异** | Fork ownership 具备计划恢复/删除与版本保护；上游模型移除和隔离范围缩小合同未见对应字段。<br>`backend/internal/repository/quality_ownership_core.go`、`backend/internal/service/quality_probe_targets.go`、`frontend/src/views/admin/AccountQualityView.vue` | 预览→版本/owner 栅栏→单模型确认→恢复/保留其他限制→刷新回读；不改变福利 |
| U04 / #588 | 管理员滚动 24 小时：**部分已有** | Dashboard 与 Usage 已传 rolling24-hours；不能据此证明所有查询/导出/分页窗口一致。<br>`frontend/src/views/admin/DashboardView.vue`、`frontend/src/views/admin/UsageView.vue` | 同一时间窗查询、汇总、分页、导出与非 UTC/DST |
| U05 / #583,#598,#615 | 优先调度延迟、sticky、恢复降级：**明确差异** | Fork 有 stop/deprioritize、恢复探索、attempt/质量归因；配置中未有上游 latency_sticky 完整合同。<br>`backend/internal/service/priority_scheduling_config.go`、`backend/internal/service/priority_scheduling_runtime.go`、`backend/internal/repository/usage_log_priority_scheduling.go` | HTTP/WS admission、鉴权/配额不绕过、sticky 稳定性、unknown、公平恢复与降级计费 |
| U06 / #590 | v0.2.19 聚合更新：**待核验** | 聚合 PR 必须拆开审查；不能把 cherry-pick 视为一个已完成能力。<br>`backend/internal/server/routes/admin.go`、`backend/internal/handler/admin/account_handler.go` | 拆 scoped admin keys、图片按张计费、Gemini/Antigravity/协议/导出等子项；完整路径已在提交账本 |
| U07 / #592 | Redis 6 兼容：**明确差异** | Fork passkey session 仍直接 GETDEL；当前生产 Redis 8.4，不代表向下兼容。<br>`backend/internal/repository/passkey_session_store.go`、`backend/internal/repository/user_msg_queue_cache.go` | 一次性消费原子性、固定窗 TTL、ZSET 命令在真实旧 Redis 验证；原子 Lua 禁止替换成分开 GET/DEL |
| U08 / #591 | BPS 用量快照：**待核验** | Fork BPS runtime 已存在；上游三方用量快照/来源/冻结时点需按 writer-reader 对照。<br>`backend/internal/service/billing_service.go`、`backend/internal/service/openai_gateway_usage.go` | 真实 token 来源、模型、刷新稳定性、unknown 与费用快照，不重算历史账单 |
| U09 / #596 | 质量 state probe 目标与模型输入：**部分已有** | 前端有 STATE_PROBE_QUESTION 和判定工具，质量探测目标已有；运行 worker/允许模型与判定证据待核。<br>`frontend/src/utils/intelligenceTest.ts`、`backend/internal/service/quality_probe_targets.go` | 规则编辑→目标解析→执行→状态判定→历史；不把前端枚举当实现 |
| U10 / #599,#602,#604,#610,#620,#626,#628,#635,#643 | 服务端延迟/队列/连接观测：**明确差异** | Fork 有 ops histogram 和 priority attempt；未有 ops_network_minutes、queue_wait/queue_timing 的完整存储合同。最终 #643 已撤回主表 timing 列。<br>`backend/internal/repository/ops_repo_latency_histogram_buckets.go`、`backend/internal/repository/usage_log_priority_scheduling.go` | 按最终版本实现连接/队列分段来源、NULL、分页/筛选、保留期；不把已撤回的列重新加回 |
| U11 / #595,#601 | OpenAI 多授权连接：**明确差异** | 当前树未见 auth_connections、connection_affinity、refresh fencing 表与服务；API Key 和单凭据不是等价物。<br>`backend/internal/service/openai_gateway_service.go`、`backend/internal/server/routes/admin.go` | 上游 312–318 作为一项闭环：CRUD/OAuth/lease/route/usage attribution/affinity/refresh unknown/回退 |
| U12 / #607 | WS 客户端身份：**待核验** | Fork 有 WS 路径；必须区分客户端/连接/账号身份和 session 作用域。<br>`backend/internal/service/openai_gateway_service.go` | 多用户同 session、连接复用、cookie/header、租户隔离与重连回归 |
| U13 / #612,#632 | OpenAI header timeout 默认关闭与设置：**部分已有** | Fork OpenAI transport 强制默认 0，显式静态配置生效；缺少上游动态 DB 设置接口。<br>`backend/internal/repository/http_upstream.go`、`backend/internal/config/config.go`、`backend/internal/repository/http_upstream_test.go` | 默认/显式设置、缓存刷新与旧配置兼容；动态配置单独补闭环 |
| U14 / #613,#617 | 超时故障转移与发出前栅栏：**待核验** | Fork request ledger/usage settlement 已有自研幂等；不能按上游 handler 文件覆盖。<br>`backend/internal/service/usage_billing.go`、`backend/internal/service/openai_gateway_service.go` | 请求已发出/未发出故障注入、不得重复执行/计费、取消/重试/迟到响应一致 |
| U15 / #606,#608 | 智能上游协议与 WS 模式：**明确差异** | Fork 有 passthrough/WS；未见 upstream smart protocol 服务，文档 #608 依赖实现。<br>`backend/internal/service/openai_gateway_service.go` | 连接能力选择→复用→回退→usage/取消→会话边界；和 U11 的连接依赖先确认 |
| U16 / #621 | Anthropic API Key 模拟 Claude Code：**待核验** | Fork Anthropic gateway 有兼容层；未见上游专门 mimic 实现，需核 header/body/计费。<br>`backend/internal/service/gateway_service.go` | API Key 权限、开关、User-Agent/headers、工具/多轮/stream、失败恢复 |
| U17 / #622 | 管理员新功能提示：**待核验** | 不把现有公告当新功能提示等价。<br>`frontend/src/components/layout/AppSidebar.vue` | 仅管理员、关闭/持久化、版本提示去重、双语/主题；沿用 UI 组件 |
| U18 / #623,#636 | 充值抽奖/邀请/连抽：**明确保留二开，待接入决策** | 上游新彩票/票数/邀请账本与 Fork 福利/盲盒/邀请返佣不同；不覆盖既有玩法。<br>`backend/internal/server/routes/play.go`、`frontend/src/views/admin/PlayOpsView.vue` | 先独立定义纳入边界；323/324/325、充值归因、风控、概率、幂等奖励和管理页面一起交付 |
| U19 / #631,#634,#637 | 账号性能监控：**明确差异** | 当前未见 account_performance 存储/服务；质量规则与 token guard 的状态所有权不同。<br>`backend/internal/server/routes/admin.go`、`backend/internal/service/priority_scheduling_config.go` | 窗口/阈值→公平扫描→独立暂停 generation→恢复/删除→区域/并发→审计 |
| U20 / #633 | 安全提示 SSE 类型识别：**依赖前置，待核验** | Fork 未见上游 safety_prompt_response 基础链；只搬两文件无法闭环。<br>`backend/internal/service/openai_gateway_response_handling.go` | 先决定安全提示注入/retention 基础模块，再验证错误/缺失 Content-Type 的增量识别、缓冲上限与取消 |
| U21 / #642 | 质量共享题批次：**明确差异** | Fork 有多模型重复探测和整轮保存；没有共享 batch 表/lease，非同一功能。<br>`backend/internal/service/quality_probe_targets.go`、`backend/internal/repository/quality_round_integration_test.go` | 随机题→跨账号同批→截止/租约→对照通过才处置→混合版本与回滚暂停，不能拆成 UI 占位 |
| U22 / #641 | UltraFast 账号与计费：**部分已有** | Fork 已有 ultrafast 透传和配置 multiplier ×6 测试；不是全缺，但完整账号策略/回退待核。<br>`backend/internal/service/openai_gateway_request_body.go`、`backend/internal/service/billing_service_test.go`、`frontend/src/utils/usageServiceTier.ts` | 账号开关/上游响应档位/模型费率/冻结价格/降级/余额订阅 Key 额度同笔一致 |
| U23 / #640 | Mistral 账号/Chat：**明确差异** | Fork 有模型名/图标识别，但未有 Mistral 专门协议 adapter/供应商合同。<br>`backend/internal/service/openai_model_mapping.go`、`frontend/src/components/common/ModelIcon.vue` | API Key CRUD→模型目录/图片→Chat stream accumulator→Composite→真实 SDK/计费 |

## 历史范围补充

旧 446 条 merge 的自动功能域和历史判定保留在 `commit-inventory.json`；新增 237 条直接提交同样列出全部路径，不能将其视作无功能变更。包括 DeepSeek Chat/工具/compaction、Gemini 原生流与 thinkingConfig、Seedance、plugin bridge、Mihomo 与 Harvest 等，逐项核验后才关闭。

历史功能域仍需按以下方向复核：账号/OAuth/token guard，质量/Jev/恢复/模板，Priority/attempt，模型目录/BPS/Excel/provider 媒体，channel monitor V2/V3/鹈鹕，账务/订阅/API Key/倍率，moderation/IP risk/security，request capture/support/harvest/Astra/Prism/regional，用户界面/管理员界面/部署。没有在增量表单列的历史项目并未被排除。

## 下一项建议

先做 U03 的限定核对与实现：只补“单模型移除/孤立隔离处理”的预览—确认—恢复/保留—回读闭环，保留现有质量规则界面和 ownership；若依赖未满足，先把必要依赖明确列入同一项，不同时开 U21 共享批次或 U19 性能监控。完整 U05 优先调度随后独立处理。

用户此前要求完全对齐上游（玩法福利保留）；旧账本的 scope_blocked/needs_product_scope 不是本轮自动排除。需要产品或架构决定的功能继续可见，不按本次收紧而从清单删除。
