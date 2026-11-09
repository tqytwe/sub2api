import { apiClient } from './client'
import type { ReferralCampaignProgress } from './referralCampaign'

export interface AffiliateOperationsSummary {
  reconciliation: AffiliateOperationsReconciliation
  month: string
  month_net_earned: number | null
  currency: string
  invited_count: number
  effective_paid_count: number
  pending_count: number
  available_quota: number
  frozen_quota: number
  gross_earned: number | null
  revoked_amount: number | null
  net_earned: number | null
  amounts: AffiliateEarningAmounts[]
  period_amounts: AffiliateEarningAmounts[]
  transferred_amount: number
  withdrawn_amount: number
  updated_at: string
}

export interface AffiliateOperationsReconciliation {
  relationship_count: number
  registered_month_count: number
  registered_effective_paid_count: number
  known_binding_count: number
  unknown_binding_count: number
  effective_paid_count: number
  unpaid_count: number
  refunded_count: number
  risk_excluded_count: number
  disabled_count: number
  pending_count: number
  missing_fact_count: number
  pending_payment_count: number
}

export interface AffiliateEarningAmounts {
  source_type: 'regular' | 'campaign' | 'other'
  currency: string
  gross_earned: number
  revoked_amount: number
  net_earned: number
}

export interface AffiliateOperationsPage<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface AffiliateOperationsQuery {
  page?: number
  page_size?: number
  status?: string
  search?: string
  action?: string
  start_date?: string
  end_date?: string
  scope?: 'joined' | 'ended'
}

export type AffiliateInviteeStatus = 'pending_evidence' | 'unpaid' | 'effective' | 'refunded' | 'risk_excluded' | 'disabled'
export interface AffiliateOperationsInvitee {
  user_id: number
  display_name: string
  invited_at?: string
  first_paid_at?: string
  first_paid_month?: string
  status: AffiliateInviteeStatus
  reason?: string
  total_rebate: number
}

export interface AffiliateOperationsEarning {
  id: number
  action: string
  source_type: string
  amount: number
  currency?: string
  created_at: string
  source_order_id?: number
  source_user_display_name?: string
  status: string
  frozen_until?: string
  balance_after?: number
}

export interface AffiliateLeaderboardRow {
  rank: number
  display_name: string
  effective_paid_count: number
  is_me: boolean
}

export interface AffiliateLeaderboard {
  month: string
  timezone: string
  items: AffiliateLeaderboardRow[]
  my_rank: AffiliateLeaderboardRow | null
  updated_at: string
}

export async function getAffiliateOperationsSummary(): Promise<AffiliateOperationsSummary> {
  const { data } = await apiClient.get<AffiliateOperationsSummary>('/user/aff/operations/summary')
  return data
}

export async function listAffiliateInvitees(params: AffiliateOperationsQuery = {}): Promise<AffiliateOperationsPage<AffiliateOperationsInvitee>> {
  const { data } = await apiClient.get<AffiliateOperationsPage<AffiliateOperationsInvitee>>('/user/aff/operations/invitees', { params })
  return data
}

export async function listAffiliateEarnings(params: AffiliateOperationsQuery = {}): Promise<AffiliateOperationsPage<AffiliateOperationsEarning>> {
  const { data } = await apiClient.get<AffiliateOperationsPage<AffiliateOperationsEarning>>('/user/aff/operations/earnings', { params })
  return data
}

export async function getAffiliateLeaderboard(params: { month?: string; campaign_id?: number } = {}, signal?: AbortSignal): Promise<AffiliateLeaderboard> {
  const { data } = await apiClient.get<AffiliateLeaderboard>('/user/aff/operations/leaderboard', { params, signal })
  return data
}

export async function listAffiliateCampaignHistory(params: AffiliateOperationsQuery = {}): Promise<AffiliateOperationsPage<ReferralCampaignProgress>> {
  const { data } = await apiClient.get<AffiliateOperationsPage<ReferralCampaignProgress>>('/user/aff/operations/campaigns', { params })
  return data
}
