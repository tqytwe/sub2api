type RouteSeo = {
  title: string
  description: string
  twitterTitle?: string
  twitterDescription?: string
  keywords: string
  canonicalPath: string
  lang: 'en' | 'zh-CN'
  ogLocale: 'en_US' | 'zh_CN'
  ogType: 'website' | 'article'
  siteName: string
  // Content categories are retained for route metadata compatibility only.
  // They are not emitted as Schema.org types. Structured data is restricted
  // to visible Organization, WebApplication, and catalog ItemList content.
  structuredType: 'WebSite' | 'CollectionPage' | 'AboutPage' | 'ContactPage'
  alternates: Array<{ hreflang: string; path: string }>
}

const SEO_ORIGIN = 'https://www.jisudeng.com'
const SEO_AUTHOR = 'Jisudeng'
const SEO_IMAGE = 'https://www.jisudeng.com/logo.png'
const SEO_IMAGE_ALT = 'Jisudeng logo'
const SEO_FORMAT_DETECTION = 'telephone=no,email=no,address=no'
const SEO_TWITTER_CARD = 'summary'
const SEO_TWITTER_HANDLE = '@jisudeng'

type PublicCatalogFallbackEntry = {
  path: string
  name: string
  description: string
}

function isLegacyModelsApiPath(path: string): boolean {
  return path === '/models'
    || path.startsWith('/models/')
    || path === '/en/models'
    || path.startsWith('/en/models/')
}

const ROUTE_SEO: Record<string, RouteSeo> = {
  '/': {
    title: '极速蹬 - OpenAI兼容 AI API 网关与多模型服务平台',
    description: '极速蹬为开发者、团队和 AI 工具用户提供 OpenAI 兼容 API 网关，统一接入 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等模型，支持公开价格、文档、图像生成、API Key 管理与按量计费。',
    twitterTitle: '极速蹬 OpenAI兼容 API 网关 - DeepSeek、Qwen、Kimi、GLM 多模型统一接入',
    twitterDescription: '极速蹬统一接入 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等模型，提供 OpenAI 兼容 API、公开模型价格、图像生成、API Key 管理、文档和按量计费服务，适合把现有 OpenAI SDK、AI 工具、自动化脚本和业务应用迁移到统一入口，并持续比较模型能力、调用权限与成本。',
    keywords: '极速蹬, AI API Gateway, OpenAI 兼容 API, AI 模型接口, DeepSeek API, Qwen API, Kimi API, GLM API, Claude API, Gemini API, 图像生成 API, API Key, 模型价格',
    canonicalPath: '/',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'WebSite',
    alternates: [
      { hreflang: 'zh-CN', path: '/' },
      { hreflang: 'en', path: '/en/' },
      { hreflang: 'x-default', path: '/' },
    ],
  },
  '/home': {
    title: '极速蹬 - OpenAI兼容 AI API 网关与多模型服务平台',
    description: '极速蹬为开发者、团队和 AI 工具用户提供 OpenAI 兼容 API 网关，统一接入 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等模型，支持公开价格、文档、图像生成、API Key 管理与按量计费。',
    twitterTitle: '极速蹬 OpenAI兼容 API 网关 - DeepSeek、Qwen、Kimi、GLM 多模型统一接入',
    twitterDescription: '极速蹬统一接入 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等模型，提供 OpenAI 兼容 API、公开模型价格、图像生成、API Key 管理、文档和按量计费服务，适合把现有 OpenAI SDK、AI 工具、自动化脚本和业务应用迁移到统一入口，并持续比较模型能力、调用权限与成本。',
    keywords: '极速蹬, AI API Gateway, OpenAI 兼容 API, AI 模型接口, DeepSeek API, Qwen API, Kimi API, GLM API, Claude API, Gemini API, 图像生成 API, API Key, 模型价格',
    canonicalPath: '/',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'WebSite',
    alternates: [
      { hreflang: 'zh-CN', path: '/' },
      { hreflang: 'en', path: '/en/' },
      { hreflang: 'x-default', path: '/' },
    ],
  },
  '/catalog': {
    title: '极速蹬模型价格与 API 目录 - 多模型公开计费与调用指南',
    description: '查看极速蹬公开模型目录、模型平台、用途分类与 USD / 1M tokens 计费参考，覆盖 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等模型，登录后可查看分组有效价格与调用权限，帮助开发者快速评估成本。',
    twitterTitle: '极速蹬模型价格与 API 目录 - DeepSeek、Qwen、Kimi、GLM、Claude 多模型公开计费',
    twitterDescription: '查看极速蹬公开模型目录、模型平台、用途分类、USD / 1M tokens 计费参考和分组有效价格，覆盖 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等多模型 API 调用，帮助开发者快速评估成本、调用权限、模型能力与接入路径，并在登录后核对账号实际可用价格表。',
    keywords: '极速蹬模型价格, AI 模型目录, API 计费, DeepSeek API 价格, Qwen API 价格, Kimi API 价格, GLM API 价格, Claude API, Gemini API, OpenAI 兼容接口',
    canonicalPath: '/catalog',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/catalog' },
      { hreflang: 'en', path: '/en/catalog' },
      { hreflang: 'x-default', path: '/catalog' },
    ],
  },
  '/catalog/deepseek': {
    title: 'DeepSeek API 价格与模型接入 - 极速蹬多模型目录',
    description: '查看极速蹬 DeepSeek 模型目录、公开 API 价格、本站基础售价和 USD / 1M tokens 计费参考，支持通过 OpenAI 兼容 API、统一 Key、接入文档、图像能力说明和账单入口快速评估模型能力、调用权限、接入路径与上线成本。',
    twitterTitle: 'DeepSeek API 价格与模型接入 - 极速蹬 OpenAI 兼容 API、公开计费、文档、调用与成本指南',
    twitterDescription: '查看极速蹬 DeepSeek 模型目录、公开 API 价格、本站基础售价、USD / 1M tokens 计费参考、OpenAI 兼容接入方式、API Key 设置、文档入口、图像能力说明、分组实付价提示和账单规则，帮助开发者在登录前评估模型能力、调用权限、接入路径、测试步骤、生产迁移、日常调用、成本监控和上线成本。',
    keywords: 'DeepSeek API 价格, DeepSeek 模型价格, DeepSeek 接口, OpenAI 兼容 DeepSeek, 极速蹬模型目录, API 计费, 模型接入',
    canonicalPath: '/catalog/deepseek',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/catalog/deepseek' },
      { hreflang: 'en', path: '/en/catalog/deepseek' },
      { hreflang: 'x-default', path: '/catalog/deepseek' },
    ],
  },
  '/catalog/qwen': {
    title: 'Qwen API 价格与模型接入 - 极速蹬多模型公开计费目录',
    description: '查看极速蹬 Qwen 模型目录、公开 API 价格、本站基础售价和 USD / 1M tokens 计费参考，支持通过 OpenAI 兼容 API、统一 Key、接入文档、图像能力说明和账单入口快速评估模型能力、调用权限、接入路径与上线成本。',
    twitterTitle: 'Qwen API 价格与模型接入 - 极速蹬 OpenAI 兼容 API、公开计费、文档、调用与成本指南',
    twitterDescription: '查看极速蹬 Qwen 模型目录、公开 API 价格、本站基础售价、USD / 1M tokens 计费参考、OpenAI 兼容接入方式、API Key 设置、文档入口、图像能力说明、分组实付价提示和账单规则，帮助开发者在登录前评估模型能力、调用权限、接入路径、测试步骤、生产迁移、日常调用、成本监控和上线成本。',
    keywords: 'Qwen API 价格, Qwen 模型价格, Qwen 接口, OpenAI 兼容 Qwen, 极速蹬模型目录, API 计费, 模型接入',
    canonicalPath: '/catalog/qwen',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/catalog/qwen' },
      { hreflang: 'en', path: '/en/catalog/qwen' },
      { hreflang: 'x-default', path: '/catalog/qwen' },
    ],
  },
  '/catalog/kimi': {
    title: 'Kimi API 价格与模型接入 - 极速蹬多模型公开计费目录',
    description: '查看极速蹬 Kimi 模型目录、公开 API 价格、本站基础售价和 USD / 1M tokens 计费参考，支持通过 OpenAI 兼容 API、统一 Key、接入文档、图像能力说明和账单入口快速评估模型能力、调用权限、接入路径与上线成本。',
    twitterTitle: 'Kimi API 价格与模型接入 - 极速蹬 OpenAI 兼容 API、公开计费、文档、调用与成本指南',
    twitterDescription: '查看极速蹬 Kimi 模型目录、公开 API 价格、本站基础售价、USD / 1M tokens 计费参考、OpenAI 兼容接入方式、API Key 设置、文档入口、图像能力说明、分组实付价提示和账单规则，帮助开发者在登录前评估模型能力、调用权限、接入路径、测试步骤、生产迁移、日常调用、成本监控和上线成本。',
    keywords: 'Kimi API 价格, Kimi 模型价格, Kimi 接口, OpenAI 兼容 Kimi, 极速蹬模型目录, API 计费, 模型接入',
    canonicalPath: '/catalog/kimi',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/catalog/kimi' },
      { hreflang: 'en', path: '/en/catalog/kimi' },
      { hreflang: 'x-default', path: '/catalog/kimi' },
    ],
  },
  '/catalog/glm': {
    title: 'GLM API 价格与模型接入 - 极速蹬多模型公开计费目录',
    description: '查看极速蹬 GLM 模型目录、公开 API 价格、本站基础售价和 USD / 1M tokens 计费参考，支持通过 OpenAI 兼容 API、统一 Key、接入文档、图像能力说明和账单入口快速评估模型能力、调用权限、接入路径与上线成本。',
    twitterTitle: 'GLM API 价格与模型接入 - 极速蹬 OpenAI 兼容 API、公开计费、文档、调用与成本指南',
    twitterDescription: '查看极速蹬 GLM 模型目录、公开 API 价格、本站基础售价、USD / 1M tokens 计费参考、OpenAI 兼容接入方式、API Key 设置、文档入口、图像能力说明、分组实付价提示和账单规则，帮助开发者在登录前评估模型能力、调用权限、接入路径、测试步骤、生产迁移、日常调用、成本监控和上线成本。',
    keywords: 'GLM API 价格, GLM 模型价格, GLM 接口, OpenAI 兼容 GLM, 极速蹬模型目录, API 计费, 模型接入',
    canonicalPath: '/catalog/glm',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/catalog/glm' },
      { hreflang: 'en', path: '/en/catalog/glm' },
      { hreflang: 'x-default', path: '/catalog/glm' },
    ],
  },
  '/docs': {
    title: '极速蹬 API 文档 - OpenAI兼容接口、模型调用与计费指南',
    description: '阅读极速蹬 API Key、OpenAI 兼容接口、模型选择、图片生成、异步任务、Batch Image、工具接入和计费说明，快速完成从注册到生产调用的配置。文档覆盖常见 SDK、命令行工具、环境安装与排障，适合开发者、团队和 AI 工具用户查阅。',
    twitterTitle: '极速蹬 API 文档 - OpenAI兼容接口、模型调用、图像生成、Batch Image 与计费指南',
    twitterDescription: '阅读极速蹬 API 文档，完成 API Key、OpenAI 兼容接口、模型调用、图片生成、异步任务、Batch Image、工具接入、计费说明、SDK 配置和生产排障，帮助团队从注册、创建 Key、切换 base URL 到上线调用都能快速查到步骤、示例、权限说明、排障路径、上线检查和常见问题答案。',
    keywords: '极速蹬文档, API 接入指南, OpenAI 兼容 SDK, API Key 设置, 图片生成 API, Batch Image, 异步图片任务, Claude Code, Codex CLI, Gemini CLI',
    canonicalPath: '/docs',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'article',
    siteName: '极速蹬',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/docs' },
      { hreflang: 'en', path: '/en/docs' },
      { hreflang: 'x-default', path: '/docs' },
    ],
  },
  '/download/android': {
    title: 'JisudengChat Android 下载 - 极速蹬 AI 客户端与移动模型调用入口',
    description: '下载 JisudengChat Android APK，使用极速蹬账号登录并同步余额、分组、API Key 和可用模型，在手机上查看账户状态、接入文档、模型调用入口和更新信息。页面提供版本号、安装包校验、下载说明与故障提示，适合已注册用户快速安装并开始移动端使用。',
    twitterTitle: 'JisudengChat Android 下载 - 极速蹬 AI 客户端、API Key 与模型调用入口',
    twitterDescription: '下载 JisudengChat Android APK，使用极速蹬账号登录并同步余额、分组、API Key、可用模型、接入文档、账户状态和更新信息。页面提供版本号、安装包校验、下载说明、更新提示和移动端入口，帮助已注册用户在手机上快速进入 AI 模型调用、账户查看、文档查询、移动测试与日常使用工作流。',
    keywords: 'JisudengChat Android, 极速蹬 APP, Android APK 下载, AI 客户端, API Key, 模型调用, 移动端 AI 工具',
    canonicalPath: '/download/android',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/download/android' },
      { hreflang: 'x-default', path: '/download/android' },
    ],
  },
  '/about': {
    title: '关于极速蹬 - OpenAI兼容 API 网关、模型目录与提示词库',
    description: '了解极速蹬如何为开发者、团队和 AI 工具用户提供 OpenAI 兼容 API 网关、模型目录、公开价格、接入文档、图像生成、API Key 管理和提示词库，统一接入 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等模型，帮助快速评估能力、成本和上线路径。',
    twitterTitle: '关于极速蹬 - OpenAI兼容 API 网关、模型价格、图像生成、API 文档与提示词库服务入口指南',
    twitterDescription: '了解极速蹬如何统一 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等模型入口，提供 OpenAI 兼容 API、公开价格、API Key 管理、接入文档、图像生成和提示词库，帮助开发者、团队和 AI 工具用户更快完成模型选择、成本评估、测试接入、生产迁移与日常调用。',
    keywords: '关于极速蹬, AI API 网关, OpenAI 兼容 API, 多模型服务, 模型价格, API 文档, 图像生成, 提示词库, DeepSeek API, Qwen API, Kimi API, GLM API',
    canonicalPath: '/about',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'AboutPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/about' },
      { hreflang: 'en', path: '/en/about' },
      { hreflang: 'x-default', path: '/about' },
    ],
  },
  '/contact': {
    title: '联系极速蹬客服 - API、模型调用、充值、账号与接入支持入口',
    description: '联系极速蹬客服获取登录注册、API Key、模型调用、充值支付、余额、分组权限、文档接入、图像生成、账号安全和故障排查支持。我们会帮助开发者和团队确认 OpenAI 兼容 API 接入路径、模型可用性、价格说明、调用示例、SDK 配置和上线问题。',
    twitterTitle: '联系极速蹬客服 - API Key、模型调用、充值、账号、图像生成、API 文档与开发者接入支持入口',
    twitterDescription: '通过极速蹬客服入口获取登录注册、API Key、模型调用、充值支付、余额、分组权限、文档接入、图像生成、账号安全和故障排查支持。我们会围绕 OpenAI 兼容 API、DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等模型的可用性、价格说明、接入配置和上线问题提供帮助。',
    keywords: '联系极速蹬, 极速蹬客服, API Key 支持, 模型调用支持, 充值支付支持, OpenAI 兼容 API 接入, 图像生成支持, 开发者支持',
    canonicalPath: '/contact',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'ContactPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/contact' },
      { hreflang: 'en', path: '/en/contact' },
      { hreflang: 'x-default', path: '/contact' },
    ],
  },
  '/en': {
    title: 'Jisudeng: One OpenAI-Compatible API for Frontier AI Models',
    description: 'Access DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through one OpenAI-compatible API with unified keys, public pricing, image APIs, billing, docs.',
    twitterDescription: 'Jisudeng unifies DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more behind one OpenAI-compatible API, with public pricing, API docs, image generation, usage controls, and billing.',
    keywords: 'Jisudeng, OpenAI-compatible API, AI API gateway, DeepSeek API, Qwen API, Kimi API, GLM API, Claude API, Gemini API, model pricing, image generation API, API key',
    canonicalPath: '/en/',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'website',
    siteName: 'Jisudeng',
    structuredType: 'WebSite',
    alternates: [
      { hreflang: 'en', path: '/en/' },
      { hreflang: 'zh-CN', path: '/' },
      { hreflang: 'x-default', path: '/' },
    ],
  },
  '/en/catalog': {
    title: 'DeepSeek, Qwen, Kimi, GLM, Claude API Pricing | Jisudeng',
    description: 'Compare model access and usage-based API rates for DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through Jisudeng.',
    twitterTitle: 'DeepSeek, Qwen, Kimi, GLM, GPT, Claude API Pricing | Jisudeng',
    twitterDescription: 'Compare model access, public API rates, and usage-based pricing for DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through Jisudeng, with docs and API key setup guidance.',
    keywords: 'AI model API pricing, DeepSeek API pricing, Qwen API pricing, Kimi API pricing, GLM API pricing, Claude API, Gemini API, OpenAI-compatible models, usage-based billing',
    canonicalPath: '/en/catalog',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'website',
    siteName: 'Jisudeng',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'en', path: '/en/catalog' },
      { hreflang: 'zh-CN', path: '/catalog' },
      { hreflang: 'x-default', path: '/catalog' },
    ],
  },
  '/en/catalog/deepseek': {
    title: 'DeepSeek API Pricing and Access | Jisudeng',
    description: 'Compare DeepSeek model access, public API rates, and usage-based pricing through Jisudeng. Use an OpenAI-compatible base URL, unified keys, docs, and billing.',
    twitterTitle: 'DeepSeek API Pricing, Models, and OpenAI-Compatible Access',
    twitterDescription: 'Compare DeepSeek model access, public API rates, usage-based pricing, OpenAI-compatible setup, API key guidance, docs, image API notes, and billing through Jisudeng.',
    keywords: 'DeepSeek API pricing, DeepSeek model pricing, DeepSeek API access, OpenAI-compatible DeepSeek, Jisudeng model catalog, usage-based billing',
    canonicalPath: '/en/catalog/deepseek',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'website',
    siteName: 'Jisudeng',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'en', path: '/en/catalog/deepseek' },
      { hreflang: 'zh-CN', path: '/catalog/deepseek' },
      { hreflang: 'x-default', path: '/catalog/deepseek' },
    ],
  },
  '/en/catalog/qwen': {
    title: 'Qwen API Pricing and Access | Jisudeng',
    description: 'Compare Qwen model access, public API rates, and usage-based pricing through Jisudeng. Use one OpenAI-compatible base URL with unified keys, docs, and billing.',
    twitterTitle: 'Qwen API Pricing, Models, and OpenAI-Compatible Access',
    twitterDescription: 'Compare Qwen model access, public API rates, usage-based pricing, OpenAI-compatible setup, API key guidance, docs, image API notes, and billing through Jisudeng.',
    keywords: 'Qwen API pricing, Qwen model pricing, Qwen API access, OpenAI-compatible Qwen, Jisudeng model catalog, usage-based billing',
    canonicalPath: '/en/catalog/qwen',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'website',
    siteName: 'Jisudeng',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'en', path: '/en/catalog/qwen' },
      { hreflang: 'zh-CN', path: '/catalog/qwen' },
      { hreflang: 'x-default', path: '/catalog/qwen' },
    ],
  },
  '/en/catalog/kimi': {
    title: 'Kimi API Pricing and Access | Jisudeng',
    description: 'Compare Kimi model access, public API rates, and usage-based pricing through Jisudeng. Use one OpenAI-compatible base URL with unified keys, docs, and billing.',
    twitterTitle: 'Kimi API Pricing, Models, and OpenAI-Compatible Access',
    twitterDescription: 'Compare Kimi model access, public API rates, usage-based pricing, OpenAI-compatible setup, API key guidance, docs, model catalog, and billing through Jisudeng.',
    keywords: 'Kimi API pricing, Kimi model pricing, Kimi API access, OpenAI-compatible Kimi, Jisudeng model catalog, usage-based billing',
    canonicalPath: '/en/catalog/kimi',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'website',
    siteName: 'Jisudeng',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'en', path: '/en/catalog/kimi' },
      { hreflang: 'zh-CN', path: '/catalog/kimi' },
      { hreflang: 'x-default', path: '/catalog/kimi' },
    ],
  },
  '/en/catalog/glm': {
    title: 'GLM API Pricing and Access | Jisudeng',
    description: 'Compare GLM model access, public API rates, and usage-based pricing through Jisudeng. Use one OpenAI-compatible base URL with unified keys, docs, and billing.',
    twitterTitle: 'GLM API Pricing, Models, and OpenAI-Compatible Access',
    twitterDescription: 'Compare GLM model access, public API rates, usage-based pricing, OpenAI-compatible setup, API key guidance, docs, model catalog, and billing through Jisudeng.',
    keywords: 'GLM API pricing, GLM model pricing, GLM API access, OpenAI-compatible GLM, Jisudeng model catalog, usage-based billing',
    canonicalPath: '/en/catalog/glm',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'website',
    siteName: 'Jisudeng',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'en', path: '/en/catalog/glm' },
      { hreflang: 'zh-CN', path: '/catalog/glm' },
      { hreflang: 'x-default', path: '/catalog/glm' },
    ],
  },
  '/en/docs': {
    title: 'Jisudeng API Docs: OpenAI-Compatible Gateway, Models, Images',
    description: 'Use Jisudeng with your existing OpenAI SDK. Change only the base URL and API key to access AI models, image APIs, tool setup guides, billing notes, and docs.',
    twitterDescription: 'Use Jisudeng with your existing OpenAI SDK. Change only the base URL and API key to access AI models, image APIs, tool setup guides, billing notes, and production docs.',
    keywords: 'Jisudeng API docs, OpenAI-compatible SDK, API key setup, AI model API docs, image generation API, Batch Image API, Claude Code setup, Codex CLI setup, Gemini CLI setup',
    canonicalPath: '/en/docs',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'article',
    siteName: 'Jisudeng',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'en', path: '/en/docs' },
      { hreflang: 'zh-CN', path: '/docs' },
      { hreflang: 'x-default', path: '/docs' },
    ],
  },
  '/en/about': {
    title: 'About Jisudeng: A Transparent OpenAI-Compatible API Relay',
    description: 'Learn how Jisudeng provides a transparent OpenAI-compatible API with public pricing, privacy commitments, and practical developer documentation for teams.',
    twitterTitle: 'About Jisudeng: Transparent AI API, Pricing, and Privacy',
    twitterDescription: 'Learn how Jisudeng routes major AI models through an OpenAI-compatible API with transparent channels, public pricing, request-body privacy, image access, developer docs, and support for teams.',
    keywords: 'about Jisudeng, transparent AI API relay, OpenAI-compatible gateway, DeepSeek API, model pricing, API privacy, developer API access',
    canonicalPath: '/en/about',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'website',
    siteName: 'Jisudeng',
    structuredType: 'AboutPage',
    alternates: [
      { hreflang: 'en', path: '/en/about' },
      { hreflang: 'zh-CN', path: '/about' },
      { hreflang: 'x-default', path: '/about' },
    ],
  },
  '/en/contact': {
    title: 'Contact Jisudeng: API, Account, Billing, and Support',
    description: 'Contact Jisudeng for API keys, model access, billing questions, image generation, documentation, integration help, and technical support.',
    twitterTitle: 'Contact Jisudeng: API, Billing, and Developer Support',
    twitterDescription: 'Contact Jisudeng for API keys, model access, billing, image generation, docs, integration help, privacy, and technical support for teams using major AI models.',
    keywords: 'contact Jisudeng, AI API support, API key help, model access support, billing support, OpenAI-compatible integration support, developer support',
    canonicalPath: '/en/contact',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'website',
    siteName: 'Jisudeng',
    structuredType: 'ContactPage',
    alternates: [
      { hreflang: 'en', path: '/en/contact' },
      { hreflang: 'zh-CN', path: '/contact' },
      { hreflang: 'x-default', path: '/contact' },
    ],
  },
}

function normalizePath(path: string): string {
  const clean = path.split('?')[0]?.split('#')[0] ?? '/'
  if (clean === '/') return '/'
  return clean.replace(/\/+$/, '') || '/'
}

function absoluteURL(path: string): string {
  return new URL(path, SEO_ORIGIN).toString()
}

function isHomeRoute(path: string): boolean {
  const normalized = normalizePath(path)
  return normalized === '/' || normalized === '/en'
}

function isCatalogLandingRoute(path: string): boolean {
  const normalized = normalizePath(path)
  return normalized === '/catalog' || normalized === '/en/catalog'
}

// These are the same stable model-family routes linked in the server-rendered
// catalog fallback. They are intentionally not live pricing or entitlement
// claims, so a client-side head update cannot invent data unavailable to a
// crawler or guest.
function publicCatalogFallbackEntries(english: boolean): PublicCatalogFallbackEntry[] {
  if (english) {
    return [
      { path: '/en/catalog/deepseek', name: 'DeepSeek API', description: 'Public access guidance, usage-based rate references, and OpenAI-compatible integration.' },
      { path: '/en/catalog/qwen', name: 'Qwen API', description: 'Public model-family guidance, rate references, and unified API setup.' },
      { path: '/en/catalog/kimi', name: 'Kimi API', description: 'Model access guidance, pricing references, and compatible SDK setup.' },
      { path: '/en/catalog/glm', name: 'GLM API', description: 'Public integration guidance, usage-based pricing references, and API-key setup.' },
    ]
  }

  return [
    { path: '/catalog/deepseek', name: 'DeepSeek API', description: '查看 DeepSeek 系列的公开接入说明、按量计费参考和 OpenAI 兼容调用路径。' },
    { path: '/catalog/qwen', name: 'Qwen API', description: '查看 Qwen 系列的公开模型说明、价格参考和统一 API 配置方式。' },
    { path: '/catalog/kimi', name: 'Kimi API', description: '查看 Kimi 系列的模型接入说明、价格参考和兼容 SDK 配置。' },
    { path: '/catalog/glm', name: 'GLM API', description: '查看 GLM 系列的公开接入说明、按量计费参考和 API Key 配置。' },
  ]
}

function setMeta(selector: string, attrName: 'name' | 'property', attrValue: string, content: string) {
  let node = document.head.querySelector<HTMLMetaElement>(selector)
  if (!node) {
    node = document.createElement('meta')
    node.setAttribute(attrName, attrValue)
    document.head.appendChild(node)
  }
  node.setAttribute('content', content)
}

function setCanonical(href: string) {
  let link = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'canonical'
    document.head.appendChild(link)
  }
  link.href = href
}

function clearPublicRouteMetadata() {
  document.head.querySelectorAll<HTMLLinkElement>('link[rel="canonical"], link[rel="alternate"][hreflang]').forEach((link) => link.remove())
  document.head.querySelectorAll<HTMLScriptElement>('script[type="application/ld+json"][data-jisudeng-route-seo="true"]').forEach((script) => script.remove())
}

function setAlternates(seo: RouteSeo) {
  document.head
    .querySelectorAll<HTMLLinkElement>('link[rel="alternate"][hreflang]')
    .forEach((link) => link.remove())

  for (const alternate of seo.alternates) {
    const link = document.createElement('link')
    link.rel = 'alternate'
    link.hreflang = alternate.hreflang
    link.href = absoluteURL(alternate.path)
    link.dataset.jisudengRouteSeo = 'true'
    document.head.appendChild(link)
  }
}

function buildStructuredData(seo: RouteSeo, canonical: string): Record<string, unknown> | undefined {
  if (isHomeRoute(seo.canonicalPath)) {
    return {
      '@context': 'https://schema.org',
      '@graph': [
        {
          '@type': 'Organization',
          name: seo.siteName,
          url: `${SEO_ORIGIN}/`,
          description: seo.description,
        },
        {
          '@type': 'WebApplication',
          name: 'Jisudeng',
          applicationCategory: 'DeveloperApplication',
          operatingSystem: 'Web',
          description: seo.description,
          url: canonical,
          inLanguage: seo.lang,
        },
      ],
    }
  }

  if (isCatalogLandingRoute(seo.canonicalPath)) {
    return {
      '@context': 'https://schema.org',
      '@type': 'ItemList',
      name: seo.lang === 'en' ? 'Jisudeng public model families' : '极速蹬公开模型系列',
      description: seo.description,
      url: canonical,
      inLanguage: seo.lang,
      itemListElement: publicCatalogFallbackEntries(seo.lang === 'en').map((entry, index) => ({
        '@type': 'ListItem',
        position: index + 1,
        name: entry.name,
        url: absoluteURL(entry.path),
      })),
    }
  }

  return undefined
}

function setStructuredData(seo: RouteSeo, canonical: string) {
  const data = buildStructuredData(seo, canonical)
  if (!data) {
    document.head.querySelectorAll<HTMLScriptElement>('script[type="application/ld+json"][data-jisudeng-route-seo="true"]').forEach((script) => script.remove())
    return
  }
  let script = document.head.querySelector<HTMLScriptElement>('script[type="application/ld+json"][data-jisudeng-route-seo="true"]')
  if (!script) {
    script = document.createElement('script')
    script.type = 'application/ld+json'
    script.dataset.jisudengRouteSeo = 'true'
    document.head.appendChild(script)
  }
  script.textContent = JSON.stringify(data)
}

export function resolvePublicRouteSeo(path: string): RouteSeo | undefined {
  const normalized = normalizePath(path)
  if (normalized === '/home' || isLegacyModelsApiPath(normalized)) return undefined
  return ROUTE_SEO[normalized]
}

export function applyPublicRouteSeo(path: string): RouteSeo | undefined {
  const seo = resolvePublicRouteSeo(path)
  if (typeof document === 'undefined') {
    return seo
  }

  if (!seo) {
    setMeta('meta[name="robots"]', 'name', 'robots', 'noindex,nofollow')
    clearPublicRouteMetadata()
    return undefined
  }

  const canonical = absoluteURL(seo.canonicalPath)
  document.documentElement.setAttribute('lang', seo.lang)
  document.title = seo.title
  setMeta('meta[name="description"]', 'name', 'description', seo.description)
  setMeta('meta[name="keywords"]', 'name', 'keywords', seo.keywords)
  setMeta('meta[name="author"]', 'name', 'author', SEO_AUTHOR)
  setMeta('meta[name="format-detection"]', 'name', 'format-detection', SEO_FORMAT_DETECTION)
  setMeta('meta[name="robots"]', 'name', 'robots', 'index,follow')
  setMeta('meta[property="og:type"]', 'property', 'og:type', seo.ogType)
  setMeta('meta[property="og:site_name"]', 'property', 'og:site_name', seo.siteName)
  setMeta('meta[property="og:title"]', 'property', 'og:title', seo.title)
  setMeta('meta[property="og:description"]', 'property', 'og:description', seo.description)
  setMeta('meta[property="og:url"]', 'property', 'og:url', canonical)
  setMeta('meta[property="og:locale"]', 'property', 'og:locale', seo.ogLocale)
  setMeta('meta[property="og:image"]', 'property', 'og:image', SEO_IMAGE)
  setMeta('meta[property="og:image:alt"]', 'property', 'og:image:alt', SEO_IMAGE_ALT)
  setMeta('meta[name="twitter:card"]', 'name', 'twitter:card', SEO_TWITTER_CARD)
  setMeta('meta[name="twitter:site"]', 'name', 'twitter:site', SEO_TWITTER_HANDLE)
  setMeta('meta[name="twitter:creator"]', 'name', 'twitter:creator', SEO_TWITTER_HANDLE)
  setMeta('meta[name="twitter:title"]', 'name', 'twitter:title', seo.twitterTitle || seo.title)
  setMeta('meta[name="twitter:description"]', 'name', 'twitter:description', seo.twitterDescription || seo.description)
  setMeta('meta[name="twitter:image"]', 'name', 'twitter:image', SEO_IMAGE)
  setMeta('meta[name="twitter:image:alt"]', 'name', 'twitter:image:alt', SEO_IMAGE_ALT)
  setCanonical(canonical)
  setAlternates(seo)
  setStructuredData(seo, canonical)
  return seo
}
