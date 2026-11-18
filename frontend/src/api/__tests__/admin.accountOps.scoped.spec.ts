import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  deleteAccountOpsRule,
  deleteAccountOpsWebhook,
  saveAccountOpsNotificationSettings,
  saveAccountOpsRule,
  saveAccountOpsWebhook
} from '../admin/accountOps'

const client = vi.hoisted(() => ({ put: vi.fn(), delete: vi.fn() }))
vi.mock('../client', () => ({ apiClient: client }))

describe('scoped account operations updates', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    client.put.mockResolvedValue({ data: { enabled: false } })
    client.delete.mockResolvedValue({ data: { enabled: false } })
  })

  it('saves an individual rule and preserves independent alert/recovery choices', async () => {
    const rule = { metric: 'balance' as const, enabled: true, threshold: 5, unit: 'USD', notify_alert: false, notify_recovery: true }
    await saveAccountOpsRule(12, rule)
    expect(client.put).toHaveBeenCalledWith('/admin/account-ops/rules/12', rule)
    await deleteAccountOpsRule(12, 'quota')
    expect(client.delete).toHaveBeenCalledWith('/admin/account-ops/rules/12', { params: { metric: 'quota' } })
  })

  it('updates a webhook without posting unrelated rules or credentials', async () => {
    const input = { provider: 'wecom' as const, enabled: true, url: 'https://example.test/hook' }
    await saveAccountOpsWebhook('hook/a', input)
    expect(client.put).toHaveBeenCalledWith('/admin/account-ops/webhooks/hook%2Fa', input)
    await deleteAccountOpsWebhook('hook/a')
    expect(client.delete).toHaveBeenCalledWith('/admin/account-ops/webhooks/hook%2Fa')
  })

  it('writes only shared notification settings', async () => {
    const settings = { enabled: false, recipient: '', balance_low: true, weekly_quota: false, cooldown_minutes: 60 }
    await saveAccountOpsNotificationSettings(settings)
    expect(client.put).toHaveBeenCalledWith('/admin/account-ops/notification-settings', settings)
  })
})
