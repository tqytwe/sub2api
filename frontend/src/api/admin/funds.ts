import { apiClient } from '../client'
import type { FundRefundStatus, WithdrawalCurrency } from '../wallet'
import { normalizeWithdrawalWholeAmount } from '../wallet'

export interface AdminFundRefundListQuery {
  status?: FundRefundStatus | 'all'
  account?: string
  page?: number
  page_size?: number
}

export interface AdminFundRefundRequest {
  request_no: string
  user_email: string
  request_type: string
  amount: string
  currency: string
  status: FundRefundStatus
  reason?: string
  admin_note?: string
  payout_method?: string
  payout_currency?: string
  payout_account_mask?: string
  payout_recipient_name_mask?: string
  rejected_reason?: string
  paid_at?: string
  paid_amount?: string
  paid_currency?: string
  payout_fx_rate?: string
  external_txn_id?: string
  created_at: string
  updated_at?: string
}
export interface AdminFundRefundRequestPage { items: AdminFundRefundRequest[]; total: number; page: number; page_size: number; pages: number }

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
  account_email: string
  amount: string
  reason: string
}

export interface AdminOfflineRechargeInput {
  account_email: string
  amount: string
  external_ref?: string
  reason: string
}

export interface AdminFundAccount {
  email: string
  username?: string
  status: string
  current_balance: string
}

export type FundOperationKind = 'offline_recharge' | 'ops_gift' | 'compensation' | 'refund' | 'reversal' | 'account_correction'
export type FundOperationStatus = 'completed' | 'pending' | 'canceled' | 'pending_insufficient_balance'

export interface AdminFundOperation {
  operation_no: string
  operation_kind: FundOperationKind
  status: FundOperationStatus
  account_email: string
  account_username?: string
  actor_account_email?: string
  amount: string
  currency: string
  reason?: string
  note?: string
  external_ref_masked?: string
  external_ref?: string
  balance_before?: string
  balance_after?: string
  membership_effect?: string
  original_operation_no?: string
  created_at: string
}

export interface AdminFundOperationPage { items: AdminFundOperation[]; total: number; page: number; page_size: number; pages: number }
export interface AdminFundOperationQuery { kind?: FundOperationKind | 'all'; status?: FundOperationStatus | 'all'; account?: string; operator?: string; q?: string; page?: number; page_size?: number }

export type AdminFundSensitivePayout = Record<string, unknown>

function normalizeGrantInput<T extends { amount: string }>(input: T): T {
  return {
    ...input,
    amount: input.amount.trim(),
  }
}

export async function listRefundRequests(query: AdminFundRefundListQuery = {}): Promise<AdminFundRefundRequestPage> {
	const { data } = await apiClient.get<AdminFundRefundRequestPage>('/admin/funds/refund-requests', { params: query })
  return data
}

export async function getRefundRequest(requestNo: string): Promise<AdminFundRefundRequest> {
	const { data } = await apiClient.get<AdminFundRefundRequest>(`/admin/funds/refund-requests/${encodeURIComponent(requestNo)}`)
  return data
}

export async function approveRefundRequest(requestNo: string, input: AdminFundRefundActionInput = {}): Promise<AdminFundRefundRequest> {
	const { data } = await apiClient.post<AdminFundRefundRequest>(`/admin/funds/refund-requests/${encodeURIComponent(requestNo)}/approve`, input)
  return data
}

export async function rejectRefundRequest(requestNo: string, input: Required<Pick<AdminFundRefundActionInput, 'reason'>> & AdminFundRefundActionInput): Promise<AdminFundRefundRequest> {
	const { data } = await apiClient.post<AdminFundRefundRequest>(`/admin/funds/refund-requests/${encodeURIComponent(requestNo)}/reject`, input)
  return data
}

export async function markRefundPaid(requestNo: string, input: AdminFundRefundPaidInput): Promise<AdminFundRefundRequest> {
	const { data } = await apiClient.post<AdminFundRefundRequest>(`/admin/funds/refund-requests/${encodeURIComponent(requestNo)}/mark-paid`, {
    ...input,
    paid_amount: normalizeWithdrawalWholeAmount(input.paid_amount),
  })
  return data
}

export async function getRefundSensitivePayout(requestNo: string): Promise<AdminFundSensitivePayout> {
	const { data } = await apiClient.get<AdminFundSensitivePayout>(`/admin/funds/refund-requests/${encodeURIComponent(requestNo)}/payout-sensitive`)
  return data
}

export async function grantGift(input: AdminFundGrantInput): Promise<AdminFundOperation> {
  const { data } = await apiClient.post<AdminFundOperation>('/admin/funds/gifts', normalizeGrantInput(input))
  return data
}

export async function grantOfflineRecharge(input: AdminOfflineRechargeInput): Promise<AdminFundOperation> {
  const { data } = await apiClient.post<AdminFundOperation>('/admin/funds/offline-recharges', normalizeGrantInput(input))
  return data
}

export async function grantCompensation(input: AdminFundGrantInput): Promise<AdminFundOperation> {
  const { data } = await apiClient.post<AdminFundOperation>('/admin/funds/compensations', normalizeGrantInput(input))
  return data
}

export async function searchFundAccounts(q: string): Promise<AdminFundAccount[]> {
  const { data } = await apiClient.get<AdminFundAccount[]>('/admin/funds/accounts/search', { params: { q } })
  return data
}

export async function listFundOperations(query: AdminFundOperationQuery = {}): Promise<AdminFundOperationPage> {
  const { data } = await apiClient.get<AdminFundOperationPage>('/admin/funds/operations', { params: query })
  return data
}

export async function getFundOperation(operationNo: string): Promise<AdminFundOperation> {
  const { data } = await apiClient.get<AdminFundOperation>(`/admin/funds/operations/${encodeURIComponent(operationNo)}`)
  return data
}

export async function getFundOperationSensitive(operationNo: string): Promise<AdminFundOperation> {
  const { data } = await apiClient.get<AdminFundOperation>(`/admin/funds/operations/${encodeURIComponent(operationNo)}/sensitive`)
  return data
}

export async function correctFundOperation(operationNo: string, input: { correct_account_email: string; reason: string }): Promise<AdminFundOperation> {
	const { data } = await apiClient.post<AdminFundOperation>(`/admin/funds/operations/${encodeURIComponent(operationNo)}/corrections`, input)
  return data
}

export async function retryFundOperationCorrection(operationNo: string): Promise<AdminFundOperation> {
  const { data } = await apiClient.post<AdminFundOperation>(`/admin/funds/operations/${encodeURIComponent(operationNo)}/corrections/retry`)
  return data
}

export async function cancelFundOperationCorrection(operationNo: string): Promise<AdminFundOperation> {
  const { data } = await apiClient.post<AdminFundOperation>(`/admin/funds/operations/${encodeURIComponent(operationNo)}/corrections/cancel`)
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
  grantCompensation,
  grantOfflineRecharge,
  searchFundAccounts,
  listFundOperations,
  getFundOperation,
  getFundOperationSensitive,
  correctFundOperation,
  retryFundOperationCorrection,
  cancelFundOperationCorrection,
}
