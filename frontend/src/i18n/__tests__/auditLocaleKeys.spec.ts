import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

const testDir = dirname(fileURLToPath(import.meta.url))
const auditViewSource = readFileSync(resolve(testDir, '../../views/admin/AuditLogView.vue'), 'utf8')
const routerSource = readFileSync(resolve(testDir, '../../router/index.ts'), 'utf8')

function collectAuditKeys(): string[] {
  const keys = new Set<string>()
  for (const source of [auditViewSource, routerSource]) {
    for (const match of source.matchAll(/['"]admin\.audit\.([^'"]+)['"]/g)) {
      keys.add(match[1])
    }
  }
  return [...keys].sort()
}

function getMessage(messages: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((node, part) => {
    if (!node || typeof node !== 'object') return undefined
    return (node as Record<string, unknown>)[part]
  }, messages)
}

describe('audit log locale keys', () => {
  it.each([
    ['zh', zh],
    ['en', en]
  ] as const)('has runtime translations for every %s admin.audit key used by the audit page', (_locale, messages) => {
    const missing = collectAuditKeys().filter((key) => {
      const value = getMessage(messages as Record<string, unknown>, `admin.audit.${key}`)
      return typeof value !== 'string' || value.trim() === ''
    })
    expect(missing).toEqual([])
  })
})
