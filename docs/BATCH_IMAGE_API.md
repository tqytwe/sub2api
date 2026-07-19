# Batch Image 持久批任务 API

> 状态：active
> 网站入口：`https://www.jisudeng.com/docs?cat=deploy&page=batch-image-api`
> API 地址：`https://api.jisudeng.com`
> 最后核验：2026-07-18

## 产品边界

Batch Image 是多 prompt、可恢复、可取消、可下载的持久批任务。首发生产通道是 Gemini API Batch。

它不等同于：

- 同步 Images：`POST /v1/images/generations`、`POST /v1/images/edits`
- 单请求异步：`POST /v1/images/generations/async`、`POST /v1/images/edits/async`
- 图像工作室：JWT 登录页面、模板、估价和图库

Batch 与前两者使用 API Key；图像工作室使用登录 JWT。

## 路由

```text
GET    /v1/images/batches/models
POST   /v1/images/batches
GET    /v1/images/batches
GET    /v1/images/batches/{id}
GET    /v1/images/batches/{id}/items
GET    /v1/images/batches/{id}/items/{custom_id}/content
GET    /v1/images/batches/{id}/download
POST   /v1/images/batches/{id}/cancel
DELETE /v1/images/batches/{id}/outputs
DELETE /v1/images/batches/{id}
```

## 提交前预检

每次创建任务前，必须用准备提交的同一把 API Key 调用：

```bash
curl https://api.jisudeng.com/v1/images/batches/models \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx"
```

预检同时检查：

- Batch API 是否接受新提交
- Redis、队列和 worker 是否就绪
- Key 所属分组是否允许图片生成和 Batch 图片生成
- 是否存在可调度的 Gemini API Key 账号
- 是否存在模型映射
- 是否存在图片价格

常见不可用原因：

| 错误码 | 含义 |
|---|---|
| `BATCH_IMAGE_DISABLED` | 平台停止新提交；历史列表、预览和下载仍可读取 |
| `BATCH_IMAGE_NOT_READY` | Redis、队列或 worker 运行时异常 |
| `BATCH_IMAGE_GROUP_DISABLED` | 分组未开启 Batch 图片生成 |
| `BATCH_IMAGE_INVALID_MODEL` | 模型不在预检返回列表中 |
| 账号/价格错误 | 管理员尚未完成账号、模型映射或定价配置 |

## 提交契约

成功提交返回：

- HTTP `202 Accepted`
- `Location: /v1/images/batches/{id}`
- `Retry-After`
- Batch 任务对象

强烈建议首次提交生成稳定 `Idempotency-Key`。未提供该请求头时仍可提交，但网络失败或超时后的重复请求可能创建多个任务。提供后，重试相同请求必须复用原 key；请求内容改变时使用新 key。

```bash
curl -i https://api.jisudeng.com/v1/images/batches \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: batch-product-20260718-001" \
  --data-binary @batch-request.json
```

## 5 张示例

需要 5 张时使用 5 个独立 items：

```json
{
  "model": "gemini-3.1-flash-image-preview",
  "task_name": "商品主图 5 张",
  "image_size": "1K",
  "response_mime_type": "image/png",
  "items": [
    {"custom_id": "img_001", "prompt": "白色运动鞋，电商白底主图，柔光"},
    {"custom_id": "img_002", "prompt": "黑色机械键盘，俯拍，高清细节"},
    {"custom_id": "img_003", "prompt": "透明香水瓶，棚拍，干净阴影"},
    {"custom_id": "img_004", "prompt": "银色智能手表，深色背景，边缘光"},
    {"custom_id": "img_005", "prompt": "夏日冰咖啡，浅色社媒封面，标题留白"}
  ]
}
```

## 10 张示例

需要 10 张时使用 10 个独立 items。可从一行一个 prompt 的文件构造：

```bash
jq -Rn '
  [inputs | select(length > 0)]
  | to_entries
  | {
      model: "gemini-3.1-flash-image-preview",
      task_name: "批量素材 10 张",
      image_size: "1K",
      response_mime_type: "image/png",
      items: map({
        custom_id: ("img_" + (.key + 1 | tostring)),
        prompt: .value
      })
    }
' prompts-10.txt > batch-10.json
```

`output_count` 是同一 prompt 重复生成数量：

- 默认 1
- 单 item 最多 4
- 每任务最多 200 张
- 不要使用单 item `output_count=5` 或 `output_count=10`

系统会把 `output_count` 展开成真实图片任务项，不依赖上游单次返回多图。

## 查询、明细和下载

```bash
curl https://api.jisudeng.com/v1/images/batches/{id} \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx"

curl https://api.jisudeng.com/v1/images/batches/{id}/items \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx"

curl https://api.jisudeng.com/v1/images/batches/{id}/download \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
  -o batch-results.zip
```

建议轮询间隔：

- `queued`：60-120 秒
- 连续 3 次仍为 `queued`：停止主动高频轮询，保留恢复记录
- `running`：约 60 秒
- `processing_results`：20-45 秒

预览按需加载，不应为列表自动批量下载图片内容。

## 部分失败和重试

- 只重试失败 items
- 不得重复提交已经成功的 `custom_id`
- 重试前再次调用 `/v1/images/batches/models`
- 恢复记录至少保存 batch ID、模型、输出目录、状态/明细/下载 URL、custom ID 到 prompt 的映射
- 恢复记录和日志不得包含 API Key 或参考图 Base64

## 取消、结算和清理

- `POST /v1/images/batches/{id}/cancel` 请求取消上游任务
- 已索引成功的图片按 `output_image_count` 结算；`success_count` 表示成功 item 数，`output_image_count` 表示实际可下载图片数
- 未成功部分释放冻结余额
- `DELETE /outputs` 删除输出但保留任务与结算记录
- `DELETE /{id}` 删除符合状态约束的任务记录
- 输出有保留期，完成后应及时下载 ZIP

## 生产启用

通用默认保持关闭。两阶段上线：

Redis 必须使用持久卷并启用 AOF；官方 Docker Compose 与 Zeabur 模板已配置
`appendonly yes` 和 `appendfsync everysec`。外部 Redis 若没有等价持久化，服务重启
或节点故障时 ready/delayed/active lease 无法保证恢复，不能称为持久 Batch。

1. 先部署代码并设置 `BATCH_IMAGE_QUEUE_ENABLED=true`、`BATCH_IMAGE_ENABLED=false`，让 worker 恢复或排空存量任务。
2. 配置 Gemini API Key 账号、模型映射、分组 `allow_image_generation` / `allow_batch_image_generation` 和图片价格。
3. 使用目标生产 Key 调用 `/v1/images/batches/models`，确认预检通过。
4. 设置 `BATCH_IMAGE_ENABLED=true` 开放新提交。

回滚时先关闭 `BATCH_IMAGE_ENABLED`，保持 queue/worker 排空和结算存量任务；确认无活跃任务和冻结余额后再关闭队列。
