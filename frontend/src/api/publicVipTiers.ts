import { apiClient } from './client'

export interface PublicVIPTier {
  tier: number
  label: string
  min_recharge: number
  recharge_bonus_pct: number
  color_key: string
  perks?: string[]
}

export interface PublicVIPTiersResponse {
  enabled: boolean
  tiers: PublicVIPTier[]
}

export async function fetchPublicVIPTiers(): Promise<PublicVIPTiersResponse> {
  const { data } = await apiClient.get<PublicVIPTiersResponse>('/public/vip-tiers')
  return data ?? { enabled: false, tiers: [] }
}
