/**
 * Unauthenticated public API endpoints.
 */

import { apiClient } from './client'
import type { UserAvailableChannel } from './channels'

export interface PublicModelPricingGroupPrice {
  id: number
  name: string
  rate_multiplier: number
  effective_input_price: number | null
  effective_output_price: number | null
}

export interface PublicModelPricingRow {
  name: string
  platform: string
  use_case: string
  official_input_price: number | null
  official_output_price: number | null
  our_input_price: number | null
  our_output_price: number | null
  rate_multiplier: number
  groups?: PublicModelPricingGroupPrice[]
}

export async function getPublicModelPricing(options?: { signal?: AbortSignal }): Promise<PublicModelPricingRow[]> {
  const { data } = await apiClient.get<PublicModelPricingRow[]>('/public/model-pricing', {
    signal: options?.signal,
  })
  return data ?? []
}

export async function getPublicModels(options?: { signal?: AbortSignal }): Promise<UserAvailableChannel[]> {
  const { data } = await apiClient.get<UserAvailableChannel[]>('/public/models', {
    signal: options?.signal,
  })
  return data ?? []
}

export const publicAPI = { getPublicModels, getPublicModelPricing }

export default publicAPI
