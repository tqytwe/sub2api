import { apiClient } from '../client'
import type { PlayBlindboxPool } from '../play'

export interface PlayGrowthConfig {
  blindbox_pool: PlayBlindboxPool
  blindbox_paid_enabled: boolean
  blindbox_region_enabled: boolean
  team_max_members: number
  team_weekly_token_target: number
  team_weekly_request_target: number
  public_activity_min_count: number
  founder_season_json: string
  growth_experiment_json: string
}

export async function getGrowthConfig(): Promise<PlayGrowthConfig> {
  const { data } = await apiClient.get<PlayGrowthConfig>('/admin/play/growth-config')
  return data
}

export async function updateGrowthConfig(config: PlayGrowthConfig): Promise<PlayGrowthConfig> {
  const { data } = await apiClient.put<PlayGrowthConfig>('/admin/play/growth-config', config)
  return data
}

export default { getGrowthConfig, updateGrowthConfig }
