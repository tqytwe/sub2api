import { describe, expect, it } from 'vitest'

import { resolvePublicLocaleRoute } from '../publicLocaleRoute'

describe('resolvePublicLocaleRoute', () => {
  it.each([
    ['/', '/en'],
    ['/home', '/en'],
    ['/login', '/en'],
    ['/register', '/en'],
    ['/about', '/en'],
    ['/contact', '/en'],
  ])('sends Chinese public route %s to the English landing route', (from, to) => {
    expect(resolvePublicLocaleRoute('en', from)).toEqual({ path: to })
  })

  it('keeps models and docs on their matched English public routes', () => {
    expect(resolvePublicLocaleRoute('en', '/models')).toEqual({ path: '/en/models' })
    expect(resolvePublicLocaleRoute('en', '/docs', { cat: 'tutorial', page: 'quick-start' })).toEqual({
      path: '/en/docs',
      query: { cat: 'tutorial', page: 'quick-start' },
    })
  })

  it('sends English public routes back to canonical Chinese routes', () => {
    expect(resolvePublicLocaleRoute('zh', '/en')).toEqual({ path: '/' })
    expect(resolvePublicLocaleRoute('zh', '/en/models')).toEqual({ path: '/models' })
    expect(resolvePublicLocaleRoute('zh', '/en/docs', { cat: 'tutorial' })).toEqual({
      path: '/docs',
      query: { cat: 'tutorial' },
    })
  })

  it('does not invent English paths for routes already in the requested public locale', () => {
    expect(resolvePublicLocaleRoute('en', '/en/docs')).toBeNull()
    expect(resolvePublicLocaleRoute('zh', '/home')).toBeNull()
  })
})
