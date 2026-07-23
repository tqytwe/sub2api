import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

describe('IP risk workspace routing', () => {
  it('keeps IP resources as the default proxies route and adds risk workspaces', () => {
    const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
    const viewSource = readFileSync(resolve(process.cwd(), 'src/views/admin/ProxiesView.vue'), 'utf8')

    expect(routerSource).toContain("path: '/admin/proxies'")
    expect(routerSource).toContain("name: 'AdminProxies'")
    expect(routerSource).toContain("path: '/admin/proxies/risk'")
    expect(routerSource).toContain("name: 'AdminIPRisk'")
    expect(routerSource).toContain("path: '/admin/proxies/actions'")
    expect(routerSource).toContain("name: 'AdminIPRiskActions'")

    expect(viewSource).toContain("return 'resources'")
    expect(viewSource).toContain('<TablePageLayout v-if="activeTab === \'resources\'">')
    expect(viewSource).toContain("risk: 'AdminIPRisk'")
    expect(viewSource).toContain("actions: 'AdminIPRiskActions'")
  })
})
