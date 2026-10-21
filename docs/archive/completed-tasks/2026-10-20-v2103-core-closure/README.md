# v2.10.3 五项核心闭环核对

核对时间：2026-10-20 19:04:29 UTC

本记录只核对 `ranxi2001/sub2api` `v2.10.3`（release commit
`fd1b5ee4eeb20961fbb783fa6f136a1704271e90`）中已锁定的五项核心闭环：源码、真实请求入口和测试证据是否互相对应。它不把整版上游迁移、生产验收或后续版本同步写成已完成，也不引入上游之外的功能。

## 结论

五项闭环在当前代码中均已实现并通过定向验证；没有发现需要再补的上游行为。当前分支新增的业务改动仍只有“请求台账仅管理员可见/可访问”，核心闭环没有额外二开。

| 闭环 | 实际入口与实现 | 定向证据 | 结论 |
| --- | --- | --- | --- |
| 简单模式错误路径与结算恢复 | `backend/internal/service/gateway_usage_billing.go`；`backend/internal/service/gateway_usage_billing_simple_mode_test.go` | `SimpleMode`、`UsageSettlement`、`BillingError` | 简单模式只更新约定的 API key 限流额度；失败路径、幂等结算和缓存失效保持原计费合同。 |
| `allowed_models` 存储到路由和最终准入 | `account_group.go`、`group_model_allowlist.go`、`openai_models_list.go`、`openai_gateway_scheduling.go`、`openai_turn_admission.go` | `AllowedModels`、`OpenAIChatAdmission`、`TurnAdmission`、repository policy tests | 列表发现、普通/粘性选路、映射模型和最终发送前准入都执行同一限制；列表结果不代替最终准入。 |
| Chat / 共享 CC 真实准入 | `openai_gateway_cc_pipeline.go`、`openai_gateway_chat_completions.go`、`openai_gateway_messages.go` | `OpenAIChatAdmission`、`SharedCC`、`openai_messages_admission_test.go` | 实际上游发送前重新读取并校验账号、路由和凭证；拒绝不会被当成上游健康失败。 |
| Responses 凭证刷新进入实际请求 | `openai_gateway_forward.go`、`openai_gateway_passthrough.go`、`openai_token_provider.go` | `Responses.*Credential`、`Credential.*Refresh` | HTTP recorder 捕获到轮换后的合成 `Authorization`，原凭证不会继续用于实际发送；覆盖 native、passthrough、OAuth shadow 和重试。 |
| passthrough `result + error` 用量结算和切换边界 | `openai_gateway_passthrough.go`、`openai_gateway_response_handling.go`、handler partial-usage tests | `Passthrough`、`UsageSettlement` | 已观察用量在失败终态仍只结算一次；已有计量结果不会触发同账号或跨账号重试，局部 SSE 保持原帧格式。 |

定向命令及结果：

```text
go test -tags=unit ./internal/service ./internal/handler ./internal/repository \
  -run 'OpenAIChatAdmission|SharedCC|Passthrough|SimpleMode|AllowedModels|UsageSettlement|BillingError|Responses.*Credential|Credential.*Refresh|TurnAdmission' -count=1

ok  github.com/Wei-Shaw/sub2api/internal/service      32.811s
ok  github.com/Wei-Shaw/sub2api/internal/handler       3.935s
ok  github.com/Wei-Shaw/sub2api/internal/repository    0.127s
```

## 范围边界

- `docs/upstream-migrations/source-lock.json` 仍保持 `fully_migrated: false`。这次结论只覆盖上表五项，不代表 v2.10.3 的全部页面、供应商、迁移和路由已同步。
- 非 OpenAI provider 的最新状态重读、Images/Embeddings/Audio/Realtime 等其他端点，以及移动端/Canvas，继续按原账本单独处理。
- 本记录没有生产凭据、真实 token、余额或历史账务数据；生产部署后的健康检查和游客/普通用户/管理员本机验收仍必须单独完成。
- 由于五项核心代码已经在 `origin/play/main` 的既有合并中，本分支没有复制上游整文件或增加未要求的产品行为。
