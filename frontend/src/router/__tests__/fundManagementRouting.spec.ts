import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

describe('fund management route integration', () => {
  it('adds unified admin fund management and bilingual navigation labels', () => {
    const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
    const sidebarSource = readFileSync(resolve(process.cwd(), 'src/components/layout/AppSidebar.vue'), 'utf8')
    const zhSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/zh.ts'), 'utf8')
    const enSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/en.ts'), 'utf8')

    expect(routerSource).toContain("path: '/admin/funds'")
    expect(routerSource).toContain("path: '/admin/funds/:tab(refunds|grants|classification)'")
    expect(routerSource).toContain("component: () => import('@/views/admin/AdminFundsView.vue')")
    expect(routerSource).toContain("titleKey: 'admin.funds.title'")
    expect(sidebarSource).toContain("label: t('nav.fundManagement')")
    expect(sidebarSource).toContain("path: '/admin/funds/refunds'")
    expect(sidebarSource).toContain("path: '/admin/funds/grants'")
    expect(sidebarSource).toContain("path: '/admin/funds/classification'")
    expect(sidebarSource).toContain("label: t('nav.refundRequests')")
    expect(sidebarSource).toContain("label: t('nav.giftBalance')")
    expect(sidebarSource).toContain("label: t('nav.historicalGiftReview')")
    expect(zhSource).toContain("fundManagement: '资金管理'")
    expect(zhSource).toContain("refundRequests: '退款申请'")
    expect(zhSource).toContain("giftBalance: '赠送余额'")
    expect(zhSource).toContain("historicalGiftReview: '历史赠送复核'")
    expect(enSource).toContain("fundManagement: 'Fund Management'")
    expect(enSource).toContain("refundRequests: 'Refund Requests'")
    expect(enSource).toContain("giftBalance: 'Gift Balance'")
    expect(enSource).toContain("historicalGiftReview: 'Historical Gift Review'")
  })

  it('keeps wallet recharge return copy bilingual and source-specific', () => {
    const walletApiSource = readFileSync(resolve(process.cwd(), 'src/api/wallet.ts'), 'utf8')
    const walletViewSource = readFileSync(resolve(process.cwd(), 'src/views/user/WalletView.vue'), 'utf8')
    const adminApiSource = readFileSync(resolve(process.cwd(), 'src/api/admin/funds.ts'), 'utf8')
    const zhSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/zh.ts'), 'utf8')
    const enSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/en.ts'), 'utf8')

    expect(walletApiSource).toContain('/user/wallet/refund-requests')
    expect(adminApiSource).toContain('/admin/funds/refund-requests')
    expect(walletViewSource).toContain('wallet.refunds.title')
    expect(walletViewSource).toContain('refundable_recharge_balance')
    expect(walletViewSource).toContain('integerAmountRequired')
    expect(zhSource).toContain('真实充值和线下充值的未消费部分可从这里申请退回')
    expect(zhSource).toContain('赠送余额可消费，默认不可提现或退回')
    expect(zhSource).toContain('退回金额必须为整数')
    expect(enSource).toContain('Request a return for the unconsumed part of real online or offline recharge')
    expect(enSource).toContain('Gift balance is spendable but not withdrawable or refundable by default')
    expect(enSource).toContain('Return amount must be a whole number')
  })
})
