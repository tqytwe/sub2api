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

function collectNavKeys(): string[] {
  const keys = new Set<string>()
  for (const file of sourceFiles(sourceRoot)) {
    const source = readFileSync(file, 'utf8')
    for (const match of source.matchAll(/\bt\(\s*['"]nav\.([^'"]+)['"]/g)) {
      keys.add(match[1])
    }
    for (const match of source.matchAll(/\btitleKey:\s*['"]nav\.([^'"]+)['"]/g)) {
      keys.add(match[1])
    }
  }
  return [...keys].sort()
}

describe('navigation locale keys', () => {
  it.each([
    ['zh', zh],
    ['en', en]
  ] as const)('has runtime translations for every %s nav key used by the app', (_locale, messages) => {
    const nav = (messages as { nav: Record<string, string> }).nav
    const missing = collectNavKeys().filter((key) => typeof nav[key] !== 'string' || nav[key].trim() === '')
    expect(missing).toEqual([])
  })
})
