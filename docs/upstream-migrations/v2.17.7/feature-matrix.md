# v2.17.7 功能域矩阵

核验基线：`origin/play/main@5c16bc5b5965a7b0d7c38aa35671639f045f2dea`；候选分支：`codex/v2.17.7-moderation-release-20261028@09bd145df66c0386171f902fed5c9b6f5db2feaf`；上游：`ranxi2001/sub2api@v2.17.7`。

本表按产品功能域记录同步结论。上游共同祖先到目标 tag 的大量路径变化只用于发现候选，不等于需要整树合并。`fully_migrated` 在所有纳入项具备代码、测试、生产和管理员验收证据前保持 `false`。

| 功能域 | 上游范围 | 当前 Fork/生产事实 | 当前候选事实 | 结论 | 下一道门 |
|---|---|---|---|---|---|
| 来源与版本 | v2.17.7 tag `9620d315f72a7dbf04fa10fca02970e1e410805e` | 生产分支为 `play/main`；`backend/cmd/server/VERSION` 仍为 `0.2.14` | 来源锁已固定，候选仍保留 Fork 版本标识 | 已锁定，版本标识策略待决定 | 明确是否只同步功能而保留 Fork 版本号 |
| 渠道运维 | v2.17.7 admin channel operations | PR #389 已部署到生产；管理员路由/API 已有静态和未认证拒绝证据 | 候选无重复实现 | 技术部署已证，业务现场未验收 | 管理员浏览器保存、刷新、revision 冲突和审计回读 |
| 优先调度 | v2.17.7 priority/scheduling | PR #389 已部署；硬准入和历史 unknown 需保持 Fork 合同 | 候选包含页面和 API | 技术部署已证，真实调度效果未验收 | 管理员读写、unknown 状态、粘性/后备行为 |
| GPT-6.1 Sol 计费 | 长上下文阈值与 OAuth/Setup Token 豁免 | Fork 已有实现和测试 | 候选保留并集成 | Fork 等价，生产样本未完成 | 只读抽样用量、余额、模型和 API key 边界 |
| 滚动 24 小时 Usage | 精确窗口、筛选快照、日期边界 | 生产仍为 `5c16bc5`，未部署候选 | 候选有 UI/API、定向测试和构建证据 | 候选待发布 | 与后端合同联调、一次批次部署、管理员现场验收 |
| 内容审查用户管理 | #580 / moderation user state/actions/exemptions | 生产仅有 `content_moderation_logs`；状态、hits、actions 表不存在 | 代码与迁移 `296/298/301` 已在候选；管理员 API、CAS、原因确认、owned unban 已测 | 候选待发布，生产迁移未应用 | 单批合并部署后核对 migration ledger、表、权限和审计 |
| 请求台账 | Fork admin-only request ledger | 管理员入口已部署；普通用户入口和 user route 已移除 | 候选保留权限测试 | 已完成技术隔离；现场仍需抽查 | 管理员登录和普通用户直达/API 403 验收 |
| 质量运维/Jev | #574/#575/#576 quality ownership、Jev、live regressions | 当前 Fork 有旧质量测试/规则基础，但缺上游新增 ownership/Jev 合同和完整 UI | 当前候选未实现该闭环 | 未完成 | 先完成后端/API/迁移/管理员页面和隔离状态回归，再纳入发布批次 |
| 凭证运营异常 | #574 safe error classification、token guard v2、refresh failure ledger | 当前 Fork 没有上游 `credential_operation_failure` 与 token-guard-v2 闭环 | 独立实现工作包进行中 | 未完成 | 证明 allowlist、不落原文、迁移 writer/reader、管理员读写和回退 |
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

本矩阵当前结论是 `fully_migrated=false`。任何状态变更必须同时更新代码证据、测试、生产证据和回退条件。
