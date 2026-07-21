import { apiClient } from '../client'
import type {
  WithdrawalCurrency,
  WithdrawalRequest,
  WithdrawalRequestPage,
  WithdrawalStatus,
} from '../wallet'

export interface AdminWithdrawalListQuery {
  status?: WithdrawalStatus | 'all'
  user_id?: number
  page?: number
  page_size?: number
}

export interface AdminWithdrawalSystemSettings {
  global_enabled: boolean
  minimum_amount: string
  daily_limit_amount: string
  double_review_threshold: string
  reward_maturity_hours: number
  updated_by?: number
  updated_at: string
}

export interface AdminWithdrawalSystemSettingsInput {
  global_enabled?: boolean
  minimum_amount?: string
  daily_limit_amount?: string
  double_review_threshold?: string
}

export interface AdminUserWithdrawalSettings {
  user_id: number
  enabled: boolean
  minimum_amount_override?: string
  daily_limit_amount_override?: string
  disabled_reason: string
  updated_by?: number
  updated_at: string
  recalc_status?: string
}

export interface AdminUserWithdrawalSettingsInput {
  enabled?: boolean
  minimum_amount_override?: string
  minimum_override?: string
  daily_limit_amount_override?: string
  daily_limit_override?: string
  disabled_reason?: string
}

export interface AdminBatchUserWithdrawalSettingsInput extends AdminUserWithdrawalSettingsInput {
  user_ids: number[]
}

export interface AdminBatchUserWithdrawalSettingsResult {
  affected: number
}

export interface AdminWithdrawalActionInput {
  note?: string
  reason?: string
}

export interface AdminWithdrawalMarkPaidInput {
  paid_amount: string
  paid_currency: WithdrawalCurrency
  payout_fx_rate?: string
  external_txn_id: string
  paid_at?: string
  note?: string
}

export type AdminWithdrawalSensitivePayout = Record<string, unknown>

export async function list(query: AdminWithdrawalListQuery = {}): Promise<WithdrawalRequestPage> {
  const { data } = await apiClient.get<WithdrawalRequestPage>('/admin/withdrawals', { params: query })
  return data
}

export async function get(id: number): Promise<WithdrawalRequest> {
  const { data } = await apiClient.get<WithdrawalRequest>(`/admin/withdrawals/${id}`)
  return data
}

export async function getSettings(): Promise<AdminWithdrawalSystemSettings> {
  const { data } = await apiClient.get<AdminWithdrawalSystemSettings>('/admin/withdrawals/settings')
  return data
}

export async function updateSettings(input: AdminWithdrawalSystemSettingsInput): Promise<AdminWithdrawalSystemSettings> {
  const { data } = await apiClient.put<AdminWithdrawalSystemSettings>('/admin/withdrawals/settings', input)
  return data
}

export async function getUserSettings(userID: number): Promise<AdminUserWithdrawalSettings> {
  const { data } = await apiClient.get<AdminUserWithdrawalSettings>(`/admin/withdrawals/users/${userID}/settings`)
  return data
}

export async function updateUserSettings(userID: number, input: AdminUserWithdrawalSettingsInput): Promise<AdminUserWithdrawalSettings> {
  const { data } = await apiClient.put<AdminUserWithdrawalSettings>(`/admin/withdrawals/users/${userID}/settings`, input)
  return data
}

export async function batchUpdateUserSettings(input: AdminBatchUserWithdrawalSettingsInput): Promise<AdminBatchUserWithdrawalSettingsResult> {
  const { data } = await apiClient.post<AdminBatchUserWithdrawalSettingsResult>('/admin/withdrawals/user-settings/batch', input)
  return data
}

export async function approve(id: number, input: AdminWithdrawalActionInput = {}): Promise<WithdrawalRequest> {
  const { data } = await apiClient.post<WithdrawalRequest>(`/admin/withdrawals/${id}/approve`, input)
  return data
}

export async function reject(id: number, input: Required<Pick<AdminWithdrawalActionInput, 'reason'>> & AdminWithdrawalActionInput): Promise<WithdrawalRequest> {
  const { data } = await apiClient.post<WithdrawalRequest>(`/admin/withdrawals/${id}/reject`, input)
  return data
}

export async function getSensitivePayout(id: number): Promise<AdminWithdrawalSensitivePayout> {
  const { data } = await apiClient.get<AdminWithdrawalSensitivePayout>(`/admin/withdrawals/${id}/payout-sensitive`)
  return data
}

export async function markPaid(id: number, input: AdminWithdrawalMarkPaidInput): Promise<WithdrawalRequest> {
  const { data } = await apiClient.post<WithdrawalRequest>(`/admin/withdrawals/${id}/mark-paid`, input)
  return data
}

export default {
  list,
  get,
  getSettings,
  updateSettings,
  getUserSettings,
  updateUserSettings,
  batchUpdateUserSettings,
  approve,
  reject,
  getSensitivePayout,
  markPaid,
}
