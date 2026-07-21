# Play、钱包与提现交付检查点

> 状态：active
> 当前阶段：阶段五 / CP5
> 当前结论：CP1、CP2、CP3、CP4 已通过用户本地验收；CP5 提现权限、申请、审核与线下打款代码、PR/CI、合并和部署证明已完成，等待用户本地浏览器验收。
> CP5 基线：`origin/play/main@3da334ddcacb10bae2a64a4a707381c57697837b`
> CP5 功能分支：`feat/play-withdrawals-cp5-20260721`
> 工作树：`/home/dell/worktrees/sub2api-play-withdrawals-cp5-20260721`
> 最后更新：2026-07-21

本文档只记录已经取得证据的状态。允许状态仅为：
`未开始 / 进行中 / 已通过 / 失败 / 阻塞 / 等待本地验收`。
当前阶段部署并完成本地浏览器验收前，不得开始依赖它的下一阶段。

## 总览

| 阶段或检查点 | 状态 | 当前门禁 |
|---|---|---|
| 阶段一：管理员手动修复战队成员 / CP1 | 已通过 | PR、CI、部署证明及用户本地双语/主题/添加移动闭环均已通过 |
| 阶段二：战队奖励与余额透明度 / CP2 | 已通过 | 本地实现、数据库对账、完整服务器验证、审查、PR/CI、合并、生产部署证明和用户本地浏览器验收均已通过 |
| 阶段三：代币农场最近发放与当前预估 / CP3 | 已通过 | 代码、PR/CI、合并、部署证明、用户本地验收均已通过；进入 CP4 前已补默认进入日榜 hotfix |
| 阶段四：可提现权益账务基础 / CP4 | 已通过 | 代码、审查、服务器验证、PR/CI、合并、部署证明和用户本地浏览器验收均已通过；提现总开关保持关闭 |
| 阶段五：提现权限、申请、审核与线下打款 / CP5 | 等待本地验收 | 代码、审查、服务器验证、PR/CI、合并和部署证明已通过；等待用户本地浏览器验收，提现总开关仍默认关闭 |
| CP6：服务器验证 | 已通过 | CP1-CP5 本地定向测试、完整 `make test`、`GOFLAGS=-buildvcs=false make build`、fork integrity 和 `git diff --check` 均已通过 |
| CP7：审查与合并 | 已通过 | CP1 PR #82、CP2 PR #83、CP3 PR #87、默认日榜 hotfix PR #92、CP4 PR #94、CP5 PR #96 均已全绿并以非 rebase merge 合入 |
| CP8：部署证明 | 已通过 | CP1-CP5 均已完成 live 健康、API 保护和前端资产 proof；CP5 merge commit `00968312a3b842ac11966a9c9fba85c52dcbeb3c` 的 Zeabur 检查成功 |
| CP9：最终本地浏览器验收 | 等待本地验收 | CP1、CP2、CP3、CP4 已通过；CP5 等待用户本地浏览器验收 |

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

## CP3 Token Farm 最近发放与当前预估

### 检查点状态

| 节点 | 状态 | 证据或剩余工作 |
|---|---|---|
| 需求对照检查点 | 已通过 | 已对照 CP3 要求完成：公开 reward-summary API、最近已结算日榜、当前日榜预估、`settled_at` 迁移、日榜发放 detail、历史缺失 detail 安全回补、隐私脱敏和双语前端面板 |
| RED 失败测试检查点 | 已通过 | 新增后端 service/repository/migration 和前端 ArenaView 测试后，基线按预期失败，证明缺少 summary 类型、period `settled_at`、仓库方法、迁移文件和前端 API 调用 |
| GREEN 实现检查点 | 已通过 | 后端聚焦测试和 ArenaView 聚焦测试已全部转绿；本地完整服务器验证已通过 |
| 规格审查 | 已通过 | 已复审不改写旧流水、不暴露邮箱、最近发放不等同今日榜、当前周期文案、settled_at 回填和 reward ledger 总额口径 |
| 代码质量审查 | 已通过 | 已复审 i18n、DTO、SQL fallback、排序/截断、测试隔离和 fork integrity 保护；lint/typecheck/build 通过 |
| 完整验证 | 已通过 | 聚焦验证、`make test`、`GOFLAGS=-buildvcs=false make build`、`./scripts/check-fork-integrity.sh` 和 `git diff --check` 全部通过 |
| PR / GitHub CI | 已通过 | PR #87 全部 GitHub checks 通过：backend-security、frontend-security、Documentation、Protected behavior；以 merge commit、非 rebase 合入 `play/main` |
| 部署核对 | 已通过 | Zeabur deployment `5535370977` 成功部署 CP3 merge commit `c8756db428e26cbced218f9256aaecaa54eca1d4`；健康、公开 API、隐私形状和前端资产均已核对 |
| 本地浏览器验收 | 已通过 | 用户于 2026-07-21 确认 CP3 本地浏览器验收通过；进入 CP4 前另补默认进入日榜 hotfix |

### 基线与计划变更范围

- 基线 commit：`e4aa4efe54b97154ca8b7d933b05866d69e827f5`
- 后端：新增 `GET /api/v1/play/arena/daily/reward-summary`；period 模型和仓库查询补 `period_type`、`settled_at`；日榜结算写入 period/rank/token detail；最近发放从 `play_reward_ledger` 读取并用 period 时间窗回补历史缺失 rank/token。
- 前端：`ArenaView` 日榜 tab 新增“最近发放”和“当前预估”面板；新增 API 类型和调用；新增中文/英文 `arena.dailySummary.*` 文案。
- 数据库：新增 `209_play_arena_daily_reward_summary.sql`，给 `play_arena_periods` 增加 `settled_at`，历史 settled period 以 `updated_at` 尽力回填，并添加 daily settled 查询索引。
- Fork 保护：`FORK-PLAY-003` 和 `FORK-MIGRATION-009` 增加 reward-summary 路由、服务、前端双语、迁移和聚焦测试保护。

### 测试命令与结果

RED 证据：

```text
go test -count=1 ./internal/service -run 'TestDailyArena'
结果：失败，缺少 PlayArenaDailyRewardLedgerRow、PlayArenaPeriod.PeriodType/SettledAt 和 GetDailyArenaRewardSummary。

go test -count=1 ./internal/repository -run 'TestDailyArenaRewardSummary|TestArenaPeriodQueriesExpose'
结果：失败，缺少 GetLatestSettledDailyArenaPeriod、ListArenaDailyRewardLedger 和 period settled_at 字段。

go test -count=1 ./migrations -run 'TestPlayArenaDailyRewardSummaryMigrationContract'
结果：失败，209_play_arena_daily_reward_summary.sql 不存在。

pnpm exec vitest run src/views/public/__tests__/ArenaView.competitive.spec.ts
结果：失败，getArenaDailyRewardSummary 未被调用。
```

GREEN 聚焦证据：

```text
go test -count=1 ./internal/service ./internal/repository ./internal/handler ./migrations -run '^(TestDailyArena|TestArenaPeriodQueriesExpose|TestArenaDailyRewardSummaryRoute|TestPlayArenaDailyRewardSummaryMigrationContract)'
结果：通过。

pnpm exec vitest run src/views/public/__tests__/ArenaView.competitive.spec.ts
结果：通过；3 项测试通过。
```

完整服务器验证证据：

```text
pnpm exec vitest run src/views/public/__tests__/ArenaView.competitive.spec.ts src/i18n/__tests__/localesMessageCompile.spec.ts
结果：通过；2 个文件、5 项测试通过。

pnpm exec vue-tsc --noEmit
结果：通过。

./scripts/check-fork-integrity.sh
结果：通过；包含 CP3 reward-summary 后端、前端和迁移保护。

make test
结果：通过；后端全量 Go 测试通过，golangci-lint 为 0 issues；前端 ESLint、
vue-tsc 和 Vitest 全部通过；Vitest 为 239 个文件、1575 项测试。

GOFLAGS=-buildvcs=false make build
结果：通过；后端二进制构建成功，前端 production build 成功。
关键构建资源：ArenaView-DpAW9DeC.js、jisudeng-pages.zh-7vuuOAtE.js、
zh-CSxqxLU2.js、en-B7u3TFCS.js。

git diff --check
结果：通过。
```

### 数据库与 API 对账

- 状态：已通过（本地自动化范围）；生产 live API 已通过，生产直连 SQL 对账未执行
- 自动化已覆盖：最近发放总金额等于日榜奖励流水汇总；最多返回 10 名脱敏获奖者，但 `winners_count` 和 `total_amount` 统计全量流水；历史缺失 rank/token 时按 period 时间窗查询排行榜回补；route contract 确认公开 GET 不触发 JWT，响应不包含 `email` 字段或邮箱明文。
- 生产 live API：`GET https://www.jisudeng.com/api/v1/play/arena/daily/reward-summary` 返回 200；`code=0`、`enabled=true`、recent period `2026-07-20`、current period `2026-07-21`、`winners_count=10`、返回 winners 10 名、current estimate 10 行；响应不包含邮箱字段或邮箱明文。
- 当前服务器环境没有生产 `DATABASE_URL` 或 PG 凭据，因此未执行生产直连 SQL 对账。生产 API 的最近发放数据来自 `play_reward_ledger` 查询路径；独立 SQL 汇总仍需在有生产 DB 凭据时补做，不作为本地浏览器验收的替代。

### 中文与英文检查

- 状态：已通过（本地自动化范围）
- 新增 `arena.dailySummary.*` 已覆盖中文和英文；Vue 新增面板无硬编码状态、按钮、错误或空状态文案。
- 新增日榜面板和排行榜代币数量显示已接入 i18n；中文显示“枚代币”，英文显示 `tokens`，避免 CP3 新增可见区域出现英文单位泄漏。
- 当前预估使用“当前周期 / Current period”，最近发放使用“结算周期 / Settlement period”，避免把昨日已结算榜标为今日排名。
- `localesMessageCompile` 已通过；中英文浏览器验收仍属于部署后的本地验收门禁。

### 审查发现与修复

- 日榜结算新流水 detail 补齐 `period_id`、`period_name`、`period_type`、`period_start`、`period_end`、`period`、`rank`、`token` 和 `token_sum`。
- 历史缺失字段只在 summary 读取时通过 period 时间窗回补，不改写旧 `play_reward_ledger`。
- `paid_today` 按 Asia/Shanghai 自然日比较 `settled_at` 和服务器当前时间。
- 公开 DTO 不返回邮箱；仓库内部只用邮箱生成脱敏 `display_name`。
- 最近发放统计全量流水，展示最多 10 名获奖者；当前预估按配置奖励档位查询并展示最多 10 名。
- `ArenaView` 原有排行榜行的硬编码 `tokens` 已改为 locale key，CP3 触达的中文日榜区域不再新增英文单位文案。

### 剩余风险

- 用户本地浏览器验收已由用户在 2026-07-21 确认通过。
- 生产直连 SQL 汇总对账未执行，原因是当前服务器环境没有生产 DB 凭据；本地自动化已经覆盖 reward ledger 汇总一致，生产 live API 已证明迁移和公开查询路径可用。
- 进入 CP4 前补充的默认进入日榜 hotfix 已通过 PR #92 合入 `play/main`，merge commit `75cc64473be41b646bf32a8fd51f6df8e3581461`，Zeabur deployment `6a5f4bbbe9e39c7397906c09` 成功。

### 交付 commit

- 状态：已通过
- 实现 commit：`10ea15649f04e1101a9c321e1fac3ebd9434942e`（`feat(play): add daily arena reward summary`）
- PR：`#87`，目标分支 `play/main`
- 合并 commit：`c8756db428e26cbced218f9256aaecaa54eca1d4`
- Zeabur deployment：`5535370977`，状态 `success`
- 生产前端资源：`index-DKNZxRRJ.js`、`ArenaView-DpAW9DeC.js`、`jisudeng-pages.zh-7vuuOAtE.js`、`zh-CSxqxLU2.js`、`en-B7u3TFCS.js`

## CP4 可提现权益账务基础

### 检查点状态

| 节点 | 状态 | 证据或剩余工作 |
|---|---|---|
| 需求对照检查点 | 已通过 | CP4 只做账务基础，不开放提现申请/审核；`frozen_balance` 保持图片/任务预留，新增 `withdrawable_balance` 与 `withdrawal_frozen_balance` |
| RED 失败测试检查点 | 已通过 | 新增迁移、wallet summary 和 withdrawable helper 测试后，后端按预期失败于缺少 `WithdrawableBalance` 字段与权益 helper |
| GREEN 实现检查点 | 已通过 | 已实现 migration、四余额流水扩展、权益批次、allocation、钱包透明度、图片 release 恢复原 allocation、dry-run/execute 重算器和不变量检查 |
| 规格审查 | 已通过 | 已逐项审查权益来源白名单、消费 FIFO、退款恢复原 allocation、72 小时成熟、图片冻结隔离和重算策略；未发现需要进入 CP5 的内容 |
| 代码质量审查 | 已通过 | 已审查 SQL 事务顺序、decimal 精度、错误边界和前端双语；补修纯可提现变更缓存失效与负余额重算显示边界 |
| 完整验证 | 已通过 | targeted、`make test`、`GOFLAGS=-buildvcs=false make build`、`./scripts/check-fork-integrity.sh` 和 `git diff --check` 全部退出码 0 |
| PR / GitHub CI | 已通过 | PR #94 四项 GitHub checks 全绿：backend-security、frontend-security、Documentation、Protected behavior；以 merge commit、非 rebase 合入 `play/main` |
| 部署核对 | 已通过 | `origin/play/main@1cb415ca5b849c1a11d24fc93fc5229931b0f2e0` 已上线；健康、钱包 API 保护、前端入口、钱包 chunk 和中英文资源均已核对 |
| 本地浏览器验收 | 已通过 | 用户于 2026-07-21 确认 CP4 本地浏览器验收通过 |

### 基线与计划变更范围

- 基线 commit：`725916689571325cef5ccacb2e8ddc7c4961de07`
- 后端：扩展 `balance_transactions` 的可提现与提现冻结 before/after；新增 `withdrawable_entitlements`、`withdrawable_entitlement_allocations`、重算记录表；`BalanceLedgerService` 在同一事务内维护权益批次和 allocation。
- 钱包：`/wallet` summary 增加可提现、待解冻、提现冻结、任务预留四个概念；用户流水仍只读本人安全 DTO。
- 重算：新增 `backend/cmd/recompute-withdrawable-entitlements` 和 `backend/scripts/recompute-withdrawable-entitlements.sh`；默认 dry-run，`--execute` 才写入，`--check-invariants` 检查账务不变量。
- 双语：钱包新增概念已加入中文和英文；提现申请、审核、收款账户仍属于 CP5，当前不暴露入口。

### 已观察 RED 证据

```text
go test -count=1 ./internal/service ./migrations -run 'TestWithdrawable|TestWalletSummaryReadsUserBalanceAndUnifiedLedgerTotals'
结果：失败，缺少 WalletSummary.WithdrawableBalance / PendingWithdrawableBalance / WithdrawalFrozenBalance，
且缺少 classifyWithdrawableGrant、planWithdrawableConsumption、planWithdrawableRestore 等 helper。
```

### 当前 GREEN 证据

```text
go test -count=1 ./internal/service -run 'TestWithdrawable(Recompute|Invariant|Grant|Consumption|Restore)|TestBalanceLedger(GrantArenaDaily|ImageHoldConsumes|ReleaseRestores)'
结果：通过。

go test -count=1 ./internal/service -run 'TestUsageServiceCreateWritesUsageChargeBalanceLedger|TestRefundBalanceLedgerWritesDeductAndRollbackTransactions|TestAuth.*FirstBind|TestWithdrawable|TestBalanceLedger'
结果：通过。

go test -count=1 ./internal/service ./internal/repository ./migrations ./cmd/recompute-withdrawable-entitlements
结果：通过；service 96.265s，repository 3.151s，migrations 0.042s，recompute cmd 无测试文件。

pnpm --dir frontend exec vitest run src/api/__tests__/wallet.spec.ts src/views/user/__tests__/WalletView.spec.ts src/i18n/__tests__/localesMessageCompile.spec.ts
结果：3 个文件、5 项测试通过。

go test -count=1 ./internal/service -run 'TestBalanceLedgerApplyDeltaInvalidatesWhenOnlyWithdrawableChanges|TestWithdrawableRecomputeClampsNegativeUserBalanceToZeroWithdrawable|TestWithdrawable|TestBalanceLedger'
结果：通过；补充验证纯可提现变更缓存失效和负余额重算边界。
```

### 完整验证

```text
make test
结果：通过；全量 Go 测试通过，golangci-lint 0 issues，前端 ESLint、vue-tsc 和 Vitest 全部通过；Vitest 为 239 个文件、1586 项测试。

GOFLAGS=-buildvcs=false make build
结果：通过；后端 server 构建成功，前端 production build 成功。
关键构建资源：index-BlY4jKv4.js、WalletView-DvrqYkEx.js、zh-D8MW4Uzr.js、en-_FFos_rT.js。

./scripts/check-fork-integrity.sh
结果：通过；新增 FORK-BILLING-010 的 withdrawable recompute 命令、脚本、图片 release 恢复 allocation 和钱包可提现透明度保护。

git diff --check
结果：通过。
```

### 数据库 / API 对账

- 状态：已通过（本地自动化 + live API 保护范围）
- 迁移测试确认 `users`、`balance_transactions`、`withdrawable_entitlements`、`withdrawable_entitlement_allocations` 和 `withdrawable_recalculation_runs` 字段与约束可创建。
- 账本测试确认返利转余额立即可提，农场日榜/月榜和 Agent Team 共享奖励 72 小时后可提，签到、答题、盲盒、充值、赠送和未知来源默认不生成可提现权益。
- 消费测试确认先扣不可提现余额，不足部分按 `available_at,id` FIFO 消耗权益批次。
- 回滚测试确认 refund/release 通过原始 ledger key 恢复原交易实际消耗的权益批次。
- 不变量测试确认权益不超过余额、批次汇总一致、提现冻结汇总一致、图片来源不会写 `withdrawal_frozen_delta`。
- 生产 `GET /api/v1/user/wallet/summary` 未登录返回 `401` 和稳定错误码 `UNAUTHORIZED`，证明路由存在且受认证保护；当前环境无生产 `DATABASE_URL`，未执行生产 DB invariant。

### 中文与英文检查

- 状态：已通过（本地自动化 + 生产资产范围）
- 钱包新增概念 `可提现 / 待解冻 / 提现冻结 / 任务预留` 与英文 `Withdrawable / Pending Thaw / Withdrawal Frozen / Task Reserved` 已加入 `zh.ts` 和 `en.ts`。
- 钱包组件测试覆盖中文标签渲染；locale 编译测试通过。
- 生产资产 `WalletView-DvrqYkEx.js` 包含 `withdrawable_balance`、`pending_withdrawable_balance`、`withdrawal_frozen_balance`、`task_reserved_balance`；`zh-D8MW4Uzr.js` 和 `en-_FFos_rT.js` 均包含新增钱包概念。
- CP4 未新增提现申请、审核、收款账户或管理员提现页文案，避免提前进入 CP5。

### 审查发现与修复

- `frozen_balance` 只继续代表图片/任务预留；提现冻结独立使用 `withdrawal_frozen_balance` 和权益批次字段。
- 所有新增可提现和提现冻结金额使用 `decimal` 与 `NUMERIC(20,8)`；HTTP 钱包金额继续用字符串。
- 历史重算默认 dry-run；execute 仅对 `ready` 用户写入权益和 `withdrawable_balance`，异常用户标记 `needs_review`，不覆盖可提现余额。
- 审查补修：纯可提现/提现冻结变更现在也触发余额缓存失效，避免后续 CP5 只冻结提现时缓存不更新。
- 审查补修：历史重算遇到负余额异常用户时，`computed_withdrawable_balance` 显示为 0，并将用户标记为 `needs_review`。

### 剩余风险

- 用户本地浏览器验收已由用户在 2026-07-21 确认通过。
- 生产数据库 dry-run / invariant 仍需要具备生产 `DATABASE_URL` 的环境执行；当前服务器环境没有该变量，且文档不保存凭据。
- 提现申请、审核、收款账户加密、单审/双审和线下打款均属于 CP5，当前未实现，提现总开关保持关闭。

### 交付 commit

- 本地实现 commit：`611569fb4`
- 文档回填 commit：`3417fb329`
- PR：#94
- 合并 commit：`1cb415ca5b849c1a11d24fc93fc5229931b0f2e0`
- 生产前端资源：`index-BlY4jKv4.js`、`WalletView-DvrqYkEx.js`、`zh-D8MW4Uzr.js`、`en-_FFos_rT.js`
- 生产 live proof：`/health` 返回 200 `{"status":"ok"}`；`/api/v1/user/wallet/summary` 未登录返回 401 `UNAUTHORIZED`；上述 CP4 资产均返回 200。

## CP5 提现权限、申请、审核与线下打款

### 检查点状态

| 节点 | 状态 | 证据或剩余工作 |
|---|---|---|
| 需求对照检查点 | 已通过 | 已实现总开关、默认限额、ready 用户启用限制、收款账户、提现申请/取消、管理员队列、单审/双审、拒绝/取消恢复、线下打款登记、审计脱敏和双语界面 |
| RED 失败测试检查点 | 已通过 | 新增提现 API、路由、迁移、账务冻结、审批和钱包/管理员前端测试后，基线缺少提现 service、routes、migration 和前端入口，按预期失败 |
| GREEN 实现检查点 | 已通过 | 后端提现 service、权益锁定/恢复、迁移、用户钱包提现页、管理员 `/admin/withdrawals`、通知邮件和中英文资源均已实现并通过聚焦测试 |
| 规格审查 | 已通过 | 已复审默认关闭、最低 `$10`、上海自然日 `$500`、`$100` 双人审核、72 小时成熟、单进行中申请、禁止自审、管理员 API Key 禁止敏感动作和敏感请求体不入通用审计 |
| 代码质量审查 | 已通过 | 已复审事务锁、decimal/`NUMERIC(20,8)`、HTTP 金额字符串、未导出加密快照字段、前端 fallback 翻译和 fork integrity 保护；未发现需阻断合并的问题 |
| 完整验证 | 已通过 | targeted、`pnpm --dir frontend typecheck`、`pnpm --dir frontend build`、`make test`、`GOFLAGS=-buildvcs=false make build`、`./scripts/check-fork-integrity.sh` 和 `git diff --check` 全部退出码 0 |
| PR / GitHub CI | 已通过 | PR #96 四项 GitHub checks 全绿：Protected behavior、Documentation、backend-security、frontend-security；以 merge commit、非 rebase 合入 `play/main` |
| 部署核对 | 已通过 | `origin/play/main@00968312a3b842ac11966a9c9fba85c52dcbeb3c` 已上线；Zeabur、前后端安全检查通过；健康、API 保护、前端入口、提现 chunk 和中英文资源均已核对 |
| 本地浏览器验收 | 等待本地验收 | 等待用户本地浏览器覆盖中文/英文、浅色/深色、用户钱包收款账户/申请/取消/历史、管理员权限设置/审核/敏感资料读取/线下打款闭环 |

### 基线与计划变更范围

- 基线 commit：`3da334ddcacb10bae2a64a4a707381c57697837b`
- 后端：新增 `withdrawal_system_settings`、`user_withdrawal_settings`、`withdrawal_payout_accounts`、`withdrawal_requests`、`withdrawal_status_events` 和 `withdrawal_request_entitlements`；提现资金在提交时原子减少余额和可提现权益、增加 `withdrawal_frozen_balance`，取消或拒绝恢复原权益批次，打款后消耗锁定权益。
- 用户 API：新增收款账户读取/修改、提现可用性、提现申请列表/详情/创建/取消；用户钱包继续只返回本人安全 DTO。
- 管理员 API：新增提现队列、详情、规则、单用户/批量权限、批准、拒绝、敏感收款资料读取和标记已打款；审批、拒绝、读取完整资料和打款登记均要求 JWT 管理员 TOTP step-up。
- 前端：`/wallet` 增加收款账户、申请提现、提现记录和状态历史；新增 `/admin/withdrawals` 专用管理页；管理侧菜单新增“提现管理”。
- 双语：用户钱包和管理员提现管理全部新增中文/英文文案，错误码由前端按当前 locale 翻译，中文环境不显示英文 fallback。
- Fork 保护：更新 `FORK-BILLING-010` 和 `scripts/check-fork-integrity.sh`，保护提现迁移、service、routes、管理员页面和双语资源。

### 已观察 RED 证据

```text
go test -count=1 ./internal/server/routes ./internal/handler ./internal/handler/admin ./internal/service -run 'Test(AdminWithdrawal|UserWallet|WithdrawalsCP5|ParseWithdrawal|WithdrawalFreeze|WithdrawalApproval|WithdrawalAccountMask|BalanceLedgerWithdrawal)'
结果：基线失败，缺少 admin withdrawal handler、user withdrawal routes、withdrawal service、冻结/审批账务和钱包提现 DTO。

pnpm --dir frontend test:run src/api/__tests__/wallet.withdrawals.spec.ts src/api/__tests__/admin.withdrawals.spec.ts src/router/__tests__/withdrawalRouting.spec.ts src/views/user/__tests__/WalletView.spec.ts
结果：基线失败，缺少 wallet/admin withdrawal API、管理员提现路由和钱包提现界面。
```

### 当前 GREEN 证据

```text
pnpm --dir frontend test:run src/api/__tests__/wallet.withdrawals.spec.ts src/api/__tests__/admin.withdrawals.spec.ts src/router/__tests__/withdrawalRouting.spec.ts src/views/user/__tests__/WalletView.spec.ts
结果：4 个文件、7 项测试通过。

go test -count=1 ./internal/server/routes ./internal/handler ./internal/handler/admin ./internal/service -run 'Test(AdminWithdrawal|UserWallet|WithdrawalsCP5|ParseWithdrawal|WithdrawalFreeze|WithdrawalApproval|WithdrawalAccountMask|BalanceLedgerWithdrawal)'
结果：通过。

go test -count=1 ./internal/service ./migrations -run 'Test(WithdrawalsCP5|ParseWithdrawal|WithdrawalFreeze|WithdrawalApproval|WithdrawalAccountMask|BalanceLedgerWithdrawal)'
结果：通过。

pnpm --dir frontend typecheck
结果：通过。

pnpm --dir frontend build
结果：通过。
```

### 完整验证

```text
make test
结果：通过；全量 Go 测试、golangci-lint、前端 ESLint、vue-tsc 和 Vitest 全部通过；Vitest 为 242 个文件、1592 项测试。

GOFLAGS=-buildvcs=false make build
结果：通过；后端 server 构建成功，前端 production build 成功。
关键构建资源：index-DS6mjZ16.js、AdminWithdrawalsView-CbNuKKTf.js、WalletView-_NUtp3SO.js、zh-BX8JNAEW.js、en-BHi7F07V.js。

./scripts/check-fork-integrity.sh
结果：通过；新增提现 migration、withdrawal service、用户提现 route、管理员打款 route、step-up route、管理员提现页面和中英文提现管理 locale 保护。

git diff --check
结果：通过。
```

### 数据库 / API 对账

- 状态：已通过（本地自动化 + live API 保护范围）
- 迁移测试确认 CP5 提现系统表、状态约束、金额两位小数约束、当前收款账户唯一索引、ready 用户启用触发器和提现权益锁定表可创建。
- 账务测试确认提交提现时原子执行余额减少、可提现权益减少、提现冻结增加，并锁定具体成熟权益批次。
- 取消和拒绝测试确认提现冻结减少、余额与可提现权益恢复，并恢复原锁定权益批次。
- 审批测试确认 `$100` 以下进入打款待处理，达到 `$100` 进入二审；第二名不同管理员审核后进入打款待处理；管理员不能审核自己的提现。
- 打款测试确认仅 `payout_pending` 可标记 paid，重复打款被状态机阻断，线下打款保存实际金额、币种、汇率、外部流水号和时间。
- 生产 `GET /api/v1/user/wallet/withdrawals/availability` 未登录返回 `401 UNAUTHORIZED`，`GET /api/v1/user/wallet/withdrawals` 未登录返回 `401 UNAUTHORIZED`，`GET /api/v1/admin/withdrawals/settings` 未登录返回 `401 UNAUTHORIZED`。
- 生产 `POST /api/v1/admin/withdrawals/1/approve`、`POST /api/v1/admin/withdrawals/1/reject`、`POST /api/v1/admin/withdrawals/1/mark-paid` 未登录均返回 `401 UNAUTHORIZED`；当前无生产管理员凭据，未执行生产真实提现写入。
- 当前环境无生产 `DATABASE_URL`，未执行生产直连 DB invariant；线上功能证明以 Zeabur 成功、健康接口、受保护 API 路由和前端资产为准。

### 中文与英文检查

- 状态：已通过（本地自动化 + 生产资产范围）
- 用户钱包中文包含“申请提现、收款账户、提现记录、提现冻结、待解冻、任务预留”；英文包含 `Withdrawal request`、`Payout account`、`Withdrawal Frozen`、`Offline Payout` 等对应概念。
- 管理员中文包含“提现管理、提现规则、用户提现权限、提现队列、完整收款资料、线下打款”；英文包含 `Withdrawals`、`Withdrawal rules`、`User withdrawal permissions`、`Sensitive payout details`、`Offline Payout`。
- 前端 API 和页面对未知状态/方式/敏感字段使用当前 locale fallback，避免中文界面显示英文状态 key。
- 生产资产 `zh-BX8JNAEW.js` 包含“提现管理、申请提现、提现冻结、收款账户、线下打款”；`en-BHi7F07V.js` 包含 `Withdrawals`、`Withdrawal Frozen`、`Payout account`、`Offline Payout`。

### 审查发现与修复

- 敏感收款资料只保存 AES 加密正文和掩码；普通用户接口与管理员列表/详情均不返回完整资料，完整资料只走 step-up 后的 `payout-sensitive` 接口。
- `accountSnapshotEncrypted` 和 `accountEncrypted` 为 Go 未导出字段，不会被 JSON 序列化到普通响应。
- 新提现冻结使用 `withdrawal_frozen_balance`，没有复用图片任务 `frozen_balance`。
- 金额解析要求 HTTP 请求金额为字符串且提现金额必须两位小数；内部账务和数据库使用 decimal / `NUMERIC(20,8)`。
- 通知邮件失败在异步路径记录日志，不回滚提现状态机。
- 管理员审批、拒绝、读取完整收款资料和标记打款均走 step-up route；管理员 API Key 不满足 JWT step-up 上下文。

### 剩余风险

- 用户本地浏览器验收尚未完成，CP5 状态只能写“代码和部署已完成，等待本地浏览器验收”。
- 提现总开关仍默认关闭；仅在用户完成本地验收并选定白名单测试用户后，才能灰度开启。
- 当前未保存或使用生产管理员凭据，因此没有在生产执行真实提现申请、审批或打款写入。
- 生产 DB 直连 invariant 仍需要具备生产 `DATABASE_URL` 的环境执行；当前文档不保存凭据。

### 交付 commit

- 本地实现 commit：`476e1cb71552067f8eaa8f535846352be08c1f88`
- PR：#96
- 合并 commit：`00968312a3b842ac11966a9c9fba85c52dcbeb3c`
- GitHub PR checks：Protected behavior、Documentation、backend-security、frontend-security 均 success。
- merge commit checks：Zeabur、backend-security、frontend-security 均 success。
- 生产前端资源：`index-DS6mjZ16.js`、`AdminWithdrawalsView-CbNuKKTf.js`、`WalletView-_NUtp3SO.js`、`zh-BX8JNAEW.js`、`en-BHi7F07V.js`
- 生产 live proof：`/health` 返回 200 `{"status":"ok"}`；新增用户和管理员提现 API 未登录返回 401；上述 CP5 资产均返回 200。
