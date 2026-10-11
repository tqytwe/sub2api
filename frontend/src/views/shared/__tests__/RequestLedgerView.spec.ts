import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { createPinia, disposePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import type { User } from '@/types'
const piniaInstances: ReturnType<typeof createPinia>[] = []
enableAutoUnmount(afterEach)
afterEach(() => { for (const pinia of piniaInstances.splice(0)) disposePinia(pinia) })
import { createI18n } from 'vue-i18n'
import RequestLedgerView from '../RequestLedgerView.vue'

const { list, detail, usage, sectionPage, replace, routeMeta } = vi.hoisted(() => ({ list: vi.fn(), detail: vi.fn(), usage: vi.fn(), sectionPage: vi.fn(), replace: vi.fn(), routeMeta: { requiresAdmin: false } }))
vi.mock('@/api/requestLedger', () => ({ requestLedgerAPI: { list, detail, usage, sectionPage } }))
const routeQuery = reactive<Record<string, string>>({ lang: 'en' })
vi.mock('vue-router', () => ({ useRoute: () => ({ meta: routeMeta, query: routeQuery }), useRouter: () => ({ replace }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

const item = { id: '00000000-0000-4000-8000-000000000001', route: '/v1/responses', method: 'POST', kind: 'http', user_id: 101, api_key_id: 201, started_at: '2026-10-10T00:00:00Z', ended_at: null, execution_state: 'interrupted', usage_state: 'usage_unknown', settlement_state: 'settlement_pending', attempt_count: 1, output_observed: true, error_code: 'interrupted', http_status: 200 }
const mountView = () => {
 const pinia = createPinia(), auth = useAuthStore(pinia)
 piniaInstances.push(pinia)
 auth.token = 'fixture-only'; auth.user = { id: 101, role: routeMeta.requiresAdmin ? 'admin' : 'user' } as User
 localStorage.setItem('auth_token', auth.token); localStorage.setItem('auth_user', JSON.stringify(auth.user))
 return mount(RequestLedgerView, { global: { plugins: [pinia, createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: { en: {} } })], stubs: { RouterLink: { template: '<a><slot /></a>' }, DataTable: { props: ['data'], template: '<div><slot name="cell-execution_state" v-for="row in data" :row="row" /><slot name="cell-usage_state" v-for="row in data" :row="row" /><slot name="cell-settlement_state" v-for="row in data" :row="row" /><slot name="cell-actions" v-for="row in data" :row="row" /></div>' }, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, Select: true, Pagination: true } } })
}

beforeEach(() => { delete routeQuery.private_id; routeMeta.requiresAdmin = false; replace.mockReset(); list.mockReset(); detail.mockReset(); usage.mockReset(); sectionPage.mockReset(); sectionPage.mockImplementation(async (_admin, _id, section) => { const old = await detail.mock.results.at(-1)?.value; return { items: old?.[section] ?? [], page_size: 20, has_more: false, next_cursor: '', consistency: 'live', refresh_hint: 'new_records_late_evidence_and_state_changes', attempt_evidence_unavailable: old?.attempt_evidence_unavailable } }); list.mockResolvedValue({ items: [item], total: 1, page: 1, page_size: 20 }) })
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


describe('independent detail pagination', () => {
 it('loads bounded sections and retries one failed section without hiding the others', async () => {
  detail.mockResolvedValue({ request:item, attempts:[], billing:[], wallet:[] })
  let failBilling = true
  sectionPage.mockImplementation(async (_admin, _id, section) => {
   if(section === 'billing' && failBilling) throw new Error('synthetic read failure')
   return {items:[],page_size:20,has_more:false,next_cursor:'',consistency:'live',refresh_hint:'new_records_late_evidence_and_state_changes'}
  })
  const wrapper=mountView(); await flushPromises()
  await wrapper.findAll('button').find(b=>b.text()==='requestLedger.detail')!.trigger('click'); await flushPromises()
  expect(sectionPage.mock.calls.map(c=>c[2]).sort()).toEqual(['attempts','billing','wallet'])
  expect(wrapper.get('[data-testid="ledger-section-billing"] [role="alert"]').text()).toContain('requestLedger.sectionError')
  expect(wrapper.get('[data-testid="ledger-section-attempts"]').text()).not.toContain('requestLedger.sectionError')
  expect(wrapper.text()).toContain(item.id)
  failBilling=false
  await wrapper.get('[data-testid="ledger-section-billing"] [data-action="retry"]').trigger('click'); await flushPromises()
  expect(sectionPage).toHaveBeenCalledTimes(4)
  expect(wrapper.find('[data-testid="ledger-section-billing"] [role="alert"]').exists()).toBe(false)
  wrapper.unmount()
 })
})

function sectionResult(items: unknown[] = [], next = '') {
 return {items,page_size:20,has_more:!!next,next_cursor:next,consistency:'live',refresh_hint:'new_records_late_evidence_and_state_changes'}
}
async function openPagedView() {
 detail.mockResolvedValue({request:item})
 const wrapper=mountView(); await flushPromises()
 await wrapper.findAll('button').find(b=>b.text()==='requestLedger.detail')!.trigger('click'); await flushPromises()
 return wrapper
}

describe('detail cursor interactions',()=>{
 it('blocks old-page usage reads while billing changes and ignores the aborted drill reply',async()=>{
  let releasePage!:(value:ReturnType<typeof sectionResult>)=>void
  let releaseUsage!:(value:unknown)=>void
  let usageSignal:AbortSignal|undefined
  sectionPage.mockImplementation(async(_admin,_id,section,cursor)=>{
   if(section!=='billing')return sectionResult()
   if(cursor)return new Promise(resolve=>{releasePage=resolve})
   return sectionResult([{usage_log_id:42,billed_cost:6,settled:true,batch_id:'imgbatch_fixture'}],'next')
  })
  usage.mockImplementation(async(_admin,_id,_usageID,signal)=>{
   usageSignal=signal
   return new Promise(resolve=>{releaseUsage=resolve})
  })
  const w=await openPagedView(),region=w.get('[data-testid="ledger-section-billing"]')
  const drill=()=>region.findAll('button').find(b=>b.text()==='requestLedger.usageLog #42')!
  await drill().trigger('click');await flushPromises()
  await region.get('[data-action="next"]').trigger('click');await flushPromises()
  expect(usageSignal?.aborted).toBe(true)
  expect(drill().attributes('disabled')).toBeDefined()
  await drill().trigger('click');await flushPromises()
  expect(usage).toHaveBeenCalledTimes(1)
  releasePage(sectionResult());await flushPromises()
  releaseUsage({id:42,model:'stale-usage-model',input_tokens:0,output_tokens:0,billing_mode:'image',image_count:3,billed_cost:6});await flushPromises()
  expect(w.text()).not.toContain('stale-usage-model')
  w.unmount()
 })
 it('advances an empty intermediate page without refetching other sections and refreshes from the beginning',async()=>{
  let refreshed=false
  sectionPage.mockImplementation(async(_admin,_id,section,cursor)=>{
   if(section!=='attempts')return sectionResult()
   if(cursor==='next')return sectionResult([{attempt_no:22,attempt_id:'attempt-later'}])
   return sectionResult(refreshed?[{attempt_no:1,attempt_id:'attempt-arrived-late'}]:[],'next')
  })
  const w=await openPagedView(), region=w.get('[data-testid="ledger-section-attempts"]')
  expect(region.text()).toContain('requestLedger.emptyPageMore')
  expect(region.get('[data-action="next"]').attributes('disabled')).toBeUndefined()
  await region.get('[data-action="next"]').trigger('click');await flushPromises()
  expect(region.text()).toContain('attempt-later')
  expect(sectionPage.mock.calls.filter(c=>c[2]==='billing')).toHaveLength(1)
  expect(sectionPage.mock.calls.filter(c=>c[2]==='wallet')).toHaveLength(1)
  refreshed=true
  await region.get('[data-action="refresh"]').trigger('click');await flushPromises()
  expect(sectionPage).toHaveBeenLastCalledWith(false,item.id,'attempts','',expect.any(AbortSignal))
  expect(region.text()).toContain('attempt-arrived-late')
  expect(region.text()).not.toContain('attempt-later')
  expect(region.get('[data-action="previous"]').attributes('disabled')).toBeDefined()
  w.unmount()
 })
 it('preserves the successful page on a failed next read and retries that cursor',async()=>{
  let fail=true
  sectionPage.mockImplementation(async(_admin,_id,section,cursor)=>{
   if(section!=='attempts')return sectionResult()
   if(cursor==='next'&&fail)throw Error('synthetic failure')
   return cursor?sectionResult([{attempt_no:2,attempt_id:'attempt-two'}]):sectionResult([{attempt_no:1,attempt_id:'attempt-one'}],'next')
  })
  const w=await openPagedView(),region=w.get('[data-testid="ledger-section-attempts"]')
  await region.get('[data-action="next"]').trigger('click');await flushPromises()
  expect(region.text()).toContain('attempt-one')
  expect(region.get('[role="alert"]').text()).toContain('requestLedger.sectionError')
  fail=false
  await region.get('[data-action="retry"]').trigger('click');await flushPromises()
  expect(sectionPage).toHaveBeenLastCalledWith(false,item.id,'attempts','next',expect.any(AbortSignal))
  expect(region.text()).toContain('attempt-two');expect(region.text()).not.toContain('attempt-one')
  await region.get('[data-action="previous"]').trigger('click');await flushPromises()
  expect(region.text()).toContain('attempt-one')
  w.unmount()
 })
 it('disables navigation while loading and ignores a late reply after switching requests',async()=>{
  let release!:(value:ReturnType<typeof sectionResult>)=>void
  let oldSignal:AbortSignal|undefined
  sectionPage.mockImplementation(async(_admin,id,section,_cursor,signal)=>{
   if(section==='attempts'&&id===item.id){oldSignal=signal;return new Promise(resolve=>{release=resolve})}
   return sectionResult(section==='attempts'?[{attempt_no:1,attempt_id:'current-request-attempt'}]:[])
  })
  const w=await openPagedView()
  const oldRegion=w.get('[data-testid="ledger-section-attempts"]')
  expect(oldRegion.attributes('aria-busy')).toBe('true')
  expect(oldRegion.get('[data-action="next"]').attributes('disabled')).toBeDefined()
  expect(oldRegion.get('[data-action="refresh"]').attributes('disabled')).toBeDefined()
  const newID='00000000-0000-4000-8000-000000000002'
  detail.mockResolvedValue({request:{...item,id:newID}})
  routeQuery.private_id=newID;await flushPromises()
  expect(oldSignal?.aborted).toBe(true)
  release(sectionResult([{attempt_no:99,attempt_id:'stale-request-attempt'}]));await flushPromises()
  expect(w.text()).toContain('current-request-attempt');expect(w.text()).not.toContain('stale-request-attempt')
  expect(w.text()).toContain(newID)
  w.unmount()
 })
 it('keeps admin diagnostic errors local and aborts all pending reads on close',async()=>{
  routeMeta.requiresAdmin=true
  const signals:AbortSignal[]=[]
  sectionPage.mockImplementation(async(_admin,_id,section,_cursor,signal)=>{
   signals.push(signal)
   if(section==='diagnostics')throw Error('synthetic diagnostics unavailable')
   return sectionResult()
  })
  const w=await openPagedView()
  expect(w.get('[data-testid="ledger-section-diagnostics"] [role="alert"]').exists()).toBe(true)
  expect(w.get('[data-testid="ledger-section-billing"] [data-action="refresh"]').exists()).toBe(true)
  w.unmount()
  expect(signals.every(s=>s.aborted)).toBe(true)
 })
 it('shows an error for an old or malformed page response instead of treating it as complete',async()=>{
  sectionPage.mockResolvedValue({request:item,attempts:[],billing:[]})
  const w=await openPagedView()
  expect(w.get('[data-testid="ledger-section-attempts"] [role="alert"]').text()).toContain('requestLedger.sectionError')
  expect(w.get('[data-testid="ledger-section-attempts"] [data-action="next"]').attributes('disabled')).toBeDefined()
  w.unmount()
 })
})
