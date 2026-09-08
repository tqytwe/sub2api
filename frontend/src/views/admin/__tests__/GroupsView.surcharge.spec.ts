import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminGroup } from '@/types'
import en from '@/i18n/locales/en.ts'
import zh from '@/i18n/locales/zh.ts'
import GroupsView from '@/views/admin/GroupsView.vue'

const {
  authState,
  listGroups,
  createGroup,
  updateGroup,
  getModelsListCandidates,
  getUsageSummary,
  getCapacitySummary,
  getLiveCapability,
  showSuccess,
  showError,
} = vi.hoisted(() => ({
  authState: { isSimpleMode: false },
  listGroups: vi.fn(),
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  getModelsListCandidates: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getLiveCapability: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list: listGroups,
      getAll: vi.fn(),
      create: createGroup,
      update: updateGroup,
      delete: vi.fn(),
      duplicate: vi.fn(),
      updateSortOrder: vi.fn(),
      getModelsListCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
    },
    accounts: {
      list: vi.fn(),
      getById: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const persistedGroup = {
  id: 42,
  name: 'Paid OpenAI',
  description: null,
  platform: 'openai',
  rate_multiplier: 1,
  billing_surcharge_override_enabled: true,
  billing_surcharge_enabled: true,
  billing_surcharge_mode: 'additive_multiplier',
  billing_surcharge_value: 0.05,
  rpm_limit: 0,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  long_context_pricing_enabled: true,
  force_openai_fast: false,
  free_openai_fast: false,
  model_pricing: [],
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 0.5,
  batch_image_hold_multiplier: 0.6,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  web_search_price_per_call: null,
  search_price_per_1k: null,
  audio_realtime_price_per_min: null,
  audio_tts_price_per_million_chars: null,
  audio_stt_price_per_hour: null,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  profit_control_enabled: false,
  profit_min_margin: 0,
  profit_safety_buffer: 0,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  allow_live: false,
  default_mapped_model: '',
  messages_dispatch_model_config: undefined,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 1,
  active_account_count: 1,
  rate_limited_account_count: 0,
  models_list_config: undefined,
  codex_models_manifest_config: undefined,
  sort_order: 10,
} as AdminGroup

const AppLayoutStub = defineComponent({
  template: '<main><slot /></main>',
})

const TablePageLayoutStub = defineComponent({
  template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>',
})

const DataTableStub = defineComponent({
  props: {
    data: { type: Array, default: () => [] },
  },
  template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>',
})

const BaseDialogStub = defineComponent({
  props: {
    show: { type: Boolean, default: false },
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

function mountView() {
  return mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: true,
        PlatformIcon: true,
        Icon: true,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        VueDraggable: true,
      },
    },
  })
}

function findButton(wrapper: ReturnType<typeof mount>, label: string) {
  const button = wrapper.findAll('button').find((item) => item.text() === label)
  expect(button, `button ${label}`).toBeTruthy()
  return button!
}

describe('GroupsView group surcharge controls', () => {
  beforeEach(() => {
    localStorage.clear()
    authState.isSimpleMode = false
    vi.spyOn(console, 'error').mockImplementation(() => {})
    for (const fn of [
      listGroups,
      createGroup,
      updateGroup,
      getModelsListCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
      showSuccess,
      showError,
    ]) {
      fn.mockReset()
    }
    listGroups.mockResolvedValue({
      items: [persistedGroup],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    createGroup.mockResolvedValue(persistedGroup)
    updateGroup.mockResolvedValue(persistedGroup)
    getModelsListCandidates.mockResolvedValue([])
    getUsageSummary.mockResolvedValue([])
    getCapacitySummary.mockResolvedValue([])
    getLiveCapability.mockResolvedValue({ supported: false })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('submits explicit surcharge settings when creating a group', async () => {
    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'admin.groups.createGroup').trigger('click')

    expect(wrapper.get('[data-testid="create-group-surcharge-enabled"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="create-group-surcharge-mode"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="create-group-surcharge-value"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-tour="group-form-name"]').setValue('Surcharge group')
    await wrapper.get('[data-testid="create-group-surcharge-override"]').setValue(true)
    expect(wrapper.get('[data-testid="create-group-surcharge-enabled"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="create-group-surcharge-mode"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="create-group-surcharge-value"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-testid="create-group-surcharge-enabled"]').setValue(true)
    await wrapper.get('[data-testid="create-group-surcharge-mode"]').setValue('percent_on_charged_cost')
    await wrapper.get('[data-testid="create-group-surcharge-value"]').setValue('0.03')
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()

    expect(createGroup).toHaveBeenCalledWith(expect.objectContaining({
      billing_surcharge_override_enabled: true,
      billing_surcharge_enabled: true,
      billing_surcharge_mode: 'percent_on_charged_cost',
      billing_surcharge_value: 0.03,
    }))
    wrapper.unmount()
  })

  it('shows persisted settings and includes them in the update payload', async () => {
    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'common.edit').trigger('click')
    await flushPromises()

    expect((wrapper.get('[data-testid="edit-group-surcharge-override"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-testid="edit-group-surcharge-enabled"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-testid="edit-group-surcharge-mode"]').element as HTMLSelectElement).value).toBe('additive_multiplier')
    expect((wrapper.get('[data-testid="edit-group-surcharge-value"]').element as HTMLInputElement).value).toBe('0.05')

    await wrapper.get('[data-testid="edit-group-surcharge-value"]').setValue('0.075')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({
      billing_surcharge_override_enabled: true,
      billing_surcharge_enabled: true,
      billing_surcharge_mode: 'additive_multiplier',
      billing_surcharge_value: 0.075,
    }))
    wrapper.unmount()
  })

  it('disables the override controls without erasing their persisted values', async () => {
    const wrapper = mountView()
    await flushPromises()
    await findButton(wrapper, 'common.edit').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="edit-group-surcharge-override"]').setValue(false)
    expect(wrapper.get('[data-testid="edit-group-surcharge-enabled"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="edit-group-surcharge-mode"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="edit-group-surcharge-value"]').attributes('disabled')).toBeDefined()

    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({
      billing_surcharge_override_enabled: false,
      billing_surcharge_enabled: true,
      billing_surcharge_mode: 'additive_multiplier',
      billing_surcharge_value: 0.05,
    }))
    wrapper.unmount()
  })

  it('keeps Chinese and English copy complete without mixed-language English text', () => {
    expect(zh.admin.groups.surcharge).toMatchObject({
      override: expect.any(String),
      enabled: expect.any(String),
      modeNone: expect.any(String),
      modePercent: expect.any(String),
      modeAdditive: expect.any(String),
      hint: expect.any(String),
    })
    expect(en.admin.groups.surcharge).toMatchObject({
      override: expect.any(String),
      enabled: expect.any(String),
      modeNone: expect.any(String),
      modePercent: expect.any(String),
      modeAdditive: expect.any(String),
      hint: expect.stringContaining('0.03% = 0.0003'),
    })
    expect(en.admin.groups.surcharge.hint).not.toMatch(/[\u3400-\u9fff]/u)
  })
})
