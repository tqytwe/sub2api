import { readdirSync, readFileSync, statSync } from 'node:fs'
import { extname, join, relative } from 'node:path'

import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

const sourceRoot = join(process.cwd(), 'src')
const sourceExtensions = new Set(['.ts', '.tsx', '.vue'])

function sourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    const rel = relative(sourceRoot, full)
    if (rel.startsWith('i18n/locales/') || rel.includes('/__tests__/')) continue
    const info = statSync(full)
    if (info.isDirectory()) {
      out.push(...sourceFiles(full))
    } else if (sourceExtensions.has(extname(full))) {
      out.push(full)
    }
  }
  return out
}

function flattenKeys(value: unknown, prefix = ''): string[] {
  if (Array.isArray(value)) {
    return value.flatMap((item, index) => flattenKeys(item, prefix ? `${prefix}.${index}` : String(index)))
  }

  if (typeof value === 'object' && value !== null) {
    return Object.entries(value).flatMap(([k, v]) => flattenKeys(v, prefix ? `${prefix}.${k}` : k))
  }

  return prefix ? [prefix] : []
}

function collectStaticLocaleKeys(): string[] {
  const keys = new Set<string>()
  const regexes = [
    /\bt\(\s*['"]([^'"`]+)['"]/g,
    /\b(?:titleKey|descriptionKey):\s*['"]([^'"`]+)['"]/g
  ]

  for (const file of sourceFiles(sourceRoot)) {
    const source = readFileSync(file, 'utf8')
    for (const re of regexes) {
      for (const match of source.matchAll(re)) {
        const key = match[1]
        if (key.includes('.') && !key.endsWith('.')) keys.add(key)
      }
    }
  }

  return [...keys].sort()
}

describe('static locale keys', () => {
  it.each([
    ['zh', zh],
    ['en', en]
  ] as const)('has runtime translations for every static %s locale key', (_locale, messages) => {
    const localeKeys = new Set(flattenKeys(messages))
    const missing = collectStaticLocaleKeys().filter((key) => !localeKeys.has(key))

    expect(missing).toEqual([])
  })
})
