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
| `make test-frontend`（`make test` 的前端部分） | PASS；design、eslint、typecheck；489 文件 / 3476 测试（补充清空回归后） |
| `make test-backend-unit` | PASS，完整 unit tag 套件 |
| `make build` | PASS，完整后端与前端构建 |
| `./scripts/check-fork-integrity.sh` | PASS，含受保护行为检查 |
| 新核心迁移静态契约及 CI 同范围 PostgreSQL/Redis 集成 | PASS；含 273/274、迁移锁、升级/回滚、计费对账及仓库套件 |
| `node scripts/check-doc-links.mjs` / `git diff --check` | PASS |

上述检查均在对齐 `4080e2ac7e93dc9f435e0c0a4652834635cba7be` 后运行。由于仓库体量与资源限制，`make test` 的两个组成目标分别执行；后端首次目标中的 lint 资源失败已单独完整重跑成功，没有跳过检查或缩小 lint 范围。最终提交 SHA、draft PR 与 GitHub CI 结果由交付消息和 PR 记录，避免本文件自引用提交 SHA。

## 首轮 CI 后的明确清空边界复核

首个提交 `5eeabe0e9ddb55356c98b1508e7ba17d8304c294` 的五项 GitHub 检查全部通过。父线程随后指出：仅对规范化载荷判脏，会将“纯空白历史值 → 用户明确清空”误判为未修改。新增五项组件回归先得到 [RED：5 失败 / 14 通过](whitespace-red.log)，再增加原始表单输入快照；最终值相对原始输入变化，或规范化载荷变化，任一成立才发送该字段。未触碰的空白与旧接口展示默认值仍不回写，创建、额度 clear 及金额计算逻辑未变。

[补充 PostgreSQL 浏览器 GREEN](whitespace-green-postgres.log) 在完整迁移后的隔离 DB 中设置纯空白 `product_name`、`detail_description`、`storefront_badge`，通过真实管理页面清空，断言 PUT 恰好包含三个空字符串；随后 GET/DB 核对明确清空与全部未改字段。原 description-only、取消/重开、权限和非法输入合同也重新通过。

覆盖边界：组件用例还覆盖 cover URL 和保留原始数组的兼容 features 响应。既有列表对字符串 features 先 trim/filter；纯空白若已成为空展示，则空→空不构成可辨识的字段变更，仍保留 DB，不宣称该路径可从页面明确清空。此边界经独立规格复审确认，无需扩大第三个生产文件或引入触碰跟踪。补充修复已依次通过独立规格与质量复审，两位审查者独立复跑 19 项组件测试，质量审查另复跑真实 SQLite HTTP/DB 合同；均无未解决问题。补充改动的完整闸门已复跑通过，以普通追加提交交付，不改写首个提交。

补充复验期间，既有 `TestTokenRefreshService_SaturatedProviderPreservesConcurrencyAndActualQPSStartSpacing` 在并行负载下单次断言失败（150.557µs < 5ms），区别于此前 lint 的资源终止。本次未修改该测试或运行时逻辑，隔离连续 20 次通过；随后串行执行完整后端默认 tests + 完整 lint（0 issues）+ unit tests，退出码 0。GC/并发限额仅用于 linter，测试范围和断言不放宽。执行环境重启时，前端 489 文件/3476 测试及完整构建已有完成日志，Fork 检查未完成，故完整重跑该检查，最终退出码 0、Fork integrity passed。

## 2026-10-10 主线兼容合并

根线程已将 #342 普通合并到 `play/main` `9e458b12db91d1c36402c288d61ef7a352beee1c`（父提交 `4080e2ac7` / `e58642e42`）。本分支从 `bf28ee393482866f04f771829e20fba06fdc25d8` 通过普通 merge 对齐，无冲突、无 rebase/force。#341 的生产文件和套餐测试与合并前保持字节一致；#342 的十个文件与主线保持一致，仅追加本交付记录及验证日志。独立规格兼容复核后，独立质量兼容复核也通过，无阻断项。

合并后的 [PostgreSQL 双浏览器与 HTTP/DB 合同](merge-342-green-postgres.log) 全部通过。新签到专用脚本执行真实 PostgreSQL/Redis、HTTP 与前端：6 个子用例及两个实时前端用例通过，无跳过；视频媒体迁移定向测试通过，未调整断言。完整后端 default/unit 和 lint 已通过（退出码 0，lint 0 issues）。完整前端门禁通过：489 个文件/3476 测试通过，默认跳过的两个实时签到测试由上述专用脚本实际执行通过。完整 make build、Fork integrity、文档链接与 diff 检查均通过；前端→构建→Fork 串行命令退出码 0。所有检查针对本次新组合运行，未放宽断言，未重复旧 SHA 的 CI。

此处只更新 #341 审查分支，最终确切 merge SHA 与该 SHA 的所有 CI 由 PR 及交付消息记录；根线程统一执行生产合并与部署。

## 发布边界

本任务不执行合并或部署。PR #340 在此工作期间的已知状态为已合并、Zeabur 部署/验收由主线程监控，未将其写为部署成功。本 PR 同样等待主线程统一合并部署及用户本地电脑最终验收。无生产写入，无真实用户改价，无支付/购买调用。
