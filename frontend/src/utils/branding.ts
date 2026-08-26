import { sanitizeUrl } from '@/utils/url'

const DEFAULT_FAVICON = '/logo.png'

function faviconType(url: string): string {
  const pathname = url.split(/[?#]/, 1)[0].toLowerCase()
  if (pathname.endsWith('.svg')) return 'image/svg+xml'
  if (pathname.endsWith('.png')) return 'image/png'
  return 'image/x-icon'
}

export function updateFavicon(logoUrl?: string): void {
  const sanitizedLogoUrl = sanitizeUrl(logoUrl || '', {
    allowRelative: true,
    allowDataUrl: true,
  })
  const faviconUrl = sanitizedLogoUrl || DEFAULT_FAVICON

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  link.type = faviconType(faviconUrl)
  link.href = faviconUrl
}
