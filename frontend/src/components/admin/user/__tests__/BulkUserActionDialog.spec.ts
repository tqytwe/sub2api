import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import BulkUserActionDialog from '../BulkUserActionDialog.vue'

const { previewBatchAction, executeBatchAction, runStepUp, showError } = vi.hoisted(() => ({
  previewBatchAction: vi.fn(),
  executeBatchAction: vi.fn(),
  runStepUp: vi.fn(async (action: () => Promise<unknown>) => action()),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      previewBatchAction,
      executeBatchAction,
    },
  },
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({
    visible: { value: false },
    blockedReason: { value: '' },
    prompt: vi.fn(),
    onVerified: vi.fn(),
    onCancel: vi.fn(),
    run: runStepUp,
  }),
  isStepUpCancelled: () => false,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

const preview = {
  action: 'delete' as const,
  requested_count: 3,
  eligible_users: [
    { id: 1, email: 'one@example.test', role: 'user' as const, status: 'active' as const, api_key_count: 2 },
  ],
  protected_administrators: [
    { id: 2, email: 'admin@example.test', role: 'admin' as const, status: 'active' as const, api_key_count: 0 },
  ],
  already_disabled_users: [],
  missing_user_ids: [404],
  affected_api_keys: 2,
  requires_step_up: true,
  confirmation_token: 'preview-token',
  expires_at: '2026-07-24T10:05:00Z',
}

const stepUp = {
  visible: { value: false },
  blockedReason: { value: '' },
  prompt: vi.fn(),
  onVerified: vi.fn(),
  onCancel: vi.fn(),
  run: runStepUp,
}

const mountDialog = (action: 'disable' | 'delete' = 'delete') =>
  mount(BulkUserActionDialog, {
    props: { show: true, selectedIds: [1, 2, 404], action, stepUp },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show', 'title'],
          template: '<div v-if="show"><div data-test="title">{{ title }}</div><slot /><slot name="footer" /></div>',
        },
        Icon: true,
      },
    },
  })

describe('BulkUserActionDialog', () => {
  beforeEach(() => {
    previewBatchAction.mockReset()
    executeBatchAction.mockReset()
    runStepUp.mockClear()
    showError.mockReset()
    previewBatchAction.mockResolvedValue(preview)
    executeBatchAction.mockResolvedValue({
      action: 'delete',
      status: 'completed',
      requested_count: 3,
      succeeded_user_ids: [1],
      skipped: [
        { user_id: 2, email: 'admin@example.test', reason: 'protected_administrator' },
        { user_id: 404, reason: 'not_found' },
      ],
      failed: [],
      affected_api_keys: 2,
    })
  })

  it('requires a reason, previews protected users and executes through step-up', async () => {
    const wrapper = mountDialog()

    expect(wrapper.get('[data-test="preview"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="reason"]').setValue('confirmed abuse')
    await wrapper.get('[data-test="preview"]').trigger('click')
    await flushPromises()

    expect(previewBatchAction).toHaveBeenCalledWith({
      action: 'delete',
      user_ids: [1, 2, 404],
      reason: 'confirmed abuse',
    })
    expect(wrapper.get('[data-test="eligible-count"]').text()).toBe('1')
    expect(wrapper.get('[data-test="protected-count"]').text()).toBe('1')
    expect(wrapper.get('[data-test="missing-count"]').text()).toBe('1')
    expect(wrapper.get('[data-test="key-count"]').text()).toBe('2')
    expect(wrapper.get('[data-test="preview-users"]').text()).toContain('one@example.test')
    expect(wrapper.get('[data-test="preview-users"]').text()).toContain('admin@example.test')
    expect(wrapper.get('[data-test="preview-users"]').text()).toContain(
      'admin.users.bulkActions.userId:{"id":404}'
    )
    expect(wrapper.get('[data-test="execute"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="delete-confirmation"]').setValue('DELETE 1')
    await wrapper.get('[data-test="execute"]').trigger('click')
    await flushPromises()

    expect(runStepUp).toHaveBeenCalledOnce()
    expect(executeBatchAction).toHaveBeenCalledWith({
      action: 'delete',
      user_ids: [1, 2, 404],
      reason: 'confirmed abuse',
      confirmation_token: 'preview-token',
    })
    expect(wrapper.emitted('completed')).toHaveLength(1)
  })

  it('invalidates the preview when the reason changes', async () => {
    const wrapper = mountDialog('disable')
    await wrapper.get('[data-test="reason"]').setValue('incident response')
    await wrapper.get('[data-test="preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="eligible-count"]').exists()).toBe(true)

    await wrapper.get('[data-test="reason"]').setValue('updated reason')

    expect(wrapper.find('[data-test="eligible-count"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="delete-confirmation"]').exists()).toBe(false)
  })

  it('clears an expired server preview and allows the administrator to preview again', async () => {
    executeBatchAction.mockRejectedValueOnce({
      code: 'USER_BATCH_ACTION_PREVIEW_EXPIRED',
      message: 'expired',
    })
    const wrapper = mountDialog()
    await wrapper.get('[data-test="reason"]').setValue('confirmed abuse')
    await wrapper.get('[data-test="preview"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="delete-confirmation"]').setValue('DELETE 1')
    await wrapper.get('[data-test="execute"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="eligible-count"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="preview"]').exists()).toBe(true)
    expect(showError).toHaveBeenCalledWith('admin.users.bulkActions.previewStale')
  })
})
