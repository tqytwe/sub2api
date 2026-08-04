# API 生命周期治理

## 原则

- 新功能必须先确认是否能复用现有 canonical 接口。
- legacy 接口只做兼容，不继续扩展业务能力。
- removed 接口必须经过日志观察期，不能直接删除仍有活跃客户端使用的接口。
- 管理入口必须收敛。APP 反馈、客服沟通、需求/缺陷统一维护在玩法运营的 `mobile_feedback` 体系里。

## 状态定义

- `canonical`：当前正式接口，新客户端必须优先使用。
- `legacy`：旧客户端兼容接口，只写入同一套数据，不新增能力。
- `observe`：准备下线，记录访问量和客户端版本。
- `removed`：已删除或网关拒绝。

## 移动端反馈接口

| 接口 | 状态 | 用途 | 数据归属 | 替代/处理 |
| --- | --- | --- | --- | --- |
| `POST /api/v1/mobile/support/tickets` | canonical | APP 反馈/客服工单提交，支持 JSON 和 multipart 截图 | `mobile_feedback` | 新 APP 默认使用；`Idempotency-Key` 在观察期可选，携带时按账号和请求指纹回放原工单，不重复上传截图 |
| `GET /api/v1/mobile/support/tickets` | canonical | 用户查看自己的工单列表 | `mobile_feedback` | 保留 |
| `GET /api/v1/mobile/support/tickets/:id` | canonical | 用户查看工单详情和客服回复 | `mobile_feedback`, `mobile_feedback_messages` | 保留 |
| `POST /api/v1/mobile/support/tickets/:id/messages` | canonical | 用户补充问题 | `mobile_feedback_messages` | 保留 |
| `POST /api/v1/mobile/support/tickets/:id/close` | canonical | 用户关闭工单 | `mobile_feedback` | 保留 |
| `POST /api/v1/play/mobile-feedback` | legacy | 旧 APP 反馈提交 | `mobile_feedback` | 仅兼容旧版本，不再承载新功能；当 APP 因 canonical 路由不存在而降级且复用已有 `Idempotency-Key` 时，服务端回放原工单，避免截图和工单重复写入；观察 30 天无请求后移除 |
| `GET /api/v1/admin/play/mobile-feedback` | canonical | 玩法运营统一管理 APP 反馈 | `mobile_feedback` | 保留，不新增第二套后台入口 |
| `GET /api/v1/admin/play/mobile-feedback/:id` | canonical | 反馈详情、客服记录、关联需求项 | `mobile_feedback`, `mobile_feedback_work_items` | 保留 |
| `PATCH /api/v1/admin/play/mobile-feedback/:id` | canonical | 更新反馈处理状态、客服备注、关联需求状态 | `mobile_feedback`, `mobile_feedback_work_items` | 保留 |

## 移动端统一协议 v2

### 服务端联网搜索配置

移动端联网搜索只有在服务端同时设置 `MOBILE_WEB_SEARCH_ENABLED=1`（也接受
`true`、`yes` 或 `on`）和
`EXA_API_KEY` 时才会下发为 `canonical` capability。`EXA_API_KEY` 只能通过
生产环境的 secret manager 注入，禁止写入 APP、WebView、数据库或协议响应。
未配置时协议返回 `execution_state=disabled`，请求统一返回可本地化的
`MOBILE_WEB_SEARCH_UNAVAILABLE` 错误；不声明或实现 DuckDuckGo 隐式回退。

| 接口 | 状态 | 用途 | 数据归属 | 替代/处理 |
| --- | --- | --- | --- | --- |
| `GET /api/v1/mobile/protocol` | canonical | APP 获取统一协议版本、任务状态、接口生命周期和隐私规则 | 无业务写入 | 保留 |
| `GET /api/v1/mobile/session/status` | canonical | APP 登录态自检，并返回后端判定的 `capabilities.admin`；401 时应先 refresh token 无感续期后重试原请求 | 无业务写入 | 保留 |
| `GET /api/v1/mobile/account-summary` | canonical | 账户、余额、分组、订阅、套餐消耗聚合 | user、wallet、subscription、payment | 保留 |
| `POST /api/v1/mobile/web-search` | canonical | 用户明确确认后使用服务端 Exa 联网搜索；仅 JWT，不接受管理员 API Key | server-side Exa | 保留 |
| `GET /api/v1/nextchat/mobile/account-summary` | legacy | 旧 APP 账户聚合路径 | 同 canonical | 仅兼容旧版本；新能力不扩展 |
| `GET /api/v1/nextchat/mobile/bootstrap` | canonical | 移动端聊天和生图独立托管会话启动 | user api key/session | 保留 |
| `GET /api/v1/mobile/sessions` | canonical | 获取聊天、生图独立会话 | user api key/session | 保留 |
| `POST /api/v1/mobile/sessions/chat/switch-group` | canonical | 只切换聊天会话分组 | user api key/session | 保留 |
| `POST /api/v1/mobile/sessions/image/switch-group` | canonical | 只切换生图会话分组 | user api key/session | 保留 |
| `POST /api/v1/nextchat/mobile/group` | legacy | 旧全局分组切换路径，可能造成聊天/生图互相污染 | user api key/session | 新 APP 不再使用，替代为 purpose 分离路径 |
| `POST /api/v1/mobile/tasks` | canonical | 创建聊天、生图、文件统一任务记录 | `mobile_tasks` | 保留 |
| `GET /api/v1/mobile/tasks` | canonical | 统一任务历史 | `mobile_tasks` | 保留 |
| `GET /api/v1/mobile/tasks/:id` | canonical | 任务详情 | `mobile_tasks` | 保留 |
| `DELETE /api/v1/mobile/tasks/:id` | canonical | 软删除任务 | `mobile_tasks.deleted_at` | 保留 |
| `POST /api/v1/mobile/tasks/:id/cancel` | canonical | 取消任务 | `mobile_tasks` | 保留 |
| `POST /api/v1/mobile/tasks/:id/retry` | canonical | 重试任务 | `mobile_tasks` | 保留 |
| `POST /api/v1/mobile/tasks/:id/status` | observe | 旧客户端兼容任务状态写入 | `mobile_tasks` | 新客户端不再写入；观察期后按最低支持版本下线 |
| `GET /api/v1/mobile/image-history` | canonical | 生图历史，是 `kind=image` 任务的语义化视图 | `mobile_tasks` | 保留 |
| `DELETE /api/v1/mobile/image-history/:id` | canonical | 删除生图历史 | `mobile_tasks.deleted_at` | 保留 |
| `POST /api/v1/mobile/image-history/:id/retry` | canonical | 重试生图历史 | `mobile_tasks` | 保留 |
| `POST /api/v1/mobile/assets` | canonical | 上传图片、PDF、语音、分享文件等素材 | `mobile_assets` | 保留；`Idempotency-Key` 在观察期可选，携带时按账号和文件摘要回放原素材，不重复保存文件 |
| `GET /api/v1/mobile/assets` | canonical | 素材库列表 | `mobile_assets` | 保留 |
| `GET /api/v1/mobile/assets/:id` | canonical | 素材详情 | `mobile_assets` | 保留 |
| `GET /api/v1/mobile/assets/:id/content` | canonical | 素材内容读取 | `mobile_assets` | 保留 |
| `DELETE /api/v1/mobile/assets/:id` | canonical | 删除素材 | `mobile_assets` | 保留 |
| `GET /api/v1/mobile/skills` | canonical | 服务端技能目录；skill 是可执行模板/流程，不等同 agent | `mobile_skills` | 保留 |
| `GET /api/v1/mobile/skills/:slug` | canonical | 技能详情、版本、输入要求、示例、消耗说明 | `mobile_skills`, `mobile_skill_versions` | 保留 |
| `POST /api/v1/mobile/skills/:slug/install` | canonical | 安装技能 | `user_mobile_skills` | 保留 |
| `POST /api/v1/mobile/skills/:slug/use` | canonical | 记录技能启用和最近使用 | `user_mobile_skills` | 保留；不宣传成完整工作流执行 |
| `DELETE /api/v1/mobile/skills/:slug/install` | canonical | 卸载技能 | `user_mobile_skills` | 保留 |
| `POST /api/v1/mobile/diagnostics` | canonical | 脱敏移动端网络、支付、WebView、请求失败诊断 | `mobile_diagnostics` | 保留 |
| `PUT /api/v1/mobile/devices/:installation_id` | canonical | 注册推送设备 | mobile device storage | 保留 |
| `DELETE /api/v1/mobile/devices/:installation_id` | canonical | 解绑推送设备 | mobile device storage | 保留 |
| `POST /api/v1/mobile/payments/create` | canonical | 创建移动端支付并返回拉起字段 | payment order | 保留 |
| `GET /api/v1/mobile/payments/:order_id` | canonical | 查询移动端支付订单 | payment order | 保留 |
| `POST /api/v1/mobile/payments/:order_id/sync` | canonical | 返回 APP 后主动查单同步到账 | payment order | 保留 |
| `POST /api/v1/redeem-codes/redeem` | canonical | 兑换码、活动码和套餐码兑换 | redeem code | 保留 |
| `GET /api/v1/redeem-codes/history` | canonical | 兑换记录 | redeem code | 保留 |

### 移动端创建请求重试

- `POST /api/v1/mobile/assets` 和 `POST /api/v1/mobile/support/tickets`
  在本轮接入通用 `IdempotencyCoordinator`。客户端发送同一个
  `Idempotency-Key`、`X-Client-Request-ID` 和 `X-Request-ID` 时，服务端按
  用户作用域和请求指纹回放原始 `201` 资源，成功重试不重复写入对象存储、素材或工单。
  legacy 反馈路由只为安全处理上述工单的降级重试而接入同一协调器，不能作为新客户端
  的主动提交入口。
- 无幂等键的旧客户端继续在当前观察期内执行，便于平滑升级；在最低支持版本切换前
  不强制拒绝。键复用但内容不同返回冲突，处理中返回可重试冲突，存储不可用保持
  fail-closed，避免把不确定提交伪装成成功。
- APP 只在 canonical 工单接口返回 `404`、`405` 或 `501` 时回退
  `/api/v1/play/mobile-feedback`。该 legacy 路径只为旧服务器兼容，不能被新功能主动
  选用；但降级请求必须复用 canonical 已生成的 `Idempotency-Key` 和关联 ID，服务端
  按用户和请求指纹回放原工单，避免 native transport 重试重复上传截图或创建工单。
  它仍按既定下线流程观察，不新增第二套业务语义。

### 移动管理员复用原则

- APP 只能依据 `GET /api/v1/mobile/session/status` 返回的 `capabilities.admin.available` 显示管理员入口，禁止扫描邮箱、角色别名或前端字符串猜测权限。
- `capabilities.admin.api_base_path` 指向既有 `/api/v1/admin`；用户、订单、分组、模型、玩法运营、工单和审计继续使用各自已有 canonical 管理接口，不创建同名 `/mobile/admin/*` 镜像接口。
- 需要二次验证的操作继续复用 `capabilities.admin.step_up_path` 指向的 `POST /api/v1/user/totp/step-up`。授权绑定当前 JWT 会话，APP 收到 `STEP_UP_REQUIRED` 后验证并重放原请求。
- `capabilities.admin.compliance_path` 仅在服务端实际启用管理员合规守卫时下发，值为 `/api/v1/admin/compliance`；字段缺失的旧服务保持既有只读管理路径，APP 不得探测该接口。
- 只有已有接口无法提供移动端所需的聚合、脱敏或最小化数据形状时，才可以新增专用接口；新接口必须复用现有认证、审计、幂等、请求 ID 与错误响应规范，不得复制业务规则。

### 管理员移动端核对（2026-08-03）

主 `/api/v1/admin/*` 路由统一经过 `AdminAuth`、面板限流、审计日志和合规守卫；独立注册的 `/api/v1/admin/payment/*` 也复用 `AdminAuth`、审计和合规守卫。APP 只能使用管理员 JWT，绝不保存或发送 `x-api-key`。首次遇到 `423 ADMIN_COMPLIANCE_ACK_REQUIRED` 时复用 `GET/POST /api/v1/admin/compliance`；遇到 `403 STEP_UP_REQUIRED` 时复用 TOTP step-up 后，以同一个幂等键重放原请求。

| 管理域 | 已有 canonical 路由 | 移动端处理 |
| --- | --- | --- |
| 概览与运行状态 | `/admin/dashboard/*`、`/admin/ops/*` | 可做只读概览，不复制统计逻辑 |
| 用户、余额、分组、模型 | `/admin/users/*`、`/admin/groups/*`、`/admin/model-catalog/*` | 搜索/详情复用；涉及余额、角色、分组写入先核对 step-up 与幂等 |
| 订单、卡券、套餐与资金 | `/admin/payment/*`、`/admin/promo-codes/*`、`/admin/subscriptions/*`、`/admin/funds/*`、`/admin/withdrawals/*` | 订单/用量可读；资金和结算只在路由级 step-up 与幂等验收后开放 |
| 玩法、战队、反馈与邀请 | `/admin/play/*`、`/admin/affiliates/*` | APP 反馈继续用 `/admin/play/mobile-feedback`；不增加第二套 support/work-items 路由 |
| 上游账号、代理、渠道、备份与系统 | `/admin/accounts/*`、`/admin/proxies/*`、`/admin/channels/*`、`/admin/backups/*`、`/admin/system/*` | 不在移动端直接暴露密钥、导出、对象存储、备份恢复或服务重启 |
| 审计与合规 | `/admin/audit-logs/*`、`/admin/compliance` | 查询可复用；清理继续使用现有现场 TOTP 校验 |

本轮确认不存在 `/api/v1/mobile/admin/*`，也不创建镜像路由。唯一协议变化是由已有 `GET /api/v1/mobile/session/status` 下发管理员可用性和既有路径；这不是新的管理员业务接口。当前真正待补的是移动端统一处理 `STEP_UP_REQUIRED`、`ADMIN_COMPLIANCE_ACK_REQUIRED`、`Idempotency-Key` 和 request ID，以及逐项收紧尚未被 step-up 覆盖的高风险管理写操作。

## Legacy 接口下线流程

1. 新 APP 改用 canonical 接口。
2. legacy 接口保留认证、请求体大小限制、限流和错误脱敏。
3. 记录 legacy 接口访问量、客户端版本、用户 ID、状态码。
4. 连续 30 天无请求，或低于最低支持 APP 版本后，进入 `observe`。
5. `observe` 期只允许返回明确升级提示或只读兼容。
6. 再一个发布周期后移除路由。

## 反馈到需求的统一模型

- 反馈主表：`mobile_feedback`
- 用户/客服消息：`mobile_feedback_messages`
- 自动生成和后台维护的需求/缺陷项：`mobile_feedback_work_items`

后台仍然从玩法运营反馈详情进入，不增加 `/admin/support`、`/admin/work-items` 等分散入口。
