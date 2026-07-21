import { apiClient } from '@/api/client'

export type WalletSource =
  | 'all'
  | 'recharge'
  | 'redeem'
  | 'affiliate'
  | 'team_reward'
  | 'arena_reward'
  | 'checkin'
  | 'quiz'
  | 'blind_box'
  | 'usage'
  | 'image_task'
  | 'refund'
  | 'admin_adjustment'
  | 'promotion'
  | 'subscription'
  | 'other'

export type WalletDirection = 'credit' | 'debit' | 'neutral'

export interface WalletSummary {
  available_balance: string
  task_reserved_balance: string
  total_credits: string
  total_debits: string
  transaction_count: number
  last_transaction_at?: string
}

export interface WalletTransaction {
  id: number
  source: Exclude<WalletSource, 'all'>
  direction: WalletDirection
  balance_delta: string
  frozen_delta: string
  balance_after: string
  frozen_after: string
  created_at: string
}

export interface WalletTransactionPage {
  items: WalletTransaction[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface WalletTransactionQuery {
  source?: WalletSource
  page?: number
  page_size?: number
}

export async function getWalletSummary(): Promise<WalletSummary> {
  const { data } = await apiClient.get<WalletSummary>('/user/wallet/summary')
  return data
}

export async function getWalletTransactions(query: WalletTransactionQuery = {}): Promise<WalletTransactionPage> {
  const params = {
    source: query.source,
    page: query.page,
    page_size: query.page_size,
  }
  const { data } = await apiClient.get<WalletTransactionPage>('/user/wallet/transactions', { params })
  return data
}
