# 鹈鹕旧题干迁移：PR #411 后首项交付

状态：已部署，用户本机验收待完成。维护责任：本任务提交者；长期负责人待分配。

## 范围与来源

- 稳定 ID：`pelican-prompt-349`。生产开发基线 `c302cce4f07b071417d0f42320cc4dde7428c7e9`（PR #411）。
- 分支 `fix/pelican-prompt-restriction-migration-20261106`，独立 worktree；不修改玩法福利，也不合入其他上游差异。
- 固定来源 `ranxi2001/sub2api v2.19.3`，tag object `dbb7f34a64504a91441b9f38bc13fa432fef04bc`，commit `6f813426b61b8eeba77f374059205cc34d2fa993`。2026-11-06 在线 GitHub stable release/tag 解引用核对一致。该提交静态 VERSION 为 2.19.2，构建 resolver 优先 exact release tag 得到 2.19.3；不改 Fork 产品版本。
- 上游原变更 [PR #293](https://github.com/ranxi2001/sub2api/pull/293)，merge `5eb65c087322845fc228b65e34f8b93e78930264`，`263_strip_pelican_prompt_restriction.sql`。
- Fork 使用增量 `349_strip_pelican_prompt_restriction.sql`；SQL 操作与上游一致，仅增加本地事务锁等待 2 秒、语句 30 秒限制及说明。历史 SQL 字节不变。
- 全版来源锁仍为历史 v2.17.7 且 `fully_migrated=false`；本记录只证明此单项，后续完整差异清单会独立更新分析基线。

## 行为与使用

应用启动自动迁移三张配置表：`pelican_group_test_plans`、`scheduled_test_plans`、`quality_rule_templates`。清除题干中的 `不要有任何限制` 及紧邻中文逗号，修剪两端空格；清理后为空时保留原文。其他 JSON 配置、启用状态、模型、计划与归属不变；被修改记录更新 `updated_at`，使旧计划快照失效。

管理员继续使用原鹈鹕/质量页面，无新增导航、权限、API 或手工 SQL 步骤。前端默认题干和后端默认题干当前均不含该句，已有历史作答快照保留当时发出的题干。不额外清洗上游未覆盖的预设、自定义正文或历史结果。

## 验证与审查

- RED：隔离 PostgreSQL 18.1 先确认旧测试错用上游 253/261 文件名；修正为实际 344/345 并补齐 344 已保存的 `model_id` 断言后，明确失败于缺失 `349_strip_pelican_prompt_restriction.sql`。
- GREEN：增加 349 后，真实 PostgreSQL 18.1 / Redis 8.4 回归验证三表清理、空题干保护、其他质量配置保留、两种历史结果快照不变、重复执行，以及鹈鹕计划租约、历史、费用保留和展厅读取。5 项专项全部通过。
- 首次默认 testcontainers 的 Ryuk 镜像拉取因 Docker Hub 超时失败；复用仓库受限 external test 模式，手动启动本机已有的精确版本容器，仅绑定 loopback 非标准端口、独立测试用户和数据库。生产 DSN 未复制到本地。
- 既有测试保留全部断言，只修正 Fork 迁移映射及 `model_id` 期望；扩展同一回归覆盖实际清洗行为。CI 核心 PostgreSQL job 新增该组测试及必执行断言，防止仅编译而未执行。
- 规格审查：三表范围、历史保护、空值边界和新默认题干对齐通过；未将目录差异当作整版完成。
- 代码质量审查：固定表名、JSONB 字段更新、幂等、事务/超时及旧读取者兼容通过；迁移失败会回滚，本项不修改运行时授权。
- `make test` 退出 0：Go 默认全量测试、golangci-lint 0 issues、前端 design/lint/typecheck 与 540 文件、4,179 项通过（3 文件/8 环境条件项跳过）。
- `make test-backend-unit`、`make build`、`./scripts/check-fork-integrity.sh` 全部退出 0；文档链接检查 334 文件通过。PR #412 最终 head `1201ad615b4b1ebe1e258ccd9015e05918642bf8` 的全部 9 项 CI 通过。

## 生产预检与回退

2026-11-06 只读事务已显式 ROLLBACK：最高迁移 348；鹈鹕计划 3 条，其中符合清洗条件 0、清洗后空题干 0；账号定时计划和质量模板各 0 条。本次迁移在该采样时无需修改现有配置，仍补齐新建/升级数据库的完整链路。不读取或输出实际题干、凭据、用户标识。

迁移由既有启动 runner 在事务中执行，记录完整文件名/checksum。部署后核对精确 SHA、349 ledger/checksum、同一聚合和运行日志；遇到锁超时/启动失败先停止发布并核查，不手动改历史 SQL 或账本。

代码 revert 不会恢复被移除的句子；如未来确需还原个别自定义题干，应由管理员根据原记录核实后单独修改，不能给全部计划盲目追加旧句子。本次不改变历史结果、金额或已部署迁移。

生产部署：PR #412 已于 2026-11-06 07:03:34 UTC 以 merge commit `e1626d760cf92349bb68fee5c6537a66e1dcbdc6` 合入 `play/main`；Zeabur `6aed7bc803a5d91147655138` SHA 一致，07:09:16 UTC 构建完成，状态 RUNNING。用户本机游客/普通用户/管理员验收：待完成，服务器测试不替代该项。

## 发布后核验

2026-11-06 07:11–07:14 UTC：`schema_migrations` 为 442 条，最高迁移 349；349 的 trimmed SHA-256 为 `ab2c0fe0819360c3b1387effb23a3d494152bd512126835b458354fe2f7edab6`，与源码一致。鹈鹕计划/账号定时计划/质量模板总数分别 3/0/0，可清理行数均 0；只读事务显式 ROLLBACK。

首页和 `/health` 200；未认证访问 `/api/v1/admin/pelican-group-tests`、`/api/v1/admin/pelican-group-test-results`、`/api/v1/admin/account-quality-templates` 和 `/api/v1/pelican-showcase` 均 401。最近 100 条日志采样窗口 07:10:25–07:12:05 UTC 无 ERROR/WARN/FATAL/PANIC；这只证明该窗口，不代表长期流量或生产管理员写动作。首次误用不存在的 `/admin/pelican-tests/plans` 得到 404，已依据路由代码纠正；Python urllib 的全部请求返回 403，curl 同一域名正常，客户端差异保留，不把 403 当作应用权限验收。

此项技术部署已核验。游客/普通用户/管理员本机页面验证仍待用户完成；不替代或伪造该证据。发布后记录随独立文档 PR #413 留存，不为文档更新单独部署生产。
