import { PUBLIC_DOC_TREE } from './public-docs-tree'
import { getToolGuide } from './public-docs-tool-guides'
import { ENGLISH_ARTICLES } from './public-docs-articles.en'
import type { PublicDocCategoryContent, PublicDocPageContent } from './public-docs-data.zh'

const CATEGORY_META: Record<string, Pick<PublicDocCategoryContent, 'title' | 'description'>> = {
  tutorial: {
    title: 'Quickstart',
    description: 'API keys, first calls, rate limits, and practical setup notes',
  },
  'recharge-vip': {
    title: 'Billing and Perks',
    description: 'Recharge, VIP levels, check-in rewards, and usage programs',
  },
  'about-us': {
    title: 'About Jisudeng',
    description: 'Platform principles, privacy posture, and service boundaries',
  },
  'model-learning': {
    title: 'Models and Pricing',
    description: 'Model catalog, model choice, and request best practices',
  },
  deploy: {
    title: 'API setup',
    description: 'Image APIs, SDKs, and request protocol references',
  },
  tools: {
    title: 'Tool integrations',
    description: 'Terminal, editor, desktop, and agent setup guides',
  },
  environment: {
    title: 'Local Environment',
    description: 'Node.js, package managers, and network troubleshooting',
  },
  'vibe-coding': {
    title: 'Vibe Coding Guide',
    description: 'AI-assisted development workflow, context, MCP, and task systems',
  },
}

const PAGE_TITLES: Record<string, string> = {
  'quick-start': 'Quick start',
  'api-key': 'API key setup',
  concurrency: 'Concurrency and rate limits',
  'text-to-image-api': 'Text to image API',
  'batch-image-api': 'Batch image API',
  'async-image-tasks': 'Async image tasks',
  'vip-levels': 'VIP levels',
  'how-to-recharge': 'How to recharge',
  'check-in': 'Daily check-in',
  'discount-examples': 'Discount examples',
  faq: 'FAQ',
  'blindbox-rewards': 'Blind box rewards',
  'image-studio': 'AI Creation Space',
  'token-farm': 'Token farm',
  'about-us-overview': 'Platform overview',
  'about-us-privacy': 'Privacy and request data',
  'model-list': 'Model list',
  'choose-model': 'Choose a model',
  'best-practices': 'Model calling best practices',
  'claude-code': 'Claude Code setup',
  codex: 'Codex CLI setup',
  'gemini-cli': 'Gemini CLI setup',
  'sdk-quick': 'SDK quickstart',
  'cc-switch': 'cc-switch setup',
  openclaw: 'OpenClaw setup',
  hermes: 'Hermes setup',
  'cherry-studio': 'Cherry Studio setup',
  opencode: 'opencode setup',
  cline: 'Cline setup',
  continue: 'Continue setup',
  aider: 'Aider setup',
  'nodejs-windows': 'Node.js on Windows',
  'nodejs-macos': 'Node.js on macOS',
  'nodejs-linux': 'Node.js on Linux',
  network: 'Network troubleshooting',
  'start-here': 'Start here',
  'learn-ai': 'Learn AI workflows',
  'instruct-ai': 'Instruct AI clearly',
  'habits-ai': 'AI coding habits',
  'constrain-ai': 'Constrain AI output',
  context7: 'Context7',
  serena: 'Serena',
  'playwright-mcp': 'Playwright MCP',
  exa: 'Exa search',
  deepwiki: 'DeepWiki',
  codegraph: 'Code graph',
  'codex-duo': 'Codex duo workflow',
  'official-plugins': 'Official plugins',
  'understand-anything': 'Understand anything',
  'skills-intro': 'Skills introduction',
  superpowers: 'Superpowers',
  trellis: 'Trellis',
  gsd: 'GSD workflow',
  'task-master': 'Task Master',
  'open-design': 'Open design',
}

const MANUAL_PAGES: Record<string, Omit<PublicDocPageContent, 'id'>> = {
  'tutorial:quick-start': {
    title: 'Quick start',
    summary: 'Create an API key, make a first request, and verify usage.',
    html: `<h2>Get an API key and choose a model</h2>
<ol><li><a href="/login">Sign in</a> or <a href="/register">create an account</a>. If your group charges against your balance, check your balance before making a request.</li><li>Open <a href="/keys">API Keys</a>, choose <strong>Create API Key</strong>, enter a name, and select a group. Save and copy the key from its row.</li><li>In <a href="/en/catalog">Models &amp; Pricing</a>, find the exact model ID enabled for that group and confirm it supports Chat Completions. Replace <code>YOUR_MODEL_ID</code> in the examples.</li></ol>
<h2>Base URL</h2>
<pre><code>https://api.jisudeng.com/v1</code></pre>
<p>Set the environment variable <code>JISUDENG_API_KEY</code> in the shell where you run the example.</p>
<h2>Node.js</h2>
<pre><code>import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.JISUDENG_API_KEY,
  baseURL: "https://api.jisudeng.com/v1",
});

const response = await client.chat.completions.create({
  model: "YOUR_MODEL_ID",
  messages: [{ role: "user", content: "Hello from Jisudeng" }],
});

console.log(response.choices[0]?.message?.content);</code></pre>
<h2>Python</h2>
<pre><code>import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["JISUDENG_API_KEY"],
    base_url="https://api.jisudeng.com/v1",
)

response = client.chat.completions.create(
    model="YOUR_MODEL_ID",
    messages=[{"role": "user", "content": "Hello from Jisudeng"}],
)

print(response.choices[0].message.content)</code></pre>
<h2>curl</h2>
<pre><code>curl https://api.jisudeng.com/v1/chat/completions \\
  -H "Authorization: Bearer $JISUDENG_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "YOUR_MODEL_ID",
    "messages": [{"role": "user", "content": "Hello from Jisudeng"}]
  }'</code></pre>
<h2>Verify your first request</h2><p>Check the model and charge in <a href="/usage">Usage</a>. If the request failed, check the key status, exact model ID, group and endpoint before retrying.</p>`,
  },
  'deploy:text-to-image-api': {
    title: 'Image generation API',
    summary: 'Use OpenAI-compatible Images endpoints for generation, edits, async tasks, and result retrieval.',
    html: `<p class="docs-lead">Jisudeng exposes image generation through OpenAI-compatible image endpoints. Use the model list available to your API key group.</p>
<pre><code>POST https://api.jisudeng.com/v1/images/generations
POST https://api.jisudeng.com/v1/images/edits
POST https://api.jisudeng.com/v1/images/generations/async
GET  https://api.jisudeng.com/v1/images/tasks/{task_id}</code></pre>
<h2>Text to image</h2><p>Replace <code>YOUR_IMAGE_MODEL_ID</code> with the exact ID of an Images API model available to your group in <a href="/en/catalog">Models &amp; Pricing</a>.</p>
<pre><code>curl https://api.jisudeng.com/v1/images/generations \\
  -H "Authorization: Bearer $JISUDENG_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "YOUR_IMAGE_MODEL_ID",
    "prompt": "A clean product photo of matte black wireless earbuds",
    "size": "1024x1024",
    "n": 1,
    "response_format": "b64_json"
  }'</code></pre>
<h2>Async generation</h2>
<p>Use async endpoints for long-running image jobs or when client connections may time out. Submit the job, receive a <code>task_id</code>, then poll the task endpoint.</p>
<p class="docs-tip">The exact image models available to you are shown by your API key group and the live model catalog.</p>`,
  },
  'deploy:batch-image-api': {
    title: 'Batch image API',
    summary: 'Submit multiple image requests as one durable batch job.',
    html: `<p class="docs-lead">Batch image jobs combine independent image prompts in one durable job.</p>
<h2>Check eligibility</h2><p>Call <code>GET /v1/images/batches/models</code> with the same API key you will submit with. Choose a returned model ID and assign a unique <code>custom_id</code> to each item.</p>
<h2>Submit and monitor</h2>
<pre><code>POST https://api.jisudeng.com/v1/images/batches
GET  https://api.jisudeng.com/v1/images/batches/{batch_id}
GET  https://api.jisudeng.com/v1/images/batches/{batch_id}/items
GET  https://api.jisudeng.com/v1/images/batches/{batch_id}/download
POST https://api.jisudeng.com/v1/images/batches/{batch_id}/cancel
DELETE https://api.jisudeng.com/v1/images/batches/{batch_id}</code></pre>
<p>Send the API key in the Authorization header. Use the same <code>Idempotency-Key</code> when retrying the same submission. Poll the batch status and download completed outputs before expiry. Use the cancel route to request cancellation. The DELETE route removes a batch record.</p>`,
  },
  'deploy:async-image-tasks': {
    title: 'Async image tasks',
    summary: 'Queue a long-running image request and poll its status.',
    html: `<p class="docs-lead">Async generation returns <code>202 Accepted</code> and a task ID.</p><h2>Submit and poll</h2>
<pre><code>POST https://api.jisudeng.com/v1/images/generations/async
POST https://api.jisudeng.com/v1/images/edits/async
GET  https://api.jisudeng.com/v1/images/tasks/{task_id}</code></pre>
<p>Reuse the same <code>Idempotency-Key</code> when retrying a submission. Queue and storage outages fail closed with <code>IMAGE_ASYNC_NOT_READY</code>. Result URLs are short-lived and the internal storage endpoint is never exposed.</p>`,
  },
  'recharge-vip:vip-levels': {
    title: 'VIP levels',
    summary: 'Current VIP thresholds, benefits, and qualification sources.',
    html: `<p class="docs-lead">VIP is calculated from verified online payment credit and audited offline recharge net amounts.</p>
<h2>Check your level and benefits</h2><p>Use the tier table above and the live quote on <a href="/purchase">Recharge</a> for current thresholds and benefits.</p>
<p>Gifts, check-in, quiz, blind-box, redeem codes, promotions, and compensation do not qualify. Refunds and reversals reduce the net amount. A tier upgrade affects the next recharge order.</p>`,
  },
  'recharge-vip:check-in': {
    title: 'Daily check-in',
    summary: 'Check-in participation, reward pools, and streak milestones.',
    html: `<h2>Check in and confirm today's status</h2>
<p>Sign in and open <a href="/check-in">Daily Check-in</a>, or use the check-in card in <a href="/play">Play</a>. A check-in may record participation or grant energy. If eligible for a redeemable reward, read the result for the type and amount. Check <a href="/wallet">Wallet</a> for credited balance or coupons, and <a href="/redeem">Redeem</a> for awarded codes. Check-in rewards never increase VIP qualification.</p>
<h2>Review your streak and makeup eligibility</h2>
<p>The check-in page shows your current streak and any available milestone. Follow the makeup prompt if it is available for your account, then confirm the result on the same page.</p>`,
  },
  'recharge-vip:image-studio': {
    title: 'AI Creation Space',
    summary: 'Create and manage image jobs in the authenticated workspace.',
    html: `<h2>Open the canvas</h2><p><a href="/ai-creation-space">AI Creation Space</a> takes you to the separate canvas app. Follow the sign-in prompt if it appears.</p>
<h2>Create and review an image</h2><ol><li>Open the image workbench in the canvas sidebar. If no models are available, check the configured model channel and refresh its model list.</li><li>Enter a prompt and select a model and aspect ratio. Attach a reference image if supported.</li><li>Generate a single image first to verify your setup, then preview or download the result.</li></ol>
<h2>When using the Jisudeng channel</h2><p>Check your model, group, and interface in <a href="/catalog">Models &amp; Pricing</a>. Check <a href="/usage">Usage</a> after the request. For direct integration, see the <a href="/docs?cat=deploy&amp;page=text-to-image-api">Image API</a> and <a href="/docs?cat=deploy&amp;page=batch-image-api">Batch Image API</a>.</p>`,
  },
}

export const PUBLIC_DOC_CONTENT_EN: PublicDocCategoryContent[] = PUBLIC_DOC_TREE.map((category) => {
  const meta = CATEGORY_META[category.id] ?? {
    title: titleFromId(category.id),
    description: `Documentation articles for ${titleFromId(category.id)}.`,
  }

  return {
    id: category.id,
    title: meta.title,
    description: meta.description,
    pages: category.pages.map((page) => buildEnglishPage(category.id, page.id)),
  }
})

export function findDocContentEn(catId: string, pageId: string) {
  const cat = PUBLIC_DOC_CONTENT_EN.find((c) => c.id === catId)
  return cat?.pages.find((p) => p.id === pageId)
}

export function defaultDocPageIdEn(catId: string) {
  return PUBLIC_DOC_CONTENT_EN.find((c) => c.id === catId)?.pages[0]?.id
}

function buildEnglishPage(catId: string, pageId: string): PublicDocPageContent {
  if (catId === 'deploy' || catId === 'tools') {
    const tool = getToolGuide('en', pageId)
    if (tool) return tool
  }
  const manual = MANUAL_PAGES[`${catId}:${pageId}`]
  if (manual) {
    return { id: pageId, ...manual }
  }

  const article = ENGLISH_ARTICLES[`${catId}:${pageId}`]
  if (!article) throw new Error(`Missing English documentation for ${catId}/${pageId}`)
  return { id: pageId, title: PAGE_TITLES[pageId] ?? titleFromId(pageId), ...article }
}

function titleFromId(id: string): string {
  return id
    .split('-')
    .filter(Boolean)
    .map((part) => ACRONYMS[part] ?? capitalize(part))
    .join(' ')
}

function capitalize(value: string): string {
  return value ? value[0].toUpperCase() + value.slice(1) : value
}

const ACRONYMS: Record<string, string> = {
  ai: 'AI',
  api: 'API',
  cli: 'CLI',
  faq: 'FAQ',
  gsd: 'GSD',
  mcp: 'MCP',
  qq: 'QQ',
  sdk: 'SDK',
  vip: 'VIP',
}
