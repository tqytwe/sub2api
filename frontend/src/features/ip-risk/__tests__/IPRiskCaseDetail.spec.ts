import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import IPRiskCaseDetail from '@/features/ip-risk/IPRiskCaseDetail.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
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
})
