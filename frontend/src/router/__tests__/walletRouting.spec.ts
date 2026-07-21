import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

describe('wallet route integration', () => {
  it('adds a protected wallet page and sidebar entry with localized nav key', () => {
    const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
    const sidebarSource = readFileSync(resolve(process.cwd(), 'src/components/layout/AppSidebar.vue'), 'utf8')
    const zhSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/zh.ts'), 'utf8')
    const enSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/en.ts'), 'utf8')

    expect(routerSource).toContain("path: '/wallet'")
    expect(routerSource).toContain("component: () => import('@/views/user/WalletView.vue')")
    expect(routerSource).toContain("titleKey: 'wallet.title'")
    expect(sidebarSource).toContain("{ path: '/wallet', label: t('nav.wallet')")
    expect(zhSource).toContain("wallet: '钱包'")
    expect(enSource).toContain("wallet: 'Wallet'")
  })
})
