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
| `POST /api/v1/mobile/support/tickets` | canonical | APP 反馈/客服工单提交，支持 JSON 和 multipart 截图 | `mobile_feedback` | 新 APP 默认使用 |
| `GET /api/v1/mobile/support/tickets` | canonical | 用户查看自己的工单列表 | `mobile_feedback` | 保留 |
| `GET /api/v1/mobile/support/tickets/:id` | canonical | 用户查看工单详情和客服回复 | `mobile_feedback`, `mobile_feedback_messages` | 保留 |
| `POST /api/v1/mobile/support/tickets/:id/messages` | canonical | 用户补充问题 | `mobile_feedback_messages` | 保留 |
| `POST /api/v1/mobile/support/tickets/:id/close` | canonical | 用户关闭工单 | `mobile_feedback` | 保留 |
| `POST /api/v1/play/mobile-feedback` | legacy | 旧 APP 反馈提交 | `mobile_feedback` | 仅兼容旧版本，不再扩展；观察 30 天无请求后移除 |
| `GET /api/v1/admin/play/mobile-feedback` | canonical | 玩法运营统一管理 APP 反馈 | `mobile_feedback` | 保留，不新增第二套后台入口 |
| `GET /api/v1/admin/play/mobile-feedback/:id` | canonical | 反馈详情、客服记录、关联需求项 | `mobile_feedback`, `mobile_feedback_work_items` | 保留 |
| `PATCH /api/v1/admin/play/mobile-feedback/:id` | canonical | 更新反馈处理状态、客服备注、关联需求状态 | `mobile_feedback`, `mobile_feedback_work_items` | 保留 |

## 移动端统一协议 v1

| 接口 | 状态 | 用途 | 数据归属 | 替代/处理 |
| --- | --- | --- | --- | --- |
| `GET /api/v1/mobile/protocol` | canonical | APP 获取统一协议版本、任务状态、接口生命周期和隐私规则 | 无业务写入 | 保留 |
| `GET /api/v1/mobile/session/status` | canonical | APP 登录态自检；401 时应先 refresh token 无感续期后重试原请求 | 无业务写入 | 保留 |
| `GET /api/v1/mobile/account-summary` | canonical | 账户、余额、分组、订阅、套餐消耗聚合 | user、wallet、subscription、payment | 保留 |
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
| `POST /api/v1/mobile/tasks/:id/status` | canonical | 更新任务状态 | `mobile_tasks` | 保留 |
| `GET /api/v1/mobile/image-history` | canonical | 生图历史，是 `kind=image` 任务的语义化视图 | `mobile_tasks` | 保留 |
| `DELETE /api/v1/mobile/image-history/:id` | canonical | 删除生图历史 | `mobile_tasks.deleted_at` | 保留 |
| `POST /api/v1/mobile/image-history/:id/retry` | canonical | 重试生图历史 | `mobile_tasks` | 保留 |
| `POST /api/v1/mobile/assets` | canonical | 上传图片、PDF、语音、分享文件等素材 | `mobile_assets` | 保留 |
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
