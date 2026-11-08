/**
 * Admin Affiliate API endpoints
 * Manage per-user affiliate (邀请返利) configurations:
 * exclusive invite codes (overrides aff_code) and exclusive rebate rates.
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface AffiliateAdminEntry {
  user_id: number
  email: string
  username: string
  aff_code: string
  aff_code_custom: boolean
  aff_rebate_rate_percent?: number | null
  aff_count: number
}

export interface ListAffiliateUsersParams {
  page?: number
  page_size?: number
  search?: string
}

export interface ListAffiliateRecordsParams {
  page?: number
  page_size?: number
  search?: string
  start_at?: string
  end_at?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
  timezone?: string
  inviter_id?: number
  invitee_id?: number
  user_id?: number
  source_order_id?: number
  action?: 'accrue' | 'revoke' | 'transfer' | 'withdraw'
  source_type?: string
  status?: string
  currency?: string
  campaign_id?: number
}

export type AffiliateRecordType = 'invites' | 'rebates' | 'transfers'
export interface AffiliateRecordsSummary {
  record_count: number
  total_amount: number | null
  currency: 'USD' | 'mixed' | 'n/a'
  available_amount?: number
  frozen_amount?: number
  amounts: Array<{ source_type: 'regular' | 'campaign' | 'other' | 'wallet'; currency: string; total_amount: number; available_amount?: number; frozen_amount?: number }>
}

export interface AffiliateInviteRecord {
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number
  invitee_email: string
  invitee_username: string
  aff_code: string
  total_rebate: number
  created_at: string
}

/**
 * One rebate accrual. Non-order sources (redeem codes, admin recharges) carry no
 * order, so order fields are null; invitee_id is null when the invitee account
 * was deleted.
 */
export interface AffiliateRebateRecord {
  ledger_id?: number
  currency?: string
  source_type?: string
  action?: string
  status?: string
  frozen_until?: string
  campaign_id?: number
  order_id: number | null
  out_trade_no: string
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number | null
  invitee_email: string
  invitee_username: string
  order_amount: number | null
  pay_amount: number | null
  rebate_amount: number
  payment_type: string
  order_status: string
  created_at: string
}

/** transfer = user moved quota into balance; withdraw = admin-recorded offline withdrawal. */
export type AffiliateOutflowAction = 'transfer' | 'withdraw'

/** One affiliate quota outflow: a transfer into balance or an offline withdrawal. */
export interface AffiliateTransferRecord {
  ledger_id: number
  action: AffiliateOutflowAction
  user_id: number
  user_email: string
  username: string
  amount: number
  balance_after?: number | null
  available_quota_after?: number | null
  frozen_quota_after?: number | null
  history_quota_after?: number | null
  snapshot_available: boolean
  created_at: string
  external_reference?: string
  paid_at?: string
  note?: string
  actor_id?: number
}

export interface WithdrawAffiliateQuotaRequest {
  /** Amount already paid to the user outside the site (USD). */
  amount: number
  external_reference?: string
  paid_at?: string
  note?: string
}

export interface AffiliateWithdrawResult {
  ledger_id: number
  user_id: number
  amount: number
  available_quota_after: number
  frozen_quota_after: number
  history_quota_after: number
  external_reference?: string
  paid_at?: string
  note?: string
  actor_id?: number
}

export interface AffiliateWithdrawResponse {
  result: AffiliateWithdrawResult
  /** True when the key matched an earlier registration and nothing was deducted again. */
  replayed: boolean
}

export interface AffiliateUserOverview {
  user_id: number
  email: string
  username: string
  aff_code: string
  rebate_rate_percent: number
  rebate_rate_custom?: boolean
  invited_count: number
  rebated_invitee_count: number
  available_quota: number
  history_quota: number
}

export interface UpdateAffiliateUserRequest {
  aff_code?: string
  aff_rebate_rate_percent?: number | null
  /** Set true to explicitly clear the per-user rate (sets it to NULL). */
  clear_rebate_rate?: boolean
}

export interface BatchSetRateRequest {
  user_ids: number[]
  aff_rebate_rate_percent?: number | null
  /** Set true to clear rates instead of setting. */
  clear?: boolean
}

export interface SimpleUser {
  id: number
  email: string
  username: string
}

export async function listUsers(
  params: ListAffiliateUsersParams = {},
): Promise<PaginatedResponse<AffiliateAdminEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateAdminEntry>>(
    '/admin/affiliates/users',
    {
      params: {
        page: params.page ?? 1,
        page_size: params.page_size ?? 20,
        search: params.search ?? '',
      },
    },
  )
  return data
}

export async function lookupUsers(q: string): Promise<SimpleUser[]> {
  const { data } = await apiClient.get<SimpleUser[]>(
    '/admin/affiliates/users/lookup',
    { params: { q } },
  )
  return data
}

export async function updateUserSettings(
  userId: number,
  payload: UpdateAffiliateUserRequest,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.put<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
    payload,
  )
  return data
}

export async function clearUserSettings(
  userId: number,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.delete<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
  )
  return data
}

export async function batchSetRate(
  payload: BatchSetRateRequest,
): Promise<{ affected: number }> {
  const { data } = await apiClient.post<{ affected: number }>(
    '/admin/affiliates/users/batch-rate',
    payload,
  )
  return data
}

function recordParams(params: ListAffiliateRecordsParams = {}) {
  return {
    page: params.page ?? 1,
    page_size: params.page_size ?? 20,
    search: params.search ?? '',
    start_at: params.start_at || undefined,
    end_at: params.end_at || undefined,
    sort_by: params.sort_by || undefined,
    sort_order: params.sort_order || undefined,
    timezone: params.timezone || undefined,
    inviter_id: params.inviter_id,
    invitee_id: params.invitee_id,
    user_id: params.user_id,
    source_order_id: params.source_order_id,
    action: params.action,
    source_type: params.source_type,
    status: params.status,
    currency: params.currency,
    campaign_id: params.campaign_id,
  }
}

function unpagedParams(params: ListAffiliateRecordsParams) {
  const { page: _page, page_size: _pageSize, ...filters } = params
  return filters
}

export async function getRecordsSummary(type: AffiliateRecordType, params: ListAffiliateRecordsParams = {}): Promise<AffiliateRecordsSummary> {
  const { data } = await apiClient.get<AffiliateRecordsSummary>(`/admin/affiliates/${type}/summary`, { params: unpagedParams(params) })
  return data
}

/** The server enforces the 10,000-row bound and returns 422 for larger selections. */
export async function exportRecords(type: AffiliateRecordType, params: ListAffiliateRecordsParams = {}): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/admin/affiliates/${type}/export`, { params: unpagedParams(params), responseType: 'blob' })
  return data
}

export async function listInviteRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateInviteRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateInviteRecord>>(
    '/admin/affiliates/invites',
    { params: recordParams(params) },
  )
  return data
}

export async function listRebateRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateRebateRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateRebateRecord>>(
    '/admin/affiliates/rebates',
    { params: recordParams(params) },
  )
  return data
}

export async function listTransferRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateTransferRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateTransferRecord>>(
    '/admin/affiliates/transfers',
    { params: recordParams(params) },
  )
  return data
}

export async function getUserOverview(
  userId: number,
): Promise<AffiliateUserOverview> {
  const { data } = await apiClient.get<AffiliateUserOverview>(
    `/admin/affiliates/users/${userId}/overview`,
  )
  return data
}

/**
 * Records an offline withdrawal. `idempotencyKey` identifies one registration:
 * a retry with the same key deducts nothing and returns the first result, with
 * `replayed` set.
 */
export async function withdrawUserQuota(
  userId: number,
  payload: WithdrawAffiliateQuotaRequest,
  idempotencyKey: string,
): Promise<AffiliateWithdrawResponse> {
  const response = await apiClient.post<AffiliateWithdrawResult>(
    `/admin/affiliates/users/${userId}/withdraw`,
    payload,
    { headers: { 'Idempotency-Key': idempotencyKey } },
  )
  return {
    result: response.data,
    replayed: response.headers?.['x-idempotency-replayed'] === 'true',
  }
}

export const affiliatesAPI = {
  listUsers,
  lookupUsers,
  updateUserSettings,
  clearUserSettings,
  batchSetRate,
  listInviteRecords,
  listRebateRecords,
  listTransferRecords,
  getUserOverview,
  withdrawUserQuota,
  getRecordsSummary,
  exportRecords,
}

export default affiliatesAPI
