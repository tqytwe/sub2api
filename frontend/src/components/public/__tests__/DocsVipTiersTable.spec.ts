import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import DocsVipTiersTable from '../DocsVipTiersTable.vue'

const fetchPublicVIPTiers = vi.fn()

vi.mock('@/api/publicVipTiers', () => ({
  fetchPublicVIPTiers: (...args: unknown[]) => fetchPublicVIPTiers(...args),
}))

const messages: Record<string, string> = {
  'docs.vipTiers.loading': 'Loading VIP tiers…',
  'docs.vipTiers.lead': 'Synced with Play Hub',
  'docs.vipTiers.tableTitle': 'VIP tiers',
  'docs.vipTiers.colTier': 'Tier',
  'docs.vipTiers.colRecharge': 'Lifetime top-up',
  'docs.vipTiers.colPerks': 'Perks',
  'docs.vipTiers.syncNote': 'Sync note',
  'docs.vipTiers.perks.models_vip_tag': 'VIP badge',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, fallback?: string) => messages[key] ?? fallback ?? key,
      locale: { value: 'en' },
    }),
  }
})

describe('DocsVipTiersTable', () => {
  it('renders tiers from public API', async () => {
    fetchPublicVIPTiers.mockResolvedValue({
      enabled: true,
      tiers: [
        { tier: 1, label: 'V1', min_recharge: 50, perks: ['models_vip_tag'] },
      ],
    })

    const wrapper = mount(DocsVipTiersTable)
    await flushPromises()

    expect(wrapper.text()).toContain('V1')
    expect(wrapper.text()).toContain('$50')
    expect(wrapper.text()).toContain('VIP badge')
    expect(wrapper.text()).toContain('Synced with Play Hub')
  })

  it('renders nothing when API returns no tiers', async () => {
    fetchPublicVIPTiers.mockResolvedValue({ enabled: false, tiers: [] })

    const wrapper = mount(DocsVipTiersTable)
    await flushPromises()

    expect(wrapper.find('.docs-vip-tiers').exists()).toBe(false)
  })
})
