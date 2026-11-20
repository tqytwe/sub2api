import DOMPurify from 'dompurify'

export interface DocHeading {
  id: string
  title: string
  level: 2 | 3
}

export function personalizeDocCopy(raw: string, siteName: string, apiBase: string) {
  const base = apiBase.replace(/\/+$/, '')
  return raw
    .replace(/随想 AI|随想|本站/g, () => siteName)
    .replace(/https:\/\/(?:sui-xiang\.com|api\.your-domain\.com|api\.jisudeng\.com|your-host)(?![a-z0-9.-])/gi, () => base)
}

const englishPages = new Map([['/', '/en'], ['/docs', '/en/docs'], ['/catalog', '/en/catalog'], ['/contact', '/en/contact'], ['/status', '/en/status']])
const sharedWorkspacePaths = new Set([
  '/keys', '/usage', '/play', '/wallet', '/purchase', '/orders', '/login', '/register',
  '/ai-creation-space', '/batch-image', '/docs/batch-image', '/blindbox', '/arena', '/check-in', '/redeem',
])

export function searchPublicDocs<T extends { title: string; keywords: string }>(docs: T[], query: string) {
  const terms = query.trim().normalize('NFKC').toLowerCase().split(/\s+/).filter(Boolean)
  return docs.filter(({ title, keywords }) => {
    const text = `${title} ${keywords}`.normalize('NFKC').toLowerCase()
    return terms.every((term) => text.includes(term))
  })
}

export function buildDocArticle(source: string, copyLabel: string, locale: 'zh' | 'en' = 'zh') {
  const root = document.createElement('div')
  root.append(DOMPurify.sanitize(source, {
    RETURN_DOM_FRAGMENT: true,
    FORBID_ATTR: ['data-doc-copy'],
  }))

  root.querySelectorAll<HTMLAnchorElement>('a[href]').forEach((link) => {
    const href = link.getAttribute('href') ?? ''
    if (!href.startsWith('/') || href.startsWith('//') || href.includes('\\')) return
    const url = new URL(href, 'https://www.jisudeng.com')
    if (locale === 'en') {
      url.pathname = englishPages.get(url.pathname) ?? url.pathname
      if (sharedWorkspacePaths.has(url.pathname)) url.searchParams.set('lang', 'en')
    }
    link.setAttribute('href', url.pathname + url.search + url.hash)
  })

  const seen = new Set<string>()
  const reserved = new Set(Array.from(root.querySelectorAll('[id]'), (el) => el.id))
  const headings: DocHeading[] = []
  root.querySelectorAll<HTMLHeadingElement>('h2, h3').forEach((heading, index) => {
    const title = heading.textContent?.trim()
    if (!title) return
    if (!heading.id || seen.has(heading.id)) {
      let suffix = index + 1
      while (reserved.has(`docs-section-${suffix}`)) suffix++
      heading.id = `docs-section-${suffix}`
      reserved.add(heading.id)
    }
    seen.add(heading.id)
    headings.push({ id: heading.id, title, level: heading.tagName === 'H2' ? 2 : 3 })
  })

  const code: string[] = []
  root.querySelectorAll('pre > code').forEach((snippet) => {
    const text = snippet.textContent
    const pre = snippet.parentElement
    if (!pre || !text?.trim()) return
    const wrapper = document.createElement('div')
    const toolbar = document.createElement('div')
    const button = document.createElement('button')
    wrapper.className = 'docs-code-block'
    toolbar.className = 'docs-code-toolbar'
    button.type = 'button'
    button.dataset.docCopy = String(code.length)
    button.textContent = copyLabel
    code.push(text)
    toolbar.append(button)
    pre.before(wrapper)
    wrapper.append(toolbar, pre)
  })

  return { html: root.innerHTML, headings, code }
}
