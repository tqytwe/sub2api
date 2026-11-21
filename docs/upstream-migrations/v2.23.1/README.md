# v2.23.1 上游同步账本

`deployed_partial`，`fully_migrated=false`。首批已部署，其余差异按下表跟进。

## 来源与生产基线

- ranxi 正式版 `v2.23.1`。Annotated tag `3297a4f4547076ed5c5b21e0471a12490985fb80`，源码 `00e00796080c632188cf9df7912050b144a1a44f`，版本文件 `2.23.1`。
- 初始 Fork 基线为 `play/main@9c24cd75d2369de5e65be46fbab1945c2f43f696`。
- 首批 [PR #452](https://github.com/tqytwe/sub2api/pull/452) 合并至 `99db0e5f7b4db87d02c581be8bef39bfc6ad9388`，Zeabur 部署 `7943807040` 于 2026-11-20 成功。随后 [PR #453](https://github.com/tqytwe/sub2api/pull/453) 的 `db2a21a268e09cb734e4bf60612052d0a2e30b86` 部署 `7946533971` 成功，包含首批改动。
- [PR #455](https://github.com/tqytwe/sub2api/pull/455) 通过 10 项 CI 并合入 `f3c4e62fcc0919b4f3b46a4fc821e30832c91b48`。Zeabur 部署 `7958153329` 成功。`/health` 返回 200，匿名访问管理员版本查询接口返回 401。弹窗显示 Fork 构建号、ranxi 正式版本、源码目标及同步状态。GitHub 查询失败时仍显示已知目标和状态。

## v2.23.0 至 v2.23.1 的逐项处置

比较范围包含 [18 个 first-parent 提交](v2.23.0-first-parent.tsv)、248 个变更文件、1 个新增 SQL。表中“已纳入”指对应行为在列出的 Fork 实现中有证据。表中未核完的事项继续保留为未完成。

| 上游 | 处置 | Fork 行为和待验证范围 |
|---|---|---|
| #935 `a71bcd623` | 首批已纳入 | 余额通知静默期，见 `account_ops_balance_notification_test.go`。 |
| #934 `270ae1710` | 部分纳入 | 本批覆盖 WS 生图工具参数数值精度。启动兼容模式和 HTTP、WS 恢复策略仍待适配。 |
| #936 `45ebcb708` | 首批已纳入 | New API 钱包查询的 UA、授权和费用配置，见 `new_api_upstream_user_agent_test.go` 及 `new_api_cost_test.go`。 |
| #937 `feb761541` | 本批补齐结果写入行为 | 首批包含 Grok 探测冷却状态持久化。结果入库失败保留原计划，历史清理失败继续推进，见 `scheduled_test_durable_result_test.go`。 |
| #939 `afcef2d68` | 部分纳入 | 有生图计费保底及 `X-Image-Usage: billing` 回执。分组开关、客户费用明细、价格 UI 和入口覆盖待逐项对照。 |
| #942 `6ed27208b` | 首批已纳入 | `pagedExport.ts`、`UsageView.vue` 和 `client.ts` 覆盖 429 重试、分页大小和取消。 |
| #943 `587ccb28e` | 首批已纳入 | 交互预览启用 pointer lock，静态预览继续隔离，见 `PelicanArtworkPreview.spec.ts`。 |
| #940 `009eefc27` | 部分纳入 | `upstream_request_compression.go` 和 `http_upstream.go` 有安全压缩实现。区域和 WS 回退行为待逐项对照。 |
| #945 `8f4e0d2b2` | 首批已纳入 | 钱包币种与换算，见 `new_api_wallet_test.go`。 |
| #944 `5a5199711` | 首批已纳入 | 账号统计展示缓存和查询侧减负，见 `account_performance_display_cache_test.go`。 |
| #948 `72e4545a0` | 首批已纳入 | `Select.vue` 的 `matchTriggerWidth` 用于 `KeysView.vue`，见 `Select.spec.ts`。 |
| #949 `4f1bfa96d` | 待适配 | 统一渠道配置、供应商成本模式与报表复用依赖历史证据和迁移，见下节。 |
| #952 `d846d9e59` | 首批已纳入 | MiniMax H3 模型别名，见 `minimax_h3_test.go`。 |
| #950 `48dee26a7` | 部分纳入 | API Key 非流式图片返回空数组时，结算回执、保底与按张定价均按实际产出计数。异步恢复、用量费用和映射模型价格仍需对照。 |
| #951 `b107d07da` | 部分纳入 | Fork 的 HTTP 加密历史省略路径冻结工具、指令和会话状态。发送前验证指定字段，日志只记录字段是否变化。上游投影层、WS 重建和运维事件接口仍待适配。 |
| #955 `7563b8869` | 待适配 | 报表归属依赖 #949 前置数据模型，见下节。 |
| #953 `3b79c7d9a` | 首批已纳入 | `/v1/videos` 创建、状态、下载及完成结算，见 `openai_videos_test.go` 和路由测试。 |
| #956 `00e007960` | 来源锁已记录 | 源码版本由锁定 tag 和 commit 确认，Fork 构建号独立发布。 |

## 自 v2.21.2 累计差异和迁移依赖

此范围另有 [107 个 first-parent 提交](v2.21.2-first-parent.tsv)、1,100 个可达提交和 2,185 个变更文件，仍需核对旧批次等效实现以及 QQ Bot 等缺项。初始 [整树报告](release-check.json) 比对 Fork 分析基线与上游目标，发现 172 个仅上游存在的迁移、187 个仅 Fork 存在的迁移、2 个同名内容变更和 113 个编号冲突。

| 上游迁移或提交 | 依赖与 Fork 现状 |
|---|---|
| `345`–`348`，`236200c67` | 供应商主体、加密凭证、Key 操作与刷新。Fork 的渠道绑定与钱包探测不提供相同的供应商对象和账单关系。 |
| `383`–`387`，`1c345720d` | 供应商渠道绑定与归属历史、供应商扣费凭证、钱包借记证据、同步日志和钱包快照。需与 Fork 原有 `profit_operations.go` 的计费口径及生产历史对账。 |
| `388`，`4f1bfa96d` | 在 `provider_profit_links` 上增加 `wallet` / `estimate` 模式。旧行维持钱包模式，新行默认估算。该 DDL 依赖前置表和对应的授权、管理端、钱包采集、报表运行时。 |
| `7563b8869` | 按事件时点连接手工及导入归属时间线。手工解绑阻止导入回补，同时保留既有 URL 回退。需覆盖历史时点、区间边界和联接基数。 |

本批没有引入 SQL。生产基线已使用编号至 `426`。新增迁移须重新检查远端编号及真实数据库结构，保留既有迁移校验和。

## 验收状态

- 2026-11-21 复核 GitHub 最新正式版、tag 对象及源码提交，验证固定版本来源。
- 管理员弹窗视觉证据见 [管理员版本信息](../../visual-reviews/2026-11-21-v2231-admin-version.md)。生产浏览器游客、普通用户、管理员验收待记录。
- 完整迁移仍取决于累计差异处置、供应商相关数据迁移和会计对账，以及最终的部署与产品验收。
