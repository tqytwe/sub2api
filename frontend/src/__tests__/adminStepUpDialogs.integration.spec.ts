import { defineComponent, nextTick, ref } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import BulkUserActionDialog from '@/components/admin/user/BulkUserActionDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import IPRiskActionDialog from '@/features/ip-risk/IPRiskActionDialog.vue'
import type { RiskCaseDetail } from '@/features/ip-risk/types'
import { useStepUp } from '@/composables/useStepUp'

const {
  executeBatchAction,
  executeRiskAction,
  previewBatchAction,
  previewRiskAction,
  showError,
  showSuccess,
  showWarning,
  stepUp,
} = vi.hoisted(() => ({
  executeBatchAction: vi.fn(),
  executeRiskAction: vi.fn(),
  previewBatchAction: vi.fn(),
  previewRiskAction: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
  stepUp: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      executeBatchAction,
      previewBatchAction,
    },
    ipRisk: {
      executeAction: executeRiskAction,
      previewAction: previewRiskAction,
    },
  },
}))

const appStore = {
  showError,
  showSuccess,
  showWarning,
}

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/api', () => ({
  totpAPI: {
    stepUp,
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
      te: () => false,
    }),
  }
})

const userPreview = {
  action: 'disable' as const,
  requested_count: 1,
  eligible_users: [
    {
      id: 19,
      email: 'risk-user@example.test',
      role: 'user' as const,
      status: 'active' as const,
      api_key_count: 1,
    },
  ],
  protected_administrators: [],
  already_disabled_users: [],
  missing_user_ids: [],
  affected_api_keys: 1,
  requires_step_up: true,
  confirmation_token: 'user-preview-token',
  expires_at: '2026-07-24T10:05:00Z',
}

const riskDetail: RiskCaseDetail = {
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
    first_detected_at: '2026-07-24T01:00:00Z',
    last_detected_at: '2026-07-24T01:05:00Z',
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
}

function attach(component: ReturnType<typeof defineComponent>) {
  const app = document.createElement('div')
  app.id = 'app'
  document.body.appendChild(app)
  return {
    app,
    wrapper: mount(component, {
      attachTo: app,
      global: {
        stubs: {
          Icon: true,
          Select: {
            props: ['modelValue', 'options', 'disabled', 'id'],
            template: '<select :value="modelValue"></select>',
          },
        },
      },
    }),
  }
}

function findButton(text: string) {
  const button = Array.from(document.body.querySelectorAll<HTMLButtonElement>('button'))
    .find((candidate) => candidate.textContent?.includes(text))
  if (!button) throw new Error(`button not found: ${text}`)
  return button
}

function setInput(selector: string, value: string) {
  const input = document.body.querySelector<HTMLInputElement | HTMLTextAreaElement>(selector)
  if (!input) throw new Error(`input not found: ${selector}`)
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

async function enterTotp(app: HTMLElement, value = '123456') {
  const dialog = document.body.querySelector<HTMLElement>(
    '[role="dialog"][aria-labelledby^="totp-step-up-title-"]',
  )
  expect(dialog).not.toBeNull()
  expect(app.contains(dialog)).toBe(false)
  expect((app as HTMLElement & { inert?: boolean }).inert).toBe(true)

  const inputs = Array.from(dialog!.querySelectorAll<HTMLInputElement>(
    'input:not([aria-hidden="true"])',
  ))
  expect(inputs).toHaveLength(6)
  for (const [index, input] of inputs.entries()) {
    input.value = value[index]
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
  }
  await flushPromises()
}

describe('admin page-level TOTP dialogs', () => {
  let wrapper: VueWrapper | null = null

  beforeEach(() => {
    previewBatchAction.mockReset()
    executeBatchAction.mockReset()
    previewRiskAction.mockReset()
    executeRiskAction.mockReset()
    stepUp.mockReset().mockResolvedValue(undefined)
    showError.mockReset()
    showSuccess.mockReset()
    showWarning.mockReset()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
  })

  it('opens an interactive TOTP dialog before sending a bulk user disable', async () => {
    previewBatchAction.mockResolvedValue(userPreview)
    executeBatchAction.mockResolvedValue({
      action: 'disable',
      status: 'completed',
      requested_count: 1,
      succeeded_user_ids: [19],
      skipped: [],
      failed: [],
      affected_api_keys: 1,
    })

    const Harness = defineComponent({
      components: { BulkUserActionDialog, TotpStepUpDialog },
      setup() {
        return { controller: useStepUp() }
      },
      template: `
        <BulkUserActionDialog
          :show="true"
          :selected-ids="[19]"
          action="disable"
          :step-up="controller"
        />
        <TotpStepUpDialog :controller="controller" />
      `,
    })

    const mounted = attach(Harness)
    wrapper = mounted.wrapper
    await flushPromises()
    expect(wrapper.findComponent(BulkUserActionDialog).props('stepUp')).toBe(
      (wrapper.vm as unknown as { controller: ReturnType<typeof useStepUp> }).controller,
    )
    setInput('[data-test="reason"]', 'confirmed automated registration abuse')
    await nextTick()
    findButton('admin.users.bulkActions.preview').click()
    await flushPromises()
    findButton('admin.users.bulkActions.confirmDisable').click()
    await flushPromises()

    expect(
      (wrapper.vm as unknown as { controller: ReturnType<typeof useStepUp> }).controller.visible.value,
    ).toBe(true)
    expect(executeBatchAction).not.toHaveBeenCalled()
    await enterTotp(mounted.app)

    expect(stepUp).toHaveBeenCalledWith('123456')
    expect(executeBatchAction).toHaveBeenCalledTimes(1)
    expect(executeBatchAction).toHaveBeenLastCalledWith({
      action: 'disable',
      user_ids: [19],
      reason: 'confirmed automated registration abuse',
      confirmation_token: 'user-preview-token',
    })
  })

  it('opens an interactive TOTP dialog before sending an IP risk action', async () => {
    previewRiskAction.mockResolvedValue({
      case_id: 7,
      case_version: 3,
      action_type: 'disable_users',
      user_ids: [19],
      api_key_ids: [],
      user_count: 1,
      api_key_count: 0,
      already_disabled: 0,
      protected_users: [],
      trusted_users: [],
      inferred_users: [],
      duration_minutes: 0,
      requires_step_up: true,
      confirmation_token: 'risk-preview-token',
      expires_at: '2026-07-24T01:10:00Z',
      state_digest: 'risk-state-digest',
    })
    executeRiskAction.mockResolvedValue({
      id: 90,
      case_id: 7,
      action_type: 'disable_users',
      status: 'completed',
      actor_type: 'admin',
      actor_user_id: 42,
      reason: 'confirmed clustered registrations',
      rollback_status: 'eligible',
      result: { completed_items: 1 },
      created_at: '2026-07-24T01:05:00Z',
    })

    const Harness = defineComponent({
      components: { IPRiskActionDialog, TotpStepUpDialog },
      setup() {
        return { controller: useStepUp(), riskDetail, show: ref(false) }
      },
      template: `
        <IPRiskActionDialog
          :show="show"
          :detail="riskDetail"
          :selected-user-ids="[19]"
          initial-action="disable_users"
          :step-up="controller"
        />
        <TotpStepUpDialog :controller="controller" />
      `,
    })

    const mounted = attach(Harness)
    wrapper = mounted.wrapper
    ;(wrapper.vm as unknown as { show: boolean }).show = true
    await flushPromises()
    expect(wrapper.findComponent(IPRiskActionDialog).props('stepUp')).toBe(
      (wrapper.vm as unknown as { controller: ReturnType<typeof useStepUp> }).controller,
    )
    setInput('#ip-risk-action-reason', 'confirmed clustered registrations')
    await nextTick()
    findButton('admin.ipRisk.actionDialog.preview').click()
    await flushPromises()
    findButton('admin.ipRisk.actionDialog.confirmExecute').click()
    await flushPromises()

    expect(
      (wrapper.vm as unknown as { controller: ReturnType<typeof useStepUp> }).controller.visible.value,
    ).toBe(true)
    expect(executeRiskAction).not.toHaveBeenCalled()
    await enterTotp(mounted.app)

    expect(stepUp).toHaveBeenCalledWith('123456')
    expect(executeRiskAction).toHaveBeenCalledTimes(1)
    expect(executeRiskAction).toHaveBeenLastCalledWith(7, expect.objectContaining({
      action_type: 'disable_users',
      user_ids: [19],
      preview_token: 'risk-preview-token',
    }))
  })
})
