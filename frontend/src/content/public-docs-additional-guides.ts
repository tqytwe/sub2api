import type { PublicDocLocale } from './public-docs-tree'

const ROOT = 'https://your-host'
const V1 = `${ROOT}/v1`
const KEY = 'REPLACE_WITH_YOUR_API_KEY'
const MODEL = 'REPLACE_WITH_MODEL_ID'

type Text = readonly [zh: string, en: string]
interface Guide {
  name: string
  official: string
  summary: Text
  steps: readonly Text[]
  check: Text
  fields?: readonly (readonly [Text, string])[]
  example?: string
}

const key: Text = ['API 密钥', 'API key']
const model: Text = ['模型 ID', 'Model ID']
const common = [[['Base URL', 'Base URL'], V1], [key, KEY], [model, MODEL]] as const
const codexFields = [common[0], [['接口协议', 'Wire API'], 'responses'], common[2]] as const
const claudeFields = [[['网关地址', 'Gateway URL'], ROOT], [key, KEY], [model, MODEL]] as const
const codexGuide = '/docs?cat=tools&amp;page=codex'
const codexConfig = `model_provider = "jisudeng"
model = "${MODEL}"

[model_providers.jisudeng]
name = "Jisudeng"
base_url = "${V1}"
wire_api = "responses"
env_key = "JISUDENG_API_KEY"`

const guides: Record<string, Guide> = {
  'codex-app': {
    name: 'Codex App', official: 'https://developers.openai.com/codex/app/',
    summary: ['在 Codex 桌面应用中打开项目、发起对话并审阅修改。', 'Open a project, start a conversation, and review edits in the Codex desktop app.'],
    fields: codexFields,
    steps: [
      ['从官方页面安装桌面应用。到极速蹬「API 密钥」选择支持 Responses 的分组。', 'Install the desktop app from the official site. In Jisudeng API keys, choose a group that supports Responses.'],
      [`在应用设置中打开 Codex 配置文件。已有 <code>~/.codex/config.toml</code> 时先备份，合并下方服务商配置。也可参考<a href="${codexGuide}#quick-config">控制台一键配置</a>。`, `Open the Codex configuration file in app settings. Back up <code>~/.codex/config.toml</code> before merging the provider below. See <a href="${codexGuide}#quick-config">console quick configuration</a> for the platform shortcut.`],
      ['按 CLI 教程设置 <code>JISUDENG_API_KEY</code>。确保桌面应用进程能读取变量，完全退出后重新启动，打开测试项目。', 'Set <code>JISUDENG_API_KEY</code> as described in the CLI guide. Make it available to the desktop app process, quit the app fully, reopen it, and open a test project.'],
    ],
    example: codexConfig,
    check: ['如果终端能用但桌面应用提示缺少密钥，检查桌面进程环境和当前打开的配置文件。', 'If the CLI works but the app reports a missing key, check the app process environment and the configuration file it loaded.'],
  },
  'vscode-codex': {
    name: 'VS Code Codex', official: 'https://developers.openai.com/codex/ide/',
    summary: ['在 VS Code 侧栏使用 Codex，直接查看工作区和差异。', 'Use Codex from the VS Code sidebar and review changes in the workspace.'],
    fields: codexFields,
    steps: [
      ['打开 VS Code 的扩展视图，安装 OpenAI 发布的 Codex 扩展。先打开一个练习工作区。', 'Open Extensions in VS Code and install the Codex extension published by OpenAI. Open a practice workspace.'],
      [`打开 Codex 面板设置中的 <code>config.toml</code>，备份后合并下方配置。按 <a href="${codexGuide}">CLI 教程</a>设置变量，重启编辑器。`, `Open <code>config.toml</code> from the Codex panel settings. Back it up, merge the configuration below, set the environment variable using the <a href="${codexGuide}">CLI guide</a>, and restart the editor.`],
      ['使用 Remote SSH、WSL 或容器时，确认扩展运行位置，并在该环境中核对配置与密钥。', 'With Remote SSH, WSL, or containers, check where the extension runs and set the configuration and key in that environment.'],
    ],
    example: codexConfig,
    check: ['若扩展继续使用旧服务商，检查 <code>model_provider</code>、环境变量和当前 VS Code 实例使用的配置文件。', 'If the extension uses a previous provider, check <code>model_provider</code>, the environment variable, and the configuration file used by that VS Code instance.'],
  },
  'vscode-claude-code': {
    name: 'VS Code Claude Code', official: 'https://code.claude.com/docs/en/vs-code',
    summary: ['在 VS Code 中运行 Claude Code，使用 Anthropic Messages 协议。', 'Run Claude Code in VS Code using the Anthropic Messages protocol.'],
    fields: claudeFields,
    steps: [
      ['在扩展视图安装 Anthropic 发布的 Claude Code。创建可调用 Anthropic Messages 的极速蹬密钥。', 'Install the Claude Code extension published by Anthropic. Create a Jisudeng key for a group that supports Anthropic Messages.'],
      ['在扩展设置中搜索 <code>Claude Code: Environment Variables</code>。通过 <strong>用户设置</strong>配置下方字段。将模型改成分组对应的 ID。', 'Find <code>Claude Code: Environment Variables</code> in the extension settings. Add the fields below in <strong>User Settings</strong> and use the exact model ID for your group.'],
      ['使用第三方服务商时可开启 <code>Claude Code: Disable Login Prompt</code>（扩展键为 <code>claudeCode.disableLoginPrompt</code>）。重新加载窗口，再打开扩展面板。', 'For a third-party provider, enable <code>Claude Code: Disable Login Prompt</code> (<code>claudeCode.disableLoginPrompt</code>) if needed. Reload the window and open the extension panel.'],
    ],
    example: JSON.stringify({ 'claudeCode.environmentVariables': [{ name: 'ANTHROPIC_BASE_URL', value: ROOT }, { name: 'ANTHROPIC_AUTH_TOKEN', value: KEY }, { name: 'ANTHROPIC_MODEL', value: MODEL }], 'claudeCode.disableLoginPrompt': true }, null, 2),
    check: ['网关地址填写根地址。检查配置是否放在运行扩展的环境，并避免让带密钥的编辑器设置同步或进入仓库。', 'Use the gateway root. Check that the settings apply where the extension runs, and keep settings containing secrets out of sync and version control.'],
  },
  'factory-droid-cli': {
    name: 'Factory Droid CLI', official: 'https://docs.factory.ai/cli/configuration/byok',
    summary: ['将极速蹬的对话模型加入 Droid 的自定义模型列表。', 'Add a Jisudeng chat model to Droid’s custom model list.'],
    fields: common,
    steps: [
      ['按官方 Quickstart 安装 Droid 并按客户端提示完成初始化。打开 BYOK 文档中对应版本的用户配置。', 'Install Droid using its official quickstart and follow the prompts. Open the user settings file for your release as described in the BYOK guide.'],
      ['备份 <code>~/.factory/settings.json</code>，在现有 <code>customModels</code> 数组里加入一个模型。使用支持 Chat Completions 的分组，并在启动 Droid 的环境提供 <code>JISUDENG_API_KEY</code>。', 'Back up <code>~/.factory/settings.json</code> and append a model to <code>customModels</code>. Choose a Chat Completions group and set <code>JISUDENG_API_KEY</code> in the environment that launches Droid.'],
      ['在测试目录启动 <code>droid</code>，用 <code>/model</code> 选择添加的模型，再进行验证。', 'Launch <code>droid</code> in a practice directory, select the new model with <code>/model</code>, and verify it.'],
    ],
    example: JSON.stringify({ customModels: [{ model: MODEL, displayName: 'Jisudeng', baseUrl: V1, apiKey: '${JISUDENG_API_KEY}', provider: 'generic-chat-completion-api' }] }, null, 2),
    check: ['模型未出现时先检查 JSON 合法性和配置位置，再确认客户端支持 BYOK。', 'If the model is missing, validate JSON, confirm the configuration path, and check BYOK availability in your client.'],
  },
  cursor: {
    name: 'Cursor', official: 'https://cursor.com/docs/settings/api-keys',
    summary: ['在 Cursor 的模型设置中接入 OpenAI 兼容的自定义模型。', 'Connect an OpenAI-compatible custom model from Cursor’s model settings.'],
    fields: common,
    steps: [
      ['打开 Cursor Settings → Models，找到 OpenAI API Key 和覆盖 OpenAI Base URL 的选项。', 'Open Cursor Settings → Models and find OpenAI API Key and the OpenAI Base URL override.'],
      ['填入极速蹬密钥与上表地址，执行验证。通过 Add Model 添加分组中的精确模型 ID，并启用该模型。', 'Enter the key and URL above, then run verification. Use Add Model to add the exact model ID from your group and enable it.'],
      ['新开对话，选择该模型。先试基础对话，再验证所用模式需要的工具调用。', 'Start a new chat and select the model. Test basic chat before testing tool calls required by your mode.'],
    ],
    check: ['自定义 API Key 适用的功能由 Cursor 决定。Tab 等专用功能依赖 Cursor 自带模型。验证失败时核对客户端是否能访问网关和该模型。', 'Cursor controls which features accept custom API keys. Features such as Tab rely on Cursor models. If verification fails, check gateway access and model availability.'],
  },
  dify: {
    name: 'Dify', official: 'https://docs.dify.ai/en/use-dify/workspace/model-providers',
    summary: ['将极速蹬模型加入 Dify 工作区，用于聊天应用、工作流和知识库。', 'Add Jisudeng models to a Dify workspace for chat apps, workflows, and knowledge bases.'],
    fields: common,
    steps: [
      ['用工作区所有者或管理员账号进入 Settings → Model Providers。搜索并安装 <strong>OpenAI-API-compatible</strong> 模型供应商插件。', 'As a workspace owner or admin, open Settings → Model Providers and install the <strong>OpenAI-API-compatible</strong> provider plugin.'],
      ['在该供应商下添加模型，填写 API Key、API Endpoint URL 和模型 ID。先选择 Chat / LLM。为知识库搭建时单独添加受分组支持的 Embedding 模型。', 'Add a model under that provider. Enter the key, API endpoint URL, and model ID. Start with Chat / LLM. For a knowledge base, add a separate embedding model supported by the key group.'],
      ['打开应用或工作流，点模型选择器或 LLM 节点，选刚配置的模型。在预览中发送一条短消息。', 'Open an app or workflow and choose the model in its selector or LLM node. Send a short message in Preview.'],
    ],
    check: ['应用中找不到模型时，查看当前工作区、模型启用状态和类型。自托管时由 Dify 服务端访问网关，也要检查服务器网络。', 'If the app cannot find the model, check the workspace, enabled status, and model type. For self-hosted deployments, also check connectivity from the Dify server.'],
  },
  nextchat: {
    name: 'NextChat', official: 'https://github.com/ChatGPTNextWeb/NextChat',
    summary: ['在 NextChat 的 OpenAI 提供商里使用自己的网关和模型。', 'Use your gateway and model in the NextChat OpenAI provider.'],
    fields: [[common[0][0], ROOT], common[1], common[2]],
    steps: [
      ['打开设置中的模型服务商和访问设置，选择 OpenAI 并开启自定义接口。填写站点根地址与极速蹬 API 密钥。', 'In provider and access settings, choose OpenAI and enable a custom endpoint. Enter the gateway root and your Jisudeng key.'],
      ['如果默认列表缺少目标模型，在自定义模型名中填精确 ID。多个模型以英文逗号分隔。需要明确路由到 OpenAI 提供商的版本可按客户端语法加 <code>@OpenAI</code>。', 'If the model is missing, enter the exact ID under custom models. Separate multiple values with commas. Where needed, use the client’s <code>@OpenAI</code> provider suffix.'],
      ['返回聊天页并新建会话，从模型列表选刚添加的模型。', 'Return to chat, start a new session, and select the model you added.'],
    ],
    check: ['查看最终请求应只有一层 <code>/v1/chat/completions</code>。如自托管，可参照应用搭建页核对服务端环境变量。', 'Check that the resulting request contains a single <code>/v1/chat/completions</code>. For self-hosted installations, see the app deployment guide for server settings.'],
  },
  lobechat: {
    name: 'LobeHub', official: 'https://github.com/lobehub/lobehub',
    summary: ['在 LobeHub（LobeChat）中配置 OpenAI 兼容供应商和聊天模型。', 'Configure an OpenAI-compatible provider and chat model in LobeHub (LobeChat).'],
    fields: common,
    steps: [
      ['登录后进入设置 → AI 服务商。选择允许覆盖端点的 OpenAI 或 OpenAI 兼容供应商。', 'Sign in and open Settings → AI Providers. Choose OpenAI or an OpenAI-compatible provider that accepts a custom endpoint.'],
      ['启用该供应商，填 API Key 与代理地址。如果界面允许拉取模型列表，可以先拉取。列表中没有时手动输入模型 ID。', 'Enable the provider and fill in the API key and proxy URL. Fetch the model list if offered, or add the exact model ID manually.'],
      ['启用目标模型，回到对话页，按服务商和名称选择该模型后发送测试消息。', 'Enable the model, return to Chat, and select it under the configured provider before testing.'],
    ],
    check: ['如果有多份同名模型，确认选择的是刚配置的供应商。空白回复时先试简短纯文本消息，再检查流式响应、模型能力和用量。', 'If multiple providers list the same model name, choose the configured provider. For an empty reply, try plain text, then check streaming, model capability, and usage.'],
  },
  'fluent-read': {
    name: 'FluentRead', official: 'https://github.com/Bistutu/FluentRead',
    summary: ['在流畅阅读扩展里使用极速蹬做划词或网页翻译。', 'Use Jisudeng for selection or webpage translation in FluentRead.'],
    fields: common,
    steps: [
      ['从项目提供的官方扩展商店链接安装。打开扩展选项，进入翻译服务设置。', 'Install from an extension store linked by the project. Open extension Options and locate translation services.'],
      ['选择允许自定义 API 地址的 OpenAI 或自定义大模型引擎。若字段是 OpenAI Base URL，填写上表地址。若界面明确要求完整请求地址，填 <code>https://your-host/v1/chat/completions</code>。', 'Choose OpenAI or a custom LLM engine that accepts an endpoint override. Use the URL above when asked for an OpenAI base URL. If the field explicitly asks for the complete request URL, use <code>https://your-host/v1/chat/completions</code>.'],
      ['填写密钥与模型，保存后将此引擎设为当前翻译服务。先用公开网页的一句话测试。', 'Enter the key and model, save, and make this the active translation service. Start with one sentence on a public page.'],
    ],
    check: ['用量中没有翻译请求时，核对当前选中的翻译引擎和扩展对页面的访问权限。', 'If translation does not appear in Usage, check the active engine and the extension’s site permissions.'],
  },
  'luna-translator': {
    name: 'LunaTranslator', official: 'https://docs.lunatranslator.org/zh/',
    summary: ['通过大模型通用接口为 LunaTranslator 配置文本翻译。', 'Connect a model to LunaTranslator for text translation.'],
    fields: common,
    steps: [
      ['从官方项目安装，打开「设置 → 翻译设置 → 大模型」。添加一个配置，选用大模型通用接口模板。', 'Install the official release, then open Settings → Translation → Large Models. Add a configuration from the generic LLM template.'],
      ['填写上表中的 API 地址和 API Key。刷新模型列表，或手动填写精确 ID，保存配置。', 'Enter the API URL and key above. Refresh the model list or enter the exact ID manually, then save.'],
      ['打开该配置的启用开关。先用普通文本运行一次翻译，然后设置游戏文本提取。', 'Enable the configuration. Translate a plain test sentence before setting up game text capture.'],
    ],
    check: ['如果普通文本可译而游戏没反应，检查文本抓取与当前翻译引擎。', 'If plain text works but a game does not, check text capture and the active translation engine.'],
  },
  aionui: {
    name: 'AionUi', official: 'https://github.com/iOfficeAI/AionUi/releases',
    summary: ['给 AionUi 的模型配置添加极速蹬服务商，用于桌面对话。', 'Add Jisudeng to AionUi’s model settings for desktop conversations.'],
    fields: common,
    steps: [
      ['安装对应系统的发行版。在设置的「模型配置」中选择添加模型或服务商。', 'Install the release for your platform. In Model Settings, add a model or provider.'],
      ['在提供自定义 OpenAI 接口的版本中，选择 OpenAI 兼容供应商（例如界面提供的 NewAPI），填 Base URL 和 API Key。添加指定模型并选用 OpenAI 兼容协议。', 'On versions supporting custom endpoints, choose an OpenAI-compatible provider, such as NewAPI if offered, then fill in the URL and key. Add the model and select the OpenAI-compatible protocol.'],
      ['回到新会话，选择使用该服务商的模型。外部 CLI 代理的模型配置可能由各 CLI 单独管理。', 'Start a new session and choose the model under the new provider. External CLI agents may use their own configuration.'],
    ],
    check: ['若当前代理仍使用其他服务商，切换到刚添加的模型，或用该 CLI 的独立教程配置。', 'If the current agent uses a different provider, switch to the configured model or follow the separate guide for that CLI.'],
  },
  langbot: {
    name: 'LangBot', official: 'https://langbot.app/docs/',
    summary: ['先验证模型，再把它用到 LangBot 流水线和聊天机器人。', 'Verify a model first, then use it in a LangBot pipeline and bot.'],
    fields: common,
    steps: [
      ['按官方说明运行 LangBot。打开模型管理，添加 OpenAI 兼容的对话模型。', 'Run LangBot using its official instructions. Open model management and add an OpenAI-compatible chat model.'],
      ['填写 Base URL、密钥和模型 ID。若当前版本将可配置 API 端点的提供商称为 NewAPI，选该提供商。', 'Fill in the base URL, key, and model ID. If your version labels the custom-endpoint provider NewAPI, select it.'],
      ['保存后在流水线里选择这个模型，用调试会话验证回复。搭建知识库时再配置单独的 Embedding 模型。', 'Save, select the model in a pipeline, and verify with a debug conversation. Configure a separate embedding model if you add a knowledge base.'],
    ],
    check: ['调试正常但聊天平台无回复时，核对机器人与流水线绑定、渠道权限和机器人日志。', 'If debugging works but a chat platform receives no reply, check the pipeline binding, channel permissions, and bot logs.'],
  },
  astrbot: {
    name: 'AstrBot', official: 'https://docs.astrbot.app/',
    summary: ['在 AstrBot 管理面板配置服务商和机器人默认模型。', 'Configure a provider and a bot’s default model in the AstrBot dashboard.'],
    fields: common,
    steps: [
      ['运行官方发行版，进入管理面板的服务商或模型设置，新增 OpenAI 兼容服务。', 'Run an official release and open provider or model settings in the dashboard. Add an OpenAI-compatible provider.'],
      ['填地址、API Key 与模型 ID。启用提供商，并在机器人配置中选择该模型作为对话模型。', 'Enter the URL, key, and model ID. Enable the provider and select that model for the bot’s conversations.'],
      ['在管理面板或已绑定的测试会话发送一条消息。确认回复后再绑定其他渠道。', 'Send a test in the dashboard or a bound test conversation before adding more channels.'],
    ],
    check: ['用量正常而群聊没有回复时，查看渠道适配器、触发条件以及机器人是否应用了当前配置。', 'If usage is recorded but group chat gets no reply, check the channel adapter, trigger rules, and active bot configuration.'],
  },
  'app-deployment': {
    name: 'App deployment', official: 'https://github.com/ChatGPTNextWeb/NextChat',
    summary: ['将极速蹬接到自托管聊天或知识库应用，检查服务端配置与用量。', 'Connect a self-hosted chat or knowledge base app and verify its server configuration and usage.'],
    steps: [
      ['准备专用低额度密钥，在「模型与价格」核对应用所需接口。知识库的 Embedding 与聊天模型需分别核对。', 'Create a dedicated key with a small budget and check the interfaces the app needs in Models and Pricing. Check embedding and chat models separately.'],
      ['NextChat 使用部署版本支持的服务端变量 <code>BASE_URL</code> 和 <code>OPENAI_API_KEY</code>，地址填根域名。使用 <code>CODE</code> 或部署环境的身份认证限制访问。', 'For compatible NextChat releases, set server-side <code>BASE_URL</code> to the gateway root and configure <code>OPENAI_API_KEY</code>. Use <code>CODE</code> or your hosting platform authentication to restrict access.'],
      ['LobeHub 和 Dify 建议先走各自后台的服务商设置。通过下列教程填写地址、密钥及模型，完成后重启或重新部署需要加载环境变量的服务。', 'Use the provider settings in the LobeHub or Dify dashboard. Follow the guides below for the URL, key, and model. Restart or redeploy services that need to reload environment variables.'],
      ['依次查看 <a href="/docs?cat=tools&amp;page=nextchat">NextChat</a>、<a href="/docs?cat=tools&amp;page=lobechat">LobeHub</a> 或 <a href="/docs?cat=tools&amp;page=dify">Dify</a>。密钥只放进服务器的 Secret 管理或应用后台，勿提交至代码仓库或打包进前端。', 'See <a href="/docs?cat=tools&amp;page=nextchat">NextChat</a>, <a href="/docs?cat=tools&amp;page=lobechat">LobeHub</a>, or <a href="/docs?cat=tools&amp;page=dify">Dify</a>. Store keys in server secrets or app settings. Keep them out of source control and client bundles.'],
    ],
    example: `BASE_URL=${ROOT}\nOPENAI_API_KEY=${KEY}`,
    check: ['本机成功但线上失败时，从部署服务所在环境检查出站网络、变量注入和访问权限。', 'If local setup works but deployment fails, check outbound connectivity, injected variables, and access policies from the deployed service.'],
  },
}

export function getAdditionalToolGuide(language: PublicDocLocale, pageId: string) {
  if (!Object.prototype.hasOwnProperty.call(guides, pageId)) return undefined
  const guide = guides[pageId]
  const zh = language === 'zh'
  const index = zh ? 0 : 1
  const fields = guide.fields?.map(([label, value]) => `<div><dt>${label[index]}</dt><dd><code>${value}</code></dd></div>`).join('')
  const example = guide.example?.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return {
    name: zh && pageId === 'app-deployment' ? '应用搭建' : guide.name,
    official: guide.official,
    use: guide.summary[index],
    setup: `${fields ? `<figure class="docs-parameter-figure"><figcaption>${zh ? '参数填写示意' : 'Configuration field map'}</figcaption><dl>${fields}</dl></figure>` : ''}
<ol>${guide.steps.map((step) => `<li>${step[index]}</li>`).join('')}</ol>${example ? `<pre><code>${example}</code></pre>` : ''}`,
    check: guide.check[index],
  }
}
