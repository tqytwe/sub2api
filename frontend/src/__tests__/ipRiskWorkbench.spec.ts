import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import IPRiskWorkbench from '@/features/ip-risk/IPRiskWorkbench.vue'
import type { RiskCaseDetail } from '@/features/ip-risk/types'

const {
  getCase,
  getOverview,
  getRuntime,
  getScan,
  listCases,
  showError,
  showSuccess,
  startScan,
} = vi.hoisted(() => ({
  getCase: vi.fn(),
  getOverview: vi.fn(),
  getRuntime: vi.fn(),
  getScan: vi.fn(),
  listCases: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  startScan: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    ipRisk: {
      getCase,
      getOverview,
      getRuntime,
      getScan,
      listCases,
      startScan,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
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

const detail = (): RiskCaseDetail => ({
  case: {
    id: 7,
    primary_ip: '203.0.113.8',
    primary_network: '203.0.113.8/32',
    score: 94,
    level: 'critical',
    status: 'open',
    evidence_confidence: 'exact',
    signals: [{ code: 'registration_24h', family: 'registration', score: 45, count: 10 }],
    related_user_count: 4,
    selected_user_count: 1,
    auto_block_eligible: true,
    first_detected_at: '2026-07-23T00:00:00Z',
    last_detected_at: '2026-07-23T01:00:00Z',
    version: 3,
  },
  evidence: {
    primary_ip: '203.0.113.8',
    primary_network: '203.0.113.8/32',
    primary_ip_registration_count: 10,
    registration_count_10m: 8,
    registration_count_1h: 8,
    registration_count_24h: 10,
    exact_registration_count: 8,
    max_shared_ua_count: 5,
    email_pattern_account_count: 3,
    shared_api_ip_user_count: 5,
    rapid_key_or_gift_user_count: 0,
    shared_signup_code_count: 0,
    trusted_account_count: 1,
    all_key_evidence_exact: true,
    allowlisted: false,
    known_shared_network: false,
  },
  recommended_actions: ['temporary_registration_block'],
  users: [
    relatedUser(1, 'suspected_new', 'exact', true, 'user'),
    relatedUser(2, 'trusted_existing', 'exact', true, 'user'),
    relatedUser(3, 'suspected_new', 'inferred', true, 'user'),
    relatedUser(4, 'suspected_new', 'exact', true, 'admin'),
  ],
  timeline: [],
  actions: [],
})

function relatedUser(
  userId: number,
  relation: 'suspected_new' | 'trusted_existing' | 'disabled',
  confidence: 'exact' | 'inferred' | 'mixed',
  recommended: boolean,
  role: string,
) {
  return {
    user_id: userId,
    email: `user${userId}@example.test`,
    username: `user${userId}`,
    role,
    status: 'active',
    signup_source: 'email',
    relation_type: relation,
    evidence_confidence: confidence,
    recommended_selected: recommended,
    first_seen_at: '2026-07-23T00:00:00Z',
    last_seen_at: '2026-07-23T01:00:00Z',
    created_at: '2026-07-23T00:00:00Z',
    total_recharged: relation === 'trusted_existing' ? 100 : 0,
    balance: 0,
    primary_ip_registrations: confidence === 'exact' ? 1 : 0,
    shared_ua_account_count: 3,
    gift_granted: 10,
    gift_consumed: 4,
    gift_remaining: 6,
    api_key_count: 0,
    active_api_key_count: 0,
    api_keys: [],
    evidence: {},
  }
}

const CaseDetailStub = defineComponent({
  props: {
    selectedUserIds: {
      type: Array,
      default: () => [],
    },
  },
  template: '<div data-testid="selected-users">{{ selectedUserIds.join(",") }}</div>',
})

function mountWorkbench() {
  return mount(IPRiskWorkbench, {
    global: {
      stubs: {
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /></div>',
        },
        Icon: true,
        Pagination: true,
        Select: true,
        IPRiskCaseDetail: CaseDetailStub,
        IPRiskActionDialog: true,
        IPRiskPolicyDialog: true,
      },
    },
  })
}

describe('IPRiskWorkbench', () => {
  beforeEach(() => {
    getCase.mockReset().mockResolvedValue(detail())
    getOverview.mockReset().mockResolvedValue({
      open_cases: 12,
      critical_cases: 4,
      blocked_policies: 3,
      review_users: 27,
    })
    getRuntime.mockReset().mockResolvedValue({
      enabled: true,
      started: true,
      shadow_mode: true,
      auto_block_enabled: false,
      historical_backfill_enabled: false,
      degraded: false,
      evaluation_queue_size: 0,
      evaluation_queue_capacity: 4096,
    })
    getScan.mockReset()
    listCases.mockReset().mockResolvedValue({
      items: [detail().case],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    startScan.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads a case and selects only exact suspected new accounts by default', async () => {
    const wrapper = mountWorkbench()
    await flushPromises()

    expect(getOverview).toHaveBeenCalledTimes(1)
    expect(getRuntime).toHaveBeenCalledTimes(1)
    expect(listCases).toHaveBeenCalledTimes(1)
    expect(getCase).toHaveBeenCalledWith(7)
    expect(wrapper.get('[data-testid="selected-users"]').text()).toBe('1')
    expect(wrapper.text()).toContain('admin.ipRisk.shadowModeHint')
  })

  it('debounces search filters and polls an asynchronous scan to completion', async () => {
    vi.useFakeTimers()
    startScan.mockResolvedValue({
      id: 19,
      scan_type: 'manual',
      status: 'running',
      range_start: '2026-07-22T00:00:00Z',
      range_end: '2026-07-23T00:00:00Z',
      progress: 20,
      candidate_count: 10,
      case_count: 2,
      inferred_event_count: 0,
      created_at: '2026-07-23T00:00:00Z',
      updated_at: '2026-07-23T00:00:00Z',
    })
    getScan.mockResolvedValue({
      id: 19,
      scan_type: 'manual',
      status: 'completed',
      range_start: '2026-07-22T00:00:00Z',
      range_end: '2026-07-23T00:00:00Z',
      progress: 100,
      candidate_count: 10,
      case_count: 4,
      inferred_event_count: 0,
      created_at: '2026-07-23T00:00:00Z',
      updated_at: '2026-07-23T00:01:00Z',
    })

    const wrapper = mountWorkbench()
    await flushPromises()
    listCases.mockClear()

    await wrapper.get('input').setValue('203.0.113.8')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(listCases).toHaveBeenCalledWith(expect.objectContaining({ search: '203.0.113.8' }))

    const scanButton = wrapper.findAll('button').find((button) =>
      button.text().includes('admin.ipRisk.scanNow'),
    )
    expect(scanButton).toBeTruthy()
    await scanButton!.trigger('click')
    await flushPromises()
    expect(startScan).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()
    expect(getScan).toHaveBeenCalledWith(19)
    expect(showSuccess).toHaveBeenCalledWith('admin.ipRisk.scanCompleted')
  })
})
