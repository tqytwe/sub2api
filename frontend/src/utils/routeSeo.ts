type RouteSeo = {
  title: string
  description: string
  canonicalPath: string
  lang: 'en' | 'zh-CN'
  ogLocale: 'en_US' | 'zh_CN'
  alternates: Array<{ hreflang: string; path: string }>
}

const SEO_ORIGIN = 'https://www.jisudeng.com'

const ROUTE_SEO: Record<string, RouteSeo> = {
  '/': {
    title: '极速蹬 - AI API Gateway 与中文提示词库',
    description: '极速蹬是面向开发者、团队和 AI 工具用户的 AI API Gateway，提供模型接口、中文提示词库、图像工作室和统一计费服务。',
    canonicalPath: '/',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    alternates: [
      { hreflang: 'zh-CN', path: '/' },
      { hreflang: 'en', path: '/en/' },
      { hreflang: 'x-default', path: '/en/' },
    ],
  },
  '/home': {
    title: '极速蹬 - AI API Gateway 与中文提示词库',
    description: '极速蹬是面向开发者、团队和 AI 工具用户的 AI API Gateway，提供模型接口、中文提示词库、图像工作室和统一计费服务。',
    canonicalPath: '/',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    alternates: [
      { hreflang: 'zh-CN', path: '/' },
      { hreflang: 'en', path: '/en/' },
      { hreflang: 'x-default', path: '/en/' },
    ],
  },
  '/models': {
    title: '极速蹬模型与价格 - AI API Gateway',
    description: '查看极速蹬公开模型目录、模型平台、用途分类与 USD / 1M tokens 计费参考，登录后可查看分组有效价格。',
    canonicalPath: '/models',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    alternates: [
      { hreflang: 'zh-CN', path: '/models' },
      { hreflang: 'en', path: '/en/models' },
      { hreflang: 'x-default', path: '/en/models' },
    ],
  },
  '/docs': {
    title: '极速蹬使用文档 - API 接入指南',
    description: '阅读极速蹬 API Key、OpenAI 兼容接口、图片生成、异步任务、Batch Image 与计费说明。',
    canonicalPath: '/docs',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    alternates: [
      { hreflang: 'zh-CN', path: '/docs' },
      { hreflang: 'en', path: '/en/docs' },
      { hreflang: 'x-default', path: '/en/docs' },
    ],
  },
  '/download/android': {
    title: 'JisudengChat Android 下载 - 极速蹬',
    description: '下载 JisudengChat Android APK，使用极速蹬平台账号登录并同步余额、分组、API Key 和可用模型。',
    canonicalPath: '/download/android',
    lang: 'zh-CN',
    ogLocale: 'zh_CN',
    alternates: [
      { hreflang: 'zh-CN', path: '/download/android' },
      { hreflang: 'x-default', path: '/download/android' },
    ],
  },
  '/en': {
    title: 'Jisudeng: One OpenAI-Compatible API for Frontier AI Models',
    description: 'Access DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through one OpenAI-compatible API.',
    canonicalPath: '/en/',
    lang: 'en',
    ogLocale: 'en_US',
    alternates: [
      { hreflang: 'en', path: '/en/' },
      { hreflang: 'zh-CN', path: '/' },
      { hreflang: 'x-default', path: '/en/' },
    ],
  },
  '/en/models': {
    title: 'AI Models API: DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini | Jisudeng',
    description: 'Compare model access and usage-based API rates for DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through Jisudeng.',
    canonicalPath: '/en/models',
    lang: 'en',
    ogLocale: 'en_US',
    alternates: [
      { hreflang: 'en', path: '/en/models' },
      { hreflang: 'zh-CN', path: '/models' },
      { hreflang: 'x-default', path: '/en/models' },
    ],
  },
  '/en/docs': {
    title: 'Jisudeng API Docs - OpenAI-Compatible Gateway Quickstart',
    description: 'Use Jisudeng with your existing OpenAI SDK. Change only your base URL and API key to access multiple frontier AI models.',
    canonicalPath: '/en/docs',
    lang: 'en',
    ogLocale: 'en_US',
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
  setMeta('meta[property="og:title"]', 'property', 'og:title', seo.title)
  setMeta('meta[property="og:description"]', 'property', 'og:description', seo.description)
  setMeta('meta[property="og:url"]', 'property', 'og:url', canonical)
  setMeta('meta[property="og:locale"]', 'property', 'og:locale', seo.ogLocale)
  setMeta('meta[name="twitter:title"]', 'name', 'twitter:title', seo.title)
  setMeta('meta[name="twitter:description"]', 'name', 'twitter:description', seo.description)
  setCanonical(canonical)
  setAlternates(seo)
  return seo
}
