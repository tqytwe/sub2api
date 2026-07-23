import { describe, expect, it } from 'vitest'

import { localeFromPath, localeScopeForPath } from '../index'
import enCore from '../locales/en/core'
import zhCore from '../locales/zh/core'
import enFull from '../locales/en'
import zhFull from '../locales/zh'

function readPath(messages: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => {
    if (!node || typeof node !== 'object') return undefined
    return (node as Record<string, unknown>)[key]
  }, messages)
}

describe('lazy locale loading scopes', () => {
  it.each([
    '/',
    '/home',
    '/en',
    '/en/',
    '/login',
    '/register',
    '/setup',
    '/key-usage',
  ])('keeps %s on the lightweight core locale scope', (path) => {
    expect(localeScopeForPath(path)).toBe('core')
  })

  it.each([
    '/en/models',
    '/en/docs',
    '/dashboard',
    '/wallet',
    '/arena',
    '/admin/users',
    '/admin/settings',
  ])('loads full locale messages before rendering %s', (path) => {
    expect(localeScopeForPath(path)).toBe('full')
  })

  it('forces English only for the explicit /en route layer', () => {
    expect(localeFromPath('/en')).toBe('en')
    expect(localeFromPath('/en/')).toBe('en')
    expect(localeFromPath('/en/models')).toBe('en')
    expect(localeFromPath('/en/docs?cat=tutorial')).toBe('en')
  })

  it('forces Chinese on public Chinese routes even after English route visits', () => {
    expect(localeFromPath('/')).toBe('zh')
    expect(localeFromPath('/home')).toBe('zh')
    expect(localeFromPath('/models')).toBe('zh')
    expect(localeFromPath('/docs')).toBe('zh')
    expect(localeFromPath('/login')).toBe('zh')
    expect(localeFromPath('/register')).toBe('zh')
    expect(localeFromPath('/about')).toBe('zh')
    expect(localeFromPath('/contact')).toBe('zh')
    expect(localeFromPath('/dashboard')).toBeNull()
  })

  it.each([
    ['zh', zhCore],
    ['en', enCore],
  ] as const)('%s core locale includes public entry copy without admin payload', (_locale, messages) => {
    expect(readPath(messages, 'auth.signIn')).toBeTruthy()
    expect(readPath(messages, 'authAside.titleLogin')).toBeTruthy()
    expect(readPath(messages, 'authAside.backHome')).toBeTruthy()
    expect(readPath(messages, 'home.jisudeng.nav.signIn')).toBeTruthy()
    expect(readPath(messages, 'home.jisudeng.registerBanner.signupCredit')).toBeTruthy()
    expect(readPath(messages, 'admin')).toBeUndefined()
  })

  it.each([
    ['zh', zhFull],
    ['en', enFull],
  ] as const)('%s full locale keeps admin copy available', (_locale, messages) => {
    expect(readPath(messages, 'admin.users.title')).toBeTruthy()
    expect(readPath(messages, 'admin.settings.title')).toBeTruthy()
  })
})
