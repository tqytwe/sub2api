import { beforeEach, describe, expect, it } from 'vitest'
import { updateFavicon } from '@/utils/branding'

describe('updateFavicon', () => {
  beforeEach(() => {
    document.head.innerHTML = '<link rel="icon" href="/logo.png">'
  })

  it('replaces the default favicon with the configured logo', () => {
    updateFavicon('https://example.com/custom-logo.png')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.href).toBe('https://example.com/custom-logo.png')
    expect(link?.type).toBe('image/png')
  })

  it('uses the DENG PNG fallback for unsafe logo URLs', () => {
    updateFavicon('javascript:alert(1)')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.getAttribute('href')).toBe('/logo.png')
    expect(link?.type).toBe('image/png')
  })

  it('uses the DENG PNG fallback when no configured logo is available', () => {
    updateFavicon('')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.getAttribute('href')).toBe('/logo.png')
    expect(link?.type).toBe('image/png')
  })
})
