import { apiClient } from '../client'
import type { FundRefundRequest, FundRefundRequestPage, FundRefundStatus, WithdrawalCurrency } from '../wallet'
import { normalizeWithdrawalWholeAmount } from '../wallet'

export interface AdminFundRefundListQuery {
  status?: FundRefundStatus | 'all'
  user_id?: number
  page?: number
  page_size?: number
}

export interface AdminFundRefundActionInput {
  reason?: string
  note?: string
}

export interface AdminFundRefundPaidInput {
  paid_amount: string
  paid_currency: WithdrawalCurrency
  payout_fx_rate?: string
  external_txn_id: string
  paid_at?: string
  note?: string
}

export interface AdminFundGrantInput {
  user_id: number
  amount: string
  reason: string
}

export interface AdminOfflineRechargeInput {
  user_id: number
  amount: string
  external_ref?: string
  reason: string
}

export interface AdminFundClassificationCandidate {
  user_id: number
  user_email: string
  transaction_id: number
  amount: string
  created_at: string
  recommended_kind: string
  estimated_remaining: string
  estimated_consumed: string
  current_balance: string
  existing_classified_funds: string
}

export interface AdminFundClassificationPreview {
  mode: 'preview'
  generated_at: string
  candidate_count: number
  candidates: AdminFundClassificationCandidate[]
}

export interface AdminFundClassificationExecuteInput {
  transaction_ids: number[]
  reason: string
}

export interface AdminFundClassificationExecuteResult {
  mode: 'execute'
  generated_at: string
  affected_count: number
  candidates: AdminFundClassificationCandidate[]
}

export type AdminFundSensitivePayout = Record<string, unknown>

function normalizeGrantInput<T extends { amount: string }>(input: T): T {
  return {
    ...input,
    amount: normalizeWithdrawalWholeAmount(input.amount),
  }
}

export async function listRefundRequests(query: AdminFundRefundListQuery = {}): Promise<FundRefundRequestPage> {
  const { data } = await apiClient.get<FundRefundRequestPage>('/admin/funds/refund-requests', { params: query })
  return data
}

export async function getRefundRequest(id: number): Promise<FundRefundRequest> {
  const { data } = await apiClient.get<FundRefundRequest>(`/admin/funds/refund-requests/${id}`)
  return data
}

export async function approveRefundRequest(id: number, input: AdminFundRefundActionInput = {}): Promise<FundRefundRequest> {
  const { data } = await apiClient.post<FundRefundRequest>(`/admin/funds/refund-requests/${id}/approve`, input)
  return data
}

export async function rejectRefundRequest(id: number, input: Required<Pick<AdminFundRefundActionInput, 'reason'>> & AdminFundRefundActionInput): Promise<FundRefundRequest> {
  const { data } = await apiClient.post<FundRefundRequest>(`/admin/funds/refund-requests/${id}/reject`, input)
  return data
}

export async function markRefundPaid(id: number, input: AdminFundRefundPaidInput): Promise<FundRefundRequest> {
  const { data } = await apiClient.post<FundRefundRequest>(`/admin/funds/refund-requests/${id}/mark-paid`, {
    ...input,
    paid_amount: normalizeWithdrawalWholeAmount(input.paid_amount),
  })
  return data
}

export async function getRefundSensitivePayout(id: number): Promise<AdminFundSensitivePayout> {
  const { data } = await apiClient.get<AdminFundSensitivePayout>(`/admin/funds/refund-requests/${id}/payout-sensitive`)
  return data
}

export async function grantGift(input: AdminFundGrantInput) {
  const { data } = await apiClient.post('/admin/funds/gifts', normalizeGrantInput(input))
  return data
}

export async function grantOfflineRecharge(input: AdminOfflineRechargeInput) {
  const { data } = await apiClient.post('/admin/funds/offline-recharges', normalizeGrantInput(input))
  return data
}

export async function previewSignupGift30(limit = 100): Promise<AdminFundClassificationPreview> {
  const { data } = await apiClient.get<AdminFundClassificationPreview>('/admin/funds/classifications/signup-gift-30/preview', { params: { limit } })
  return data
}

export async function executeSignupGift30(input: AdminFundClassificationExecuteInput): Promise<AdminFundClassificationExecuteResult> {
  const { data } = await apiClient.post<AdminFundClassificationExecuteResult>('/admin/funds/classifications/signup-gift-30/execute', input)
  return data
}

export default {
  listRefundRequests,
  getRefundRequest,
  approveRefundRequest,
  rejectRefundRequest,
  markRefundPaid,
  getRefundSensitivePayout,
  grantGift,
  grantOfflineRecharge,
  previewSignupGift30,
  executeSignupGift30,
}
