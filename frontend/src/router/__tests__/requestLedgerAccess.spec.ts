import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
const usageSource = readFileSync(resolve(process.cwd(), 'src/views/user/UsageView.vue'), 'utf8')
const userRoutesSource = readFileSync(resolve(process.cwd(), '../backend/internal/server/routes/user.go'), 'utf8')
const adminRoutesSource = readFileSync(resolve(process.cwd(), '../backend/internal/server/routes/admin.go'), 'utf8')

describe('request ledger access boundary', () => {
  it('exposes the request ledger only through the administrator route', () => {
    expect(routerSource).not.toContain("path: '/requests'")
    expect(routerSource).not.toContain("name: 'RequestLedger'")
    const adminRouteStart = routerSource.indexOf("name: 'AdminRequestLedger'")
    expect(adminRouteStart).toBeGreaterThanOrEqual(0)
    expect(routerSource.slice(adminRouteStart, adminRouteStart + 300)).toContain('requiresAdmin: true')
    expect(usageSource).not.toContain("name: 'RequestLedger'")
  })

  it('does not register user request ledger API routes', () => {
    expect(userRoutesSource).not.toContain('h.RequestLedger.ListUser')
    expect(userRoutesSource).not.toContain('h.RequestLedger.GetUser')
    expect(userRoutesSource).not.toContain('h.RequestLedger.UsageUser')
    expect(adminRoutesSource).toContain('h.RequestLedger.ListAdmin')
    expect(adminRoutesSource).toContain('h.RequestLedger.GetAdmin')
  })
})
