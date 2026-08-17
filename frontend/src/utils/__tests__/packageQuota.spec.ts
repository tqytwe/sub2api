import { describe, expect, it } from 'vitest'

import { packageQuotaRows } from '../packageQuota'

describe('packageQuotaRows', () => {
  it('returns request, amount, and Token counters for a package entitlement', () => {
    expect(packageQuotaRows({
      id: 1,
      payment_order_id: 331,
      expires_at: '2026-09-16T17:19:00Z',
      status: 'active',
      exhausted_reason: null,
      request_limit: 12000,
      request_used: 25,
      amount_limit_usd: 700,
      amount_used_usd: 12.5,
      token_limit: 1_000_000_000,
      token_used: 456
    })).toEqual([
      { dimension: 'request', used: 25, limit: 12000 },
      { dimension: 'amount', used: 12.5, limit: 700 },
      { dimension: 'token', used: 456, limit: 1_000_000_000 }
    ])
  })
})
