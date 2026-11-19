import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import OAuthAutomationPanel from '../OAuthAutomationPanel.vue'

const api = vi.hoisted(() => ({ get: vi.fn(), save: vi.fn(), groups: vi.fn(), plans: vi.fn() }))
const identity = reactive({ user: { id: 1, role: 'admin' } })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => identity }))
vi.mock('@/api/admin/autoConfig', () => ({ getAutoConfig: api.get, saveAutoConfig: api.save }))
vi.mock('@/api/admin/groups', () => ({ getAll: api.groups }))
vi.mock('@/api/admin/accountQuality', () => ({ listPlans: api.plans }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual('vue-i18n'), useI18n: () => ({ t: (k: string) => k }) }))
const cfg = () => ({
  enabled: false, platform: 'openai', priority: 50, load_factor: 1, concurrency: 3,
  group_ids: [10], upgrade_enabled: false, upgrade_group_ids: [10], successes_per_step: 20,
  upgrade_step: 1, max_concurrency: 100, cooldown_seconds: 60, cost_multiplier: 0.07,
  revision: 'rev1', selected_models: null, model_mappings: [],
  degradation_action: { enabled: false, threshold_percent: 50, target_concurrency: 10, models: ['model-a'], group_ids: [] },
  openai_apikey_fallback: { enabled: false, account_ids: [7], metric: 'sla', threshold: 95, window_minutes: 5 },
  openai_apikey_fallback_status: { last_checked_at: '2026-11-18T00:00:00Z', samples: 0 }
})
const mountPanel = () => mount(OAuthAutomationPanel, { global: { stubs: {
  OAuthDegradationCard: true,
  OpenAIAPIKeyFallbackCard: { props: ['modelValue'], template: '<button data-testid="refresh-fallback" @click="$emit(\'refresh-status\')" />', methods: { validationError: () => null } },
  OAuthInitialModelSelection: true, AutoConfigHistory: true
} } })

describe('OAuth automation in existing quality operations', () => {
  beforeEach(() => {
    Object.values(api).forEach(f => f.mockReset())
    api.get.mockResolvedValue(cfg())
    api.groups.mockResolvedValue([{ id: 10, name: 'OAuth pool', platform: 'openai', status: 'active' }])
    api.plans.mockResolvedValue([])
    api.save.mockImplementation(async c => c)
    identity.user = { id: 1, role: 'admin' }
  })
  it('saves initial and upgrade switches independently, preserving disabled degradation and fallback', async () => {
    const w = mountPanel(); await flushPromises()
    await w.get('[data-testid="initial-enabled"]').setValue(true)
    await w.get('form').trigger('submit'); await flushPromises()
    const saved = api.save.mock.calls[0][0]
    expect(saved).toMatchObject({ enabled: true, upgrade_enabled: false, revision: 'rev1', openai_apikey_fallback: { enabled: false, account_ids: [7] }, degradation_action: { enabled: false } })
    expect(saved).not.toHaveProperty('runtime_blocked')
    expect(saved).not.toHaveProperty('openai_apikey_fallback_status')
    expect(w.get('[role="status"]').text()).toBe('autoConfig.saved')
  })
  it('rejects enabled initial configuration without a group and retains the draft', async () => {
    api.get.mockResolvedValue({ ...cfg(), group_ids: [] })
    const w = mountPanel(); await flushPromises()
    await w.get('[data-testid="initial-enabled"]').setValue(true)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(api.save).not.toHaveBeenCalled()
    expect(w.get('[role="alert"]').text()).toBe('autoConfig.invalid')
    expect((w.get('[data-testid="initial-enabled"]').element as HTMLInputElement).checked).toBe(true)
  })
  it('copies a quality rule snapshot without source identity or runtime output', async () => {
    const rule = { id: 8, account_id: 44, account_name: 'source', model_id: 'model-a', cron_expression: '*/30 * * * *', enabled: true, max_results: 100, pelican_config: { quality: { action: 'observe_only' }, expected_answer: '21' }, last_result: 'private-runtime' }
    api.plans.mockResolvedValue([rule])
    const w = mountPanel(); await flushPromises()
    await w.get('[data-testid="quality-rule-select"]').setValue('8')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(api.save.mock.calls[0][0].quality_rule).toEqual({ model_id: rule.model_id, cron_expression: rule.cron_expression, enabled: true, max_results: 100, pelican_config: rule.pelican_config })
  })
  it('refreshes fallback status without replacing unsaved configuration', async () => {
    const w = mountPanel(); await flushPromises()
    await w.get('[data-testid="concurrency"]').setValue('9')
    api.get.mockResolvedValue({ ...cfg(), concurrency: 99 })
    await w.get('[data-testid="refresh-fallback"]').trigger('click')
    await flushPromises()
    expect((w.get('[data-testid="concurrency"]').element as HTMLInputElement).value).toBe('9')
  })
  it('retains edits after save fails and allows retry', async () => {
    api.save.mockRejectedValueOnce(new Error('conflict'))
    const w = mountPanel(); await flushPromises()
    await w.get('[data-testid="priority"]').setValue('12')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(w.get('[role="alert"]').text()).toBe('autoConfig.saveFailed')
    expect((w.get('[data-testid="priority"]').element as HTMLInputElement).value).toBe('12')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(api.save).toHaveBeenCalledTimes(2)
  })
  it('provides a retry on load failure', async () => {
    api.get.mockRejectedValueOnce(new Error('unavailable'))
    const w = mountPanel(); await flushPromises()
    expect(w.get('[role="alert"]').text()).toBe('autoConfig.loadFailed')
    await w.get('button').trigger('click'); await flushPromises()
    expect(w.find('form').exists()).toBe(true)
  })
  it('resets platform-specific defaults and copied rule when platform changes', async () => {
    const w = mountPanel(); await flushPromises()
    await w.get('[data-testid="platform"]').setValue('anthropic')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(api.save.mock.calls[0][0]).toMatchObject({ platform: 'anthropic', group_ids: [], selected_models: null, quality_rule: null, openai_totp_rotate_on_429: false })
  })
  it('removes a loaded administrator draft immediately when identity changes', async () => {
    const w = mountPanel(); await flushPromises()
    identity.user = { id: 2, role: 'user' }; await flushPromises()
    expect(w.find('form').exists()).toBe(false)
  })
})
