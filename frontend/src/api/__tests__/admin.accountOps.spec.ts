import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getAccountOpsThresholdAccounts, testAccountOpsWebhook } from '../admin/accountOps'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('../client', () => ({ apiClient: client }))

describe('account operations API', () => {
  beforeEach(() => vi.resetAllMocks())

  it('loads account threshold data without exposing credential fields', async () => {
    client.get.mockResolvedValue({ data: { items: [{ account_id: 1, balance: null, balance_status: 'unknown' }] } })
    await expect(getAccountOpsThresholdAccounts()).resolves.toEqual([
      { account_id: 1, balance: null, balance_status: 'unknown' }
    ])
    expect(client.get).toHaveBeenCalledWith('/admin/account-ops/threshold-accounts')
  })

  it('requests webhook test delivery through the admin endpoint', async () => {
    client.post.mockResolvedValue({ data: { sent: true } })
    await testAccountOpsWebhook('hook/1')
    expect(client.post).toHaveBeenCalledWith('/admin/account-ops/webhooks/hook%2F1/test')
  })
})
