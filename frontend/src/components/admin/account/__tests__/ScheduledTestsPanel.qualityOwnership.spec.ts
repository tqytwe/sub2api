import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'

const api = vi.hoisted(() => ({
  listByAccount: vi.fn(),
  ownershipPreview: vi.fn(),
  reconcileOwnership: vi.fn(),
  restoreOwnership: vi.fn(),
  deleteOwnedPlan: vi.fn(),
  delete: vi.fn(),
  listResults: vi.fn()
}))
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin', () => ({ adminAPI: { scheduledTests: api } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const emptyState = {
  plan_id: 11, account_id: 7, rule_version: 'v1', fence: 'fence-empty',
  action: 'disable_scheduling', state_present: false, restore_ready: false, reconcile_ready: false
}
const ownedState = { ...emptyState, fence: 'fence-owned', state_present: true, restore_ready: true }
const priorState = { ...ownedState, fence: 'fence-prior', restore_ready: false, reconcile_ready: true }
const driftState = { ...ownedState, fence: 'fence-drift', restore_ready: false }
const plan = (quality: boolean) => ({
  id: 11, account_id: 7, model_id: 'gpt-test', cron_expression: '*/30 * * * *',
  enabled: true, max_results: 100, auto_recover: false, last_run_at: null,
  next_run_at: null, created_at: '', updated_at: '',
  pelican_config: quality ? { quality: { action: 'stop' } } : null
})

const stubs = {
  BaseDialog: { template: '<div v-if="show"><slot /></div>', props: ['show'] },
  ConfirmDialog: {
    props: ['show', 'title', 'message', 'confirmText', 'danger'],
    emits: ['confirm', 'cancel'],
    template: '<div v-if="show"><p>{{ title }}</p><p>{{ message }}</p><button data-test="confirm" @click="$emit(\'confirm\')">confirm</button></div>'
  },
  HelpTooltip: { template: '<span><slot name="trigger" /><slot /></span>' },
  Select: true, Input: true, Toggle: true, Icon: true
}

async function mountPanel(testPlan: ReturnType<typeof plan>) {
  api.listByAccount.mockResolvedValue([testPlan])
  const wrapper = mount(ScheduledTestsPanel, {
    props: { show: true, accountId: 7, modelOptions: [] },
    global: { stubs }
  })
  await flushPromises()
  return wrapper
}

describe('scheduled quality rule ownership lifecycle', () => {
  beforeEach(() => {
    Object.values(api).forEach((mock) => mock.mockReset())
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('restores only an exact currently owned state and blocks deletion', async () => {
    api.ownershipPreview.mockResolvedValue(ownedState)
    api.restoreOwnership.mockResolvedValue(emptyState)
    const wrapper = await mountPanel(plan(true))

    await wrapper.find('button[title="admin.scheduledTests.ownershipPreview"]').trigger('click')
    await flushPromises()

    expect(api.ownershipPreview).toHaveBeenCalledWith(11)
    expect(wrapper.text()).toContain('admin.scheduledTests.restoreOwnership')
    expect(wrapper.text()).not.toContain('admin.scheduledTests.reconcileOwnership')
    expect(wrapper.text()).not.toContain('admin.scheduledTests.deleteQualityRule')
    expect(api.delete).not.toHaveBeenCalled()
    await wrapper.findAll('button').find((button) => button.text() === 'admin.scheduledTests.restoreOwnership')!.trigger('click')
    await wrapper.find('[data-test="confirm"]').trigger('click')
    await flushPromises()
    expect(api.restoreOwnership).toHaveBeenCalledWith(11, 'fence-owned')
  })

  it('reconciles only when the account already has its recorded prior value', async () => {
    api.ownershipPreview.mockResolvedValue(priorState)
    api.reconcileOwnership.mockResolvedValue(emptyState)
    const wrapper = await mountPanel(plan(true))
    await wrapper.find('button[title="admin.scheduledTests.ownershipPreview"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.scheduledTests.reconcileOwnership')
    expect(wrapper.text()).not.toContain('admin.scheduledTests.restoreOwnership')
    await wrapper.findAll('button').find((button) => button.text() === 'admin.scheduledTests.reconcileOwnership')!.trigger('click')
    await wrapper.find('[data-test="confirm"]').trigger('click')
    await flushPromises()
    expect(api.reconcileOwnership).toHaveBeenCalledWith(11, 'fence-prior')
  })

  it('offers no mutation when the account has drifted from both ownership values', async () => {
    api.ownershipPreview.mockResolvedValue(driftState)
    const wrapper = await mountPanel(plan(true))
    await wrapper.find('button[title="admin.scheduledTests.ownershipPreview"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.scheduledTests.ownershipConflict')
    expect(wrapper.text()).not.toContain('admin.scheduledTests.reconcileOwnership')
    expect(wrapper.text()).not.toContain('admin.scheduledTests.restoreOwnership')
  })

  it('deletes an empty-state quality rule only after confirmation with the preview fence', async () => {
    api.ownershipPreview.mockResolvedValue(emptyState)
    api.deleteOwnedPlan.mockResolvedValue(emptyState)
    const wrapper = await mountPanel(plan(true))

    await wrapper.find('button[title="admin.scheduledTests.ownershipPreview"]').trigger('click')
    await flushPromises()
    const deleteAction = wrapper.findAll('button').find((button) => button.text() === 'admin.scheduledTests.deleteQualityRule')
    expect(deleteAction).toBeTruthy()
    await deleteAction!.trigger('click')
    expect(api.deleteOwnedPlan).not.toHaveBeenCalled()
    await wrapper.find('[data-test="confirm"]').trigger('click')
    await flushPromises()

    expect(api.deleteOwnedPlan).toHaveBeenCalledWith(11, 'fence-empty')
    expect(wrapper.text()).not.toContain('gpt-test')
  })

  it('keeps the ownership panel and reports fence conflicts', async () => {
    api.ownershipPreview.mockResolvedValue(emptyState)
    api.deleteOwnedPlan.mockRejectedValue({ response: { status: 409 } })
    const wrapper = await mountPanel(plan(true))

    await wrapper.find('button[title="admin.scheduledTests.ownershipPreview"]').trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === 'admin.scheduledTests.deleteQualityRule')!.trigger('click')
    await wrapper.find('[data-test="confirm"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.scheduledTests.ownershipPreviewChanged')
    expect(wrapper.text()).toContain('admin.scheduledTests.ownedState')
  })

  it('keeps ordinary plan deletion on the existing endpoint', async () => {
    const wrapper = await mountPanel(plan(false))
    await wrapper.find('button[title="admin.scheduledTests.deletePlan"]').trigger('click')
    await wrapper.find('[data-test="confirm"]').trigger('click')
    await flushPromises()

    expect(api.delete).toHaveBeenCalledWith(11)
    expect(api.ownershipPreview).not.toHaveBeenCalled()
  })
})
