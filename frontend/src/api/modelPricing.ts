import { apiClient } from './client'

export interface MyModelPricingGroup {
  id: number
  name: string
  rate_multiplier: number
}

export interface MyModelPricingRow {
  name: string
  platform: string
  channel?: string
  use_case?: string
  groups: MyModelPricingGroup[]
  base_input_price: number | null
  base_output_price: number | null
  effective_input_price: number | null
  effective_output_price: number | null
  official_input_price: number | null
  official_output_price: number | null
  site_input_price: number | null
  site_output_price: number | null
}

export interface MyModelPricingResponse {
  models: MyModelPricingRow[]
  rate_multiplier_note: string
  enabled: boolean
}

export async function getMyModelPricing(options?: { signal?: AbortSignal }): Promise<MyModelPricingResponse> {
  const { data } = await apiClient.get<MyModelPricingResponse>('/models/my-pricing', {
    signal: options?.signal,
  })
  return data ?? { models: [], rate_multiplier_note: '', enabled: false }
}

export const modelPricingAPI = { getMyModelPricing }

export default modelPricingAPI
