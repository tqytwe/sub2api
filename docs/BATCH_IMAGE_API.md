# 多张 / 批量生图调用

> 状态：active
> 网站入口：`https://www.jisudeng.com/docs?cat=tutorial&page=batch-image-api`
> API Key：`https://www.jisudeng.com/keys`
> API 地址：`https://api.jisudeng.com`
> 最后核验：2026-07-16

## 当前口径

极速蹬当前面向用户的图片生成主要是 GPT 图片生成和 Grok 图片生成。批量调用不是换一个新地址，而是继续使用同一个生成接口：

```text
POST https://api.jisudeng.com/v1/images/generations
```

常见两种批量方式：

| 方式 | 怎么做 | 适合场景 |
|------|--------|----------|
| 一次请求多张 | 设置 `n`，例如 `n: 4` | 同一个 prompt 出多张备选图 |
| 多个 prompt 批量跑 | 循环调用 `/v1/images/generations` | 商品图、封面、头像、素材批处理 |

GPT/Grok 图片批量调用主要就是调用 `https://api.jisudeng.com/v1/images/generations`。不要换成其他地址，也不要把 API Key 放到 URL 参数里。

## 一次请求生成多张

```bash
curl https://api.jisudeng.com/v1/images/generations \
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "电商白底主图：一双白色运动鞋，居中摆放，柔和棚拍光，高清真实摄影",
    "size": "1024x1024",
    "n": 4,
    "response_format": "b64_json"
  }' \
  -o multi-response.json
```

保存所有图片：

```bash
mkdir -p outputs
jq -r '.data[].b64_json' multi-response.json | nl -w1 -s' ' | while read -r index b64; do
  printf '%s' "$b64" | base64 -d > "outputs/shoe-${index}.png"
done
```

Grok 分组写法一样，只改模型：

```json
{
  "model": "grok-imagine",
  "prompt": "未来感跑车海报，夜晚城市，霓虹光，高对比，电影质感",
  "n": 4,
  "response_format": "b64_json"
}
```

## 多个 prompt 批量跑

准备 `prompts.txt`，一行一个 prompt：

```text
电商白底主图：白色运动鞋，柔光，干净阴影
电商白底主图：黑色机械键盘，俯拍，高清细节
社媒封面：夏日冰咖啡，浅色背景，标题留白
游戏头像：银发机甲角色，蓝色边缘光，半身像
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
  }' > "batch-outputs/request-${i}.json"

  curl https://api.jisudeng.com/v1/images/generations \
    -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
    -H "Content-Type: application/json" \
    -d @"batch-outputs/request-${i}.json" \
    -o "batch-outputs/response-${i}.json"

  jq -r '.data[0].b64_json' "batch-outputs/response-${i}.json" \
    | base64 -d > "batch-outputs/image-${i}.png"

  i=$((i + 1))
done < prompts.txt
```

要用 Grok，把脚本里的 `model: "gpt-image-2"` 改成：

```text
model: "grok-imagine"
```

## 每个 prompt 生成多张

如果每行 prompt 要出 3 张，把请求里的 `n` 改成 3，并保存每条响应里的所有图片：

```bash
mkdir -p batch-outputs
i=1
while IFS= read -r prompt || [ -n "$prompt" ]; do
  [ -z "$prompt" ] && continue

  jq -n --arg prompt "$prompt" '{
    model: "gpt-image-2",
    prompt: $prompt,
    size: "1024x1024",
    n: 3,
    response_format: "b64_json"
  }' > "batch-outputs/request-${i}.json"

  curl https://api.jisudeng.com/v1/images/generations \
    -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \
    -H "Content-Type: application/json" \
    -d @"batch-outputs/request-${i}.json" \
    -o "batch-outputs/response-${i}.json"

  jq -r '.data[].b64_json' "batch-outputs/response-${i}.json" | nl -w1 -s' ' | while read -r j b64; do
    printf '%s' "$b64" | base64 -d > "batch-outputs/image-${i}-${j}.png"
  done

  i=$((i + 1))
done < prompts.txt
```

## Node.js 批量保存

```js
import fs from "node:fs"

const apiKey = "sk-xxxxxxxxxxxxxxx"
const prompts = [
  "电商白底主图：白色运动鞋，柔光，干净阴影",
  "社媒封面：夏日冰咖啡，浅色背景，标题留白",
  "游戏头像：银发机甲角色，蓝色边缘光，半身像",
]

fs.mkdirSync("batch-outputs", { recursive: true })

for (let i = 0; i < prompts.length; i++) {
  const res = await fetch("https://api.jisudeng.com/v1/images/generations", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${apiKey}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      model: "gpt-image-2",
      prompt: prompts[i],
      size: "1024x1024",
      n: 2,
      response_format: "b64_json",
    }),
  })

  const json = await res.json()
  json.data.forEach((item, j) => {
    fs.writeFileSync(
      `batch-outputs/image-${i + 1}-${j + 1}.png`,
      Buffer.from(item.b64_json, "base64"),
    )
  })
}
```

## 网页批量显示

本地测试时可以把返回的 base64 直接塞进 `<img>`：

```html
<div id="grid" style="display:grid;grid-template-columns:repeat(2,180px);gap:12px;"></div>
<script>
async function run() {
  const res = await fetch("https://api.jisudeng.com/v1/images/generations", {
    method: "POST",
    headers: {
      "Authorization": "Bearer sk-xxxxxxxxxxxxxxx",
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      model: "gpt-image-2",
      prompt: "小红书封面：夏日柠檬气泡水，浅色背景，标题留白",
      size: "1024x1536",
      n: 4,
      response_format: "b64_json"
    })
  })
  const json = await res.json()
  document.querySelector("#grid").innerHTML = json.data.map((item) => {
    const src = item.url || `data:image/png;base64,${item.b64_json}`
    return `<img src="${src}" style="width:180px;border-radius:10px;" />`
  }).join("")
}
run()
</script>
```

正式网站不要把 API Key 暴露在前端，请让你的后端调用极速蹬 API，再把图片结果返回给浏览器。

## 批量调用注意事项

- 每张图都会计费，`n=4` 就是 4 张。
- 大批量脚本建议按顺序跑，遇到 `429` 时等待几秒再重试。
- 图片建议用 `response_format: "b64_json"`，保存和展示都最稳定。
- prompt 中有换行或引号时，用 `jq -n --arg prompt "$prompt"` 生成 JSON，别手拼字符串。
- API Key 只放请求头：`Authorization: Bearer sk-...`。

## 相关页面

- API Key：`https://www.jisudeng.com/keys`
- 模型与价格：`https://www.jisudeng.com/models`
- 单张生成完整说明：[GPT / Grok 图片生成 API](./IMAGE_GENERATION_API.md)
- 图像工作室：`https://www.jisudeng.com/image-studio`
