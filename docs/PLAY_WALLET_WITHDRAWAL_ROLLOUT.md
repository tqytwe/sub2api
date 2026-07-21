# Play、钱包与提现交付检查点

> 状态：active
> 当前阶段：阶段三 / CP3
> 当前结论：CP2 已通过（CP1、CP2 均已完成 PR/CI、部署证明和用户本地浏览器验收；下一步从最新 `origin/play/main` 创建 CP3 独立 worktree）
> CP2 功能部署基线：`origin/play/main@6531cb83dde134bccbde139220bd29e3623e36d7`
> 审查分支：`docs/play-wallet-transparency-cp2-acceptance-20260721`
> 工作树：`/home/dell/worktrees/sub2api-play-wallet-transparency-20260721`
> 最后更新：2026-07-21

本文档只记录已经取得证据的状态。允许状态仅为：
`未开始 / 进行中 / 已通过 / 失败 / 阻塞 / 等待本地验收`。
当前阶段部署并完成本地浏览器验收前，不得开始依赖它的下一阶段。

## 总览

| 阶段或检查点 | 状态 | 当前门禁 |
|---|---|---|
| 阶段一：管理员手动修复战队成员 / CP1 | 已通过 | PR、CI、部署证明及用户本地双语/主题/添加移动闭环均已通过 |
| 阶段二：战队奖励与余额透明度 / CP2 | 已通过 | 本地实现、数据库对账、完整服务器验证、审查、PR/CI、合并、生产部署证明和用户本地浏览器验收均已通过 |
| 阶段三：代币农场最近发放与当前预估 / CP3 | 未开始 | CP2 已通过；下一步创建独立 worktree 后开始 RED 检查点 |
| 阶段四：可提现权益账务基础 / CP4 | 未开始 | 依赖 CP3 |
| 阶段五：提现权限、申请、审核与线下打款 / CP5 | 未开始 | 依赖 CP4，且总开关保持关闭直至灰度条件满足 |
| CP6：服务器验证 | 进行中 | CP1 已通过；CP2 本地完整服务器验证、GitHub CI 保护检查、构建和部署后 API/资产复核均已通过；后续阶段未开始 |
| CP7：审查与合并 | 进行中 | CP1 已通过；CP2 PR #83 已全绿，并以非 rebase merge commit 合入 `play/main`；后续阶段未开始 |
| CP8：部署证明 | 进行中 | CP1 已通过；CP2 merge commit `6531cb83dde134bccbde139220bd29e3623e36d7` 已由 Zeabur 部署到 production 并完成 live proof；后续阶段未开始 |
| CP9：最终本地浏览器验收 | 进行中 | CP1、CP2 已通过；CP3-CP5 仍须逐阶段验收 |

## CP1 管理员手动修复战队成员

### 检查点状态

| 节点 | 状态 | 证据或剩余工作 |
|---|---|---|
| 需求对照检查点 | 已通过 | 三个管理员接口、候选预检、添加/移动、上海本月时间、幂等、TOTP、队长规则、事件与审计已逐项复审 |
| RED 失败测试检查点 | 已通过 | 已真实观察单次预检读取两次服务器时间、搜索失败保留旧候选、跨月弹窗边界不刷新、中文标签泄漏四类失败 |
| GREEN 实现检查点 | 已通过 | RED 已全部转绿；定向后端、前端、真实 PostgreSQL 集成和完整测试均通过 |
| 规格审查 | 已通过 | 已修复软删除候选、不可变快照并发、锁顺序、事件脱敏、系统原因码、中文状态和陈旧候选问题；最终逐条复审无剩余规格缺口 |
| 代码质量审查 | 已通过 | 已修复中文页面透出后端英文 `message`、提取 blocker 常量、单次捕获 `now`、补仓储 SQL 测试、刷新弹窗时间边界；复审无未解决问题 |
| 完整验证 | 已通过 | 定向测试、真实 PostgreSQL integration、`make test`、构建、Fork integrity 和 `git diff --check` 全部退出码为 0 |
| PR / GitHub CI | 已通过 | PR #82 的 GitHub CI 全绿并以非 rebase 方式合入 `play/main` |
| 部署核对 | 已通过 | Zeabur deployment `5528554772` 成功部署 merge commit `09e03463aac5e7626a3f0daaa09f9c8131cf049b`；健康、路由与双语前端资产均已核对 |
| 本地浏览器验收 | 已通过 | 用户于 2026-07-21 确认管理员中文/英文、浅色/深色及实际添加/移动闭环通过 |

### 基线与变更范围

- 基线 commit：`633cf432fad6e266f477e128f9608950591959e1`
- 后端：管理员战队候选、成员修复、事件查询、幂等写入、TOTP step-up、锁顺序、结算快照互斥、业务事件和审计动作。
- 前端：`/admin/play-ops` 成员修复弹窗、预检、阻断/警告、影响预览、确认、事件时间线和中英文资源。
- 数据库：阶段一不新增迁移；复用 `play_team_members`、`play_team_events`、`play_team_settlements`。
- Fork 保护：更新 `FORK-PLAY-003` 和 `scripts/check-fork-integrity.sh`。

### 测试命令与结果

RED 证据：

```text
go test -count=1 ./internal/service -run '^TestAdminTeamMemberCandidatePreviewCapturesServerNowOnce$'
结果：失败，expected now calls 1，actual 2。

pnpm exec vitest run src/views/admin/__tests__/PlayOpsView.spec.ts src/i18n/locales/adminPlayOpsParity.spec.ts
结果：失败 3 项，分别证明旧候选残留、跨月边界缓存、中文操作标签泄漏。
```

最终 GREEN 与服务器验证证据：

```text
go test -count=1 ./internal/service -run '^TestAdminTeamMemberCandidatePreviewCapturesServerNowOnce$'
结果：通过。

go test -count=1 ./internal/repository -run '^TestAdminTeamRepairRepositoryReadsArchivedSourceAndSafeEvents$'
结果：通过。

go test -count=1 ./internal/server/middleware ./internal/handler/admin ./internal/service ./internal/repository
结果：通过。

go test -tags=integration -count=1 ./internal/repository -run '^TestAdminTeamRepair'
结果：通过；真实 PostgreSQL 测试组完成，退出码 0。

pnpm exec vitest run \
  src/api/__tests__/admin.play.teamRepair.spec.ts \
  src/components/auth/__tests__/TotpStepUpDialog.spec.ts \
  src/components/common/__tests__/BaseDialog.spec.ts \
  src/i18n/__tests__/bilingualProductUi.spec.ts \
  src/i18n/locales/adminPlayOpsParity.spec.ts \
  src/views/admin/__tests__/PlayOpsView.spec.ts
结果：6 个文件、30 项测试通过。

make test
结果：通过；全量 Go 测试、golangci-lint（0 issues）、前端 ESLint、
vue-tsc 和 Vitest 全部通过；Vitest 为 236 个文件、1568 项测试。

GOFLAGS=-buildvcs=false make build
结果：通过；后端构建成功，前端 production build 成功。
关键构建资源：PlayOpsView-C6dod7an.js、zh-CCDRAMXD.js、en-JtftiADk.js。

./scripts/check-fork-integrity.sh
结果：通过；包含 FORK-PLAY-003 三条管理员路由、服务、中英文资源和定向测试保护。

git diff --check
结果：通过。
```

### 数据库与 API 对账

- 状态：已通过
- 真实 PostgreSQL 自动化对账已确认：
  - 并发添加只有一个请求成功，用户最终只有一个活跃 membership。
  - 跨队并发移动无死锁；旧 `left_at` 与新 `joined_at` 完全一致。
  - 移动分别写入来源队和目标队事件；最后一名队长移动时额外写入归档事件。
  - 原因中的数据库邀请码、`token`、`invite_code` 和自定义敏感串不会进入事件明文。
  - 修复会等待相同战队上的结算快照锁；快照生成后拒绝修改该不可变区间。
  - 事件写入失败时，membership 变更和归档操作全部回滚。
  - 已归档来源队在回溯时间后仍有成员历史时拒绝移动，避免错误重写历史。

### 中文与英文检查

- 状态：已通过（服务器自动化范围）
- 运行时 `zh/en` key 对称；成员修复、用户状态、结算状态、阻断、警告、确认、成功、错误和事件名称均有双语 key。
- 中文 Play Ops 自动化检查不出现 `Token/Tokens/Team/Arena` 操作标签；未知后端错误使用当前语言回退，不显示后端英文 `message`。
- 全量前端测试、类型检查和 production build 已通过。
- 生产资产核对及用户本地浏览器中文/英文、浅色/深色验收仍分别属于部署检查点和本地验收检查点。

### 审查发现与修复

- 软删除用户现在可搜索并显示“已删除 / Deleted”，但不能提交。
- 修复与月度结算共用战队行锁，避免修复穿过不可变快照。
- 锁顺序统一为用户、按 ID 排序的队伍、活跃 membership，并在锁后复核来源关系。
- 事件 detail 使用白名单过滤，邀请码和令牌不进入管理员事件 DTO。
- 系统原因使用 `reason_code`，前端按 locale 翻译。
- 搜索词、操作和生效时间变化会清空旧预检；重复搜索失败也不会保留旧候选。
- 同一次候选预检只捕获一次服务器时间，避免月末跨月数据窗口不一致。
- 每次打开修复弹窗都会刷新上海当前时间和本月起点。
- Play Ops 统一按稳定错误码翻译；无已知错误码时使用当前语言的功能级回退，不透传后端英文错误。
- 战队切换、榜单切换和事件加载失败均有明确错误处理，不会留下错误选择状态或静默失败。

### 剩余风险

- CP1 没有未解决的交付门禁。
- 后续钱包与提现阶段不得以 CP1 的验收证据替代各自的数据对账、部署证明和本地浏览器验收。

### 交付 commit

- 状态：已通过
- CP1 实现 commit：`89561e39a`（`feat(play): add controlled admin team member repair`）
- CP1 验证记录 commit：`5b4262e1c`（`docs(play): record CP1 validation checkpoint`）
- PR：`#82`，目标分支 `play/main`
- 合并 commit：`09e03463aac5e7626a3f0daaa09f9c8131cf049b`
- Zeabur deployment：`5528554772`，状态 `success`

## CP2 战队奖励与余额透明度

### 检查点状态

| 节点 | 状态 | 证据或剩余工作 |
|---|---|---|
| 需求对照检查点 | 已通过 | 已对照 CP2 要求完成：管理员队伍透明度、用户 Agent Team 本人结算、用户钱包汇总/流水、安全 DTO、统一流水复用、双语来源/状态/分页/空状态 |
| RED 失败测试检查点 | 已通过 | 在临时基线 worktree `09e03463...` 套入新增钱包后端测试，`go test -count=1 ./internal/service -run TestWallet` 按预期失败，缺少 `NewWalletService`、`WalletTransactionQuery` 和来源映射 |
| GREEN 实现检查点 | 已通过 | 钱包服务/handler/路由、用户结算隐私 DTO、管理员成员最近奖励字段、前端 `/wallet`、Agent Team 和 Play Ops 展示均已实现；聚焦 Go/Vitest/i18n 测试通过 |
| 规格审查 | 已通过 | 已修复审查发现的 `source=other` 漏筛未知历史来源、图片任务冻结流水筛选不一致、任务预留变动在钱包金额列不可见三项细节 |
| 代码质量审查 | 已通过 | 钱包 DTO 不暴露 metadata、description、source_id、idempotency、actor/admin reason、邮箱或其他用户资料；Vue 用户文案走 i18n，中文页面无英文状态/按钮/空状态回退 |
| 完整验证 | 已通过 | 聚焦验证、PostgreSQL 对账、`make test`、`GOFLAGS=-buildvcs=false make build`、`./scripts/check-fork-integrity.sh`、`git diff --check` 全部通过 |
| PR / GitHub CI | 已通过 | PR #83 全部 GitHub checks 通过：backend-security、frontend-security、Documentation、Protected behavior |
| 部署核对 | 已通过 | Zeabur deployment `5533768058` 成功部署 CP2 merge commit `6531cb83dde134bccbde139220bd29e3623e36d7`；健康、路由保护和前端资产均已核对 |
| 本地浏览器验收 | 已通过 | 用户于 2026-07-21 确认普通用户钱包、Agent Team、管理员 Play Ops 的中文/英文、浅色/深色本地浏览器验收通过 |

### 基线与计划变更范围

- 基线 commit：`09e03463aac5e7626a3f0daaa09f9c8131cf049b`
- 后端：新增用户钱包汇总与安全流水 API；用户战队结算改为仅返回本人分配；管理员战队成员补充最近一期实际奖励、状态和入账时间。
- 前端：新增 `/wallet` 页面和导航；Agent Team 显示结算月份、团队奖励池、个人分成、状态和入账时间；Play Ops 显示加入时间和最近一期实际奖励。
- 数据库：阶段二不新建余额账；直接读取统一 `balance_transactions`，并与战队结算/分配记录三方对账。
- 双语：所有新增状态、来源、筛选、金额方向、日期、空状态、分页与错误回退同时覆盖中文和英文。

### 需求对照结论

- 现有管理员完整余额流水继续复用，不重复建设。
- 用户钱包 DTO 不返回 `metadata`、内部描述、来源 ID、幂等键、管理员/操作者信息、邮箱、可信度或内部原因。
- 用户战队结算只能查询当前用户自己的 allocation，不能继续把全队成员分配明细返回给普通用户。
- 金额和日期由前端按当前 locale 格式化，中文界面不得出现硬编码英文状态或固定英文金额格式。
- CP2 部署并完成本地浏览器验收前，CP3 保持未开始。
- 钱包来源筛选使用稳定公开分类；未知历史来源落入“其他”，图片任务相关 `image_balance_*` 流水归入“图片任务”。
- `frozen_balance` 在 CP2 只作为“任务预留”展示，不与后续 CP4/CP5 的可提现冻结概念混用。

### 测试命令与结果

RED 证据：

```text
临时 worktree：/home/dell/worktrees/sub2api-cp2-red-check-*，基线 09e03463aac5e7626a3f0daaa09f9c8131cf049b
命令：go test -count=1 ./internal/service -run TestWallet
结果：失败，退出码 1。
关键失败：undefined: NewWalletService、undefined: WalletTransactionQuery、undefined: WalletPublicSourceTeamReward、
undefined: walletRawSourceFilter、undefined: WalletPublicSourceOther。
结论：CP2 基线缺少用户钱包服务、查询模型和公开来源映射。
```

GREEN 聚焦证据：

```text
go test -count=1 ./internal/service -run 'TestWallet|TestUserTeamRewardSettlements'
go test -count=1 ./internal/server/routes -run 'TestUserWalletRoutesContract'
go test -count=1 ./internal/handler -run 'Test.*Team|Test.*Wallet|TestPlay'
go test -count=1 ./internal/repository -run 'TestTeamReward|Test.*Team'
go test -count=1 ./cmd/server
结果：全部通过。

pnpm vitest run \
  src/api/__tests__/wallet.spec.ts \
  src/router/__tests__/walletRouting.spec.ts \
  src/views/user/__tests__/WalletView.spec.ts \
  src/views/public/__tests__/AgentTeamView.competitive.spec.ts \
  src/views/admin/__tests__/PlayOpsView.spec.ts \
  src/i18n/__tests__/navLocaleKeys.spec.ts \
  src/i18n/locales/adminPlayOpsParity.spec.ts \
  src/i18n/__tests__/localesMessageCompile.spec.ts
结果：8 个文件、29 项测试通过。

pnpm vitest run src/views/user/__tests__/WalletView.spec.ts src/i18n/__tests__/localesMessageCompile.spec.ts
结果：2 个文件、3 项测试通过；覆盖任务预留冻结变动文案。
```

完整服务器验证证据：

```text
make test
结果：通过；后端全量 Go 测试通过，golangci-lint 为 0 issues；前端 ESLint、
vue-tsc 和 Vitest 全部通过；Vitest 为 239 个文件、1574 项测试。

CI 安全补修：
frontend-security 首轮发现 axios 新高危公告 GHSA-gcfj-64vw-6mp9。
修复：升级 `frontend` 依赖 axios `1.16.0 -> 1.18.1`。
验证：`CI=true npx -y pnpm@9.15.9 install --frozen-lockfile` 通过；
`CI=true npx -y pnpm@9.15.9 audit --prod --audit-level=high --json`
配合 `tools/check_pnpm_audit_exceptions.py` 通过，仅剩已有 xlsx 例外。

GOFLAGS=-buildvcs=false make build
结果：通过；后端二进制构建成功，前端 production build 成功。
关键构建资源：WalletView-Didu7ySV.js、PlayOpsView-DM7PaIbn.js、
AgentTeamView-BnyP2EJb.js、zh-8NyoUD2n.js、en-DGxkFgof.js。

./scripts/check-fork-integrity.sh
结果：通过；Fork integrity passed。

git diff --check
结果：通过。
```

GitHub CI 证据：

```text
PR：#83，目标分支 play/main
head：e8d4acf9f6e67c75e26acd0168c0ec884fdb1a3d
checks：backend-security = success；frontend-security = success；Documentation = success；Protected behavior = success
合并方式：merge commit，非 rebase
合并 commit：6531cb83dde134bccbde139220bd29e3623e36d7
```

生产部署核对：

```text
Zeabur deployment：5533768058
environment：production
sha：6531cb83dde134bccbde139220bd29e3623e36d7
state：success
target_url：https://zeabur.com/projects/6a51de14c2881a93656fa4c5/services/6a51ee2df04125ac9a34c28c/deployments/6a5f0f7de9e39c7397905a74?envID=6a51de14104975fcb46761c3

https://www.jisudeng.com/health -> 200 application/json，{"status":"ok"}
https://www.jisudeng.com/wallet -> 200 text/html
https://www.jisudeng.com/admin/play-ops -> 200 text/html
GET /api/v1/user/wallet/summary -> 401 application/json，UNAUTHORIZED
GET /api/v1/user/wallet/transactions -> 401 application/json，UNAUTHORIZED
GET /api/v1/play/teams/settlements -> 401 application/json，UNAUTHORIZED
GET /api/v1/admin/play/teams/1/settlements -> 401 application/json，UNAUTHORIZED

生产前端资源：index-CHOYH8ey.js、WalletView-Didu7ySV.js、PlayOpsView-DM7PaIbn.js、
AgentTeamView-BnyP2EJb.js、zh-8NyoUD2n.js、en-DGxkFgof.js。
WalletView 资源包含 `/user/wallet/summary`、`/user/wallet/transactions`
和 `wallet.table.taskReservedChange`；zh/en 资源包含钱包、任务预留变动、
待发放/已入账/Pending payout/Paid 等双语文案。
```

### 数据库与 API 对账

- 状态：已通过（本地临时 PostgreSQL）
- 对账数据使用 `users`、`balance_transactions`、`play_team_settlements`、`play_team_reward_allocations` 和 `play_teams` 最小真实表结构。
- 对账结果：

```text
wallet_summary: passed = t
available = 18.50000000, reserved = 3.25000000, credits = 26.00000000, debits = 7.50000000, tx_count = 3

user_settlements_privacy: passed = t
records = 2026-06:old team:paid, 2026-07:new team:processing

admin_member_latest_reward: passed = t
user_id = 11, month = 2026-07, reward_amount = 4.00000000, payout_status = processing

team_reward_ledger_reconciliation: passed = t
paid_allocations = 15.00000000, ledger_rewards = 15.00000000
```

### 中文与英文检查

- 状态：已通过（本地自动化范围）
- 新增 `nav.wallet`、`wallet.*`、`agentTeam.personalShare`、管理员成员最近奖励字段均同时覆盖中文和英文。
- 钱包来源、方向、任务预留变动、分页、空状态、加载失败和表头均走 i18n。
- `adminPlayOpsParity`、`navLocaleKeys`、`localesMessageCompile` 已通过；中文钱包和 Play Ops 组件测试无英文 key 回退。
- 用户于 2026-07-21 完成本地浏览器验收；验收覆盖普通用户钱包、Agent Team、管理员 Play Ops、中文/英文与浅色/深色。

### 审查发现与修复

- 用户钱包只读取本人 `balance_transactions`，返回安全 DTO；不暴露内部 metadata、管理员原因、邮箱或其他用户信息。
- `source=other` 已改为排除所有已知公开来源后的剩余流水，避免未知历史来源在“全部”里显示为其他但筛选不到。
- 图片任务来源补齐 `image_balance_hold`、`image_balance_capture`、`image_balance_release`，并保留 `image` 兜底匹配。
- 钱包金额列新增“任务预留变动”，避免可用余额转冻结时净变化为 0 而用户看不到预留变化。
- 用户 Agent Team 结算接口改为按 `allocation.user_id` 返回本人历史记录，兼容旧数组结构但新 DTO 不返回其他成员明细。
- 管理员队伍成员补充加入时间、最近一期实际奖励、发放状态和入账时间。
- 复用现有管理员完整余额流水，不新建管理员流水系统。

### 剩余风险

- CP2 没有未解决的交付门禁。
- CP3 必须从最新 `origin/play/main` 创建独立 worktree，并重新经过 RED、GREEN、审查、验证、PR、部署和本地验收。
- CP2 不启用提现能力，不改变 `frozen_balance` 现有任务预留语义；CP4/CP5 需独立处理可提现权益与提现冻结。

### 交付 commit

- 状态：已通过
- CP2 实现 commit：`5dc5afb80`（`feat(play): add wallet transparency`）
- CP2 本地验证记录 commit：`8408a12d3`（`docs(play): record CP2 local validation`）
- CP2 CI 安全补修 commit：`e8d4acf9f`（`fix(frontend): update axios for audit gate`）
- PR：`#83`，目标分支 `play/main`
- 合并 commit：`6531cb83dde134bccbde139220bd29e3623e36d7`
- Zeabur deployment：`5533768058`，状态 `success`
- CP2 部署记录 commit：`39de5dd19`（`docs(play): record CP2 deployment checkpoint`）
- CP2 验收记录 commit：本提交（`docs(play): record CP2 local acceptance`）
