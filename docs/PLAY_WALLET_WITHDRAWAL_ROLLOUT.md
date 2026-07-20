# Play、钱包与提现交付检查点

> 状态：active
> 当前阶段：阶段一 / CP1
> 当前结论：进行中（服务器验证已通过，等待 PR、CI、部署和本地浏览器验收）
> 生产基线：`origin/play/main@633cf432fad6e266f477e128f9608950591959e1`
> 审查分支：`feat/play-wallet-withdrawal-20260720`
> 工作树：`/home/dell/worktrees/sub2api-play-wallet-withdrawal-20260720`
> 最后更新：2026-07-20

本文档只记录已经取得证据的状态。允许状态仅为：
`未开始 / 进行中 / 已通过 / 失败 / 阻塞 / 等待本地验收`。
阶段一部署并完成本地浏览器验收前，阶段二至五不得开始。

## 总览

| 阶段或检查点 | 状态 | 当前门禁 |
|---|---|---|
| 阶段一：管理员手动修复战队成员 / CP1 | 进行中 | PR、GitHub CI、部署和本地浏览器验收 |
| 阶段二：战队奖励与余额透明度 / CP2 | 未开始 | 依赖 CP1 部署并通过本地浏览器验收 |
| 阶段三：代币农场最近发放与当前预估 / CP3 | 未开始 | 依赖 CP2 |
| 阶段四：可提现权益账务基础 / CP4 | 未开始 | 依赖 CP3 |
| 阶段五：提现权限、申请、审核与线下打款 / CP5 | 未开始 | 依赖 CP4，且总开关保持关闭直至灰度条件满足 |
| CP6：服务器验证 | 进行中 | CP1 已通过；CP2 至 CP5 尚未开始 |
| CP7：审查与合并 | 未开始 | 等待 CP1 服务器验证全部通过 |
| CP8：部署证明 | 未开始 | 等待 CP1 非 rebase 合入 `play/main` |
| CP9：最终本地浏览器验收 | 未开始 | 由用户本地电脑完成，不能由服务器测试替代 |

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
| PR / GitHub CI | 未开始 | 尚未提交、推送或创建 PR |
| 部署核对 | 未开始 | 尚未合入 `play/main` |
| 本地浏览器验收 | 未开始 | 部署后由用户以管理员身份检查中英文及浅色/深色 |

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

- PR CI、生产部署和本地浏览器验收尚未执行。
- 本地浏览器验收前，CP1 不能标记为“已通过”，阶段二不能开始。

### 交付 commit

- 状态：进行中
- CP1 实现 commit：`89561e39a`（`feat(play): add controlled admin team member repair`）
