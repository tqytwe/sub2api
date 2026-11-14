import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const { listAccounts, listWithEtag, getBillingRates, probeBilling, getTodayStats, getProbeSettings, getProxies, getGroups, syncModels } = vi.hoisted(() => ({
  listAccounts: vi.fn(), syncModels: vi.fn(),
  listWithEtag: vi.fn(),
  getBillingRates: vi.fn(), probeBilling: vi.fn(), getTodayStats: vi.fn(), getProbeSettings: vi.fn(),
  getProxies: vi.fn(), getGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts, syncUpstreamModels: syncModels,
      listWithEtag,
      getUpstreamBillingRatesWithEtag: getBillingRates,
      probeUpstreamBilling: probeBilling,
      getBatchTodayStats: getTodayStats,
      getUpstreamBillingProbeSettings: getProbeSettings
    },
    proxies: { getAll: getProxies },
    groups: { getAll: getGroups }
  }
}))
vi.mock('@/components/account/AccountCapacityCell.vue', () => ({ default: { template: '<span>capacity</span>' } }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false, isAdmin: true })
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const rows = [
  { id: 12, name: 'Busy account', current_concurrency: 7, concurrency: 30 },
  { id: 15, name: 'Small account', current_concurrency: 2, concurrency: 2 }
].map(row => ({ ...row, platform: 'openai', type: 'apikey', status: 'active', schedulable: true, extra: {}, credentials: {} }))
const page = (items = rows) => ({ items, total: items.length, page: 1, page_size: 20, pages: 1 })
const wrappers: VueWrapper[] = []

function mountView() {
  const wrapper = mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        AccountTableActions: { template: '<div><button data-test="manual-refresh" @click="$emit(\'refresh\')">Refresh</button><slot name="after" /></div>' },
        AccountTableFilters: { template: '<button data-test="change-filter" @click="$emit(\'update:filters\', { search: \'changed\' }); $emit(\'change\')">Filter</button>' },
        AccountBulkActionsBar: true,
        Pagination: { props: ['page', 'total', 'pageSize'], template: '<div><span data-test="pagination-total">{{ total }}</span><button data-test="next-page" @click="$emit(\'update:page\', 2)">Next</button></div>' },
        ConfirmDialog: true, AccountActionMenu: true, PlatformTypeBadge: true,
        AccountStatusIndicator: true, AccountTodayStatsCell: true, AccountGroupsCell: true, SchedulableTimerControl: true,
        AccountUsageCell: true,
        UpstreamBillingRateCell: { template: '<button data-test="probe-billing" @click="$emit(\'probe\')">Probe</button>' },
        AccountPriorityCell: true,
        TotpStepUpDialog: true
      }
    }
  })
  wrappers.push(wrapper)
  return wrapper
}

describe('admin account upstream model auto sync', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => ({
      matches: true, media: query, onchange: null, addListener: vi.fn(), removeListener: vi.fn(),
      addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn()
    }))
    listAccounts.mockReset().mockResolvedValue(page())
    syncModels.mockReset().mockResolvedValue({ models: ['new'], changed: false })
    probeBilling.mockReset().mockResolvedValue({ snapshot: { status: 'ok', effective_rate: 1 } })
    getTodayStats.mockReset().mockResolvedValue({ stats: {} })
    getProbeSettings.mockReset().mockResolvedValue({ enabled: true })
    getProxies.mockReset().mockResolvedValue([])
    getGroups.mockReset().mockResolvedValue([])
    listWithEtag.mockReset().mockResolvedValue({ notModified: true, etag: 'accounts-etag', data: null })
    getBillingRates.mockReset().mockResolvedValue({
      notModified: false,
      etag: 'billing-etag',
      data: { items: rows.map(row => ({ account_id: row.id, snapshot: null })), total: 2 }
    })
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.useRealTimers()
    vi.restoreAllMocks()
  })


  it('adds opt-in account-type controls to the existing refresh menu and persists them', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[title="admin.accounts.autoRefresh"]').trigger('click')
    const input = wrapper.get('[data-testid="auto-refresh-upstream-models-apikey"]')
    expect((input.element as HTMLInputElement).checked).toBe(false)
    await input.setValue(true)
    expect(JSON.parse(localStorage.getItem('account-auto-refresh')!).upstream_models).toEqual({ oauth: false, apikey: true })
    expect(syncModels).not.toHaveBeenCalled()
  })

  it('applies eligible current-page models through the real refresh loop and cancels on unmount', async () => {
    localStorage.setItem('account-auto-refresh', JSON.stringify({enabled:true, interval_seconds:30, upstream_models:{oauth:false,apikey:true}}))
    const resolvers: Array<(value: {models:string[];changed:boolean}) => void> = []
    syncModels.mockImplementation(() => new Promise(resolve => { resolvers.push(resolve) }))
    const wrapper = mountView()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)
    expect(syncModels).toHaveBeenCalledTimes(2)
    expect(syncModels).toHaveBeenCalledWith(12, {apply:true, automatic:true}, {signal:expect.any(AbortSignal)})
    const signal = syncModels.mock.calls[0][2].signal
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
    resolvers.forEach(resolve => resolve({models:['new'],changed:true}))
    await flushPromises()
    expect(listWithEtag).not.toHaveBeenCalled()
  })
  it('reloads account state after an authoritative catalog changed the stored mapping', async () => {
    localStorage.setItem('account-auto-refresh', JSON.stringify({enabled:true, interval_seconds:30, upstream_models:{oauth:false,apikey:true}}))
    syncModels.mockResolvedValue({models:['new'],applied:true,changed:true})
    mountView()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()
    expect(syncModels).toHaveBeenCalledTimes(2)
    expect(listWithEtag).toHaveBeenCalledTimes(1)
    expect(listWithEtag).toHaveBeenLastCalledWith(1,20,expect.anything(),{etag:null})
  })

})
