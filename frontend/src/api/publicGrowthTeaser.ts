import { apiClient } from './client'

export interface PublicGrowthTeaser {
  registration_enabled: boolean
  signup_balance_usd: number
  signup_grant_enabled: boolean
  payment_enabled: boolean
  checkin_enabled: boolean
  checkin_daily_reward?: number
  affiliate_enabled: boolean
  affiliate_rebate_pct?: number
  public_models_enabled: boolean
  public_model_count: number
  play_any_enabled: boolean
  play_features?: string[]
  vip_tiers_enabled: boolean
  total_requests?: number
  has_live_stats: boolean
}

export async function fetchPublicGrowthTeaser(): Promise<PublicGrowthTeaser | null> {
  try {
    const { data } = await apiClient.get<PublicGrowthTeaser>('/public/growth-teaser', {
      timeout: 8000,
    })
    return data ?? null
  } catch {
    return null
  }
}
