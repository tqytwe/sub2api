import { describe, expect, it } from 'vitest'

import { applyPublicRouteSeo, resolvePublicRouteSeo } from '../routeSeo'

describe('public route SEO', () => {
  it('keeps public metadata within the crawler and social length budgets', () => {
    for (const path of ['/', '/home', '/models', '/docs', '/download/android', '/en', '/en/models', '/en/docs']) {
      const seo = resolvePublicRouteSeo(path)

      expect(seo, path).toBeTruthy()
      if (!seo) throw new Error(`missing public SEO metadata for ${path}`)
      const twitterTitle = seo.twitterTitle || seo.title
      const twitterDescription = seo.twitterDescription || seo.description
      expect(seo.title.length, `${path} title`).toBeGreaterThanOrEqual(30)
      expect(seo.title.length, `${path} title`).toBeLessThanOrEqual(60)
      expect(seo.description.length, `${path} description`).toBeGreaterThanOrEqual(120)
      expect(seo.description.length, `${path} description`).toBeLessThanOrEqual(160)
      expect(twitterTitle.length, `${path} twitter:title`).toBeGreaterThanOrEqual(50)
      expect(twitterTitle.length, `${path} twitter:title`).toBeLessThanOrEqual(70)
      expect(twitterDescription.length, `${path} twitter:description`).toBeGreaterThanOrEqual(150)
      expect(twitterDescription.length, `${path} twitter:description`).toBeLessThanOrEqual(200)
      expect(seo.keywords.trim(), `${path} keywords`).not.toBe('')
    }
  })

  it('resolves English brand metadata for the /en layer', () => {
    const home = resolvePublicRouteSeo('/en/')
    const models = resolvePublicRouteSeo('/en/models')
    const docs = resolvePublicRouteSeo('/en/docs')

    expect(home?.lang).toBe('en')
    expect(home?.title).toBe('Jisudeng: One OpenAI-Compatible API for Frontier AI Models')
    expect(home?.description).toContain('Access DeepSeek, Qwen, Kimi, GLM')
    expect(home?.description.length).toBeGreaterThanOrEqual(120)
    expect(home?.description.length).toBeLessThanOrEqual(160)
    expect(home?.twitterDescription?.length).toBeGreaterThanOrEqual(150)
    expect(home?.twitterDescription?.length).toBeLessThanOrEqual(200)
    expect(home?.keywords).toContain('OpenAI-compatible API')
    expect(models?.canonicalPath).toBe('/en/models')
    expect(models?.description).toContain('Claude, Gemini')
    expect(docs?.description).toContain('OpenAI SDK')
  })

  it('keeps Chinese metadata on Chinese public routes', () => {
    expect(resolvePublicRouteSeo('/')?.lang).toBe('zh-CN')
    expect(resolvePublicRouteSeo('/')?.title.length).toBeGreaterThanOrEqual(30)
    expect(resolvePublicRouteSeo('/')?.description.length).toBeGreaterThanOrEqual(120)
    expect(resolvePublicRouteSeo('/')?.twitterTitle?.length).toBeGreaterThanOrEqual(50)
    expect(resolvePublicRouteSeo('/')?.twitterDescription?.length).toBeGreaterThanOrEqual(150)
    expect(resolvePublicRouteSeo('/models')?.canonicalPath).toBe('/models')
    expect(resolvePublicRouteSeo('/docs')?.alternates.some((link) => link.path === '/en/docs')).toBe(true)
    expect(resolvePublicRouteSeo('/download/android')?.canonicalPath).toBe('/download/android')
  })

  it('applies title, lang, canonical, and hreflang tags in the browser head', () => {
    document.head.innerHTML = `
      <meta name="description" content="old" />
      <link rel="canonical" href="https://old.example/" />
    `

    const seo = applyPublicRouteSeo('/en/models')

    expect(seo?.lang).toBe('en')
    expect(document.documentElement.getAttribute('lang')).toBe('en')
    expect(document.title).toBe('DeepSeek, Qwen, Kimi, GLM, Claude API Pricing | Jisudeng')
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toContain('DeepSeek, Qwen')
    expect(document.head.querySelector('meta[name="keywords"]')?.getAttribute('content')).toContain('AI model API pricing')
    expect(document.head.querySelector('meta[name="author"]')?.getAttribute('content')).toBe('Jisudeng')
    expect(document.head.querySelector('meta[name="format-detection"]')?.getAttribute('content')).toBe('telephone=no,email=no,address=no')
    expect(document.head.querySelector('meta[property="og:site_name"]')?.getAttribute('content')).toBe('Jisudeng')
    expect(document.head.querySelector('meta[property="og:image"]')?.getAttribute('content')).toBe('https://www.jisudeng.com/logo.png')
    expect(document.head.querySelector('meta[name="twitter:site"]')?.getAttribute('content')).toBe('@jisudeng')
    expect(document.head.querySelector('meta[name="twitter:creator"]')?.getAttribute('content')).toBe('@jisudeng')
    expect(document.head.querySelector('meta[name="twitter:title"]')?.getAttribute('content')).toBe('DeepSeek, Qwen, Kimi, GLM, GPT, Claude API Pricing | Jisudeng')
    expect(document.head.querySelector('meta[name="twitter:image"]')?.getAttribute('content')).toBe('https://www.jisudeng.com/logo.png')
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://www.jisudeng.com/en/models')
    expect(Array.from(document.head.querySelectorAll('link[rel="alternate"][hreflang]')).map((link) => link.getAttribute('hreflang'))).toEqual(['en', 'zh-CN', 'x-default'])
    const structured = document.head.querySelector('script[type="application/ld+json"][data-jisudeng-route-seo="true"]')
    expect(structured).toBeTruthy()
    expect(JSON.parse(structured?.textContent || '{}')).toMatchObject({
      '@context': 'https://schema.org',
      '@type': 'CollectionPage',
      name: 'DeepSeek, Qwen, Kimi, GLM, Claude API Pricing | Jisudeng',
      headline: 'DeepSeek, Qwen, Kimi, GLM, Claude API Pricing | Jisudeng',
      inLanguage: 'en',
    })
  })

  it('uses the brand as the Website entity name and the title as headline', () => {
    document.head.innerHTML = ''

    applyPublicRouteSeo('/')

    const structured = document.head.querySelector('script[type="application/ld+json"][data-jisudeng-route-seo="true"]')
    expect(JSON.parse(structured?.textContent || '{}')).toMatchObject({
      '@type': 'WebSite',
      name: '极速蹬',
      headline: '极速蹬 - OpenAI兼容 AI API 网关与多模型服务平台',
      inLanguage: 'zh-CN',
    })
  })
})
