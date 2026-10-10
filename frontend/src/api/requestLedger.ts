import { apiClient } from './client'

export interface RequestRecord {
  id: string
  parent_id: string | null
  kind: string
  turn_no: number | null
  route: string
  method: string
  user_id: number | null
  api_key_id: number | null
  started_at: string
  ended_at: string | null
  execution_state: string
  usage_state: string
  settlement_state: string
  error_code: string
  http_status: number | null
  attempt_count: number
  output_observed: boolean
}
export interface RequestAttempt {
  usage_state: string
  output_observed: boolean
  upstream_kind: string
  phase: string
  attempt_no: number
  account_id?: number
  credential_account_id?: number
  started_at: string
  ended_at: string | null
  execution_state: string
  http_status: number | null
  error_code: string
}
export interface BillingReference {
  usage_log_id: number | null
  wallet_transaction_id: number | null
  subscription_id: number | null
  package_entitlement_id: number | null
  settled: boolean
  applied: boolean
  billed_cost: number | null
}
export interface LinkedUsage { id: number; model: string; input_tokens: number; output_tokens: number; created_at: string; billed_cost: number | null }
export interface RequestDetail {
  request: RequestRecord
  attempts: RequestAttempt[]
  billing: BillingReference[]
  wallet?: { id: number; operation: string }[]
}
export interface RequestPage { items: RequestRecord[]; total: number; page: number; page_size: number }
export type RequestFilters = Record<string, string | number | undefined>
const endpoint = (admin: boolean) => admin ? '/admin/requests' : '/requests'
export const requestLedgerAPI = {
  async usage(admin: boolean, id: string, usageID: number, signal?: AbortSignal) {
    return (await apiClient.get<LinkedUsage>(`${endpoint(admin)}/${encodeURIComponent(id)}/usage/${usageID}`, { signal })).data
  },
  async list(admin: boolean, params: RequestFilters, signal?: AbortSignal) {
    return (await apiClient.get<RequestPage>(endpoint(admin), { params, signal })).data
  },
  async detail(admin: boolean, id: string, signal?: AbortSignal) {
    return (await apiClient.get<RequestDetail>(`${endpoint(admin)}/${encodeURIComponent(id)}`, { signal })).data
  },
}
