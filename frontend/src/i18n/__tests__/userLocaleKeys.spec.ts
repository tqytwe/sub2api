import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

const testDir = dirname(fileURLToPath(import.meta.url))
const usersViewSource = readFileSync(resolve(testDir, '../../views/admin/UsersView.vue'), 'utf8')
const bulkEditModalSource = readFileSync(
  resolve(testDir, '../../components/admin/user/BulkEditUserModal.vue'),
  'utf8'
)

function collectUserKeys(): string[] {
  const keys = new Set<string>()
  for (const source of [usersViewSource, bulkEditModalSource]) {
    for (const match of source.matchAll(/['"]admin\.users\.bulkLimits\.([^'"]+)['"]/g)) {
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

describe('user management locale keys', () => {
  it.each([
    ['zh', zh],
    ['en', en]
  ] as const)('has runtime translations for every %s bulk limits key', (_locale, messages) => {
    const missing = collectUserKeys().filter((key) => {
      const value = getMessage(messages as Record<string, unknown>, `admin.users.bulkLimits.${key}`)
      return typeof value !== 'string' || value.trim() === ''
    })
    expect(missing).toEqual([])
  })
})
