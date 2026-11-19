import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AutoConfigHistory from '../AutoConfigHistory.vue'
const events = vi.hoisted(() => vi.fn())
vi.mock('@/api/admin/autoConfig', () => ({ getAutoConfigEvents: events }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({user: {id: 1,role: 'admin'}}) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual('vue-i18n'), useI18n: () => ({ t: (k: string) => k }) }))
const item = (id: number, kind = 'config_saved') => ({id,kind,account_id: 7,account_name: 'fixture',platform: 'openai',created_at: '2026-11-18T00:00:00Z',details: {}})
describe('automatic configuration audit history', () => {
 beforeEach(() => events.mockReset())
 it('pages in descending ID order and retries a failed next page without clearing existing events', async () => {
  events.mockResolvedValueOnce({items: [item(20)],has_more: true}).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({items: [item(10)],has_more: false})
  const w=mount(AutoConfigHistory,{props:{refreshKey:0}});await flushPromises()
  await w.get('[data-testid="history-more"]').trigger('click');await flushPromises()
  expect(w.findAll('[data-testid="history-row"]')).toHaveLength(1)
  await w.get('[data-testid="history-retry"]').trigger('click');await flushPromises()
  expect(events).toHaveBeenLastCalledWith({limit:20,kind:'',before:20})
  expect(w.findAll('[data-testid="history-row"]')).toHaveLength(2)
 })
 it('provides filter and summaries for fallback opening and manual recovery', async () => {
  events.mockResolvedValue({items:[{...item(5,'manual_recovery'),details:{recovered_controls:['concurrency']}},{...item(4,'apikey_fallback_opened'),details:{fallback_reason:'oauth_unavailable'}}],has_more:false})
  const w=mount(AutoConfigHistory,{props:{refreshKey:0}});await flushPromises()
  expect(w.text()).toContain('autoConfig.logs.manualRecovery')
  expect(w.text()).toContain('autoConfig.apikeyFallback.event')
  await w.get('[data-testid="history-kind"]').setValue('manual_recovery');await flushPromises()
  expect(events).toHaveBeenLastCalledWith({limit:20,kind:'manual_recovery',before:undefined})
 })
})
