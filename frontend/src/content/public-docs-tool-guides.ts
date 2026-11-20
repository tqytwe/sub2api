import type { PublicDocPageContent } from './public-docs-data.zh'
import type { PublicDocLocale } from './public-docs-tree'

// The reader resolves this host against the same public setting as the key console.
const ROOT = 'https://your-host'
const V1 = `${ROOT}/v1`
const KEY = 'REPLACE_WITH_YOUR_API_KEY'
const MODEL = 'REPLACE_WITH_MODEL_ID'
const codexToml = `model_provider = "jisudeng"
model = "${MODEL}"

[model_providers.jisudeng]
name = "Jisudeng"
base_url = "${V1}"
wire_api = "responses"
env_key = "JISUDENG_API_KEY"`

const snippets = {
  codex: {
    install: 'npm install -g @openai/codex\ncodex --version',
    mac: `export JISUDENG_API_KEY="${KEY}"\ncodex`,
    win: `$env:JISUDENG_API_KEY = "${KEY}"\ncodex`,
  },
  claude: {
    install: 'npm install -g @anthropic-ai/claude-code\nclaude --version',
    mac: `export ANTHROPIC_BASE_URL="${ROOT}"\nexport ANTHROPIC_AUTH_TOKEN="${KEY}"\nclaude`,
    win: `$env:ANTHROPIC_BASE_URL = "${ROOT}"\n$env:ANTHROPIC_AUTH_TOKEN = "${KEY}"\nclaude`,
  },
  gemini: {
    install: 'npm install -g @google/gemini-cli\ngemini --version',
    mac: `export GOOGLE_GEMINI_BASE_URL="${ROOT}"\nexport GEMINI_API_KEY="${KEY}"\ngemini`,
    win: `$env:GOOGLE_GEMINI_BASE_URL = "${ROOT}"\n$env:GEMINI_API_KEY = "${KEY}"\ngemini`,
  },
} as const

function code(value: string) {
  return `<pre><code>${value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')}</code></pre>`
}

function figure(file: string, alt: string, caption: string) {
  return `<figure class="docs-figure"><img src="/doc-assets/integrations/${file}" alt="${alt}" loading="lazy"/><figcaption>${caption}</figcaption></figure>`
}

function issueList(language: PublicDocLocale, detail: string) {
  return language === 'zh'
    ? `<h2 id="troubleshooting">常见问题</h2><ul><li><strong>401 / 403：</strong>在 <a href="/keys">API 密钥</a>核对密钥是否启用、是否过期，以及服务分组权限。</li><li><strong>模型不可用：</strong>以当前密钥分组展示的模型 ID 为准，区分大小写和后缀。</li><li><strong>404 或连接失败：</strong>${detail}</li><li><strong>429：</strong>检查额度、频率和并发限制，稍后用短提示词重试。</li></ul>`
    : `<h2 id="troubleshooting">Troubleshooting</h2><ul><li><strong>401 / 403:</strong> Check that the <a href="/keys">API key</a> is enabled, has not expired, and can access the selected group.</li><li><strong>Model unavailable:</strong> Copy the exact model ID, including case and suffix, from the key's group.</li><li><strong>404 or connection failure:</strong> ${detail}</li><li><strong>429:</strong> Check quota, rate, and concurrency limits, then retry with a short prompt.</li></ul>`
}

function shellSections(mac: string, win: string) {
  return `<h3>macOS / Linux</h3>${code(mac)}<h3>Windows PowerShell</h3>${code(win)}`
}

function docsVerify(language: PublicDocLocale) {
  return language === 'zh'
    ? `<h2 id="verify">验证接入</h2><p>新开一个会话，先发一条低成本的测试消息，例如「只回复 READY」。收到回复后，到 <a href="/usage">用量记录</a>核对请求时间、模型和密钥。能回复但查不到用量时，先确认工具没有继续使用旧服务商。</p>`
    : `<h2 id="verify">Verify the connection</h2><p>Start a new session and send a short test, such as “Reply with READY only.” After a response, check the time, model, and key in <a href="/usage">Usage</a>. A reply with no usage entry may mean the client is still using another provider.</p>`
}

const GUIDES: Record<PublicDocLocale, Record<string, Omit<PublicDocPageContent, 'id'>>> = {
  zh: {
    codex: {
      title: 'OpenAI Codex CLI 接入',
      summary: '把终端中的 Codex 接到极速蹬。可从控制台一键配置，也可手动添加 Responses 服务商。',
      html: `<p class="docs-lead"><strong>适用场景：</strong>在终端中读代码、修改文件和运行验证。CLI 在你运行它的目录里工作，首次测试建议选一个练习目录。</p>
<h2 id="prerequisites">前置要求</h2><p>在 <a href="/keys">API 密钥</a>中创建或选择可用密钥，并在 <a href="/catalog">模型与价格</a>确认该分组支持 Responses 和目标模型。按 <a href="https://developers.openai.com/codex/cli/">Codex 官方指南</a>安装，或在已安装 Node.js 的环境中运行：</p>${code(snippets.codex.install)}
<h2 id="quick-config">方法一：控制台一键配置</h2><ol><li>打开 <a href="/keys">API 密钥</a>，在目标密钥的「操作」列点击「一键配置」。只在支持的密钥行显示；没有时用下方手动方法。</li><li>选 Windows 或 macOS / Linux。需要导入当前密钥可用模型时，再勾选模型目录选项。</li><li>先备份现有配置，再下载并在自己的电脑运行脚本。Windows 下载 <code>.cmd</code> 后按弹窗说明运行；macOS / Linux 按弹窗命令运行 <code>.sh</code>。看到成功提示后，完全退出并重启 Codex。</li></ol>
${figure('codex-quick-config-zh.png', '极速蹬 Codex 一键配置：系统选择、模型目录选项和下载脚本', '选择操作系统，再下载脚本。画面使用脱敏演示数据。')}
<p class="docs-tip"><strong>安全提示：</strong>脚本包含当前 API Key，且会覆盖同名旧配置。不要分享脚本或把密钥提交到仓库。</p>
<h2 id="manual-config">方法二：手动配置 config.toml</h2><p>编辑 <code>~/.codex/config.toml</code>；原生 Windows 默认位于 <code>%USERPROFILE%\\.codex\\config.toml</code>。保留已有无关配置，不要重复定义同名 TOML 表。用当前分组的模型 ID 替换 <code>${MODEL}</code>。</p>${code(codexToml)}
<p>在启动 Codex 的同一个终端设置密钥，替换 <code>${KEY}</code>。不要把真实密钥写进共享配置或截图。</p>${shellSections(snippets.codex.mac, snippets.codex.win)}
${docsVerify('zh')}${issueList('zh', 'Codex 的 Base URL 必须带一次 <code>/v1</code>；不要把 <code>/responses</code> 填进 Base URL。')}
<h2 id="references">参考链接</h2><p><a href="https://developers.openai.com/codex/config-reference/">Codex 配置参考</a> · <a href="/docs?cat=tools&amp;page=cc-switch">CC Switch 教程</a></p>`,
    },
    'claude-code': {
      title: 'Claude Code CLI 接入',
      summary: '配置 Anthropic Messages 兼容入口，在终端中用 Claude Code 阅读、修改和验证代码。',
      html: `<p class="docs-lead"><strong>适用场景：</strong>在项目目录里问代码问题、查看修改建议或执行开发任务。先在练习项目验证接入。</p>
<h2 id="prerequisites">前置要求</h2><p>准备包含目标模型且支持 Messages 协议的 <a href="/keys">API 密钥</a>。从 <a href="https://code.claude.com/docs/en/setup">Claude Code 官方安装页</a>选择系统；如果使用 npm，先安装 Node.js，再执行：</p>${code(snippets.claude.install)}
<h2 id="configure">第 1 步：配置环境变量</h2><p>将下面的占位密钥换成你的极速蹬密钥；<code>ANTHROPIC_BASE_URL</code> 填根地址，不带 <code>/v1</code>。在同一个终端运行 <code>claude</code>。</p>${shellSections(snippets.claude.mac, snippets.claude.win)}
<h2 id="console">第 2 步：按分组选择模型</h2><p>在 <a href="/keys">API 密钥</a>行点击「使用密钥」，可以查看当前分组提供的客户端配置。需要固定模型时，在 Claude Code 中选择分组支持的实际模型 ID，不照抄其他分组的示例。</p>
<p class="docs-tip">经常切换服务商，可以用 <a href="/docs?cat=tools&amp;page=cc-switch">CC Switch</a> 的桌面界面管理配置。避免多个工具同时覆盖同一份配置。</p>
${docsVerify('zh')}${issueList('zh', '如果请求出现 <code>/v1/v1/messages</code>，删掉 Base URL 尾部的 <code>/v1</code>。')}`,
    },
    'gemini-cli': {
      title: 'Gemini CLI 接入',
      summary: '用 Gemini 原生协议和极速蹬密钥接入 Google 的终端工具。',
      html: `<p class="docs-lead">适合在终端中使用 Gemini 处理代码或文件。这个教程使用 Gemini API Key 认证，不使用 Google 账号登录或 Vertex AI。</p>
<h2 id="prerequisites">前置要求</h2><p>选择支持 Gemini 原生协议和目标模型的 <a href="/keys">API 密钥</a>。在安装 Node.js 后运行以下命令；如遇系统或版本问题，参考 <a href="https://geminicli.com/docs/get-started/installation/">官方安装说明</a>。</p>${code(snippets.gemini.install)}
<h2 id="configure">第 1 步：配置地址和密钥</h2><p>以下是标准 Gemini 分组的配置。若你的分组在「使用密钥 → Gemini」给出专用地址，以该值为准。标准地址不要手动加 <code>/v1</code> 或 <code>/v1beta</code>。</p>${shellSections(snippets.gemini.mac, snippets.gemini.win)}
<h2 id="authentication">第 2 步：选择认证和模型</h2><p>如客户端询问认证方式，选择 Gemini API Key；若仍使用旧认证方式，先在客户端的认证设置切换。按 <a href="/catalog">模型与价格</a>中当前分组可用的模型 ID 测试。</p>
${docsVerify('zh')}${issueList('zh', '检查是否误把 OpenAI 兼容地址用于 Gemini 原生客户端。')}`,
    },
    cline: {
      title: 'Cline：VS Code 扩展接入',
      summary: '在 VS Code 中配置 OpenAI Compatible 提供方，验证聊天后再授权项目操作。',
      html: `<p class="docs-lead">Cline 是编辑器内的编程助手，可读取文件、提出修改和调用终端命令。初次接入先测试文本回复，再逐项批准所需操作。</p>
<h2 id="prerequisites">前置要求</h2><p>准备 VS Code 和一个支持目标模型的 <a href="/keys">API 密钥</a>。从扩展面板搜索并安装 <a href="https://marketplace.visualstudio.com/items?itemName=saoudrizwan.claude-dev">Cline</a>。</p>
<h2 id="configure">第 1 步：填写提供方配置</h2><ol><li>打开 Cline 侧边栏，进入设置中的 API 提供方配置。</li><li>选择 <strong>OpenAI Compatible</strong>，不要选要求官方 OpenAI 地址的提供方。</li><li><strong>Base URL</strong> 填 <code>${V1}</code>；<strong>API Key</strong> 粘贴极速蹬密钥；<strong>Model ID</strong> 填 <a href="/catalog">模型与价格</a>中属于该分组的精确 ID。</li><li>保存。如果 Plan 与 Act 分别配置，核对当前模式使用的是刚保存的提供方。</li></ol>
${docsVerify('zh')}${issueList('zh', '核对提供方类型及 <code>/v1</code>，不要输入完整的 <code>/chat/completions</code> 路径。')}`,
    },
  },
  en: {
    codex: {
      title: 'OpenAI Codex CLI setup',
      summary: 'Connect Codex in your terminal with Quick Configure or a custom Responses provider.',
      html: `<p class="docs-lead"><strong>When to use it:</strong> Work with code, files, and verification commands from a terminal. Use a practice directory for the first test.</p>
<h2 id="prerequisites">Prerequisites</h2><p>Create or select an <a href="/keys">API key</a>, then check that its group exposes Responses and your desired model in <a href="/en/catalog">Models and pricing</a>. Use the <a href="https://developers.openai.com/codex/cli/">official Codex installer</a>, or run the following if Node.js is already installed:</p>${code(snippets.codex.install)}
<h2 id="quick-config">Option 1: Quick Configure</h2><ol><li>Open <a href="/keys">API keys</a> and click <strong>Quick Configure</strong> in the key's Actions column. If the button is not available for that key, use the manual method below.</li><li>Select Windows or macOS / Linux. Turn on model catalog import only if you want a local catalog of models available to this key.</li><li>Back up your existing Codex settings. Download the script, run it locally using the dialog instructions, and fully quit and restart Codex after it reports success.</li></ol>
${figure('codex-quick-config-en.png', 'Jisudeng Codex Quick Configure dialog with OS selection and download button', 'Select your operating system before downloading. Shown with redacted demo data.')}
<p class="docs-tip"><strong>Keep the script private:</strong> It contains your API key and replaces existing configuration files of the same name. Never share it or commit it to source control.</p>
<h2 id="manual-config">Option 2: Edit config.toml</h2><p>Edit <code>~/.codex/config.toml</code> (native Windows default: <code>%USERPROFILE%\\.codex\\config.toml</code>). Merge with your current settings without defining a TOML table twice, and replace <code>${MODEL}</code> with the model ID for your group.</p>${code(codexToml)}
<p>Replace <code>${KEY}</code> and set the environment variable in the same terminal where you start Codex. Keep real secrets out of shared files and screenshots.</p>${shellSections(snippets.codex.mac, snippets.codex.win)}
${docsVerify('en')}${issueList('en', 'The Codex base URL needs <code>/v1</code> once. Do not append <code>/responses</code>.')}
<h2 id="references">References</h2><p><a href="https://developers.openai.com/codex/config-reference/">Codex configuration reference</a> · <a href="/en/docs?cat=tools&amp;page=cc-switch">CC Switch guide</a></p>`,
    },
    'claude-code': {
      title: 'Claude Code CLI setup',
      summary: 'Connect Claude Code using the Anthropic Messages-compatible gateway.',
      html: `<p class="docs-lead">Use Claude Code from a project directory to ask about code, review changes, and run development tasks. Start in a practice project.</p>
<h2 id="prerequisites">Prerequisites</h2><p>Prepare an <a href="/keys">API key</a> for a group that supports Messages and the required model. Choose your platform in the <a href="https://code.claude.com/docs/en/setup">official setup guide</a>, or install from npm if Node.js is already installed:</p>${code(snippets.claude.install)}
<h2 id="configure">Step 1: Set connection variables</h2><p>Replace the placeholder with your Jisudeng key. Use the host root without <code>/v1</code> for <code>ANTHROPIC_BASE_URL</code> and start the CLI in the same terminal.</p>${shellSections(snippets.claude.mac, snippets.claude.win)}
<h2 id="console">Step 2: Select an available model</h2><p>In <a href="/keys">API keys</a>, choose <strong>Use Key</strong> to check the client settings for the selected group. If you pin a model, use its exact ID in <a href="/en/catalog">Models and pricing</a>. Use <a href="/en/docs?cat=tools&amp;page=cc-switch">CC Switch</a> if you regularly change providers.</p>
${docsVerify('en')}${issueList('en', 'If you see <code>/v1/v1/messages</code>, remove <code>/v1</code> from the configured base URL.')}`,
    },
    'gemini-cli': {
      title: 'Gemini CLI setup',
      summary: 'Connect with native Gemini protocol and a Jisudeng API key.',
      html: `<p class="docs-lead">Use this setup for Gemini API key authentication, not Google account login or Vertex AI.</p>
<h2 id="prerequisites">Prerequisites</h2><p>Choose an <a href="/keys">API key</a> for a group that supports the native Gemini protocol. With Node.js installed, use the following or follow the <a href="https://geminicli.com/docs/get-started/installation/">official installation guide</a>.</p>${code(snippets.gemini.install)}
<h2 id="configure">Step 1: Configure the URL and key</h2><p>This is the standard Gemini base URL. If <strong>Use Key → Gemini</strong> shows a group-specific endpoint, use that endpoint. Do not add <code>/v1</code> or <code>/v1beta</code> to the standard host.</p>${shellSections(snippets.gemini.mac, snippets.gemini.win)}
<h2 id="authentication">Step 2: Choose authentication and model</h2><p>Choose Gemini API Key if prompted. If another authentication method is active, switch it in the client first. Test with an available model from your group in <a href="/en/catalog">Models and pricing</a>.</p>
${docsVerify('en')}${issueList('en', 'Use the native Gemini endpoint rather than the OpenAI-compatible base URL.')}`,
    },
    cline: {
      title: 'Cline for VS Code',
      summary: 'Set up an OpenAI Compatible provider and verify chat before approving project actions.',
      html: `<p class="docs-lead">Cline reads files, proposes edits, and can run terminal commands from your editor. Check a plain response before approving project actions.</p>
<h2 id="prerequisites">Prerequisites</h2><p>Install <a href="https://marketplace.visualstudio.com/items?itemName=saoudrizwan.claude-dev">Cline</a> in VS Code and prepare an <a href="/keys">API key</a> for your model group.</p>
<h2 id="configure">Step 1: Configure the provider</h2><ol><li>Open Cline's sidebar and locate API provider settings.</li><li>Select <strong>OpenAI Compatible</strong>.</li><li>Enter <code>${V1}</code> as <strong>Base URL</strong>, your Jisudeng <strong>API Key</strong>, and the exact <strong>Model ID</strong> from your group in <a href="/en/catalog">Models and pricing</a>.</li><li>Save. If Plan and Act have separate configurations, check the one you will use.</li></ol>
${docsVerify('en')}${issueList('en', 'Check provider type and <code>/v1</code>. Do not enter a full <code>/chat/completions</code> path.')}`,
    },
  },
}

type Bilingual = Record<PublicDocLocale, string>

const OTHER_GUIDES: Record<string, { name: string; official: string; use: Bilingual; setup: Bilingual; check: Bilingual }> = {
  'cc-switch': {
    name: 'CC Switch', official: 'https://github.com/farion1231/cc-switch/releases',
    use: { zh: '在桌面端集中管理 Claude Code、Codex 等客户端的服务商配置。', en: 'Manage provider configurations for supported clients such as Claude Code and Codex in one desktop app.' },
    setup: {
      zh: `<ol><li>从发布页下载并安装对应系统的桌面版本，先启动 CC Switch。</li><li>在极速蹬「API 密钥」找到目标密钥。如果操作列提供导入 CC Switch 的按钮，点击后选客户端，在 CC Switch 中核对并确认导入。</li><li>如果没有导入入口，在 CC Switch 选对应客户端，手动新增服务商。Claude Code 使用 <code>${ROOT}</code>，Codex 使用 <code>${V1}</code> 和 Responses。保存后启用该服务商，重启目标客户端。</li></ol>`,
      en: `<ol><li>Install and open the desktop release for your OS.</li><li>In Jisudeng API keys, use the CC Switch import action if that key offers it. Choose the target client and review the provider before confirming in CC Switch.</li><li>If no import action is available, add a provider manually in CC Switch. Use <code>${ROOT}</code> for Claude Code or <code>${V1}</code> with Responses for Codex. Activate the provider and restart the target client.</li></ol>`,
    },
    check: { zh: '若链接没唤起客户端，先确认已安装并允许系统打开 CC Switch，再试手动新增。', en: 'If importing does not open CC Switch, check the installation and OS link handler, or add the provider manually.' },
  },
  'cherry-studio': {
    name: 'Cherry Studio', official: 'https://github.com/CherryHQ/cherry-studio/releases',
    use: { zh: '用于桌面聊天和比较多个模型，不需要命令行。', en: 'Chat and compare models from a desktop interface without a terminal.' },
    setup: {
      zh: `<ol><li>安装并打开 Cherry Studio，进入「设置 → 模型服务」，添加自定义供应商。</li><li>选择 OpenAI 兼容类型，填入极速蹬密钥。常规模型服务地址填根域名 <code>${ROOT}</code>，让客户端拼接 <code>/v1</code>。</li><li>启用供应商，从「管理」拉取模型或手动添加当前分组的模型 ID。打开新对话，在模型选择处切到刚添加的模型。</li></ol>`,
      en: `<ol><li>Install Cherry Studio and open <strong>Settings → Model Services</strong>. Add a custom provider.</li><li>Choose the OpenAI-compatible type and enter your key. For the standard provider URL field, enter <code>${ROOT}</code> and let Cherry Studio append <code>/v1</code>.</li><li>Enable the provider, fetch models using Manage or add your group’s exact model ID manually, then choose it in a new chat.</li></ol>`,
    },
    check: { zh: '使用供应商的「检查」功能；如果请求路径含两个 /v1，删掉手填的那一个。', en: 'Use the provider’s connection check. If the request path contains /v1 twice, remove the manually added suffix.' },
  },
  opencode: {
    name: 'OpenCode', official: 'https://opencode.ai/docs/providers/',
    use: { zh: '在项目目录里使用终端编程助手，并按分组选择模型提供方。', en: 'Use a terminal coding assistant with the provider configuration appropriate for your API key group.' },
    setup: {
      zh: `<ol><li>安装 OpenCode，运行 <code>opencode --version</code> 确认命令可用。</li><li>在 <a href="/keys">API 密钥</a>选择「使用密钥 → OpenCode」，复制当前分组的配置。OpenAI 兼容分组一般使用 <code>${V1}</code>；原生 Gemini、Anthropic 或专用路径以页面生成值为准。</li><li>按弹窗指示合并到 <code>opencode.json</code>，保留已有配置。不要把含密钥的文件提交到项目仓库。</li><li>在项目目录启动 <code>opencode</code>，通过 <code>/models</code> 选择新增服务商下的模型。</li></ol>`,
      en: `<ol><li>Install OpenCode and check <code>opencode --version</code>.</li><li>In API keys, choose <strong>Use Key → OpenCode</strong> and copy the configuration for that group. An OpenAI-compatible group generally uses <code>${V1}</code>; use the generated value for native or group-specific providers.</li><li>Merge the generated configuration into <code>opencode.json</code>. Do not commit files containing API keys.</li><li>Launch <code>opencode</code> in the project, then use <code>/models</code> to select a model under the new provider.</li></ol>`,
    },
    check: { zh: '模型列表没有新服务商时，先查配置文件层级和 JSON 语法，再核对当前目录使用的配置。', en: 'If the provider is missing, check JSON syntax and whether OpenCode is reading that configuration file in the current directory.' },
  },
  continue: {
    name: 'Continue', official: 'https://docs.continue.dev/',
    use: { zh: '在编辑器里用自定义提供方聊天和辅助编辑。', en: 'Use a custom provider for chat and assisted editing inside your IDE.' },
    setup: {
      zh: `<p>安装 Continue 扩展，打开扩展的本地配置。使用 YAML 的版本可在 <code>models</code> 下加入下列条目；已有配置时只合并条目，不重复定义 <code>models</code>：</p>${code(`models:\n  - name: Jisudeng\n    provider: openai\n    model: ${MODEL}\n    apiBase: ${V1}\n    apiKey: ${KEY}`)}<p>用当前分组可用模型替换模型 ID，并按当前版本支持的密钥方式保存。不要把含有真实密钥的配置纳入 Git。保存后，在聊天模型选择器中选 Jisudeng。</p>`,
      en: `<p>Install the extension and open its local configuration. For versions using YAML, add a model under the existing <code>models</code> key:</p>${code(`models:\n  - name: Jisudeng\n    provider: openai\n    model: ${MODEL}\n    apiBase: ${V1}\n    apiKey: ${KEY}`)}<p>Use an exact model ID, keep API keys out of Git, and select Jisudeng in the chat model picker.</p>`,
    },
    check: { zh: '确认扩展正在使用本地配置，且 OpenAI 兼容地址以 /v1 结尾。', en: 'Check that the extension is using the local configuration and that the OpenAI-compatible endpoint ends in /v1.' },
  },
  aider: {
    name: 'Aider', official: 'https://aider.chat/docs/install.html',
    use: { zh: '在终端与 AI 协作修改 Git 仓库，适合希望随时用 Git 检查改动的用户。', en: 'Pair on code from the terminal and review changes in Git.' },
    setup: {
      zh: `<p>按官方指南安装，运行 <code>aider --version</code>。先在测试仓库里使用。分别按自己的终端设置环境变量，并把模型 ID 换成分组可用值：</p>${shellSections(`export OPENAI_API_BASE="${V1}"\nexport OPENAI_API_KEY="${KEY}"\naider --model openai/${MODEL}`, `$env:OPENAI_API_BASE = "${V1}"\n$env:OPENAI_API_KEY = "${KEY}"\naider --model openai/${MODEL}`)}`,
      en: `<p>Install Aider and run <code>aider --version</code>. In a practice repository, set the variables for your terminal and replace the model ID:</p>${shellSections(`export OPENAI_API_BASE="${V1}"\nexport OPENAI_API_KEY="${KEY}"\naider --model openai/${MODEL}`, `$env:OPENAI_API_BASE = "${V1}"\n$env:OPENAI_API_KEY = "${KEY}"\naider --model openai/${MODEL}`)}`,
    },
    check: { zh: 'OpenAI 兼容模式使用 openai/ 前缀给 Aider 识别提供方；前缀不是极速蹬模型 ID 的一部分。', en: 'The openai/ prefix selects the provider in Aider. It is not part of the Jisudeng model ID.' },
  },
  openclaw: {
    name: 'OpenClaw', official: 'https://docs.openclaw.ai/',
    use: { zh: '把极速蹬模型接入自行运行的智能体。首次只在本机测试，不对公网开放。', en: 'Connect a model to a self-managed assistant. Test locally before exposing any service.' },
    setup: {
      zh: `<ol><li>按官方指南安装，运行 <code>openclaw --version</code>，再启动该版本的初始化向导。</li><li>选择自定义模型提供方。使用 OpenAI 兼容协议时，Base URL 填 <code>${V1}</code>，API Key 填极速蹬密钥，模型填与所选接口兼容的 ID。</li><li>确认向导最终保存的默认模型是刚建立的服务商下的模型。按向导启动本地客户端，先不配置聊天渠道或额外权限。</li></ol>`,
      en: `<ol><li>Install using the official instructions, verify <code>openclaw --version</code>, and open the onboarding wizard for that release.</li><li>Select a custom provider. For an OpenAI-compatible connection, enter <code>${V1}</code>, your Jisudeng key, and a model ID that supports the selected interface.</li><li>Confirm the default model is assigned to the new provider. Start locally without configuring messaging channels or extra permissions until the test passes.</li></ol>`,
    },
    check: { zh: '若提示模型或接口不受支持，检查默认模型、提供方和协议是否来自同一组配置。', en: 'If the model or interface is unsupported, check that the default model, provider, and protocol belong to the same configuration.' },
  },
  hermes: {
    name: 'Hermes Agent', official: 'https://github.com/NousResearch/hermes-agent',
    use: { zh: '用于终端里的通用智能体。先跑通基础会话，再开放外部工具。', en: 'Run a general-purpose assistant from your terminal. Verify chat before enabling external tools.' },
    setup: {
      zh: `<ol><li>按官方项目说明安装，通过 <code>hermes --version</code> 检查。</li><li>打开客户端的模型设置，选择自定义的 OpenAI 兼容端点，填 <code>${V1}</code>、极速蹬 API 密钥和模型 ID；支持文件配置的版本可用以下字段合并到用户配置。</li></ol>${code(`model:\n  provider: custom\n  default: ${MODEL}\n  base_url: ${V1}\n  api_key: ${KEY}`)}<p>避免把带真实密钥的 YAML 同步到公开仓库。</p>`,
      en: `<ol><li>Use the official installation guide, then check <code>hermes --version</code>.</li><li>In model setup, choose a custom OpenAI-compatible endpoint and enter <code>${V1}</code>, your API key, and the exact model ID. On versions supporting YAML settings, merge the fields below into your user configuration.</li></ol>${code(`model:\n  provider: custom\n  default: ${MODEL}\n  base_url: ${V1}\n  api_key: ${KEY}`)}<p>Do not sync a configuration containing an API key to a public repository.</p>`,
    },
    check: { zh: '确认客户端启动时加载了自定义服务商和预期模型，而非旧的默认模型。', en: 'Check that the client loaded the intended custom provider and model instead of a previous default.' },
  },
}

export function getToolGuide(language: PublicDocLocale, pageId: string): PublicDocPageContent | undefined {
  if (Object.prototype.hasOwnProperty.call(GUIDES[language], pageId)) return { id: pageId, ...GUIDES[language][pageId] }
  if (!Object.prototype.hasOwnProperty.call(OTHER_GUIDES, pageId)) return undefined
  const guide = OTHER_GUIDES[pageId]
  const zh = language === 'zh'
  return {
    id: pageId,
    title: zh ? `${guide.name} 接入` : `${guide.name} setup`,
    summary: guide.use[language],
    html: `<p class="docs-lead">${guide.use[language]}</p>
<h2 id="prerequisites">${zh ? '前置要求' : 'Prerequisites'}</h2><p><a href="${guide.official}">${zh ? '官方安装与资料' : 'Official setup and resources'}</a> · <a href="/keys">${zh ? '创建 API 密钥' : 'Create an API key'}</a> · <a href="${zh ? '/catalog' : '/en/catalog'}">${zh ? '确认分组中的模型 ID' : 'Find the exact model ID for your group'}</a></p>
<h2 id="configure">${zh ? '安装与配置' : 'Install and configure'}</h2>${guide.setup[language]}
${docsVerify(language)}${issueList(language, guide.check[language])}`,
  }
}
