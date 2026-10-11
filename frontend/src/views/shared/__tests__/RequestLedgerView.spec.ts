import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import RequestLedgerView from '../RequestLedgerView.vue'

const { list, detail, usage, replace, routeMeta } = vi.hoisted(() => ({ list: vi.fn(), detail: vi.fn(), usage: vi.fn(), replace: vi.fn(), routeMeta: { requiresAdmin: false } }))
vi.mock('@/api/requestLedger', () => ({ requestLedgerAPI: { list, detail, usage } }))
const routeQuery = reactive<Record<string, string>>({ lang: 'en' })
vi.mock('vue-router', () => ({ useRoute: () => ({ meta: routeMeta, query: routeQuery }), useRouter: () => ({ replace }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

const item = { id: '00000000-0000-4000-8000-000000000001', route: '/v1/responses', method: 'POST', kind: 'http', user_id: 101, api_key_id: 201, started_at: '2026-10-10T00:00:00Z', ended_at: null, execution_state: 'interrupted', usage_state: 'usage_unknown', settlement_state: 'settlement_pending', attempt_count: 1, output_observed: true, error_code: 'interrupted', http_status: 200 }
const mountView = () => mount(RequestLedgerView, { global: { plugins: [createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: { en: {} } })], stubs: { RouterLink: { template: '<a><slot /></a>' }, DataTable: { props: ['data'], template: '<div><slot name="cell-execution_state" v-for="row in data" :row="row" /><slot name="cell-usage_state" v-for="row in data" :row="row" /><slot name="cell-settlement_state" v-for="row in data" :row="row" /><slot name="cell-actions" v-for="row in data" :row="row" /></div>' }, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, Select: true, Pagination: true } } })

beforeEach(() => { delete routeQuery.private_id; routeMeta.requiresAdmin = false; replace.mockReset(); list.mockReset(); detail.mockReset(); usage.mockReset(); list.mockResolvedValue({ items: [item], total: 1, page: 1, page_size: 20 }) })
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

describe('exact correlation', () => {
 it.each([false, true])('keeps core details visible and labels an evidence read failure (admin=%s)', async (admin) => {
  routeMeta.requiresAdmin = admin
  detail.mockResolvedValue({ request: item, attempts: [{ attempt_no: 1, attempt_id: null }], billing: [], attempt_evidence_unavailable: true })
  const wrapper = mountView(); await flushPromises()
  await wrapper.findAll('button').find(b => b.text() === 'requestLedger.detail')!.trigger('click'); await flushPromises()
  expect(wrapper.text()).toContain(item.id)
  expect(wrapper.text()).toContain('requestLedger.evidenceUnavailable')
  expect(wrapper.text()).not.toContain('requestLedger.noExactEvidence')
 })
 const attempt = { attempt_no: 1, attempt_id: '10000000-0000-4000-8000-000000000001', upstream_request_id: 'req_fixture_one', upstream_response_id: 'resp_fixture_one', identifier_conflict: false, phase: 'request', execution_state: 'failed', usage_state: 'usage_unknown' }
 async function openFixture(admin: boolean) {
  routeMeta.requiresAdmin = admin
  detail.mockResolvedValue({ request: item, attempts: [attempt, { ...attempt, attempt_no: 2, attempt_id: null, upstream_request_id: null, upstream_response_id: null }], billing: [], diagnostics: [{ attempt_no: 1, ops_error_log_id: 42, available: true }, { attempt_no: 2, ops_error_log_id: 43, available: false }] })
  const wrapper = mountView(); await flushPromises()
  const button = wrapper.findAll('button').find(b => b.text() === 'requestLedger.detail')!
  await button.trigger('click'); await flushPromises()
  return wrapper
 }
 it('shows the exact attempt identity and retained or missing Ops references to admins', async () => {
  const wrapper = await openFixture(true)
  expect(wrapper.text()).toContain(attempt.attempt_id)
  expect(wrapper.text()).toContain('req_fixture_one')
  expect(wrapper.text()).toContain('resp_fixture_one')
  expect(wrapper.get('[data-testid="ledger-ops-42"]').text()).toContain('42')
  expect(wrapper.find('[data-testid="ledger-ops-43"]').exists()).toBe(false)
  expect(wrapper.text()).toContain('requestLedger.diagnosticExpired')
  expect(wrapper.text()).toContain('requestLedger.noExactEvidence')
 })
 it('does not show upstream or Ops internals on a user page even if supplied', async () => {
  const wrapper = await openFixture(false)
  expect(wrapper.text()).toContain(attempt.attempt_id)
  expect(wrapper.text()).not.toContain('req_fixture_one')
  expect(wrapper.text()).not.toContain('resp_fixture_one')
  expect(wrapper.find('[data-testid="ledger-ops-42"]').exists()).toBe(false)
 })
})

it('follows an exact Ops link when the same route component is reused', async () => {
 routeMeta.requiresAdmin = true
 detail.mockImplementation((_admin, id) => Promise.resolve({request:{...item,id},attempts:[],billing:[]}))
 const wrapper = mountView(); await flushPromises()
 routeQuery.private_id = '00000000-0000-4000-8000-000000000002'
 await flushPromises()
 expect(list).toHaveBeenLastCalledWith(true, expect.objectContaining({private_id:routeQuery.private_id}), expect.any(AbortSignal))
 expect(detail).toHaveBeenLastCalledWith(true, routeQuery.private_id, expect.any(AbortSignal))
 expect(wrapper.text()).toContain(routeQuery.private_id)
 wrapper.unmount()
})


describe('batch image usage drill', () => {
 async function openUsage(billingMode: string | null, imageCount: number | null) {
  detail.mockResolvedValue({request:item,attempts:[],billing:[{usage_log_id:42,billed_cost:6,settled:true,applied:false,batch_id:'imgbatch_fixture'}]})
  usage.mockResolvedValue({id:42,model:'batch-image-fixture',input_tokens:0,output_tokens:0,image_count:imageCount,billing_mode:billingMode,batch_id:'imgbatch_fixture',billed_cost:6})
  const wrapper=mountView(); await flushPromises()
  await wrapper.findAll('button').find(b=>b.text()==='requestLedger.detail')!.trigger('click'); await flushPromises()
  await wrapper.findAll('button').find(b=>b.text()==='requestLedger.usageLog #42')!.trigger('click'); await flushPromises()
  return wrapper
 }
 it.each([false,true])('shows batch settlement and actual image count without invented token usage (admin=%s)', async admin=>{
  routeMeta.requiresAdmin=admin
  const wrapper=await openUsage('image',3)
  expect(usage).toHaveBeenCalledWith(admin,item.id,42,expect.any(AbortSignal))
  expect(wrapper.text()).toContain('requestLedger.batchSettled')
  expect(wrapper.text()).toContain('requestLedger.imageCount')
  expect(wrapper.text()).toContain('imgbatch_fixture')
  expect(wrapper.text()).not.toContain('requestLedger.inputTokens')
  expect(wrapper.text()).not.toContain('requestLedger.outputTokens')
  expect(wrapper.text()).not.toContain('requestLedger.applied')
  wrapper.unmount()
 })
 it('keeps a missing image count unknown',async()=>{
  const wrapper=await openUsage('image',null)
  expect(wrapper.text()).toContain('requestLedger.imageCount')
  expect(wrapper.findAll('dt').find(dt=>dt.text()==='requestLedger.imageCount')!.element.nextElementSibling!.textContent).toBe('requestLedger.states.usage_unknown')
  wrapper.unmount()
 })
 it('preserves the existing token-mode drill',async()=>{
  const wrapper=await openUsage('token',null)
  expect(wrapper.text()).toContain('requestLedger.inputTokens')
  expect(wrapper.text()).toContain('requestLedger.outputTokens')
  expect(wrapper.text()).not.toContain('requestLedger.imageCount')
  wrapper.unmount()
 })
})
