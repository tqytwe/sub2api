import { expect, it } from 'vitest'
import { isRoutingVeto, parseRoutingEvidence, routingCoverageV1 } from '../requestRouting'
const id = '00000000-0000-4000-8000-000000000001'
const event = { selection_no: 1, stage: 'initial_filter', reason: 'model_rate_limited', value: 0, at: '2026-10-11T00:00:00Z' }
const evidence = () => ({ request_id: id, coverage_version: routingCoverageV1, status: 'complete', selection_count: 1, event_count: 1, veto_count: 1, omitted_count: 0, incomplete: false, source_population_complete: false, events: [event] })
it.each(['selection_started','selection_selected','selection_wait','selection_failed','prefiltered_schedulable_pool','pool_read_failed','wait_counter_failed','load_read_failed','unknown_future_reason'])('never promotes observation or unknown reason %s into veto', reason => {
  expect(isRoutingVeto(routingCoverageV1, {...event,reason})).toBe(false)
})
it('requires the known coverage version for veto interpretation', () => {
  expect(isRoutingVeto(routingCoverageV1,event)).toBe(true)
  expect(isRoutingVeto('future_v2',event)).toBe(false)
  expect(isRoutingVeto('',event)).toBe(false)
})
it.each(['pending','incomplete'])('retains valid %s summary/returned count mismatch', status => {
  const value = {...evidence(),status,events:[]}
  expect(parseRoutingEvidence(value,id)).toEqual(value)
})
it.each([
  {events:Array.from({length:129},()=>event)}, {events:[{...event,reason:'<script>secret</script>'}]},
  {events:[{...event,account_id:-1}]}, {events:[{...event,at:'invalid'}]},
  {coverage_version:'x'.repeat(97)}, {event_count:129}, {omitted_count:-1}, {status:'success'},
])('rejects unbounded or invalid contract fields %j', invalid => {
  expect(()=>parseRoutingEvidence({...evidence(),...invalid},id)).toThrow('Unsupported routing evidence response')
})
