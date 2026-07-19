# Gateway 单请求异步图片任务

> 状态：active
> 最后核验：2026-07-18

Gateway async 让客户端提交一个 OpenAI-compatible Images 请求后立即获得任务 ID，不需要保持长 HTTP 连接。它适合 GPT/Grok 长耗时生成与编辑；多个 prompt 的持久批任务应使用 `/v1/images/batches`。

## 路由

```text
POST /v1/images/generations/async
POST /v1/images/edits/async
GET  /v1/images/tasks/{task_id}
GET  /v1/images/task-assets/{path}
```

提交使用 API Key，body 与同步生成/编辑接口一致。流式请求不接受异步提交。

## 运行时

请求不再由无界进程内 goroutine 执行。当前实现：

- 加密持久化请求信封
- Redis ready/active 队列
- 可配置有界 worker
- 原子提交和用户/API Key 范围的幂等映射
- active lease、心跳和锁续期
- queued -> processing 和 processing -> terminal 使用 Redis CAS，迟到写入不能覆盖终态
- 租约或 job lock 丢失会取消正在执行的 provider 请求，不 ACK、不伪造失败结算
- 服务重启后继续处理 queued 任务
- 崩溃前已进入 processing 的任务明确转 failed，不重放可能已经计费的上游请求
- worker 重新加载 API Key、用户、分组和订阅上下文；订阅恢复失败直接终止，不回退余额计费
- worker 使用 task ID 作为稳定 request/client request ID，防止重试产生多份计费关联
- worker 内部强制使用 Base64，再把结果一次性转存到 task asset storage
- terminal 记录清除加密请求信封，只保留结果、错误和必要审计元数据

Redis 队列的重启恢复以持久卷和 AOF 为前提。官方 Docker Compose 与 Zeabur 模板已启用
`appendonly yes`、`appendfsync everysec` 并挂载 Redis 数据卷；使用外部 Redis 时必须
配置等价持久化。运行时健康检查只能确认 Redis 可访问，不能替外部托管服务证明其
持久化策略。

## 启用

通用部署默认关闭。生产需要：

```text
IMAGE_STORAGE_ENABLED=true
IMAGE_STORAGE_BACKEND=local
IMAGE_ASYNC_QUEUE_ENABLED=true
IMAGE_ASYNC_ENABLED=true
IMAGE_ASYNC_WORKER_COUNT=4
```

`IMAGE_ASYNC_ENABLED=true` 要求 queue 和图片存储同时启用，否则配置校验失败。

允许：

```text
IMAGE_ASYNC_ENABLED=false
IMAGE_ASYNC_QUEUE_ENABLED=true
```

用于停止新提交、继续恢复和排空存量任务。

状态含义：

- API 未启用：提交返回 `404 not_found_error`
- API 已启用但 Redis、队列或 worker 未就绪：`503 IMAGE_ASYNC_NOT_READY`
- 未就绪时不创建任务、不执行上游调用

管理员可调用：

```text
GET /api/v1/admin/ops/image-runtimes/health
```

查看 Gateway async 的开关、存储、Redis、worker、ready/active backlog 和最近错误，不暴露凭据。

## 提交与幂等

```bash
curl -i https://api.jisudeng.com/v1/images/generations/async \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: image-task-20260718-001" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "暴风雪中的灯塔，电影感，横向构图",
    "size": "1536x1024",
    "n": 2,
    "response_format": "url"
  }'
```

成功返回 `202 Accepted`：

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "queued",
  "created_at": 1784160000,
  "expires_at": 1784246400,
  "poll_url": "/v1/images/tasks/imgtask_0123456789abcdef"
}
```

响应头：

```text
Location: /v1/images/tasks/imgtask_0123456789abcdef
Retry-After: 3
Cache-Control: no-store
```

`Idempotency-Key` 最多 255 字节。超长时返回
`400 IMAGE_TASK_IDEMPOTENCY_KEY_INVALID`，不会创建任务或入队。同 owner、
同 `Idempotency-Key`、同请求返回已有任务，并设置
`Idempotent-Replayed: true`；同 key 不同请求返回
`409 IMAGE_TASK_IDEMPOTENCY_CONFLICT`。

同步接口的请求校验契约同样适用于异步提交：空白 prompt 返回
`400 IMAGE_PROMPT_REQUIRED`，非法 `response_format` 返回
`400 IMAGE_RESPONSE_FORMAT_INVALID`，且都发生在任务创建和入队之前。

## 轮询

必须使用提交任务时的同一把 API Key：

```bash
curl https://api.jisudeng.com/v1/images/tasks/imgtask_0123456789abcdef \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx"
```

状态机：

```text
queued -> processing -> completed
                     -> failed
```

`queued` 和 `processing` 响应带 `Retry-After: 3`。

恢复规则是保守的：`queued` 尚未调用 provider，可以在重启后继续执行；一旦任务已
进入 `processing`，平台无法证明崩溃前的上游请求是否已经成功或计费，因此恢复时会
转为 `failed`，错误码为 `IMAGE_TASK_RECOVERY_UNAVAILABLE`，不会自动重放。

## 结果

worker 内部把生成请求规范化为 Base64，完成后把每张图片写入结果存储。Redis 任务记录只保存紧凑 URL，不保存大段 Base64。

```json
{
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "completed",
  "http_status": 200,
  "image_url": "/v1/images/task-assets/images/imgtask_0123456789abcdef-0.png",
  "result": {
    "created": 1784160123,
    "data": [
      {"url": "/v1/images/task-assets/images/imgtask_0123456789abcdef-0.png"},
      {"url": "/v1/images/task-assets/images/imgtask_0123456789abcdef-1.png"}
    ]
  },
  "completed_at": 1784160123,
  "expires_at": 1784246523
}
```

task asset 必须使用同一把 API Key 下载。其他 Key 和未知任务均返回 404，避免泄露任务存在性。

结果存储失败时任务转 failed，不回退把 Base64 写入 Redis。

## 关闭与回滚

1. 先设置 `IMAGE_ASYNC_ENABLED=false` 停止新提交。
2. 保持 `IMAGE_ASYNC_QUEUE_ENABLED=true`，等待 ready/active backlog 归零。
3. 确认无 processing 任务后再关闭 queue。
4. 结果仍在保留期内时保持图片存储可读。
