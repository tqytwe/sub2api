import { describe, expect, it } from 'vitest'

import { applyPublicRouteSeo, resolvePublicRouteSeo } from '../routeSeo'

describe('public route SEO', () => {
  it('resolves English brand metadata for the /en layer', () => {
    const home = resolvePublicRouteSeo('/en/')
    const models = resolvePublicRouteSeo('/en/models')
    const docs = resolvePublicRouteSeo('/en/docs')

    expect(home?.lang).toBe('en')
    expect(home?.title).toBe('Jisudeng: One OpenAI-Compatible API for Frontier AI Models')
    expect(home?.description).toContain('Access DeepSeek, Qwen, Kimi, GLM')
    expect(models?.canonicalPath).toBe('/en/models')
    expect(models?.description).toContain('Claude, Gemini')
    expect(docs?.description).toContain('OpenAI SDK')
  })

  it('keeps Chinese metadata on Chinese public routes', () => {
    expect(resolvePublicRouteSeo('/')?.lang).toBe('zh-CN')
    expect(resolvePublicRouteSeo('/models')?.canonicalPath).toBe('/models')
    expect(resolvePublicRouteSeo('/docs')?.alternates.some((link) => link.path === '/en/docs')).toBe(true)
  })

  it('applies title, lang, canonical, and hreflang tags in the browser head', () => {
    document.head.innerHTML = `
      <meta name="description" content="old" />
      <link rel="canonical" href="https://old.example/" />
    `

    const seo = applyPublicRouteSeo('/en/models')

    expect(seo?.lang).toBe('en')
    expect(document.documentElement.getAttribute('lang')).toBe('en')
    expect(document.title).toBe('AI Models API: DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini | Jisudeng')
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toContain('DeepSeek, Qwen')
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://www.jisudeng.com/en/models')
    expect(Array.from(document.head.querySelectorAll('link[rel="alternate"][hreflang]')).map((link) => link.getAttribute('hreflang'))).toEqual(['en', 'zh-CN', 'x-default'])
  })
})
