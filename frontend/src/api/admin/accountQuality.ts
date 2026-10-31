import { apiClient } from '../client'
import type { PelicanTestConfig, QualityHistoryPage, ScheduledTestPlan, ScheduledTestResult } from '@/types'

export interface QualityRuleAccountFilter {
  group?: string
  type?: string
  search?: string
  statuses?: string[]
}

export interface QualityRuleTemplate {
  id: number
  account_filter: QualityRuleAccountFilter
  model_id: string
  cron_expression: string
  enabled: boolean
  max_results: number
  pelican_config: PelicanTestConfig
  plan_ids: number[]
  last_synced_at: string | null
  created_at: string
  updated_at: string
}

export interface QualityRuleTemplateRequest {
  account_filter?: QualityRuleAccountFilter
  model_id?: string
  cron_expression?: string
  enabled?: boolean
  max_results?: number
  pelican_config?: PelicanTestConfig
}

export interface QualityRulePresetRequest {
  name: string
  model_id: string
  cron_expression: string
  enabled: boolean
  max_results: number
  pelican_config: PelicanTestConfig
}

export interface QualityRulePreset extends QualityRulePresetRequest {
  id: number
  created_at: string
  updated_at: string
}

export async function listPlans(): Promise<ScheduledTestPlan[]> {
  const { data } = await apiClient.get<ScheduledTestPlan[]>('/admin/account-quality-plans')
  return data ?? []
}

export async function runPlan(planId: number): Promise<void> {
  await apiClient.post(`/admin/account-quality-plans/${planId}/run`)
}

export async function listResults(planId: number, limit = 50): Promise<ScheduledTestResult[]> {
  const { data } = await apiClient.get<ScheduledTestResult[]>(`/admin/scheduled-test-plans/${planId}/results`, {
    params: { limit }
  })
  return data ?? []
}

export async function listHistory(beforeId = 0): Promise<QualityHistoryPage> {
  const { data } = await apiClient.get<QualityHistoryPage>('/admin/account-quality-results', {
    params: { before_id: beforeId }
  })
  return data ?? { items: [], next_cursor: 0 }
}

export async function listPresets(): Promise<QualityRulePreset[]> {
  const { data } = await apiClient.get<QualityRulePreset[]>('/admin/account-quality-presets')
  return data ?? []
}

export async function createPreset(body: QualityRulePresetRequest): Promise<QualityRulePreset> {
  const { data } = await apiClient.post<QualityRulePreset>('/admin/account-quality-presets', body)
  return data
}

export async function listTemplates(): Promise<QualityRuleTemplate[]> {
  const { data } = await apiClient.get<QualityRuleTemplate[]>('/admin/account-quality-templates')
  return data ?? []
}

export async function createTemplate(body: QualityRuleTemplateRequest): Promise<{ template: QualityRuleTemplate; created: number }> {
  const { data } = await apiClient.post('/admin/account-quality-templates', body)
  return data
}

export async function updateTemplate(id: number, body: QualityRuleTemplateRequest): Promise<{ template: QualityRuleTemplate; updated: number; failed: number; created: number }> {
  const { data } = await apiClient.put(`/admin/account-quality-templates/${id}`, body)
  return data
}

export async function deleteTemplate(id: number, deletePlans: boolean): Promise<{ deleted: number }> {
  const { data } = await apiClient.delete(`/admin/account-quality-templates/${id}`, { params: deletePlans ? { delete_plans: 'true' } : undefined })
  return data
}

export const accountQualityAPI = { listPlans, runPlan, listResults, listHistory, listPresets, createPreset, listTemplates, createTemplate, updateTemplate, deleteTemplate }

export default accountQualityAPI
