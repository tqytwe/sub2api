import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ArenaRewardSettings from '@/components/admin/play/ArenaRewardSettings.vue'

const { getArenaRewardSettingsMock, updateArenaRewardSettingsMock, showErrorMock, showSuccessMock } = vi.hoisted(() => ({
  getArenaRewardSettingsMock: vi.fn(),
  updateArenaRewardSettingsMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
}))

vi.mock('@/api/admin/play', () => ({
  default: {
    getArenaRewardSettings: (...args: unknown[]) => getArenaRewardSettingsMock(...args),
    updateArenaRewardSettings: (...args: unknown[]) => updateArenaRewardSettingsMock(...args),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: showErrorMock, showSuccess: showSuccessMock }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ locale: { value: 'zh-CN' } }) }
})

const settings = {
  monthly: [{ rank_max: 1, amount: 50 }, { rank_max: 3, amount: 20 }, { rank_max: 10, amount: 5 }],
  daily: [{ rank_max: 1, amount: 0.5 }, { rank_max: 3, amount: 0.2 }, { rank_max: 10, amount: 0.1 }],
  daily_budget: 50,
}

enableAutoUnmount(afterEach)

describe('ArenaRewardSettings', () => {
  beforeEach(() => {
    getArenaRewardSettingsMock.mockReset().mockResolvedValue(structuredClone(settings))
    updateArenaRewardSettingsMock.mockReset().mockImplementation(async (value) => value)
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
  })

  it('calculates monthly payout by rank ranges rather than adding tier amounts', async () => {
    const wrapper = mount(ArenaRewardSettings, { global: { stubs: { Icon: true } } })
    await flushPromises()

    expect(wrapper.text()).toContain('$125.00')
    expect(wrapper.text()).toContain('奖励名次 10')
  })

  it('blocks a non-increasing rank schedule before save', async () => {
    const wrapper = mount(ArenaRewardSettings, { global: { stubs: { Icon: true } } })
    await flushPromises()

    await wrapper.findAll('input[type="number"]')[2].setValue(1)
    expect(wrapper.text()).toContain('名次必须严格递增')
    expect(wrapper.get('.btn-primary').attributes('disabled')).toBeDefined()
    await wrapper.get('.btn-primary').trigger('click')
    expect(updateArenaRewardSettingsMock).not.toHaveBeenCalled()
  })
})
