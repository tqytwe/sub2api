import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountTodayStatsCell from '../AccountTodayStatsCell.vue'
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
const stats = { requests: 2, tokens: 1200, cost: 0.25, user_cost: 0.8 }
describe('retained-history account totals', () => {
  it('renders account cumulative cost separately from user charges', () => {
    const wrapper = mount(AccountTodayStatsCell, { props: { stats: { ...stats, lifetime_tokens: 1230000, lifetime_cost: 12.5 } } })
    expect(wrapper.get('[data-testid="lifetime-tokens"]').text()).toContain('1.23M')
    expect(wrapper.get('[data-testid="lifetime-cost"]').text()).toContain('$12.50')
    expect(wrapper.text()).toContain('admin.accounts.stats.retainedHistory')
    expect(wrapper.text()).toContain('$0.80')
  })
  it('omits absent totals for old servers and accepts explicit zero', () => {
    const old = mount(AccountTodayStatsCell, { props: { stats } })
    expect(old.find('[data-testid="lifetime-tokens"]').exists()).toBe(false)
    const zero = mount(AccountTodayStatsCell, { props: { stats: { ...stats, lifetime_tokens: 0, lifetime_cost: 0 } } })
    expect(zero.get('[data-testid="lifetime-tokens"]').text()).toContain('0')
    expect(zero.get('[data-testid="lifetime-cost"]').text()).toContain('$0.00')
  })
  it('keeps the existing loading, error and no-data states', () => {
    // design-governance-allow: continuous-motion - Assertion of the existing bounded loading skeleton; no animation is introduced.
    expect(mount(AccountTodayStatsCell, { props: { loading: true } }).find('.animate-pulse').exists()).toBe(true)
    expect(mount(AccountTodayStatsCell, { props: { error: 'local failure' } }).text()).toBe('local failure')
    expect(mount(AccountTodayStatsCell).text()).toBe('-')
  })
})
