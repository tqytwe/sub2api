# 移动端后端统一改造方案

## 目标

本方案把 APP 依赖的后端能力统一到稳定的 `/api/v1/mobile/*` 和少量通用接口上，减少 APP 多接口并发同步、聊天和生图分组互相污染、支付拉起状态丢失、历史记录不能管理等问题。

本次只完成后端代码与迁移准备，不部署、不重启生产。生产更新必须由负责人确认时间窗口后再执行。

## P0 一次性后端契约

### 账户聚合

- `GET /api/v1/mobile/protocol`
- `GET /api/v1/mobile/session/status`
- `GET /api/v1/mobile/account-summary`
- 保留兼容：`GET /api/v1/nextchat/mobile/account-summary`
- 返回账户页所需的余额、冻结余额、分组、订阅、套餐进度等聚合信息。
- APP 首选新路径，失败时可短期回退旧路径。

`/mobile/protocol` 返回移动端统一协议版本、接口生命周期、任务状态、隐私脱敏规则。`/mobile/session/status` 返回当前登录态，APP 遇到 401 时必须先尝试 `/api/v1/auth/refresh`，刷新成功后重放原请求；只有 refresh token 也明确失效时才提示重新登录。

### 聊天和生图独立托管会话

- `GET /api/v1/mobile/sessions`
- `POST /api/v1/mobile/sessions/chat/switch-group`
- `POST /api/v1/mobile/sessions/image/switch-group`
- 请求体：

```json
{
  "group_id": 123
}
```

- 后端继续复用 NextChat managed session，但移动端必须按 `purpose=chat/image` 分开签发和切换。
- APP 不再依赖后端“当前分组”作为全局状态，避免生图切组影响聊天。

### 统一任务协议

- `POST /api/v1/mobile/tasks`
- `GET /api/v1/mobile/tasks`
- `GET /api/v1/mobile/tasks/:id`
- `DELETE /api/v1/mobile/tasks/:id`
- `POST /api/v1/mobile/tasks/:id/cancel`
- `POST /api/v1/mobile/tasks/:id/retry`
- `POST /api/v1/mobile/tasks/:id/status`

任务类型统一为 `chat`、`image`、`file`。状态统一为 `queued`、`running`、`streaming`、`completed`、`partial`、`failed`、`cancelled`。

删除采用软删除：新增 `mobile_tasks.deleted_at`，APP 列表和详情默认不可见，数据库仍保留诊断记录。

### 生图历史

- `GET /api/v1/mobile/image-history`
- `DELETE /api/v1/mobile/image-history/:id`
- `POST /api/v1/mobile/image-history/:id/retry`

这三个接口是任务协议的语义化包装，内部只处理 `kind=image` 的移动任务。失败、生图完成、取消都能进入同一套历史管理。

### 兑换码

- `POST /api/v1/redeem-codes/redeem`
- `GET /api/v1/redeem-codes/history`
- 保留兼容：`POST /api/v1/user/redeem`、`GET /api/v1/user/redeem/history`
- 新路径复用原兑换服务，不新增另一套兑换规则。

### 移动支付闭环

- `POST /api/v1/mobile/payments/create`
- `GET /api/v1/mobile/payments/:order_id`
- `POST /api/v1/mobile/payments/:order_id/sync`

创建支付强制按移动端场景处理，返回：

- `launch`
- `deeplink`
- `scheme_url`
- `mweb_url`
- `h5_url`
- `pay_url`
- `qr_code`
- `return_url`
- `resume_token`
- `verify_after_ms`
- `paid`
- `completed`
- `can_retry_payment`

查询和同步返回订单状态。同步接口会根据订单 `out_trade_no` 主动查单并触发原有到账流程。

## P1 后端支撑

- 素材库继续使用 `/api/v1/mobile/assets`，系统分享文件由 APP 上传后生成 asset 记录。
- 技能中心继续使用 `/api/v1/mobile/skills`，后续服务端技能数据必须区分 `skill` 和 `agent`：
  - `agent` 是智能体角色、人格、专业背景。
  - `skill` 是可被智能体调用的能力、模板、步骤、输入要求和消耗说明。
- 客服工单继续使用 `/api/v1/mobile/support/tickets`，诊断信息走 `/api/v1/mobile/diagnostics`，不得上传聊天全文、access token、API key。

## P2 稳定性支撑

- 设备注册继续使用 `/api/v1/mobile/devices/:installation_id`。
- FCM 推送可绑定任务终态、支付到账、客服回复。
- 网络诊断需要 APP 上报网络类型、失败 URL 分类、状态码、超时、WebView 错误码，不上传敏感正文。

## 数据库迁移

- 新增：`backend/migrations/223_mobile_task_visibility_and_image_history.sql`
- 内容：
  - `mobile_tasks.deleted_at TIMESTAMPTZ NULL`
  - 可见任务索引：`user_id, kind, created_at`
  - 可见状态索引：`user_id, status, created_at`
- 新增：`backend/migrations/224_mobile_feedback_work_items.sql`
- 内容：
  - `mobile_feedback_work_items`
  - 关联现有 `mobile_feedback`
  - 不新增第二套后台管理入口

## 部署要求

生产部署前必须：

1. 在维护窗口执行数据库迁移。
2. 部署后端新版本。
3. 验证新旧接口都可用。
4. 再发布只依赖新接口的 APP。

不得在未获确认时更新或重启生产后端。

## 验收清单

- APP 账户页只请求 `/mobile/account-summary` 即可加载。
- 新对话和已有对话切换聊天分组都成功。
- 生图切换分组不影响聊天分组。
- 任务取消、重试、删除状态一致。
- 生图失败历史可删除，可重试。
- 兑换码新旧路径都能兑换并看到历史。
- 支付创建返回移动拉起字段，返回 APP 后 `sync` 能查单到账。
- 弱网下认证失败只在真正无效时返回 401，APP 可先自动刷新/重试，不应直接打断用户。

## 回滚

- 旧接口全部保留，后端回滚不会影响旧 APP。
- `deleted_at` 是新增空列，回滚代码后不会破坏现有任务数据。
- APP 仍可短期保留旧路径 fallback。

## 当前边界

- 当前已具备服务端技能目录、版本、输入要求、示例和消耗说明字段，但 `/mobile/skills/:slug/use` 主要记录启用/最近使用；完整 skill 执行编排仍需后续工作流执行器支撑。
- 当前已具备统一任务记录、取消、重试、删除和生图历史包装；真实聊天 SSE 中断、生图上游取消仍取决于对应执行链路是否接入任务 ID。
- 当前未把智能体协作宣传为完整后端多智能体编排；真正协作需要服务端保存步骤、中间状态、参与 agent、合并策略和可追踪结果。
