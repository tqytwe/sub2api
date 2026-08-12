import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

describe('Play Billing admin route integration', () => {
  it('adds the admin payment navigation entry and bilingual labels', () => {
    const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
    const sidebarSource = readFileSync(resolve(process.cwd(), 'src/components/layout/AppSidebar.vue'), 'utf8')
    const zhSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/zh.ts'), 'utf8')
    const enSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/en.ts'), 'utf8')

    expect(routerSource).toContain("path: '/admin/orders/play-billing'")
    expect(routerSource).toContain("name: 'AdminPlayBillingConfig'")
    expect(routerSource).toContain("component: () => import('@/views/admin/orders/AdminPlayBillingConfigView.vue')")
    expect(routerSource).toContain("titleKey: 'nav.playBilling'")

    const orderNavBlock = sidebarSource.slice(
      sidebarSource.indexOf("path: '/admin/orders'"),
      sidebarSource.indexOf("path: '/admin/usage'")
    )
    expect(orderNavBlock).toContain("path: '/admin/orders/play-billing'")
    expect(orderNavBlock).toContain("label: t('nav.playBilling')")
    expect(zhSource).toContain("playBilling: 'Play 内购'")
    expect(enSource).toContain("playBilling: 'Play Billing'")
    expect(zhSource).toContain('Play Billing 商品映射')
    expect(enSource).toContain('Play Billing Product Mapping')
  })
})
