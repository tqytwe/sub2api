import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import IPRiskActionDialog from '@/features/ip-risk/IPRiskActionDialog.vue'
import IPRiskActionsView from '@/features/ip-risk/IPRiskActionsView.vue'
import type { RiskCaseDetail } from '@/features/ip-risk/types'

const {
  executeAction,
  listActions,
  previewAction,
  rollbackAction,
  showError,
  showSuccess,
  showWarning,
  stepUpRun,
} = vi.hoisted(() => ({
  executeAction: vi.fn(),
  listActions: vi.fn(),
  previewAction: vi.fn(),
  rollbackAction: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
  stepUpRun: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    ipRisk: {
      executeAction,
      listActions,
      previewAction,
      rollbackAction,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showWarning,
  }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({
    visible: { value: false },
    blockedReason: { value: '' },
    prompt: vi.fn(),
    onVerified: vi.fn(),
    onCancel: vi.fn(),
    run: stepUpRun,
  }),
  isStepUpCancelled: (error: unknown) =>
    Boolean((error as { code?: string })?.code === 'STEP_UP_CANCELLED'),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let value = key
        Object.entries(params || {}).forEach(([name, replacement]) => {
          value = value.replace(`{${name}}`, String(replacement))
        })
        return value
      },
    }),
  }
})

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const detail = (): RiskCaseDetail => ({
  case: {
    id: 7,
    primary_ip: '203.0.113.8',
    primary_network: '203.0.113.8/32',
    score: 94,
    level: 'critical',
    status: 'open',
    evidence_confidence: 'exact',
    signals: [],
    related_user_count: 1,
    selected_user_count: 1,
    auto_block_eligible: true,
    first_detected_at: '2026-07-23T00:00:00Z',
    last_detected_at: '2026-07-23T01:00:00Z',
    version: 3,
  },
  evidence: {
    primary_ip: '203.0.113.8',
    primary_network: '203.0.113.8/32',
    primary_ip_registration_count: 5,
    registration_count_10m: 5,
    registration_count_1h: 5,
    registration_count_24h: 5,
    exact_registration_count: 5,
    max_shared_ua_count: 5,
    email_pattern_account_count: 0,
    shared_api_ip_user_count: 3,
    rapid_key_or_gift_user_count: 0,
    shared_signup_code_count: 0,
    trusted_account_count: 0,
    all_key_evidence_exact: true,
    allowlisted: false,
    known_shared_network: false,
  },
  recommended_actions: [],
  users: [],
  timeline: [],
  actions: [],
})

function mountActionDialog() {
  return mount(IPRiskActionDialog, {
    props: {
      show: false,
      detail: detail(),
      selectedUserIds: [2],
      initialAction: 'disable_users',
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Icon: true,
        Select: true,
        TotpStepUpDialog: true,
      },
    },
  })
}

function buttonByText(wrapper: ReturnType<typeof mount>, text: string) {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) throw new Error(`button not found: ${text}`)
  return button
}

describe('IP risk action flows', () => {
  beforeEach(() => {
    executeAction.mockReset()
    listActions.mockReset()
    previewAction.mockReset()
    rollbackAction.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    showWarning.mockReset()
    stepUpRun.mockReset().mockImplementation((action: () => Promise<unknown>) => action())
  })

  it('previews and executes a destructive action through step-up, preserving partial results', async () => {
    previewAction.mockResolvedValue({
      case_id: 7,
      case_version: 3,
      action_type: 'disable_users',
      user_ids: [2],
      api_key_ids: [],
      user_count: 1,
      api_key_count: 0,
      already_disabled: 0,
      protected_users: [],
      trusted_users: [],
      inferred_users: [],
      duration_minutes: 0,
      requires_step_up: true,
      confirmation_token: 'preview-token',
      expires_at: '2026-07-23T01:05:00Z',
      state_digest: 'state-digest',
    })
    executeAction.mockResolvedValue({
      id: 90,
      case_id: 7,
      action_type: 'disable_users',
      status: 'partial',
      actor_type: 'admin',
      actor_user_id: 42,
      reason: 'confirmed clustered registrations',
      rollback_status: 'eligible',
      result: { completed_items: 1, failed_items: 1 },
      created_at: '2026-07-23T01:00:00Z',
    })

    const wrapper = mountActionDialog()
    await wrapper.setProps({ show: true })
    await wrapper.get('textarea').setValue('confirmed clustered registrations')
    await buttonByText(wrapper, 'admin.ipRisk.actionDialog.preview').trigger('click')
    await flushPromises()

    expect(previewAction).toHaveBeenCalledWith(7, expect.objectContaining({
      action_type: 'disable_users',
      user_ids: [2],
    }))
    await buttonByText(wrapper, 'admin.ipRisk.actionDialog.confirmExecute').trigger('click')
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledTimes(1)
    expect(executeAction).toHaveBeenCalledWith(7, expect.objectContaining({
      preview_token: 'preview-token',
    }))
    expect(wrapper.text()).toContain('admin.ipRisk.actionDialog.result.partial')
    expect(wrapper.emitted('completed')).toHaveLength(1)
  })

  it('clears a stale preview and requests case reload on 409', async () => {
    previewAction.mockResolvedValue({
      case_id: 7,
      case_version: 3,
      action_type: 'disable_users',
      user_ids: [2],
      api_key_ids: [],
      user_count: 1,
      api_key_count: 0,
      already_disabled: 0,
      protected_users: [],
      trusted_users: [],
      inferred_users: [],
      duration_minutes: 0,
      requires_step_up: true,
      confirmation_token: 'stale-token',
      expires_at: '2026-07-23T01:05:00Z',
      state_digest: 'stale-state-digest',
    })
    executeAction.mockRejectedValue({ status: 409, reason: 'risk_action_preview_stale' })

    const wrapper = mountActionDialog()
    await wrapper.setProps({ show: true })
    await wrapper.get('textarea').setValue('confirmed clustered registrations')
    await buttonByText(wrapper, 'admin.ipRisk.actionDialog.preview').trigger('click')
    await flushPromises()
    await buttonByText(wrapper, 'admin.ipRisk.actionDialog.confirmExecute').trigger('click')
    await flushPromises()

    expect(wrapper.emitted('stale')).toHaveLength(1)
    expect(showWarning).toHaveBeenCalledWith('admin.ipRisk.actionDialog.previewStale')
    expect(wrapper.text()).toContain('admin.ipRisk.actionDialog.previewRequired')
  })

  it('rolls back eligible actions through step-up and shows conflicts', async () => {
    const eligibleAction = {
      id: 90,
      case_id: 7,
      action_type: 'disable_users' as const,
      status: 'completed',
      actor_type: 'admin' as const,
      actor_user_id: 42,
      reason: 'confirmed clustered registrations',
      rollback_status: 'eligible',
      result: { completed_items: 1 },
      created_at: '2026-07-23T01:00:00Z',
    }
    listActions.mockResolvedValue({
      items: [eligibleAction],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    rollbackAction.mockResolvedValue({
      ...eligibleAction,
      id: 91,
      action_type: 'rollback',
      status: 'partial',
      rollback_status: 'not_requested',
      rollback_of_action_id: 90,
      result: { completed_items: 1, conflict_items: 1, skipped_items: 1 },
    })

    const wrapper = mount(IPRiskActionsView, {
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
          Pagination: true,
          TotpStepUpDialog: true,
        },
      },
    })
    await flushPromises()

    await buttonByText(wrapper, 'admin.ipRisk.actionsView.rollback').trigger('click')
    await wrapper.get('#ip-risk-rollback-reason').setValue('restore only unchanged action state')
    await buttonByText(wrapper, 'admin.ipRisk.actionsView.confirmRollback').trigger('click')
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledTimes(1)
    expect(rollbackAction).toHaveBeenCalledWith(90, 'restore only unchanged action state')
    expect(wrapper.text()).toContain('admin.ipRisk.actionsView.rollbackResult')
  })
})
