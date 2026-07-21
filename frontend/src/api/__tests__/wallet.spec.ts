import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { getWalletSummary, getWalletTransactions } from '@/api/wallet'

describe('wallet API', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('loads the wallet summary from the user wallet endpoint', async () => {
    get.mockResolvedValueOnce({
      data: {
        available_balance: '42.50000000',
        withdrawable_balance: '12.00000000',
        pending_withdrawable_balance: '4.50000000',
        withdrawal_frozen_balance: '1.25000000',
        task_reserved_balance: '3.25000000',
        total_credits: '100.00000000',
        total_debits: '57.50000000',
        transaction_count: 5,
      },
    })

    const summary = await getWalletSummary()

    expect(get).toHaveBeenCalledWith('/user/wallet/summary')
    expect(summary.available_balance).toBe('42.50000000')
    expect(summary.withdrawable_balance).toBe('12.00000000')
  })

  it('passes source and pagination filters to the transaction endpoint', async () => {
    get.mockResolvedValueOnce({
      data: {
        items: [],
        total: 0,
        page: 2,
        page_size: 20,
        pages: 1,
      },
    })

    await getWalletTransactions({ source: 'team_reward', page: 2, page_size: 20 })

    expect(get).toHaveBeenCalledWith('/user/wallet/transactions', {
      params: { source: 'team_reward', page: 2, page_size: 20 },
    })
  })
})
