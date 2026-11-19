import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpenAIAPIKeyFallbackCard from '../OpenAIAPIKeyFallbackCard.vue'
const list = vi.hoisted(() => vi.fn())
vi.mock('@/api/admin/accounts', () => ({ list }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual('vue-i18n'), useI18n: () => ({ t: (k: string) => k }) }))
const account = (id: number, overrides = {}) => ({ id, name: 'fallback-' + id, platform: 'openai', type: 'apikey', status: 'active', schedulable: false, parent_account_id: null, group_ids: [10], ...overrides })
const mountCard = (enabled = true, ids = [7]) => mount(OpenAIAPIKeyFallbackCard, {
 props: { modelValue: { enabled, account_ids: ids, metric: 'sla', threshold: 95, window_minutes: 5 }, groups: [], userId: 1, statusLoading: false, statusFailed: false }
})
describe('selected API Key fallback inventory', () => {
 beforeEach(() => list.mockReset())
 it('loads all pages, includes paused active parents and excludes shadows, expired and disabled accounts', async () => {
  list.mockResolvedValueOnce({ page: 1, pages: 2, total: 6, items: [account(7), account(8, { parent_account_id: 7 }), account(9, { expires_at: 1 })] })
  list.mockResolvedValueOnce({ page: 2, pages: 2, total: 6, items: [account(10, { status: 'inactive' }), account(11, { platform: 'qwen' }), account(12, { schedulable: true })] })
  const w = mountCard(); await flushPromises()
  expect(w.findAll('[data-account-id]').map(a => a.attributes('data-account-id'))).toEqual(['7', '12'])
  expect(list.mock.calls[0][2]).not.toHaveProperty('status')
  expect((w.vm as unknown as {validationError: () => string | null}).validationError()).toBe(null)
 })
 it('retains selected IDs and disables selection if any page fails', async () => {
  list.mockResolvedValueOnce({ page: 1, pages: 2, total: 2, items: [account(7)] }).mockRejectedValueOnce(new Error('unavailable'))
  const w = mountCard(); await flushPromises()
  expect(w.find('[data-testid="apikey-fallback-accounts-error"]').exists()).toBe(true)
  expect(w.get('[data-testid="apikey-fallback-retained-ids"]').text()).toContain('#7')
  expect(w.emitted('update:modelValue')).toBeUndefined()
  expect((w.vm as unknown as {validationError: () => string | null}).validationError()).toBe('autoConfig.apikeyFallback.accountsFailed')
 })
 it('allows turning off a policy when its saved account has been deleted', async () => {
  list.mockResolvedValue({ page: 1, pages: 1, total: 0, items: [] })
  const w = mountCard(false); await flushPromises()
  expect(w.find('[data-testid="apikey-fallback-unavailable"]').exists()).toBe(true)
  expect((w.vm as unknown as {validationError: () => string | null}).validationError()).toBe(null)
 })
})
