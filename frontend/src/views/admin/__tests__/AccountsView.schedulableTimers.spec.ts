import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import AccountsView from '../AccountsView.vue'

const { list, setSchedulable, bulkUpdate, auth } = vi.hoisted(() => ({
  list: vi.fn(), setSchedulable: vi.fn(), bulkUpdate: vi.fn(),
  auth: { token: 'synthetic-token', isSimpleMode: false, isAdmin: true }
}))
vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: { list, setSchedulable, bulkUpdate, listWithEtag: vi.fn(),
      getBatchTodayStats: vi.fn().mockResolvedValue({ stats: {} }),
      getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({ enabled: false }) },
    proxies: { getAll: vi.fn().mockResolvedValue([]) },
    groups: { getAll: vi.fn().mockResolvedValue([]) }
  }
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const TimerStub = {
  props: ['accountId', 'accountName', 'schedulable', 'switching', 'refreshKey', 'canManageTimers'],
  emits: ['executed', 'toggle'],
  template: `<div data-test="timer-entry" :data-account="accountId" :data-refresh="refreshKey" :data-schedulable="schedulable" :data-manage="canManageTimers">
    <button data-test="timer-executed" @click="$emit('executed')" />
    <button data-test="timer-toggle" @click="$emit('toggle')" />
  </div>`
}
let wrapper: VueWrapper | undefined
function render() {
  wrapper = mount(AccountsView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    TablePageLayout: { template: '<div><slot name="table" /></div>' },
    DataTable: { props: ['data'], template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-schedulable" :row="row" /></div></div>' },
    AccountBulkActionsBar: { emits: ['select-page', 'toggle-schedulable'], template: '<div><button data-test="select-page" @click="$emit(\'select-page\')" /><button data-test="bulk-disable" @click="$emit(\'toggle-schedulable\', false)" /></div>' },
    SchedulableTimerControl: TimerStub,
    AccountTableActions: true, AccountTableFilters: true, Pagination: true,
    ConfirmDialog: true, AccountActionMenu: true, ImportDataModal: true,
    ReAuthAccountModal: true, AccountTestModal: true, AccountStatsModal: true,
    ScheduledTestsPanel: true, SyncFromCrsModal: true, TempUnschedStatusModal: true,
    ErrorPassthroughRulesModal: true, TLSFingerprintProfilesModal: true,
    CreateAccountModal: true, EditAccountModal: true, BulkEditAccountModal: true,
    PlatformTypeBadge: true, AccountCapacityCell: true, AccountStatusIndicator: true,
    AccountTodayStatsCell: true, AccountGroupsCell: true, AccountUsageCell: true,
    HelpTooltip: true, Icon: true, Teleport: true
  } } })
  return wrapper
}
beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  auth.isAdmin = true
  list.mockResolvedValue({ items: [{ id: 7, name: 'Timer account', platform: 'openai', type: 'apikey', status: 'active', schedulable: true, group_ids: [], credentials: {}, extra: {} }], total: 1, page: 1, page_size: 20, pages: 1 })
  setSchedulable.mockResolvedValue({ schedulable: false })
  bulkUpdate.mockResolvedValue({ success_ids: [7], failed_ids: [] })
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.useRealTimers() })

describe('AccountsView scheduling task integration', () => {
  it('places administrator timers in the existing scheduling cell beside the manual switch', async () => {
    const view = render()
    await flushPromises()
    expect(view.get('[data-test="timer-entry"]').attributes()).toMatchObject({ 'data-account': '7', 'data-manage': 'true', 'data-schedulable': 'true' })
    expect(view.findAll('button[role="switch"]')).toHaveLength(1)
  })

  it('preserves the nonadministrator manual switch without an administrator timer entry', async () => {
    auth.isAdmin = false
    const view = render()
    await flushPromises()
    expect(view.find('[data-test="timer-entry"]').exists()).toBe(false)
    await view.get('button[role="switch"]').trigger('click')
    await flushPromises()
    expect(setSchedulable).toHaveBeenCalledWith(7, false)
  })

  it('reloads timer snapshots after a manual switch, including the dialog immediate action', async () => {
    const view = render()
    await flushPromises()
    const before = Number(view.get('[data-test="timer-entry"]').attributes('data-refresh'))
    await view.get('[data-test="timer-toggle"]').trigger('click')
    await flushPromises()
    expect(setSchedulable).toHaveBeenCalledWith(7, false)
    expect(Number(view.get('[data-test="timer-entry"]').attributes('data-refresh'))).toBe(before + 1)
    expect(view.get('[data-test="timer-entry"]').attributes('data-schedulable')).toBe('false')
  })

  it('reloads cancelled plans after a successful bulk manual switch', async () => {
    const view = render()
    await flushPromises()
    const before = Number(view.get('[data-test="timer-entry"]').attributes('data-refresh'))
    await view.get('[data-test="select-page"]').trigger('click')
    await view.get('[data-test="bulk-disable"]').trigger('click')
    await flushPromises()
    expect(bulkUpdate).toHaveBeenCalledWith([7], { schedulable: false })
    expect(Number(view.get('[data-test="timer-entry"]').attributes('data-refresh'))).toBe(before + 1)
  })

  it('coalesces simultaneous timer executions into one account reconciliation', async () => {
    const view = render()
    await flushPromises()
    vi.useFakeTimers()
    const calls = list.mock.calls.length
    await view.get('[data-test="timer-executed"]').trigger('click')
    await view.get('[data-test="timer-executed"]').trigger('click')
    await vi.advanceTimersByTimeAsync(150)
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(calls + 1)
  })

  it('cancels queued account reconciliation when leaving the page', async () => {
    const view = render()
    await flushPromises()
    vi.useFakeTimers()
    const calls = list.mock.calls.length
    await view.get('[data-test="timer-executed"]').trigger('click')
    view.unmount()
    wrapper = undefined
    await vi.advanceTimersByTimeAsync(150)
    expect(list).toHaveBeenCalledTimes(calls)
  })
})
