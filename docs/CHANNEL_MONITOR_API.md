# 渠道监控与站点存活接口

本文档说明当前已经存在的接口、认证边界和 QQ 机器人（Arstbot）可安全使用的范围。
它不新增机器人凭据或公开的全站渠道健康摘要接口。

## 认证与响应约定

- 面板 JWT：登录面板后取得的 JWT，使用 `Authorization: Bearer <panel-jwt>`。
- 管理员 JWT：具备管理员角色的面板 JWT；它不是网关 API Key。
- 网关 API Key：仅用于网关 OpenAI 兼容接口，不能认证以下面板接口。
- 除 `/health` 外，面板成功响应使用标准信封：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

错误响应同样使用该信封，HTTP 状态码表示错误类别。调用方应先检查 HTTP 状态码，再检查 `code`；不要把错误正文当作健康数据缓存。

## Arstbot 可直接调用的公开接口

### 站点存活

`GET https://www.jisudeng.com/health` 无需认证，适合机器人定时检测网站是否可达。

```bash
curl --fail-with-body --connect-timeout 5 --max-time 10 \
  https://www.jisudeng.com/health
```

成功时返回 HTTP `200`：

```json
{"status":"ok"}
```

这仅证明站点健康检查端点可响应，不等同于任一上游渠道、账号、模型或请求链路健康。

### 公开设置

`GET https://www.jisudeng.com/api/v1/settings/public` 无需认证，可读取公开版本与功能开关。

```bash
curl --fail-with-body --connect-timeout 5 --max-time 10 \
  https://www.jisudeng.com/api/v1/settings/public
```

它返回标准响应信封。该接口可用于识别公开的 `channel_monitor_enabled`、版本等设置，但不能代表渠道实际健康，也不应被解释为聚合数据已产生。

## 登录用户的 V2 只读接口

以下接口都要求：渠道监控已启用、运行模式为 V2、用户已经登录，并受面板重查询限流保护。

```text
GET /api/v1/channel-monitor-v2/dimensions
GET /api/v1/channel-monitor-v2/snapshot
GET /api/v1/channel-monitor-v2/models
GET /api/v1/channel-monitor-v2/matrix
GET /api/v1/channel-monitor-v2/errors
GET /api/v1/channel-monitor-v2/users
```

调用示例：

```bash
PANEL_JWT='<panel-jwt>'
curl --fail-with-body \
  -H "Authorization: Bearer ${PANEL_JWT}" \
  'https://www.jisudeng.com/api/v1/channel-monitor-v2/snapshot?range=90m'
```

共有查询参数：

| 参数 | 可选值或格式 | 说明 |
| --- | --- | --- |
| `range` | `90m`、`24h`、`7d`、`30d` | 统计窗口。 |
| `platform` | 可重复或逗号分隔 | 例如 `platform=openai&platform=anthropic` 或 `platform=openai,anthropic`。 |
| `group_id` | 可重复或逗号分隔的正整数 | 分组过滤。 |
| `model` | 可重复或逗号分隔 | 模型过滤。 |
| `group_by` | 仅 `matrix`：`platform`、`platform_group`、`platform_model`、`platform_group_model` | 矩阵聚合维度。 |

例如：

```bash
curl --fail-with-body \
  -H "Authorization: Bearer ${PANEL_JWT}" \
  'https://www.jisudeng.com/api/v1/channel-monitor-v2/matrix?range=24h&platform=openai,anthropic&group_id=7&group_by=platform_group'
```

普通用户响应会继续执行既有权限与脱敏规则。当前开启 `channel_monitor_hide_throughput=true` 时，用户数据不返回可用于推算规模的 RPM/TPM；机器人不得尝试通过其他字段反推流量规模。`/users` 也只返回当前权限允许的用户视图。

常见结果：

- `401`：JWT 缺失、过期或无效。
- `403`：功能关闭、V2 模式未启用或当前身份无权限。
- `400`：`range`、`group_id` 或 `group_by` 不合法。
- `429`：触发面板重查询限流，应指数退避，不能高频轮询。
- `5xx`：服务端暂时失败；记录响应状态和请求时间后重试，不把它表示为渠道故障。

## V1 历史接口

V1 主动探测模式使用下面的历史只读接口：

```text
GET /api/v1/channel-monitors
GET /api/v1/channel-monitors/:id/status
```

它们需要登录用户 JWT。V2 被动聚合启用时，应使用上一节的 V2 接口，而不是将 V1 探测结果和 V2 聚合指标混为同一口径。

## 管理员接口

管理员面板 JWT 可使用配置接口：

```text
GET /api/v1/admin/channel-monitor-v2/config
PUT /api/v1/admin/channel-monitor-v2/config
```

`GET`/`PUT config` 只要求渠道监控功能已启用，便于管理员在切换到 V2 前准备配置。V2 管理员读取接口与用户接口路径相同，前缀改为 `/api/v1/admin/channel-monitor-v2/`，包括 `dimensions`、`snapshot`、`models`、`matrix`、`errors`、`users`；这些读取接口额外要求 V2 模式。

既有 `/api/v1/admin/ops/*` 运维接口同样只面向管理员操作，不是外部机器人的服务状态接口。

不要把管理员 JWT、浏览器 Cookie 或网关 API Key 放入 QQ 机器人、群机器人配置、日志、截图或消息正文。

## 当前机器人边界

Arstbot 当前可以安全地查询 `/health`，从而报告站点在线或不可达。当前没有一个可供外部机器人在无用户身份下查询“全站渠道健康摘要”的接口。

若后续需要机器人查询渠道总览，应另立需求，至少包含独立的 bot 凭据、最小化且脱敏的摘要、限流、审计日志、凭据轮换，以及可选的 IP 白名单。不能复用管理员 JWT、浏览器会话或网关 API Key 作为临时方案。
