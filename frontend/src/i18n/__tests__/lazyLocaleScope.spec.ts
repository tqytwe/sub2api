import { describe, expect, it } from 'vitest'

import { localeScopeForPath } from '../index'
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
    '/login',
    '/register',
    '/setup',
    '/key-usage',
  ])('keeps %s on the lightweight core locale scope', (path) => {
    expect(localeScopeForPath(path)).toBe('core')
  })

  it.each([
    '/dashboard',
    '/wallet',
    '/arena',
    '/admin/users',
    '/admin/settings',
  ])('loads full locale messages before rendering %s', (path) => {
    expect(localeScopeForPath(path)).toBe('full')
  })

  it.each([
    ['zh', zhCore],
    ['en', enCore],
  ] as const)('%s core locale includes public entry copy without admin payload', (_locale, messages) => {
    expect(readPath(messages, 'auth.signIn')).toBeTruthy()
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
