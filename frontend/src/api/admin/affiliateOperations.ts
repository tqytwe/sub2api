import { apiClient } from '@/api/client'
import type { PaginatedResponse } from '@/types'
import type { AffiliateOperationsReconciliation } from '@/api/affiliateOperations'

export type InviteQualificationStatus = 'pending_evidence' | 'unpaid' | 'effective' | 'refunded' | 'risk_excluded' | 'disabled'
export interface OperationsQuery {
  month?: string
  page?: number
  page_size?: number
  search?: string
  user_id?: number
  invitee_id?: number
  status?: string
  source_order_id?: number
  action?: string
}
export interface AffiliateOperationsOverview {
  reconciliation: AffiliateOperationsReconciliation
  month: string
  currency: 'USD' | 'CNY' | 'mixed'
  monetary_scope: 'all_time'
  invited_count: number
  effective_paid_count: number
  cohort_effective_paid_count: number
  pending_count: number
  conversion_rate: number
  available_quota: number
  frozen_quota: number
  gross_earned: number | null
  revoked_amount: number | null
  net_earned: number | null
  month_net_earned: number | null
  amounts: AffiliateOperationsMoney[]
  period_amounts: AffiliateOperationsMoney[]
  transferred_amount: number
  withdrawn_amount: number
  exceptions_count: number
  trend: Array<{ date: string; invited_count: number; effective_paid_count: number }>
  updated_at: string
}
export interface AffiliateOperationsMoney {
  source_type: 'regular' | 'campaign' | 'other'
  currency: string
  gross_earned: number
  revoked_amount: number
  net_earned: number
}
export interface AffiliatePromoter {
  user_id: number
  email: string
  username: string
  aff_code: string
  invited_count: number
  effective_paid_count: number
  available_quota: number
  frozen_quota: number
  gross_earned: number | null
  net_earned: number | null
  currency: string
  amounts: AffiliateOperationsMoney[]
}
export interface AffiliateOperationsInvite {
  inviter_id: number
  inviter_email: string
  invitee_id: number
  invitee_email: string
  invited_at?: string
  first_paid_at?: string
  first_paid_month?: string
  status: InviteQualificationStatus
  reason: string
  total_rebate: number
  risk_case_id?: number
}
export interface AffiliateOperationEarning {
  id: number
  user_id: number
  email: string
  invitee_id?: number
  invitee_email?: string
  action: 'accrue' | 'revoke' | 'transfer' | 'withdraw'
  source_type: 'order_rebate' | 'campaign_reward' | 'other_bonus' | 'balance_transfer' | 'offline_withdrawal'
  source_order_id?: number
  amount: number
  currency: string
  status: 'frozen' | 'available' | 'revoked' | 'completed'
  created_at: string
}
export interface AffiliateOperationAudit {
  id: number
  user_id?: number
  actor_id?: number
  action: string
  detail: Record<string, unknown>
  created_at: string
}
export interface AdminAffiliateRank {
  rank: number
  user_id: number
  email: string
  username: string
  effective_paid_count: number
}
export interface AdminAffiliateLeaderboard {
  month: string
  timezone: string
  items: AdminAffiliateRank[]
  updated_at: string
}

async function read<T>(path: string, params: OperationsQuery = {}, signal?: AbortSignal): Promise<T> {
  const { data } = await apiClient.get<T>(`/admin/affiliates/operations/${path}`, { params, ...(signal ? { signal } : {}) })
  return data
}

export default {
  getOverview: (params: OperationsQuery = {}) => read<AffiliateOperationsOverview>('overview', params),
  getLeaderboard: (params: OperationsQuery = {}, signal?: AbortSignal) => read<AdminAffiliateLeaderboard>('leaderboard', params, signal),
  listPromoters: (params: OperationsQuery = {}) => read<PaginatedResponse<AffiliatePromoter>>('promoters', params),
  listInvitees: (params: OperationsQuery = {}) => read<PaginatedResponse<AffiliateOperationsInvite>>('invitees', params),
  listEarnings: (params: OperationsQuery = {}) => read<PaginatedResponse<AffiliateOperationEarning>>('earnings', params),
  listAudits: (params: OperationsQuery = {}) => read<PaginatedResponse<AffiliateOperationAudit>>('audits', params),
  listExceptions: (params: OperationsQuery = {}) => read<PaginatedResponse<AffiliateOperationsInvite>>('exceptions', params),
}
