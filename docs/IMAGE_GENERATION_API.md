# GPT / Grok 图片生成 API

> 状态：active
> 网站入口：`https://www.jisudeng.com/docs?cat=deploy&page=text-to-image-api`
> API Key：`https://www.jisudeng.com/keys`
> 模型与价格：`https://www.jisudeng.com/models`
> API 地址：`https://api.jisudeng.com`
> 最后核验：2026-07-16

## 现在怎么用

极速蹬当前对外主推的是 GPT 图片生成和 Grok 图片生成，开发者直接调用 OpenAI 兼容的 Images API：

```text
POST https://api.jisudeng.com/v1/images/generations
POST https://api.jisudeng.com/v1/images/edits
```

可用模型以你的 API Key 所属分组为准，常用模型是：

```text
gpt-image-2
gpt-image-1.5
gpt-image-1
grok-imagine
grok-imagine-image-quality
grok-imagine-image
grok-imagine-edit
```

GPT 分组不传 `model` 时默认走 `gpt-image-2`。Grok 分组必须传 `model`，通常直接填 `grok-imagine` 即可，网关会在图像生成端点规范化到 Grok 图片模型。

## 准备 API Key

1. 打开 `https://www.jisudeng.com/keys`。
2. 创建或选择一把绑定到 GPT/OpenAI 图像分组或 Grok 图像分组的 API Key。
3. 在 `https://www.jisudeng.com/models` 确认这把 Key 所属分组能看到 `gpt-image-*` 或 `grok-imagine*` 图片模型。
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

计费按实际图片张数计算。`n=4` 就是 4 张图，不是 1 次请求只算 1 张。

## 多个 prompt 批量生成

当前 GPT/Grok 图片生成的批量用法，是对同一个接口循环提交多个 prompt。先准备 `prompts.txt`：

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

## 常用字段

| 字段 | 说明 |
|------|------|
| `model` | GPT 用 `gpt-image-2`；Grok 用 `grok-imagine` |
| `prompt` | 图片描述，生成和编辑都放这里 |
| `n` | 输出张数，默认 1；多张就填 2、4 等 |
| `size` | GPT 常用 `1024x1024`、`1024x1536`、`1536x1024`；Grok 会用于站内计费分档 |
| `response_format` | 推荐 `b64_json`，方便保存和直接显示 |
| `quality` | 支持的上游会透传，如 `standard`、`high` |
| `background` | 支持的上游会透传，如 `auto`、`transparent` |
| `output_format` | 支持的上游会透传，如 `png`、`jpeg`、`webp` |
| `images` | 编辑接口的参考图 URL 数组 |
| `image` / `image[]` | multipart 编辑接口的本地图片字段 |

## 常见错误

| HTTP | 场景 |
|------|------|
| `400 invalid_request_error` | JSON 写错、请求体为空、`n <= 0`、编辑接口缺少图片 |
| `401 API_KEY_REQUIRED / INVALID_API_KEY` | 没传 Key、Key 写错或已失效 |
| `403 permission_error` | Key 所属分组没有开启图片生成 |
| `404 not_found_error` | Key 所属分组不是 GPT/OpenAI 或 Grok 图片分组 |
| `413 invalid_request_error` | 上传图片或请求体太大 |
| `429` | Key、用户、分组、账号或上游限流 |
| `502 upstream_error` | 上游异常或没有返回有效图片 |

## 和图像工作室的区别

图像工作室入口是 `https://www.jisudeng.com/image-studio`，适合手动生成和查看图库。开发者 API 调用请使用：

```text
https://api.jisudeng.com/v1/images/generations
https://api.jisudeng.com/v1/images/edits
```

需要多个 prompt 批量生成时，看 [多张 / 批量生图调用](./BATCH_IMAGE_API.md)。
