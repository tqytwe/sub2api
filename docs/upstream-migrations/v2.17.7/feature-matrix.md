# v2.17.7 功能域矩阵

核验基线：`origin/play/main@5c16bc5b5965a7b0d7c38aa35671639f045f2dea`；当前本地候选：`codex/v2.17.7-moderation-release-20261028@5e921e000`；上游：`ranxi2001/sub2api@v2.17.7`。

本表按产品功能域记录同步结论。上游共同祖先到目标 tag 的大量路径变化只用于发现候选，不等于需要整树合并。`fully_migrated` 在所有纳入项具备代码、测试、生产和管理员验收证据前保持 `false`。

| 功能域 | 上游范围 | 当前 Fork/生产事实 | 当前候选事实 | 结论 | 下一道门 |
|---|---|---|---|---|---|
| 来源与版本 | v2.17.7 tag `9620d315f72a7dbf04fa10fca02970e1e410805e` | 生产分支为 `play/main`；`backend/cmd/server/VERSION` 仍为 `0.2.14` | 来源锁已固定，候选仍保留 Fork 版本标识 | 已锁定，版本标识策略待决定 | 明确是否只同步功能而保留 Fork 版本号 |
| 渠道运维 | v2.17.7 admin channel operations | PR #389 已部署到生产；管理员路由/API 已有静态和未认证拒绝证据 | 候选保留已部署的 Fork 适配实现；上游 #573 增量尚未证明完全等价 | 技术部署已证，业务现场未验收 | 管理员浏览器保存、刷新、revision 冲突和审计回读，并完成上游增量对照 |
| 优先调度 | #315/#422/#508/#520/#572 priority/scheduling | PR #389 已部署；硬准入和历史 unknown 需保持 Fork 合同 | `321da3b7d` 起按公开模型及入口隔离历史，`52d6c0de4` 在真实管理员快照显示入口；`4327df128` 新增按物理 attempt、model、endpoint、group 的 `attributed_v1` 观察写入/读取和 30 天有界清理；定向测试通过 | 部分等价；精确观察基础已完成，完整 hard-stop、penalty clock、quality round 联动仍缺 | 接入质量 round 身份和完整 HTTP/WS admission/penalty 行为，再做管理员验收 |
| GPT-6.1 Sol 计费 | 长上下文阈值与 OAuth/Setup Token 豁免 | Fork 已有实现和测试 | 候选保留并集成 | Fork 等价，生产样本未完成 | 只读抽样用量、余额、模型和 API key 边界 |
| 滚动 24 小时 Usage | 精确窗口、筛选快照、日期边界 | 生产仍为 `5c16bc5`，未部署候选 | 候选有 UI/API、定向测试和构建证据 | 候选待发布 | 与后端合同联调、一次批次部署、管理员现场验收 |
| 内容审查用户管理 | #580 / moderation user state/actions/exemptions | 生产仅有 `content_moderation_logs`；状态、hits、actions 表不存在 | 代码与迁移 `296/298/301` 已在候选；管理员 API、CAS、原因确认、owned unban 已测 | 候选待发布，生产迁移未应用 | 单批合并部署后核对 migration ledger、表、权限和审计 |
| 请求台账 | Fork admin-only request ledger | 管理员入口已部署；普通用户入口和 user route 已移除 | 候选保留权限测试 | 已完成技术隔离；现场仍需抽查 | 管理员登录和普通用户直达/API 403 验收 |
| 质量运维/Jev | #51/#174/#437/#447/#522/#528/#548/#550/#565/#575/#576 quality ownership、Jev、live regressions | 当前 Fork 有旧质量测试/规则基础，生产尚无新增质量表 | `b81d3e6da` 新增 320 核心迁移、Pelican 配置/结果快照、opaque round、边界校验、ownership CAS 管理员 API 和前端类型/API；Jev judge、live regression、真实 restriction restore/reconcile、可见管理 UI 尚未实现 | 核心基础候选待发布，整包未完成 | 补 runner/judge、真实隔离恢复和管理员可见交互，再做生产迁移与验收 |
| 凭证运营异常 | #574 safe error classification、token guard v2、refresh failure ledger | 当前 Fork 没有上游 `credential_operation_failure` 与 token-guard-v2 闭环 | 候选已合并 token guard/reauth、330–335 迁移、allowlist writer/reader、管理员 API/UI；Python worker 23 项与前端定向 32 项通过；Go 已找到 `/opt/go/bin/go`，但编译被候选 stale Wire provider 引用阻断，且已移除不存在的 `internal/mihomo` 默认依赖 | 未完成，生产未发布；不得宣称完整闭环 | 先修复 Wire 与实际源文件装配一致性，再完成 Go 定向/全量门禁、迁移预检和管理员验收后纳入一次发布 |
| Request capture | 上游 request-capture routes/UI | 当前 Fork 未发现对应页面和后端 session 实现 | 未迁移 | 未完成/待范围确认 | 先确认是否属于生产产品范围及隐私保留合同 |
| Token guard/reauth | 上游 account token guard v2、reauth runtime | 当前 Fork 无对应 v2 文件 | 未迁移 | 未完成/依赖凭证异常闭环 | 与凭证异常一起审查，不单独接入自动重登录 |
| Safety prompt audit / #293 | security penalty 原文记录 | Fork 已有相邻 prompt audit 明文且无 TTL | 上游设计会新增最多 100,000 runes 明文、管理员跨用户读取 | 隐私阻断，明确不原样迁移 | 先定最小读取、保留期、自动清理和审计合同 |
| Support tickets / harvest / 其他上游页面 | v2.17.7 相关 admin/user domains | 当前 Fork 与上游存在大规模分叉，不能由路径差异直接判定 | 尚未逐项分类 | 未完成范围审查 | 建立纳入/等价/排除/阻断证据后再开发 |
| 生产迁移与数据 | 所有纳入功能的 SQL、锁和业务不变量 | 生产 `schema_migrations=416`；`293/296/298/301` 不存在；settlement pending 仍待归因 | 候选迁移只在代码中 | 生产闭环未完成 | 只读复核迁移、日志、账务抽样；禁止补数或批量标 settled |
| 生产交接 | 精确 SHA、Zeabur、日志、DB、三角色验收 | deployment `6ae1a532ea16512cf741806a` 对 `5c16bc5`；滚动日志无 ERROR 但 WARN 有配置提示 | 候选未发布 | 未完成 | 单次部署后完成管理员现场验收和脱敏交接记录 |

## 状态定义

- **Fork 等价**：当前生产已有行为，且有对应代码/测试证据；仍需按风险做生产抽样。
- **已部署**：精确生产 SHA 和部署记录可核对；不代表管理员业务验收完成。
- **候选待发布**：本地代码和门禁通过，但生产未应用，不能对外宣称可用。
- **未完成/阻断**：缺少功能闭环，或存在隐私、数据、权限或依赖风险；不能用页面占位或文档替代。

## 上游全版本关键功能索引

来源范围是共同祖先 `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a` 到 tag `9620d315f72a7dbf04fa10fca02970e1e410805e`。以下为已发现的重要功能簇，不是 446 个 first-parent merge 的逐项关闭证明。未出现于表中的 merge 仍是审计未结项；不得仅凭文件路径差异推断是否纳入。

| 上游功能簇 / 关键 PR | 上游证据锚点 | Fork 当前证据 | 分类与后续 |
|---|---|---|---|
| BPS/Excel OAuth：#346/#373/#374/#379/#506/#541 | Excel OAuth、BPS runtime/reauth、Excel account capability | production 未发现同一 OAuth provider/runtime 链 | 需先定产品范围；若纳入，作为 OAuth、网关、刷新恢复和能力权限的完整工作包 |
| Token guard/自动重登/凭证失败：#185/#440/#557/#560/#574 | `account_token_guard_v2.go`、reauth runtime、迁移 254-257/262/300/309 | Fork 缺 v2 guard/reauth worker/credential failure ledger；生产已有 Fork 自有 309 | 缺失/依赖阻断；整套迁移需重编号、权限/secret 合同和隔离 PG 预检，不允许单独引入刷新失败分类 |
| 质量隔离/恢复/Jev：#51/#174/#437/#447/#522/#528/#548/#550/#565/#575/#576 | quality ownership、rule template、judge、live regression 与恢复审计 | Fork 有旧 scheduled tests/outbox/isolations；缺上游 ownership/version/Jev/API/UI 闭环 | 部分等价；避免平行重造，按缺失合同扩展现有模块 |
| Priority/latency admission：#315/#422/#508/#520/#572 | timing details、physical attempt identity、HTTP/WS latency admission、penalty clock | Fork 已有优先调度页/API和硬准入；当前候选按公开模型/入口隔离历史 | 部分等价；候选切片已实现，attempt evidence 与准入/处罚剩余 |
| 用量/模型/认证：#413/#432/#490/#504/#562/#567/#570/#577/#579 | usage cache、GPT-6.1 Sol、response model、connection diagnostics、token rotation、traffic records、group multiplier、rolling window | GPT-6.1 Sol 有部署实现/测试；#579 精确滚动窗只在本地候选；#567 user traffic 未发现同名 Fork runtime | 混合状态；逐行为审计 Fork billing/auth contract 后分类，不能把 ledger 有实现等同于全部上游行为 |
| 协议/媒体/供应商：#448/#449/#457/#459/#460/#468/#473/#485/#506/#542 | Pelican、模型广场、image scheduling、Grok realtime/TTS、MiniMax video、OpenAI SIWC、additional models | Fork 有独立 provider/capability/catalog 路径；SIWC 等路径未发现直接等价证据 | 需按当前支持 provider 和真实用户流量确定纳入，并逐协议检查认证、转发、计费和前端能力 |
| 请求采集/支持/平台运行时：#77/#82/#143/#303/#334/#385/#391/#399/#419-428 | request capture、Mihomo manager/static proxy、monitor V3、support tickets、Astra、regional runtimes/settlement | Fork 没有相同完整运行时；部分依赖多实例协调、守护进程、区域节点或独立服务 | 范围/架构待定；未有依赖基础前不得当作简单 UI 移植或声称已明确排除 |
| 安全/IP 风险/审计：#445/#487/#492/#513/#523/#525/#529 | auth IP guard、IP risk、prompt penalty、abuse block、evidence retention、fingerprint block、security workflow | Fork 有自有 IP risk/security audit/prompt audit；语义和留存/权限范围不等价 | 需逐合同适配；#492 全文保存和跨用户详情无 TTL，保持隐私阻断，不原样迁入 |
| 管理和生产交付：#479/#493/#516/#519/#556/#563/#566/#573/#580/#581 | API key 多组、cookie/session、timer、moderation recovery、audit categories/locales、channel ops、release identity | 请求台账 admin-only 已生产；渠道运维已部署；本地候选有 moderation recovery/滚动 24h | 按 Fork 已有实现做增量对照；当前候选未部署，现场管理员验收仍待完成 |

本索引帮助避免漏掉早期功能簇，但不能替代“每个相关 merge 被某个功能簇引用”的覆盖审计。集成负责人需产出 merge SHA → 功能簇映射，并给每个最终功能域附 Fork 路由/服务/迁移/测试证据，再将结论置为“等价、Fork 适配、缺失、明确排除、隐私/架构阻断”之一。

本矩阵当前结论是 `fully_migrated=false`。任何状态变更必须同时更新代码证据、测试、生产证据和回退条件。
