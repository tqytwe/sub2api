import { readdirSync, readFileSync } from 'node:fs'
import { extname, join, relative, resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

import { LOCALE_LOAD_SCOPES, i18n, loadLocaleMessages } from '../index'

type MessageTree = Record<string, unknown>

const SOURCE_ROOT = resolve(process.cwd(), 'src')
const TRANSLATION_CALL = /(?<![A-Za-z0-9_$])(?:\$?t|i18n\.global\.t)\s*\(\s*(['"])([A-Za-z][A-Za-z0-9_-]*(?:\.[A-Za-z][A-Za-z0-9_-]*)+)\1/g

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const file = join(directory, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === '__tests__') return []
      return sourceFiles(file)
    }
    if (!entry.isFile() || entry.name.endsWith('.spec.ts')) return []
    return ['.ts', '.vue'].includes(extname(entry.name)) ? [file] : []
  })
}

function literalTranslationKeys(): Map<string, string[]> {
  const locations = new Map<string, string[]>()
  for (const file of sourceFiles(SOURCE_ROOT)) {
    const content = readFileSync(file, 'utf8')
    for (const match of content.matchAll(TRANSLATION_CALL)) {
      const key = match[2]
      if (!key) continue
      const lines = locations.get(key) ?? []
      lines.push(relative(SOURCE_ROOT, file))
      locations.set(key, lines)
    }
  }
  return locations
}

function messageAt(messages: MessageTree, key: string): unknown {
  return key.split('.').reduce<unknown>((node, segment) => {
    if (!node || typeof node !== 'object') return undefined
    return (node as MessageTree)[segment]
  }, messages)
}

const LITERAL_TRANSLATION_KEYS = literalTranslationKeys()

describe('literal translation coverage', () => {
  it.each(['zh', 'en'] as const)(
    'resolves every literal user-visible translation key from actual %s fragments',
    async (locale) => {
      // Deliberately load the production fragments one at a time. Do not import
      // legacy zh.ts/en.ts here: they are not a runtime dependency and can mask
      // a route fragment that was omitted from the loader registry.
      await Promise.all(LOCALE_LOAD_SCOPES.map((scope) => loadLocaleMessages(locale, scope)))

      const messages = i18n.global.getLocaleMessage(locale) as MessageTree
      const missing = [...LITERAL_TRANSLATION_KEYS.entries()]
        .filter(([key]) => {
          const value = messageAt(messages, key)
          return typeof value !== 'string' || !value.trim()
        })
        .map(([key, locations]) => `${key} (${[...new Set(locations)].join(', ')})`)

      expect(missing).toEqual([])
    },
  )
})
