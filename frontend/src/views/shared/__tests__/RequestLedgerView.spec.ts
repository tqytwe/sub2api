import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import RequestLedgerView from '../RequestLedgerView.vue'

const { list, detail, replace } = vi.hoisted(() => ({ list: vi.fn(), detail: vi.fn(), replace: vi.fn() }))
vi.mock('@/api/requestLedger', () => ({ requestLedgerAPI: { list, detail } }))
vi.mock('vue-router', () => ({ useRoute: () => ({ meta: { requiresAdmin: false }, query: { lang: 'en' } }), useRouter: () => ({ replace }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

const item = { id: '00000000-0000-4000-8000-000000000001', route: '/v1/responses', method: 'POST', kind: 'http', user_id: 101, api_key_id: 201, started_at: '2026-10-10T00:00:00Z', ended_at: null, execution_state: 'interrupted', usage_state: 'usage_unknown', settlement_state: 'settlement_pending', attempt_count: 1, output_observed: true, error_code: 'interrupted', http_status: 200 }
const mountView = () => mount(RequestLedgerView, { global: { plugins: [createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: { en: {} } })], stubs: { RouterLink: { template: '<a><slot /></a>' }, DataTable: { props: ['data'], template: '<div><slot name="cell-execution_state" v-for="row in data" :row="row" /><slot name="cell-usage_state" v-for="row in data" :row="row" /><slot name="cell-settlement_state" v-for="row in data" :row="row" /><slot name="cell-actions" v-for="row in data" :row="row" /></div>' }, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, Select: true, Pagination: true } } })

beforeEach(() => { replace.mockReset(); list.mockReset(); detail.mockReset(); list.mockResolvedValue({ items: [item], total: 1, page: 1, page_size: 20 }) })
describe('request ledger', () => {
 it('preserves URL language when applying filters', async () => {
  const wrapper = mountView(); await flushPromises()
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(replace).toHaveBeenCalledWith({ query: { lang: 'en' } })
 })
 it('shows uncertainty separately from execution and never renders an inferred zero price', async () => {
  detail.mockResolvedValue({ request: item, attempts: [], billing: [] })
  const wrapper = mountView(); await flushPromises()
  expect(wrapper.text()).toContain('requestLedger.states.interrupted')
  expect(wrapper.text()).toContain('requestLedger.states.usage_unknown')
  expect(wrapper.text()).toContain('requestLedger.states.settlement_pending')
  expect(wrapper.text()).not.toContain('$0.00')
  expect(wrapper.find('[name="user_id"]').exists()).toBe(false)
  expect(list).toHaveBeenCalledWith(false, expect.any(Object), expect.any(AbortSignal))
  await wrapper.get('[data-testid="ledger-refresh"]').trigger('click'); await flushPromises()
  expect(list).toHaveBeenCalledTimes(2)
 })
 it('keeps errors visible with retry rather than claiming an empty ledger', async () => {
  list.mockRejectedValueOnce(new Error('fixture failure'))
  const wrapper = mountView(); await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('requestLedger.loadError')
  expect(wrapper.text()).not.toContain('fixture failure')
  await wrapper.get('[data-testid="ledger-retry"]').trigger('click'); await flushPromises()
  expect(wrapper.find('[role="alert"]').exists()).toBe(false)
 })
})
