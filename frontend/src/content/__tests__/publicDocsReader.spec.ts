import { describe, expect, it } from 'vitest'
import { buildDocArticle, personalizeDocCopy, searchPublicDocs } from '../public-docs-reader'
import { DOC_TOOL_INTEGRATIONS } from '../public-docs-integrations'
import { normalizePublicDocLocation } from '../public-docs-tree'
import { PUBLIC_DOC_CONTENT_ZH } from '../public-docs-data.zh'
import { PUBLIC_DOC_CONTENT_EN } from '../public-docs-data.en'

describe('public document reader', () => {
  it('keeps existing anchors and creates unique headings, but skips headings inside examples', () => {
    const article = buildDocArticle(`
      <h2>前置要求</h2><h3>Install &amp; verify</h3>
      <h2 id="existing">第一步</h2><h2>前置要求</h2>
      <pre><code>&lt;h2&gt;literal&lt;/h2&gt;</code></pre>
    `, 'Copy code')
    expect(article.headings).toEqual([
      { id: 'docs-section-1', title: '前置要求', level: 2 },
      { id: 'docs-section-2', title: 'Install & verify', level: 3 },
      { id: 'existing', title: '第一步', level: 2 },
      { id: 'docs-section-4', title: '前置要求', level: 2 },
    ])
    const root = document.createElement('div')
    root.innerHTML = article.html
    expect(root.querySelector('[data-doc-copy="0"]')?.textContent).toBe('Copy code')
    expect(article.code).toEqual(['<h2>literal</h2>'])
  })

  it('removes unsafe markup and does not trust authored copy controls', () => {
    const article = buildDocArticle(`
      <script>alert(1)</script><img src="/test.png" onerror="alert(2)">
      <a href="javascript:alert(3)" data-doc-copy="999">test</a>
    `, '<img src=x onerror=alert(4)>')
    expect(article.html).not.toMatch(/<script|onerror=|javascript:|data-doc-copy/)
  })

  it('resolves the full API host placeholder without replacing lookalike domains', () => {
    expect(personalizeDocCopy('https://api.your-domain.com/v1 https://your-host/v1 https://your-host.com', 'Jisudeng', 'https://api.jisudeng.com/'))
      .toBe('https://api.jisudeng.com/v1 https://api.jisudeng.com/v1 https://your-host.com')
  })

  it('matches case-insensitive terms across the localized title and aliases', () => {
    const docs = [{ title: '编辑器配置', keywords: 'Cline VS Code' }, { title: 'Cline', keywords: 'terminal' }]
    expect(searchPublicDocs(docs, 'CLINE  vs')).toEqual([docs[0]])
    expect(searchPublicDocs(docs, '')).toEqual(docs)
  })

  it('keeps English article links in the English public pages or shared console', () => {
    const article = buildDocArticle('<a href="/keys">Keys</a><a href="/check-in">Check-in</a><a href="/redeem">Redeem</a><a href="/catalog">Models</a><a href="/docs?cat=tools&amp;page=codex">Codex</a><a href="https://example.com/keys">External</a>', 'Copy code', 'en')
    const root = document.createElement('div')
    root.innerHTML = article.html
    expect([...root.querySelectorAll('a')].map(link => link.getAttribute('href'))).toEqual([
      '/keys?lang=en', '/check-in?lang=en', '/redeem?lang=en', '/en/catalog', '/en/docs?cat=tools&page=codex', 'https://example.com/keys',
    ])
  })

  it('has an authored English body for every article in the tree', () => {
    for (const category of PUBLIC_DOC_CONTENT_EN) {
      for (const page of category.pages) {
        expect(page.html, `${category.id}/${page.id}`).toMatch(/<h2[ >]/)
        expect(page.html, `${category.id}/${page.id}`).not.toContain('Open this article in English')
      }
    }
  })

  it('only lists integrations with a reachable article in both locales', () => {
    const keys = DOC_TOOL_INTEGRATIONS.map(({ cat, page }) => `${cat}/${page}`)
    expect(new Set(keys).size).toBe(keys.length)
    for (const { cat, page, name } of DOC_TOOL_INTEGRATIONS) {
      expect(name).not.toBe('')
      for (const docs of [PUBLIC_DOC_CONTENT_ZH, PUBLIC_DOC_CONTENT_EN]) {
        expect(docs.find((category) => category.id === cat)?.pages.some((article) => article.id === page), `${cat}/${page}`).toBe(true)
      }
    }
  })

  it('uses the tools category as the single tool entry point', () => {
    expect(DOC_TOOL_INTEGRATIONS.every(({ cat }) => cat === 'tools')).toBe(true)
    for (const docs of [PUBLIC_DOC_CONTENT_ZH, PUBLIC_DOC_CONTENT_EN]) {
      expect(docs.find((category) => category.id === 'deploy')?.pages.some((page) => ['claude-code', 'codex', 'gemini-cli'].includes(page.id))).toBe(false)
      expect(docs.find((category) => category.id === 'tools')?.pages.map((page) => page.id).slice(0, 3)).toEqual(['claude-code', 'codex', 'gemini-cli'])
    }
    for (const pageId of ['claude-code', 'codex', 'gemini-cli']) {
      expect(normalizePublicDocLocation('deploy', pageId)).toEqual({ catId: 'tools', pageId })
    }
  })
})
