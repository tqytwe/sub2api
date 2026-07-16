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
<p>访问首页右上角 <code>登录 / 注册</code>，使用邮箱注册。新用户默认为 <strong>V0</strong>，可在 <a href="/play">玩法中枢</a> 查看 VIP 进度。</p>
<h2>第 2 步 · 充值（可选）</h2>
<p>侧边栏 <code>充值</code> 选择金额完成支付。充值成功后会累加 VIP 累计金额，并可能触发 <strong>24h 充值加成</strong>（签到双倍、盲盒额外次数等，需管理员开启）。</p>
<h2>第 3 步 · 创建 API Key</h2>
<p>控制台 <code>API 密钥</code> → <code>新建</code> → 选择<strong>服务分组</strong>（决定可用模型与倍率）→ 命名保存。Key 仅展示一次，请立即复制。</p>
<h2>第 4 步 · 发起请求</h2>
<p>本站兼容 OpenAI 与 Anthropic 协议。下面两种调用方式任选其一（将 <code>your-host</code> 换成你的网关域名）：</p>
<h3>OpenAI 兼容（推荐通用 SDK）</h3>
<pre><code>curl https://your-host/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}'</code></pre>
<h3>Anthropic Messages API</h3>
<pre><code>curl https://your-host/v1/messages \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}'</code></pre>
<p class="docs-tip">完整模型清单与实时单价见 <a href="/models">模型与价格</a>。游客可看精选预览，登录后显示你分组下的<strong>真实可用模型</strong>与倍率。</p>`,
      },
      {
        id: 'text-to-image-api',
        title: "GPT / Grok 图片生成 API",
        summary: "真实 API 地址、单张/多张生成、prompt、保存与网页显示",
        html: `<p class="docs-lead">极速蹬当前图片生成主要使用 <strong>GPT 图片模型</strong> 和 <strong>Grok 图片模型</strong>。开发者直接调用 Images API，图像编辑调用 Edits API。</p>
<pre><code>POST https://api.jisudeng.com/v1/images/generations
POST https://api.jisudeng.com/v1/images/edits</code></pre>

<h2>真实入口</h2>
<ul>
  <li>网站：<a href="https://www.jisudeng.com">https://www.jisudeng.com</a></li>
  <li>API Key：<a href="https://www.jisudeng.com/keys">https://www.jisudeng.com/keys</a></li>
  <li>模型与价格：<a href="https://www.jisudeng.com/models">https://www.jisudeng.com/models</a></li>
  <li>生成接口：<code>POST https://api.jisudeng.com/v1/images/generations</code></li>
  <li>编辑接口：<code>POST https://api.jisudeng.com/v1/images/edits</code></li>
</ul>

<h2>鉴权</h2>
<pre><code>Authorization: Bearer sk-xxxxxxxxxxxxxxx
Content-Type: application/json</code></pre>
<p>也兼容 <code>x-api-key</code> 和 <code>x-goog-api-key</code>。不要把 Key 放到 URL 的 <code>key</code> / <code>api_key</code> 参数里。</p>

<h2>可用模型</h2>
<ul>
  <li>GPT：<code>gpt-image-2</code>、<code>gpt-image-1.5</code>、<code>gpt-image-1</code>。不传 <code>model</code> 时默认 <code>gpt-image-2</code>。</li>
  <li>Grok：<code>grok-imagine</code>、<code>grok-imagine-image-quality</code>、<code>grok-imagine-image</code>、<code>grok-imagine-edit</code>。Grok 必须传 <code>model</code>。</li>
</ul>
<p>最终能用哪些模型，以你的 API Key 所属分组在 <a href="/models">模型与价格</a> 页面显示为准。</p>

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
<p>请求使用 <code>"response_format": "url"</code> 时，每张图片位于 <code>data[].url</code>。不同上游可能返回临时公网 URL 或 data URL，客户端都应按 URL 字段读取并及时保存。</p>
<pre><code>{
  "created": 1784160000,
  "data": [
    {"url": "https://example.com/generated/image.png"}
  ]
}</code></pre>

<h3>异步任务返回</h3>
<p>长耗时请求可提交到 <code>/v1/images/generations/async</code> 或 <code>/v1/images/edits/async</code>，先收到 <code>202</code> 和 <code>task_id</code>，再轮询 <code>/v1/images/tasks/{task_id}</code>。完整 processing / completed / failed 契约见 <a href="/docs?cat=deploy&amp;page=async-image-tasks">异步图片任务</a>。</p>

<h2>单张生成</h2>
<pre><code>curl https://api.jisudeng.com/v1/images/generations \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
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
    "model": "gpt-image-2",
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
  "model": "gpt-image-2",
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
      model: "gpt-image-2",
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
    model: "gpt-image-2",
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
    "model": "gpt-image-2",
    "prompt": "把背景替换成明亮的摄影棚渐变背景，保留主体",
    "images": [{"image_url": "https://example.com/source.png"}],
    "size": "1024x1024",
    "response_format": "b64_json"
  }'</code></pre>
<p>如果要直接上传文件，用 multipart：</p>
<pre><code>curl https://api.jisudeng.com/v1/images/edits \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -F "model=gpt-image-2" \\
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
<tr><td><code>model</code></td><td>GPT 用 <code>gpt-image-2</code>；Grok 用 <code>grok-imagine</code></td></tr>
<tr><td><code>prompt</code></td><td>图片描述，生成和编辑都放这里</td></tr>
<tr><td><code>n</code></td><td>输出张数，默认 1；多张就填 2、4 等</td></tr>
<tr><td><code>size</code></td><td>GPT 常用 <code>1024x1024</code>、<code>1024x1536</code>、<code>1536x1024</code>；Grok 会用于站内计费分档</td></tr>
<tr><td><code>response_format</code></td><td>推荐 <code>b64_json</code>，方便保存和直接显示</td></tr>
<tr><td><code>images</code></td><td>编辑接口的参考图 URL 数组</td></tr>
<tr><td><code>image</code> / <code>image[]</code></td><td>multipart 编辑接口的本地图片字段</td></tr>
</tbody>
</table>
</div>

<h2>常见错误</h2>
<ul>
  <li><code>400 invalid_request_error</code> — JSON 写错、请求体为空、<code>n &lt;= 0</code>、编辑接口缺少图片</li>
  <li><code>401 API_KEY_REQUIRED / INVALID_API_KEY</code> — Key 缺失或无效</li>
  <li><code>403 permission_error</code> — Key 所属分组没有开启图片生成</li>
  <li><code>404 not_found_error</code> — Key 所属分组不是 GPT/OpenAI 或 Grok 图片分组</li>
  <li><code>413 invalid_request_error</code> — multipart 文件或文本字段超过 20 MiB，或整个请求超过部署上限</li>
  <li><code>429</code> — Key、用户、分组、账号或上游限流</li>
  <li><code>502 upstream_error</code> — 上游异常或没有返回有效图片</li>
</ul>

<h2>和图像工作室的区别</h2>
<ul>
  <li><strong>API 生成</strong>：开发者直调 <code>/v1/images/generations</code>，Base URL 是 <code>https://api.jisudeng.com</code>，自己保存和展示图片。</li>
  <li><strong>图像工作室</strong>：打开 <a href="/image-studio">/image-studio</a> 手动生成，站内负责模板、估价、异步 job 和图库。</li>
  <li><strong>批量调用</strong>：多个 prompt 或一次多张，见 <a href="/docs?cat=deploy&amp;page=batch-image-api">多张 / 批量生图调用</a>。</li>
  <li><strong>异步调用</strong>：避免长连接超时，见 <a href="/docs?cat=deploy&amp;page=async-image-tasks">异步图片任务</a>。</li>
</ul>`,
      },
      {
        id: 'batch-image-api',
        title: "多张 / 批量生图调用",
        summary: "同一个 GPT/Grok 图片接口：n 多张、多 prompt 批量跑、保存本地",
        html: `<p class="docs-lead">批量调用 GPT/Grok 图片生成时，继续使用同一个 Images API。常见方式有两种：同一个 prompt 设置 <code>n</code> 一次出多张；多个 prompt 循环调用接口并保存结果。</p>
<pre><code>POST https://api.jisudeng.com/v1/images/generations</code></pre>

<h2>两种批量方式</h2>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>方式</th><th>怎么做</th><th>适合场景</th></tr></thead>
<tbody>
<tr><td>一次请求多张</td><td>设置 <code>n</code>，例如 <code>n: 4</code></td><td>同一个 prompt 出多张备选图</td></tr>
<tr><td>多个 prompt 批量跑</td><td>循环调用 <code>/v1/images/generations</code></td><td>商品图、封面、头像、素材批处理</td></tr>
</tbody>
</table>
</div>
<p class="docs-tip">GPT/Grok 批量调用主要就是调用上面的 <code>/v1/images/generations</code> 接口，Base URL 是 <code>https://api.jisudeng.com</code>。不要换成其他地址，也不要把 API Key 放到 URL 参数里。</p>

<h2>一次请求生成多张</h2>
<pre><code>curl https://api.jisudeng.com/v1/images/generations \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
    "prompt": "电商白底主图：一双白色运动鞋，居中摆放，柔和棚拍光，高清真实摄影",
    "size": "1024x1024",
    "n": 4,
    "response_format": "b64_json"
  }' \\
  -o multi-response.json</code></pre>
<pre><code>mkdir -p outputs
jq -r '.data[].b64_json' multi-response.json | nl -w1 -s' ' | while read -r index b64; do
  printf '%s' "$b64" | base64 -d &gt; "outputs/shoe-$index.png"
done</code></pre>

<h2>多个 prompt 批量跑</h2>
<p>准备 <code>prompts.txt</code>，一行一个 prompt：</p>
<pre><code>电商白底主图：白色运动鞋，柔光，干净阴影
电商白底主图：黑色机械键盘，俯拍，高清细节
社媒封面：夏日冰咖啡，浅色背景，标题留白
游戏头像：银发机甲角色，蓝色边缘光，半身像</code></pre>
<p>批量生成并保存：</p>
<pre><code>mkdir -p batch-outputs
i=1
while IFS= read -r prompt || [ -n "$prompt" ]; do
  [ -z "$prompt" ] &amp;&amp; continue

  jq -n --arg prompt "$prompt" '{
    model: "gpt-image-2",
    prompt: $prompt,
    size: "1024x1024",
    n: 1,
    response_format: "b64_json"
  }' &gt; "batch-outputs/request-$i.json"

  curl https://api.jisudeng.com/v1/images/generations \\
    -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
    -H "Content-Type: application/json" \\
    -d @"batch-outputs/request-$i.json" \\
    -o "batch-outputs/response-$i.json"

  jq -r '.data[0].b64_json' "batch-outputs/response-$i.json" \\
    | base64 -d &gt; "batch-outputs/image-$i.png"

  i=$((i + 1))
done &lt; prompts.txt</code></pre>
<p>要用 Grok，把脚本里的 <code>model: "gpt-image-2"</code> 改成 <code>model: "grok-imagine"</code>。</p>

<h2>每个 prompt 生成多张</h2>
<p>每行 prompt 出 3 张时，把请求里的 <code>n</code> 改成 3，再保存每条响应里的所有图片：</p>
<pre><code>jq -r '.data[].b64_json' "batch-outputs/response-$i.json" | nl -w1 -s' ' | while read -r j b64; do
  printf '%s' "$b64" | base64 -d &gt; "batch-outputs/image-$i-$j.png"
done</code></pre>

<h2>Node.js 批量保存</h2>
<pre><code>import fs from "node:fs"

const apiKey = "sk-xxxxxxxxxxxxxxx"
const prompts = [
  "电商白底主图：白色运动鞋，柔光，干净阴影",
  "社媒封面：夏日冰咖啡，浅色背景，标题留白",
  "游戏头像：银发机甲角色，蓝色边缘光，半身像",
]

fs.mkdirSync("batch-outputs", { recursive: true })

for (let i = 0; i &lt; prompts.length; i++) {
  const res = await fetch("https://api.jisudeng.com/v1/images/generations", {
    method: "POST",
    headers: {
      Authorization: "Bearer " + apiKey,
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
  json.data.forEach((item, j) =&gt; {
    fs.writeFileSync("batch-outputs/image-" + (i + 1) + "-" + (j + 1) + ".png", Buffer.from(item.b64_json, "base64"))
  })
}</code></pre>

<h2>网页批量显示</h2>
<pre><code>&lt;div id="grid" style="display:grid;grid-template-columns:repeat(2,180px);gap:12px;"&gt;&lt;/div&gt;
&lt;script&gt;
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
  document.querySelector("#grid").innerHTML = json.data.map(function(item) {
    const src = item.url || ("data:image/png;base64," + item.b64_json)
    return '&lt;img src="' + src + '" style="width:180px;border-radius:10px;" /&gt;'
  }).join("")
}
run()
&lt;/script&gt;</code></pre>
<p class="docs-tip">正式网站不要把 API Key 暴露在前端，请让你的后端调用极速蹬 API，再把图片结果返回给浏览器。</p>`,
      },
      {
        id: 'async-image-tasks',
        title: "异步图片任务",
        summary: "202 提交、任务轮询、对象存储 URL 与失败返回契约",
        html: `<p class="docs-lead">异步图片任务适合耗时较长的 GPT / Grok 生成与编辑请求。提交接口立即返回任务 ID，客户端按建议间隔轮询，不需要保持一条长 HTTP 连接。</p>
<pre><code>POST https://api.jisudeng.com/v1/images/generations/async
POST https://api.jisudeng.com/v1/images/edits/async
GET  https://api.jisudeng.com/v1/images/tasks/{task_id}</code></pre>
<p class="docs-tip">异步任务依赖对象存储。功能未启用或存储配置不完整时，上述接口返回 <code>404 not_found_error</code>，不会创建任务。</p>

<h2>提交任务</h2>
<p>请求体与同步生成或编辑接口相同。提交成功返回 <code>202 Accepted</code>，响应头同时包含 <code>Location</code> 和建议轮询间隔 <code>Retry-After: 3</code>。</p>
<pre><code>curl -i https://api.jisudeng.com/v1/images/generations/async \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
    "prompt": "暴风雪中的灯塔，电影感，横向构图",
    "size": "1536x1024",
    "n": 2
  }'</code></pre>
<pre><code>{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "processing",
  "created_at": 1784160000,
  "expires_at": 1784246400,
  "poll_url": "/v1/images/tasks/imgtask_0123456789abcdef"
}</code></pre>

<h2>轮询任务</h2>
<p>必须使用提交任务时的同一把 API Key：</p>
<pre><code>curl https://api.jisudeng.com/v1/images/tasks/imgtask_0123456789abcdef \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx"</code></pre>

<h3>处理中</h3>
<pre><code>{
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "processing",
  "created_at": 1784160000,
  "expires_at": 1784246400
}</code></pre>

<h3>已完成</h3>
<p>生成结果会先转存到对象存储，最终图片位于 <code>result.data[].url</code>；<code>image_url</code> 是第一张图片的便捷字段。异步结果不会把大段 <code>b64_json</code> 存入任务记录。</p>
<pre><code>{
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "completed",
  "http_status": 200,
  "image_url": "https://cdn.example.com/images/first.png",
  "result": {
    "created": 1784160123,
    "data": [
      {"url": "https://cdn.example.com/images/first.png"},
      {"url": "https://cdn.example.com/images/second.png"}
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
<p>成功提交和成功轮询响应都带 <code>Cache-Control: no-store</code>。未知任务 ID 与其他 API Key 拥有的任务都返回 404，避免泄露任务是否存在。</p>`,
      },
      {
        id: 'api-key',
        title: "API Key 管理",
        summary: "Key 的创建 / 启用 / 禁用 / 删除 / 限速",
        html: `<p class="docs-lead">API Key 是访问网关的凭据,每个 Key 绑定到一个服务分组,继承该分组的模型/费率/限制。</p>
<h2>创建 Key</h2>
<p>侧边栏 <code>API 密钥</code> → <code>新建</code>,需要选择 <strong>服务分组</strong>(决定可用模型和倍率)。</p>
<h2>启用 / 禁用</h2>
<p>在 Key 列表点 <code>启用 / 禁用</code> 切换状态。禁用后立即生效,不影响已生成的请求。</p>
<h2>限速</h2>
<p>每个 Key 可设置 <code>RPM</code>(每分钟请求数)上限。0 = 不限制(走所属分组兜底)。</p>
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
        html: `<p class="docs-lead">系统采用 三层限流 模型,优先级从高到低: <strong>Key 级别</strong> → <strong>账号级别</strong> → <strong>分组级别</strong>。</p>
<h2>账号并发</h2>
<p>个人资料页可看到 <code>并发限制</code>(默认 5)。同一时刻最多 5 个未完成请求。</p>
<h2>Key 级 RPM</h2>
<p>API Key 可单独设置 RPM。RPM=0 时回退到分组的 RPM 限制。</p>
<h2>分组限流</h2>
<p>分组的 RPM/TPM/日预算 是共享的兜底,所有该分组下的 Key 共用。</p>
<h2>命中限流如何处理?</h2>
<pre><code>HTTP/1.1 429 Too Many Requests
{"error":{"type":"rate_limit_exceeded","message":"..."}}</code></pre>
<p>客户端应实现 <strong>指数退避重试</strong>(初始 1s,最大 32s,最多 5 次)。</p>`,
      },
    ],
  },
  {
    id: 'recharge-vip',
    title: "充值与 VIP",
    description: "VIP 分级 · 享受折扣 · 充值流程 · 签到奖励",
    pages: [
      {
        id: 'vip-levels',
        title: "VIP 分级体系",
        summary: "4 档会员等级、对应折扣与省钱直观对照",
        html: `<p class="docs-lead">按 <strong>累计充值</strong> 自动升级，档位永久保留。权益在 Play 中枢与模型页展示，计费引擎仍按分组倍率 × 上游单价扣费。</p>

<h2>升级规则</h2>
<ul>
  <li>每次充值成功自动累加累计金额，达到阈值即时升级</li>
  <li>档位<strong>永久保留</strong>，不会因消费或时间降级</li>
  <li>玩法奖励（签到、盲盒、Arena）不计入 VIP 累计</li>
</ul>

<h2>在哪里查看</h2>
<ul>
  <li><strong>玩法中枢</strong>（/play）— VIP 卡片与权益列表</li>
  <li><strong>模型页</strong>（/models）— V1+ 显示 VIP 徽章</li>
  <li><strong>充值页</strong> — 首充 / 余额低提醒与充值联动玩法</li>
</ul>`,
      },
      {
        id: 'how-to-recharge',
        title: "充值流程",
        summary: "4 种支付方式、几分钟完成充值、即刻到账",
        html: `<p class="docs-lead">充值的钱会 1:1 进入你的账户余额,既可用于后续按量计费的所有调用,又会自动累加到 VIP 升级进度里。</p>

<h2>支持的支付方式</h2>
<ul>
  <li><strong>微信支付</strong> — 扫码或在微信内 H5 直接打开</li>
  <li><strong>支付宝</strong> — 扫码或移动端打开</li>
  <li><strong>Stripe</strong> — 国际信用卡(Visa / Mastercard / Amex)</li>
  <li><strong>Airwallex</strong> — 企业银行账户</li>
</ul>

<h2>三步完成充值</h2>
<ol>
  <li>从侧边栏点 <strong>"充值"</strong> 进入充值页</li>
  <li>输入想充值的金额(最低 $1)→ 选择喜欢的支付方式</li>
  <li>按提示完成支付,通常 5-30 秒内到账</li>
</ol>
<p>支付成功后,首页会自动弹出一张<strong>纸张小票</strong> — 上面有订单号、金额、支付时间。可以拖动它放到屏幕喜欢的位置,双击就能关上。</p>

<h2>没看到到账?</h2>
<p>如果支付了但 1 分钟还没看到余额变化:</p>
<ol>
  <li>先刷新一下页面,大多数情况会立刻看到</li>
  <li>进入侧边栏的 <strong>"我的订单"</strong>页面查看这笔订单是否已经显示完成</li>
  <li>如果订单一直显示等待中,联系客服并提供订单号</li>
</ol>

<h2>找回历史小票</h2>
<p>每一笔成功的充值都可以重新查看小票:进入"我的订单",找到那笔订单,点 <strong>"查看小票"</strong> 就能再次打开纸张小票。</p>`,
      },
      {
        id: 'check-in',
        title: "每日签到",
        summary: "每日领余额 · 连续签到里程碑 · 充值可补签",
        html: `<p class="docs-lead">每日签到是 Play 域的日活入口。管理员可在后台配置奖励金额；默认每次签到赠送 <strong>$0.50</strong> 余额（可在 Settings 调整）。</p>

<h2>怎么签到</h2>
<ul>
  <li>控制台侧边栏 <strong>每日签到</strong>，或 <a href="/play">玩法中枢</a> 一键签到</li>
  <li>每天 0 点（服务器时区）重置，每人每天一次</li>
  <li>奖励直接进入余额，可用于 API 调用</li>
</ul>

<h2>连续签到（Streak）</h2>
<p>连续签到天数会累计展示。达到里程碑时额外发放 balance（默认配置）：</p>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>连续天数</th><th>额外奖励</th></tr></thead>
<tbody>
<tr><td>7 天</td><td>+$1</td></tr>
<tr><td>14 天</td><td>+$2</td></tr>
<tr><td>30 天</td><td>+$5</td></tr>
</tbody>
</table>
</div>
<p class="docs-tip">里程碑金额由 <code>play_checkin_streak_milestones</code> JSON 配置，以实际后台为准。</p>

<h2>充值补签</h2>
<p>若昨日未签到，且在 <strong>24 小时内有过充值</strong>，可使用 <code>POST /play/checkin/makeup</code> 补签昨日（需开启 <code>play_checkin_makeup_enabled</code>）。</p>

<h2>充值加成 24h</h2>
<p>充值成功后若开启 <code>play_recharge_boost_enabled</code>，24 小时内享受：</p>
<ul>
  <li>签到奖励 ×2（可配）</li>
  <li>盲盒每日额外开箱次数</li>
  <li>Arena 展示积分倍率加成</li>
</ul>

<h2>常见问题</h2>
<ul>
  <li><strong>签到余额能升 VIP 吗？</strong> — 不能，VIP 只看累计充值。</li>
  <li><strong>与 VIP 档位关系？</strong> — VIP 提供模型页徽章、盲盒/Arena 等玩法权益，不改变签到基础金额（除非运营单独配置活动）。</li>
</ul>`,
      },
      {
        id: 'discount-examples',
        title: "Play 权益一览",
        summary: "VIP 档位能解锁什么 · 玩法中枢怎么串联",
        html: `<p class="docs-lead">本站 VIP <strong>不改变计费公式</strong>（仍按上游单价 × 分组倍率），而是解锁身份标识与 Play 玩法权益。下面按实际代码行为说明。</p>

<h2>VIP 档位权益（默认配置）</h2>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>档位</th><th>累计充值</th><th>权益</th></tr></thead>
<tbody>
<tr><td>V0</td><td>$0</td><td>基础功能</td></tr>
<tr><td>V1</td><td>$50</td><td>模型页 VIP 徽章</td></tr>
<tr><td>V2</td><td>$200</td><td>+ 盲盒奖池升级标识</td></tr>
<tr><td>V3</td><td>$500</td><td>+ Arena 结算加成 · 邀请返利 +5%</td></tr>
</tbody>
</table>
</div>

<h2>Play 中枢（/play）聚合能力</h2>
<ul>
  <li><strong>图像工作室</strong> — 模板向导出图、今日是否已出图、首图引导</li>
  <li><strong>每日任务</strong> — 签到 / 出图 / API 调用进度与能量等级</li>
  <li><strong>签到</strong> — 今日是否已签、连续天数、距下一里程碑</li>
  <li><strong>Token 农场 / Arena</strong> — 日榜 + 月榜排名、距上一名 token 差、周期结算</li>
  <li><strong>盲盒</strong> — 今日剩余次数、是否可开</li>
  <li><strong>答题闯关</strong> — 今日题目提交状态</li>
  <li><strong>Agent Team</strong> — 小队进度与邀请合一链接</li>
  <li><strong>限时活动</strong> — 充值加赠、盲盒额外次数、Arena 倍率（Campaign 引擎）</li>
</ul>

<h2>充值联动</h2>
<p>充值成功后可能触发：</p>
<ul>
  <li><strong>Recharge Boost 24h</strong> — 签到双倍、盲盒额外次数、Arena 展示倍率（需开启）</li>
  <li><strong>首充 / 余额低 Banner</strong> — Dashboard 与 Hub 引导回充值页</li>
</ul>
<p class="docs-tip">所有 Play 模块均有 Admin 开关，未开启时 Hub 会显示引导文案而非报错。</p>`,
      },
      {
        id: 'faq',
        title: "常见问题",
        summary: "VIP 与充值最常被问到的 8 个问题",
        html: `<p class="docs-lead">下面汇总充值、VIP 与 Play 玩法最常见的问题。</p>

<h2>会员等级会过期吗?</h2>
<p>不会。达到某档位后永久保留，不会因长时间未使用而降级。</p>

<h2>VIP 会打折计费吗?</h2>
<p><strong>不会。</strong> VIP 提供模型页徽章、盲盒/Arena/邀请返利等 Play 权益。API 扣费仍按 <code>上游单价 × 分组倍率</code>，与 VIP 档位无关。</p>

<h2>赠送的余额能升 VIP 吗?</h2>
<p>不能。只有<strong>真实充值</strong>计入 VIP 累计。签到、盲盒、活动赠送不算。</p>

<h2>充值加成与活动能叠加吗?</h2>
<p>可以。Recharge Boost（24h）与 Campaign 活动规则（充值加赠、盲盒额外次数、Arena 倍率）在代码层可叠加，具体以 Hub 展示为准。</p>

<h2>玩法开关关闭怎么办?</h2>
<p>签到、Arena、盲盒、Quiz、Team 均有独立 Admin 开关。关闭后对应入口隐藏或 Hub 显示「暂未开启」引导。</p>

<h2>模型列表在哪里看?</h2>
<p>公开页 <a href="/models">/models</a>：游客看精选预览；登录后拉取你分组下的<strong>真实可用模型与单价</strong>。</p>`,
      },
      {
        id: 'blindbox-rewards',
        title: "盲盒玩法",
        summary: "扣费开箱 · 随机返余额 · 每日次数上限",
        html: `<p class="docs-lead">盲盒是 Play 域的随机奖励玩法：消耗少量余额开箱，随机返还更高或更低的 balance。与对标站「充值送盲盒」不同，<strong>本站默认是余额扣费开箱</strong>。</p>

<h2>基本规则（默认配置）</h2>
<ul>
  <li>每次开箱扣费 <strong>$0.50</strong>（<code>play_blindbox_cost</code>）</li>
  <li>每日上限 <strong>10 次</strong>（<code>play_blindbox_daily_limit</code>）</li>
  <li>随机返奖档位：$0.05 / $0.20 / $0.50 / $1.00 / $2.00（加权概率）</li>
  <li>净收益 = 返奖 − 扣费，直接写入余额</li>
</ul>

<h2>加成与上限</h2>
<ul>
  <li><strong>充值 Boost 24h</strong> — 额外每日开箱次数</li>
  <li><strong>限时活动 Campaign</strong> — 可配置 <code>blindbox_extra_opens</code></li>
  <li><strong>V2+ VIP</strong> — 展示「盲盒奖池升级」权益标识</li>
</ul>

<h2>入口</h2>
<p><a href="/blindbox">/blindbox</a> 或 <a href="/play">玩法中枢</a>。余额不足时引导充值。</p>

<h2>API</h2>
<ul>
  <li><code>GET /api/v1/play/blindbox/status</code> — 今日次数、是否可开</li>
  <li><code>POST /api/v1/play/blindbox/open</code> — 开箱（支持幂等键）</li>
</ul>
<p class="docs-tip">需管理员开启 <code>play_blindbox_enabled</code>。概率与扣费可在 Settings 调整。</p>`,
      },
      {
        id: 'image-studio',
        title: "图像工作室",
        summary: "模板向导 · 生成前估价 · 图库 · 与每日任务联动",
        html: `<p class="docs-lead">图像工作室面向<strong>不会写 prompt</strong>的用户：选意图 → 选模板 → 填描述 → 确认费用后生成。开发者批量调用请直接使用 GPT/Grok 图片生成 API，不需要进入图像工作室。</p>

<h2>入口</h2>
<ul>
  <li>控制台侧边栏 <strong>图像工作室</strong>（<a href="/image-studio">/image-studio</a>）</li>
  <li><a href="/play">玩法中枢</a>「今日出图」卡片</li>
  <li>首页 IMAGE 区 CTA「免费试做一张」</li>
</ul>

<h2>四步向导</h2>
<ol>
  <li><strong>意图</strong> — 电商白底 / 小红书封面 / 自由创作</li>
  <li><strong>模板</strong> — 每类默认 1 个模板（尺寸、合规提示已预填）</li>
  <li><strong>内容</strong> — 产品描述、主色、尺寸、数量；可折叠编辑专家 prompt</li>
  <li><strong>确认</strong> — 选择 API Key、查看预估费用与余额状态后生成</li>
</ol>
<p class="docs-tip">已充值用户默认生成 4 张变体；新用户默认 1 张，避免赠金过快耗尽。回访用户可跳过前两步，直达上次模板。</p>

<h2>费用与余额</h2>
<ul>
  <li>生成前调用 <code>GET /api/v1/image-studio/estimate</code> 展示预估费用</li>
  <li>余额不足时引导 <code>/purchase?return=/image-studio</code></li>
  <li>扣费与 Gateway 图像 API 同源，按所选 Key 的分组计费</li>
</ul>

<h2>图库与隐私</h2>
<ul>
  <li>最近任务可在页面底部图库查看、下载、删除</li>
  <li>可选 <strong>7 天自动清理</strong>（仅影响本地偏好与任务过期标记）</li>
  <li>Gateway 请求正文仍<strong>不入库</strong>；工作室任务表仅存 template_id 与合成 prompt（可配置不落盘）</li>
</ul>

<h2>与 Play 联动</h2>
<ul>
  <li>每日首次成功出图 → 完成「出图 1 张」任务（+20 能量）</li>
  <li>首次出图 → 首图仪式庆祝（客户端 localStorage）</li>
  <li>Hub <code>pending_actions</code> 含未完成任务与首图引导</li>
</ul>

<h2>API</h2>
<ul>
  <li><code>GET /api/v1/image-studio/templates</code></li>
  <li><code>GET /api/v1/image-studio/estimate</code></li>
  <li><code>POST /api/v1/image-studio/generate</code>（异步 job，前端轮询）</li>
  <li><code>GET /api/v1/image-studio/jobs</code> / <code>DELETE .../jobs/:id</code></li>
</ul>
<p class="docs-tip">需开启 <code>image_studio_enabled</code>。开发者直调请看 <a href="/docs?cat=deploy&amp;page=text-to-image-api">GPT / Grok 图片生成 API</a>；多 prompt 或一次多张请看 <a href="/docs?cat=deploy&amp;page=batch-image-api">多张 / 批量生图调用</a>。</p>`,
      },
      {
        id: 'token-farm',
        title: "Token 农场",
        summary: "日榜 + 月榜 · 每日任务 · RPG 能量等级",
        html: `<p class="docs-lead">Token 农场将 API Token 消耗可视化为 Ink 风格 RPG：每日任务攒能量、日榜即时反馈、月榜赛季大奖。底层仍使用 <code>usage_logs</code> 统计，不引入玩法代币。</p>

<h2>排行榜</h2>
<ul>
  <li><strong>日榜</strong> — 当日有效 Token 消耗排名；Top 10 次日自动发放小额余额奖励（日预算上限 $50）</li>
  <li><strong>月榜</strong> — 赛季周期排名；Admin 调用 <code>POST /admin/play/arena/settle</code> 结算大奖</li>
  <li>展示「距上一名还差 X tokens」与 Recharge Boost / Campaign Buff 倍率</li>
</ul>

<h2>每日任务（能量 → 等级，纯展示）</h2>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>任务</th><th>条件</th><th>能量</th></tr></thead>
<tbody>
<tr><td>签到</td><td>当日完成签到</td><td>+10</td></tr>
<tr><td>出图 1 张</td><td>图像工作室成功 1 次</td><td>+20</td></tr>
<tr><td>API 调用</td><td>当日 usage ≥ 100 tokens</td><td>+15</td></tr>
</tbody>
</table>
</div>
<p>能量与等级用于 HUD 进度条，<strong>不单独兑换余额</strong>（防刷量设计）。</p>

<h2>零消耗用户</h2>
<p>若日榜与月榜消耗均为 0，页面展示「播种指南」：创建 Key → 调用 API 或先去图像工作室出图。</p>

<h2>入口与 API</h2>
<ul>
  <li>页面 <a href="/arena">/arena</a> 或玩法中枢 Token 农场卡片</li>
  <li><code>GET /api/v1/play/arena/daily/current</code> · <code>.../daily/leaderboard</code></li>
  <li><code>GET /api/v1/play/quests/today</code></li>
</ul>
<p class="docs-tip">需开启 <code>play_arena_enabled</code> 与 <code>play_daily_arena_enabled</code>。对外文案：<strong>每日任务有奖 · 每月赛季大奖</strong>。</p>`,
      },
    ],
  },
  {
    id: 'about-us',
    title: "了解我们",
    description: "中转技术真伪 · 隐私边界 · 我们的承诺",
    pages: [
      {
        id: 'about-us-overview',
        title: "为什么我们是真的中转",
        summary: "不靠营销话术,用具体技术细节告诉你我们做了什么",
        html: `<p class="docs-lead">AI 中转站这个赛道,劣币驱逐良币的现象很严重。本节用<strong>可验证的技术事实</strong>,
说清楚我们做了什么、不做什么,以及你如何<strong>自己核实</strong>。营销话术不能保真,代码细节可以。</p>

<h2>1. 请求正文 · 永不入库</h2>
<p>你打过来的 prompt、消息内容、上下文、上传的图片、tool call 参数,<strong>从不写入我们的数据库或日志</strong>。
管理后台能看到的只有:</p>
<ul>
  <li><code>User-Agent</code> — 你用的客户端 (SDK 版本 / IDE)</li>
  <li><code>token 计数</code> — input_tokens, output_tokens, cached_tokens</li>
  <li><code>model</code> — 调用的模型名</li>
  <li><code>created_at</code> — 调用时间戳</li>
  <li><code>status_code</code> — HTTP 状态码 (200/429/500 等)</li>
  <li><code>upstream_account_id</code> — 调度到哪个上游账号</li>
  <li><code>latency_ms</code> — 端到端耗时</li>
</ul>
<p>请求体是<strong>流式直通</strong>到上游,中间不落盘、不缓存、不分析、不审计。响应体同理。</p>
<p class="docs-tip"><strong>自证方法:</strong> 联系客服调阅你自己 24 小时内任意一笔调用的"全部存储字段"。
你会发现里面没有 messages 数组、没有 prompt 字符串、没有 image base64。这是设计层面承诺,不是运营自觉。</p>

<h2>2. 上游账号 · 完整分类</h2>
<p>每个上游账号在管理面板按 <code>platform</code> 字段分类,我们公开承认每种类型的稳定性差异:</p>
<table class="docs-table">
  <thead><tr><th>类型</th><th>稳定性</th><th>说明</th></tr></thead>
  <tbody>
    <tr><td>官方 API Key (Anthropic / OpenAI / Google)</td><td>最高</td><td>付费购买的官方 console key,签合规协议</td></tr>
    <tr><td>官方 OAuth (Pro / Plus / Team / Enterprise)</td><td>高</td><td>登录 OAuth 拿 access token,有 rate limit 但稳定</td></tr>
    <tr><td>第三方 IDE 集成 (Cursor / Windsurf / Codex)</td><td>中</td><td>逆向 IDE 内嵌的官方账号,会跟随 IDE 风控调整</td></tr>
    <tr><td>Pro 订阅 (claude.ai / chat.openai.com)</td><td>低-中</td><td>个人账号订阅,有较严风控</td></tr>
  </tbody>
</table>
<p>我们<strong>不</strong>把"IDE 集成"伪装成"官方 API"卖。如果你抽中的是 IDE 通道,
错误日志和上游账号 ID 会明确显示来源。</p>

<h2>3. 计费可对账</h2>
<p>计费完全按上游 <code>usage</code> 字段。我们透传 <code>response_id</code> / <code>chat_id</code>,
你拿到这个 ID 后,如果有官方账号可以去后台对账:</p>
<pre><code># Anthropic 响应 (透传)
{
  "id": "msg_01ABC...",         // ← 这个 ID 在 Anthropic 后台可查
  "type": "message",
  "model": "claude-sonnet-4-6",
  "usage": {
    "input_tokens": 1234,        // ← 计费按这个
    "output_tokens": 567,
    "cache_read_input_tokens": 800,
    "cache_creation_input_tokens": 0
  }
}</code></pre>
<p>扣费 = 上游 <code>usage</code> token 数 × 该模型官方单价 × <strong>分组倍率</strong>。VIP 档位不改变此公式，仅影响 Play 玩法权益。</p>

<h2>4. 鉴别中转真伪 · 10 个测试项</h2>
<p>你可以拿下面 10 项去衡量<strong>任何一家</strong>中转 (包括我们):</p>
<ol>
  <li><strong>调用一个明确 400 的请求</strong> (例: temperature=999),看返回的 <code>error.type</code> 是否原样
  (真官方: <code>invalid_request_error</code> + 具体字段)</li>
  <li><strong>对比首 token 延迟</strong> 与官方 API 的差距 (官方一般 &lt;1s,中转应 &lt;1.5s)</li>
  <li><strong>测 prompt caching</strong> (Anthropic) — 第二次同 prefix 是否命中 cache,<code>cache_read_input_tokens</code> 是否&gt;0</li>
  <li><strong>测 vision</strong> — 上传一张图 (base64),看是否真的能 OCR/描述</li>
  <li><strong>测 tool use</strong> — 复杂 JSON schema 的 tool calling 是否能正确执行</li>
  <li><strong>测 stream usage</strong> — 流式响应末尾的 <code>message_delta.usage</code> 是否完整</li>
  <li><strong>对比同 prompt 在多家中转的输出</strong> (固定 temperature=0) — 应该几乎一致</li>
  <li><strong>查响应 header</strong> — 是否含 <code>x-request-id</code>、<code>anthropic-version</code> 等官方头</li>
  <li><strong>测长 context</strong> (200k tokens) — 真 Opus / Sonnet 能吃下,小模型会截断</li>
  <li><strong>测复杂推理</strong> — 数学竞赛题、代码 debug、长链条逻辑 — 这是 Opus 与 Haiku 最明显的差距</li>
</ol>

<h2>5. 行业常见掺水手法</h2>
<p>列在这里供识别。我们不做这些,你可以拿去对照其他中转。</p>

<h3>手法一 · IDE 逆向冒充官方 API</h3>
<p>抓 KIRO、Cursor、Windsurf、Codex 等 IDE 的 Anthropic 集成 cookie,
把这个端点伪装成"Claude 官方 API"卖。<strong>稳定性极差</strong>:
IDE 任何一次更新、Anthropic 改协议、对方账号被风控,你的"官方 API"立刻挂。
真官方 API 的 beta header、prompt caching、stream usage 末段统计往往不全。</p>

<h3>手法二 · Cookie 中转</h3>
<p>抓某个用户的 claude.ai / chat.openai.com 登录态 cookie 包装成 API 卖。
原账号被风控、改密码、cookie 过期,所有买家立刻断流。
而且明确<strong>违反服务条款</strong>,被发现可能反追溯。</p>

<h3>手法三 · 模型欺诈</h3>
<p>对外宣称 GPT-4 / Claude Opus,实际后端转发到 GPT-3.5 / Claude Haiku,
或用开源 Llama 微调套壳。<strong>固定一个高难度 prompt 多家对比</strong>就能识破。</p>

<h3>手法四 · token 虚标</h3>
<p>上游返回 usage 是 1000 token,中转改成 1500 多扣 50%。
<strong>对账方法:</strong> 拿透传的 <code>response_id</code> 去官方后台查实际 usage。
我们的扣费严格按上游 usage 字段,可对账。</p>

<h3>手法五 · 假"流式"</h3>
<p>非流式生成完后切片伪装流式 (首 token 延迟 5s+),或在流中段插入广告/统计。
我们的流是<strong>纯透传</strong>,首 token 延迟和官方差距只有网络往返。</p>

<h3>手法六 · 错误码改写</h3>
<p>把上游的 <code>overloaded_error</code> / <code>insufficient_quota</code> 等统一改写成
"服务繁忙"。<strong>掩盖了真实问题</strong>,客户端无法做精细化退避。
我们原样透传所有上游错误,你能定位到精确层。</p>

<h2>6. 我们对自己的要求 (公开承诺)</h2>
<ul>
  <li><strong>事故公开</strong> — 任何超过 5 分钟的中断在公告里留痕,不删历史</li>
  <li><strong>退款明文</strong> — 充值不可退,但余额可转给其他注册用户 (走客服)</li>
  <li><strong>价格不暗调</strong> — 模型单价改动提前 7 天预告</li>
  <li><strong>不卖渠道源</strong> — 不会把你的 API key、IP、调用模式打包卖给第三方画像</li>
  <li><strong>开源策略</strong> — 平台主体代码对自部署用户开放,商业模式不靠技术黑盒</li>
  <li><strong>日志保留 7 天</strong> — 错误响应可能保留 7 天用于排障 (仅 status + 响应头,不含正文),之后自动删除</li>
</ul>`,
      },
      {
        id: 'about-us-privacy',
        title: "隐私边界 · 我们存什么",
        summary: "详细的字段级隐私清单,逐项说明存什么不存什么",
        html: `<p class="docs-lead">这一节用<strong>字段级清单</strong>告诉你:我们后台数据库里每张表存了什么。
你的请求体、对话内容、上传文件,<strong>都不在这个列表里</strong>。</p>

<h2>usage_logs (调用流水)</h2>
<table class="docs-table">
  <thead><tr><th>字段</th><th>含义</th><th>包含内容?</th></tr></thead>
  <tbody>
    <tr><td>id</td><td>主键</td><td>—</td></tr>
    <tr><td>user_id</td><td>用户 ID</td><td>否</td></tr>
    <tr><td>api_key_id</td><td>API Key ID</td><td>否</td></tr>
    <tr><td>model</td><td>模型名</td><td>仅模型名,如 "claude-opus-4-7"</td></tr>
    <tr><td>input_tokens</td><td>输入 token 数</td><td>只是数字</td></tr>
    <tr><td>output_tokens</td><td>输出 token 数</td><td>只是数字</td></tr>
    <tr><td>cache_read_tokens</td><td>缓存读 token 数</td><td>只是数字</td></tr>
    <tr><td>actual_cost</td><td>实际扣费</td><td>金额（上游单价 × 分组倍率）</td></tr>
    <tr><td>request_id</td><td>请求 ID</td><td>UUID,用于去重</td></tr>
    <tr><td>response_id</td><td>上游响应 ID</td><td>上游返回的 msg_xxx</td></tr>
    <tr><td>client_ip</td><td>客户端 IP</td><td>用于风控,30 天后 GDPR-style 截断</td></tr>
    <tr><td>user_agent</td><td>客户端 UA</td><td>SDK/IDE 版本</td></tr>
    <tr><td>upstream_account_id</td><td>上游账号 ID</td><td>调度到哪个账号</td></tr>
    <tr><td>status_code</td><td>HTTP 状态码</td><td>数字</td></tr>
    <tr><td>latency_ms</td><td>耗时</td><td>毫秒数</td></tr>
    <tr><td>created_at</td><td>时间戳</td><td>—</td></tr>
  </tbody>
</table>
<p class="docs-tip"><strong>没有的字段:</strong> messages、prompt、system、tools、tool_results、images、audio、
上传文件 base64、response.content、stream chunks。</p>

<h2>临时错误样本 (出错时,保留 7 天)</h2>
<p>仅当上游返回 4xx/5xx 时,为了排障会临时保留:</p>
<ul>
  <li>响应 header (含 <code>x-request-id</code>、<code>retry-after</code> 等)</li>
  <li>响应 body 的 <code>error</code> 对象 (如 <code>{"type":"overloaded_error","message":"..."}</code>)</li>
  <li>请求 metadata (model, max_tokens 数值,<strong>不含 messages</strong>)</li>
</ul>
<p>7 天后自动删除。如果你不希望保留任何错误样本,可以在<code>个人中心 → 隐私</code>关掉。</p>

<h2>不会触碰的</h2>
<ul>
  <li>对话原文 / prompt</li>
  <li>tool call 参数和返回值</li>
  <li>图片 / 音频 / 文件上传内容</li>
  <li>streaming 中间内容</li>
  <li>你的 API key 自身 (只存 SHA-256 hash,前 8 位明文用于识别)</li>
</ul>`,
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
        summary: "当前网关支持的 Claude / GPT / Gemini 模型",
        html: `<p class="docs-lead">模型 ID 以 <a href="/models">模型与价格</a> 为准：游客看精选预览，登录后显示你分组下的<strong>实时可用列表与单价</strong>。下列为 2026 年 7 月主流 lineup（定价库已收录，实际上线取决于分组配置）。</p>

<h2>Anthropic Claude（2026-07）</h2>
<ul>
  <li><code>claude-opus-4-8</code> — 旗舰推理与 Agent，复杂任务首选</li>
  <li><code>claude-sonnet-4-6</code> — 速度与质量平衡，日常开发/对话主力</li>
  <li><code>claude-haiku-4-5</code> — 低延迟、低成本，适合高频小任务</li>
</ul>
<p class="docs-tip">Anthropic 于 2026-06 发布 Sonnet 5（<code>claude-sonnet-5</code>）等新模型；若你的分组已接入，会在模型页自动出现。旧版 Sonnet/Opus 4（如 <code>*-20250514</code>）已在官方侧退役，请勿在新项目中使用。</p>

<h2>OpenAI · GPT-5.6 系列（2026-07 最新）</h2>
<p>OpenAI 于 2026 年中发布 GPT-5.6 三档变体，均支持约 <strong>105 万 token</strong> 上下文窗口，适合长代码库与 Agent 工作流：</p>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>模型 ID</th><th>定位</th><th>典型场景</th></tr></thead>
<tbody>
<tr><td><code>gpt-5.6-sol</code></td><td>Sol · 旗舰推理</td><td>复杂架构设计、深度 Debug、多步 Agent</td></tr>
<tr><td><code>gpt-5.6-terra</code></td><td>Terra · 均衡型</td><td>日常开发、代码审查、文档生成</td></tr>
<tr><td><code>gpt-5.6-luna</code></td><td>Luna · 轻量高速</td><td>高频小任务、批量改写、快速补全</td></tr>
</tbody>
</table>
</div>

<h2>OpenAI · GPT-5.5 与 Codex 系</h2>
<ul>
  <li><code>gpt-5.5</code> / <code>gpt-5.5-pro</code> — 上一代旗舰，<strong>OpenAI Codex CLI 默认推荐模型</strong>，工具调用与代码生成极稳</li>
  <li><code>gpt-5-codex</code> — Codex 专用变体，走 Responses API</li>
  <li><code>gpt-5</code> / <code>gpt-5-mini</code> — 经典旗舰与轻量版，兼容旧项目</li>
  <li><code>gpt-4.1</code> — 指令遵循稳定，适合结构化输出</li>
  <li><code>o4-mini</code> / <code>o3</code> — 推理向模型，数学与逻辑任务</li>
</ul>
<p class="docs-tip"><strong>怎么选？</strong> 新项目优先 <code>gpt-5.6-sol</code> 或 <code>gpt-5.6-terra</code>；已配好 Codex 且不想改配置可继续用 <code>gpt-5.5</code>。三档 5.6 变体均支持 reasoning effort（low / medium / high / xhigh）。</p>

<h2>Google Gemini</h2>
<ul>
  <li><code>gemini-2.5-pro</code> — 长上下文与多模态</li>
  <li><code>gemini-2.5-flash</code> — 极速、低成本</li>
  <li><code>gemini-3-flash</code> — 2026 新一代 Flash 系列（分组接入后可用）</li>
</ul>

<h2>如何确认「我能用哪些」</h2>
<ol>
  <li>登录 → 打开 <a href="/models">/models</a></li>
  <li>查看当前分组支持的模型名与 input/output 单价</li>
  <li>调用时在 JSON 里填<strong>与列表完全一致</strong>的 model 字符串</li>
</ol>`,
      },
      {
        id: 'choose-model',
        title: "如何选模型",
        summary: "按场景推荐 · 成本/速度/质量权衡",
        html: `<p class="docs-lead">没有「最好」的模型，只有最适合场景与预算的组合。计费 = 上游 token 单价 × 分组倍率（与 VIP 档位无关）。</p>
<h2>场景速查（2026-07 推荐）</h2>
<div class="docs-table-wrap">
<table class="docs-table">
<thead><tr><th>场景</th><th>首选</th><th>备选</th><th>原因</th></tr></thead>
<tbody>
<tr><td>代码 / Agent / CLI</td><td>claude-sonnet-4-6</td><td>gpt-5.6-terra</td><td>工具调用稳、上下文长、性价比高</td></tr>
<tr><td>Codex / OpenAI CLI</td><td>gpt-5.5</td><td>gpt-5.6-sol</td><td>Codex 默认模型；5.6 Sol 推理更深</td></tr>
<tr><td>文档摘要 / 分类</td><td>claude-haiku-4-5</td><td>gpt-5.6-luna</td><td>便宜、延迟低</td></tr>
<tr><td>复杂推理 / 数学</td><td>claude-opus-4-8</td><td>gpt-5.6-sol</td><td>深度推理与长链路任务</td></tr>
<tr><td>多模态（图片）</td><td>gpt-5.6-sol</td><td>gemini-2.5-pro</td><td>视觉理解质量高</td></tr>
<tr><td>超长文本 (&gt;100K)</td><td>gemini-2.5-pro</td><td>gpt-5.6-terra</td><td>百万级 context 窗口</td></tr>
<tr><td>批量低成本</td><td>gpt-5.6-luna</td><td>gemini-3-flash</td><td>高并发、轻量任务</td></tr>
</tbody>
</table>
</div>
<h2>性价比策略</h2>
<p>「强模型起草 + 弱模型收尾」：例如 Opus / 5.6 Sol 出架构与关键逻辑，Haiku / 5.6 Luna 做格式化、摘要、批量改写。</p>
<p class="docs-tip">GPT-5.6 三档（Sol / Terra / Luna）上线后优先在 <a href="/models">模型页</a> 确认分组是否已开放；Codex 用户可继续用 <code>gpt-5.5</code> 或升级到 <code>gpt-5.6-terra</code>。</p>`,
      },
      {
        id: 'best-practices',
        title: "调用最佳实践",
        summary: "提示词模板 · 缓存策略 · 错误重试",
        html: `<p class="docs-lead">下面是经过验证的实战技巧,可以显著提升体验和降低成本。</p>
<h2>充分利用 Prompt Caching</h2>
<p>Claude 系列支持 prompt cache,缓存读取价格 ≈ 0.1× 原价。如果你有固定的 system prompt,加上缓存标记:</p>
<pre><code>{
  "model": "claude-sonnet-4-6",
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
    title: "接入部署",
    description: "图片 API · 主流 AI 编程 CLI 接入本站 · 一步到位",
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
<figure class="docs-figure">
  <div class="docs-figure-placeholder" data-shot="create-key">
    <span class="docs-figure-tag">截图位</span>
    <span class="docs-figure-hint">控制台 · 创建 API Key 弹窗</span>
  </div>
  <figcaption>在 <code>/keys</code> 页面新建 Key,选好服务分组后保存</figcaption>
</figure>

<h2>第 4 步 · 用 CC-Switch 一键添加本站</h2>
<p>把刚拿到的 Key 加进 CC-Switch:</p>
<pre><code>cc add gateway \\
  --base-url https://api.your-domain.com \\
  --token sk-xxxxxxxxxxxxxxx</code></pre>
<p><code>gateway</code> 是这个 provider 的本地名字,可随便取。完成后看一眼:</p>
<pre><code>cc list

  *  anthropic      (官方)
     gateway       https://api.your-domain.com    [新增]</code></pre>
<figure class="docs-figure">
  <div class="docs-figure-placeholder" data-shot="cc-switch-list">
    <span class="docs-figure-tag">截图位</span>
    <span class="docs-figure-hint">终端 · cc list 输出</span>
  </div>
  <figcaption>cc list 能看到 anthropic 与 gateway 两个 provider</figcaption>
</figure>

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
model = "gpt-5.5"

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
<p>创建一个支持 Gemini 模型的分组下的 Key。在侧边栏 <code>模型与价格</code> 可看到我们当前支持的 Gemini 模型清单(<code>gemini-2.5-pro</code> / <code>gemini-2.5-flash</code> 等)。</p>

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
    model="claude-sonnet-4-6",
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
    model="gpt-5",
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
  model: "claude-sonnet-4-6",
  max_tokens: 1024,
  messages: [{ role: "user", content: "你好" }],
})
console.log(msg.content[0].text)</code></pre>

<h2>cURL · 通用 HTTP</h2>
<pre><code>curl https://api.your-domain.com/v1/chat/completions \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-5",
    "messages": [{"role":"user","content":"hi"}]
  }'</code></pre>

<p class="docs-tip"><strong>协议兼容性</strong>:网关同时支持 OpenAI / Anthropic / Gemini 三套协议路径,一份 Key 通吃。</p>`,
      },
    ],
  },
  {
    id: 'tools',
    title: "工具与客户端",
    description: "主流开源 AI 编程客户端 / IDE / 配置工具 · 一键接入本站",
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

<p style="text-align:center;margin:1rem 0;"><img src="/docs/agents/openclaw-logo.svg" alt="OpenClaw" style="max-width:240px;height:auto"/></p>

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
  --custom-model-id "claude-opus-4-7"</code></pre>

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

<p style="text-align:center;margin:1rem 0;"><img src="/docs/agents/hermes-banner.png" alt="Hermes Agent" style="max-width:100%;height:auto;border-radius:8px"/></p>

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
  default:  "claude-opus-4-7"
  provider: "custom"
  base_url: "https://api.your-domain.com/v1"
  api_key:  "sk-你的密钥"</code></pre>
<p class="docs-tip">不要用 <code>~/.hermes/.env</code> 设 <code>OPENAI_BASE_URL</code>,那会泄漏到子进程(Hermes 官方 issue #1002 详细说明)。直接写 config.yaml 最干净。</p>

<h2>第 3 步 · 试用</h2>
<pre><code>hermes "用一句话介绍你自己"     # 一句话模式
hermes                          # 进交互式 TUI(/model 切模型 · /new 新会话)</code></pre>

<h2>第 4 步 · 副模型 fallback(可选,推荐)</h2>
<p>Hermes 原生支持主模型挂了自动切副模型:</p>
<pre><code>hermes fallback add claude-sonnet-4-6
# 之后主模型(opus)任何报错自动切到 sonnet</code></pre>

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
<figure class="docs-figure">
  <img src="/docs/agents/cherry-provider-settings-1.jpg" alt="Cherry Studio 模型服务设置" style="max-width:100%;height:auto;border-radius:8px;border:1px solid var(--color-border)"/>
  <figcaption>设置 → 模型服务 → 添加</figcaption>
</figure>

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
<figure class="docs-figure">
  <img src="/docs/agents/cherry-openai-provider.png" alt="Cherry Studio 填写 API Key" style="max-width:100%;height:auto;border-radius:8px;border:1px solid var(--color-border)"/>
  <figcaption>填好 API 地址和密钥</figcaption>
</figure>

<p class="docs-tip"><strong>API 地址重要细节</strong>:Cherry Studio 会自动加 <code>/v1/chat/completions</code>。如果想用<strong>完整自定义路径</strong>,地址末尾加 <code>#</code> 就不再拼接(高级用法)。</p>

<h2>第 4 步 · 添加模型</h2>
<p>填好 API 信息后,点配置页底部 <strong>「管理」</strong> 按钮,Cherry Studio 会自动拉取本站支持的模型列表。点列表里模型右侧的 <strong>「+」</strong> 把它加入你的可用列表。</p>
<figure class="docs-figure">
  <img src="/docs/agents/cherry-provider-settings-2.jpg" alt="管理模型列表" style="max-width:100%;height:auto;border-radius:8px;border:1px solid var(--color-border)"/>
  <figcaption>点「管理」自动拉取模型列表,加号添加</figcaption>
</figure>

<p>常用模型:</p>
<ul>
  <li><code>claude-opus-4-7</code> — 旗舰款,复杂编程 / 长文创作</li>
  <li><code>claude-sonnet-4-6</code> — 平衡型,日常对话首选</li>
  <li><code>claude-haiku-4-5</code> — 极速款,简单任务</li>
  <li><code>gpt-5.6-sol</code> / <code>gpt-5.6-terra</code> / <code>gpt-5.6-luna</code> — GPT-5.6 三档（2026 最新）</li>
  <li><code>gpt-5.5</code> / <code>gpt-5-codex</code> — Codex 系主力</li>
  <li><code>gpt-5</code> — 经典旗舰</li>
  <li><code>gemini-2.5-pro</code> — Google 系</li>
</ul>

<h2>第 5 步 · 启用 + 试用</h2>
<p><strong>关键</strong>:别忘了点配置页右上角的「开关」把这个供应商<strong>启用</strong>,否则模型列表里看不到。</p>
<figure class="docs-figure">
  <img src="/docs/agents/cherry-provider-settings-3.jpg" alt="启用供应商开关" style="max-width:100%;height:auto;border-radius:8px;border:1px solid var(--color-border)"/>
  <figcaption>右上角开关 → 启用</figcaption>
</figure>

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
        "claude-sonnet-4-6": { "name": "Claude Sonnet 4.6" },
        "gpt-5.6-terra": { "name": "GPT-5.6 Terra" },
        "gpt-5.5": { "name": "GPT-5.5" }
      }
    }
  }
}</code></pre>

<h2>第 3 步 · 运行</h2>
<pre><code>cd your-project
opencode             # 进入交互模式
# 用 /model 命令切到 gateway/claude-sonnet-4-6</code></pre>

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
  <li>Model ID: <code>claude-sonnet-4-6</code></li>
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
      "model": "claude-sonnet-4-6",
      "apiKey": "sk-xxxxxxxxxxxxxxx",
      "apiBase": "https://api.your-domain.com/v1"
    },
    {
      "title": "本站 · GPT-5.6 Terra",
      "provider": "openai",
      "model": "gpt-5.6-terra",
      "apiKey": "sk-xxxxxxxxxxxxxxx",
      "apiBase": "https://api.your-domain.com/v1"
    },
    {
      "title": "本站 · GPT-5.5",
      "provider": "openai",
      "model": "gpt-5.5",
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
aider --model openai/claude-sonnet-4-6 file1.py file2.py</code></pre>
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
        html: `<p class="docs-lead">所有主流 AI 编程 CLI(Claude Code / Codex / Gemini CLI)都依赖 Node.js。Windows 上推荐用官方 MSI 安装包,简单稳定。</p>

<h2>第 1 步 · 下载 Node.js LTS</h2>
<p>访问 <code>https://nodejs.org/zh-cn/download</code>,下载 <strong>Windows Installer (.msi)</strong>,64-bit。任意 LTS 版本(18 / 20 / 22)都行。</p>
<figure class="docs-figure">
  <div class="docs-figure-placeholder" data-shot="nodejs-download-win">
    <span class="docs-figure-tag">截图位</span>
    <span class="docs-figure-hint">Node.js 官网下载页 · Windows 安装包</span>
  </div>
  <figcaption>下载 .msi 安装包</figcaption>
</figure>

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
<pre><code>curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash

# 安装脚本会自动追加到 ~/.bashrc 或 ~/.zshrc
source ~/.bashrc       # 或 source ~/.zshrc</code></pre>

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
        summary: "国内直连 · 不需要科学上网 · 节点稳定性",
        html: `<p class="docs-lead">本站 网关部署在境内 + 海外多节点,<strong>国内用户直接访问 API 域名即可</strong>,不需要科学上网。我们已经处理了与上游(Anthropic / OpenAI / Google)的网络通路。</p>

<h2>测试连通性</h2>
<pre><code>curl -I https://api.your-domain.com/health
# 期望 HTTP/2 200</code></pre>

<h2>如果你确实在用代理</h2>
<p>大部分情况下你不需要,但如果终端默认走了系统代理,可能 cURL/SDK 会经过代理 → 增加延迟。可以临时关代理:</p>
<pre><code># macOS / Linux
unset HTTP_PROXY HTTPS_PROXY

# Windows PowerShell
$env:HTTP_PROXY=$null
$env:HTTPS_PROXY=$null</code></pre>

<h2>企业网 / 校园网</h2>
<p>少数企业/校园出网会拦截非白名单域名。如果你 curl 失败,联系网管把 <code>*.your-domain.com</code> 加入白名单即可。</p>

<h2>查看真实延迟</h2>
<p>侧边栏 <code>服务状态</code> 实时展示我们到上游各供应商的延迟统计(中位数 / P95 / SLA)。这是真实数据、不掺水,可作为你选模型的参考。</p>`,
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

<p class="docs-tip">所有教程默认你已装好其中一家 CLI。还没装?先看 <a href="/docs?cat=deploy&amp;page=claude-code">Claude Code CLI 接入</a> 或 <a href="/docs?cat=deploy&amp;page=codex">Codex CLI 接入</a>,顺便把本站 Key 配上。</p>`,
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
      <span class="docs-launch-tag">57k stars</span>
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
      <span class="docs-launch-tag">25k stars</span>
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
      <span class="docs-launch-tag">33k stars</span>
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
      <span class="docs-launch-tag">47k stars</span>
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
    <div class="docs-step-body"><pre><code>npm install -g @openai/codex</code></pre><p>还没配 Key?看 <a href="/docs?cat=deploy&amp;page=codex">Codex CLI 接入</a>,用本站 Key 直接跑。</p></div>
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
  <a class="docs-launch-link" href="https://github.com/Lum1104/Understand-Anything" target="_blank" rel="noopener">
    <span class="docs-launch-link-icon" aria-hidden="true">↗</span>
    <span class="docs-launch-link-label">GitHub</span>
    <span class="docs-launch-link-host">Lum1104/Understand-Anything</span>
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
      <span class="docs-launch-tag">149k stars</span>
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
        summary: "225k star 的超能力包 · 头脑风暴/计划/TDD/调试全套内功",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">Superpowers</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">插件</span>
      <span class="docs-launch-tag">方法论</span>
      <span class="docs-launch-tag">225k stars</span>
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
        summary: "6 万 star 工作流的官方延续 · spec 驱动 · 一段一段把活 ship 出去",
        html: `<div class="docs-launch">
  <div class="docs-launch-body">
    <div class="docs-launch-title">GSD(Get Shit Done)</div>
    <div class="docs-launch-tags">
      <span class="docs-launch-tag">工作流</span>
      <span class="docs-launch-tag">全平台</span>
      <span class="docs-launch-tag">spec 驱动</span>
    </div>
    <div class="docs-launch-blurb">原 Get Shit Done(64k star)的官方延续。轻量 meta-prompting + 上下文工程 + spec 驱动开发:把"想清楚再写"固化成几条命令,新项目防跑偏神器。</div>
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
      <span class="docs-launch-tag">27k stars</span>
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
      <span class="docs-launch-tag">63k stars</span>
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
        pages: [...imageApiPages, ...category.pages],
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
