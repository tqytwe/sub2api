import { apiClient } from './client'

export interface PublicVIPTier {
  tier: number
  label: string
  min_recharge: number
  perks?: string[]
}

export interface PublicVIPTiersResponse {
  enabled: boolean
  tiers: PublicVIPTier[]
}

export async function fetchPublicVIPTiers(): Promise<PublicVIPTiersResponse> {
  const { data } = await apiClient.get<{ data: PublicVIPTiersResponse }>('/public/vip-tiers')
  return data.data
}
