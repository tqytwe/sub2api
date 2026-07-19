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
  'docs.vipTiers.colRecharge': 'Base recharge total',
  'docs.vipTiers.colRechargeBonus': 'Recharge bonus',
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
        { tier: 0, label: 'V0', min_recharge: 0, recharge_bonus_pct: 0, color_key: 'neutral', perks: [] },
        { tier: 1, label: 'V1', min_recharge: 50, recharge_bonus_pct: 2, color_key: 'emerald', perks: ['models_vip_tag'] },
        { tier: 2, label: 'V2', min_recharge: 100, recharge_bonus_pct: 4, color_key: 'sky', perks: ['models_vip_tag'] },
        { tier: 3, label: 'V3', min_recharge: 200, recharge_bonus_pct: 6, color_key: 'indigo', perks: ['models_vip_tag'] },
        { tier: 4, label: 'V4', min_recharge: 500, recharge_bonus_pct: 8, color_key: 'amber', perks: ['models_vip_tag'] },
        { tier: 5, label: 'V5', min_recharge: 1000, recharge_bonus_pct: 10, color_key: 'gold', perks: ['models_vip_tag'] },
      ],
    })

    const wrapper = mount(DocsVipTiersTable)
    await flushPromises()

    expect(wrapper.findAll('tbody tr')).toHaveLength(6)
    expect(wrapper.text()).toContain('V0')
    expect(wrapper.text()).toContain('V5')
    expect(wrapper.text()).toContain('$1,000')
    expect(wrapper.text()).toContain('Recharge bonus')
    expect(wrapper.text()).toContain('+10%')
    expect(wrapper.text()).toContain('VIP badge')
    expect(wrapper.text()).toContain('Synced with Play Hub')
    expect(wrapper.find('.vip-tier-badge-gold').exists()).toBe(true)
  })

  it('renders nothing when API returns no tiers', async () => {
    fetchPublicVIPTiers.mockResolvedValue({ enabled: false, tiers: [] })

    const wrapper = mount(DocsVipTiersTable)
    await flushPromises()

    expect(wrapper.find('.docs-vip-tiers').exists()).toBe(false)
  })
})
