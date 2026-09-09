import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('admin fund API contract', () => {
  const source = readFileSync(resolve(process.cwd(), 'src/api/admin/funds.ts'), 'utf8')
  it('uses account-based credit payloads and public operation APIs', () => {
    expect(source).toContain("'/admin/funds/accounts/search'")
    expect(source).toContain("'/admin/funds/operations'")
    expect(source).toContain("'/admin/funds/compensations'")
    expect(source).toContain('/corrections/retry')
    expect(source).toContain('/corrections/cancel')
    expect(source).toContain('account_email: string')
    expect(source).not.toContain('user_id: number')
    expect(source).not.toContain('classifications/signup-gift-30')
  })
})
