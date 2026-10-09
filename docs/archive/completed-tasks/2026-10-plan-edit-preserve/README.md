# 套餐编辑配置保留：交付记录

## 范围与基线

- 原始生产基线：`a7e1c00330170754683216e3018cbd4279d7f733`。
- 工作分支：`codex/plan-edit-preserve-20261009`，独立 worktree。
- 工作期间 PR #340 合入；通过普通 fast-forward merge 对齐实际 `play/main` `4080e2ac7e93dc9f435e0c0a4652834635cba7be`，没有 rebase/force，也未操作 #340 原分支。
- 对应 Fork 边界：FORK-BILLING-010 / FORK-REWARDS-015；只修复套餐管理数据合同，不改变定价、付款、额度或结算语义。
- `.agents`/`.codex` 目录为空；AGENTS 引用的 `docs/PROJECT_HYGIENE.md` 在该基线不存在，已遵循 AGENTS 的归档要求和现有 DELIVERY_WORKFLOW。

## RED 与最小修复

[完整迁移 PostgreSQL + 实际浏览器 RED](red-postgres.log)、[前端 RED](red-frontend.log)。归档日志仅将制表符展开、移除行尾空白，保留断言与结果；带引号的字段值保持原样。测试初始套餐的全部持久化字段均填充非默认值，价格为隔离测试数据 19.99；浏览器只改描述，不触发购买/支付。

实际 GET 投影遗漏 `cover_image_url`、`detail_description`、`storefront_platform`、`storefront_category`、`storefront_featured`、`storefront_badge`。旧弹窗把缺值转换为空/false/推断货架后全量 PUT，真实数据库丢失六项配置。浏览器还证明 `product_name`、`features` 的空白被未授权编辑意图之外的 trim 改写。

逐项核对 `subscriptionplan.Columns` 及响应类型：全部 24 个持久化列均已覆盖，增加自动覆盖检查；另补齐同一 DTO 的 4 个只读分组字段 `peak_rate_enabled`、`peak_start`、`peak_end`、`peak_rate_multiplier`。既有币种、原价、3 个套餐额度、有效期、排序、上下架、时间戳及其余分组元数据未删改。

修复仅两处生产文件：列表投影补字段；弹窗记录初始化后的载荷基线，更新仅发送实际差异，初始化期间不触发分组联动推断。缺字段旧接口的默认展示值不会回写；数据库已有明确空货架值保留为空。服务现有 PUT patch 语义不变：省略保留，显式 `""`/false/0 更新，3 个 clear flag 明确清除额度。价格仍必须大于 0，原价可为 0，额度必须正数或明确清除。

## 真实链路与重复运行

[GREEN PostgreSQL 记录](green-postgres.log)：实际管理页面、实际弹窗、实际 Axios client、实际 Gin handler/service、实际 PostgreSQL；仅移除无关全局导航。测试 GET 后进入编辑，取消无 PUT，重开恢复原值，仅修改 description，PUT 后再次 GET 并直接从 DB 读取，对账所有未改字段（仅 updated_at 允许变化）。独立 HTTP 契约还覆盖旧全量客户端回传、显式清空/false/0、遗漏保留、非法输入原子拒绝、真实游客401/普通用户403。

默认 `go test ./internal/planacceptance` 使用真实临时 SQLite，保证常规 CI 覆盖 HTTP/服务/DB；浏览器 opt-in 测试明确跳过，需以下命令显式运行。PostgreSQL 模式每用例创建随机命名空库，调用 `repository.ApplyMigrations` 执行当前完整迁移链（含273/274），不以 Ent schema 代替。测试角色/JWT 随机生成且仅通过进程环境传递。

在独立开发机安装仓库锁定依赖、Go 1.27.2、Playwright Chromium；例如将 Playwright 安装于外部临时 npm 目录，设置 `PLAN_PLAYWRIGHT_PACKAGE` 指向该目录的 package.json。启动仅供本测试的 loopback PostgreSQL 容器，使用 `sub2api_test` 用户及 `sub2api_plan_edit_test` 控制库。测试不接受应用环境连接串。

```bash
# 所有变量指向一次性测试资源；不使用任何生产凭据。
export PLAN_CONTRACT_POSTGRES_DSN='postgres://sub2api_test@127.0.0.1:55433/sub2api_plan_edit_test?sslmode=disable'
export PLAN_BROWSER_CONTRACT=1
# 需要外置 Playwright 时设置 PLAN_PLAYWRIGHT_PACKAGE；自定义浏览器缓存用 PLAYWRIGHT_BROWSERS_PATH。
cd backend
go test ./internal/planacceptance -run '^TestAdminPlan' -v -count=1
```

`PLAN_CONTRACT_BASELINE=1` 仅用于修复前 RED 取证；修复后运行必须不设置。`PLAN_CONTRACT_EVIDENCE` 可将截图输出到外部临时目录。

## 审查与验证

独立规格审查通过后，再执行独立质量审查；均无阻断问题。质量审查建议补充“关闭 A 后打开不同分组 B”的回归，已增加并由质量审查者独立复跑通过；B 的明确空货架值和仅发送描述的载荷均得到验证。

| 闸门 | 结果 |
| --- | --- |
| 真实 PostgreSQL 全迁移 + 浏览器/HTTP/DB 契约 | PASS；取消 0 PUT，描述修改仅 1 PUT，GET/DB 未改字段一致 |
| `make test` 的后端部分：完整 `go test ./...` | PASS |
| 完整 `golangci-lint run ./...` | PASS，0 issues；首次与全量测试并行时进程被资源终止，限流至 concurrency=2 / GOMEMLIMIT=3GiB 后完整重跑通过 |
| `make test-frontend`（`make test` 的前端部分） | PASS；design、eslint、typecheck；489 文件 / 3471 测试 |
| `make test-backend-unit` | PASS，完整 unit tag 套件 |
| `make build` | PASS，完整后端与前端构建 |
| `./scripts/check-fork-integrity.sh` | PASS，含受保护行为检查 |
| 新核心迁移静态契约及 CI 同范围 PostgreSQL/Redis 集成 | PASS；含 273/274、迁移锁、升级/回滚、计费对账及仓库套件 |
| `node scripts/check-doc-links.mjs` / `git diff --check` | PASS |

上述检查均在对齐 `4080e2ac7e93dc9f435e0c0a4652834635cba7be` 后运行。由于仓库体量与资源限制，`make test` 的两个组成目标分别执行；后端首次目标中的 lint 资源失败已单独完整重跑成功，没有跳过检查或缩小 lint 范围。最终提交 SHA、draft PR 与 GitHub CI 结果由交付消息和 PR 记录，避免本文件自引用提交 SHA。

## 发布边界

本任务不执行合并或部署。PR #340 在此工作期间的已知状态为已合并、Zeabur 部署/验收由主线程监控，未将其写为部署成功。本 PR 同样等待主线程统一合并部署及用户本地电脑最终验收。无生产写入，无真实用户改价，无支付/购买调用。
