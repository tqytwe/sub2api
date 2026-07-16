import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import BlindboxPoolEditor from '@/components/admin/play/BlindboxPoolEditor.vue'

const {
  getBlindboxPoolMock,
  updateBlindboxPoolMock,
  showErrorMock,
  showSuccessMock,
} = vi.hoisted(() => ({
  getBlindboxPoolMock: vi.fn(),
  updateBlindboxPoolMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
}))

vi.mock('@/api/admin/play', () => ({
  default: {
    getBlindboxPool: (...args: unknown[]) => getBlindboxPoolMock(...args),
    updateBlindboxPool: (...args: unknown[]) => updateBlindboxPoolMock(...args),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock,
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh-CN' },
    }),
  }
})

const configuredPool = {
  version: 'season-1-v1',
  cost: 0.5,
  rtp_cap: 0.9,
  tiers: [
    { amount: 0.05, weight: 4000 },
    { amount: 0.2, weight: 3000 },
    { amount: 0.5, weight: 1800 },
    { amount: 1, weight: 800 },
    { amount: 3, weight: 300 },
    { amount: 10, weight: 90 },
    { amount: 20, weight: 10 },
  ],
}

enableAutoUnmount(afterEach)

function clonePool() {
  return JSON.parse(JSON.stringify(configuredPool))
}

async function mountEditor() {
  const wrapper = mount(BlindboxPoolEditor, {
    global: {
      stubs: {
        Icon: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('BlindboxPoolEditor', () => {
  beforeEach(() => {
    getBlindboxPoolMock.mockReset()
    updateBlindboxPoolMock.mockReset()
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
    getBlindboxPoolMock.mockResolvedValue(clonePool())
    updateBlindboxPoolMock.mockImplementation(async (pool) => pool)
  })

  it('loads the live pool and shows its calculated totals', async () => {
    const wrapper = await mountEditor()

    expect(getBlindboxPoolMock).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="pool-version"]').element).toHaveProperty('value', 'season-1-v1')
    expect(wrapper.get('[data-testid="total-weight"]').text()).toContain('10000')
    expect(wrapper.get('[data-testid="expected-reward"]').text()).toContain('$0.45')
    expect(wrapper.get('[data-testid="effective-rtp"]').text()).toContain('90.00%')
    expect(wrapper.findAll('[data-testid="tier-row"]')).toHaveLength(7)
  })

  it('rejects total weight 9999 and does not save', async () => {
    const wrapper = await mountEditor()
    const firstWeight = wrapper.get('[data-testid="tier-weight-0"]')

    await firstWeight.setValue('3999')

    expect(wrapper.get('[data-testid="total-weight"]').text()).toContain('9999')
    expect(wrapper.get('[data-testid="save-pool"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="save-pool"]').trigger('click')
    expect(updateBlindboxPoolMock).not.toHaveBeenCalled()
  })

  it('rejects an RTP above the cap and does not save', async () => {
    const wrapper = await mountEditor()

    await wrapper.get('[data-testid="tier-amount-6"]').setValue('21')

    expect(wrapper.get('[data-testid="effective-rtp"]').text()).toContain('90.20%')
    expect(wrapper.get('[data-testid="save-pool"]').attributes('disabled')).toBeDefined()
    expect(updateBlindboxPoolMock).not.toHaveBeenCalled()
  })

  it('uses the backend decimal boundary when validating RTP', async () => {
    getBlindboxPoolMock.mockResolvedValue({
      version: 'decimal-boundary',
      cost: 1,
      rtp_cap: 0.1,
      tiers: [{ amount: 0.1000000000000001, weight: 10000 }],
    })
    const wrapper = await mountEditor()

    expect(wrapper.get('[data-testid="save-pool"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('有效 RTP 不能超过 RTP 上限')
    await wrapper.get('[data-testid="save-pool"]').trigger('click')
    expect(updateBlindboxPoolMock).not.toHaveBeenCalled()
  })

  it('rejects a value rounded above the cap by shopspring decimal division', async () => {
    getBlindboxPoolMock.mockResolvedValue({
      version: 'shopspring-round-up',
      cost: 1,
      rtp_cap: 0.12345678901234566,
      tiers: [{ amount: 0.12345678901234566, weight: 10000 }],
    })
    const wrapper = await mountEditor()

    expect(wrapper.get('[data-testid="save-pool"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('有效 RTP 不能超过 RTP 上限')
  })

  it('accepts a value rounded down to the cap by shopspring decimal division', async () => {
    getBlindboxPoolMock.mockResolvedValue({
      version: 'shopspring-round-down',
      cost: 1,
      rtp_cap: 0.1,
      tiers: [{ amount: 0.10000000000000003, weight: 10000 }],
    })
    const wrapper = await mountEditor()

    expect(wrapper.get('[data-testid="save-pool"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="expected-reward"]').text()).toContain('$0.1')
    expect(wrapper.get('[data-testid="effective-rtp"]').text()).toContain('10.00%')
  })

  it('preserves small tier contributions when other rewards are very large', async () => {
    getBlindboxPoolMock.mockResolvedValue({
      version: 'wide-magnitude-range',
      cost: 1e18,
      rtp_cap: 1,
      tiers: [
        { amount: 2e18, weight: 5000 },
        { amount: 0.01, weight: 5000 },
      ],
    })
    const wrapper = await mountEditor()

    expect(wrapper.get('[data-testid="save-pool"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('有效 RTP 不能超过 RTP 上限')
  })

  it('gives every tier input an accessible row-specific name', async () => {
    const wrapper = await mountEditor()

    expect(wrapper.get('[data-testid="tier-amount-0"]').attributes('aria-label')).toBe('档位 1 奖励金额')
    expect(wrapper.get('[data-testid="tier-weight-0"]').attributes('aria-label')).toBe('档位 1 权重')
    expect(wrapper.get('[data-testid="tier-amount-6"]').attributes('aria-label')).toBe('档位 7 奖励金额')
    expect(wrapper.get('[data-testid="tier-weight-6"]').attributes('aria-label')).toBe('档位 7 权重')
  })

  it('prevents removing the only tier', async () => {
    getBlindboxPoolMock.mockResolvedValue({
      version: 'single-tier',
      cost: 1,
      rtp_cap: 1,
      tiers: [{ amount: 1, weight: 10000 }],
    })
    const wrapper = await mountEditor()

    const removeButton = wrapper.get('[data-testid="remove-tier-0"]')
    expect(removeButton.attributes('disabled')).toBeDefined()
    await removeButton.trigger('click')
    expect(wrapper.findAll('[data-testid="tier-row"]')).toHaveLength(1)
  })

  it('prevents adding a thirty-third tier', async () => {
    getBlindboxPoolMock.mockResolvedValue({
      version: 'max-tiers',
      cost: 1,
      rtp_cap: 1,
      tiers: Array.from({ length: 32 }, (_, index) => ({
        amount: 0,
        weight: index === 31 ? 9969 : 1,
      })),
    })
    const wrapper = await mountEditor()

    const addButton = wrapper.get('[data-testid="add-tier"]')
    expect(addButton.attributes('disabled')).toBeDefined()
    await addButton.trigger('click')
    expect(wrapper.findAll('[data-testid="tier-row"]')).toHaveLength(32)
  })

  it('supports adding and removing tiers within the allowed range', async () => {
    const wrapper = await mountEditor()

    await wrapper.get('[data-testid="add-tier"]').trigger('click')
    expect(wrapper.findAll('[data-testid="tier-row"]')).toHaveLength(8)

    await wrapper.get('[data-testid="remove-tier-7"]').trigger('click')
    expect(wrapper.findAll('[data-testid="tier-row"]')).toHaveLength(7)
  })

  it('announces load failures to assistive technology', async () => {
    getBlindboxPoolMock.mockRejectedValue(new Error('network unavailable'))
    const wrapper = await mountEditor()

    const alert = wrapper.get('[role="alert"]')
    expect(alert.attributes('aria-live')).toBe('polite')
    expect(alert.text()).toContain('network unavailable')
  })

  it('saves a valid seven-tier pool through the admin PUT API', async () => {
    const wrapper = await mountEditor()

    await wrapper.get('[data-testid="pool-version"]').setValue('season-2-v1')
    await wrapper.get('[data-testid="save-pool"]').trigger('click')
    await flushPromises()

    expect(updateBlindboxPoolMock).toHaveBeenCalledTimes(1)
    expect(updateBlindboxPoolMock).toHaveBeenCalledWith({
      ...configuredPool,
      version: 'season-2-v1',
    })
    expect(showSuccessMock).toHaveBeenCalledTimes(1)
  })
})
