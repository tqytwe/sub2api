import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'
import AccountCapacityCell from '@/components/account/AccountCapacityCell.vue'

const { listAccounts, listWithEtag, getBillingRates, probeBilling, getTodayStats, getProbeSettings, getProxies, getGroups } = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBillingRates: vi.fn(), probeBilling: vi.fn(), getTodayStats: vi.fn(), getProbeSettings: vi.fn(),
  getProxies: vi.fn(), getGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
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
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false })
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
        AccountStatusIndicator: true, AccountTodayStatsCell: true, AccountGroupsCell: true,
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

const capacityHeader = (wrapper: VueWrapper) => wrapper.findAll('th').find(th => th.text().includes('admin.accounts.columns.capacity'))!
const names = (wrapper: VueWrapper) => wrapper.findAll('tbody tr').map(row => row.text())

describe('admin account current-concurrency ordering', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => ({
      matches: true, media: query, onchange: null, addListener: vi.fn(), removeListener: vi.fn(),
      addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn()
    }))
    listAccounts.mockReset().mockResolvedValue(page())
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

  it('sorts capacity by the server current count and persists the table column, not its API alias', async () => {
    const wrapper = mountView()
    await flushPromises()
    const header = capacityHeader(wrapper)
    expect(header.find('button').exists()).toBe(true)
    await header.get('button').trigger('click')
    await flushPromises()
    expect(header.attributes('aria-sort')).toBe('ascending')
    expect(listAccounts).toHaveBeenLastCalledWith(1, 20,
      expect.objectContaining({ lite: '1', sort_by: 'current_concurrency', sort_order: 'asc' }),
      expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(JSON.parse(localStorage.getItem('account-table-sort')!)).toEqual({ key: 'capacity', order: 'asc' })
    await header.get('button').trigger('click')
    await flushPromises()
    expect(header.attributes('aria-sort')).toBe('descending')
    expect(listAccounts).toHaveBeenLastCalledWith(1, 20,
      expect.objectContaining({ sort_by: 'current_concurrency', sort_order: 'desc' }), expect.anything())
  })

  it('restores persisted capacity before the first list request and retains its sort indicator', async () => {
    localStorage.setItem('account-table-sort', JSON.stringify({ key: 'capacity', order: 'desc' }))
    const wrapper = mountView()
    await flushPromises()
    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({ sort_by: 'current_concurrency', sort_order: 'desc' }))
    expect(capacityHeader(wrapper).attributes('aria-sort')).toBe('descending')
  })

  it('keeps invalid persisted API aliases out of the table whitelist', async () => {
    localStorage.setItem('account-table-sort', JSON.stringify({ key: 'current_concurrency', order: 'desc' }))
    mountView()
    await flushPromises()
    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({ sort_by: 'name', sort_order: 'asc' }))
  })

  it('applies fresh server row order and counts together during automatic refresh', async () => {
    localStorage.setItem('account-table-sort', JSON.stringify({ key: 'capacity', order: 'desc' }))
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    const wrapper = mountView()
    await flushPromises()
    const freshRows = [{ ...rows[1], current_concurrency: 9 }, { ...rows[0], current_concurrency: 1 }]
    listWithEtag.mockResolvedValueOnce({ notModified: false, etag: 'new-counts', data: page(freshRows) })
    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()
    expect(listWithEtag).toHaveBeenLastCalledWith(1, 20,
      expect.objectContaining({ sort_by: 'current_concurrency', sort_order: 'desc', lite: '1' }), expect.anything())
    expect(names(wrapper)[0]).toContain('Small account')
    expect(wrapper.findAllComponents(AccountCapacityCell)[0].findAll('.font-mono').map(cell => cell.text())).toEqual(['9', '2'])
  })

  it('refreshes current counts when a billing probe snapshot retains the same IDs and order', async () => {
    localStorage.setItem('account-table-sort', JSON.stringify({ key: 'capacity', order: 'desc' }))
    const wrapper = mountView()
    await flushPromises()
    const callsBeforeRefresh = listAccounts.mock.calls.length
    listAccounts.mockResolvedValueOnce(page(rows.map(row => ({ ...row, current_concurrency: row.id === 12 ? 8 : 3 }))))
    await wrapper.findAll('[data-test="probe-billing"]')[0].trigger('click')
    await flushPromises()
    expect(getBillingRates).toHaveBeenCalled()
    expect(listAccounts.mock.calls.length).toBe(callsBeforeRefresh + 1)
    expect(wrapper.findAllComponents(AccountCapacityCell)[0].findAll('.font-mono').map(cell => cell.text())).toEqual(['8', '30'])
  })

  it('retains the displayed list when automatic current-count refresh fails', async () => {
    localStorage.setItem('account-table-sort', JSON.stringify({ key: 'capacity', order: 'desc' }))
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    const wrapper = mountView()
    await flushPromises()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    listWithEtag.mockRejectedValueOnce(new Error('Count snapshot unavailable'))
    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()
    expect(names(wrapper)[0]).toContain('Busy account')
    expect(names(wrapper)[1]).toContain('Small account')
  })

  it.each(['sort', 'filter', 'page', 'same-condition reload'])(
    'ignores a late automatic poll after a newer %s list has completed, including its ETag',
    async (change) => {
      localStorage.setItem('account-table-sort', JSON.stringify({ key: 'capacity', order: 'desc' }))
      localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
      listAccounts.mockResolvedValueOnce({ ...page(), total: 3, pages: 2 })
      const wrapper = mountView()
      await flushPromises()
      let resolveOld!: (value: unknown) => void
      listWithEtag.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
      await vi.advanceTimersByTimeAsync(6000)
      await flushPromises()
      expect(listWithEtag).toHaveBeenCalledTimes(1)

      const freshRows = [{ ...rows[0], name: 'New condition account', current_concurrency: 10 }]
      listAccounts.mockResolvedValueOnce({ ...page(freshRows), total: 33, pages: 3 })
      if (change === 'sort') await capacityHeader(wrapper).get('button').trigger('click')
      else if (change === 'filter') {
        await wrapper.get('[data-test="change-filter"]').trigger('click')
        await vi.advanceTimersByTimeAsync(300)
      } else if (change === 'page') await wrapper.get('[data-test="next-page"]').trigger('click')
      else await wrapper.get('[data-test="manual-refresh"]').trigger('click')
      await flushPromises()
      expect(names(wrapper)[0]).toContain('New condition account')

      resolveOld({ notModified: false, etag: 'obsolete-poll-etag', data: { ...page(), total: 90, pages: 5 } })
      await flushPromises()
      expect(names(wrapper)).toHaveLength(1)
      expect(names(wrapper)[0]).toContain('New condition account')
      expect(wrapper.get('[data-test="pagination-total"]').text()).toBe('33')

      await vi.advanceTimersByTimeAsync(6000)
      await flushPromises()
      expect(listWithEtag.mock.calls[1]?.[3]).toEqual(expect.objectContaining({ etag: null }))
    }
  )
})
