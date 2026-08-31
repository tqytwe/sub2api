# v0.1.182 数据库与部署只读基线

> 核对时间：2026-08-29（UTC）
> 目标服务：Zeabur 项目 `gmssh` 的 `sub2api-wortic`
> 核对性质：只读；未修改设置、数据、迁移记录或部署

## 部署身份

- Zeabur 环境：`6a51de14104975fcb46761c3`
- 当前运行提交：`ab69d89a851a0b17a270707e1a7e6293b3dbe0eb`
- 来源：`tqytwe/sub2api` 的 `refs/heads/play/main`
- PostgreSQL：18.4（Debian 18.4-1.pgdg13+1）

这确认生产仍以 `origin/play/main@ab69d89a` 为实现基线；本工作树的未提交候选改动尚未部署。

## 迁移谱系

生产 `schema_migrations` 有 373 条记录。当前仓库在加入本候选的 262-268 后有 373 个 SQL 迁移文件；其中与生产共享的 366 个文件 SHA256 全部一致，新增待部署迁移为 7 条。生产多出的 7 条历史日卡迁移仍属于“已在生产执行、仓库退休”的谱系差异：

- `188_growth_world_v1.sql`
- `228_daily_card_entitlements.sql`
- `230_daily_card_lifecycle_reconciliation.sql`
- `231_allow_zero_daily_card_holds.sql`
- `232_daily_card_redeem_entitlement_sources.sql`
- `233_usage_log_daily_card_entitlements.sql`
- `247_daily_card_request_replays.sql`

生产实际最后一条已执行迁移为 `261_mobile_video_execution_billing.sql`；生产尚未执行：

- `262_play_growth_qualification.sql`
- `263_public_status_snapshots.sql`
- `264_public_status_ttft_window_index_notx.sql`
- `265_play_growth_eligibility_orders_index_notx.sql`
- `266_play_growth_reward_snapshot_links.sql`
- `267_public_status_ops_aggregation_watermark.sql`
- `268_play_growth_governance.sql`

新迁移为 forward-only 候选。不得重建或重跑历史缺失迁移，不得修改已应用迁移，也不得使用 `033_ops_monitoring_vnext.sql` 做对齐。

## 关键生产结构与水位

- `usage_logs`：659,931 行；`MAX(created_at)=2026-08-29 08:49:45.172489+00`
- `ops_metrics_hourly`：6,129 行，其中 overall 聚合行 728 条；
  `MAX(bucket_start)=2026-08-29 07:00:00+00`
- `payment_orders` 已包含 `list_amount`、`qualifying_recharge_amount`、`refund_amount`、`payment_currency`、`completed_at`，可承载候选资格查询。
- `usage_logs` 已包含 `first_token_ms`、`actual_cost`、`billed_cost`，可承载候选状态快照查询。
- 生产没有 `public_status_snapshots`、`ops_aggregation_watermarks` 或
  `play_growth_eligibility_snapshots` 表；这是候选未部署的预期结果，不是要求人工补表的理由。
- 生产当前未发现候选新增 TTFT 或充值资格局部索引。
- 只读 `EXPLAIN (COSTS)` 预检显示，当前 24 小时 TTFT 原始查询可使用已有
  `idx_usage_logs_created_at`（估算成本约 `64.83..64.84`）；充值资格查询可使用已有
  `idx_payment_orders_user_id`（估算成本约 `8.21..8.23`，估算 430 条订单）。这只是
  生产当前结构的基线，不替代恢复演练库上的 `EXPLAIN (ANALYZE, BUFFERS)` 性能门禁。

## 已核对的运行配置

- `frontend_url` 当前为 `http://jisudeng.com`，尚未达到规范值
  `https://www.jisudeng.com`。
- `play_checkin_enabled=false`，`play_quiz_enabled=false`；候选增长规则不能被描述为
  已在线发奖。
- `custom_menu_items` 当前含历史文档项
  `095790f89fc04920 -> https://www.jisudeng.com/docs`。候选前端对该固定 ID 做无参数
  原生文档路由兼容跳转，因而部署后不会再为该书签创建 iframe。

`frontend_url` 是邮件和站外跳转的规范站点来源。其更新必须通过管理员 Settings UI 的
审计保存流程完成，不能通过生产 SQL 或容器命令绕过审计。本候选已把它从 CSP
`frame-ancestors` 决策中移除，因而这一待办不会重新放宽嵌入安全边界。

## 上线前门禁

1. 在恢复演练数据库先执行 262-268，并验证迁移 runner、约束、外键、局部索引、水位单调推进、成长治理审批/预算账本不可变性和回滚演练策略。对于 264/265，runner 在每次非事务重试前必须检测并删除同名无效并发索引，避免 `IF NOT EXISTS` 跳过中断遗留索引后错误写入迁移记录。
2. 对 24 小时 `first_token_ms` 百分位和累计调用数运行 `EXPLAIN (ANALYZE, BUFFERS)`，保留计划与耗时证据。
3. 候选合并并部署后重新核对实际 Zeabur SHA、`schema_migrations`、新增表列/约束/索引、快照水位、`/health`、CSP/XFO 与公开状态 API。
4. 生产 `frontend_url` 仍必须通过管理员 Settings UI 审计保存为 `https://www.jisudeng.com`；本次只读核对没有执行该设置变更。随后验证密码重置、通知邮件和 NextChat 来源链接均使用规范 HTTPS `www` 域名。

## 2026-08-29 只读 HTTP 复核

在未携带任何凭据的情况下重新探测生产 `https://www.jisudeng.com`：

- `GET /health` 返回 `200`，body 为 `{"status":"ok"}`。
- `GET /docs` 返回 `200 text/html`，当前响应仍为生产旧版本，
  `Content-Security-Policy` 包含 `frame-ancestors http://jisudeng.com`，同时保留
  `X-Frame-Options: SAMEORIGIN`；这再次证明协议/主机不匹配是文档 iframe 拒绝的确定性原因。
- `GET /api/v1/public/status-summary` 返回 `404`，符合候选状态接口尚未部署的预期。
- `GET /models` 携带 `Accept: */*` 返回 `401 API_KEY_REQUIRED`，说明候选的
  `Vary` 与爬虫内容协商修复尚未进入生产；不能把本地代码门禁当成线上行为。

本次探测为只读，未写入数据库、设置、缓存或部署；响应中的 request ID 仅用于本次
审计关联，不作为版本或成功发布证明。
