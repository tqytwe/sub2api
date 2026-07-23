import type { PublicDocCategoryContent } from './public-docs-data.zh'

export const PUBLIC_DOC_CONTENT_EN: PublicDocCategoryContent[] = [
  {
    id: 'tutorial',
    title: 'Quickstart',
    description: 'OpenAI SDK setup, API keys, and first request',
    pages: [
      {
        id: 'quick-start',
        title: 'Use Jisudeng with your existing OpenAI SDK',
        summary: 'Change only baseURL and API key to call frontier AI models through one gateway.',
        html: `<p class="docs-lead">Jisudeng is an OpenAI-compatible API gateway. You can keep your existing OpenAI SDK and change only the <code>baseURL</code> and API key.</p>
<h2>Base URL</h2>
<pre><code>https://api.jisudeng.com/v1</code></pre>
<h2>Node.js</h2>
<pre><code>import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.JISUDENG_API_KEY,
  baseURL: "https://api.jisudeng.com/v1",
});

const response = await client.chat.completions.create({
  model: "deepseek-v4-pro",
  messages: [{ role: "user", content: "Hello from Jisudeng" }],
});

console.log(response.choices[0]?.message?.content);</code></pre>
<h2>Python</h2>
<pre><code>from openai import OpenAI

client = OpenAI(
    api_key="JISUDENG_API_KEY",
    base_url="https://api.jisudeng.com/v1",
)

response = client.chat.completions.create(
    model="deepseek-v4-pro",
    messages=[{"role": "user", "content": "Hello from Jisudeng"}],
)

print(response.choices[0].message.content)</code></pre>
<h2>curl</h2>
<pre><code>curl https://api.jisudeng.com/v1/chat/completions \\
  -H "Authorization: Bearer $JISUDENG_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "deepseek-v4-pro",
    "messages": [{"role": "user", "content": "Hello from Jisudeng"}]
  }'</code></pre>
<p class="docs-tip">Use the same base URL and API key. Switch models by changing only the <code>model</code> name.</p>`,
      },
      {
        id: 'model-switching',
        title: 'Switch models with one API key',
        summary: 'Call DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more from the same integration.',
        html: `<p class="docs-lead">Jisudeng lets one application call multiple model families through one OpenAI-compatible API surface.</p>
<h2>Model names</h2>
<p>Open the <a href="/en/models">Models &amp; Pricing</a> page to see the current public model catalog and usage-based rates.</p>
<pre><code>const models = [
  "deepseek-v4-pro",
  "kimi-k2.6",
  "glm-5",
  "claude-sonnet-5",
  "gemini-3-flash"
];</code></pre>
<h2>Runtime switching</h2>
<pre><code>async function ask(model, content) {
  return client.chat.completions.create({
    model,
    messages: [{ role: "user", content }],
  });
}</code></pre>
<p class="docs-tip">Model availability depends on the API key group and current channel health. Signed-in users can see effective group pricing on the models page.</p>`,
      },
    ],
  },
  {
    id: 'deploy',
    title: 'API Guides',
    description: 'Chat completions, image generation, and production usage notes',
    pages: [
      {
        id: 'text-to-image-api',
        title: 'Image generation API',
        summary: 'Use the OpenAI-compatible Images API for generation and edits.',
        html: `<p class="docs-lead">Jisudeng exposes image generation through OpenAI-compatible image endpoints. Use the model list available to your API key group.</p>
<pre><code>POST https://api.jisudeng.com/v1/images/generations
POST https://api.jisudeng.com/v1/images/edits
POST https://api.jisudeng.com/v1/images/generations/async
GET  https://api.jisudeng.com/v1/images/tasks/{task_id}</code></pre>
<h2>Text to image</h2>
<pre><code>curl https://api.jisudeng.com/v1/images/generations \\
  -H "Authorization: Bearer $JISUDENG_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
    "prompt": "A clean product photo of matte black wireless earbuds",
    "size": "1024x1024",
    "n": 1,
    "response_format": "b64_json"
  }'</code></pre>
<h2>Async generation</h2>
<p>Use async endpoints for long-running image jobs or when client connections may time out. Submit the job, receive a <code>task_id</code>, then poll the task endpoint.</p>
<p class="docs-tip">The exact image models available to you are shown by your API key group and the live model catalog.</p>`,
      },
    ],
  },
  {
    id: 'about',
    title: 'Billing and usage',
    description: 'API keys, usage logs, model pricing, and support',
    pages: [
      {
        id: 'pricing-and-usage',
        title: 'Usage-based billing',
        summary: 'Track model usage and pricing from one dashboard.',
        html: `<p class="docs-lead">Jisudeng uses usage-based billing. Public base rates are visible on the models page, and signed-in users see their group effective rates.</p>
<h2>What you can track</h2>
<ul>
  <li>API key usage by model</li>
  <li>Input and output token counts</li>
  <li>Site base price and group effective price</li>
  <li>Balance and recharge history</li>
</ul>
<h2>Useful links</h2>
<ul>
  <li><a href="/en/models">Models &amp; Pricing</a></li>
  <li><a href="/keys">Create API key</a></li>
  <li><a href="/usage">Usage dashboard</a></li>
</ul>`,
      },
    ],
  },
]

export function findDocContentEn(catId: string, pageId: string) {
  const cat = PUBLIC_DOC_CONTENT_EN.find((c) => c.id === catId)
  return cat?.pages.find((p) => p.id === pageId)
}

export function defaultDocPageIdEn(catId: string) {
  return PUBLIC_DOC_CONTENT_EN.find((c) => c.id === catId)?.pages[0]?.id
}
