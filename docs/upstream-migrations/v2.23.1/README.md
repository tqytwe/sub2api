# v2.23.1 上游同步账本

`deployed_partial`，`fully_migrated=false`。固定累计范围 107 个提交已全部完成逐 SHA 处置（87 项 `equivalent`、20 项 `migrated`），0 项开放。整版代码由 [PR #460](https://github.com/tqytwe/sub2api/pull/460) 的合并 SHA `4ba7e6a64c44a2f676fd3ff5fff56322f4a8fd24` 部署到生产，游客验收已通过；生产数据对账、普通用户和管理员登录态验收仍待证据，因此后台继续如实显示未完全迁移。固定跟踪项、责任角色和放行条件见[项目清单](./DEVELOPMENT_CHECKLIST.md)和[生产验收记录](./PRODUCTION_ACCEPTANCE_2026-11-27.md)。

## 来源与生产基线

- ranxi 正式版 `v2.23.1`。Annotated tag `3297a4f4547076ed5c5b21e0471a12490985fb80`，源码 `00e00796080c632188cf9df7912050b144a1a44f`，版本文件 `2.23.1`。
- 初始 Fork 基线为 `play/main@9c24cd75d2369de5e65be46fbab1945c2f43f696`。
- 首批 [PR #452](https://github.com/tqytwe/sub2api/pull/452) 合并至 `99db0e5f7b4db87d02c581be8bef39bfc6ad9388`，Zeabur 部署 `7943807040` 于 2026-11-20 成功。随后 [PR #453](https://github.com/tqytwe/sub2api/pull/453) 的 `db2a21a268e09cb734e4bf60612052d0a2e30b86` 部署 `7946533971` 成功，包含首批改动。
- [PR #455](https://github.com/tqytwe/sub2api/pull/455) 通过 10 项 CI 并合入 `f3c4e62fcc0919b4f3b46a4fc821e30832c91b48`。Zeabur 部署 `7958153329` 成功。`/health` 返回 200，匿名访问管理员版本查询接口返回 401。弹窗显示 Fork 构建号、ranxi 正式版本、源码目标及同步状态。GitHub 查询失败时仍显示已知目标和状态。
- [PR #456](https://github.com/tqytwe/sub2api/pull/456) 通过 9 项 CI 并合入 `3131b73c689f007964e23ffbfd4de6f0e65ab16d`。Zeabur 部署 `7958851461` 于 2026-11-21 13:03:46（北京时间）成功，`/health` 返回 200。

## 本批合并与部署

- [PR #460](https://github.com/tqytwe/sub2api/pull/460) 于 2026-11-26 21:23:20 UTC 合入 `play/main`，合并 SHA 为 `4ba7e6a64c44a2f676fd3ff5fff56322f4a8fd24`；PR 所有必需 CI 检查通过。
- Zeabur 于 2026-11-26 21:28:03 UTC 对同一 SHA 部署成功。GitHub production deployment ID：`8096214204`；Zeabur deployment ID：`6b08a34b9a6efec056d2e9a8`。GitHub 部署记录的 SHA 与 PR 合并 SHA 完全一致。
- 2026-11-27 生产探测：`https://www.jisudeng.com/health` 返回 HTTP 200、`{"status":"ok"}`；匿名 `GET /api/v1/admin/system/check-updates` 返回预期 HTTP 401 `UNAUTHORIZED`。无 Cookie 浏览器已完成游客页面及未登录权限边界验收，详见 [生产验收记录](./PRODUCTION_ACCEPTANCE_2026-11-27.md)。
- 尚未关闭的 G5 项：生产确认迁移 `431_provider_profit_links.sql` 已应用，并对供应商链接/历史成本数据完成只读对账；普通用户和管理员登录态浏览器验收。完成前后台保持 `fully_migrated=false`。

## v2.23.0 至 v2.23.1 的逐项处置

比较范围包含 [18 个 first-parent 提交](v2.23.0-first-parent.tsv)、248 个变更文件、1 个新增 SQL。表中“已纳入”指对应行为在列出的 Fork 实现中有证据。表中未核完的事项继续保留为未完成。

| 上游 | 处置 | Fork 行为和待验证范围 |
|---|---|---|
| #935 `a71bcd623` | migrated | 余额通知静默期，见 `account_ops_balance_notification_test.go`。 |
| #934 `270ae1710` | migrated | 默认关闭/环境配置、HTTP 账号池重试、三种 WS 模式双轮续接、owner 不写入和 no-replay 控制已完成；证据见 `dispositions.json` 与对应差异审计。 |
| #936 `45ebcb708` | migrated | New API 钱包查询的 UA、授权和费用配置，见 `new_api_upstream_user_agent_test.go` 及 `new_api_cost_test.go`。 |
| #937 `feb761541` | migrated | Grok 探测冷却状态持久化；结果入库失败保留原计划，历史清理失败继续推进，见 `scheduled_test_durable_result_test.go`。 |
| #939 `afcef2d68` | migrated | 生图实际产出计数、分组策略、映射模型价格、各入口和结算回执均已覆盖；生产数据验收另见 G5。 |
| #942 `6ed27208b` | migrated | `pagedExport.ts`、`UsageView.vue` 和 `client.ts` 覆盖 429 重试、分页大小和取消。 |
| #943 `587ccb28e` | migrated | 交互预览启用 pointer lock，静态预览继续隔离，见 `PelicanArtworkPreview.spec.ts`。 |
| #940 `009eefc27` | migrated | 安全压缩、区域传输、Codex/WS 准入、明文回退边界及仅发送一次的回归已完成。 |
| #945 `8f4e0d2b2` | migrated | 钱包币种与换算，见 `new_api_wallet_test.go`。 |
| #944 `5a5199711` | migrated | 账号统计展示缓存和查询侧减负，见 `account_performance_display_cache_test.go`。 |
| #948 `72e4545a0` | migrated | `Select.vue` 的 `matchTriggerWidth` 用于 `KeysView.vue`，见 `Select.spec.ts`。 |
| #949 `4f1bfa96d` | migrated | 供应商成本模式、钱包窗口快照、扣费证据、历史归属和按窗口报表已完成代码及集成测试；生产历史数据对账仍属 G5。 |
| #952 `d846d9e59` | migrated | MiniMax H3 模型别名，见 `minimax_h3_test.go`。 |
| #950 `48dee26a7` | migrated | 同步/异步结算、SSE/JSON 回执、映射模型定价和零产出解释均有回归覆盖。 |
| #951 `b107d07da` | migrated | HTTP/WS 恢复控制、重建拒绝、脱敏运维诊断和安全字段防泄露已完成。 |
| #955 `7563b8869` | migrated | 手工/导入归属历史、时段边界、冻结扣费身份和历史报表联接已有 PostgreSQL 集成覆盖。 |
| #953 `3b79c7d9a` | migrated | `/v1/videos` 创建、状态、下载及完成结算，见 `openai_videos_test.go` 和路由测试。 |
| #956 `00e007960` | equivalent | 源码版本由锁定 tag 和 commit 确认，Fork 构建号独立发布。 |

## 自 v2.21.2 累计差异和迁移依赖

此范围为 [107 个 first-parent 提交](v2.21.2-first-parent.tsv)、1,100 个可达提交和 2,185 个变更文件。逐 SHA 审查现已全部结束：87 项 `equivalent`、20 项 `migrated`、0 项开放。初始 [整树报告](release-check.json) 是历史比对快照，不代表当前仍有未审查代码差异。剩余工作是 G5 所列生产数据和产品验收，不是已知代码差异。

| 上游迁移或提交 | 依赖与 Fork 现状 |
|---|---|
| `345`–`348`，`236200c67` | Fork 迁移 `427`–`430` 和关联管理端能力已合入；凭证不自动导入。生产环境中现有供应商/凭证配置由授权管理员确认。 |
| `383`–`388`，`1c345720d`、`4f1bfa96d` | 供应商渠道归属、成本模式、扣费证据、钱包快照和报表运行时已纳入；Fork 迁移 `431_provider_profit_links.sql` 为既有供应商创建明确链接。生产需核对迁移版本、链接行数和历史成本归属，见 G5-DATA。 |
| `7563b8869` | 手工及导入归属时间线、时段边界和历史报表联接已由 PostgreSQL 集成测试覆盖；生产历史数据只读对账仍待 G5-DATA。 |

供应商目录基于编号至 `426` 的生产分支；`427`–`431` 为本 Fork 新增迁移，不会自动导入账号凭证。后续新增迁移须重新检查远端编号及真实数据库结构，保留既有迁移校验和。

`427`–`430` 与上游原始迁移逐字节一致，仅变更文件编号；`431` 增加 Fork 供应商利润归属。协议代码的 [LGPL 来源记录](mature-provider-provenance.json) 保留在本账本目录。PostgreSQL 定向 CI 覆盖迁移重放、目录版本冲突、渠道隔离、凭证加密、Key 幂等和自动刷新；生产 `431` 应用及历史数据核对尚未取得证据。

## 验收状态

- 2026-11-27：PR #460 已合并，精确 SHA `4ba7e6a64c44a2f676fd3ff5fff56322f4a8fd24` 的 Zeabur 部署成功（deployment `6b08a34b9a6efec056d2e9a8`）。生产 `/health` 为 HTTP 200；匿名管理员版本接口为预期 401；无 Cookie 游客页面及未登录权限边界验收通过。后台目标代码为 `2.23.1`、`fully_migrated=false`。生产迁移/历史数据核对、普通用户和管理员验收仍 pending；不得据部署成功或游客验收提前关闭整版。
- 2026-11-27：累计 107 项全部达到终态，`python3 scripts/check_upstream_dispositions.py --verify-git --require-merge-ready` 通过；PR 必需 CI、后端 default/unit 全量测试、前端 design governance 和生产构建均通过。代码差异关闭，生产验收仍由 G5 跟踪。

- 2026-11-26：完成 #836 历史平台筛选逐文件审查与处置，补强 15 种 provider 的 handler 传播测试，并将 PostgreSQL 历史/插入/index 与迁移 schema 回归纳入 core CI。全量前端 639 文件通过（3 跳过，5,077 测试通过、40 跳过），生产构建及 disposition 单测通过；定向 PostgreSQL 用例通过。累计 107 项现为 13 项终态、94 项开放（85 未审查、8 部分、1 阻塞）。同步 PR 完成门禁已加入候选 CI，但需随完整同步一同交付；开放项目标不得合并/部署。

- 2026-11-21 复核 GitHub 最新正式版、tag 对象及源码提交，验证固定版本来源。
- 管理员弹窗视觉证据见 [管理员版本信息](../../visual-reviews/2026-11-21-v2231-admin-version.md)；生产游客验收见 [2026-11-27 记录](./PRODUCTION_ACCEPTANCE_2026-11-27.md)。普通用户和管理员生产登录态验收待记录。
- 代码层累计差异处置已完成并部署；整版仍取决于 G5-DATA 的供应商历史数据对账，以及 G5-USER/G5-ADMIN 生产产品验收。

- 2026-11-26：完成 #837 上游差异审查并适配账号组成员选择器与保存/重开并发保护；修复浏览器验收中发现的中英文未解析质量运维文案，补齐视觉记录与 fork-integrity PR 浏览器回归 job。合成数据 Playwright 交互/多视口检查通过；定向组件和 locale 13 项通过。全量前端 641 个测试文件通过、3 个跳过；5,091 项通过、40 项跳过；前端 lint、typecheck、生产构建均通过；后端 `go test -count=1 ./...` 与 `go test -tags=unit ./...` 通过；`./scripts/check-fork-integrity.sh`、文档链接、YAML 与账本校验通过。候选累计状态现为 13 项终态、94 项开放（85 未审查、8 部分、1 阻塞）；全量交付与生产部署仍未完成。
- 2026-11-26：继续核对 #934 并修复 handler 测试对 service 私有字段的跨包访问；补三种 WebSocket 模式双轮兼容续接回归。配置、HTTP/WS 定向测试和前端完整测试/构建通过。#934 仍保持 `partial`，完整累计同步与部署未完成。
- 2026-11-26：#835 的 HTTP/Redis、精确及包装错误恢复回归、发送保护、设置 API 契约、运维诊断脱敏和合成数据浏览器验证已通过。原生三次发送及 HTTP 200 流内排除仍待补充，保留 `partial`。旧版仅改 VERSION 的 `ccf997c73` 按独立上游来源锁记为 `equivalent`。合并与生产验收分为两道门禁。累计 14 项终态、93 项开放，未创建候选 PR、未部署。
- 2026-11-26：#835 已补原生三次发送与 HTTP 200 流内排除回归，并修复代理变化、不可读取/非法出站请求体仍可重试的问题，候选标记 `migrated`。#839 开始适配明确周窗口和重复头判定，保留 `partial`。累计 15 项终态、92 项开放；未创建候选 PR、未部署。

- 2026-11-26：#839 完成候选处置。修复快照节流、同账号 429 重试及 BPS 旁路漏记较新无效周证据的问题，保留原临时冷却行为；定向 unit 与隔离 PostgreSQL 18.1 / Redis 8.4 回归通过。全量默认测试发现 #835 候选阻断既有精确 compaction 恢复，已恢复**唯一可识别密文**的兼容路径，仍禁止通用回退删除 compaction；定向回归及修正后全量默认 Go 通过。累计 16 项终态、91 项开放（82 未审查、8 部分、1 阻塞），未创建候选 PR、未部署。

- 2026-11-26：#847 完成候选处置。BPS 校验但不再发送 `service_tier`，避免上游 422；流式/非流式、全局/分组强制 Fast、重建/工具修正、实际标准费率扣费及原生 OpenAI Fast 定向回归通过。#847 变更前一次完整 Go default/unit、前端 5,092 项测试、前端生产构建及 Fork integrity 均通过；最终候选仍需完整发布门禁。累计 17 项终态、90 项开放（81 未审查、8 部分、1 阻塞），未创建候选 PR、未部署。
