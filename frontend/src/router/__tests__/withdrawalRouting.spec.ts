import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

describe('withdrawal route integration', () => {
  it('adds admin withdrawal management route, sidebar entry, and bilingual nav labels', () => {
    const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
    const sidebarSource = readFileSync(resolve(process.cwd(), 'src/components/layout/AppSidebar.vue'), 'utf8')
    const zhSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/zh.ts'), 'utf8')
    const enSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/en.ts'), 'utf8')

    expect(routerSource).toContain("path: '/admin/withdrawals'")
    expect(routerSource).toContain("component: () => import('@/views/admin/AdminWithdrawalsView.vue')")
    expect(routerSource).toContain("titleKey: 'admin.withdrawals.title'")
    expect(sidebarSource).toContain("{ path: '/admin/withdrawals', label: t('nav.withdrawals')")
    expect(zhSource).toContain("withdrawals: '提现管理'")
    expect(enSource).toContain("withdrawals: 'Withdrawals'")
  })

  it('keeps wallet withdrawal labels bilingual and avoids English fallback in Chinese resources', () => {
    const zhSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/zh.ts'), 'utf8')
    const enSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/en.ts'), 'utf8')

    for (const text of ['申请提现', '收款账户', '最低提现金额', '线下打款', '双人审核']) {
      expect(zhSource).toContain(text)
    }
    for (const text of ['Request Withdrawal', 'Payout Account', 'Minimum Withdrawal', 'Offline Payout', 'Dual Review']) {
      expect(enSource).toContain(text)
    }
  })
})
