import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
vi.hoisted(() => { vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true) })
import { createI18n } from 'vue-i18n'
import { nextTick } from 'vue'
import UserKeyRiskNotices from '../UserKeyRiskNotices.vue'
import { userKeyRiskApi, type UserKeyRiskNotice } from '../keyRiskApi'

enableAutoUnmount(afterEach)
vi.mock('../keyRiskApi', () => ({ userKeyRiskApi: { list: vi.fn(), acknowledge: vi.fn() } }))
const notice = (): UserKeyRiskNotice => ({ event_id: 4, generation: 2, rule: 'key_ip_spread', state: 'pending', key_ids: [11], keys: [{ id: 11, name: 'Production key' }], first_seen: '2026-11-07T01:00:00Z', last_seen: '2026-11-07T01:01:00Z', attempts: 1, max_reminders: 3, acknowledged_at: null })
const messages = { ipAudit: { keyRisk: { userTitle: 'API key risk notices', noticeCount: '{count} notices', expandNotices: 'Expand', collapseNotices: 'Collapse', userHint: 'Review recent key source activity.', error: 'Unable to update notice.', noNotices: 'No notices', key_ip_spread: 'One key used from multiple IPs', keys: 'Keys', attempts: '{count}/{max}', acknowledged: 'Acknowledged', acknowledge: 'Acknowledge', ackConfirm: 'Confirm this notice.', ackSubmit: 'Confirm', limitReached: 'Reminder limit reached', pending: 'Pending' } }, common: { refresh: 'Refresh', cancel: 'Cancel' } }
function render() { return mount(UserKeyRiskNotices, { attachTo: document.body, global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: messages } })] } }) }
beforeEach(() => { vi.resetAllMocks(); vi.mocked(userKeyRiskApi.list).mockResolvedValue({ items: [notice()] }); vi.mocked(userKeyRiskApi.acknowledge).mockResolvedValue() })
describe('API key risk acknowledgement contract', () => {
  it('requires confirmation', async () => { const w = render(); await flushPromises(); await w.get('[data-test="toggle-risk-notices"]').trigger('click'); await w.get('[data-test="acknowledge-risk"]').trigger('click'); expect(userKeyRiskApi.acknowledge).not.toHaveBeenCalled(); await w.get('[data-test="confirm-risk"]').trigger('click'); await flushPromises(); expect(userKeyRiskApi.acknowledge).toHaveBeenCalledWith(notice()) })
  it('keeps retry after failure', async () => { vi.mocked(userKeyRiskApi.acknowledge).mockRejectedValue(new Error('stale')); const w = render(); await flushPromises(); await w.get('[data-test="toggle-risk-notices"]').trigger('click'); await w.get('[data-test="acknowledge-risk"]').trigger('click'); await w.get('[data-test="confirm-risk"]').trigger('click'); await flushPromises(); expect(w.get('[role="alert"]').exists()).toBe(true); expect(w.get('[data-test="confirm-risk"]').exists()).toBe(true) })
  it('prevents duplicate submissions', async () => { let resolve!: () => void; vi.mocked(userKeyRiskApi.acknowledge).mockImplementation(() => new Promise<void>(r => { resolve = r })); const w = render(); await flushPromises(); await w.get('[data-test="toggle-risk-notices"]').trigger('click'); await w.get('[data-test="acknowledge-risk"]').trigger('click'); const c = w.get('[data-test="confirm-risk"]'); await c.trigger('click'); await nextTick(); await c.trigger('click'); expect(userKeyRiskApi.acknowledge).toHaveBeenCalledTimes(1); resolve(); await flushPromises() })
})
