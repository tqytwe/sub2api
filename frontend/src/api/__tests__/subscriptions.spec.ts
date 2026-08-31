import { beforeEach, describe, expect, it } from 'vitest'

import { apiClient } from '../client'
import { getSubscriptionProgress, getSubscriptionsProgress } from '../subscriptions'
import { getProgress as getAdminSubscriptionProgress } from '../admin/subscriptions'

describe('subscription progress API boundary', () => {
  beforeEach(() => {
    apiClient.defaults.adapter = undefined
  })

  it('normalizes the current backend envelope and window field names into the frontend model', async () => {
    apiClient.defaults.adapter = async (config) => ({
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
      data: {
        code: 0,
        data: [{
          subscription: { id: 42, group_id: 7, status: 'active' },
          progress: {
            id: 42,
            group_name: 'Pro',
            expires_at: '2026-09-30T00:00:00Z',
            expires_in_days: 31,
            daily: {
              limit_usd: 10,
              used_usd: 2.5,
              remaining_usd: 7.5,
              percentage: 25,
              window_start: '2026-08-29T00:00:00Z',
              resets_at: '2026-08-30T00:00:00Z',
              resets_in_seconds: 3600,
            },
          },
        }],
      },
    })

    const [entry] = await getSubscriptionsProgress()

    expect(entry).toMatchObject({
      subscription: { id: 42, group_id: 7 },
      progress: {
        id: 42,
        groupName: 'Pro',
        expiresInDays: 31,
        daily: {
          limitUsd: 10,
          usedUsd: 2.5,
          remainingUsd: 7.5,
          resetsAt: '2026-08-30T00:00:00Z',
        },
      },
    })
  })

  it('accepts the legacy direct response shape without leaking legacy field names to callers', async () => {
    apiClient.defaults.adapter = async (config) => ({
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
      data: {
        code: 0,
        data: config.url === '/subscriptions/9/progress'
          ? {
              subscription_id: 9,
              days_remaining: 2,
              daily: { used: 1, limit: 4, percentage: 25, reset_in_seconds: 60 },
            }
          : [],
      },
    })

    await expect(getSubscriptionProgress(9)).resolves.toMatchObject({
      id: 9,
      expiresInDays: 2,
      daily: { usedUsd: 1, limitUsd: 4, remainingUsd: 3, resetsInSeconds: 60 },
    })
  })

  it('normalizes the administrator progress response through the same public model', async () => {
    apiClient.defaults.adapter = async (config) => ({
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
      data: {
        code: 0,
        data: {
          id: 15,
          group_name: 'Enterprise',
          expires_in_days: 20,
          monthly: {
            limit_usd: 100,
            used_usd: 60,
            remaining_usd: 40,
            percentage: 60,
            resets_in_seconds: 7200,
          },
        },
      },
    })

    await expect(getAdminSubscriptionProgress(15)).resolves.toMatchObject({
      id: 15,
      groupName: 'Enterprise',
      expiresInDays: 20,
      monthly: { usedUsd: 60, limitUsd: 100, remainingUsd: 40, resetsInSeconds: 7200 },
    })
  })
})
