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
  | 'withdrawal'
  | 'other'

export type WalletDirection = 'credit' | 'debit' | 'neutral'

export interface WalletSummary {
  available_balance: string
  withdrawable_balance: string
  pending_withdrawable_balance: string
  withdrawal_frozen_balance: string
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
  withdrawable_delta?: string
  withdrawal_frozen_delta?: string
  balance_after: string
  frozen_after: string
  withdrawable_after?: string
  withdrawal_frozen_after?: string
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

export type WithdrawalPayoutMethod = 'alipay' | 'bank_transfer' | 'other'
export type WithdrawalCurrency = 'CNY' | 'USD'
export type WithdrawalStatus =
  | 'pending_review'
  | 'second_review'
  | 'payout_pending'
  | 'paid'
  | 'rejected'
  | 'canceled'

export interface WithdrawalAvailability {
  global_enabled: boolean
  user_enabled: boolean
  can_apply: boolean
  disabled_reason?: string
  recalc_status: string
  minimum_amount: string
  daily_limit_amount: string
  daily_used_amount: string
  remaining_daily_amount: string
}

export interface WithdrawalPayoutAccount {
  id: number
  method: WithdrawalPayoutMethod
  currency: WithdrawalCurrency
  recipient_name_mask: string
  account_mask: string
  created_at: string
  updated_at: string
}

export interface WithdrawalPayoutAccountInput {
  method: WithdrawalPayoutMethod
  currency: WithdrawalCurrency
  recipient_name: string
  details: Record<string, string>
}

export interface WithdrawalStatusEvent {
  id: number
  status: WithdrawalStatus
  actor_type: 'user' | 'admin' | string
  actor_user_id?: number
  note?: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface WithdrawalRequest {
  id: number
  request_no: string
  user_id?: number
  user_email?: string
  amount: string
  currency: WithdrawalCurrency | string
  status: WithdrawalStatus
  payout_method: WithdrawalPayoutMethod
  payout_currency: WithdrawalCurrency
  payout_account_mask: string
  payout_recipient_name_mask: string
  first_approved_by?: number
  first_approved_at?: string
  second_approved_by?: number
  second_approved_at?: string
  rejected_by?: number
  rejected_at?: string
  rejected_reason?: string
  canceled_at?: string
  paid_by?: number
  paid_at?: string
  paid_amount?: string
  paid_currency?: WithdrawalCurrency | string
  payout_fx_rate?: string
  external_txn_id?: string
  external_fee_amount: string
  payout_note?: string
  created_at: string
  updated_at: string
  events?: WithdrawalStatusEvent[]
}

export interface WithdrawalRequestPage {
  items: WithdrawalRequest[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface WithdrawalRequestQuery {
  status?: WithdrawalStatus | 'all'
  page?: number
  page_size?: number
}

export interface WithdrawalCreateInput {
  amount: string
}

export function normalizeWithdrawalWholeAmount(value: string): string {
  const trimmed = value.trim()
  const match = trimmed.match(/^(\d+)(?:\.0+)?$/)
  if (!match) return trimmed
  const normalized = match[1].replace(/^0+(?=\d)/, '')
  return normalized
}

export async function getWithdrawalAvailability(): Promise<WithdrawalAvailability> {
  const { data } = await apiClient.get<WithdrawalAvailability>('/user/wallet/withdrawals/availability')
  return data
}

export async function getWithdrawalAccount(): Promise<WithdrawalPayoutAccount | null> {
  const { data } = await apiClient.get<WithdrawalPayoutAccount | null>('/user/wallet/withdrawal-account')
  return data
}

export async function updateWithdrawalAccount(input: WithdrawalPayoutAccountInput): Promise<WithdrawalPayoutAccount> {
  const { data } = await apiClient.put<WithdrawalPayoutAccount>('/user/wallet/withdrawal-account', input)
  return data
}

export async function getWithdrawals(query: WithdrawalRequestQuery = {}): Promise<WithdrawalRequestPage> {
  const { data } = await apiClient.get<WithdrawalRequestPage>('/user/wallet/withdrawals', { params: query })
  return data
}

export async function getWithdrawal(id: number): Promise<WithdrawalRequest> {
  const { data } = await apiClient.get<WithdrawalRequest>(`/user/wallet/withdrawals/${id}`)
  return data
}

export async function createWithdrawal(input: WithdrawalCreateInput): Promise<WithdrawalRequest> {
  const { data } = await apiClient.post<WithdrawalRequest>('/user/wallet/withdrawals', {
    ...input,
    amount: normalizeWithdrawalWholeAmount(input.amount),
  })
  return data
}

export async function cancelWithdrawal(id: number, note?: string): Promise<WithdrawalRequest> {
  const path = `/user/wallet/withdrawals/${id}/cancel`
  const { data } = note ? await apiClient.post<WithdrawalRequest>(path, { note }) : await apiClient.post<WithdrawalRequest>(path)
  return data
}
