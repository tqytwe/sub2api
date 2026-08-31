import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const directory = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(directory, '../AppHeader.vue'), 'utf8')
const plazaNavSource = readFileSync(resolve(directory, '../../modelPlaza/PlazaNavBar.vue'), 'utf8')

describe('model catalog navigation paths', () => {
  it('keeps the authenticated header entry on browser catalog routes', () => {
    expect(headerSource).toContain('const modelPlazaTarget')
    expect(headerSource).toContain("? '/en/catalog' : '/catalog'")
    expect(headerSource).toContain("locale.value.toLowerCase().startsWith('en')")
    expect(headerSource).toContain('resolveCustomMenuRoute({ url: docUrl.value }, locale.value)')
    expect(headerSource).not.toContain("path: '/models', query: { embedded: '1' }")
  })

  it('preserves the localized catalog page after guests sign in', () => {
    expect(plazaNavSource).toContain("redirect: locale === 'en' ? '/en/catalog' : '/catalog'")
    expect(plazaNavSource).not.toContain("redirect: '/models'")
  })
})
