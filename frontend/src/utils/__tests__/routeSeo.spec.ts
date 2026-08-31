import { describe, expect, it } from 'vitest'

import routeSeoContract from '../public-route-seo-contract.json'
import { applyPublicRouteSeo, resolvePublicRouteSeo } from '../routeSeo'

function jsonLdTypes(value: unknown, types = new Set<string>()): string[] {
  if (Array.isArray(value)) {
    for (const item of value) jsonLdTypes(item, types)
    return Array.from(types).sort()
  }
  if (!value || typeof value !== 'object') return Array.from(types).sort()

  const record = value as Record<string, unknown>
  const type = record['@type']
  if (typeof type === 'string') types.add(type)
  for (const item of Object.values(record)) jsonLdTypes(item, types)
  return Array.from(types).sort()
}

describe('public route SEO', () => {
	it('keeps hydration metadata aligned with the cross-layer public route contract', () => {
		document.head.innerHTML = ''

		for (const [path, expected] of Object.entries(routeSeoContract.routes)) {
			const seo = resolvePublicRouteSeo(path)
			expect(seo, path).toBeTruthy()
			if (!seo) throw new Error(`missing public SEO metadata for ${path}`)

			expect(seo.title, `${path} title`).toBe(expected.title)
			expect(seo.description, `${path} description`).toBe(expected.description)
			expect(seo.twitterTitle || seo.title, `${path} twitter:title`).toBe(expected.twitterTitle)
			expect(seo.twitterDescription || seo.description, `${path} twitter:description`).toBe(expected.twitterDescription)
			expect(seo.canonicalPath, `${path} canonical`).toBe(expected.canonicalPath)
			expect(seo.alternates, `${path} hreflang`).toEqual(expected.alternates)

			applyPublicRouteSeo(path)
			expect(document.title, `${path} hydrated title`).toBe(expected.title)
			expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content'), `${path} hydrated description`).toBe(expected.description)
			expect(document.head.querySelector('meta[name="twitter:title"]')?.getAttribute('content'), `${path} hydrated twitter:title`).toBe(expected.twitterTitle)
			expect(document.head.querySelector('meta[name="twitter:description"]')?.getAttribute('content'), `${path} hydrated twitter:description`).toBe(expected.twitterDescription)
			expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href'), `${path} hydrated canonical`).toBe(`https://www.jisudeng.com${expected.canonicalPath}`)
			expect(
			Array.from(document.head.querySelectorAll<HTMLLinkElement>('link[rel="alternate"][hreflang]')).map((link) => ({
				hreflang: link.hreflang,
				path: new URL(link.href).pathname,
			})),
			`${path} hydrated hreflang`,
		).toEqual(expected.alternates.map((alternate) => ({
			...alternate,
			path: new URL(alternate.path, 'https://www.jisudeng.com').pathname,
		})))

			const structured = document.head.querySelector<HTMLScriptElement>('script[type="application/ld+json"][data-jisudeng-route-seo="true"]')
			const data = structured?.textContent ? JSON.parse(structured.textContent) : undefined
			expect(jsonLdTypes(data), `${path} JSON-LD types`).toEqual([...expected.jsonLdTypes].sort())
		}
	})

  it('keeps public metadata within the crawler and social length budgets', () => {
    for (const path of ['/', '/catalog', '/catalog/deepseek', '/catalog/qwen', '/catalog/kimi', '/catalog/glm', '/docs', '/download/android', '/about', '/contact', '/en', '/en/catalog', '/en/catalog/deepseek', '/en/catalog/qwen', '/en/catalog/kimi', '/en/catalog/glm', '/en/docs', '/en/about', '/en/contact']) {
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
    const models = resolvePublicRouteSeo('/en/catalog')
    const docs = resolvePublicRouteSeo('/en/docs')

    expect(home?.lang).toBe('en')
    expect(home?.title).toBe('Jisudeng: One OpenAI-Compatible API for Frontier AI Models')
    expect(home?.description).toContain('Access DeepSeek, Qwen, Kimi, GLM')
    expect(home?.description.length).toBeGreaterThanOrEqual(120)
    expect(home?.description.length).toBeLessThanOrEqual(160)
    expect(home?.twitterDescription?.length).toBeGreaterThanOrEqual(150)
    expect(home?.twitterDescription?.length).toBeLessThanOrEqual(200)
    expect(home?.keywords).toContain('OpenAI-compatible API')
    expect(models?.canonicalPath).toBe('/en/catalog')
    expect(models?.description).toContain('Claude, Gemini')
    expect(docs?.description).toContain('OpenAI SDK')
  })

  it('keeps every public canonical and x-default link on the catalog-era paths', () => {
    for (const path of ['/', '/catalog', '/catalog/deepseek', '/catalog/qwen', '/catalog/kimi', '/catalog/glm', '/docs', '/download/android', '/about', '/contact', '/en', '/en/catalog', '/en/catalog/deepseek', '/en/catalog/qwen', '/en/catalog/kimi', '/en/catalog/glm', '/en/docs', '/en/about', '/en/contact']) {
      const seo = resolvePublicRouteSeo(path)
      expect(seo, path).toBeTruthy()
      expect(seo?.canonicalPath).not.toContain('/models')
      const zh = seo?.alternates.find((link) => link.hreflang === 'zh-CN')
      const fallback = seo?.alternates.find((link) => link.hreflang === 'x-default')
      expect(fallback?.path, path).toBe(zh?.path)
      expect(seo?.alternates.some((link) => link.path.includes('/models'))).toBe(false)
    }
  })

  it('keeps Chinese metadata on Chinese public routes', () => {
    expect(resolvePublicRouteSeo('/')?.lang).toBe('zh-CN')
    expect(resolvePublicRouteSeo('/')?.title.length).toBeGreaterThanOrEqual(30)
    expect(resolvePublicRouteSeo('/')?.description.length).toBeGreaterThanOrEqual(120)
    expect(resolvePublicRouteSeo('/')?.twitterTitle?.length).toBeGreaterThanOrEqual(50)
    expect(resolvePublicRouteSeo('/')?.twitterDescription?.length).toBeGreaterThanOrEqual(150)
    expect(resolvePublicRouteSeo('/pricing')).toBeUndefined()
    expect(resolvePublicRouteSeo('/pricing/deepseek')).toBeUndefined()
    expect(resolvePublicRouteSeo('/models')).toBeUndefined()
    expect(resolvePublicRouteSeo('/catalog')?.canonicalPath).toBe('/catalog')
    expect(resolvePublicRouteSeo('/catalog')?.alternates).toEqual([
      { hreflang: 'zh-CN', path: '/catalog' },
      { hreflang: 'en', path: '/en/catalog' },
      { hreflang: 'x-default', path: '/catalog' },
    ])
    expect(resolvePublicRouteSeo('/docs')?.alternates.some((link) => link.path === '/en/docs')).toBe(true)
    expect(resolvePublicRouteSeo('/download/android')?.canonicalPath).toBe('/download/android')
    expect(resolvePublicRouteSeo('/about')?.canonicalPath).toBe('/about')
    expect(resolvePublicRouteSeo('/about')?.alternates).toEqual([
      { hreflang: 'zh-CN', path: '/about' },
      { hreflang: 'en', path: '/en/about' },
      { hreflang: 'x-default', path: '/about' },
    ])
    expect(resolvePublicRouteSeo('/contact')?.structuredType).toBe('ContactPage')
    expect(resolvePublicRouteSeo('/en/about')?.lang).toBe('en')
    expect(resolvePublicRouteSeo('/en/contact')?.canonicalPath).toBe('/en/contact')
  })

  it('applies title, lang, canonical, and hreflang tags in the browser head', () => {
    document.head.innerHTML = `
      <meta name="description" content="old" />
      <link rel="canonical" href="https://old.example/" />
    `

    const seo = applyPublicRouteSeo('/en/catalog')

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
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://www.jisudeng.com/en/catalog')
    expect(Array.from(document.head.querySelectorAll('link[rel="alternate"][hreflang]')).map((link) => link.getAttribute('hreflang'))).toEqual(['en', 'zh-CN', 'x-default'])
    const structured = document.head.querySelector('script[type="application/ld+json"][data-jisudeng-route-seo="true"]')
    expect(structured).toBeTruthy()
    expect(JSON.parse(structured?.textContent || '{}')).toMatchObject({
      '@context': 'https://schema.org',
      '@type': 'ItemList',
      name: 'Jisudeng public model families',
      inLanguage: 'en',
    })
    expect(JSON.parse(structured?.textContent || '{}').itemListElement).toEqual(expect.arrayContaining([
      expect.objectContaining({
        '@type': 'ListItem',
        name: 'DeepSeek API',
        url: 'https://www.jisudeng.com/en/catalog/deepseek',
      }),
    ]))
  })

  it('removes indexable route metadata from private SPA routes', () => {
    document.head.innerHTML = ''

    applyPublicRouteSeo('/catalog')
    expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('index,follow')
    expect(document.head.querySelector('link[rel="canonical"]')).toBeTruthy()

    for (const path of ['/status', '/en/status', '/login', '/subscriptions', '/monitor', '/ai-creation-space']) {
      expect(applyPublicRouteSeo(path), path).toBeUndefined()
      expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex,nofollow')
      expect(document.head.querySelector('link[rel="canonical"]')).toBeNull()
      expect(document.head.querySelector('link[rel="alternate"][hreflang]')).toBeNull()
      expect(document.head.querySelector('script[data-jisudeng-route-seo="true"]')).toBeNull()
    }
  })

  it('uses only the visible home organization and web application entities', () => {
    document.head.innerHTML = ''

    applyPublicRouteSeo('/')

    const structured = document.head.querySelector('script[type="application/ld+json"][data-jisudeng-route-seo="true"]')
    const data = JSON.parse(structured?.textContent || '{}')
    expect(data).toMatchObject({ '@context': 'https://schema.org' })
    expect(data['@graph']).toEqual(expect.arrayContaining([
      expect.objectContaining({ '@type': 'Organization', name: '极速蹬' }),
      expect.objectContaining({ '@type': 'WebApplication', name: 'Jisudeng', inLanguage: 'zh-CN' }),
    ]))
    expect(JSON.stringify(data)).not.toContain('WebSite')
    expect(JSON.stringify(data)).not.toContain('SoftwareApplication')
  })

  it('does not emit schema entities for public routes without matching visible content', () => {
    document.head.innerHTML = ''

    applyPublicRouteSeo('/about')

    expect(document.head.querySelector('script[type="application/ld+json"][data-jisudeng-route-seo="true"]')).toBeNull()
  })
})
