import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('AdminFundsView fund-operation workflow', () => {
  const source = readFileSync(resolve(process.cwd(), 'src/views/admin/AdminFundsView.vue'), 'utf8')
  it('uses account selection and confirmation before a step-up protected credit', () => {
    expect(source).toContain('searchFundAccounts')
    expect(source).toContain('selectedAccount')
    expect(source).toContain('confirmationEmail !== selectedAccount?.email')
    expect(source).toContain('fundStepUp.run')
    expect(source).toContain('grantCompensation')
  })
  it('keeps database user identifiers out of the fund-management UI', () => {
    expect(source).not.toContain('user_id')
    expect(source).not.toContain('User ID')
    expect(source).toContain('operation_no')
    expect(source).toContain('correctFundOperation')
  })

  it('retains the complete protected refund workflow while adding fund operations', () => {
    expect(source).toContain('approveRefundRequest')
    expect(source).toContain('rejectRefundRequest')
    expect(source).toContain('markRefundPaid')
    expect(source).toContain('getRefundSensitivePayout')
    expect(source).toContain('payout_fx_rate')
    expect(source).toContain('external_txn_id')
    expect(source).toContain('fundStepUp.run')
    expect(source).toContain('request_no')
  })

  it('does not offer unsafe automatic correction for offline recharge records', () => {
    expect(source).toContain("['ops_gift', 'compensation'].includes(operation.operation_kind)")
    expect(source).not.toContain("['offline_recharge', 'ops_gift', 'compensation'].includes(operation.operation_kind)")
  })

  it('keeps pending correction retry and cancellation behind the same step-up flow', () => {
    expect(source).toContain('retryFundOperationCorrection')
    expect(source).toContain('cancelFundOperationCorrection')
    expect(source).toContain('fundStepUp.run(() => retryFundOperationCorrection')
    expect(source).toContain('fundStepUp.run(() => cancelFundOperationCorrection')
  })
})
