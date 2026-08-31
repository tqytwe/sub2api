import { PUBLIC_DOC_TREE } from './public-docs-tree'
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
    title: 'API and Tool Setup',
    description: 'Image APIs, Claude Code, Codex, Gemini CLI, and SDK quickstarts',
  },
  tools: {
    title: 'Coding Tools',
    description: 'Client setup for common coding agents and development tools',
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
    title: 'Use Jisudeng with your existing OpenAI SDK',
    summary: 'Change only baseURL and API key to call AI models through one gateway.',
    html: `<p class="docs-lead">Jisudeng is an OpenAI-compatible API gateway. Keep your existing OpenAI SDK and change only the <code>baseURL</code> and API key.</p>
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
  'deploy:text-to-image-api': {
    title: 'Image generation API',
    summary: 'Use OpenAI-compatible Images endpoints for generation, edits, async tasks, and result retrieval.',
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
  'deploy:batch-image-api': {
    title: 'Batch image API',
    summary: 'Submit multiple image requests as one durable batch job.',
    html: `<p class="docs-lead">Batch image jobs are useful when you have many prompts and do not need an immediate response.</p>
<pre><code>POST https://api.jisudeng.com/v1/images/batches
GET  https://api.jisudeng.com/v1/images/batches/{batch_id}
DELETE https://api.jisudeng.com/v1/images/batches/{batch_id}</code></pre>
<p>Use an API key in the Authorization header. The service validates every item before enqueueing the batch. Completed results are temporary download URLs and should be downloaded before expiry.</p>`,
  },
  'deploy:async-image-tasks': {
    title: 'Async image tasks',
    summary: 'Queue a long-running image request and poll its status.',
    html: `<p class="docs-lead">Async generation returns <code>202 Accepted</code> and a task ID. Poll the task URL until it is <code>completed</code> or <code>failed</code>.</p>
<pre><code>POST https://api.jisudeng.com/v1/images/generations/async
GET  https://api.jisudeng.com/v1/images/tasks/{task_id}</code></pre>
<p>Reuse the same <code>Idempotency-Key</code> when retrying a submission. Queue and storage outages fail closed with <code>IMAGE_ASYNC_NOT_READY</code>. Result URLs are short-lived and the internal storage endpoint is never exposed.</p>`,
  },
  'recharge-vip:vip-levels': {
    title: 'VIP levels',
    summary: 'Current VIP thresholds, benefits, and qualification sources.',
    html: `<p class="docs-lead">VIP is calculated from verified online payment credit and audited offline recharge net amounts.</p>
<p>Current production tiers are V0 (0), V1 (50), V2 (500), V3 (1000), V4 (2000), V5 (5000), and V6 (10000). Amounts are in CNY unless the account shows another approved currency.</p>
<p>Gifts, check-in, quiz, blind-box, redeem codes, promotions, and compensation do not qualify. Refunds and reversals reduce the net amount. A tier upgrade affects the next recharge order.</p>`,
  },
  'recharge-vip:check-in': {
    title: 'Daily check-in',
    summary: 'Check-in participation, reward pools, and streak milestones.',
    html: `<p class="docs-lead">Check-in participation and redeemable rewards are separate. The reward type and amount come from the published pool, not a fixed daily promise.</p>
<p>Seven-, fourteen-, and thirty-day streak milestones are separate balance rewards. They are credited even when the daily result is a coupon or redeem code. Check-in rewards never increase VIP qualification.</p>
<p>The production check-in and quiz switches may be disabled while the service runs a cohort or budget review.</p>`,
  },
  'recharge-vip:image-studio': {
    title: 'AI Creation Space',
    summary: 'Create and manage image jobs in the authenticated workspace.',
    html: `<p class="docs-lead">Open <code>/ai-creation-space</code> to create images, upload references, and review jobs. The legacy <code>/image-studio</code> route redirects to the new workspace.</p>
<p>Account access, model permissions, billing, and job status remain managed by the main platform. Stored assets are served through temporary signed URLs; internal object-storage endpoints are not public API values.</p>`,
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
  const manual = MANUAL_PAGES[`${catId}:${pageId}`]
  if (manual) {
    return { id: pageId, ...manual }
  }

  const title = PAGE_TITLES[pageId] ?? titleFromId(pageId)
  return {
    id: pageId,
    title,
    summary: `${title} guidance for Jisudeng users.`,
    html: fallbackHtml(catId, pageId, title),
  }
}

function fallbackHtml(catId: string, pageId: string, title: string): string {
  const sourceHref = `/en/docs?cat=${encodeURIComponent(catId)}&page=${encodeURIComponent(pageId)}`
  return `<p class="docs-lead"><strong>${escapeHtml(title)}</strong> for Jisudeng.</p>
<p>This page describes the supported workflow, request boundaries, and account or API requirements for this feature. Check the live catalog and your account permissions before making a production request.</p>
<p class="docs-tip"><a href="${sourceHref}">Open this article in English</a> or return to the documentation index for related API examples.</p>`
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

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
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
