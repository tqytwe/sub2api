import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

import enCommon from '@/i18n/locales/en/common'
import zhCommon from '@/i18n/locales/zh/common'
import { localizedEnumOrUnknown } from '@/utils/localizedEnum'

function translateFor(locale: 'zh' | 'en') {
  const known = locale === 'zh' ? '已知状态' : 'Known status'
  const common = (locale === 'zh' ? zhCommon : enCommon).common
  return (key: string): string => {
    if (key === 'common.unknownStatus') return common.unknownStatus
    if (key === 'testEnum.known') return known
    return key
  }
}

describe('localizedEnumOrUnknown', () => {
  it.each([
    ['zh', '未知状态'],
    ['en', 'Unknown status'],
  ] as const)('uses a localized unknown-status label for a missing %s enum key', (locale, expected) => {
    const t = translateFor(locale)

    expect(localizedEnumOrUnknown(t, 'testEnum.new_backend_value')).toBe(expected)
    expect(localizedEnumOrUnknown(t, 'testEnum.known')).toBe(locale === 'zh' ? '已知状态' : 'Known status')
  })

  it('keeps every audited server-status surface on the safe enum path', () => {
    const sourceExpectations: Record<string, string> = {
      'src/components/imageStudio/ImageStudioGallery.vue': 'localizedEnumOrUnknown',
      'src/components/payment/OrderStatusBadge.vue': "t('common.unknownStatus')",
      'src/views/admin/AnnouncementsView.vue': 'localizedEnumOrUnknown',
      'src/components/admin/usage/UsageCleanupDialog.vue': 'localizedEnumOrUnknown',
      'src/views/public/AgentTeamView.vue': 'localizedEnumOrUnknown',
      'src/views/admin/SubscriptionsView.vue': 'localizedEnumOrUnknown',
      'src/composables/useChannelMonitorFormat.ts': 'localizedEnumOrUnknown',
      'src/views/user/KeysView.vue': 'localizedEnumOrUnknown',
      'src/views/admin/RedeemView.vue': 'localizedEnumOrUnknown',
      'src/components/user/UserErrorRequestsTable.vue': 'localizedEnumOrUnknown',
      'src/components/user/UserErrorDetailModal.vue': 'localizedEnumOrUnknown',
      'src/components/coupon/CouponWalletPanel.vue': 'localizedEnumOrUnknown',
      'src/views/user/SpeedTestView.vue': 'localizedEnumOrUnknown',
      'src/features/ip-risk/IPRiskActionsView.vue': 'localizedEnumOrUnknown',
      'src/features/ip-risk/IPRiskWorkbench.vue': 'localizedEnumOrUnknown',
      'src/features/ip-risk/IPRiskCaseDetail.vue': 'localizedEnumOrUnknown',
      'src/features/channel-monitor-v2/MonitorSettingsPanel.vue': 'localizedEnumOrUnknown',
    }

    for (const [source, expected] of Object.entries(sourceExpectations)) {
      expect(readFileSync(resolve(process.cwd(), source), 'utf8'), source).toContain(expected)
    }
  })
})
