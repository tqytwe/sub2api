import { apiClient } from '../client'
import type {
  WithdrawalCurrency,
  WithdrawalRequest,
  WithdrawalRequestPage,
  WithdrawalStatus,
} from '../wallet'
import { normalizeWithdrawalWholeAmount } from '../wallet'

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

function normalizeOptionalWholeAmount(value: string | undefined): string | undefined {
  if (value === undefined) return undefined
  return normalizeWithdrawalWholeAmount(value)
}

function normalizeSystemSettingsInput(input: AdminWithdrawalSystemSettingsInput): AdminWithdrawalSystemSettingsInput {
  return {
    ...input,
    minimum_amount: normalizeOptionalWholeAmount(input.minimum_amount),
    daily_limit_amount: normalizeOptionalWholeAmount(input.daily_limit_amount),
    double_review_threshold: normalizeOptionalWholeAmount(input.double_review_threshold),
  }
}

function normalizeUserSettingsInput(input: AdminUserWithdrawalSettingsInput): AdminUserWithdrawalSettingsInput {
  return {
    ...input,
    minimum_amount_override: normalizeOptionalWholeAmount(input.minimum_amount_override),
    minimum_override: normalizeOptionalWholeAmount(input.minimum_override),
    daily_limit_amount_override: normalizeOptionalWholeAmount(input.daily_limit_amount_override),
    daily_limit_override: normalizeOptionalWholeAmount(input.daily_limit_override),
  }
}

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
  const { data } = await apiClient.put<AdminWithdrawalSystemSettings>('/admin/withdrawals/settings', normalizeSystemSettingsInput(input))
  return data
}

export async function getUserSettings(userID: number): Promise<AdminUserWithdrawalSettings> {
  const { data } = await apiClient.get<AdminUserWithdrawalSettings>(`/admin/withdrawals/users/${userID}/settings`)
  return data
}

export async function updateUserSettings(userID: number, input: AdminUserWithdrawalSettingsInput): Promise<AdminUserWithdrawalSettings> {
  const { data } = await apiClient.put<AdminUserWithdrawalSettings>(`/admin/withdrawals/users/${userID}/settings`, normalizeUserSettingsInput(input))
  return data
}

export async function batchUpdateUserSettings(input: AdminBatchUserWithdrawalSettingsInput): Promise<AdminBatchUserWithdrawalSettingsResult> {
  const { data } = await apiClient.post<AdminBatchUserWithdrawalSettingsResult>('/admin/withdrawals/user-settings/batch', {
    ...normalizeUserSettingsInput(input),
    user_ids: input.user_ids,
  })
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
  const { data } = await apiClient.post<WithdrawalRequest>(`/admin/withdrawals/${id}/mark-paid`, {
    ...input,
    paid_amount: normalizeWithdrawalWholeAmount(input.paid_amount),
  })
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
