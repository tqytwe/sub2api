import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import IPRiskCaseDetail from '@/features/ip-risk/IPRiskCaseDetail.vue'
import type { RiskCaseDetail } from '@/features/ip-risk/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key === 'common.unknownStatus' ? 'Unknown status' : key,
    }),
  }
})

describe('IPRiskCaseDetail', () => {
  it('centers the empty prompt across the full detail pane', () => {
    const wrapper = mount(IPRiskCaseDetail, {
      props: {
        detail: null,
        selectedUserIds: [],
      },
    })

    const emptyState = wrapper.get('[data-testid="ip-risk-detail-empty"]')
    expect(emptyState.classes()).toEqual(expect.arrayContaining([
      'flex-1',
      'items-center',
      'justify-center',
      'text-center',
    ]))
  })

  it('does not expose a newer risk level as an untranslated key or raw enum', () => {
    const detail = {
      case: {
        id: 1,
        primary_ip: '203.0.113.8',
        score: 12,
        level: 'deferred_review',
        evidence_confidence: 'exact',
        signals: [],
        last_detected_at: '2026-08-26T00:00:00Z',
      },
      evidence: { known_shared_network: false },
      users: [],
      timeline: [],
      actions: [],
    } as unknown as RiskCaseDetail
    const wrapper = mount(IPRiskCaseDetail, {
      props: {
        detail,
        selectedUserIds: [],
      },
    })

    expect(wrapper.text()).toContain('Unknown status')
    expect(wrapper.text()).not.toContain('deferred_review')
    expect(wrapper.text()).not.toContain('admin.ipRisk.levels.deferred_review')
  })
})
