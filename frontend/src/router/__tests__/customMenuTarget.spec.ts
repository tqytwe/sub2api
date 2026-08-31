import { describe, expect, it } from 'vitest'

import {
  CANONICAL_APPLICATION_ORIGIN,
  LEGACY_DOC_CUSTOM_PAGE_ID,
  isFirstPartyDocsTarget,
  resolveCustomMenuRoute,
  resolveCustomMenuRouteById,
  resolveLegacyDocsCustomPageRoute,
} from '../customMenuTarget'

describe('custom menu native document targets', () => {
  it('routes a canonical same-origin docs menu to the Chinese native docs route without query parameters', () => {
    expect(resolveCustomMenuRoute(
      { url: 'https://www.jisudeng.com/docs?token=must-not-survive', label: '使用文档' },
      'zh',
    )).toEqual('/docs')
  })

  it('routes the same canonical docs menu to English for an English session', () => {
    expect(resolveCustomMenuRoute(
      { url: 'https://www.jisudeng.com/en/docs', label: 'Docs' },
      'en',
    )).toEqual('/en/docs')
  })

  it('uses the fixed canonical origin even when the runtime page is served from an alias host', () => {
    expect(CANONICAL_APPLICATION_ORIGIN).toBe('https://www.jisudeng.com')
    expect(resolveCustomMenuRoute({ url: 'https://www.jisudeng.com/docs' }, 'zh')).toBe('/docs')
    expect(resolveCustomMenuRoute({ url: 'https://jisudeng.com/docs' }, 'zh')).toBeNull()
  })

  it.each([
    'http://www.jisudeng.com/docs',
    'https://jisudeng.com/docs',
    'https://www.jisudeng.com/docs/other',
    'https://docs.example.com/docs',
    'https://www.jisudeng.com.evil.example/docs',
    'https://user:pass@www.jisudeng.com/docs',
  ])('keeps non-canonical custom URLs in the external custom-page flow: %s', (url) => {
    expect(resolveCustomMenuRoute({ url, label: 'Docs' }, 'zh')).toBeNull()
  })

  it.each([
    'https://www.jisudeng.com/docs',
    'http://www.jisudeng.com/docs',
    'https://jisudeng.com/en/docs',
  ])('recognizes first-party docs aliases for iframe context stripping: %s', (url) => {
    expect(isFirstPartyDocsTarget({ url })).toBe(true)
  })

  it.each([
    'https://www.jisudeng.com/docs/child',
    'https://docs.example.com/docs',
    'https://www.jisudeng.com.evil.example/docs',
  ])('does not classify unrelated targets as first-party docs: %s', (url) => {
    expect(isFirstPartyDocsTarget({ url })).toBe(false)
  })

  it('redirects the published legacy docs bookmark without consulting menu configuration', () => {
    expect(resolveLegacyDocsCustomPageRoute(LEGACY_DOC_CUSTOM_PAGE_ID, 'zh')).toBe('/docs')
    expect(resolveLegacyDocsCustomPageRoute(LEGACY_DOC_CUSTOM_PAGE_ID, 'en')).toBe('/en/docs')
    expect(resolveLegacyDocsCustomPageRoute('other-custom-page', 'zh')).toBeNull()
  })

  it('resolves canonical docs from an admin-only menu collection by id', () => {
    expect(resolveCustomMenuRouteById([
      { id: 'admin-docs', url: 'https://www.jisudeng.com/docs', label: 'Admin docs' },
    ], 'admin-docs', 'en')).toBe('/en/docs')
  })
})
