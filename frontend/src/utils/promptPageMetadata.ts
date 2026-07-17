export interface PromptPageMetadata {
  title: string
  description: string
  path: string
  image?: string
  kind: 'square' | 'detail'
  publishedAt?: string
}

interface PromptPageMetadataSnapshot {
  title: string
  metas: Array<{
    selector: string
    existed: boolean
    content: string
  }>
  canonical: {
    existed: boolean
    href: string
  }
}

const metaSelectors = [
  'meta[name="description"]',
  'meta[property="og:title"]',
  'meta[property="og:description"]',
  'meta[property="og:site_name"]',
  'meta[property="og:type"]',
  'meta[property="og:url"]',
  'meta[property="og:image"]',
]

let promptPageSnapshot: PromptPageMetadataSnapshot | null = null

function capturePromptPageSnapshot() {
  if (
    promptPageSnapshot
    && !document.head.querySelector('#jisudeng-prompt-structured-data')
    && !document.head.querySelector('[data-jisudeng-prompt="true"]')
  ) {
    promptPageSnapshot = null
  }
  if (promptPageSnapshot) return
  const canonical = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  promptPageSnapshot = {
    title: document.title,
    metas: metaSelectors.map((selector) => {
      const element = document.head.querySelector<HTMLMetaElement>(selector)
      return {
        selector,
        existed: !!element,
        content: element?.content ?? '',
      }
    }),
    canonical: {
      existed: !!canonical,
      href: canonical?.href ?? '',
    },
  }
}

function setMeta(selector: string, attribute: 'name' | 'property', key: string, content: string) {
  let element = document.head.querySelector<HTMLMetaElement>(selector)
  if (!element) {
    element = document.createElement('meta')
    element.setAttribute(attribute, key)
    element.dataset.jisudengPrompt = 'true'
    document.head.appendChild(element)
  }
  element.content = content
}

export function applyPromptPageMetadata(metadata: PromptPageMetadata): void {
  capturePromptPageSnapshot()
  const title = `${metadata.title} - 极速蹬提示词库`
  const canonical = new URL(metadata.path, window.location.origin).toString()
  document.title = title

  setMeta('meta[name="description"]', 'name', 'description', metadata.description)
  setMeta('meta[property="og:title"]', 'property', 'og:title', title)
  setMeta('meta[property="og:description"]', 'property', 'og:description', metadata.description)
  setMeta('meta[property="og:site_name"]', 'property', 'og:site_name', '极速蹬提示词库')
  setMeta('meta[property="og:type"]', 'property', 'og:type', metadata.kind === 'detail' ? 'article' : 'website')
  setMeta('meta[property="og:url"]', 'property', 'og:url', canonical)
  if (metadata.image) {
    setMeta('meta[property="og:image"]', 'property', 'og:image', metadata.image)
  } else {
    document.head.querySelector('meta[property="og:image"][data-jisudeng-prompt="true"]')?.remove()
  }

  let link = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'canonical'
    link.dataset.jisudengPrompt = 'true'
    document.head.appendChild(link)
  }
  link.href = canonical

  let structured = document.head.querySelector<HTMLScriptElement>(
    '#jisudeng-prompt-structured-data',
  )
  if (!structured) {
    structured = document.createElement('script')
    structured.id = 'jisudeng-prompt-structured-data'
    structured.type = 'application/ld+json'
    document.head.appendChild(structured)
  }
  structured.textContent = JSON.stringify({
    '@context': 'https://schema.org',
    '@type': metadata.kind === 'detail' ? 'CreativeWork' : 'CollectionPage',
    name: metadata.title,
    description: metadata.description,
    url: canonical,
    image: metadata.image || undefined,
    datePublished: metadata.publishedAt || undefined,
    inLanguage: 'zh-CN',
    publisher: {
      '@type': 'Organization',
      name: '极速蹬',
    },
  })
}

export function clearPromptPageMetadata(): void {
  if (promptPageSnapshot) {
    document.title = promptPageSnapshot.title
    for (const item of promptPageSnapshot.metas) {
      const element = document.head.querySelector<HTMLMetaElement>(item.selector)
      if (item.existed) {
        if (element) element.content = item.content
      } else if (element?.dataset.jisudengPrompt === 'true') {
        element.remove()
      }
    }

    const canonical = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
    if (promptPageSnapshot.canonical.existed) {
      if (canonical) canonical.href = promptPageSnapshot.canonical.href
    } else if (canonical?.dataset.jisudengPrompt === 'true') {
      canonical.remove()
    }
    promptPageSnapshot = null
  }
  document.head.querySelectorAll('[data-jisudeng-prompt="true"]').forEach((element) => element.remove())
  document.head.querySelector('#jisudeng-prompt-structured-data')?.remove()
}
