import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import AdminCheckinRecords from '../AdminCheckinRecords.vue'
const api = vi.hoisted(() => ({ listCheckinRecords: vi.fn(), previewCheckinRepair: vi.fn(), repairCheckins: vi.fn() }))
vi.mock('@/api/admin/play', () => ({ default: api }))
vi.mock('@/composables/useStepUp', () => ({ useStepUp: () => ({ run: (fn: () => unknown) => fn(), onCancel: vi.fn() }), isStepUpCancelled: () => false, isStepUpBlocked: () => false, stepUpBlockReason: () => '' }))
vi.mock('@/components/auth/TotpStepUpDialog.vue', () => ({ default: { render: () => null } }))
enableAutoUnmount(afterEach)
const result = { items: [{ id: 1, user_id: 42, email: 'fixture@example.test', date: '2026-11-04', stored_streak: 0, observed_streak: 7, reward_amount: '0.00000000', ledger_amount: '0.00000000', growth_energy: 1, reward_mode: 'energy', eligibility_reason: 'no_recent_usage', snapshot_id: 901, evidence_status: 'streak_mismatch', created_at: '2026-11-04T00:00:00Z' }], total: 21, page: 1, page_size: 20, start_date: '2026-10-06', end_date: '2026-11-04', as_of: '2026-11-04T02:00:00Z' }
function createView() { return mount(AdminCheckinRecords, { global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] } }) }
beforeEach(() => { vi.clearAllMocks(); api.listCheckinRecords.mockResolvedValue(result) })
describe('AdminCheckinRecords', () => {
  it('distinguishes no-reward participation from historical energy grants', async () => {
    api.listCheckinRecords.mockResolvedValue({ ...result, items: [
      { ...result.items[0], id: 2, growth_energy: 0, evidence_status: 'recorded' },
      { ...result.items[0], id: 3, growth_energy: 1, evidence_status: 'recorded' },
    ] })
    const view = createView(); await flushPromises()
    const rows = view.findAll('tbody tr')
    expect(rows[0].text()).toContain('仅签到（未达领奖资格）')
    expect(rows[0].text()).not.toContain('历史成长能量')
    expect(rows[1].text()).toContain('历史成长能量')
  })

  it('loads persisted reward evidence and preserves the as-of fence while paging', async () => {
    const view = createView(); await flushPromises()
    expect(view.text()).toContain('fixture@example.test')
    expect(view.text()).toContain('连续天数待核对')
    await view.get('[data-testid="checkin-next"]').trigger('click'); await flushPromises()
    expect(api.listCheckinRecords).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2, as_of: result.as_of }))
  })
  it('distinguishes a failed request from an empty result and supports retry', async () => {
    api.listCheckinRecords.mockRejectedValueOnce(new Error('offline'))
    const view = createView(); await flushPromises()
    expect(view.find('[role="alert"]').exists()).toBe(true)
    expect(view.text()).not.toContain('没有符合筛选条件的签到记录')
    await view.get('[data-testid="checkin-retry"]').trigger('click'); await flushPromises()
    expect(view.find('[role="alert"]').exists()).toBe(false)
  })
  it('does not claim records are current after filters change until the query completes', async () => {
    const view = createView(); await flushPromises()
    await view.get('#checkin-user-id').setValue('55')
    await view.get('form').trigger('submit'); await flushPromises()
    expect(api.listCheckinRecords).toHaveBeenLastCalledWith(expect.objectContaining({ user_id: 55, page: 1, as_of: undefined }))
  })
  it('previews selected evidence, requires confirmation and refreshes after an audited repair', async () => {
    const preview = { target: { user_id: 42, date: '2026-11-04' }, checkin_id: 1, snapshot_id: 901, original_streak: 0, effective_streak: 0, observed_streak: 7, reward_mode: 'energy', bonus: 0, can_repair: true, status: 'ready', digest: 'a'.repeat(64) }
    api.previewCheckinRepair.mockResolvedValue(preview)
    api.repairCheckins.mockResolvedValue([preview])
    const view = createView(); await flushPromises()
    await view.get('[data-testid="checkin-select-1"]').setValue(true)
    await view.get('[data-testid="checkin-preview-repair"]').trigger('click'); await flushPromises()
    expect(api.previewCheckinRepair).toHaveBeenCalledWith(preview.target)
    expect(view.get('[data-testid="checkin-confirm-repair"]').attributes('disabled')).toBeDefined()
    await view.get('textarea').setValue('Reviewed immutable original evidence')
    await view.get('[data-testid="checkin-repair-confirmed"]').setValue(true)
    await view.get('[data-testid="checkin-repair-form"]').trigger('submit'); await flushPromises()
    expect(api.repairCheckins).toHaveBeenCalledWith([{ target: preview.target, digest: preview.digest }], 'Reviewed immutable original evidence', expect.any(String))
    expect(api.listCheckinRecords).toHaveBeenLastCalledWith(expect.objectContaining({ as_of: undefined }))
    expect(view.text()).toContain('修复已完成')
  })
  it('shows missing historical evidence without enabling a payout', async () => {
    api.previewCheckinRepair.mockResolvedValue({ target: { user_id: 42, date: '2026-11-04' }, can_repair: false, bonus: 0, status: 'historical_evidence_missing' })
    const view = createView(); await flushPromises()
    await view.get('[data-testid="checkin-select-1"]').setValue(true)
    await view.get('[data-testid="checkin-preview-repair"]').trigger('click'); await flushPromises()
    expect(view.text()).toContain('历史证据缺失')
    expect(view.find('[data-testid="checkin-repair-form"]').exists()).toBe(false)
    expect(api.repairCheckins).not.toHaveBeenCalled()
  })

})
