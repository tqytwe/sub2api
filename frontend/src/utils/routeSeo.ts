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
  structuredType: 'WebSite' | 'CollectionPage'
  alternates: Array<{ hreflang: string; path: string }>
}

const SEO_ORIGIN = 'https://www.jisudeng.com'
const SEO_AUTHOR = 'Jisudeng'
const SEO_IMAGE = 'https://www.jisudeng.com/logo.png'
const SEO_IMAGE_ALT = 'Jisudeng logo'
const SEO_FORMAT_DETECTION = 'telephone=no,email=no,address=no'
const SEO_TWITTER_CARD = 'summary'
const SEO_TWITTER_HANDLE = '@jisudeng'

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
      { hreflang: 'x-default', path: '/en/' },
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
      { hreflang: 'x-default', path: '/en/' },
    ],
  },
  '/models': {
    title: '极速蹬模型价格与 API 目录 - 多模型公开计费与调用指南',
    description: '查看极速蹬公开模型目录、模型平台、用途分类与 USD / 1M tokens 计费参考，覆盖 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等模型，登录后可查看分组有效价格与调用权限，帮助开发者快速评估成本。',
    twitterTitle: '极速蹬模型价格与 API 目录 - DeepSeek、Qwen、Kimi、GLM、Claude 多模型公开计费',
    twitterDescription: '查看极速蹬公开模型目录、模型平台、用途分类、USD / 1M tokens 计费参考和分组有效价格，覆盖 DeepSeek、Qwen、Kimi、GLM、GPT、Claude、Gemini 等多模型 API 调用，帮助开发者快速评估成本、调用权限、模型能力与接入路径，并在登录后核对账号实际可用价格表。',
    keywords: '极速蹬模型价格, AI 模型目录, API 计费, DeepSeek API 价格, Qwen API 价格, Kimi API 价格, GLM API 价格, Claude API, Gemini API, OpenAI 兼容接口',
    canonicalPath: '/models',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    ogType: 'website',
    siteName: '极速蹬',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'zh-CN', path: '/models' },
      { hreflang: 'en', path: '/en/models' },
      { hreflang: 'x-default', path: '/en/models' },
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
      { hreflang: 'x-default', path: '/en/docs' },
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
      { hreflang: 'x-default', path: '/en/' },
    ],
  },
  '/en/models': {
    title: 'DeepSeek, Qwen, Kimi, GLM, Claude API Pricing | Jisudeng',
    description: 'Compare model access and usage-based API rates for DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through Jisudeng.',
    twitterTitle: 'DeepSeek, Qwen, Kimi, GLM, GPT, Claude API Pricing | Jisudeng',
    twitterDescription: 'Compare model access, public API rates, and usage-based pricing for DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through Jisudeng, with docs and API key setup guidance.',
    keywords: 'AI model API pricing, DeepSeek API pricing, Qwen API pricing, Kimi API pricing, GLM API pricing, Claude API, Gemini API, OpenAI-compatible models, usage-based billing',
    canonicalPath: '/en/models',
    lang: 'en',
    ogLocale: 'en_US',
    ogType: 'website',
    siteName: 'Jisudeng',
    structuredType: 'CollectionPage',
    alternates: [
      { hreflang: 'en', path: '/en/models' },
      { hreflang: 'zh-CN', path: '/models' },
      { hreflang: 'x-default', path: '/en/models' },
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
      { hreflang: 'x-default', path: '/en/docs' },
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

function buildStructuredData(seo: RouteSeo, canonical: string) {
  const structuredName = seo.structuredType === 'WebSite' ? seo.siteName : seo.title
  const graph: Record<string, unknown> = {
    '@context': 'https://schema.org',
    '@type': seo.structuredType,
    name: structuredName,
    headline: seo.title,
    description: seo.description,
    url: canonical,
    inLanguage: seo.lang,
    isPartOf: {
      '@type': 'WebSite',
      name: 'Jisudeng',
      url: `${SEO_ORIGIN}/`,
    },
    publisher: {
      '@type': 'Organization',
      name: seo.siteName,
      url: `${SEO_ORIGIN}/`,
      logo: {
        '@type': 'ImageObject',
        url: SEO_IMAGE,
      },
    },
  }

  if (seo.structuredType === 'WebSite') {
    graph.alternateName = seo.lang === 'zh-CN' ? ['极速蹬', 'Jisudeng'] : ['Jisudeng']
    graph.mainEntity = {
      '@type': 'SoftwareApplication',
      name: 'Jisudeng',
      applicationCategory: 'DeveloperApplication',
      operatingSystem: 'Web',
      description: seo.description,
      url: canonical,
    }
  }

  return graph
}

function setStructuredData(seo: RouteSeo, canonical: string) {
  let script = document.head.querySelector<HTMLScriptElement>('script[type="application/ld+json"][data-jisudeng-route-seo="true"]')
  if (!script) {
    script = document.createElement('script')
    script.type = 'application/ld+json'
    script.dataset.jisudengRouteSeo = 'true'
    document.head.appendChild(script)
  }
  script.textContent = JSON.stringify(buildStructuredData(seo, canonical))
}

export function resolvePublicRouteSeo(path: string): RouteSeo | undefined {
  return ROUTE_SEO[normalizePath(path)]
}

export function applyPublicRouteSeo(path: string): RouteSeo | undefined {
  const seo = resolvePublicRouteSeo(path)
  if (!seo || typeof document === 'undefined') {
    return seo
  }

  const canonical = absoluteURL(seo.canonicalPath)
  document.documentElement.setAttribute('lang', seo.lang)
  document.title = seo.title
  setMeta('meta[name="description"]', 'name', 'description', seo.description)
  setMeta('meta[name="keywords"]', 'name', 'keywords', seo.keywords)
  setMeta('meta[name="author"]', 'name', 'author', SEO_AUTHOR)
  setMeta('meta[name="format-detection"]', 'name', 'format-detection', SEO_FORMAT_DETECTION)
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
