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
