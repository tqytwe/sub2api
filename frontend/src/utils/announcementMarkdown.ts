import { marked } from 'marked'
import DOMPurify from 'dompurify'

const ALERT_TONES = new Set(['info', 'success', 'warning', 'danger', 'note'])
const INLINE_TONES = new Set(['info', 'success', 'warning', 'danger', 'muted'])
const ANNOUNCEMENT_ASSET_PREFIX = '/api/v1/announcement-assets/'

marked.setOptions({
  breaks: true,
  gfm: true,
})

export function renderAnnouncementMarkdown(content: string): string {
  if (!content?.trim()) return ''
  const html = marked.parse(preprocessAnnouncementMarkdown(content)) as string
  const sanitized = DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      'a',
      'blockquote',
      'br',
      'code',
      'em',
      'h1',
      'h2',
      'h3',
      'h4',
      'hr',
      'img',
      'li',
      'mark',
      'ol',
      'p',
      'pre',
      'span',
      'strong',
      'table',
      'tbody',
      'td',
      'th',
      'thead',
      'tr',
      'ul',
    ],
    ALLOWED_ATTR: ['alt', 'data-announcement-alert', 'data-announcement-tone', 'href', 'rel', 'src', 'target', 'title'],
    FORBID_ATTR: ['class', 'style'],
    FORBID_TAGS: ['button', 'embed', 'form', 'iframe', 'input', 'object', 'script', 'style'],
  })

  if (typeof document === 'undefined') return sanitized

  const template = document.createElement('template')
  template.innerHTML = sanitized
  normalizeAnnouncementLinks(template.content)
  normalizeAnnouncementImages(template.content)
  normalizeAnnouncementAlerts(template.content)
  return template.innerHTML
}

export function isSafeAnnouncementImageUrl(raw: string): boolean {
  const value = raw.trim()
  if (!value || value.startsWith('data:') || value.startsWith('//')) return false
  if (value.startsWith(ANNOUNCEMENT_ASSET_PREFIX)) return true
  try {
    const parsed = new URL(value, window.location.origin)
    return parsed.origin === window.location.origin && parsed.pathname.startsWith(ANNOUNCEMENT_ASSET_PREFIX)
  } catch {
    return false
  }
}

function preprocessAnnouncementMarkdown(content: string): string {
  return content
    .replace(/==([^=\n][^=\n]*?)==/g, (_match, text: string) => `<mark>${escapeHTML(text)}</mark>`)
    .replace(/::(info|success|warning|danger|muted)\[([^\]\n]+)\]/gi, (_match, tone: string, text: string) => {
      const normalizedTone = tone.toLowerCase()
      if (!INLINE_TONES.has(normalizedTone)) return escapeHTML(text)
      return `<span data-announcement-tone="${normalizedTone}">${escapeHTML(text)}</span>`
    })
}

function normalizeAnnouncementLinks(root: DocumentFragment): void {
  root.querySelectorAll('a[href]').forEach((link) => {
    const href = link.getAttribute('href')?.trim() || ''
    if (!isSafeAnnouncementLink(href)) {
      link.removeAttribute('href')
      return
    }
    link.setAttribute('target', '_blank')
    link.setAttribute('rel', 'noopener noreferrer')
  })
}

function normalizeAnnouncementImages(root: DocumentFragment): void {
  root.querySelectorAll('img').forEach((img) => {
    const src = img.getAttribute('src')?.trim() || ''
    if (!isSafeAnnouncementImageUrl(src)) {
      img.replaceWith(document.createTextNode(img.getAttribute('alt') || ''))
      return
    }
    img.setAttribute('loading', 'lazy')
    img.removeAttribute('srcset')
  })
}

function normalizeAnnouncementAlerts(root: DocumentFragment): void {
  root.querySelectorAll('blockquote').forEach((blockquote) => {
    const firstParagraph = blockquote.querySelector('p')
    if (!firstParagraph) return
    const marker = firstParagraph.textContent?.trim().match(/^\[!(INFO|SUCCESS|WARNING|DANGER|NOTE)]/i)
    if (!marker) return
    const tone = marker[1].toLowerCase()
    if (!ALERT_TONES.has(tone)) return
    blockquote.setAttribute('data-announcement-alert', tone === 'note' ? 'info' : tone)
    firstParagraph.innerHTML = firstParagraph.innerHTML.replace(/^\s*\[!(INFO|SUCCESS|WARNING|DANGER|NOTE)]\s*/i, '')
  })
}

function isSafeAnnouncementLink(raw: string): boolean {
  if (!raw || raw.startsWith('javascript:') || raw.startsWith('data:')) return false
  if (raw.startsWith('/') && !raw.startsWith('//')) return true
  try {
    const parsed = new URL(raw)
    return parsed.protocol === 'https:' || parsed.protocol === 'mailto:'
  } catch {
    return false
  }
}

function escapeHTML(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}
