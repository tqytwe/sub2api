import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type {
  IPRiskOverview,
  IPRiskPolicy,
  IPRiskPolicyInput,
  RiskActionInput,
  RiskActionPreview,
  RiskActionRecord,
  RiskCase,
  RiskCaseDetail,
  RiskCaseQuery,
  RiskConfig,
  RiskRuntime,
  RiskScan,
} from '@/features/ip-risk/types'

const base = '/admin/ip-risk'

async function getOverview(): Promise<IPRiskOverview> {
  const { data } = await apiClient.get<IPRiskOverview>(`${base}/overview`)
  return data
}

async function getRuntime(): Promise<RiskRuntime> {
  const { data } = await apiClient.get<RiskRuntime>(`${base}/runtime`)
  return data
}

async function listCases(query: RiskCaseQuery): Promise<PaginatedResponse<RiskCase>> {
  const { data } = await apiClient.get<PaginatedResponse<RiskCase>>(`${base}/cases`, { params: query })
  return data
}

async function getCase(id: number): Promise<RiskCaseDetail> {
  const { data } = await apiClient.get<RiskCaseDetail>(`${base}/cases/${id}`)
  return data
}

async function startScan(rangeStart: string, rangeEnd: string): Promise<RiskScan> {
  const { data } = await apiClient.post<RiskScan>(`${base}/scans`, {
    range_start: rangeStart,
    range_end: rangeEnd,
  })
  return data
}

async function getScan(id: number): Promise<RiskScan> {
  const { data } = await apiClient.get<RiskScan>(`${base}/scans/${id}`)
  return data
}

async function getConfig(): Promise<RiskConfig> {
  const { data } = await apiClient.get<RiskConfig>(`${base}/config`)
  return data
}

async function updateConfig(input: RiskConfig): Promise<RiskConfig> {
  const { data } = await apiClient.put<RiskConfig>(`${base}/config`, input)
  return data
}

async function listPolicies(): Promise<IPRiskPolicy[]> {
  const { data } = await apiClient.get<IPRiskPolicy[]>(`${base}/policies`)
  return data
}

async function createPolicy(input: IPRiskPolicyInput): Promise<IPRiskPolicy> {
  const { data } = await apiClient.post<IPRiskPolicy>(`${base}/policies`, input)
  return data
}

async function updatePolicy(id: number, input: IPRiskPolicyInput): Promise<IPRiskPolicy> {
  const { data } = await apiClient.put<IPRiskPolicy>(`${base}/policies/${id}`, input)
  return data
}

async function deletePolicy(id: number): Promise<void> {
  await apiClient.delete(`${base}/policies/${id}`)
}

async function previewAction(caseId: number, input: RiskActionInput): Promise<RiskActionPreview> {
  const { data } = await apiClient.post<RiskActionPreview>(
    `${base}/cases/${caseId}/actions/preview`,
    input,
  )
  return data
}

async function executeAction(caseId: number, input: RiskActionInput): Promise<RiskActionRecord> {
  const { data } = await apiClient.post<RiskActionRecord>(
    `${base}/cases/${caseId}/actions`,
    input,
  )
  return data
}

async function listActions(page = 1, pageSize = 20): Promise<PaginatedResponse<RiskActionRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<RiskActionRecord>>(`${base}/actions`, {
    params: { page, page_size: pageSize },
  })
  return data
}

async function rollbackAction(id: number, reason: string): Promise<RiskActionRecord> {
  const { data } = await apiClient.post<RiskActionRecord>(`${base}/actions/${id}/rollback`, {
    reason,
  })
  return data
}

export default {
  getOverview,
  getRuntime,
  listCases,
  getCase,
  startScan,
  getScan,
  getConfig,
  updateConfig,
  listPolicies,
  createPolicy,
  updatePolicy,
  deletePolicy,
  previewAction,
  executeAction,
  listActions,
  rollbackAction,
}
