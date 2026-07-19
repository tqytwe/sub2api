# 图片生成 API

> 状态：active
> 网站入口：`https://www.jisudeng.com/docs?cat=deploy&page=text-to-image-api`
> API Key：`https://www.jisudeng.com/keys`
> 模型与价格：`https://www.jisudeng.com/models`
> API 地址：`https://api.jisudeng.com`
> 最后核验：2026-07-20

## 现在怎么用

极速蹬图片生成使用 OpenAI 兼容的 Images API，支持 GPT、Grok、Agnes 和 Gemini 等分组可见的图片模型。短耗时测试可以使用同步接口：

```text
POST https://api.jisudeng.com/v1/images/generations
POST https://api.jisudeng.com/v1/images/edits
```

生产里的单个长耗时图片请求（例如 `gpt-image-2`、大尺寸、预计超过 60-90 秒）
请优先使用 Gateway 单请求异步接口：

```text
POST https://api.jisudeng.com/v1/images/generations/async
POST https://api.jisudeng.com/v1/images/edits/async
GET  https://api.jisudeng.com/v1/images/tasks/{task_id}
```

同步接口经过 CDN/Cloudflare 时可能在上游仍在生成期间收到 `524`。把客户端超时从
180 秒调到 300 秒不能避免中间层超时；上游完成后后台仍可能记录成功和扣费，但
客户端连接已经断开。异步接口会先返回 `202 task_id`，再由客户端轮询结果，适合
生产稳定接入。多个 prompt、需要持久恢复/取消/ZIP 下载的任务使用
`/v1/images/batches`，不要把单请求异步当作 Batch。

可用模型以你的 API Key 所属分组为准，常用模型是：

```text
gpt-image-2
gpt-image-1.5
gpt-image-1
grok-imagine
grok-imagine-image-quality
grok-imagine-image
grok-imagine-edit
agnes-image-2.1-flash
gemini-3.1-flash-image-preview
```

GPT 分组不传 `model` 时默认走 `gpt-image-2`。Grok 分组必须传 `model`，通常直接填 `grok-imagine` 即可，网关会在图像生成端点规范化到 Grok 图片模型。不是所有名字里带 `image` 的模型都一定能提交到 Images API；如果模型属于文本、视频或尚未接入的图片能力，接口会在创建任务前返回 `400 invalid_request_error`。

## 三类返回契约

### 同步 Base64 返回

请求使用 `"response_format": "b64_json"` 时，每张图片位于
`data[].b64_json`。当 `n > 1` 时，`data` 包含实际返回的全部图片，客户端
不得只读取 `data[0]`：

```json
{
  "created": 1784160000,
  "data": [
    { "b64_json": "iVBORw0KGgo..." },
    { "b64_json": "iVBORw0KGgo..." }
  ]
}
```

### 同步 URL 返回

请求使用 `"response_format": "url"` 时，网关先把图片写入私有结果存储，再在
`data[].url` 返回同 API Key 鉴权的临时平台扩展 URL：
`/v1/images/results/{result_id}/{index}`。响应同时包含 `expires_at`。
存储不可用时返回 `503 IMAGE_RESULT_STORAGE_UNAVAILABLE`，不回退伪装成 data URL：

```json
{
  "created": 1784160000,
  "data": [
    {
      "url": "/v1/images/results/imgres_012345/0",
      "expires_at": 1784246400,
      "size": "1254x1254"
    }
  ],
  "requested_size": "1024x1024",
  "model": "gpt-image-2",
  "upstream_model": "gpt-image-2-codex"
}
```

请求 `size` 是生成档位，网关不强制拉伸或裁剪。`requested_size` 保留请求值，
每张 `data[].size` 是实际像素尺寸。

临时 URL 到期后，访问元数据会失效，后台清理器会继续根据独立清理索引删除底层
本地卷或 S3/R2 对象；服务重启不会遗失待清理记录。对象删除失败时保留记录并在
后续周期重试。运维可通过 `IMAGE_STORAGE_CLEANUP_INTERVAL_SECONDS` 调整扫描间隔，
通过 `IMAGE_STORAGE_CLEANUP_BATCH_SIZE` 调整每轮上限，默认分别为 60 秒和 100 条。

流式请求使用 `"stream": true, "response_format": "url"` 时，网关会在连接上游前
预检私有结果存储。`*.partial_image` 事件只携带 `b64_json` 预览，不返回 URL；
`*.completed` 事件写入完整图片后返回 `/v1/images/results/{result_id}/{index}`、
`result_id` 和 `expires_at`，不会返回 data URL 或完整 Base64。存储未就绪时在 SSE
开始前返回 `503 IMAGE_RESULT_STORAGE_UNAVAILABLE`。

### 异步任务返回

长耗时请求应提交到 `/v1/images/generations/async` 或
`/v1/images/edits/async`，先收到 `202` 和 `task_id`，再轮询
`/v1/images/tasks/{task_id}`。queued、processing、completed 和 failed 的完整契约见
[异步图片任务](./ASYNC_IMAGE_TASKS.md)。该能力依赖图片存储、Redis 持久队列和
有界 worker；生产同时设置 `IMAGE_STORAGE_ENABLED=true`、
`IMAGE_ASYNC_QUEUE_ENABLED=true`、`IMAGE_ASYNC_ENABLED=true` 和
`IMAGE_ASYNC_WORKER_COUNT=4`。极速蹬生产结果写入 RustFS/S3 的
`image-task-results/images/`，通过 `https://jisu.zeabur.app` 返回 24 小时预签 URL。

## 准备 API Key

1. 打开 `https://www.jisudeng.com/keys`。
2. 创建或选择一把绑定到 GPT/OpenAI 图像分组或 Grok 图像分组的 API Key。
3. 在 `https://www.jisudeng.com/models` 确认这把 Key 所属分组能看到可提交到 Images API 的图片模型，例如 `gpt-image-*`、`grok-imagine*`、`agnes-image-2.1-flash` 或 `gemini-3.1-flash-image-preview`。
4. 调用时把 Key 放在请求头里，不要放 URL 参数里。

推荐认证头：

```text
Authorization: Bearer sk-xxxxxxxxxxxxxxx
Content-Type: application/json
```

也兼容：

```text
x-api-key: sk-xxxxxxxxxxxxxxx
x-goog-api-key: sk-xxxxxxxxxxxxxxx
```

`?key=` 和 `?api_key=` 已禁用，避免密钥出现在浏览器历史、日志和分享链接里。

## 单张生成

GPT 示例：

```bash
curl https://api.jisudeng.com/v1/images/generations \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "电商白底产品图：一副磨砂黑无线耳机，居中摆放，柔和棚拍光，干净阴影，高清商业摄影",
    "size": "1024x1024",
    "n": 1,
    "response_format": "b64_json"
  }' \
  -o image-response.json
```

保存到本地 PNG：

```bash
jq -r '.data[0].b64_json' image-response.json | base64 -d > gpt-image.png
```

Grok 示例：

```bash
curl https://api.jisudeng.com/v1/images/generations \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-imagine",
    "prompt": "赛博朋克城市夜景，雨后的霓虹街道，电影感构图，高细节",
    "n": 1,
    "response_format": "b64_json"
  }' \
  -o grok-response.json
```

保存：

```bash
jq -r '.data[0].b64_json' grok-response.json | base64 -d > grok-image.png
```

## 一次生成多张

`n` 正式支持 1-10。对不接受上游多图字段的 Responses/Gemini 兼容路径，网关会
删除不兼容字段并执行多个 `n=1` 子请求。`stream=true` 只允许 `n=1`。

把 `n` 改成需要的张数即可，例如一次出 4 张：

```bash
curl https://api.jisudeng.com/v1/images/generations \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "小红书封面：夏日柠檬气泡水，清爽浅色背景，标题留白，真实摄影质感",
    "size": "1024x1536",
    "n": 4,
    "response_format": "b64_json"
  }' \
  -o multi-response.json
```

把返回的所有图片保存到本地：

```bash
mkdir -p outputs
jq -r '.data[].b64_json' multi-response.json | nl -w1 -s' ' | while read -r index b64; do
  printf '%s' "$b64" | base64 -d > "outputs/image-${index}.png"
done
```

部分子请求失败但至少一张成功时，响应包含 `requested_n`、`completed_n` 和
`failed_n`，只按实际成功图片结算。全部失败时返回标准错误。

计费按实际图片张数计算。`n=4` 最多请求 4 张图，不是 1 次请求只算 1 张。

## 多个 prompt 批量生成

短任务可以对同步接口循环提交多个 prompt；需要持久恢复、失败项重试和 ZIP 下载时，
应使用 [Batch Image 持久批任务](./BATCH_IMAGE_API.md)。同步循环示例先准备
`prompts.txt`：

```text
电商白底图：黑色无线耳机，柔光，干净阴影
小红书封面：夏日柠檬气泡水，浅色背景，标题留白
游戏头像：未来感女战士，蓝紫霓虹，半身像
```

批量生成并保存：

```bash
mkdir -p batch-outputs
i=1
while IFS= read -r prompt || [ -n "$prompt" ]; do
  [ -z "$prompt" ] && continue

  jq -n --arg prompt "$prompt" '{
    model: "gpt-image-2",
    prompt: $prompt,
    size: "1024x1024",
    n: 1,
    response_format: "b64_json"
  }' > request.json

  curl https://api.jisudeng.com/v1/images/generations \
    -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
    -H "Content-Type: application/json" \
    -d @request.json \
    -o "batch-outputs/response-${i}.json"

  jq -r '.data[0].b64_json' "batch-outputs/response-${i}.json" \
    | base64 -d > "batch-outputs/image-${i}.png"

  i=$((i + 1))
done < prompts.txt
```

如果这把 Key 属于 Grok 分组，把 `model` 改成 `grok-imagine`。

## prompt 怎么写

`prompt` 就是请求 JSON 里的文本字段：

```json
{
  "model": "gpt-image-2",
  "prompt": "主体：一杯冰美式咖啡。场景：木质桌面、自然光。风格：真实商业摄影。要求：杯身清晰、背景干净、不要文字水印。",
  "size": "1024x1024",
  "n": 1,
  "response_format": "b64_json"
}
```

建议按这四段写：

```text
主体：要画什么
场景：在哪里、什么背景
风格：摄影、插画、3D、海报、国潮、赛博朋克等
要求：尺寸感、留白、不要文字、不要水印、颜色偏好
```

多行 prompt 可以写成 JSON 字符串里的 `\n`：

```json
{
  "prompt": "主体：白色运动鞋\n场景：纯白摄影棚\n风格：电商主图\n要求：居中、干净阴影、不要文字"
}
```

## 直接在网页显示

如果返回 `b64_json`，前端给它加上 `data:image/png;base64,` 就能放进 `<img>`：

```html
<img id="preview" style="max-width: 360px; border-radius: 12px;" />
<script>
async function generateImage() {
  const res = await fetch("https://api.jisudeng.com/v1/images/generations", {
    method: "POST",
    headers: {
      "Authorization": "Bearer sk-xxxxxxxxxxxxxxx",
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      model: "gpt-image-2",
      prompt: "电商白底产品图：一只透明玻璃香水瓶，柔光，高清",
      size: "1024x1024",
      n: 1,
      response_format: "b64_json"
    })
  })
  const json = await res.json()
  const item = json.data[0]
  document.querySelector("#preview").src =
    item.url || `data:image/png;base64,${item.b64_json}`
}
generateImage()
</script>
```

浏览器里直接写 API Key 只适合本地测试。正式网站建议由你的后端调用 `https://api.jisudeng.com/v1/images/generations`，再把图片 URL 或 base64 返回给前端，避免 Key 泄露。

## Node.js 保存到本地

```js
import fs from "node:fs"

const res = await fetch("https://api.jisudeng.com/v1/images/generations", {
  method: "POST",
  headers: {
    Authorization: "Bearer sk-xxxxxxxxxxxxxxx",
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model: "gpt-image-2",
    prompt: "科技产品海报：黑色智能手表，深色背景，边缘光，商业摄影",
    size: "1024x1024",
    n: 2,
    response_format: "b64_json",
  }),
})

const json = await res.json()
json.data.forEach((item, index) => {
  const buffer = Buffer.from(item.b64_json, "base64")
  fs.writeFileSync(`image-${index + 1}.png`, buffer)
})
```

## Python 保存到本地

```python
import base64
import json
import urllib.request

payload = {
    "model": "gpt-image-2",
    "prompt": "国潮插画：红色包装礼盒，祥云纹样，春节氛围，高级质感",
    "size": "1024x1024",
    "n": 2,
    "response_format": "b64_json",
}

req = urllib.request.Request(
    "https://api.jisudeng.com/v1/images/generations",
    data=json.dumps(payload).encode("utf-8"),
    headers={
        "Authorization": "Bearer sk-xxxxxxxxxxxxxxx",
        "Content-Type": "application/json",
    },
    method="POST",
)

with urllib.request.urlopen(req) as resp:
    data = json.loads(resp.read().decode("utf-8"))

for i, item in enumerate(data["data"], start=1):
    with open(f"image-{i}.png", "wb") as f:
        f.write(base64.b64decode(item["b64_json"]))
```

## 图像编辑

已经有公网图片 URL 时：

```bash
curl https://api.jisudeng.com/v1/images/edits \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "把背景替换成明亮的摄影棚渐变背景，保留主体",
    "images": [
      { "image_url": "https://example.com/source.png" }
    ],
    "size": "1024x1024",
    "response_format": "b64_json"
  }'
```

直接上传本地文件时：

```bash
curl https://api.jisudeng.com/v1/images/edits \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
  -F "model=gpt-image-2" \
  -F "prompt=把背景替换成明亮的摄影棚渐变背景，保留主体" \
  -F "image=@source.png;type=image/png" \
  -F "size=1024x1024" \
  -F "response_format=b64_json" \
  -o edit-response.json
```

保存编辑结果：

```bash
jq -r '.data[0].b64_json' edit-response.json | base64 -d > edited.png
```

每个 multipart 文件或文本字段最多 **20 MiB**；整个请求还受部署配置的全局
请求体上限约束。

## 常用字段

| 字段 | 说明 |
|------|------|
| `model` | GPT 用 `gpt-image-2`；Grok 用 `grok-imagine` |
| `prompt` | 图片描述，生成和编辑都放这里 |
| `n` | 输出张数 1-10，默认 1；流式请求只允许 1 |
| `size` | 请求生成档位；响应用 `requested_size` 和每张实际 `size` 区分请求与真实像素 |
| `response_format` | `b64_json` 返回 Base64；`url` 返回同 API Key 鉴权的私有临时 URL |
| `quality` | 支持的上游会透传，如 `standard`、`high` |
| `background` | 支持的上游会透传，如 `auto`、`transparent` |
| `output_format` | 支持的上游会透传，如 `png`、`jpeg`、`webp` |
| `images` | 编辑接口的参考图 URL 数组 |
| `image` / `image[]` | multipart 编辑接口的本地图片字段 |

## 常见错误

| HTTP | 场景 |
|------|------|
| `400 IMAGE_PROMPT_REQUIRED` | prompt 为空白；在选择账号和调用上游前返回 |
| `400 IMAGE_RESPONSE_FORMAT_INVALID` | `response_format` 不是 `b64_json` 或 `url` |
| `400 invalid_request_error` | JSON 写错、`n` 不在 1-10、流式多图、编辑接口缺少图片 |
| `401 API_KEY_REQUIRED / INVALID_API_KEY` | 没传 Key、Key 写错或已失效 |
| `403 permission_error` | Key 所属分组没有开启图片生成 |
| `404 not_found_error` | Key 所属分组不是 GPT/OpenAI 或 Grok 图片分组 |
| `413 invalid_request_error` | multipart 文件或文本字段超过 20 MiB，或整个请求超过部署上限 |
| `429` | Key、用户、分组、账号或上游限流 |
| `502 upstream_error` | 上游异常或没有返回有效图片 |

## 和图像工作室的区别

图像工作室入口是 `https://www.jisudeng.com/image-studio`，适合手动生成和查看图库。开发者 API 调用请使用：

```text
https://api.jisudeng.com/v1/images/generations
https://api.jisudeng.com/v1/images/edits
```

需要多个 prompt 批量生成时，看 [Batch Image 持久批任务](./BATCH_IMAGE_API.md)。
需要避免长连接超时时，看 [异步图片任务](./ASYNC_IMAGE_TASKS.md)。
