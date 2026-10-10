import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

import AccountsView from '../AccountsView.vue'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'

const {
  listAccounts,
  listWithEtag,
  getById,
  getBatchTodayStats,
  getBatchUsage,
  getUpstreamBillingProbeSettings,
  getAllProxies,
  getAllGroups,
  refreshCredentials,
  loadEditModule,
  recoverChunk,
  onComponentError,
  showError,
  showWarning
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getById: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getBatchUsage: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  refreshCredentials: vi.fn(),
  loadEditModule: vi.fn(),
  recoverChunk: vi.fn(),
  onComponentError: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn()
}))

vi.mock('@/router/chunkRecovery', () => ({ recoverFromChunkLoadError: recoverChunk }))

vi.mock('vue', async () => {
  const actual = await vi.importActual<typeof import('vue')>('vue')
  return {
    ...actual,
    defineAsyncComponent: (source: Parameters<typeof actual.defineAsyncComponent>[0]) => {
      const options = typeof source === 'function' ? { loader: source } : source
      return actual.defineAsyncComponent(options.loader.toString().includes('/EditAccountModal.vue')
        ? { ...options, loader: loadEditModule }
        : options)
    }
  }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      getById,
      listWithEtag,
      getBatchTodayStats,
      getBatchUsage,
      getUpstreamBillingProbeSettings,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn(),
      refreshCredentials
    },
    proxies: { getAll: getAllProxies },
    groups: { getAll: getAllGroups }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess: vi.fn(), showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: `
    <div>
      <div v-for="row in data" :key="row.id" :data-account-name="row.name">
        <slot name="cell-groups" :row="row" />
        <slot name="cell-usage" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
})

const AccountUsageCellStub = defineComponent({
  props: ['account', 'requestBatchedUsage', 'batchedUsage'],
  template: '<span data-test="account-usage">{{ batchedUsage?.five_hour?.utilization }}</span>'
})

const AccountGroupsCellStub = defineComponent({
  props: { groups: { type: Array, default: () => [] } },
  template: '<span data-test="account-groups">{{ groups.map(group => group.name).join(",") }}</span>'
})

const EditAccountModalStub = defineComponent({
  props: { show: Boolean, account: { type: Object, default: null } },
  template: '<div data-test="edit-account">{{ show ? account?.name : "" }}</div>'
})

const AccountTestModalStub = defineComponent({
  props: { show: Boolean, account: { type: Object, default: null } },
  template: '<div data-test="test-account">{{ show ? account?.name : "" }}</div>'
})

const AccountStatsModalStub = defineComponent({
  props: { show: Boolean, account: { type: Object, default: null } },
  template: '<div data-test="stats-account">{{ show ? account?.name : "" }}</div>'
})

function mountView(stubActionMenu = true, stubEditModal = true) {
  return mount(AccountsView, {
    attachTo: document.body,
    global: {
      config: stubEditModal ? {} : { errorHandler: onComponentError },
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        DataTable: DataTableStub,
        AccountTableActions: { template: '<div><slot name="after" /></div>' },
        AccountTableFilters: true,
        AccountBulkActionsBar: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountActionMenu: stubActionMenu,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: AccountTestModalStub,
        AccountStatsModal: AccountStatsModalStub,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: stubEditModal ? EditAccountModalStub : false,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: AccountGroupsCellStub,
        AccountUsageCell: AccountUsageCellStub,
        UpstreamBillingRateCell: true,
        HelpTooltip: true,
        Icon: true,
        Teleport: stubActionMenu
      }
    }
  })
}

const listRow = {
  id: 42,
  name: 'compact row',
  platform: 'openai',
  type: 'oauth',
  status: 'active',
  schedulable: true,
  concurrency: 2,
  priority: 1,
  group_ids: [7],
  extra: {},
  credentials: {}
}

const fullAccount = {
  ...listRow,
  groups: [{ id: 7, name: 'codex', platform: 'openai' }],
  account_groups: [{ account_id: 42, group_id: 7 }],
  credentials: { api_key: 'redacted' },
  extra: { detail_only: true }
}

describe('admin AccountsView lite account list', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => ({
      matches: true, media: query, onchange: null,
      addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(),
      removeEventListener: vi.fn(), dispatchEvent: vi.fn()
    }))
    getBatchUsage.mockReset().mockResolvedValue({ usage: { '42': { five_hour: { utilization: 12 } } } })
    listAccounts.mockReset().mockResolvedValue({ items: [listRow], total: 1, page: 1, page_size: 20, pages: 1 })
    listWithEtag.mockReset().mockResolvedValue({ notModified: true, etag: 'compact-etag', data: null })
    getById.mockReset().mockResolvedValue(fullAccount)
    getBatchTodayStats.mockReset().mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockReset().mockResolvedValue({ enabled: true })
    getAllProxies.mockReset().mockResolvedValue([])
    getAllGroups.mockReset().mockResolvedValue([{ id: 7, name: 'codex', platform: 'openai' }])
    refreshCredentials.mockReset()
    loadEditModule.mockReset().mockResolvedValue(EditAccountModalStub)
    recoverChunk.mockReset().mockReturnValue(false)
    onComponentError.mockReset()
    showError.mockReset()
    showWarning.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('reads a changed snapshot past the page cache without forcing the upstream', async () => {
    vi.useFakeTimers()
    const wrapper = mountView()
    await flushPromises()
    const cell = wrapper.findComponent(AccountUsageCellStub)
    const request = cell.props('requestBatchedUsage')
    request(listRow)
    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(cell.props('batchedUsage').five_hour.utilization).toBe(12)
    expect(getBatchUsage).toHaveBeenCalledTimes(1)

    request(listRow)
    await vi.advanceTimersByTimeAsync(1)
    expect(getBatchUsage).toHaveBeenCalledTimes(1)

    getBatchUsage.mockResolvedValue({ usage: { '42': { five_hour: { utilization: 37.5 } } } })
    const updated = { ...listRow, extra: { codex_usage_updated_at: '2026-10-10T12:00:00Z' } }
    request(updated, { bypassCache: true })
    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(getBatchUsage).toHaveBeenCalledTimes(2)
    expect(getBatchUsage).toHaveBeenLastCalledWith([42], false)
    expect(cell.props('batchedUsage').five_hour.utilization).toBe(37.5)
    wrapper.unmount()
  })

  it('keeps lite=1 on the initial list request', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({ lite: '1' }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    wrapper.unmount()
  })

  it('maps group_ids through the group catalog for the table cell', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="account-groups"]').text()).toBe('codex')
    wrapper.unmount()
  })

  it('keeps the action menu open during internal scrolling but closes it on table scrolling', async () => {
    const wrapper = mountView(false)
    await flushPromises()

    const trigger = wrapper.findAll('button').find(button => button.text() === 'common.more')!
    await trigger.trigger('click')
    const menu = new DOMWrapper(document.body.querySelector('.action-menu-content')!)
    menu.element.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(wrapper.findComponent(AccountActionMenu).props('show')).toBe(true)

    menu.get('button').element.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(wrapper.findComponent(AccountActionMenu).props('show')).toBe(true)

    wrapper.getComponent(DataTableStub).element.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(wrapper.findComponent(AccountActionMenu).props('show')).toBe(false)
    wrapper.unmount()
  })

  it('keeps lite=1 on automatic ETag refreshes', async () => {
    vi.useFakeTimers()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    const wrapper = mountView()
    await flushPromises()

    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()

    expect(listWithEtag).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({ lite: '1' }),
      expect.objectContaining({ etag: null })
    )
    wrapper.unmount()
  })

  it('loads the full account by id before opening edit, test, and stats actions', async () => {
    const wrapper = mountView()
    await flushPromises()

    const editButton = wrapper.findAll('button').find(button => button.text().includes('common.edit'))
    expect(editButton).toBeTruthy()
    await editButton!.trigger('click')
    await flushPromises()
    expect(getById).toHaveBeenCalledWith(42)
    expect(wrapper.get('[data-test="edit-account"]').text()).toBe('compact row')

    const menu = wrapper.findComponent(AccountActionMenu)
    menu.vm.$emit('test', listRow)
    await flushPromises()
    expect(getById).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-test="test-account"]').text()).toBe('compact row')

    menu.vm.$emit('stats', listRow)
    await flushPromises()
    expect(getById).toHaveBeenCalledTimes(3)
    expect(wrapper.get('[data-test="stats-account"]').text()).toBe('compact row')
    wrapper.unmount()
  })

  it('shows the warning and patches the account after a partial Antigravity refresh', async () => {
    refreshCredentials.mockResolvedValue({
      account: { ...fullAccount, name: 'refreshed account' },
      message: 'Token refreshed, but project_id is temporarily unavailable',
      warning: 'missing_project_id_temporary'
    })
    const wrapper = mountView(false)
    await flushPromises()

    wrapper.findComponent(AccountActionMenu).vm.$emit('refresh-token', listRow)
    await flushPromises()

    expect(refreshCredentials).toHaveBeenCalledWith(42)
    expect(wrapper.get('[data-account-name]').attributes('data-account-name')).toBe('refreshed account')
    expect(showWarning).toHaveBeenCalledWith('Token refreshed, but project_id is temporarily unavailable')
    wrapper.unmount()
  })

  it('shows an error and keeps the modal closed when detail loading fails', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    getById.mockRejectedValueOnce(new Error('detail failed'))
    const wrapper = mountView()
    await flushPromises()

    const editButton = wrapper.findAll('button').find(button => button.text().includes('common.edit'))
    await editButton!.trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('detail failed')
    expect(wrapper.find('[data-test="edit-account"]').exists()).toBe(false)
    consoleError.mockRestore()
    wrapper.unmount()
  })

  it('reports an async edit module failure and lets the next click load it again', async () => {
    const failure = new Error('Failed to fetch dynamically imported module')
    loadEditModule.mockRejectedValueOnce(failure)
    const wrapper = mountView(true, false)
    await flushPromises()
    const editButton = wrapper.findAll('button').find(button => button.text().includes('common.edit'))!
    await editButton.trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('admin.accounts.editLoadFailed')
    expect(recoverChunk).toHaveBeenCalledWith(failure, undefined)
    expect(onComponentError).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="edit-account"]').exists()).toBe(false)

    await editButton.trigger('click')
    await flushPromises()
    expect(getById).toHaveBeenCalledTimes(2)
    expect(loadEditModule).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-test="edit-account"]').text()).toBe('compact row')
    wrapper.unmount()
  })

  it('does not recover a late edit module failure after leaving the accounts page', async () => {
    let rejectModule!: (error: Error) => void
    loadEditModule.mockImplementationOnce(() => new Promise((_resolve, reject) => {
      rejectModule = reject
    }))
    const wrapper = mountView(true, false)
    await flushPromises()
    const editButton = wrapper.findAll('button').find(button => button.text().includes('common.edit'))!
    await editButton.trigger('click')
    await flushPromises()
    expect(loadEditModule).toHaveBeenCalledOnce()
    wrapper.unmount()

    rejectModule(new Error('Failed to fetch dynamically imported module'))
    await flushPromises()
    expect(showError).not.toHaveBeenCalled()
    expect(recoverChunk).not.toHaveBeenCalled()
    expect(onComponentError).not.toHaveBeenCalled()
  })

  it.each([false, true])('ignores an earlier account detail response after selecting another account (closed=%s)', async (closed) => {
    let resolveFirst!: (account: typeof fullAccount) => void
    const firstRequest = new Promise<typeof fullAccount>(resolve => { resolveFirst = resolve })
    const second = { ...fullAccount, id: 43, name: 'second account' }
    listAccounts.mockResolvedValue({ items: [listRow, second], total: 2, page: 1, page_size: 20, pages: 1 })
    getById.mockImplementation((id: number) => id === 42 ? firstRequest : Promise.resolve(second))
    const wrapper = mountView()
    await flushPromises()
    const editButtons = wrapper.findAll('button').filter(button => button.text().includes('common.edit'))
    await editButtons[0].trigger('click')
    await editButtons[1].trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="edit-account"]').text()).toBe('second account')
    if (closed) {
      wrapper.findComponent(EditAccountModalStub).vm.$emit('close')
      await flushPromises()
    }

    resolveFirst(fullAccount)
    await flushPromises()
    if (closed) expect(wrapper.find('[data-test="edit-account"]').exists()).toBe(false)
    else expect(wrapper.get('[data-test="edit-account"]').text()).toBe('second account')
    wrapper.unmount()
  })

  it('keeps the latest selection when returning to an account with an earlier detail request pending', async () => {
    let resolveFirst!: (account: typeof fullAccount) => void
    let resolveSecond!: (account: typeof fullAccount) => void
    let resolveLatest!: (account: typeof fullAccount) => void
    const firstRequest = new Promise<typeof fullAccount>(resolve => { resolveFirst = resolve })
    const secondRequest = new Promise<typeof fullAccount>(resolve => { resolveSecond = resolve })
    const latestRequest = new Promise<typeof fullAccount>(resolve => { resolveLatest = resolve })
    const second = { ...fullAccount, id: 43, name: 'second account' }
    const latest = { ...fullAccount, name: 'latest compact row' }
    listAccounts.mockResolvedValue({ items: [listRow, second], total: 2, page: 1, page_size: 20, pages: 1 })
    getById.mockReturnValueOnce(firstRequest).mockReturnValueOnce(secondRequest).mockReturnValueOnce(latestRequest)
    const wrapper = mountView()
    await flushPromises()
    const editButtons = wrapper.findAll('button').filter(button => button.text().includes('common.edit'))
    await editButtons[0].trigger('click')
    await editButtons[1].trigger('click')
    await editButtons[0].trigger('click')
    expect(getById.mock.calls.map(([id]) => id)).toEqual([42, 43, 42])
    resolveLatest(latest)
    await flushPromises()
    expect(wrapper.get('[data-test="edit-account"]').text()).toBe('latest compact row')
    resolveFirst(fullAccount)
    await flushPromises()
    expect(wrapper.get('[data-test="edit-account"]').text()).toBe('latest compact row')
    resolveSecond(second)
    await flushPromises()
    expect(wrapper.get('[data-test="edit-account"]').text()).toBe('latest compact row')
    wrapper.unmount()
  })

  it('reuses a pending detail request for repeated clicks on the same account', async () => {
    let resolveDetail!: (account: typeof fullAccount) => void
    getById.mockReturnValueOnce(new Promise<typeof fullAccount>(resolve => { resolveDetail = resolve }))
    const wrapper = mountView()
    await flushPromises()
    const editButton = wrapper.findAll('button').find(button => button.text().includes('common.edit'))!
    await editButton.trigger('click')
    await editButton.trigger('click')
    expect(getById).toHaveBeenCalledOnce()
    resolveDetail(fullAccount)
    await flushPromises()
    expect(wrapper.get('[data-test="edit-account"]').text()).toBe('compact row')
    wrapper.unmount()
  })

  it.each([false, true])('ignores late detail errors after closing or leaving the page (unmounted=%s)', async (unmounted) => {
    let rejectFirst!: (error: Error) => void
    const firstRequest = new Promise<typeof fullAccount>((_resolve, reject) => { rejectFirst = reject })
    const second = { ...fullAccount, id: 43, name: 'second account' }
    listAccounts.mockResolvedValue({ items: [listRow, second], total: 2, page: 1, page_size: 20, pages: 1 })
    getById.mockReturnValueOnce(firstRequest).mockResolvedValueOnce(second)
    const wrapper = mountView()
    await flushPromises()
    const editButtons = wrapper.findAll('button').filter(button => button.text().includes('common.edit'))
    await editButtons[0].trigger('click')
    await editButtons[1].trigger('click')
    await flushPromises()
    if (unmounted) wrapper.unmount()
    else wrapper.findComponent(EditAccountModalStub).vm.$emit('close')
    rejectFirst(new Error('stale detail failed'))
    await flushPromises()
    expect(showError).not.toHaveBeenCalled()
    if (!unmounted) {
      expect(wrapper.find('[data-test="edit-account"]').exists()).toBe(false)
      wrapper.unmount()
    }
  })
})
