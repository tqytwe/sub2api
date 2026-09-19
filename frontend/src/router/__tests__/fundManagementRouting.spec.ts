import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('fund-management routing', () => {
  it('retires the classification route through a compatibility redirect', () => {
    const router = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
    const sidebar = readFileSync(resolve(process.cwd(), 'src/components/layout/AppSidebar.vue'), 'utf8')
    expect(router).toContain("path: '/admin/funds/classification'")
    expect(router).toContain("redirect: '/admin/funds/operations'")
    expect(router).toContain("path: '/admin/funds/:tab(refunds|credits|operations)'")
    expect(sidebar).toContain("path: '/admin/funds/credits'")
    expect(sidebar).toContain("path: '/admin/funds/operations'")
    expect(sidebar).not.toContain("path: '/admin/funds/classification'")
  })
})
