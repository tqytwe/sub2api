import { apiClient } from '../client'

export interface ModelBillingRule {
  model: string
  multiplier: number
  group_ids: number[] | null
}

export interface ModelBillingConfig {
  enabled: boolean
  rules: ModelBillingRule[] | null
}

export interface ModelBillingSnapshot {
  config: ModelBillingConfig
  revision: string
}

export async function getModelBillingConfig(): Promise<ModelBillingSnapshot> {
  const { data } = await apiClient.get<ModelBillingSnapshot>('/admin/model-billing/config')
  return data
}

export async function saveModelBillingConfig(snapshot: ModelBillingSnapshot): Promise<ModelBillingSnapshot> {
  const { data } = await apiClient.put<ModelBillingSnapshot>('/admin/model-billing/config', snapshot)
  return data
}
