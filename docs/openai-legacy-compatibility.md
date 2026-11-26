# OpenAI 旧版兼容模式

## 目的和默认值

`gateway.openai_legacy_compatibility_mode` 是为需要暂时恢复旧版 OpenAI agent-history 重试行为的部署提供的兼容开关。默认值为 `false`，建议保持关闭；关闭时沿用当前的 owner-aware recovery 和请求重放保护。

配置示例（`deploy/config.example.yaml`）：

```yaml
gateway:
  openai_legacy_compatibility_mode: false
```

该配置只在网关启动时读取。更改时应安排维护窗口并同时重启所有网关实例，避免不同实例在恢复与重试策略上不一致。

## 行为边界

开启时，OpenAI gateway 不会建立或使用较新的 agent-owner recovery 上下文，不会执行对应的加密历史省略处理，并允许既有 HTTP/WS 普通账号池重试策略处理请求。它用于兼容旧客户端/旧上游交互，不是推荐的常态运行模式。

该开关只改变 OpenAI agent-history 恢复、历史处理和相关重试决策；它不替代认证、账号/分组授权、计费或请求大小限制，也不代表任意失败都可安全重放。重试仍可能导致上游重复执行或重复计费，故仅应在明确验证兼容性后短期启用。

## 启用、验证与回滚

1. 在所有网关实例的受管配置中设置 `gateway.openai_legacy_compatibility_mode: true`。
2. 维护窗口内同时重启所有网关实例；确认所有实例使用相同配置。
3. 使用受控测试账号验证所需 HTTP 与 WebSocket 流程，并检查请求是否重复执行、上游计费及账户切换结果。
4. 问题排查完毕后将值恢复为 `false`，再次同时重启所有实例；随后确认 owner-aware recovery/no-replay 路径已恢复。

若出现重复上游执行、费用异常或客户端状态不一致，立即回滚为 `false` 并重启全部网关实例。不得仅凭客户端重试成功判断安全。

## 回归测试

```sh
cd backend
go test ./internal/config ./internal/service ./internal/handler -run 'LegacyCompatibility'
```

配置解析与覆盖优先级另由 `issue914_legacy_compatibility_test.go` 覆盖。该功能文档不表示兼容模式已在生产启用；生产配置与部署 SHA 应分别记录在对应发布账本中。
