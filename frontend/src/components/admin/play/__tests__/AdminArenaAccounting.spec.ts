import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import AdminArenaAccounting from '../AdminArenaAccounting.vue'
const api = vi.hoisted(() => ({ getArenaUserReport: vi.fn(), previewArenaAccounting: vi.fn(), activateArenaAccounting: vi.fn() }))
vi.mock('@/api/admin/play', () => ({ default: api }))
vi.mock('@/composables/useStepUp', () => ({ useStepUp: () => ({ run: (fn: () => unknown) => fn(), onCancel: vi.fn() }), isStepUpCancelled: () => false, isStepUpBlocked: () => false, stepUpBlockReason: () => '' }))
const preview = { period_id: 7, version: 1, status: 'active', has_payout: false, can_activate: true, digest: 'a'.repeat(32), total_users: 0, truncated: false, changes: [] }
const createView = () => mount(AdminArenaAccounting, { props: { periodId: 7 }, global: { plugins: [createI18n({ legacy: false, locale: 'zh' })], stubs: { TotpStepUpDialog: true } } })
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); api.previewArenaAccounting.mockResolvedValue(preview); api.activateArenaAccounting.mockResolvedValue({ ...preview, version: 2, can_activate: false }) })
describe('AdminArenaAccounting', () => {
  it('shows the same task and monthly token energy as the user farm', async () => {
    const tokens = { input: 10000, output: 2000, cache_creation: 1000, cache_read: 7000, total: 20000 }
    api.getArenaUserReport.mockResolvedValue({ user_id: 42, day_start: '2026-11-06T00:00:00+08:00', day_end: '2026-11-07T00:00:00+08:00', month_start: '2026-11-01T00:00:00+08:00', month_end: '2026-12-01T00:00:00+08:00', as_of: '2026-11-06T12:00:00+08:00', daily: tokens, monthly: tokens, monthly_actual_token_energy: 2, monthly_legacy_token_energy: 1, qualification_growth_energy: 9, current_farm: { enabled: true, server_date: '2026-11-06', task_energy: 10, monthly_token_energy: 2, energy: 12, tasks: [{ key: 'checkin', completed: true, energy: 10 }] } })
    const view = createView()
    await view.get('input[type="number"]').setValue(42)
    await view.findAll('form')[0].trigger('submit'); await flushPromises()
    expect(view.text()).toContain('今日任务能量: 10')
    expect(view.text()).toContain('本月计分 Token 能量: 2')
    expect(view.text()).toContain('农场当前能量: 12')
    expect(view.text()).toContain('签到 · 已完成 · 10')
  })
  it('requires preview, explanation and confirmation before activating', async () => {
    const view = createView()
    expect(view.find('textarea').exists()).toBe(false)
    const previewButton = view.findAll('button').find(button => button.text() === '预览新旧口径')!
    await previewButton.trigger('click'); await flushPromises()
    const activate = view.findAll('button').find(button => button.text() === '确认切换本周期')!
    expect(activate.attributes('disabled')).toBeDefined()
    await view.get('textarea').setValue('已对照本周期实际用量和奖励排名')
    await view.get('input[type="checkbox"]').setValue(true)
    await view.findAll('form')[1].trigger('submit'); await flushPromises()
    expect(api.activateArenaAccounting).toHaveBeenCalledWith(7, preview.digest, '已对照本周期实际用量和奖励排名', expect.any(String))
    expect(view.emitted('activated')).toHaveLength(1)
    expect(view.text()).toContain('本周期已使用四项 Token 口径')
  })
  it('has no activation control when the period has already paid rewards', async () => {
    api.previewArenaAccounting.mockResolvedValue({ ...preview, can_activate: false, has_payout: true })
    const view = createView()
    await view.findAll('button').find(button => button.text() === '预览新旧口径')!.trigger('click'); await flushPromises()
    expect(view.find('textarea').exists()).toBe(false)
    expect(view.text()).toContain('不能直接切换')
  })
  it('does not replace a newly selected period with an earlier activation response', async () => {
    let finish!: (value: unknown) => void
    api.activateArenaAccounting.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const view = createView()
    await view.findAll('button').find(button => button.text() === '预览新旧口径')!.trigger('click'); await flushPromises()
    await view.get('textarea').setValue('已对照本周期实际用量和奖励排名')
    await view.get('input[type="checkbox"]').setValue(true)
    await view.findAll('form')[1].trigger('submit'); await flushPromises()
    await view.setProps({ periodId: 8 })
    finish({ ...preview, version: 2, can_activate: false }); await flushPromises()
    expect(view.text()).not.toContain('本周期已使用四项 Token 口径')
    expect(view.emitted('activated')).toBeUndefined()
  })
})
