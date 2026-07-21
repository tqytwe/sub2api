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
    expect(sidebarSource).toContain("path: '/admin/funds'")
    expect(sidebarSource).toContain("{ path: '/admin/withdrawals', label: t('nav.rewardWithdrawals')")
    expect(zhSource).toContain("withdrawals: '提现管理'")
    expect(zhSource).toContain("fundManagement: '资金管理'")
    expect(zhSource).toContain("rewardWithdrawals: '奖励提现'")
    expect(enSource).toContain("withdrawals: 'Withdrawals'")
    expect(enSource).toContain("fundManagement: 'Fund Management'")
    expect(enSource).toContain("rewardWithdrawals: 'Reward Withdrawals'")
  })

  it('keeps wallet withdrawal labels bilingual and avoids English fallback in Chinese resources', () => {
    const zhSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/zh.ts'), 'utf8')
    const enSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/en.ts'), 'utf8')

    for (const text of ['申请提现', '收款账户', '最低提现金额', '线下打款']) {
      expect(zhSource).toContain(text)
    }
    for (const text of ['Request Withdrawal', 'Payout Account', 'Minimum Withdrawal', 'Offline Payout']) {
      expect(enSource).toContain(text)
    }
  })

  it('localizes admin withdrawal recompute status codes', () => {
    const viewSource = readFileSync(resolve(process.cwd(), 'src/views/admin/AdminWithdrawalsView.vue'), 'utf8')
    const zhSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/zh.ts'), 'utf8')
    const enSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/en.ts'), 'utf8')

    expect(viewSource).toContain('recalcStatusLabel(userSettings?.recalc_status)')
    expect(viewSource).toContain('runUserRecomputeDryRun')
    expect(viewSource).toContain('executeUserRecompute')
    expect(viewSource).toContain('existing_entitlements_verified')
    expect(viewSource).toContain('hasExistingEntitlementReview')
    expect(viewSource).not.toContain("{{ userSettings?.recalc_status || '-' }}")
    expect(zhSource).toContain("ready: '已通过复核'")
    expect(zhSource).toContain("needs_review: '待复核'")
    expect(zhSource).toContain("runRecomputeCheck: '运行复核检查'")
    expect(zhSource).toContain("writeRecomputeResult: '写入复核结果'")
    expect(zhSource).toContain("recomputeExistingVerified: '现有权益已核对一致'")
    expect(zhSource).toContain("existing_entitlements_mismatch: '已有权益批次与本次重算结果不一致，请先按页面金额和来源完成业务核对。'")
    expect(zhSource).toContain("transaction_confidence: '流水 #{transaction_id} 缺少高可信账务标记，需核对来源。'")
    expect(zhSource).not.toContain('仅 ready 用户可开启提现')
    expect(zhSource).not.toContain('已有权益批次，需确认没有重复重算。')
    expect(enSource).toContain("ready: 'Ready'")
    expect(enSource).toContain("needs_review: 'Needs review'")
    expect(enSource).toContain("runRecomputeCheck: 'Run review check'")
    expect(enSource).toContain("writeRecomputeResult: 'Write review result'")
    expect(enSource).toContain("recomputeExistingVerified: 'Existing entitlements verified'")
  })

  it('keeps the admin withdrawal review panel responsive and explains withdrawable balance', () => {
    const viewSource = readFileSync(resolve(process.cwd(), 'src/views/admin/AdminWithdrawalsView.vue'), 'utf8')
    const zhSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/zh.ts'), 'utf8')
    const enSource = readFileSync(resolve(process.cwd(), 'src/i18n/locales/en.ts'), 'utf8')

    expect(viewSource).toContain('xl:grid-cols-[minmax(0,1fr)_minmax(0,420px)]')
    expect(viewSource).toContain('card min-w-0 overflow-hidden')
    expect(viewSource).toContain('min-w-0 overflow-hidden rounded border border-gray-200')
    expect(viewSource).toContain('max-w-full min-w-0 overflow-x-auto')
    expect(viewSource).toContain('class="break-words [overflow-wrap:anywhere]"')
    expect(viewSource).toContain('withdrawableBalanceExplanation')
    expect(viewSource).toContain('<Icon name="xCircle" size="sm" />')
    expect(viewSource).not.toContain('xl:grid-cols-[minmax(0,1fr)_minmax(360px,420px)]')
    expect(viewSource).not.toContain('<Icon name="x" size="sm" />')
    expect(zhSource).toContain('流水余额高不代表可以提现')
    expect(zhSource).toContain('账户总余额包含充值、赠送、签到、任务等所有余额')
    expect(enSource).toContain('A high ledger balance does not mean the user can withdraw it.')
  })
})
