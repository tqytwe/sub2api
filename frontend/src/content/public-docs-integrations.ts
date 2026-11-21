export const DOC_TOOL_GROUPS = ['coding', 'clients', 'applications'] as const

type ToolGroup = typeof DOC_TOOL_GROUPS[number]
export type ToolProtocol = 'anthropic' | 'responses' | 'gemini' | 'openai' | 'provider'

export const DOC_TOOL_INTEGRATIONS: ReadonlyArray<{
  cat: 'tools'
  page: string
  name: string
  group: ToolGroup
  protocol: ToolProtocol
}> = [
  { cat: 'tools', page: 'claude-code', name: 'Claude Code', group: 'coding', protocol: 'anthropic' },
  { cat: 'tools', page: 'codex', name: 'Codex CLI', group: 'coding', protocol: 'responses' },
  { cat: 'tools', page: 'gemini-cli', name: 'Gemini CLI', group: 'coding', protocol: 'gemini' },
  { cat: 'tools', page: 'vscode-claude-code', name: 'VS Code Claude Code', group: 'coding', protocol: 'anthropic' },
  { cat: 'tools', page: 'vscode-codex', name: 'VS Code Codex', group: 'coding', protocol: 'responses' },
  { cat: 'tools', page: 'codex-app', name: 'Codex App', group: 'coding', protocol: 'responses' },
  { cat: 'tools', page: 'factory-droid-cli', name: 'Factory Droid CLI', group: 'coding', protocol: 'openai' },
  { cat: 'tools', page: 'cursor', name: 'Cursor', group: 'coding', protocol: 'openai' },
  { cat: 'tools', page: 'cc-switch', name: 'CC Switch', group: 'coding', protocol: 'provider' },
  { cat: 'tools', page: 'opencode', name: 'OpenCode', group: 'coding', protocol: 'openai' },
  { cat: 'tools', page: 'cline', name: 'Cline', group: 'coding', protocol: 'openai' },
  { cat: 'tools', page: 'continue', name: 'Continue', group: 'coding', protocol: 'openai' },
  { cat: 'tools', page: 'aider', name: 'Aider', group: 'coding', protocol: 'openai' },
  { cat: 'tools', page: 'cherry-studio', name: 'Cherry Studio', group: 'clients', protocol: 'openai' },
  { cat: 'tools', page: 'nextchat', name: 'NextChat', group: 'clients', protocol: 'openai' },
  { cat: 'tools', page: 'lobechat', name: 'LobeHub', group: 'clients', protocol: 'openai' },
  { cat: 'tools', page: 'fluent-read', name: 'FluentRead', group: 'clients', protocol: 'openai' },
  { cat: 'tools', page: 'luna-translator', name: 'LunaTranslator', group: 'clients', protocol: 'openai' },
  { cat: 'tools', page: 'aionui', name: 'AionUi', group: 'applications', protocol: 'openai' },
  { cat: 'tools', page: 'openclaw', name: 'OpenClaw', group: 'applications', protocol: 'provider' },
  { cat: 'tools', page: 'hermes', name: 'Hermes Agent', group: 'applications', protocol: 'openai' },
  { cat: 'tools', page: 'dify', name: 'Dify', group: 'applications', protocol: 'openai' },
  { cat: 'tools', page: 'langbot', name: 'LangBot', group: 'applications', protocol: 'openai' },
  { cat: 'tools', page: 'astrbot', name: 'AstrBot', group: 'applications', protocol: 'openai' },
  { cat: 'tools', page: 'app-deployment', name: 'App deployment', group: 'applications', protocol: 'provider' },
]

export const DOC_READER_COPY = {
  zh: {
    docs: '使用文档', docsHome: '文档首页', integrations: '工具集成', intro: '查阅账号与密钥、模型与计费、API 调用和第三方工具接入说明。',
    firstRequest: '开始前先准备', firstRequestText: '先创建 API 密钥，再根据密钥分组确认可用模型和协议。', quickStart: '快速开始',
    universal: '通用配置', openAIBase: 'OpenAI 兼容地址', anthropicBase: 'Anthropic 根地址', anthropicSuffix: '不加 /v1',
    key: 'API 密钥', models: '模型与价格', console: '控制台', usage: '用量记录',
    modelHelp: '按密钥分组确认支持的协议和模型 ID。',
    tool: '工具 / 教程', protocol: '协议', backToCategory: '返回上一级', geminiBase: 'Gemini 原生地址',
    groups: { coding: '编程工具与 CLI', clients: '聊天客户端与翻译', applications: '智能体、机器人与应用搭建' },
    choose: '怎么选择', chooseText: '命令行编程可用 Claude Code 或 Codex CLI，编辑器内可用 VS Code 扩展。日常聊天和翻译可选 Cherry Studio、NextChat 或 FluentRead。知识库、机器人及自托管应用可从 Dify、LangBot、AstrBot 或应用搭建教程开始。',
    diagnose: '排查顺序', diagnoseKey: '在控制台确认密钥启用、有效且绑定了对应分组。不要在截图或日志中暴露密钥。',
    diagnoseUrl: '按教程核对 API 协议、Base URL 和模型 ID。不要把完整请求路径填进 Base URL。',
    diagnoseUsage: '用一个短提示词复测，在用量记录中核对模型和结果。',
    search: '搜索文档', searchHint: '输入工具名、文章标题或配置关键词', searchEmpty: '没有找到相关文章，试试缩短关键词。',
    navigation: '文档目录', onPage: '本页目录', backToDocs: '返回文档首页', share: '分享此页', copyLink: '复制链接', copyCode: '复制代码', copied: '已复制',
    load: '正在加载文档…', loadError: '文档加载失败。', retry: '重试', notFound: '未找到这篇文档。',

    protocols: { anthropic: 'Anthropic Messages', responses: 'OpenAI Responses', gemini: 'Gemini 原生协议', openai: 'OpenAI 兼容', provider: '按工具选择协议' },
  },
  en: {
    docs: 'Documentation', docsHome: 'Documentation home', integrations: 'Tool integrations', intro: 'Find guides for accounts and API keys, models and billing, API requests, and integrations.',
    firstRequest: 'Before your first request', firstRequestText: 'Create an API key first, then check the models and protocol enabled for its group.', quickStart: 'Quick start',
    universal: 'Common configuration', openAIBase: 'OpenAI-compatible base URL', anthropicBase: 'Anthropic base URL', anthropicSuffix: 'without /v1',
    key: 'API keys', models: 'Models and pricing', console: 'Console', usage: 'Usage',
    modelHelp: 'Check which protocols and model IDs your key group supports.',
    tool: 'Tool / guide', protocol: 'Protocol', backToCategory: 'Back to category', geminiBase: 'Native Gemini base URL',
    groups: { coding: 'Coding tools and CLI', clients: 'Chat clients and translation', applications: 'Agents, bots and app deployment' },
    choose: 'How to choose', chooseText: 'Use Claude Code or Codex CLI in a terminal and a VS Code extension inside the editor. For chat and translation, start with Cherry Studio, NextChat, or FluentRead. For knowledge bases, bots, or self-hosting, start with Dify, LangBot, AstrBot, or the app deployment guide.',
    diagnose: 'Troubleshooting order', diagnoseKey: 'Check that the key is enabled, valid, and assigned to the correct group. Never expose the key in screenshots or logs.',
    diagnoseUrl: 'Match the protocol, base URL, and model ID to the guide. Do not paste a full request path into the base URL field.',
    diagnoseUsage: 'Retry with a short prompt and check the resulting model and request in Usage.',
    search: 'Search docs', searchHint: 'Search a tool, article title, or configuration term', searchEmpty: 'No matching articles. Try a shorter search.',
    navigation: 'Documentation navigation', onPage: 'On this page', backToDocs: 'Back to documentation home', share: 'Share this page', copyLink: 'Copy link', copyCode: 'Copy code', copied: 'Copied',
    load: 'Loading documentation…', loadError: 'Documentation could not load.', retry: 'Retry', notFound: 'This article was not found.',

    protocols: { anthropic: 'Anthropic Messages', responses: 'OpenAI Responses', gemini: 'Native Gemini', openai: 'OpenAI-compatible', provider: 'Depends on the tool' },
  },
} as const
