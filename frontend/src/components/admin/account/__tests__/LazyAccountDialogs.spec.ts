import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountStatsModal from '../AccountStatsModal.vue'
import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'

const { getStats, listByAccount, showError, showSuccess } = vi.hoisted(() => ({
  getStats: vi.fn(),
  listByAccount: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: { getStats },
    scheduledTests: { listByAccount }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const account = {
  id: 7,
  name: 'Test account',
  platform: 'openai',
  type: 'oauth',
  status: 'active'
} as any

const commonStubs = {
  BaseDialog: { template: '<div v-if="show"><slot /><slot name="footer" /></div>', props: ['show'] },
  ConfirmDialog: { template: '<div />' },
  HelpTooltip: { template: '<span><slot name="trigger" /><slot /></span>' },
  Select: { template: '<select />' },
  Input: { template: '<input />' },
  Toggle: { template: '<input type="checkbox" />' },
  Icon: true,
  Line: true,
  LoadingSpinner: true,
  ModelDistributionChart: true,
  EndpointDistributionChart: true
}

describe('lazy account dialog initialization', () => {
  beforeEach(() => {
    getStats.mockReset().mockResolvedValue(null)
    listByAccount.mockReset().mockResolvedValue([])
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('loads account statistics when first mounted visible', async () => {
    mount(AccountStatsModal, {
      props: { show: true, account },
      global: { stubs: commonStubs }
    })
    await flushPromises()

    expect(getStats).toHaveBeenCalledWith(7, 30)
  })

  it('loads scheduled test plans when first mounted visible', async () => {
    mount(ScheduledTestsPanel, {
      props: { show: true, accountId: 7, modelOptions: [] },
      global: { stubs: commonStubs }
    })
    await flushPromises()

    expect(listByAccount).toHaveBeenCalledWith(7)
  })
})
