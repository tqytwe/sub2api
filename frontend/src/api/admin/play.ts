import { apiClient } from '../client'
import type { PlayBlindboxPool, PlayTeamSettlementRecord, TeamRewardTier } from '../play'

export type { PlayBlindboxPool, PlayBlindboxPoolTier } from '../play'

export async function getBlindboxPool(): Promise<PlayBlindboxPool> {
  const { data } = await apiClient.get<PlayBlindboxPool>('/admin/play/blindbox/pool')
  return data
}

export async function updateBlindboxPool(pool: PlayBlindboxPool): Promise<PlayBlindboxPool> {
  const { data } = await apiClient.put<PlayBlindboxPool>('/admin/play/blindbox/pool', pool)
  return data
}

export interface TeamRewardSettings {
  enabled: boolean
  tiers: TeamRewardTier[]
  cap: string
  start_month: string
}

export async function getTeamRewardSettings(): Promise<TeamRewardSettings> {
  const { data } = await apiClient.get<TeamRewardSettings>('/admin/play/team-rewards/settings')
  return data
}

export async function updateTeamRewardSettings(settings: TeamRewardSettings): Promise<TeamRewardSettings> {
  const { data } = await apiClient.put<TeamRewardSettings>('/admin/play/team-rewards/settings', settings)
  return data
}

export async function listTeamRewardSettlements(): Promise<PlayTeamSettlementRecord[]> {
  const { data } = await apiClient.get<PlayTeamSettlementRecord[]>('/admin/play/team-rewards/settlements')
  return data ?? []
}

export async function retryTeamRewardSettlement(id: number): Promise<void> {
  await apiClient.post(`/admin/play/team-rewards/settlements/${id}/retry`)
}

export const adminPlayAPI = {
  getBlindboxPool,
  updateBlindboxPool,
  getTeamRewardSettings,
  updateTeamRewardSettings,
  listTeamRewardSettlements,
  retryTeamRewardSettlement,
}

export default adminPlayAPI
