import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import TeamRewardSettings from '@/components/admin/play/TeamRewardSettings.vue'

const {
  getTeamRewardSettingsMock,
  updateTeamRewardSettingsMock,
  listTeamRewardSettlementsMock,
  retryTeamRewardSettlementMock,
  showErrorMock,
  showSuccessMock,
} = vi.hoisted(() => ({
  getTeamRewardSettingsMock: vi.fn(),
  updateTeamRewardSettingsMock: vi.fn(),
  listTeamRewardSettlementsMock: vi.fn(),
  retryTeamRewardSettlementMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
}))

vi.mock('@/api/admin/play', () => ({
  default: {
    getTeamRewardSettings: (...args: unknown[]) => getTeamRewardSettingsMock(...args),
    updateTeamRewardSettings: (...args: unknown[]) => updateTeamRewardSettingsMock(...args),
    listTeamRewardSettlements: (...args: unknown[]) => listTeamRewardSettlementsMock(...args),
    retryTeamRewardSettlement: (...args: unknown[]) => retryTeamRewardSettlementMock(...args),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: showErrorMock, showSuccess: showSuccessMock }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ locale: { value: 'zh-CN' } }) }
})

const configuredSettings = {
  enabled: true,
  cap: '100',
  start_month: '2026-08',
  tiers: [
    { threshold: '20', rate: '0.003' },
    { threshold: '100', rate: '0.004' },
    { threshold: '500', rate: '0.005' },
    { threshold: '2000', rate: '0.006' },
  ],
}

enableAutoUnmount(afterEach)

function cloneSettings() {
  return JSON.parse(JSON.stringify(configuredSettings))
}

async function mountEditor() {
  const wrapper = mount(TeamRewardSettings, { global: { stubs: { Icon: true, Toggle: true } } })
  await flushPromises()
  return wrapper
}

describe('TeamRewardSettings', () => {
  beforeEach(() => {
    getTeamRewardSettingsMock.mockReset().mockResolvedValue(cloneSettings())
    updateTeamRewardSettingsMock.mockReset().mockImplementation(async (settings) => settings)
    listTeamRewardSettlementsMock.mockReset().mockResolvedValue([])
    retryTeamRewardSettlementMock.mockReset()
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
  })

  it('prevents a descending reward rate from being sent to the API', async () => {
    const wrapper = await mountEditor()

    await wrapper.get('[data-testid="team-rate-3"]').setValue('0.001')

    expect(wrapper.get('[data-testid="team-reward-validation"]').text()).toContain('返还比例必须严格递增')
    expect(wrapper.get('[data-testid="save-team-rewards"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="save-team-rewards"]').trigger('click')
    expect(updateTeamRewardSettingsMock).not.toHaveBeenCalled()
  })

  it('adds a valid next tier and supports removing it before saving', async () => {
    const wrapper = await mountEditor()

    await wrapper.get('[data-testid="add-team-reward-tier"]').trigger('click')
    expect(wrapper.findAll('[data-testid="team-reward-tier-row"]')).toHaveLength(5)
    expect(wrapper.get('[data-testid="team-threshold-4"]').element).toHaveProperty('value', '4000')
    expect(wrapper.get('[data-testid="team-rate-4"]').element).toHaveProperty('value', '0.007')

    await wrapper.get('[data-testid="remove-team-reward-tier-4"]').trigger('click')
    expect(wrapper.findAll('[data-testid="team-reward-tier-row"]')).toHaveLength(4)
  })

  it('saves a valid editable configuration through the admin API', async () => {
    const wrapper = await mountEditor()

    await wrapper.get('[data-testid="team-reward-cap"]').setValue('120')
    await wrapper.get('[data-testid="save-team-rewards"]').trigger('click')
    await flushPromises()

    expect(updateTeamRewardSettingsMock).toHaveBeenCalledWith({
      ...configuredSettings,
      cap: '120',
    })
    expect(showSuccessMock).toHaveBeenCalledTimes(1)
  })

  it('keeps settlement allocations collapsed until an operator opens the record', async () => {
    listTeamRewardSettlementsMock.mockResolvedValue([
      {
        settlement: { id: 8, period_start: '2026-07-01T00:00:00Z', pool_amount: '10', status: 'completed' },
        allocations: [{ id: 81, display_name: '已脱敏用户', email: 'u***@example.com', reward_amount: '4.84', payout_status: 'paid', paid_at: '2026-08-01T00:10:00Z' }],
      },
    ])
    const wrapper = await mountEditor()

    expect(wrapper.text()).toContain('发放人数 1')
    expect(wrapper.text()).not.toContain('已脱敏用户')
    await wrapper.get('button[aria-expanded="false"]').trigger('click')
    expect(wrapper.text()).toContain('已脱敏用户')
    expect(wrapper.text()).toContain('$4.84')
    expect(wrapper.text()).toContain('已到账')
  })
})
