import { getToolGuide } from './public-docs-tool-guides'

/** Public /docs content — adapted for 极速蹬 (gateway docs). */
export interface PublicDocPageContent {
  id: string
  title: string
  summary: string
  html: string
}

export interface PublicDocCategoryContent {
  id: string
  title: string
  description: string
  pages: PublicDocPageContent[]
}

const PUBLIC_DOC_CONTENT_ZH_SOURCE: PublicDocCategoryContent[] = [
  {
    id: 'tutorial',
    title: "使用教程",
    description: "从零上手 · API Key 创建 · 调用示例",
    pages: [
      {
        id: 'quick-start',
        title: "快速开始",
        summary: "5 分钟创建第一个 API Key 并发起请求",
        html: `<p class="docs-lead">本教程帮助你在 5 分钟内完成账号注册、API Key 创建并发起第一次模型调用。</p>
<h2>第 1 步 · 注册账号</h2>
<p>有账号请先<a href="/login">登录</a>，没有账号则前往<a href="/register">注册</a>，按页面提供的方式完成验证。</p>
<h2>第 2 步 · 充值（可选）</h2>
<p>如果选用按余额计费的分组，请先确认余额是否足够；需要补充时，到<a href="/purchase">充值</a>页面查看可用支付方式和预计到账金额。</p>
<h2>第 3 步 · 创建 API Key</h2>
<p>打开 <a href="/keys">API 密钥</a>，点击「创建密钥」，填写名称并选择支持目标模型和协议的分组。保存后，从列表对应行复制密钥。</p>
<h2>第 4 步 · 发起请求</h2>
<p>先在<a href="/catalog">模型与价格</a>核对分组、协议和模型 ID。按照该模型支持的接口选一条命令，替换 <code>YOUR_API_KEY</code> 和 <code>YOUR_MODEL_ID</code>。</p>
<h3>OpenAI 兼容（推荐通用 SDK）</h3>
<pre><code>curl https://your-host/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"YOUR_MODEL_ID","messages":[{"role":"user","content":"hi"}]}'</code></pre>
<h3>Anthropic Messages API</h3>
<pre><code>curl https://your-host/v1/messages \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{"model":"YOUR_MODEL_ID","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}'</code></pre>
<h2>第 5 步 · 核对结果</h2>
<p>收到响应后，在<a href="/usage">用量记录</a>按密钥和时间核对模型与消耗。请求失败时，先核对密钥状态、分组权限、模型 ID 和接口路径。</p>`,
      },
      {
        id: 'text-to-image-api',
        title: "图片生成 API",
        summary: "真实 API 地址、单张/多张生成、prompt、保存与网页显示",
        html: `<p class="docs-lead">极速蹬图片生成使用 OpenAI 兼容的 Images API，支持 GPT、Grok、Agnes 和 Gemini 等分组可见的图片模型。短耗时测试可直接调用同步 Images API；生产长耗时生成建议使用异步任务，避免同步长连接 524。</p>
<pre><code>POST https://api.jisudeng.com/v1/images/generations
POST https://api.jisudeng.com/v1/images/edits
POST https://api.jisudeng.com/v1/images/generations/async
GET  https://api.jisudeng.com/v1/images/tasks/{task_id}</code></pre>

<h2>真实入口</h2>
<ul>
  <li><a href="/keys">API 密钥</a></li>
  <li><a href="/catalog">模型与价格</a></li>
  <li>生成接口：<code>POST https://api.jisudeng.com/v1/images/generations</code></li>
  <li>编辑接口：<code>POST https://api.jisudeng.com/v1/images/edits</code></li>
</ul>

<h2>鉴权</h2>
<pre><code>Authorization: Bearer sk-xxxxxxxxxxxxxxx
Content-Type: application/json</code></pre>
<p>也兼容 <code>x-api-key</code> 和 <code>x-goog-api-key</code>。不要把 Key 放到 URL 的 <code>key</code> / <code>api_key</code> 参数里。</p>

	<h2>选择图片模型</h2>
	<p>先在 <a href="/catalog">模型与价格</a>中选择与 API Key 相同的分组，再确认模型明确支持 Images API。把示例中的 <code>YOUR_IMAGE_MODEL_ID</code> 替换为页面显示的精确 ID；模型名称、默认值和分组权限会变化，本页不维护静态清单。</p>
	<p>不是所有名字里带 image 的模型都能提交到 Images API。若接口或分组不匹配，请以模型页当前能力说明及接口返回的错误为准。</p>

<p class="docs-tip">单个大尺寸请求或预计超过 60-90 秒的生成请优先使用 Gateway 单请求异步；多个 prompt 的持久任务请使用 Batch Image。同步接口经过 CDN/Cloudflare 时可能在上游仍在生成期间收到 <code>524</code>；后台可能已经生成成功并计费，但调用方连接已经断开。</p>

<h2>三类返回契约</h2>
<h3>同步 Base64 返回</h3>
<p>请求使用 <code>"response_format": "b64_json"</code> 时，每张图片位于 <code>data[].b64_json</code>。设置 <code>n</code> 大于 1 时，<code>data</code> 会包含实际返回的全部图片，不要只读取第 1 项。</p>
<pre><code>{
  "created": 1784160000,
  "data": [
    {"b64_json": "iVBORw0KGgo..."},
    {"b64_json": "iVBORw0KGgo..."}
  ]
}</code></pre>

<h3>同步 URL 返回</h3>
<p>请求使用 <code>"response_format": "url"</code> 时，每张图片位于 <code>data[].url</code>。网关会先把图片写入私有结果存储，再返回同 API Key 鉴权的临时平台扩展 URL：<code>/v1/images/results/{result_id}/{index}</code> 和 <code>expires_at</code>。它不是 data URL，也不是永久公网地址；存储不可用时返回 <code>503 IMAGE_RESULT_STORAGE_UNAVAILABLE</code>。</p>
<p>流式 URL 请求会在 SSE 开始前预检结果存储：<code>*.partial_image</code> 只返回 <code>b64_json</code> 预览，<code>*.completed</code> 才返回临时 URL、<code>result_id</code> 和 <code>expires_at</code>，不会把 data URL 当作最终 URL。</p>
<pre><code>{
  "created": 1784160000,
  "data": [
    {
      "url": "/v1/images/results/imgres_012345/0",
      "expires_at": 1784246400,
      "size": "1254x1254"
    }
  ],
  "requested_size": "1024x1024",
  "model": "YOUR_IMAGE_MODEL_ID",
  "upstream_model": "EXAMPLE_UPSTREAM_MODEL_ID"
}</code></pre>
<p>必须使用生成请求的同一把 API Key 下载临时 URL。请求 <code>size</code> 是生成档位；网关不会强制拉伸或裁剪，<code>requested_size</code> 保留请求值，每张 <code>data[].size</code> 是实际像素尺寸。</p>

<h3>多图部分成功</h3>
<p><code>n</code> 正式支持 1-10。非流式多图会按上游能力原生提交或拆成多个 <code>n=1</code> 子请求；至少一张成功时返回成功图片和完成统计，只按实际成功结果结算：</p>
<pre><code>{
  "requested_n": 4,
  "completed_n": 3,
  "failed_n": 1,
  "data": [
    {"b64_json": "...", "size": "1254x1254"}
  ]
}</code></pre>
<p><code>stream=true</code> 仅支持 <code>n=1</code>；<code>stream=true &amp;&amp; n&gt;1</code> 返回 400。</p>

<h3>异步任务返回</h3>
<p>长耗时请求应提交到 <code>/v1/images/generations/async</code> 或 <code>/v1/images/edits/async</code>，先收到 <code>202</code> 和 <code>task_id</code>，再轮询 <code>/v1/images/tasks/{task_id}</code>。请求会加密写入 Redis 持久队列，由有界 worker 处理；重启后只继续尚未调用 provider 的 queued 任务，processing 任务不会自动重放。完整契约见 <a href="/docs?cat=deploy&amp;page=async-image-tasks">异步图片任务</a>。</p>

<h2>单张生成</h2>
<pre><code>curl https://api.jisudeng.com/v1/images/generations \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "YOUR_IMAGE_MODEL_ID",
    "prompt": "电商白底产品图：一副磨砂黑无线耳机，居中摆放，柔和棚拍光，干净阴影，高清商业摄影",
    "size": "1024x1024",
    "n": 1,
    "response_format": "b64_json"
  }' \\
  -o image-response.json</code></pre>
<p>保存到本地 PNG：</p>
<pre><code>jq -r '.data[0].b64_json' image-response.json | base64 -d &gt; gpt-image.png</code></pre>

<h2>Grok 生成</h2>
<pre><code>curl https://api.jisudeng.com/v1/images/generations \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "grok-imagine",
    "prompt": "赛博朋克城市夜景，雨后的霓虹街道，电影感构图，高细节",
    "n": 1,
    "response_format": "b64_json"
  }' \\
  -o grok-response.json</code></pre>
<pre><code>jq -r '.data[0].b64_json' grok-response.json | base64 -d &gt; grok-image.png</code></pre>

<h2>一次生成多张</h2>
<p>把 <code>n</code> 改成需要的张数，例如一次出 4 张：</p>
<pre><code>curl https://api.jisudeng.com/v1/images/generations \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "YOUR_IMAGE_MODEL_ID",
    "prompt": "小红书封面：夏日柠檬气泡水，清爽浅色背景，标题留白，真实摄影质感",
    "size": "1024x1536",
    "n": 4,
    "response_format": "b64_json"
  }' \\
  -o multi-response.json</code></pre>
<p>保存返回的所有图片：</p>
<pre><code>mkdir -p outputs
jq -r '.data[].b64_json' multi-response.json | nl -w1 -s' ' | while read -r index b64; do
  printf '%s' "$b64" | base64 -d &gt; "outputs/image-$index.png"
done</code></pre>
<p class="docs-tip"><code>n=4</code> 就是 4 张图，会按 4 张计费。</p>

<h2>prompt 怎么加</h2>
<p><code>prompt</code> 就是请求 JSON 里的描述字段。建议按「主体 / 场景 / 风格 / 要求」写：</p>
<pre><code>{
  "model": "YOUR_IMAGE_MODEL_ID",
  "prompt": "主体：一杯冰美式咖啡。场景：木质桌面、自然光。风格：真实商业摄影。要求：杯身清晰、背景干净、不要文字水印。",
  "size": "1024x1024",
  "n": 1,
  "response_format": "b64_json"
}</code></pre>
<p>多行 prompt 用 <code>\\n</code>：</p>
<pre><code>{
  "prompt": "主体：白色运动鞋\\n场景：纯白摄影棚\\n风格：电商主图\\n要求：居中、干净阴影、不要文字"
}</code></pre>

<h2>网页直接显示</h2>
<p>返回 <code>b64_json</code> 时，加上 <code>data:image/png;base64,</code> 就能塞进 <code>&lt;img&gt;</code>：</p>
<pre><code>&lt;img id="preview" style="max-width:360px;border-radius:12px;" /&gt;
&lt;script&gt;
async function generateImage() {
  const res = await fetch("https://api.jisudeng.com/v1/images/generations", {
    method: "POST",
    headers: {
      "Authorization": "Bearer sk-xxxxxxxxxxxxxxx",
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      model: "YOUR_IMAGE_MODEL_ID",
      prompt: "电商白底产品图：一只透明玻璃香水瓶，柔光，高清",
      size: "1024x1024",
      n: 1,
      response_format: "b64_json"
    })
  })
  const json = await res.json()
  const item = json.data[0]
  document.querySelector("#preview").src = item.url || ("data:image/png;base64," + item.b64_json)
}
generateImage()
&lt;/script&gt;</code></pre>
<p class="docs-tip">浏览器直接写 API Key 只适合本地测试。正式网站应由你的后端调用极速蹬 API，再把图片结果返回给前端。</p>

<h2>Node.js 保存</h2>
<pre><code>import fs from "node:fs"

const res = await fetch("https://api.jisudeng.com/v1/images/generations", {
  method: "POST",
  headers: {
    Authorization: "Bearer sk-xxxxxxxxxxxxxxx",
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model: "YOUR_IMAGE_MODEL_ID",
    prompt: "科技产品海报：黑色智能手表，深色背景，边缘光，商业摄影",
    size: "1024x1024",
    n: 2,
    response_format: "b64_json",
  }),
})

const json = await res.json()
json.data.forEach((item, index) =&gt; {
  fs.writeFileSync("image-" + (index + 1) + ".png", Buffer.from(item.b64_json, "base64"))
})</code></pre>

<h2>图像编辑</h2>
<p>如果图片已经有公网 URL，可以用 JSON：</p>
<pre><code>curl https://api.jisudeng.com/v1/images/edits \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "YOUR_IMAGE_MODEL_ID",
    "prompt": "把背景替换成明亮的摄影棚渐变背景，保留主体",
    "images": [{"image_url": "https://example.com/source.png"}],
    "size": "1024x1024",
    "response_format": "b64_json"
  }'</code></pre>
<p>如果要直接上传文件，用 multipart：</p>
<pre><code>curl https://api.jisudeng.com/v1/images/edits \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -F "model=YOUR_IMAGE_MODEL_ID" \\
  -F "prompt=把背景替换成明亮的摄影棚渐变背景，保留主体" \\
  -F "image=@source.png;type=image/png" \\
  -F "size=1024x1024" \\
  -F "response_format=b64_json" \\
  -o edit-response.json</code></pre>
<pre><code>jq -r '.data[0].b64_json' edit-response.json | base64 -d &gt; edited.png</code></pre>
<p class="docs-tip">每个 multipart 文件或文本字段最多 <strong>20 MiB</strong>；整个请求还受部署配置的全局请求体上限约束。</p>

<h2>常用字段</h2>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>字段</th><th>说明</th></tr></thead>
<tbody>
<tr><td><code>model</code></td><td>填写模型与价格页中当前分组明确支持 Images API 的精确模型 ID</td></tr>
<tr><td><code>prompt</code></td><td>图片描述，生成和编辑都放这里</td></tr>
<tr><td><code>n</code></td><td>输出张数 1-10，默认 1；流式请求只允许 1</td></tr>
<tr><td><code>size</code></td><td>请求生成档位，不保证强制输出像素；响应看 <code>requested_size</code> 与每张实际 <code>size</code></td></tr>
<tr><td><code>response_format</code></td><td><code>b64_json</code> 返回 Base64；<code>url</code> 返回平台私有临时 URL</td></tr>
<tr><td><code>images</code></td><td>编辑接口的参考图 URL 数组</td></tr>
<tr><td><code>image</code> / <code>image[]</code></td><td>multipart 编辑接口的本地图片字段</td></tr>
</tbody>
</table>
</div>

<h2>常见错误</h2>
<ul>
  <li><code>400 IMAGE_PROMPT_REQUIRED</code> — prompt 为空白，在选账号和调用上游前返回</li>
  <li><code>400 IMAGE_RESPONSE_FORMAT_INVALID</code> — <code>response_format</code> 不是 <code>b64_json</code> 或 <code>url</code></li>
  <li><code>400 invalid_request_error</code> — JSON 写错、<code>n</code> 不在 1-10、流式多图、编辑接口缺少图片</li>
  <li><code>401 API_KEY_REQUIRED / INVALID_API_KEY</code> — Key 缺失或无效</li>
  <li><code>403 permission_error</code> — Key 所属分组没有开启图片生成</li>
  <li><code>404 not_found_error</code> — Key 所属分组不是 GPT/OpenAI 或 Grok 图片分组</li>
  <li><code>413 invalid_request_error</code> — multipart 文件或文本字段超过 20 MiB，或整个请求超过部署上限</li>
  <li><code>429</code> — Key、用户、分组、账号或上游限流</li>
  <li><code>502 upstream_error</code> — 上游异常或没有返回有效图片</li>
</ul>
<p class="docs-tip">JSON 请求会兼容剥离可选 UTF-8 BOM。Windows PowerShell 写出的 BOM JSON 可以直接用 <code>curl --data-binary @request.json</code> 提交，但仍建议保存为 UTF-8 without BOM，便于其他工具链处理。</p>

<h2>创作入口与 API 的区别</h2>
<ul>
  <li><strong>API 生成</strong>：开发者直调 <code>/v1/images/generations</code>，Base URL 是 <code>https://api.jisudeng.com</code>，自己保存和展示图片。</li>
  <li><strong>AI创作空间</strong>：打开 <a href="/ai-creation-space">/ai-creation-space</a> 进入统一创作入口；旧的 <code>/ai</code> 与 <code>/image-studio</code> 网页地址会自动跳转。</li>
  <li><strong>一次多张</strong>：一次多张使用同步 Images 的 <code>n=1-10</code>；流式只支持 <code>n=1</code>。</li>
  <li><strong>多个 prompt</strong>：多个 prompt 使用 Batch Image，见 <a href="/docs?cat=deploy&amp;page=batch-image-api">Batch Image 持久批任务</a>。</li>
  <li><strong>单请求异步</strong>：避免一个 Images 请求占用长连接，见 <a href="/docs?cat=deploy&amp;page=async-image-tasks">异步图片任务</a>。</li>
</ul>`,
      },
      {
        id: 'batch-image-api',
        title: "Batch Image 持久批任务",
        summary: "Gemini 多 prompt 批任务：预检、202 提交、轮询、失败重试和 ZIP 下载",
        html: `<p class="docs-lead">Batch Image 用于多个 prompt、可恢复、可取消、可下载的持久批任务。首发通道是 Gemini API Batch；它和同步 Images 的 <code>n</code>、单请求异步 <code>/generations/async</code> 是三套不同能力。</p>
<pre class="docs-endpoint-list"><code>GET  https://api.jisudeng.com/v1/images/batches/models
POST https://api.jisudeng.com/v1/images/batches
GET  https://api.jisudeng.com/v1/images/batches
GET  https://api.jisudeng.com/v1/images/batches/{id}
GET  https://api.jisudeng.com/v1/images/batches/{id}/items
GET  https://api.jisudeng.com/v1/images/batches/{id}/items/{custom_id}/content
GET  https://api.jisudeng.com/v1/images/batches/{id}/download
POST https://api.jisudeng.com/v1/images/batches/{id}/cancel
DELETE https://api.jisudeng.com/v1/images/batches/{id}/outputs
DELETE https://api.jisudeng.com/v1/images/batches/{id}</code></pre>

<h2>先做模型预检</h2>
<p>每次创建任务前，必须使用准备提交的同一把 API Key 调用：</p>
<pre><code>curl https://api.jisudeng.com/v1/images/batches/models \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx"</code></pre>
<p>只有预检返回的模型才可提交。把示例中的 <code>YOUR_BATCH_MODEL_ID</code> 换成预检返回的精确 ID。预检会检查平台开关、队列、分组权限、可调度账号、模型映射和图片价格。</p>
<ul>
  <li><code>BATCH_IMAGE_DISABLED</code>：平台暂不接受新 Batch 提交；历史列表、预览和下载仍可读取。</li>
  <li><code>BATCH_IMAGE_NOT_READY</code>：Redis、队列或 worker 运行时异常，稍后重试或联系管理员。</li>
  <li>分组未开、无账号、无模型、缺少价格：按错误信息修正 Key 所属分组或管理员配置。</li>
</ul>

<h2>提交 5 张</h2>
<p>生成 5 张时使用 <strong>5 个独立 items</strong>。每个 item 的 <code>custom_id</code> 必须唯一：</p>
<pre><code>curl -i https://api.jisudeng.com/v1/images/batches \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: batch-product-20260718-001" \\
  -d '{
    "model": "YOUR_BATCH_MODEL_ID",
    "task_name": "商品主图 5 张",
    "image_size": "1K",
    "response_mime_type": "image/png",
    "items": [
      {"custom_id":"img_001","prompt":"白色运动鞋，电商白底主图，柔光"},
      {"custom_id":"img_002","prompt":"黑色机械键盘，俯拍，高清细节"},
      {"custom_id":"img_003","prompt":"透明香水瓶，棚拍，干净阴影"},
      {"custom_id":"img_004","prompt":"银色智能手表，深色背景，边缘光"},
      {"custom_id":"img_005","prompt":"夏日冰咖啡，浅色社媒封面，标题留白"}
    ]
  }'</code></pre>
<p>成功返回 <code>202 Accepted</code>，响应头含 <code>Location</code> 和 <code>Retry-After</code>。网络超时后重试同一请求必须复用原 <code>Idempotency-Key</code>；请求内容改变时使用新 key。</p>

<h2>提交 10 张</h2>
<p>生成 10 张同样使用 <strong>10 个独立 items</strong>。可以用 <code>jq</code> 从 prompt 文件构造：</p>
<pre><code>jq -Rn '
  [inputs | select(length &gt; 0)]
  | to_entries
  | {
      model: "YOUR_BATCH_MODEL_ID",
      task_name: "批量素材 10 张",
      image_size: "1K",
      response_mime_type: "image/png",
      items: map({
        custom_id: ("img_" + (.key + 1 | tostring)),
        prompt: .value
      })
    }
' prompts-10.txt &gt; batch-10.json

curl -i https://api.jisudeng.com/v1/images/batches \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: batch-10-20260718-001" \\
  --data-binary @batch-10.json</code></pre>
<p class="docs-tip"><code>output_count</code> 表示同一 prompt 重复生成几张，默认 1，<strong>单 item 最多 4</strong>；每任务最多 200 张。不要用单 item 的 <code>output_count=5</code> 或 <code>output_count=10</code>。</p>

<h2>轮询与下载</h2>
<pre><code>curl https://api.jisudeng.com/v1/images/batches/{id} \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx"

curl https://api.jisudeng.com/v1/images/batches/{id}/items \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx"

curl https://api.jisudeng.com/v1/images/batches/{id}/download \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -o batch-results.zip</code></pre>
<p>queued 状态建议 60-120 秒轮询一次，running 状态约 60 秒一次，接近完成时可缩短到 20-45 秒。只重试失败 items，不要重新提交已经成功的 custom_id。</p>

<h2>取消、清理与保留</h2>
<ul>
  <li>取消：<code>POST /v1/images/batches/{id}/cancel</code>。已索引成功的图片按 <code>output_image_count</code> 结算，其余冻结金额释放；<code>success_count</code> 表示成功 item 数。</li>
  <li>删除输出：<code>DELETE /v1/images/batches/{id}/outputs</code>，保留任务记录和结算信息。</li>
  <li>删除记录：<code>DELETE /v1/images/batches/{id}</code>，只允许删除符合状态约束的任务。</li>
  <li>输出有保留期；完成后应及时下载 ZIP，不要把平台临时结果当永久对象存储。</li>
</ul>

<h2>四条产品边界</h2>
<ul>
  <li><strong>同步 Images</strong>：API Key，短请求，<code>/v1/images/generations</code> 和 <code>/v1/images/edits</code>。</li>
  <li><strong>Gateway 单请求异步</strong>：API Key，一个 Images 请求进入 Redis 队列，使用 <code>/generations/async</code> 或 <code>/edits/async</code>。</li>
  <li><strong>Batch Image</strong>：API Key，多个 prompt 的持久批任务，使用 <code>/v1/images/batches</code>。</li>
  <li><strong>AI创作空间</strong>：登录后进入统一画布与创作工作台；账号、模型权限、计费和异步任务仍由主平台统一管理。</li>
</ul>`,
      },
      {
        id: 'async-image-tasks',
        title: "异步图片任务",
        summary: "202 提交、任务轮询、结果存储 URL 与失败返回契约",
        html: `<p class="docs-lead">异步图片任务适合耗时较长的 GPT、Grok、Agnes、Gemini 生成与编辑请求。提交接口立即返回任务 ID，客户端按建议间隔轮询，不需要保持一条长 HTTP 连接。</p>
<pre class="docs-endpoint-list"><code>POST https://api.jisudeng.com/v1/images/generations/async
POST https://api.jisudeng.com/v1/images/edits/async
GET  https://api.jisudeng.com/v1/images/tasks/{task_id}</code></pre>
	<p class="docs-tip">异步任务使用加密请求信封、Redis 持久队列、服务端 S3/RustFS 结果存储和有界 worker。生产通过 <code>https://api.jisudeng.com</code> 接收任务，并由 API 返回短期签名下载 URL；内部 bucket、endpoint 和访问密钥不会公开。API 关闭时返回 404；API 已开但 Redis/worker 未就绪时返回 <code>503 IMAGE_ASYNC_NOT_READY</code>，不会创建任务。</p>
	<h2>生产存储配置</h2>
	<p>极速蹬当前生产异步生图使用 RustFS/S3 后端，关键环境变量如下。访问密钥只配置在服务端环境变量中，不会写入文档、客户端代码或请求示例。</p>
	<pre><code>IMAGE_STORAGE_ENABLED=true
IMAGE_STORAGE_BACKEND=s3
# 生产 endpoint 仅配置在服务端，不对外公开
IMAGE_STORAGE_BUCKET=image-task-results
IMAGE_STORAGE_PREFIX=images/
IMAGE_STORAGE_FORCE_PATH_STYLE=true
IMAGE_STORAGE_REGION=us-east-1
IMAGE_STORAGE_PRESIGN_EXPIRY_HOURS=24
IMAGE_STORAGE_MAX_DOWNLOAD_BYTES=33554432

IMAGE_ASYNC_QUEUE_ENABLED=true
IMAGE_ASYNC_ENABLED=true
IMAGE_ASYNC_WORKER_COUNT=20</code></pre>
	<p class="docs-tip">异步结果有两层 24 小时边界：预签 URL 24 小时过期，RustFS 生命周期规则按 1 天清理 <code>image-task-results</code> bucket 内对象。结果 URL 是临时交付地址，不要当作永久对象存储。</p>

	<h2>提交任务</h2>
	<p>请求体与同步生成或编辑接口相同。提交成功返回 <code>202 Accepted</code>，响应体里的 <code>poll_url</code> 是稳定轮询地址；标准响应也会设置 <code>Location</code> 和建议轮询间隔 <code>Retry-After: 3</code>，如果客户端或代理层未暴露这些响应头，直接按 <code>poll_url</code> 轮询即可。首次提交建议携带 <code>Idempotency-Key</code>；网络失败重试同一请求时复用该 key。Key 最多 255 字节，超长返回 <code>400 IMAGE_TASK_IDEMPOTENCY_KEY_INVALID</code>，不会创建任务或入队。</p>
<pre><code>curl -i https://api.jisudeng.com/v1/images/generations/async \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: image-task-20260718-001" \\
  -d '{
    "model": "YOUR_IMAGE_MODEL_ID",
    "prompt": "暴风雪中的灯塔，电影感，横向构图",
    "size": "1536x1024",
    "n": 2
  }'</code></pre>
<pre><code>{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "queued",
  "created_at": 1784160000,
  "expires_at": 1784246400,
  "poll_url": "/v1/images/tasks/imgtask_0123456789abcdef"
}</code></pre>
<p class="docs-tip">同步接口的请求校验契约同样适用于异步提交：空白 prompt 返回 <code>400 IMAGE_PROMPT_REQUIRED</code>，非法 <code>response_format</code> 返回 <code>400 IMAGE_RESPONSE_FORMAT_INVALID</code>，且都发生在任务创建和入队之前。</p>

<h2>轮询任务</h2>
<p>必须使用提交任务时的同一把 API Key：</p>
<pre><code>curl https://api.jisudeng.com/v1/images/tasks/imgtask_0123456789abcdef \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx"</code></pre>

<h3>排队中</h3>
<pre><code>{
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "queued",
  "created_at": 1784160000,
  "expires_at": 1784246400
}</code></pre>

<h3>处理中</h3>
<pre><code>{
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "processing",
  "created_at": 1784160000,
  "expires_at": 1784246400
}</code></pre>

<h3>已完成</h3>
	<p>生成结果会先转存到结果存储；本地部署默认返回受 API Key 鉴权保护的 <code>/v1/images/task-assets/...</code> 地址，极速蹬生产返回 RustFS/S3 预签 URL，例如 <code>https://jisu.zeabur.app/image-task-results/images/imgtask_0123456789abcdef-0.png</code> 后跟签名查询参数。<code>image_url</code> 是第一张图片的便捷字段。异步结果不会把大段 <code>b64_json</code> 存入任务记录。</p>
<pre><code>{
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
  "created_at": 1784160000,
  "completed_at": 1784160123,
  "expires_at": 1784246523
}</code></pre>

<h3>失败</h3>
<pre><code>{
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "failed",
  "http_status": 502,
  "error": {
    "type": "api_error",
    "message": "Upstream request failed"
  },
  "created_at": 1784160000,
  "completed_at": 1784160123,
  "expires_at": 1784246523
}</code></pre>
<p>queued 任务会在服务重启后继续处理；崩溃前已经进入 processing 的任务会转为 failed，并返回 <code>IMAGE_TASK_RECOVERY_UNAVAILABLE</code>。平台不会重放可能已经计费的上游请求。</p>
<p>成功提交和成功轮询响应都带 <code>Cache-Control: no-store</code>。未知任务 ID 与其他 API Key 拥有的任务都返回 404，避免泄露任务是否存在。</p>`,
      },
      {
        id: 'api-key',
        title: "API Key 管理",
        summary: "创建、复制、停用和设置密钥限额",
        html: `<p class="docs-lead">API 密钥是访问网关的凭据。可用模型、费率和限制取决于密钥所绑定的分组。</p>
<h2>创建 Key</h2>
<p>进入 <a href="/keys">API 密钥</a> →「创建密钥」，填名称并选择主分组。若需要且页面提供适用分组，可在「附加分组」中选择；首次接入建议先验证一个分组。保存后使用密钥行内的复制按钮。</p>
<figure class="docs-figure"><img src="/doc-assets/integrations/create-api-key-zh.png" alt="创建密钥表单，已填写名称并选择主分组" width="512" height="824" loading="lazy" /><figcaption>填写名称、选择分组后点「创建」。图中为演示分组。</figcaption></figure>
<h2>启用 / 禁用</h2>
<p>在列表中切换目标密钥的状态。更换生产密钥时，先验证新密钥并更新应用，再禁用或删除旧密钥。</p>
<h2>并发与消费限额</h2>
<p>密钥表单可设置额外的并发上限与消费限额。5 小时、1 天、7 天限额按消费金额计算。将单项设为 0 代表不加该项密钥限制，账号和分组限制仍然适用。</p>
<h2>安全建议</h2>
<ul>
  <li>不要把 Key 提交到 git 仓库</li>
  <li>可疑泄露时立即删除并新建</li>
  <li>生产环境与测试环境分别建 Key</li>
</ul>`,
      },
      {
        id: 'concurrency',
        title: "并发与限流",
        summary: "账号 / Key / 分组 三级限流如何叠加",
        html: `<p class="docs-lead">并发表示同时进行的请求数，RPM 表示每分钟请求数，消费限额按金额计算。一次请求可能同时受多项限制。</p>
<h2>账号并发</h2>
<p>以账号实际分配的并发额度为准。在<a href="/keys">API 密钥</a>列表查看密钥的并发与等待状态。</p>
<h2>密钥限额</h2>
<p>用密钥编辑页核对额外并发上限和 5 小时、1 天、7 天消费限额。是否等待及等待上限以列表显示的统一排队设置为准。</p>
<h2>分组限流</h2>
<p>分组或上游也可能限制请求。返回错误时先核对具体原因，降低同时发出的请求数量，并检查当前余额及消费窗口。</p>
<h2>命中限流如何处理?</h2>
<pre><code>HTTP/1.1 429 Too Many Requests
{"error":{"type":"rate_limit_exceeded","message":"..."}}</code></pre>
<p>短时限流可按 <code>Retry-After</code>（如有）或采用有上限的指数退避。额度耗尽时先处理额度；创建异步任务时按接口说明复用原幂等键，避免重复提交。</p>`,
      },
    ],
  },
  {
    id: 'recharge-vip',
    title: "充值与 VIP",
    description: "VIP 分级 · 充值到账加赠 · 充值流程 · 签到奖励",
    pages: [
      {
        id: 'vip-levels',
        title: "VIP 分级体系",
        summary: "查看当前会员等级、升级门槛与充值加赠",
        html: `<p class="docs-lead">VIP 按符合条件的充值净额计算。当前档位、升级门槛和加赠比例以本页实时等级表与充值页为准。VIP 充值加赠从升级后的下一笔订单开始生效。</p>

<h2>升级规则</h2>
<ul>
  <li>支付订单成功后，<strong>基础到账金额</strong>计入 VIP 累计，达到阈值即时升级</li>
	<li>线上已验证支付基础到账与审核通过的真实线下充值净额计入 VIP；VIP 加赠、活动加赠、签到、盲盒、兑换码和赠送余额不计入</li>
  <li>退款成功后，会按订单快照回退对应的基础到账累计，避免退款后累计虚高</li>
</ul>

<h2>在哪里查看</h2>
<ul>
  <li><a href="/play">玩法中枢</a> — 登录后查看自己的等级、累计金额与权益</li>
  <li><a href="/purchase">充值</a> — 下单前核对基础到账、VIP 加赠、活动加赠与预计到账</li>
</ul>`,
      },
      {
        id: 'how-to-recharge',
        title: "充值流程",
        summary: "支付金额、基础到账、VIP 加赠与活动加赠如何计算",
        html: `<p class="docs-lead">充值会先按后台基础倍率换算成基础到账金额，再叠加当前 VIP 加赠和活动加赠。API 扣费仍按模型价格与分组倍率计算，不因 VIP 降价。</p>

<h2>三步完成充值</h2>
<ol>
  <li>打开<a href="/purchase">充值</a>页面</li>
  <li>按页面显示的币种、金额范围和可用支付方式填写订单</li>
  <li>确认页面上的当前 VIP、基础到账、VIP 加赠、活动加赠和预计到账后，按提示完成支付</li>
</ol>
<p>支付完成后返回平台，在<a href="/orders">我的订单</a>查看支付状态，在<a href="/wallet">钱包</a>核对余额变化。</p>

<h2>到账示例</h2>
<p>以下数字只用于说明计算方法。假设基础倍率为 1.00、VIP 加赠为 6%、活动加赠为 5%。</p>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>项目</th><th>示例</th><th>说明</th></tr></thead>
<tbody>
<tr><td>支付金额</td><td>$100.00</td><td>你实际提交给支付网关的金额，手续费和限额仍按此口径校验</td></tr>
<tr><td>基础到账</td><td>$100.00</td><td>$100.00 × 基础倍率 1.00，也是本单计入 VIP 累计和邀请返利的基数</td></tr>
<tr><td>VIP 加赠</td><td>+$6.00</td><td>$100.00 × 6%，按下单前 VIP 等级计算</td></tr>
<tr><td>活动加赠</td><td>+$5.00</td><td>$100.00 × 5%，如无活动则为 0</td></tr>
<tr><td>预计到账</td><td>$111.00</td><td>$100.00 × (1 + 6% + 5%)，四舍五入到 2 位</td></tr>
</tbody>
</table>
</div>
<p class="docs-tip">充值升级后，新档位的加赠从下一笔订单开始生效。</p>

<h2>支付后余额没有变化</h2>
<p>先刷新「我的订单」并按订单号定位该笔付款。如果订单仍待确认，或订单已完成但钱包余额未更新，请通过<a href="/contact">联系支持</a>提供订单号。先核实原订单，再决定是否需要重新支付。</p>`,
      },
      {
        id: 'check-in',
        title: "每日签到",
        summary: "完成签到、查看结果并确认是否可补签",
        html: `<h2>完成签到</h2>
<p>登录后打开<a href="/check-in">每日签到</a>，也可通过<a href="/play">玩法中枢</a>的签到卡片进入。先看页面显示的参与条件，再点签到并确认今日状态。</p>
<h2>查看结果</h2>
<p>签到可能只记录参与，也可能积累能量。符合可兑换奖励条件时，结果取决于当前发布的奖池，可能是余额、优惠券或兑换码。余额和优惠券可在<a href="/wallet">钱包</a>核对，兑换码按结果提示前往<a href="/redeem">兑换</a>。签到奖励不计入 VIP 累计。</p>
<h2>连续签到与补签</h2>
<p>在签到页查看连续天数和可用的里程碑提示。如果页面显示补签入口且按钮可用，按提示补签并重新核对状态。补签资格以当前页面提示为准。</p>`,
      },
      {
        id: 'discount-examples',
        title: "Play 权益与充值加赠",
        summary: "VIP 充值加赠怎么生效 · 玩法中枢怎么串联",
        html: `<p class="docs-lead">VIP <strong>不改变 API 计费公式</strong>（仍按上游单价 × 分组倍率），也不会让订单少付钱。VIP 只影响余额充值成功后额外到账多少，并配合 Play 中枢展示身份与玩法权益。</p>

<h2>VIP 加赠怎么生效</h2>
<ul>
  <li>每笔余额充值创建订单时，系统读取<strong>充值前</strong>的 VIP 等级并写入订单快照</li>
  <li>各档比例以 VIP 等级表和下单时的充值预览为准</li>
  <li>活动加赠与 VIP 加赠按加法叠加，但会在付款页、支付结果页和订单详情中分开展示</li>
  <li>订单创建后即使后台配置变化，本单也按快照完成入账</li>
</ul>

<h2>VIP 不影响什么</h2>
<ul>
  <li>不改变 API 扣费公式；模型消耗仍按上游 usage、模型价格和分组倍率计算</li>
  <li>不改变订阅订单价格</li>
  <li>不改变支付网关实付金额、手续费、每日限额或通道校验</li>
</ul>

<h2>查看当前活动和权益</h2>
<p>登录后打开<a href="/play">玩法中枢</a>，查看已发布的活动、有效期及账号可用的权益。充值加赠以<a href="/purchase">充值</a>页的预计到账为准，积分、次数等玩法权益以对应活动卡片为准。</p>`,
      },
      {
        id: 'faq',
        title: "常见问题",
        summary: "VIP 与充值最常被问到的问题",
        html: `<p class="docs-lead">下面汇总充值、VIP 与 Play 玩法最常见的问题。</p>

<h2>为什么我的 VIP 档位变了？</h2>
<p>档位取决于符合条件的充值净额和当前等级门槛。消费余额不会减少充值累计，退款和冲正会影响净额。登录<a href="/play">玩法中枢</a>核对账号的当前档位。</p>

<h2>本单充值升级后，能立刻享受新等级加赠吗?</h2>
<p>不能。本单按<strong>充值前</strong> VIP 等级计算加赠；充值成功后如果升级，新等级从下一笔充值订单开始生效。</p>

<h2>VIP 是否影响 API 扣费?</h2>
<p><strong>不会。</strong> VIP 不改变 API 扣费公式，也不会改变订阅价格。API 扣费仍按 <code>上游 usage × 模型价格 × 分组倍率</code>，与 VIP 档位无关。</p>

<h2>赠送余额能升 VIP 吗?</h2>
<p>不能。VIP 累计由支付订单基础到账和审核通过的线下真实充值净额组成。VIP 加赠、活动加赠、签到、答题、盲盒、兑换码和赠送余额不计入；退款或冲正按净额回退。</p>

<h2>退款后 VIP 累计怎么处理?</h2>
<p>退款成功后，系统按订单快照回退对应的基础到账累计；用户余额扣减则按本单最终到账余额处理。</p>

<h2>为什么到账金额比支付金额多?</h2>
<p>订单详情会显示充值快照：支付金额、基础到账、当时使用的 VIP、VIP 加赠、活动加赠和最终到账。用户可以据此解释“为什么到账这么多”。</p>

<h2>充值加成与活动能叠加吗?</h2>
<p>可以。VIP 加赠与 Campaign 活动加赠按加法叠加；多活动同时生效时，活动充值加赠沿用当前实现取最高值，具体以付款页和 Hub 展示为准。</p>

<h2>玩法开关关闭怎么办?</h2>
<p>签到、Arena、盲盒、Quiz、Team 均有独立 Admin 开关。关闭后对应入口隐藏或 Hub 显示「暂未开启」引导。</p>

<h2>模型列表在哪里看?</h2>
<p>打开<a href="/catalog">模型与价格</a>，登录后核对所用密钥的分组、模型 ID 和可用协议。</p>`,
      },
      {
        id: 'blindbox-rewards',
        title: "盲盒玩法",
        summary: "扣费开箱 · 随机开奖 · 每日次数上限",
        html: `<h2>开箱前核对</h2>
<p>登录后打开<a href="/blindbox">盲盒</a>，或从<a href="/play">玩法中枢</a>进入。确认页面显示的参与条件、单次扣费、剩余次数与当前奖池，再决定是否开箱。可用次数随活动变化，单次奖励随机。</p>
<h2>核对结果</h2>
<p>看结果提示区分余额、优惠券和兑换码。余额与优惠券可到<a href="/wallet">钱包</a>核对，兑换码按结果提示前往<a href="/redeem">兑换</a>。</p>`,
      },
      {
        id: 'image-studio',
        title: "AI创作空间",
        summary: "画布创作 · 生图工作台 · 素材保存 · 与每日任务联动",
        html: `<h2>从主站进入画布</h2>
<p>打开<a href="/ai-creation-space">AI创作空间</a>，页面会跳转到独立的「无限画布」。如果画布显示「登录」，按画布里的提示继续。</p>
<h2>在生图工作台试一次</h2>
<ol>
  <li>在画布侧栏进入<strong>生图工作台</strong>。如果提示没有模型，在「配置与用户偏好」中核对本地模型渠道并拉取模型列表。</li>
  <li>填写提示词，选择可用模型与画幅。所选模型支持参考图时，可添加参考图。</li>
  <li>按界面提示发起生成。先用一张图验证配置，再预览并下载需要保留的结果。</li>
</ol>
<h2>使用极速蹬渠道时</h2>
<p>到主站的<a href="/catalog">模型与价格</a>确认模型、分组与接口，生成后在<a href="/usage">用量</a>核对记录。开发者接入可查看<a href="/docs?cat=deploy&amp;page=text-to-image-api">图片生成 API</a>或<a href="/docs?cat=deploy&amp;page=batch-image-api">批量生图 API</a>。</p>`,
      },
      {
        id: 'token-farm',
        title: "Token 农场",
        summary: "查看当前排行榜、参与规则和结算结果",
        html: `<h2>查看排行榜与活动规则</h2>
<p>打开<a href="/arena">Token 农场</a>，也可从<a href="/play">玩法中枢</a>的活动卡片进入。查看当前开放的榜单、统计周期、上榜条件和奖励档位。</p>
<h2>核对进度与结算</h2>
<p>先在<a href="/usage">用量</a>核对 API 请求，再到活动页核对计入的进度。榜单上的排名随周期内的有效用量变化，奖励以结算结果为准。</p>`,
      },
    ],
  },
  {
    id: 'about-us',
    title: "了解我们",
    description: "网关接入、请求核对与数据处理",
    pages: [
      {
        id: 'about-us-overview',
        title: "平台概览",
        summary: "了解网关支持的接口，核对请求和费用",
        html: `<h2>选择接入方式</h2>
<p>极速蹬提供 API 网关和 AI 创作空间。使用网关时，在<a href="/keys">API 密钥</a>中准备密钥，再到<a href="/catalog">模型与价格</a>核对分组、模型与接口。</p>
<h2>核对调用结果</h2>
<p>先发一个简短的测试请求，再到<a href="/usage">用量记录</a>核对时间、模型和费用。服务连接异常时可查看<a href="/status">系统状态</a>；如需排查单次请求，请将请求 ID、时间和错误码提供给<a href="/contact">客服</a>。</p>`,
      },
      {
        id: 'about-us-privacy',
        title: "隐私与请求数据",
        summary: "了解用量记录、异步任务和安全审计的数据处理差异",
        html: `<h2>请求与用量记录</h2>
<p>网关会将请求转发给相应服务。用量记录包含模型、用量和计费等请求元数据。图片异步任务会保留任务所需的数据和限时结果；启用安全审计时，命中的审计事件可能保存提示内容。</p>
<h2>排障时需要提供哪些信息</h2>
<p>请提供请求 ID、时间和错误码，分享截图之前遮盖密钥和个人信息。涉及敏感数据时，可先向<a href="/contact">客服</a>确认相关功能的数据处理方式。</p>`,
      },
    ],
  },
  {
    id: 'model-learning',
    title: "模型学习",
    description: "可用模型 · 选型建议 · 调用范例",
    pages: [
      {
        id: 'model-list',
        title: "可用模型一览",
        summary: "按密钥分组查看实时可用模型、协议和价格",
        html: `<p class="docs-lead">模型会持续更新，而且不同 API 密钥分组可用的模型可能不同。本页不维护静态模型名称清单；请以登录后 <a href="/catalog">模型与价格</a> 中当前账号和分组可见的数据为准。</p>

<h2>查看当前可用模型</h2>
<ol>
  <li>登录极速蹬并打开 <a href="/catalog">模型与价格</a>。</li>
  <li>确认页面当前选择的服务分组与将要使用的 API 密钥一致。</li>
  <li>复制模型 ID，并核对协议、输入/输出价格及模型状态。</li>
  <li>在请求或客户端设置中原样填写模型 ID；不要根据模型显示名称自行拼写。</li>
</ol>

<h2>为什么文档不列固定模型名</h2>
<p>模型上线、下线、别名和分组权限由平台配置及上游变化决定。旧教程里的模型名、默认模型和价格可能过期；请求报模型不存在或无权限时，先回到模型页与 API 密钥分组重新确认。</p>
<p class="docs-tip">客户端能否使用某个模型，还取决于它支持的 API 协议。模型页或密钥页提供的客户端配置优先于通用示例。</p>`,
      },
      {
        id: 'choose-model',
        title: "如何选模型",
        summary: "按实际可用模型、协议与预算进行选型",
        html: `<p class="docs-lead">没有适合所有任务的固定推荐。先在 <a href="/catalog">模型与价格</a>确认当前密钥分组实际可用的模型、协议和单价，再用小请求比较效果与成本。</p>
<h2>按任务选型</h2>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>任务</th><th>比较维度</th><th>验证方法</th></tr></thead>
<tbody>
<tr><td>代码与 Agent</td><td>工具调用、目标协议、上下文与价格</td><td>用小型真实任务检查修改质量及工具调用</td></tr>
<tr><td>摘要与分类</td><td>延迟、输出长度与单价</td><td>用代表性样本比较准确率和响应时间</td></tr>
<tr><td>复杂推理</td><td>推理能力、上下文需求与预算</td><td>用已知答案的问题检查正确性和稳定性</td></tr>
<tr><td>图片任务</td><td>确认模型明确支持所需图片接口</td><td>按 <a href="/docs?cat=deploy&amp;page=text-to-image-api">图片生成 API</a> 文档发送低风险测试</td></tr>
<tr><td>长文本或批量处理</td><td>上下文/批任务能力、限额和总成本</td><td>先拆小批验证，再逐步增加输入规模</td></tr>
</tbody>
</table>
</div>
<h2>控制成本并验证结果</h2>
<p>先从较短输入和较低输出上限开始，检查回答质量与 <a href="/usage">用量记录</a>中的实际消耗，再决定是否扩大上下文或切换模型。不同模型的参数和能力并不完全相同，按对应客户端及 API 协议配置。</p>
<p class="docs-tip">模型 ID、价格和分组权限以模型页实时显示为准。不要直接复制其他账号、旧截图或旧教程中的模型 ID。</p>`,
      },
      {
        id: 'best-practices',
        title: "调用最佳实践",
        summary: "提示词模板 · 缓存策略 · 错误重试",
        html: `<p class="docs-lead">下面是经过验证的实战技巧,可以显著提升体验和降低成本。</p>
<h2>充分利用提示词缓存</h2>
<p>只有明确支持提示词缓存的模型和接口才能使用缓存控制。缓存计费规则与价格以当前模型说明和实际用量记录为准；如需设置缓存标记，可按对应协议示例配置：</p>
<pre><code>{
  "model": "YOUR_MODEL_ID",
  "system": [
    {"type":"text","text":"很长的系统提示...","cache_control":{"type":"ephemeral"}}
  ],
  "messages": [...]
}</code></pre>
<p class="docs-tip">用量记录 <code>HIT</code> 数字表示本次缓存命中的 tokens,越大越省钱。</p>

<h2>流式响应</h2>
<p>对于 chatbot 等实时场景,加 <code>stream: true</code> 可以让用户提前看到内容生成,体验更好。</p>

<h2>错误重试</h2>
<p>建议对 5xx 和 429 实现指数退避重试:初始 1s,每次 ×2,最多 5 次,最大间隔 32s。</p>

<h2>限制输出长度</h2>
<p>明确指定 <code>max_tokens</code>,避免模型自由发挥导致输出过长 → 成本意外增加。</p>`,
      },
    ],
  },
  {
    id: 'deploy',
    title: "API 接入",
    description: "图片 API、SDK 与请求协议，按真实接口契约调用",
    pages: [
      {
        id: 'claude-code',
        title: "Claude Code CLI 接入",
        summary: "推荐用 CC-Switch 一键添加 · 也支持环境变量手动配置",
        html: `<p class="docs-lead">Claude Code 是 Anthropic 官方的命令行编程助手。将 Base URL 指向本网关后，调用经中转直连上游，<strong>Token 不蒸馏、不替换模型</strong>，与官方 API 行为一致。</p>

<p class="docs-tip"><strong>推荐路径</strong>:用开源工具 <strong>CC-Switch</strong>(MIT 协议)做配置管理 — 添加本站 一次,以后官方 / 中转之间一键切换,免去反复改环境变量。下面优先讲这条路径。</p>

<h2>第 1 步 · 装 Node.js + Claude Code</h2>
<p>没有 Node.js 先看 <strong>环境准备 → 安装 Node.js</strong>。已有环境:</p>
<pre><code>npm install -g @anthropic-ai/claude-code
claude --version    # 验证安装成功</code></pre>

<h2>第 2 步 · 装 CC-Switch</h2>
<pre><code>npm install -g cc-switch
cc --version</code></pre>

<h2>第 3 步 · 在控制台创建专属 Key</h2>
<p>登录本站 控制台,侧边栏点 <strong>API 密钥</strong> → <strong>新建</strong>:</p>
<ol>
  <li>选择服务分组(决定可用模型 + 折扣倍率)</li>
  <li>给 Key 起个识别名(如 <code>my-cc</code>)</li>
  <li>保存,复制生成的 Key — <em>只显示一次,务必先存起来</em></li>
</ol>
<div class="docs-procedure"><strong>页面路径</strong><span>控制台 → API 密钥 → 新建</span><span>选择服务分组后保存，并立即复制只显示一次的密钥。</span></div>

<h2>第 4 步 · 用 CC-Switch 一键添加本站</h2>
<p>把刚拿到的 Key 加进 CC-Switch:</p>
<pre><code>cc add gateway \\
  --base-url https://api.your-domain.com \\
  --token sk-xxxxxxxxxxxxxxx</code></pre>
<p><code>gateway</code> 是这个 provider 的本地名字,可随便取。完成后看一眼:</p>
<pre><code>cc list

  *  anthropic      (官方)
     gateway       https://api.your-domain.com    [新增]</code></pre>
<div class="docs-procedure"><strong>验证结果</strong><span>运行 <code>cc list</code>，确认列表同时显示官方 provider 和新建的 gateway。</span><span>运行 <code>cc use gateway</code> 后再启动 Claude Code。</span></div>

<h2>第 5 步 · 切换到本站 并验证</h2>
<pre><code>cc use gateway        # 切到本站
claude                 # 进入交互模式,任意提问</code></pre>
<p>如果能正常回复就接入成功。每次调用都会同步出现在控制台 <strong>用量记录</strong>,实时计费、可审计。要切回官方:</p>
<pre><code>cc use anthropic</code></pre>

<h2>进阶:不用 CC-Switch,直接配环境变量</h2>
<p>不想装额外工具的话,Claude Code 也支持原生环境变量:</p>
<pre><code># macOS / Linux (bash / zsh)
export ANTHROPIC_BASE_URL="https://api.your-domain.com"
export ANTHROPIC_AUTH_TOKEN="sk-xxxxxxxxxxxxxxx"

# Windows PowerShell
$env:ANTHROPIC_BASE_URL = "https://api.your-domain.com"
$env:ANTHROPIC_AUTH_TOKEN = "sk-xxxxxxxxxxxxxxx"</code></pre>
<p>永久生效:把 export 行写到 <code>~/.bashrc</code> / <code>~/.zshrc</code> / Windows 用户环境变量。<strong>缺点</strong>:在官方和本站 之间切换时,得每次手动改 export 然后 source — 这正是 CC-Switch 想替你解决的痛点。</p>

<h2>常见问题</h2>
<ul>
  <li><strong>401 Unauthorized</strong> — AUTH_TOKEN 拼错或 Key 已被禁用,重新检查 / 重建 Key</li>
  <li><strong>连接超时 / 能查余额但不能请求（或反过来）</strong> — <code>ANTHROPIC_BASE_URL</code> 只填根域名（如 <code>https://api.jisudeng.com</code>），<strong>不要</strong>带尾部 <code>/v1</code>。Claude Code 会拼 <code>/v1/messages</code>，余额查询会拼 <code>/v1/usage</code>；多写 <code>/v1</code> 会变成 <code>/v1/v1/...</code>，两者会打架</li>
  <li><strong>模型不可用</strong> — 服务分组下没该模型,在控制台 <strong>模型与价格</strong> 看分组支持清单</li>
  <li><strong>cc use 后还是连官方</strong> — 没新开终端窗口。CC-Switch 写的是 shell 环境变量,新窗口才生效</li>
</ul>`,
      },
      {
        id: 'codex',
        title: "OpenAI Codex CLI 接入",
        summary: "把 Codex 指向本站 网关 · 兼容 OpenAI 协议",
        html: `<p class="docs-lead">Codex 是 OpenAI 官方推出的命令行编程助手。本站 网关同时兼容 Anthropic 和 OpenAI 协议,Codex 只需把 base URL 指过来即可。</p>

<h2>第 1 步 · 安装 Codex</h2>
<pre><code>npm install -g @openai/codex</code></pre>

<h2>第 2 步 · 创建 Key</h2>
<p>同 Claude Code,在侧边栏 <code>API 密钥</code> 创建一个 Key。<strong>注意</strong>:Codex 走 OpenAI 协议路径,所以创建 Key 时选择支持 GPT 模型的服务分组。</p>

<h2>第 3 步 · 配置</h2>
<p>Codex 通过 <code>~/.codex/config.toml</code> 读配置,也支持环境变量。推荐配置文件方式:</p>
<pre><code># ~/.codex/config.toml
model_provider = "gateway"
model = "YOUR_MODEL_ID"

[model_providers.gateway]
name = "本站"
base_url = "https://api.your-domain.com/v1"
wire_api = "responses"
env_key = "GATEWAY_API_KEY"</code></pre>
<p>把 Key 写入环境变量:</p>
<pre><code>export GATEWAY_API_KEY="sk-xxxxxxxxxxxxxxx"</code></pre>

<h2>第 4 步 · 验证</h2>
<pre><code>codex --help
codex "帮我写一个 fibonacci 函数"</code></pre>
<p class="docs-tip">Codex 默认会读取当前目录上下文,所以请在你想协作的项目根目录运行。</p>

<h2>多账户切换</h2>
<p>如果你同时有官方账号 + 本站 账号,推荐配合 <strong>CC-Switch</strong> 这类工具一键切换,详见 <em>工具与客户端</em> 章节。</p>`,
      },
      {
        id: 'gemini-cli',
        title: "Gemini CLI 接入",
        summary: "把 Gemini CLI 指向本站 网关 · 百万级上下文",
        html: `<p class="docs-lead">Gemini CLI 是 Google 官方的命令行 AI 助手,Gemini 系列模型擅长百万级超长上下文。本站 网关支持 Gemini 协议,接入方式简洁。</p>

<h2>第 1 步 · 安装</h2>
<pre><code>npm install -g @google/gemini-cli</code></pre>

<h2>第 2 步 · 创建 Key</h2>
<p>创建或选择有 Gemini 协议权限的 API Key，并从 <a href="/catalog">模型与价格</a>复制该分组当前可用的模型 ID。不要照抄其他账号或旧教程中的型号。</p>

<h2>第 3 步 · 配置</h2>
<pre><code># 环境变量
export GOOGLE_GEMINI_BASE_URL="https://api.your-domain.com"
export GEMINI_API_KEY="sk-xxxxxxxxxxxxxxx"</code></pre>
<p>配置文件方式见 Gemini CLI 官方文档 <code>~/.gemini/settings.json</code>,加入 baseUrl 字段。</p>

<h2>第 4 步 · 验证</h2>
<pre><code>gemini "/help"
gemini "用 python 写一个快速排序,要带注释"</code></pre>

<h2>为什么用 Gemini?</h2>
<ul>
  <li><strong>超长上下文</strong> — 单次请求可塞百万 token,适合整仓库代码分析</li>
  <li><strong>多模态原生</strong> — 直接喂图、PDF、视频片段</li>
  <li><strong>价格低</strong> — Flash 系列单价非常便宜,做批量处理性价比高</li>
</ul>
<p class="docs-tip">超长上下文虽强,但单次调用 token 也大,记得在侧边栏 <code>用量记录</code> 关注消费曲线,合理设置每个 Key 的日预算。</p>`,
      },
      {
        id: 'sdk-quick',
        title: "SDK 与 cURL 直调",
        summary: "Python / Node SDK · cURL 一行直调",
        html: `<p class="docs-lead">除了 CLI,你也可以直接用官方 SDK 或 cURL 调用网关。<strong>协议 100% 兼容</strong>官方,只需要改一个 base_url 参数。</p>

<h2>Python · Anthropic SDK</h2>
<pre><code>from anthropic import Anthropic

client = Anthropic(
    base_url="https://api.your-domain.com",
    api_key="sk-xxxxxxxxxxxxxxx",
)
msg = client.messages.create(
    model="YOUR_MODEL_ID",
    max_tokens=1024,
    messages=[{"role": "user", "content": "你好"}],
)
print(msg.content[0].text)</code></pre>

<h2>Python · OpenAI SDK</h2>
<pre><code>from openai import OpenAI

client = OpenAI(
    base_url="https://api.your-domain.com/v1",
    api_key="sk-xxxxxxxxxxxxxxx",
)
resp = client.chat.completions.create(
    model="YOUR_MODEL_ID",
    messages=[{"role": "user", "content": "你好"}],
)
print(resp.choices[0].message.content)</code></pre>

<h2>Node.js · Anthropic SDK</h2>
<pre><code>import Anthropic from "@anthropic-ai/sdk"

const client = new Anthropic({
  baseURL: "https://api.your-domain.com",
  apiKey: "sk-xxxxxxxxxxxxxxx",
})
const msg = await client.messages.create({
  model: "YOUR_MODEL_ID",
  max_tokens: 1024,
  messages: [{ role: "user", content: "你好" }],
})
console.log(msg.content[0].text)</code></pre>

<h2>cURL · 通用 HTTP</h2>
<pre><code>curl https://api.your-domain.com/v1/chat/completions \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "YOUR_MODEL_ID",
    "messages": [{"role":"user","content":"hi"}]
  }'</code></pre>

<p class="docs-tip">模型 ID 不同于显示名称。请在 <a href="/catalog">模型与价格</a>中确认分组权限及协议后复制精确 ID；不要把占位符原样用于请求。</p>`,
      },
    ],
  },
  {
    id: 'tools',
    title: "工具集成",
    description: "终端、编辑器、桌面客户端与智能体，统一从这里选择接入教程",
    pages: [
      {
        id: 'cc-switch',
        title: "CC-Switch · Claude Code 配置切换",
        summary: "一键在官方与本站 之间切换 · 免改环境变量",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">CC-Switch</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">CLI</span>
      <span class="docs-launch-tag">macOS</span>
      <span class="docs-launch-tag">Linux</span>
      <span class="docs-launch-tag">Windows</span>
      <span class="docs-launch-tag">MIT</span>
    </div>
    <div class="docs-launch-blurb">在 Claude Code 的多个 provider 配置之间一键切换 — 把本站 与官方 Key 都装进来,免去反复改环境变量。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/farion1231/cc-switch" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">farion1231/cc-switch</span>
  </a>
</div>

<h2>安装</h2>
<pre><code>npm install -g cc-switch
cc --version</code></pre>

<h2>添加本站 配置</h2>
<pre><code>cc add gateway \\
  --base-url https://api.your-domain.com \\
  --token sk-xxxxxxxxxxxxxxx</code></pre>
<p><code>gateway</code> 是 provider 名,可自取。</p>

<h2>切换 & 验证</h2>
<pre><code>cc use gateway     # 切到本站
cc use anthropic    # 切回官方
cc list             # 看当前所有 provider</code></pre>

<p class="docs-tip">CC-Switch 改的是 shell 环境变量。<strong>切换后新开一个终端窗口</strong> 才生效。</p>`,
      },
      {
        id: 'openclaw',
        title: "大龙虾 (OpenClaw) · 终端 AI 助手",
        summary: "npm 安装 · 本站「一键部署」按钮 30 秒搞定 · 跨平台",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">OpenClaw 🦞 (大龙虾)</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">CLI</span>
      <span class="docs-launch-tag">Node.js 22+</span>
      <span class="docs-launch-tag">macOS</span>
      <span class="docs-launch-tag">Linux</span>
      <span class="docs-launch-tag">WSL2</span>
    </div>
    <div class="docs-launch-blurb">本地运行的个人 AI 助手,中文社区昵称"大龙虾"(logo 是只龙虾)。原生支持 custom OpenAI-compatible endpoint,完美适配本站。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/openclaw/openclaw" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">openclaw/openclaw</span>
  </a>
</div>


<h2>第 1 步 · 安装 OpenClaw</h2>
<p>OpenClaw 需要 <strong>Node.js ≥ 22.12</strong>。Windows 用户必须装在 WSL2 里(官方不支持原生 Windows)。</p>
<pre><code># macOS / Linux / WSL
npm install -g openclaw@latest

# 检查
openclaw --version</code></pre>
<p class="docs-tip">没装 Node 22?用 nvm: <code>curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash && nvm install 22</code></p>

<h2>第 2 步(推荐)· 用本站「一键部署」</h2>
<p>在本站 <code>API 密钥</code> 页面找到任意 active 密钥的行 → 点 <strong>「一键部署 ▾」</strong> → 选 <strong>OpenClaw</strong> → 选目标系统(Linux / macOS / WSL / 其他 VPS 都用 bash) → 复制一行命令 → 粘到终端运行。</p>
<p>脚本自动完成:</p>
<ul>
  <li>检测 Node 版本 + OpenClaw 是否已装</li>
  <li>提示粘贴密钥(输入隐藏不进 shell 历史)</li>
  <li>从中转站拉取模型列表,上下方向键选主模型 + 副模型</li>
  <li>备份现有配置 → 写入新配置 → 自检</li>
  <li>失败自动回滚,不留半成品</li>
</ul>

<h2>第 2 步(手动)· 不用脚本直接配</h2>
<pre><code>openclaw onboard \\
  --non-interactive --accept-risk --skip-health \\
  --auth-choice custom-api-key \\
  --custom-base-url "https://api.your-domain.com/v1" \\
  --custom-api-key "sk-你的密钥" \\
  --custom-compatibility openai \\
  --custom-model-id "YOUR_MODEL_ID"</code></pre>

<h2>第 3 步 · 试用</h2>
<pre><code>openclaw doctor --non-interactive    # 自诊
openclaw agent --local -m "用一句话介绍你自己"</code></pre>

<h2>常见问题</h2>
<ul>
  <li><strong>配置位置?</strong> <code>~/.openclaw/openclaw.json</code></li>
  <li><strong>想回滚?</strong> 备份在 <code>~/.openclaw.bak.&lt;时间戳&gt;</code>,<code>mv</code> 回去即可</li>
</ul>`,
      },
      {
        id: 'hermes',
        title: "Hermes Agent · Python 终端 AI",
        summary: "pipx 安装 · 本站「一键部署」 · 支持 fallback 副模型",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Hermes Agent 💫</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">CLI + 多平台</span>
      <span class="docs-launch-tag">Python 3.10+</span>
      <span class="docs-launch-tag">macOS</span>
      <span class="docs-launch-tag">Linux</span>
      <span class="docs-launch-tag">Windows</span>
    </div>
    <div class="docs-launch-blurb">Nous Research 出品的"会成长的 AI agent",完整 TUI、跨终端 + Telegram/Discord/Slack 多平台、内置记忆/技能系统。完全 OpenAI 兼容。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/NousResearch/hermes-agent" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">NousResearch/hermes-agent</span>
  </a>
</div>


<h2>第 1 步 · 安装 Hermes</h2>
<p>用 <strong>pipx</strong> 装(隔离环境,不污染系统 Python)。需要 Python 3.10+。</p>
<pre><code>pipx install hermes-agent

# 检查
hermes --version</code></pre>
<p class="docs-tip">没装 pipx?<code>python3 -m pip install --user pipx && python3 -m pipx ensurepath</code></p>

<h2>第 2 步(推荐)· 用本站「一键部署」</h2>
<p>同 OpenClaw — 在 <code>API 密钥</code> 页面点行级「一键部署」按钮 → 选 <strong>Hermes Agent</strong> → 选系统 → 复制命令 → 粘进终端。脚本会写好 <code>~/.hermes/config.yaml</code> 并自检。</p>

<h2>第 2 步(手动)· 编辑 yaml</h2>
<p>编辑 <code>~/.hermes/config.yaml</code>(不存在则新建):</p>
<pre><code># 由本站接入写入
model:
  default:  "YOUR_MODEL_ID"
  provider: "custom"
  base_url: "https://api.your-domain.com/v1"
  api_key:  "sk-你的密钥"</code></pre>
<p class="docs-tip">不要用 <code>~/.hermes/.env</code> 设 <code>OPENAI_BASE_URL</code>,那会泄漏到子进程(Hermes 官方 issue #1002 详细说明)。直接写 config.yaml 最干净。</p>

<h2>第 3 步 · 试用</h2>
<pre><code>hermes "用一句话介绍你自己"     # 一句话模式
hermes                          # 进交互式 TUI(/model 切模型 · /new 新会话)</code></pre>

<h2>第 4 步 · 副模型 fallback(可选,推荐)</h2>
<p>Hermes 原生支持主模型挂了自动切副模型:</p>
<pre><code>hermes fallback add YOUR_FALLBACK_MODEL_ID</code></pre>

<h2>常见问题</h2>
<ul>
  <li><strong>配置位置?</strong> <code>~/.hermes/config.yaml</code>(也可用 <code>hermes config path</code> 看)</li>
  <li><strong>报 "empty response"?</strong> 一般是上游瞬态紧张,Hermes 客户端自带 3 次 retry,等几秒重试即可</li>
  <li><strong>想切模型?</strong> <code>hermes model</code> 交互选,或者直接改 yaml 的 <code>model.default</code></li>
</ul>`,
      },
      {
        id: 'cherry-studio',
        title: "CherryStudio 桌面客户端",
        summary: "跨平台 GUI 客户端 · 鼠标点击配置 · 多模型并排对照",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">CherryStudio 🍒</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">桌面 App</span>
      <span class="docs-launch-tag">Windows</span>
      <span class="docs-launch-tag">macOS</span>
      <span class="docs-launch-tag">Linux</span>
      <span class="docs-launch-tag">AGPL-3.0</span>
    </div>
    <div class="docs-launch-blurb">开源跨平台 AI 桌面客户端,把不同 provider 的模型聚合到一个窗口里,支持多模型并排对照、知识库、agent 工作流。最适合不爱开终端的用户。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/CherryHQ/cherry-studio" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">CherryHQ/cherry-studio</span>
  </a>
</div>

<h2>第 1 步 · 下载安装</h2>
<p>官网 <a href="https://www.cherry-ai.com/download" target="_blank">cherry-ai.com/download</a> 选对应平台的安装包(Windows <code>.exe</code> / macOS <code>.dmg</code> / Linux <code>.AppImage</code>),2 分钟装完。</p>

<h2>第 2 步 · 添加本站 为自定义供应商</h2>
<p>打开 Cherry Studio → 点左下角 <strong>设置</strong> 齿轮 → 进 <strong>模型服务</strong> → 左侧底部 <strong>「+ 添加」</strong> 自定义供应商。</p>
<div class="docs-procedure"><strong>页面路径</strong><span>Cherry Studio → 设置 → 模型服务 → 添加自定义供应商</span><span>不同版本的按钮名称可能略有差异，以当前版本的模型服务设置页为准。</span></div>

<h2>第 3 步 · 填两个核心字段</h2>
<p>选 <strong>OpenAI 兼容</strong> 类型,然后填:</p>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>字段</th><th>值</th><th>说明</th></tr></thead>
<tbody>
<tr><td><strong>API 地址</strong></td><td><code>https://api.your-domain.com</code></td><td>注意:<strong>不带</strong> <code>/v1</code>,Cherry Studio 自动拼接</td></tr>
<tr><td><strong>API 密钥</strong></td><td><code>sk-你的密钥</code></td><td>在本站 API 密钥页面复制</td></tr>
</tbody>
</table>
</div>
<div class="docs-procedure"><strong>字段填写</strong><span>类型选 OpenAI 兼容，API 地址使用文档首页显示的网关地址，密钥粘贴 API 密钥页生成的值。</span></div>

<p class="docs-tip"><strong>API 地址重要细节</strong>:Cherry Studio 会自动加 <code>/v1/chat/completions</code>。如果想用<strong>完整自定义路径</strong>,地址末尾加 <code>#</code> 就不再拼接(高级用法)。</p>

<h2>第 4 步 · 添加模型</h2>
<p>填好 API 信息后,点配置页底部 <strong>「管理」</strong> 按钮,Cherry Studio 会自动拉取本站支持的模型列表。点列表里模型右侧的 <strong>「+」</strong> 把它加入你的可用列表。</p>
<div class="docs-procedure"><strong>模型列表</strong><span>点击「管理」拉取当前分组可用模型；只添加当前密钥分组实际返回的模型 ID。</span></div>

<p>不在教程中列出固定模型清单。使用 Cherry Studio 的模型管理功能读取列表后，按 API Key 所属分组确认可用模型；如需手动添加，请从 <a href="/catalog">模型与价格</a>复制当前模型 ID。</p>

<h2>第 5 步 · 启用 + 试用</h2>
<p><strong>关键</strong>:别忘了点配置页右上角的「开关」把这个供应商<strong>启用</strong>,否则模型列表里看不到。</p>
<div class="docs-procedure"><strong>启用供应商</strong><span>打开供应商开关后，在新会话的模型选择器中选择刚添加的模型。</span></div>

<p>启用后,主界面顶部模型选择器 → 选你刚加的模型 → 开始对话,跟 ChatGPT 一样用。</p>

<h2>第 6 步 · 验证连通性</h2>
<p>在 API 密钥输入框旁边有个 <strong>「检查」</strong> 按钮,点一下会用列表里最后一个聊天模型测试连接。返回 ✓ 说明配好了。</p>
<p class="docs-tip">检查失败常见原因:① 模型列表里有不支持的模型(删掉重新「管理」拉取);② API 地址多了 <code>/v1</code>(去掉);③ 密钥拷错了(在本站重新复制)。</p>

<h2>多 Key 轮询(高级)</h2>
<p>Cherry Studio 支持单供应商多 Key 自动轮询。如果你建了多个本站密钥,可以在 API 密钥字段里用 <strong>英文逗号</strong> 分隔:</p>
<pre><code>sk-xxx1,sk-xxx2,sk-xxx3</code></pre>
<p>会按顺序轮询使用,适合个人多 Key 分摊用量。</p>

<p class="docs-tip">CherryStudio 的"模型并排对照"很适合做选型 — 同一个提示词同时发给三家模型,直接看哪家答得好。</p>`,
      },
      {
        id: 'opencode',
        title: "OpenCode · 终端 AI 编程助手",
        summary: "SST 开源的终端 pair-programming · 直连任意 OpenAI 兼容网关",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">OpenCode</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">CLI</span>
      <span class="docs-launch-tag">macOS</span>
      <span class="docs-launch-tag">Linux</span>
      <span class="docs-launch-tag">Windows</span>
      <span class="docs-launch-tag">MIT</span>
    </div>
    <div class="docs-launch-blurb">SST 团队开源的终端 AI 编程助手 — 在终端里 pair-programming,完全控制本地文件与命令执行,支持任意 OpenAI 兼容 provider。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/sst/opencode" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">sst/opencode</span>
  </a>
</div>

<h2>第 1 步 · 安装</h2>
<pre><code># 跨平台一键安装
curl -fsSL https://opencode.ai/install | bash

# 或 npm
npm install -g opencode-ai</code></pre>

<h2>第 2 步 · 配置本站 provider</h2>
<p>编辑 <code>~/.config/opencode/config.json</code>(没有就新建):</p>
<pre><code>{
  "provider": {
    "gateway": {
      "name": "本站",
      "type": "openai",
      "options": {
        "baseURL": "https://api.your-domain.com/v1",
        "apiKey": "sk-xxxxxxxxxxxxxxx"
      },
      "models": {
        "YOUR_MODEL_ID": { "name": "Model shown in your Jisudeng catalog" }
      }
    }
  }
}</code></pre>

<h2>第 3 步 · 运行</h2>
<pre><code>cd your-project
opencode             # 进入交互模式
# 用 /model 命令切到 gateway/YOUR_MODEL_ID</code></pre>

<p class="docs-tip">OpenCode 默认会读取当前目录文件作为上下文,适合在项目根目录运行。</p>`,
      },
      {
        id: 'cline',
        title: "Cline · VSCode 编程助手扩展",
        summary: "装在 VSCode 里的自主 AI 助手 · 能读写文件 + 跑命令",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Cline</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">VSCode 扩展</span>
      <span class="docs-launch-tag">跨平台</span>
      <span class="docs-launch-tag">Apache-2.0</span>
    </div>
    <div class="docs-launch-blurb">VSCode 里的自主编程 agent,能读写文件、跑终端命令、装依赖,每一步操作都需要你确认,可控性强。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/cline/cline" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">cline/cline</span>
  </a>
</div>

<h2>第 1 步 · 在 VSCode 装扩展</h2>
<p>打开 VSCode 扩展面板(Cmd/Ctrl + Shift + X),搜 <strong>Cline</strong>,点 Install。</p>

<h2>第 2 步 · 配置本站</h2>
<ol>
  <li>点左侧 Cline 图标打开侧边栏</li>
  <li>API Provider 选 <strong>OpenAI Compatible</strong></li>
  <li>Base URL: <code>https://api.your-domain.com/v1</code></li>
  <li>API Key: 你的 Key</li>
  <li>Model ID: <code>YOUR_MODEL_ID</code> (copy the exact ID from <a href="/catalog">模型与价格</a>)</li>
</ol>

<h2>第 3 步 · 开始</h2>
<p>新建一个 Cline 任务,描述你的需求(如"帮我加一个登录页面"),Cline 会逐步规划 + 改文件,每一步都问你 Approve / Reject。</p>

<p class="docs-tip">Cline 比 Cursor 更"自主",适合让它独立完成一个完整 feature,而不只是补全单行。</p>`,
      },
      {
        id: 'continue',
        title: "Continue · 开源 Cursor 替代",
        summary: "VSCode / JetBrains 双端扩展 · 自由配置任意 provider",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Continue</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">VSCode 扩展</span>
      <span class="docs-launch-tag">JetBrains 扩展</span>
      <span class="docs-launch-tag">Apache-2.0</span>
    </div>
    <div class="docs-launch-blurb">VSCode 与 JetBrains 双端 AI 编程扩展,功能像 Cursor 但完全开源 + 自由配置,支持多模型并行 + 内置规则 (rules) 系统。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/continuedev/continue" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">continuedev/continue</span>
  </a>
</div>

<h2>第 1 步 · 装扩展</h2>
<ul>
  <li><strong>VSCode</strong>: 扩展面板搜 <strong>Continue</strong> → Install</li>
  <li><strong>JetBrains</strong>: Plugins → Marketplace 搜 <strong>Continue</strong> → Install</li>
</ul>

<h2>第 2 步 · 配置本站</h2>
<p>打开 <code>~/.continue/config.json</code>(扩展安装后自动生成),在 <code>models</code> 数组里加:</p>
<pre><code>{
  "models": [
    {
      "title": "本站 · Claude Sonnet",
      "provider": "openai",
      "model": "YOUR_MODEL_ID",
      "apiKey": "sk-xxxxxxxxxxxxxxx",
      "apiBase": "https://api.your-domain.com/v1"
    },
    {
      "title": "本站 · GPT-5.6 Terra",
      "provider": "openai",
      "model": "YOUR_MODEL_ID",
      "apiKey": "sk-xxxxxxxxxxxxxxx",
      "apiBase": "https://api.your-domain.com/v1"
    },
    {
      "title": "本站 · GPT-5.5",
      "provider": "openai",
      "model": "YOUR_MODEL_ID",
      "apiKey": "sk-xxxxxxxxxxxxxxx",
      "apiBase": "https://api.your-domain.com/v1"
    }
  ]
}</code></pre>

<h2>第 3 步 · 常用快捷键</h2>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>快捷键</th><th>动作</th></tr></thead>
<tbody>
<tr><td>Cmd / Ctrl + L</td><td>选中代码加进对话上下文</td></tr>
<tr><td>Cmd / Ctrl + I</td><td>内联编辑选中代码</td></tr>
<tr><td>Cmd / Ctrl + Shift + L</td><td>把文件加进上下文</td></tr>
</tbody>
</table>
</div>

<p class="docs-tip">Continue 的多模型配置很灵活 — 可以同时把"聪明的 opus"和"便宜的 haiku"加进来,日常用 haiku,关键问题切到 opus。</p>`,
      },
      {
        id: 'aider',
        title: "Aider · 终端 pair-programming",
        summary: "Python 实现的终端 AI 助手 · 直接编辑文件 + 自动 git commit",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Aider</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">CLI · Python</span>
      <span class="docs-launch-tag">macOS</span>
      <span class="docs-launch-tag">Linux</span>
      <span class="docs-launch-tag">Windows</span>
      <span class="docs-launch-tag">Apache-2.0</span>
    </div>
    <div class="docs-launch-blurb">命令行 AI pair-programming 工具,Python 实现 — 直接改本地仓库文件,每次改动自动 git commit,适合喜欢终端 + 高度可审计工作流的开发者。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/Aider-AI/aider" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">Aider-AI/aider</span>
  </a>
</div>

<h2>第 1 步 · 安装</h2>
<pre><code># 推荐用 pipx 隔离安装(不污染系统 Python)
python -m pip install aider-install
aider-install

# 或直接 pip
python -m pip install aider-chat</code></pre>

<h2>第 2 步 · 配置本站</h2>
<pre><code># 设环境变量
export OPENAI_API_BASE=https://api.your-domain.com/v1
export OPENAI_API_KEY=sk-xxxxxxxxxxxxxxx</code></pre>

<h2>第 3 步 · 进项目跑</h2>
<pre><code>cd your-project
aider --model openai/YOUR_MODEL_ID file1.py file2.py</code></pre>
<p>Aider 会进入 REPL,你描述需求 → Aider 改文件 → 自动 git commit。可审计性强。</p>

<p class="docs-tip">Aider 的杀手锏是 <strong>自动 git commit</strong> — 每个 AI 改动都是独立 commit,出问题 <code>git reset</code> 即可。比"AI 直接覆盖文件"安全得多。</p>`,
      },
    ],
  },
  {
    id: 'environment',
    title: "环境准备",
    description: "Node.js 分平台安装 · 终端 · 网络",
    pages: [
      {
        id: 'nodejs-windows',
        title: "Node.js · Windows",
        summary: "一步装好 Node.js · MSI 安装包 + PowerShell 验证",
        html: `<p class="docs-lead">通过 npm 安装工具时需要 Node.js。若工具提供独立安装包，请按该工具的官方前置要求操作。Windows 用户可用 Node.js 官方 MSI 安装包。</p>

<h2>第 1 步 · 下载 Node.js LTS</h2>
<p>访问 <a href="https://nodejs.org/zh-cn/download" target="_blank" rel="noopener">Node.js 官方下载页</a>,下载当前提供的 <strong>Windows Installer (.msi)</strong> 64-bit LTS 版本。请勿按旧文章中的固定版本号选包。</p>
<div class="docs-procedure"><strong>下载选择</strong><span>在 Node.js 官方下载页选择 Windows Installer (.msi) 的 64-bit LTS 安装包。</span><span>安装时保持 Add to PATH 勾选，完成后重新打开 PowerShell。</span></div>

<h2>第 2 步 · 双击安装</h2>
<ol>
  <li>双击下载的 .msi 文件</li>
  <li>一路 Next,<strong>保持默认勾选 "Add to PATH"</strong></li>
  <li>选 Custom Install 也可以,但默认配置足够日常使用</li>
  <li>等待 ~1 分钟安装完成</li>
</ol>

<h2>第 3 步 · 验证</h2>
<p>新开一个 PowerShell 窗口(<strong>不能用安装前已经打开的</strong>,PATH 没刷新):</p>
<pre><code>node -v
# 输出类似 v20.10.0

npm -v
# 输出类似 10.2.3</code></pre>

<h2>多版本切换:nvm-windows</h2>
<p>如果你需要在多个 Node.js 版本之间切换:</p>
<ol>
  <li>访问 <code>https://github.com/coreybutler/nvm-windows/releases</code></li>
  <li>下载 <code>nvm-setup.exe</code> 安装</li>
  <li>用法:<code>nvm install 20</code> / <code>nvm use 20</code></li>
</ol>
<p class="docs-tip">企业内网装包失败 → 切换 npm 源到国内镜像:<code>npm config set registry https://registry.npmmirror.com</code></p>`,
      },
      {
        id: 'nodejs-macos',
        title: "Node.js · macOS",
        summary: "用 Homebrew + nvm 装 Node.js · 多版本管理友好",
        html: `<p class="docs-lead">macOS 推荐用 Homebrew 装 nvm,再用 nvm 装 Node.js — 多版本切换最灵活。</p>

<h2>第 1 步 · 装 Homebrew(如果还没装)</h2>
<pre><code>/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"</code></pre>
<p>装完按提示把 <code>brew</code> 加入 PATH(M1/M2 Mac 路径是 <code>/opt/homebrew/bin</code>,Intel Mac 是 <code>/usr/local/bin</code>)。</p>

<h2>第 2 步 · 装 nvm</h2>
<pre><code>brew install nvm

# 加 nvm 初始化到 zsh 配置
mkdir -p ~/.nvm
echo 'export NVM_DIR="$HOME/.nvm"' >> ~/.zshrc
echo '[ -s "$(brew --prefix)/opt/nvm/nvm.sh" ] && \\. "$(brew --prefix)/opt/nvm/nvm.sh"' >> ~/.zshrc
source ~/.zshrc</code></pre>

<h2>第 3 步 · 装 Node.js LTS</h2>
<pre><code>nvm install --lts
nvm use --lts
nvm alias default 'lts/*'    # 设默认版本</code></pre>

<h2>第 4 步 · 验证</h2>
<pre><code>node -v
npm -v
which node    # 看到指向 ~/.nvm/... 路径才算 nvm 接管成功</code></pre>

<h2>常见问题</h2>
<ul>
  <li><strong>command not found: nvm</strong> — <code>~/.zshrc</code> 没 source 上,新开终端窗口或重新 <code>source ~/.zshrc</code></li>
  <li><strong>SSL 证书报错</strong> — macOS 系统证书过期,跑 <code>brew install --cask oracle-jdk</code> 之前的版本可能不带证书。最简单:升级 macOS 到最新</li>
  <li><strong>装包慢</strong> — <code>npm config set registry https://registry.npmmirror.com</code> 切国内镜像</li>
</ul>

<p class="docs-tip">不想装 nvm?也可以直接 <code>brew install node</code>,但日后切版本麻烦,不推荐。</p>`,
      },
      {
        id: 'nodejs-linux',
        title: "Node.js · Linux",
        summary: "Ubuntu / Debian / CentOS · 用 nvm 装 Node.js",
        html: `<p class="docs-lead">Linux 各发行版的包管理器都能装 Node.js,但版本通常滞后。推荐用 nvm,跟 macOS 路径几乎一致。</p>

<h2>第 1 步 · 装 nvm</h2>
<p>按 <a href="https://github.com/nvm-sh/nvm#installing-and-updating">nvm 官方安装说明</a>安装，完成后重新打开终端。</p>

<h2>第 2 步 · 装 Node.js LTS</h2>
<pre><code>nvm install --lts
nvm use --lts
nvm alias default 'lts/*'</code></pre>

<h2>第 3 步 · 验证</h2>
<pre><code>node -v
npm -v</code></pre>

<h2>系统包管理器路径(备选)</h2>
<p>Ubuntu / Debian:</p>
<pre><code>curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
sudo apt-get install -y nodejs</code></pre>
<p>CentOS / RHEL / Fedora:</p>
<pre><code>curl -fsSL https://rpm.nodesource.com/setup_lts.x | sudo bash -
sudo dnf install -y nodejs</code></pre>

<h2>npm 全局装包权限(可选)</h2>
<p>用 nvm 不会有权限问题,但若用系统包管理器,<code>npm install -g</code> 可能需要 sudo。更推荐改 npm 全局目录到用户目录:</p>
<pre><code>mkdir -p ~/.npm-global
npm config set prefix '~/.npm-global'
echo 'export PATH=~/.npm-global/bin:$PATH' >> ~/.bashrc
source ~/.bashrc</code></pre>

<p class="docs-tip">Linux 服务器装 CLI 比较少见,大部分人 Linux 上用 Python SDK / cURL 直调更顺。CLI 主要在桌面环境用。</p>`,
      },
      {
        id: 'network',
        title: "网络与代理",
        summary: "检查网关连通性、客户端配置与服务状态",
        html: `<p class="docs-lead">先在运行客户端的网络和终端中检查网关能否访问，再排查鉴权、模型与协议。</p>

<h2>测试连通性</h2>
<pre><code>curl -i https://api.your-domain.com/health</code></pre>
<p>HTTP 200 表示网关可达。随后用<a href="/docs?cat=tutorial&amp;page=quick-start">快速开始</a>的请求验证密钥和所选模型。</p>

<h2>如果你确实在用代理</h2>
<p>大部分情况下你不需要,但如果终端默认走了系统代理,可能 cURL/SDK 会经过代理 → 增加延迟。可以临时关代理:</p>
<pre><code># macOS / Linux
unset HTTP_PROXY HTTPS_PROXY

# Windows PowerShell
$env:HTTP_PROXY=$null
$env:HTTPS_PROXY=$null</code></pre>

<h2>企业网 / 校园网</h2>
<p>企业或校园网络可能要求出站域名在允许列表中。请向网络管理员提供文档首页显示的 API 网关域名及实际错误。</p>

<h2>查看真实延迟</h2>
<p>打开<a href="/status">系统状态</a>查看平台当前公开的可用性信息。若问题持续，请将时间、错误码和请求 ID 提供给<a href="/contact">客服</a>。</p>`,
      },
    ],
  },
  {
    id: 'vibe-coding',
    title: "Vibe Coding 必装",
    description: "插件 · Skill · 工作流 · Claude Code 与 Codex 双家对照",
    pages: [
      {
        id: 'start-here',
        title: "开篇 · 3 分钟搞懂四个词",
        summary: "MCP / 插件 / Skill / 工作流是什么 · Claude Code 与 Codex 各怎么装",
        html: `<p class="docs-lead">这个板块介绍的工具全部满足三个条件:<strong>开源、免费起步、装完立刻有感</strong>。教程同时覆盖 <strong>Claude Code</strong> 和 <strong>Codex</strong> 两家 CLI — 你用哪个,就抄哪行命令。动手前先花 3 分钟搞懂四个词。</p>

<h2>四个词,四句话</h2>
<ul>
  <li><strong>MCP</strong> — 给 AI 装的"外挂接口"。装一个,AI 就多一个本事:能联网、能开浏览器、能查文档。Claude Code 和 Codex 都支持,是两家通吃的标准。</li>
  <li><strong>插件 (Plugin)</strong> — 一键安装的功能包,自带斜杠命令,像手机装 App。两家各有各的商店,互不通用。</li>
  <li><strong>Skill</strong> — 教 AI 新套路的 Markdown 说明书,丢进目录就生效,零代码。两家用同一套开放标准,只是目录不同。</li>
  <li><strong>工作流 (Workflow)</strong> — 给 AI 套上的做事流程,治"改着改着就失控"。</li>
</ul>

<h2>装备全景</h2>
<figure class="docs-map">
<svg viewBox="0 0 880 500" fill="none" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="Vibe Coding 装备库思维导图">
  <!-- 中心节点 -->
  <rect x="24" y="206" width="148" height="64" rx="12" fill="none" stroke="var(--ink)" stroke-width="1.2"/>
  <text x="98" y="234" text-anchor="middle" font-family="'Noto Serif SC',serif" font-size="15" font-weight="600" letter-spacing="0.5" fill="var(--ink)">Vibe Coding</text>
  <text x="98" y="255" text-anchor="middle" font-family="'Noto Serif SC',serif" font-size="12" letter-spacing="4" fill="var(--ink-3)">装备库</text>
  <!-- 五条干线 -->
  <path d="M172 238 C 208 238 212 94 246 94" stroke="var(--line)" stroke-width="1.3" stroke-linecap="round"/>
  <path d="M172 238 C 208 238 212 240 246 240" stroke="var(--line)" stroke-width="1.3" stroke-linecap="round"/>
  <path d="M172 238 C 208 238 212 330 246 330" stroke="var(--line)" stroke-width="1.3" stroke-linecap="round"/>
  <path d="M172 238 C 208 238 212 392 246 392" stroke="var(--line)" stroke-width="1.3" stroke-linecap="round"/>
  <path d="M172 238 C 208 238 212 460 246 460" stroke="var(--line)" stroke-width="1.3" stroke-linecap="round"/>
  <!-- 分支 1 · MCP -->
  <text x="252" y="99" font-family="'Noto Serif SC',serif" font-size="14" font-weight="600" fill="var(--ink)">MCP 外挂</text>
  <text x="252" y="116" font-family="'Noto Sans SC',system-ui" font-size="10" letter-spacing="3" fill="var(--ink-3)">两家通吃</text>
  <path d="M338 94 C 392 94 404 24 462 24" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 94 C 392 94 404 52 462 52" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 94 C 392 94 404 80 462 80" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 94 C 392 94 404 108 462 108" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 94 C 392 94 404 136 462 136" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 94 C 392 94 404 164 462 164" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <circle cx="470" cy="24" r="2.4" fill="var(--ink-2)"/><text x="483" y="28.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Context7</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  查最新文档</tspan></text>
  <circle cx="470" cy="52" r="2.4" fill="var(--ink-2)"/><text x="483" y="56.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Serena</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  符号级读代码</tspan></text>
  <circle cx="470" cy="80" r="2.4" fill="var(--ink-2)"/><text x="483" y="84.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Playwright</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  AI 开浏览器</tspan></text>
  <circle cx="470" cy="108" r="2.4" fill="var(--ink-2)"/><text x="483" y="112.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Exa</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  联网搜索</tspan></text>
  <circle cx="470" cy="136" r="2.4" fill="var(--ink-2)"/><text x="483" y="140.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">DeepWiki</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  仓库变百科</tspan></text>
  <circle cx="470" cy="164" r="2.4" fill="var(--ink-2)"/><text x="483" y="168.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">CodeGraph</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  代码地图</tspan></text>
  <!-- 分支 2 · 插件 -->
  <text x="252" y="245" font-family="'Noto Serif SC',serif" font-size="14" font-weight="600" fill="var(--ink)">插件</text>
  <text x="252" y="262" font-family="'Noto Sans SC',system-ui" font-size="10" letter-spacing="3" fill="var(--ink-3)">两家各有商店</text>
  <path d="M338 240 C 392 240 404 198 462 198" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 240 C 392 240 404 226 462 226" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 240 C 392 240 404 254 462 254" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 240 C 392 240 404 282 462 282" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <circle cx="470" cy="198" r="2.4" fill="var(--ink-2)"/><text x="483" y="202.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Codex</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  双模型互相挑错</tspan></text>
  <circle cx="470" cy="226" r="2.4" fill="var(--ink-2)"/><text x="483" y="230.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Superpowers</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  方法论内功</tspan></text>
  <circle cx="470" cy="254" r="2.4" fill="var(--ink-2)"/><text x="483" y="258.5"><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" font-weight="600" fill="var(--ink)">官方插件市场</tspan><tspan font-family="'JetBrains Mono',monospace" font-size="11" fill="var(--ink-3)">  /plugin · /plugins</tspan></text>
  <circle cx="470" cy="282" r="2.4" fill="var(--ink-2)"/><text x="483" y="286.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Understand-Anything</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  知识图谱</tspan></text>
  <!-- 分支 3 · Skill -->
  <text x="252" y="335" font-family="'Noto Serif SC',serif" font-size="14" font-weight="600" fill="var(--ink)">Skill</text>
  <text x="252" y="352" font-family="'Noto Sans SC',system-ui" font-size="10" letter-spacing="3" fill="var(--ink-3)">同一套开放标准</text>
  <path d="M338 330 C 392 330 404 316 462 316" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 330 C 392 330 404 344 462 344" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <circle cx="470" cy="316" r="2.4" fill="var(--ink-2)"/><text x="483" y="320.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">anthropics/skills</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  官方精选</tspan></text>
  <circle cx="470" cy="344" r="2.4" fill="var(--ink-2)"/><text x="483" y="348.5"><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" font-weight="600" fill="var(--ink)">自己写</tspan><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)"> SKILL.md</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  零代码</tspan></text>
  <!-- 分支 4 · 工作流 -->
  <text x="252" y="397" font-family="'Noto Serif SC',serif" font-size="14" font-weight="600" fill="var(--ink)">工作流</text>
  <text x="252" y="414" font-family="'Noto Sans SC',system-ui" font-size="10" letter-spacing="3" fill="var(--ink-3)">防失控</text>
  <path d="M338 392 C 392 392 404 366 462 366" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 392 L 462 392" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <path d="M338 392 C 392 392 404 418 462 418" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <circle cx="470" cy="366" r="2.4" fill="var(--ink-2)"/><text x="483" y="370.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Trellis</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  计划 → 执行 → 沉淀</tspan></text>
  <circle cx="470" cy="392" r="2.4" fill="var(--ink-2)"/><text x="483" y="396.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">GSD</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  spec 驱动,一段一段 ship</tspan></text>
  <circle cx="470" cy="418" r="2.4" fill="var(--ink-2)"/><text x="483" y="422.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Task Master</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  PRD 拆任务清单</tspan></text>
  <!-- 分支 5 · 设计台 -->
  <text x="252" y="465" font-family="'Noto Serif SC',serif" font-size="14" font-weight="600" fill="var(--ink)">设计台</text>
  <text x="252" y="482" font-family="'Noto Sans SC',system-ui" font-size="10" letter-spacing="3" fill="var(--ink-3)">桌面应用</text>
  <path d="M338 460 L 462 460" stroke="var(--line)" stroke-width="1.1" stroke-linecap="round"/>
  <circle cx="470" cy="460" r="2.4" fill="var(--ink-2)"/><text x="483" y="464.5"><tspan font-family="'JetBrains Mono',monospace" font-size="12" font-weight="600" fill="var(--ink)">Open Design</tspan><tspan font-family="'Noto Sans SC',system-ui" font-size="11.5" fill="var(--ink-3)">  出图出原型出 PPT</tspan></text>
</svg>
</figure>

<h2>三种安装方式 · 两家对照</h2>
<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">装 MCP — 终端一条命令,两家几乎一样</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>claude mcp add 名字 -- 启动命令</code></pre><span class="docs-cli">Codex</span><pre><code>codex mcp add 名字 -- 启动命令</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">装插件 — 在各自对话框里逛商店</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>/plugin</code></pre><span class="docs-cli">Codex</span><pre><code>/plugins</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">装 Skill — 把文件夹拷进各自的目录</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>~/.claude/skills/</code></pre><span class="docs-cli">Codex</span><pre><code>~/.agents/skills/</code></pre></div>
  </div>
</div>

<h2>场景速查</h2>
<figure class="docs-map">
<svg viewBox="0 0 880 470" fill="none" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="按场景选工具对照图">
  <text x="370" y="18" text-anchor="end" font-family="'JetBrains Mono',monospace" font-size="10" font-weight="700" letter-spacing="2.5" fill="var(--ink-3)">你的场景</text>
  <text x="510" y="18" font-family="'JetBrains Mono',monospace" font-size="10" font-weight="700" letter-spacing="2.5" fill="var(--ink-3)">装这个</text>
  <text x="370" y="63" text-anchor="end" font-family="'Noto Sans SC',system-ui" font-size="13.5" fill="var(--ink-2)">怕 AI 用过时 API 写错代码</text>
  <line x1="384" y1="58" x2="496" y2="58" stroke="var(--line)" stroke-width="1.2"/>
  <circle cx="384" cy="58" r="2.2" fill="var(--ink-3)"/><circle cx="496" cy="58" r="2.2" fill="var(--ink-3)"/>
  <text x="510" y="63" font-family="'JetBrains Mono',monospace" font-size="13" font-weight="700" fill="var(--ink)">Context7</text>
  <text x="370" y="117" text-anchor="end" font-family="'Noto Sans SC',system-ui" font-size="13.5" fill="var(--ink-2)">仓库太大,token 烧得快</text>
  <line x1="384" y1="112" x2="496" y2="112" stroke="var(--line)" stroke-width="1.2"/>
  <circle cx="384" cy="112" r="2.2" fill="var(--ink-3)"/><circle cx="496" cy="112" r="2.2" fill="var(--ink-3)"/>
  <text x="510" y="117" font-family="'JetBrains Mono',monospace" font-size="13" font-weight="700" fill="var(--ink)">Serena · CodeGraph</text>
  <text x="370" y="171" text-anchor="end" font-family="'Noto Sans SC',system-ui" font-size="13.5" fill="var(--ink-2)">改完前端想让 AI 自己验收</text>
  <line x1="384" y1="166" x2="496" y2="166" stroke="var(--line)" stroke-width="1.2"/>
  <circle cx="384" cy="166" r="2.2" fill="var(--ink-3)"/><circle cx="496" cy="166" r="2.2" fill="var(--ink-3)"/>
  <text x="510" y="171" font-family="'JetBrains Mono',monospace" font-size="13" font-weight="700" fill="var(--ink)">Playwright MCP</text>
  <text x="370" y="225" text-anchor="end" font-family="'Noto Sans SC',system-ui" font-size="13.5" fill="var(--ink-2)">要查最新资料 / 版本 / 价格</text>
  <line x1="384" y1="220" x2="496" y2="220" stroke="var(--line)" stroke-width="1.2"/>
  <circle cx="384" cy="220" r="2.2" fill="var(--ink-3)"/><circle cx="496" cy="220" r="2.2" fill="var(--ink-3)"/>
  <text x="510" y="225" font-family="'JetBrains Mono',monospace" font-size="13" font-weight="700" fill="var(--ink)">Exa</text>
  <text x="370" y="279" text-anchor="end" font-family="'Noto Sans SC',system-ui" font-size="13.5" fill="var(--ink-2)">想快速读懂别人的开源项目</text>
  <line x1="384" y1="274" x2="496" y2="274" stroke="var(--line)" stroke-width="1.2"/>
  <circle cx="384" cy="274" r="2.2" fill="var(--ink-3)"/><circle cx="496" cy="274" r="2.2" fill="var(--ink-3)"/>
  <text x="510" y="279" font-family="'JetBrains Mono',monospace" font-size="13" font-weight="700" fill="var(--ink)">DeepWiki · Understand-Anything</text>
  <text x="370" y="333" text-anchor="end" font-family="'Noto Sans SC',system-ui" font-size="13.5" fill="var(--ink-2)">bug 修不动,想换个脑子</text>
  <line x1="384" y1="328" x2="496" y2="328" stroke="var(--line)" stroke-width="1.2"/>
  <circle cx="384" cy="328" r="2.2" fill="var(--ink-3)"/><circle cx="496" cy="328" r="2.2" fill="var(--ink-3)"/>
  <text x="510" y="333" font-family="'JetBrains Mono',monospace" font-size="13" font-weight="700" fill="var(--ink)">Codex 双模型</text>
  <text x="370" y="387" text-anchor="end" font-family="'Noto Sans SC',system-ui" font-size="13.5" fill="var(--ink-2)">项目改着改着就失控</text>
  <line x1="384" y1="382" x2="496" y2="382" stroke="var(--line)" stroke-width="1.2"/>
  <circle cx="384" cy="382" r="2.2" fill="var(--ink-3)"/><circle cx="496" cy="382" r="2.2" fill="var(--ink-3)"/>
  <text x="510" y="387" font-family="'JetBrains Mono',monospace" font-size="13" font-weight="700" fill="var(--ink)">Trellis</text>
  <text x="370" y="441" text-anchor="end" font-family="'Noto Sans SC',system-ui" font-size="13.5" fill="var(--ink-2)">要出海报 / 网页原型 / PPT</text>
  <line x1="384" y1="436" x2="496" y2="436" stroke="var(--line)" stroke-width="1.2"/>
  <circle cx="384" cy="436" r="2.2" fill="var(--ink-3)"/><circle cx="496" cy="436" r="2.2" fill="var(--ink-3)"/>
  <text x="510" y="441" font-family="'JetBrains Mono',monospace" font-size="13" font-weight="700" fill="var(--ink)">Open Design</text>
</svg>
</figure>

<p class="docs-tip">所有教程默认你已装好其中一家 CLI。还没装?先看 <a href="/docs?cat=tools&amp;page=claude-code">Claude Code CLI 接入</a> 或 <a href="/docs?cat=tools&amp;page=codex">Codex CLI 接入</a>,顺便把本站 Key 配上。</p>`,
      },
      {
        id: 'learn-ai',
        title: "协作心法① · 先搞懂 AI 怎么花 token",
        summary: "学习 → 要求 → 约束 · 知道钱花在哪,才知道怎么省",
        html: `<p class="docs-lead">同样一个活,有人几块钱搞定,有人花十倍。差距不在模型,在<strong>你怎么跟它说话</strong>。这一系列四页讲的就是怎么少花冤枉钱 —— Claude Code 和 Codex 通用。</p>

<h2>三步走</h2>
<figure class="docs-map">
<svg viewBox="0 0 880 220" fill="none" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="学习 AI 要求 AI 约束 AI 三阶段">
  <text x="120" y="58" text-anchor="middle" font-family="'Noto Serif SC',serif" font-size="20" font-weight="600" fill="var(--ink)">学习 AI</text>
  <text x="120" y="86" text-anchor="middle" font-family="'Noto Sans SC',system-ui" font-size="12.5" fill="var(--ink-2)">先懂它怎么干活</text>
  <text x="120" y="106" text-anchor="middle" font-family="'Noto Sans SC',system-ui" font-size="11" fill="var(--ink-3)">它会"读 + 猜 + 试错"</text>
  <path d="M232 70 L 318 70" stroke="var(--line)" stroke-width="1.3"/>
  <path d="M318 70 l -8 -4 l 0 8 z" fill="var(--ink-3)"/>
  <text x="440" y="58" text-anchor="middle" font-family="'Noto Serif SC',serif" font-size="20" font-weight="600" fill="var(--ink)">要求 AI</text>
  <text x="440" y="86" text-anchor="middle" font-family="'Noto Sans SC',system-ui" font-size="12.5" fill="var(--ink-2)">把任务说清楚</text>
  <text x="440" y="106" text-anchor="middle" font-family="'Noto Sans SC',system-ui" font-size="11" fill="var(--ink-3)">你说得越多,它猜得越少</text>
  <path d="M552 70 L 638 70" stroke="var(--line)" stroke-width="1.3"/>
  <path d="M638 70 l -8 -4 l 0 8 z" fill="var(--ink-3)"/>
  <text x="760" y="58" text-anchor="middle" font-family="'Noto Serif SC',serif" font-size="20" font-weight="600" fill="var(--ink)">约束 AI</text>
  <text x="760" y="86" text-anchor="middle" font-family="'Noto Sans SC',system-ui" font-size="12.5" fill="var(--ink-2)">把规矩写进配置</text>
  <text x="760" y="106" text-anchor="middle" font-family="'Noto Sans SC',system-ui" font-size="11" fill="var(--ink-3)">让它默认就照你的来</text>
  <line x1="40" y1="150" x2="840" y2="150" stroke="var(--line-soft)" stroke-width="1"/>
  <text x="40" y="182" font-family="'JetBrains Mono',monospace" font-size="11.5" letter-spacing="1" fill="var(--ink-3)">这一页</text>
  <text x="440" y="182" text-anchor="middle" font-family="'JetBrains Mono',monospace" font-size="11.5" letter-spacing="1" fill="var(--ink-3)">第 ② 页</text>
  <text x="840" y="182" text-anchor="end" font-family="'JetBrains Mono',monospace" font-size="11.5" letter-spacing="1" fill="var(--ink-3)">第 ③④ 页</text>
</svg>
</figure>

<h2>钱花在"猜"上,不是花在"写"上</h2>
<p>这些 CLI 背后是个 <strong>agent</strong>。接到活,它会先<strong>读</strong>你的代码,想<strong>怎么改</strong>,动手<strong>试</strong>,跑错了再<strong>调</strong>。每一步都在花 token。</p>
<p>最费钱的是 <strong>"猜"</strong> 这一步:你给的信息越少,它越要靠想象去填空。填错了就回头重试,一来一回,钱就上去了。</p>
<p class="docs-tip"><strong>一句话:</strong> 你说不清 → 它瞎猜 → 猜错 → 重试、重调、重做 → 钱蹭蹭涨。后面所有技巧,干的都是同一件事:<strong>别让它猜</strong>。</p>

<h2>所以"省 token"不是让你少打字</h2>
<p>正好相反 —— 一开始把话说清楚,反而最省。前面多花一句话,后面省下整轮返工。怎么把话说清楚,看下一页。</p>`,
      },
      {
        id: 'instruct-ai',
        title: "协作心法② · 把任务说清楚",
        summary: "划边界 · 反馈给坐标 · 先出计划 · 大活拆开",
        html: `<p class="docs-lead">这一页全是"要求 AI"的实操。核心就一句:你说得越具体,它猜得越少,钱花得越少。</p>

<h2>1. 开口前,先划好边界</h2>
<p>最常见的浪费,就是甩一句"帮我做个落地页"然后撒手不管。AI 只能自己猜:用什么组件库?什么配色?要不要动效?猜的过程都在花钱,猜出来还多半不合你意,又得返工。</p>
<p>把脑子里想要的样子尽量说出来,它就不用猜了:</p>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>说啥</th><th>含糊(费钱)</th><th>具体(省钱)</th></tr></thead>
<tbody>
<tr><td>技术栈</td><td>"做个前端页"</td><td>"Vue 3 + Tailwind,组件用 shadcn-vue"</td></tr>
<tr><td>内容</td><td>"放点介绍"</td><td>"一句 slogan + 3 个卖点卡 + 一个按钮"</td></tr>
<tr><td>风格</td><td>"好看点"</td><td>"黑白极简,大标题,留白多,像 Linear 官网"</td></tr>
<tr><td>动效</td><td>"加点动画"</td><td>"卡片滚进来淡入上移,别用第三方库"</td></tr>
<tr><td>边界</td><td>(没说)</td><td>"静态页,不用加 loading 和错误处理"</td></tr>
</tbody>
</table>
</div>
<p class="docs-tip">拿不准的地方,直接写"这块你拿不准就先问我"。一句话,省掉一整轮试错。</p>

<h2>2. 反馈要说"哪里",别只说"不对"</h2>
<p>AI 做错了,你回一句"这样不对",它是听懂了,但不知道<strong>哪里</strong>不对,只能把代码从头翻一遍找问题 —— 翻一遍就是钱。你心里其实清楚问题在哪,直接告诉它,等于帮它把范围从"整个项目"缩到"这一行"。</p>
<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">✕</span>
    <div class="docs-step-title">这样说 —— 它得重新翻一遍找错</div>
    <div class="docs-step-body"><pre><code>&gt; 不对,重做</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">✓</span>
    <div class="docs-step-title">这样说 —— 哪里错、怎么改、为什么、要什么</div>
    <div class="docs-step-body"><pre><code>&gt; 登录按钮点了没反应。问题在 LoginForm.vue 的 onSubmit,
&gt; login() 那个 promise 没 await,所以转圈状态没起来。
&gt; 改成 await,失败的话在 catch 里弹个 toast。</code></pre></div>
  </div>
</div>
<p>记住四件事:<strong>哪里错、怎么改、为什么、我要啥</strong>。说全了,它一步到位。</p>

<h2>3. 复杂点的活,先让它说计划</h2>
<p>别让 AI 一上来就闷头改。先要它说思路 —— 动哪些文件、打算怎么改、有什么风险。你瞄一眼,不对就拦,比它吭哧吭哧写完一大坨再推倒重来便宜多了。</p>
<div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>&gt; 先别动代码,说下思路:改哪些文件、怎么改、有啥风险,
&gt; 我点头你再写。（也可以按 Shift+Tab 进 Plan Mode）</code></pre><span class="docs-cli">Codex</span><pre><code>&gt; 先出方案,等我确认再写。</code></pre></div>

<h2>4. 活太大,拆开交给子 agent</h2>
<p>"调研下这三个库哪个好""把这 20 个文件扫一遍,找出用了旧 API 的地方" —— 这种<strong>要查一大堆</strong>的活,交给子 agent 去干。它在一个单独的空间里翻,翻完只把<strong>结论</strong>给你,中间那一堆搜索、读文件的杂音不会塞进你的主对话,主线反而更清爽更省。</p>
<p class="docs-tip">一句话:<strong>主对话保持干净,脏活累活外包。</strong>凡是"得读一大堆才能回答"的,先想想能不能丢给子 agent。</p>`,
      },
      {
        id: 'habits-ai',
        title: "协作心法③ · 两个省钱好习惯",
        summary: "勤 commit 留存档点 · 换任务就清上下文",
        html: `<p class="docs-lead">两个小习惯,养成了一直在帮你省钱。都很简单。</p>

<h2>1. 做好一段就 commit</h2>
<p>一个功能做完、自己点了没问题,就 <strong>commit 一次</strong>。好处有两个:</p>
<ul>
  <li>AI 读项目常常从 commit 历史入手,记录清楚,它一眼就懂"这次在啥基础上改"。</li>
  <li>万一后面改崩了,有干净的存档点能直接退回去,不用花钱让 AI 帮你"想办法救"。</li>
</ul>
<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">一段做完且自测过,马上提交</div>
    <div class="docs-step-body"><pre><code>git add -A
git commit -m "feat: 登录按钮加转圈和错误提示"</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">下一步改崩了?退回上一个干净点</div>
    <div class="docs-step-body"><pre><code>git restore .        ← 丢掉没提交的改动,回到刚才那次 commit</code></pre></div>
  </div>
</div>
<p class="docs-tip">vibe coding 最怕"改了俩小时全乱了,还说不清哪步崩的"。勤 commit = 随时能读档重来。</p>

<h2>2. 换任务就清上下文</h2>
<p>一段对话越拖越长,AI 每次回答都得把前面全部重读一遍 —— 哪怕你早换话题了。聊完一件事,要开下一件不相关的,先清掉,等于帮它卸下不用的包袱。</p>
<div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>/clear        ← 清空当前对话,开新活之前来一发</code></pre><span class="docs-cli">Codex</span><pre><code>/new          ← 开个全新会话</code></pre></div>
<p class="docs-tip">判断很简单:<strong>"接下来这件事,用得上刚才聊的吗?"</strong> 用不上就清。同一件事干到一半别清(会丢掉刚建立的理解),换事一定清。</p>`,
      },
      {
        id: 'constrain-ai',
        title: "协作心法④ · CLAUDE.md / AGENTS.md 模板",
        summary: "把规矩写进配置,一次到位 · 附可直接抄的脱敏模板",
        html: `<p class="docs-lead">前面那些都得"每次手动说"。最省事的办法,是把反复要交代的规矩,一次性写进配置文件,让 AI 每次开工自动照着办。这就是"约束"。</p>

<h2>写哪个文件</h2>
<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">●</span>
    <div class="docs-step-title">Claude Code 读 CLAUDE.md</div>
    <div class="docs-step-body"><pre><code>~/.claude/CLAUDE.md       ← 全局,所有项目都生效
&lt;项目根&gt;/CLAUDE.md          ← 项目级,优先级更高</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">●</span>
    <div class="docs-step-title">Codex 读 AGENTS.md</div>
    <div class="docs-step-body"><pre><code>~/.codex/AGENTS.md        ← 全局
&lt;项目根&gt;/AGENTS.md          ← 项目级,优先级更高</code></pre></div>
  </div>
</div>
<p>两边语法一样,都是普通 Markdown。约束写得越具体,AI 越稳,你要重复的废话越少。</p>

<h2>该写什么(五类)</h2>
<ul>
  <li><strong>读代码用什么</strong> —— 例:"探索代码先取骨架,别一上来就全文件 grep"</li>
  <li><strong>遇到啥情况怎么办</strong> —— 例:"用第三方库前先查最新文档,别凭记忆写 API"</li>
  <li><strong>什么不许做</strong> —— 例:"别自己加没要求的兜底逻辑""改完先跑 lint 和测试再说完成"</li>
  <li><strong>怎么跟我说话</strong> —— 例:"中文,先给结论再说细节,别长篇大论"</li>
  <li><strong>项目的坑</strong> —— 例:"node_modules 很大别递归扫;某某目录没权限,绕开"</li>
</ul>

<h2>拿去就能改的模板</h2>
<p>下面是一份精简模板,两家通用 —— 存成 <code>CLAUDE.md</code> 给 Claude Code 用,同样内容存成 <code>AGENTS.md</code> 给 Codex 用。按自己的项目改几句就行。</p>
<pre><code># 协作准则

## 沟通
- 默认中文。代码、命令、路径、报错保持原文。
- 先给结论 + 关键理由,别长篇大论。
- 可能变化的信息(版本、价格、库 API)先查证再说,别凭记忆。
- 没验证过别说"完成""通过""可提交"。

## 干活原则
- 先把需求问清楚再动手;先小范围,别一上来铺很大。
- 改之前先读相关文件、配置、最近几条 commit。
- 我要方案 / 计划时,别擅自改源码。
- 优先最小改动,别加没要求的兜底和包装。
- 复杂活先出计划等我确认;简单活直接做。
- 别动我没提交的改动;有冲突先停下问我。

## 读代码
- 先看文件 / 模块骨架,再下钻到具体函数。
- 找字符串 / 配置用 rg;别用全盘递归 grep。
- 大目录(node_modules 等)和没权限的目录,绕开别扫。

## 改完要做
- 跑 lint / 类型检查 / 测试,绿了再说完成。
- 给 interface 加方法,记得把所有 mock / stub 一起补。

## 不许做
- 别 mock 掉测试、跳过测试当"通过"。
- 别凭训练数据猜第三方库的新版 API。
- 别在我没要求时加降级 / 兜底分支(会改变可见行为)。

## 提交
- 中文提交信息,格式:类型: 简短描述
- 类型:feat / fix / refactor / docs / test / chore
</code></pre>
<p class="docs-tip"><strong>约束 = 给 AI 装护栏。</strong>写进去的每一条,都是以后不用再说第无数遍的话。一份顺手的 CLAUDE.md / AGENTS.md,能让它第一次出手就八九不离十。用一阵子,把你反复纠正的点慢慢补进去,它会越来越懂你。</p>

<h2>四页串起来</h2>
<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">配置先放好</div>
    <div class="docs-step-body"><p>项目里有 CLAUDE.md / AGENTS.md,规矩都写好了。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">说清楚再下达</div>
    <div class="docs-step-body"><p>带上技术栈 / 内容 / 风格 / 边界,复杂的先让它出计划。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">纠错给坐标</div>
    <div class="docs-step-body"><p>"哪里错 + 怎么改 + 为什么",别只说"不对"。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">4</span>
    <div class="docs-step-title">勤提交、勤清场</div>
    <div class="docs-step-body"><p>一段稳了就 commit;换任务就 <code>/clear</code> 或 <code>/new</code>。</p></div>
  </div>
</div>
<p class="docs-tip">这一章后面介绍的工具(<a href="/docs?cat=vibe-coding&amp;page=context7">Context7</a> 查文档、<a href="/docs?cat=vibe-coding&amp;page=serena">Serena</a> / <a href="/docs?cat=vibe-coding&amp;page=codegraph">CodeGraph</a> 省读码 token、<a href="/docs?cat=vibe-coding&amp;page=trellis">Trellis</a> 防失控),都是这套心法的自动化版本 —— 心法是内功,工具是兵器。</p>`,
      },
      {
        id: 'context7',
        title: "Context7 · 让 AI 查最新文档",
        summary: "告别过时 API · 零注册零配置 · 一条命令接入",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Context7</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">MCP</span>
      <span class="docs-launch-tag">零注册</span>
    </div>
    <div class="docs-launch-blurb">AI 的训练数据是旧的。Context7 让它写码前先查一遍最新官方文档,告别"这个 API 早就改名了"。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/upstash/context7" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">upstash/context7</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">连 — 一条命令接入,用哪家抄哪行</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>claude mcp add context7 -- npx -y @upstash/context7-mcp@latest</code></pre><span class="docs-cli">Codex</span><pre><code>codex mcp add context7 -- npx -y @upstash/context7-mcp@latest</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">用 — 提问时点名即可</div>
    <div class="docs-step-body"><pre><code>&gt; 用 context7 查一下 Vue 3.6 的 defineModel 怎么用,然后帮我改这个组件</code></pre></div>
  </div>
</div>

<p class="docs-tip">不用注册、不要 Key。写新框架 / 新库的代码时让 AI"先查 context7 再动手",能避开 90% 的过时 API 坑。</p>`,
      },
      {
        id: 'serena',
        title: "Serena · 符号级代码导航",
        summary: "按函数 / 类精准读代码 · 大仓库 token 省一大半",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Serena</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">MCP</span>
      <span class="docs-launch-tag">需要 uv</span>
    </div>
    <div class="docs-launch-blurb">给 AI 装"代码导航仪":按函数、类、引用关系精准读代码,不再整个文件硬吞。大仓库提效立竿见影。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/oraios/serena" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">oraios/serena</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">装 uv — Python 工具管理器,装过的跳过</div>
    <div class="docs-step-body"><pre><code>curl -LsSf https://astral.sh/uv/install.sh | sh</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">连 — 用哪家抄哪行</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>claude mcp add serena -- uvx --from git+https://github.com/oraios/serena \\
  serena start-mcp-server --context ide-assistant</code></pre><span class="docs-cli">Codex</span><pre><code>codex mcp add serena -- uvx --from git+https://github.com/oraios/serena \\
  serena start-mcp-server --context ide-assistant</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">用 — 首次进项目先让它"认门"</div>
    <div class="docs-step-body"><pre><code>&gt; 用 serena 激活当前项目并完成 onboarding</code></pre><p>之后"找出谁调用了 createOrder""把 UserService 重命名"这类活,它做得又快又准。</p></div>
  </div>
</div>

<p class="docs-tip">几个文件的小项目用不上它;上百个文件的仓库提效最明显 — 省下的 token 都是钱。</p>`,
      },
      {
        id: 'playwright-mcp',
        title: "Playwright MCP · AI 自己开浏览器",
        summary: "点页面 · 填表单 · 截图 · 改完前端自己验收",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Playwright MCP</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">MCP</span>
      <span class="docs-launch-tag">微软官方</span>
    </div>
    <div class="docs-launch-blurb">AI 自己打开浏览器:点页面、填表单、截图、看 console 报错。改完前端不用你来回切窗口,它自己验收。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/microsoft/playwright-mcp" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">microsoft/playwright-mcp</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">连 — 用哪家抄哪行</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>claude mcp add playwright -- npx -y @playwright/mcp@latest</code></pre><span class="docs-cli">Codex</span><pre><code>codex mcp add playwright -- npx -y @playwright/mcp@latest</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">用 — 直接下指令</div>
    <div class="docs-step-body"><pre><code>&gt; 打开 localhost:3000,点"登录"按钮,截图给我看</code></pre><p>首次运行会自动下载浏览器内核,等一两分钟。</p></div>
  </div>
</div>

<p class="docs-tip">调试时不加参数,能亲眼看着 AI 操作浏览器;想后台静默跑,启动命令末尾加 <code>--headless</code>。</p>`,
      },
      {
        id: 'exa',
        title: "Exa · AI 联网搜索",
        summary: "查最新资讯 / 版本 / 报错解法 · 回答带来源",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Exa</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">MCP</span>
      <span class="docs-launch-tag">需要免费 Key</span>
    </div>
    <div class="docs-launch-blurb">给 AI 接上搜索引擎:查最新资讯、版本号、价格、报错解法,回答带来源链接,不再凭记忆瞎猜。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/exa-labs/exa-mcp-server" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">exa-labs/exa-mcp-server</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">领 Key — 免费注册,个人额度够用</div>
    <div class="docs-step-body"><p>打开 <code>dashboard.exa.ai</code>,注册后复制 API Key。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">连 — 注意两家传 Key 的参数名不同</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>claude mcp add exa -e EXA_API_KEY=你的Key -- npx -y exa-mcp-server</code></pre><span class="docs-cli">Codex</span><pre><code>codex mcp add exa --env EXA_API_KEY=你的Key -- npx -y exa-mcp-server</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">用</div>
    <div class="docs-step-body"><pre><code>&gt; 搜一下 Tailwind 4 正式版有哪些破坏性变更</code></pre></div>
  </div>
</div>

<p class="docs-tip">时效性问题(版本 / 价格 / 新闻)一句"搜一下"比让 AI 凭记忆猜靠谱得多 — 它的记忆停留在训练截止日。</p>`,
      },
      {
        id: 'deepwiki',
        title: "DeepWiki · 把 GitHub 仓库变成百科",
        summary: "零安装 · 官方在线服务 · 读懂陌生开源项目从几天变几分钟",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">DeepWiki</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">MCP</span>
      <span class="docs-launch-tag">零安装</span>
      <span class="docs-launch-tag">在线服务</span>
    </div>
    <div class="docs-launch-blurb">Devin 团队出品。把任何公开 GitHub 仓库变成"能问答的百科" — 架构、原理、实现细节,直接问。</div>
  </div>
  <a class="docs-launch-link" href="https://deepwiki.com" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">官网</span>
    <span class="docs-launch-link-host">deepwiki.com</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">连 — 官方在线服务,本机不用装任何东西</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>claude mcp add --transport http deepwiki https://mcp.deepwiki.com/mcp</code></pre><span class="docs-cli">Codex</span><pre><code>codex mcp add deepwiki --url https://mcp.deepwiki.com/mcp</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">用 — 报上"作者/仓库名"开问</div>
    <div class="docs-step-body"><pre><code>&gt; 用 deepwiki 查 vuejs/core:响应式系统是怎么实现的?</code></pre></div>
  </div>
</div>

<p class="docs-tip">和 Exa 搭配食用:Exa 管"全网搜",DeepWiki 管"深挖某一个仓库"。冷门仓库可先去 deepwiki.com 搜一下确认已收录。</p>`,
      },
      {
        id: 'codegraph',
        title: "CodeGraph · 给仓库建代码地图",
        summary: "谁调用谁 / 改哪影响哪 · 索引常驻 · 查询亚毫秒",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">CodeGraph</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">MCP</span>
      <span class="docs-launch-tag">Node.js</span>
    </div>
    <div class="docs-launch-blurb">给你的仓库建一张"代码地图":谁调用谁、改这里会影响哪。AI 查图毫秒出结果,不用每次现翻代码。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/colbymchenry/codegraph" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">colbymchenry/codegraph</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">装</div>
    <div class="docs-step-body"><pre><code>npm install -g @colbymchenry/codegraph</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">建索引 — 在项目根目录跑一次</div>
    <div class="docs-step-body"><pre><code>codegraph init</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">连 — 用哪家抄哪行</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>claude mcp add codegraph -- codegraph serve --mcp</code></pre><span class="docs-cli">Codex</span><pre><code>codex mcp add codegraph -- codegraph serve --mcp</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">4</span>
    <div class="docs-step-title">用 — 架构问题随便问</div>
    <div class="docs-step-body"><pre><code>&gt; 这个项目的支付回调是怎么走的?从入口到落库讲一遍</code></pre></div>
  </div>
</div>

<p class="docs-tip">索引建好后文件改动自动同步,不用重跑。本站开发时就在用它做代码导航,7 万多个符号节点随查随到。</p>`,
      },
      {
        id: 'codex-duo',
        title: "Codex 插件 · 双模型互相挑错",
        summary: "Claude 写码 GPT 复查 · 一个看不出的 bug 另一个常一眼看穿",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Codex 插件(双模型互查)</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">Claude Code 插件</span>
      <span class="docs-launch-tag">需 Codex CLI</span>
      <span class="docs-launch-tag">OpenAI 官方</span>
    </div>
    <div class="docs-launch-blurb">把 OpenAI Codex 装进 Claude Code 当"第二双眼睛":Claude 动手写,GPT 在旁边审,两个模型互相挑错。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/openai/codex-plugin-cc" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">openai/codex-plugin-cc</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">装 Codex CLI</div>
    <div class="docs-step-body"><pre><code>npm install -g @openai/codex</code></pre><p>还没配 Key?看 <a href="/docs?cat=tools&amp;page=codex">Codex CLI 接入</a>,用本站 Key 直接跑。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">装插件 — 在 Claude Code 对话框里输</div>
    <div class="docs-step-body"><pre><code>/plugin marketplace add openai/codex-plugin-cc
/plugin install codex@openai-codex</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">用 — 卡住时召唤第二意见</div>
    <div class="docs-step-body"><pre><code>&gt; 这个 bug 我修了三次都不对,让 codex 来诊断一下</code></pre></div>
  </div>
</div>

<p class="docs-tip">两个模型的盲区不重叠。实践里最值钱的用法是"Claude 动手,Codex 审查" — 修不动的疑难杂症换个脑子常常秒破。</p>`,
      },
      {
        id: 'official-plugins',
        title: "官方插件市场 · /plugin 一键逛",
        summary: "代码审查 / 前端品味 / GitHub 集成 · 全是官方维护",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">官方插件市场</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">插件</span>
      <span class="docs-launch-tag">内置</span>
      <span class="docs-launch-tag">零安装</span>
      <span class="docs-launch-tag">两家各有</span>
    </div>
    <div class="docs-launch-blurb">两家 CLI 都自带"应用商店":Claude Code 输 /plugin,Codex 输 /plugins。代码审查、前端品味、GitHub 集成…全是官方维护。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/anthropics/claude-plugins-official" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">anthropics/claude-plugins-official</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">逛 — 在各自对话框里输</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>/plugin</code></pre><span class="docs-cli">Codex</span><pre><code>/plugins</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">装 — Claude Code 用户推荐先装这三个</div>
    <div class="docs-step-body"><pre><code>/plugin install code-review@claude-plugins-official      ← 代码审查
/plugin install frontend-design@claude-plugins-official  ← 前端不再"AI 味"
/plugin install github@claude-plugins-official           ← PR / issue 集成</code></pre><p>Codex 这边在 <code>/plugins</code> 列表里直接选 OpenAI 精选(安全扫描、Slack、Google Drive 等),回车即装。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">用</div>
    <div class="docs-step-body"><pre><code>/code-review    ← 提交前把当前分支审一遍</code></pre></div>
  </div>
</div>

<p class="docs-tip">写 TypeScript / Go / Rust 的把对应语言服务插件也装上(如 <code>typescript-lsp</code>),AI 改代码时能实时拿到类型报错,少跑很多冤枉路。两家插件互不通用,别拿着 Claude 的命令去 Codex 里敲。</p>`,
      },
      {
        id: 'understand-anything',
        title: "Understand-Anything · 代码库知识图谱",
        summary: "一条命令画出整个项目的架构图 · 接手老项目神器",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Understand-Anything</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">插件</span>
      <span class="docs-launch-tag">可视化</span>
      <span class="docs-launch-tag">仅 Claude Code</span>
    </div>
    <div class="docs-launch-blurb">一条命令把陌生代码库画成可交互知识图谱:模块、调用关系、业务流程全可视化。接手老项目、读开源源码的神器。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/Egonex-AI/Understand-Anything" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">Egonex-AI/Understand-Anything</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">装 — 在 Claude Code 对话框里输</div>
    <div class="docs-step-body"><pre><code>/plugin marketplace add Lum1104/Understand-Anything
/plugin install understand-anything@understand-anything</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">跑分析 — 在目标项目里</div>
    <div class="docs-step-body"><pre><code>/understand</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">看图 — 浏览器里打开交互式图谱</div>
    <div class="docs-step-body"><pre><code>/understand-dashboard</code></pre></div>
  </div>
</div>

<p class="docs-tip">团队来新人?给他跑一个 <code>/understand-onboard</code>,自动生成项目上手指南,比口头交接靠谱。</p>`,
      },
      {
        id: 'skills-intro',
        title: "Skill 入门 · 教 AI 新套路",
        summary: "一个 Markdown 文件就是一个技能 · 官方仓库白嫖现成的",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Agent Skills</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">Skill</span>
      <span class="docs-launch-tag">零代码</span>
    </div>
    <div class="docs-launch-blurb">Skill 就是教 AI 新套路的 Markdown 说明书:放进目录就生效。Claude Code 和 Codex 用的是同一套开放标准,一份 Skill 两家都能吃。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/anthropics/skills" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">anthropics/skills</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">认路 — 两家目录不同,内容通用</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>~/.claude/skills/        ← 全局,所有项目可用
项目/.claude/skills/     ← 只对这个项目生效</code></pre><span class="docs-cli">Codex</span><pre><code>~/.agents/skills/        ← 全局,所有项目可用
项目/.agents/skills/     ← 只对这个项目生效</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">装 — 从官方仓库挑着拷</div>
    <div class="docs-step-body"><pre><code>git clone https://github.com/anthropics/skills
cp -r skills/想要的技能目录 ~/.claude/skills/    # Codex 用户拷到 ~/.agents/skills/</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">用 — 自然提到就触发,也可以点名</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>&gt; 把这份报告导出成 PDF        ← 自动匹配,或 /技能名 直接调</code></pre><span class="docs-cli">Codex</span><pre><code>&gt; $技能名 把这份报告导出成 PDF   ← $ 点名,/skills 看已装清单</code></pre></div>
  </div>
</div>

<p class="docs-tip">自己写一个超简单:新建文件夹放一个 <code>SKILL.md</code>,开头写 name 和 description,正文写你想让 AI 记住的套路。Codex 用户更省事 — 内置 <code>$skill-creator</code>,对话里召唤它替你写。</p>`,
      },
      {
        id: 'superpowers',
        title: "Superpowers · 一键灌入开发方法论",
        summary: "头脑风暴、实施计划、测试驱动与系统化调试",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Superpowers</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">插件</span>
      <span class="docs-launch-tag">方法论</span>
    </div>
    <div class="docs-launch-blurb">Claude Code 圈最火的"超能力包"。一次安装,给 AI 灌进一整套久经考验的开发方法论:先头脑风暴、再写计划、测试驱动、系统化调试 — 几十个技能按需自动触发。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/obra/superpowers" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">obra/superpowers</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">装 — 官方插件市场直装,在 Claude Code 里输</div>
    <div class="docs-step-body"><pre><code>/plugin install superpowers@claude-plugins-official</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">用 — 装完就生效,正常提需求即可</div>
    <div class="docs-step-body"><pre><code>&gt; 我想给项目加一个导出功能</code></pre><p>它会拦住"直接开写"的冲动:先和你头脑风暴需求,再写计划,然后测试驱动地实现 — 全程方法论上身。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">进阶 — 作者自己的扩展市场还有配套插件</div>
    <div class="docs-step-body"><pre><code>/plugin marketplace add obra/superpowers-marketplace</code></pre></div>
  </div>
</div>

<p class="docs-tip">和 Trellis 不冲突:Superpowers 练的是"怎么想问题"的内功,Trellis 管的是"任务怎么流转"的流程,可以同时装。</p>`,
      },
      {
        id: 'trellis',
        title: "Trellis · 让 vibe coding 不失控",
        summary: "先计划再动手 · 经验自动沉淀 · 跨会话不失忆",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Trellis</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">工作流</span>
      <span class="docs-launch-tag">Node.js</span>
      <span class="docs-launch-tag">跨平台</span>
    </div>
    <div class="docs-launch-blurb">vibe coding 最大的坑不是写不出来,是"改着改着失控"。Trellis 给 AI 套流程:先计划、再动手、完了沉淀经验,跨会话不失忆。</div>
  </div>
  <a class="docs-launch-link" href="https://www.npmjs.com/package/@mindfoldhq/trellis" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">npm</span>
    <span class="docs-launch-link-host">@mindfoldhq/trellis</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">装</div>
    <div class="docs-step-body"><pre><code>npm install -g @mindfoldhq/trellis</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">初始化 — 在项目根目录</div>
    <div class="docs-step-body"><pre><code>trellis init</code></pre><p>生成 <code>.trellis/</code> 工作流目录,自动适配 Claude Code / Cursor / Codex 等主流平台。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">用 — 新需求别直接开写</div>
    <div class="docs-step-body"><pre><code>&gt; 我想加一个导出功能,先帮我把需求理清楚</code></pre><p>AI 会走"理需求 → 写计划 → 实现 → 自查"的完整流程,所有决策落在文档里,随时能回看。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">4</span>
    <div class="docs-step-title">翻旧账 — 跨会话记忆</div>
    <div class="docs-step-body"><pre><code>trellis mem "上次那个支付 bug 怎么修的"</code></pre></div>
  </div>
</div>

<p class="docs-tip">本站的开发全程跑在 Trellis 上 — 你正在读的这篇文档,本身就是一个 Trellis 任务的产物。</p>`,
      },
      {
        id: 'gsd',
        title: "GSD · Git. Ship. Done",
        summary: "用规格约束任务并分阶段交付",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">GSD(Get Shit Done)</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">工作流</span>
      <span class="docs-launch-tag">全平台</span>
      <span class="docs-launch-tag">spec 驱动</span>
    </div>
    <div class="docs-launch-blurb">将需求澄清、规格编写和分阶段实现组织成可复用的工作流。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/open-gsd/gsd-core" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">open-gsd/gsd-core</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">装 — 一条命令,装的时候选你用的 CLI</div>
    <div class="docs-step-body"><pre><code>npx @opengsd/gsd-core@latest</code></pre><p>安装器会问你装给谁:Claude Code / Codex / Cursor / Gemini CLI 等全支持,选完自动配好。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">开项目 — 在 CLI 对话框里输</div>
    <div class="docs-step-body"><pre><code>/gsd-new-project</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">用 — 跟着引导走完一轮</div>
    <div class="docs-step-body"><p>定 spec → 拆阶段 → 一段一段实现、验收、ship。每一步都有命令接住,AI 不会越写越飘。</p></div>
  </div>
</div>

<p class="docs-tip">GSD 适合从零起步的新项目;接手已有项目想要流程管控,看上一篇 Trellis。</p>`,
      },
      {
        id: 'task-master',
        title: "Task Master · 把 PRD 拆成任务清单",
        summary: "AI 任务管理 · 喂一份 PRD 自动拆任务排依赖 · 做完一个划一个",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Task Master</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">MCP</span>
      <span class="docs-launch-tag">两家通吃</span>
    </div>
    <div class="docs-launch-blurb">AI 任务管理系统:把一份 PRD 喂给它,自动拆成带依赖关系的任务清单。AI 按序领工、做完销项,大需求不再"做着做着忘了全貌"。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/eyaltoledano/claude-task-master" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">eyaltoledano/claude-task-master</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">连 — 用哪家抄哪行</div>
    <div class="docs-step-body"><span class="docs-cli">Claude Code</span><pre><code>claude mcp add taskmaster-ai -- npx -y task-master-ai</code></pre><span class="docs-cli">Codex</span><pre><code>codex mcp add taskmaster-ai -- npx -y task-master-ai</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">配 Key — 它拆任务时自己也要调模型</div>
    <div class="docs-step-body"><p>在项目根目录 <code>.env</code> 或 MCP 配置的 env 里填模型 API Key(支持 Anthropic / OpenAI / OpenRouter 等多家)。</p></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">用 — 喂 PRD,然后按清单干活</div>
    <div class="docs-step-body"><pre><code>&gt; 用 taskmaster 解析 docs/prd.md,拆成任务清单
&gt; 下一个该做哪个任务?</code></pre></div>
  </div>
</div>

<p class="docs-tip">不开 AI 对话也能用:<code>npm i -g task-master-ai</code> 后,终端里 <code>task-master init</code> / <code>task-master parse-prd</code> 直接管任务。</p>`,
      },
      {
        id: 'open-design',
        title: "Open Design · AI 设计工作台",
        summary: "说句需求出网页 / 海报 / PPT / 视频 · agent 时代的 Figma 替代品",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Open Design</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">桌面 App</span>
      <span class="docs-launch-tag">macOS</span>
      <span class="docs-launch-tag">Windows</span>
      <span class="docs-launch-tag">Apache-2.0</span>
    </div>
    <div class="docs-launch-blurb">本地优先的开源 AI 设计工作台:说句需求,出网页、海报、PPT、视频原型。被称为"agent 时代的 Figma 替代品"。</div>
  </div>
  <a class="docs-launch-link" href="https://github.com/nexu-io/open-design" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">nexu-io/open-design</span>
  </a>
</div>

<div class="docs-steps">
  <div class="docs-step">
    <span class="docs-step-no">1</span>
    <div class="docs-step-title">下载 — 选对应系统的安装包,双击装完即用</div>
    <div class="docs-step-body"><pre><code>https://github.com/nexu-io/open-design/releases</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">2</span>
    <div class="docs-step-title">接模型 — 推荐 BYOK 填本站网关</div>
    <div class="docs-step-body"><p>它会自动检测本机的 Claude Code / Codex 直接复用;也可以在 Settings 里走 BYOK 模式,填本站网关后 GPT / Claude / Gemini 一个 Key 全搞定:</p><pre><code>base_url: https://api.your-domain.com/v1
api_key:  sk-你的本站Key</code></pre></div>
  </div>
  <div class="docs-step">
    <span class="docs-step-no">3</span>
    <div class="docs-step-title">用 — 选一套 Design System,一句话需求</div>
    <div class="docs-step-body"><pre><code>&gt; 给我的咖啡店做一张周年庆海报,复古风</code></pre></div>
  </div>
</div>

<p class="docs-tip">自带 150 套品牌级 Design System 和 261 个现成插件。先选风格再下需求,出品稳定度高得多。</p>`,
      },
    ],
  },
]

const IMAGE_API_PAGE_IDS = new Set([
  'text-to-image-api',
  'batch-image-api',
  'async-image-tasks',
])
const TOOL_PAGE_IDS = new Set(['claude-code', 'codex', 'gemini-cli'])
const imageApiPages =
  PUBLIC_DOC_CONTENT_ZH_SOURCE.find((category) => category.id === 'tutorial')?.pages.filter(
    (page) => IMAGE_API_PAGE_IDS.has(page.id),
  ) ?? []

export const PUBLIC_DOC_CONTENT_ZH: PublicDocCategoryContent[] =
  PUBLIC_DOC_CONTENT_ZH_SOURCE.map((category) => {
    if (category.id === 'tutorial') {
      return {
        ...category,
        pages: category.pages.filter((page) => !IMAGE_API_PAGE_IDS.has(page.id)),
      }
    }
    if (category.id === 'deploy') {
      return {
        ...category,
        pages: [
          ...imageApiPages,
          ...category.pages
            .filter((page) => !TOOL_PAGE_IDS.has(page.id))
            .map((page) => getToolGuide('zh', page.id) ?? page),
        ],
      }
    }
    if (category.id === 'tools') {
      const deployToolPages =
        PUBLIC_DOC_CONTENT_ZH_SOURCE.find((source) => source.id === 'deploy')?.pages.filter((page) => TOOL_PAGE_IDS.has(page.id)) ?? []
      return {
        ...category,
        pages: [...deployToolPages, ...category.pages].map((page) => getToolGuide('zh', page.id) ?? page),
      }
    }
    return category
  })

export function findDocContent(catId: string, pageId: string) {
  const cat = PUBLIC_DOC_CONTENT_ZH.find((c) => c.id === catId)
  return cat?.pages.find((p) => p.id === pageId)
}

export function defaultDocPageId(catId: string) {
  return PUBLIC_DOC_CONTENT_ZH.find((c) => c.id === catId)?.pages[0]?.id
}
