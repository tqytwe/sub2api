import { describe, expect, it } from 'vitest'

import { resolvePublicLocaleRoute } from '../publicLocaleRoute'

describe('resolvePublicLocaleRoute', () => {
  it.each([
    ['/', '/en'],
    ['/home', '/en'],
    ['/login', '/en'],
    ['/register', '/en'],
    ['/about', '/en/about'],
    ['/contact', '/en/contact'],
  ])('sends Chinese public route %s to the English landing route', (from, to) => {
    expect(resolvePublicLocaleRoute('en', from)).toEqual({ path: to })
  })

  it('keeps catalog and docs on their matched English public routes', () => {
    expect(resolvePublicLocaleRoute('en', '/pricing')).toEqual({ path: '/en/catalog' })
    expect(resolvePublicLocaleRoute('en', '/pricing/deepseek', { sort: 'price' })).toEqual({
      path: '/en/catalog/deepseek',
      query: { sort: 'price' },
    })
    expect(resolvePublicLocaleRoute('en', '/catalog')).toEqual({ path: '/en/catalog' })
    expect(resolvePublicLocaleRoute('en', '/catalog/deepseek', { sort: 'price' })).toEqual({
      path: '/en/catalog/deepseek',
      query: { sort: 'price' },
    })
    expect(resolvePublicLocaleRoute('en', '/docs', { cat: 'tutorial', page: 'quick-start' })).toEqual({
      path: '/en/docs',
      query: { cat: 'tutorial', page: 'quick-start' },
    })
    expect(resolvePublicLocaleRoute('en', '/status')).toEqual({ path: '/en/status' })
  })

  it('sends English public routes back to canonical Chinese routes', () => {
    expect(resolvePublicLocaleRoute('zh', '/en')).toEqual({ path: '/' })
    expect(resolvePublicLocaleRoute('zh', '/en/catalog')).toEqual({ path: '/catalog' })
    expect(resolvePublicLocaleRoute('zh', '/en/catalog/deepseek', { sort: 'price' })).toEqual({
      path: '/catalog/deepseek',
      query: { sort: 'price' },
    })
    expect(resolvePublicLocaleRoute('zh', '/en/docs', { cat: 'tutorial' })).toEqual({
      path: '/docs',
      query: { cat: 'tutorial' },
    })
    expect(resolvePublicLocaleRoute('zh', '/en/status')).toEqual({ path: '/status' })
    expect(resolvePublicLocaleRoute('zh', '/en/about')).toEqual({ path: '/about' })
    expect(resolvePublicLocaleRoute('zh', '/en/contact')).toEqual({ path: '/contact' })
  })

  it('removes explicit locale queries when switching to a canonical public route', () => {
    expect(resolvePublicLocaleRoute('en', '/catalog', { lang: 'zh', sort: 'price' })).toEqual({
      path: '/en/catalog',
      query: { sort: 'price' },
    })
    expect(resolvePublicLocaleRoute('zh', '/en/catalog', { lang: 'en', locale: 'en', sort: 'price' })).toEqual({
      path: '/catalog',
      query: { sort: 'price' },
    })
    expect(resolvePublicLocaleRoute('zh', '/catalog', { lang: 'en', page: '2' })).toEqual({
      path: '/catalog',
      query: { page: '2' },
    })
  })

  it('does not invent English paths for routes already in the requested public locale', () => {
    expect(resolvePublicLocaleRoute('en', '/en/docs')).toBeNull()
    expect(resolvePublicLocaleRoute('zh', '/home')).toBeNull()
  })
})
