import { apiClient } from '../client'
import type { PlayArenaPeriod, PlayBlindboxPool, PlayTeamSettlementRecord, PlayTeamSummary, TeamRewardTier } from '../play'

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

export interface AdminArenaScore {
  rank: number
  user_id: number
  display_name: string
  email?: string
  avatar_url?: string
  token_sum: number
  estimated_reward: number
}

export interface AdminArenaRewardTier {
  rank_max: number
  amount: number
}

export interface AdminArenaLeaderboard {
  period?: PlayArenaPeriod
  rewards: AdminArenaRewardTier[]
  rows: AdminArenaScore[]
}

export interface AdminPlayTeamListItem {
  id: number
  name: string
  invite_code: string
  captain_id: number
  captain_display_name: string
  captain_avatar_url?: string
  captain_email?: string
  member_count: number
  token_sum: number
  team_spend: string
  estimated_pool: string
  created_at: string
  archived_at?: string
}

export interface AdminPlayTeamList {
  items: AdminPlayTeamListItem[]
  total: number
  page: number
  page_size: number
}

export interface AdminPlayOpsSummary {
  total_teams: number
  active_teams: number
  month_spend: string
  estimated_shared_pool: string
  pending_failed_settlements: number
  monthly_arena_reward_budget: number
  daily_arena_reward_budget: number
}

export interface AdminPlayTeamDetail {
  team: PlayTeamSummary
  created_at: string
  archived_at?: string
  settlements: PlayTeamSettlementRecord[]
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

export async function getArenaLeaderboard(params: { period_type?: 'daily' | 'monthly'; period_id?: number; limit?: number } = {}): Promise<AdminArenaLeaderboard> {
  const { data } = await apiClient.get<AdminArenaLeaderboard>('/admin/play/arena/leaderboard', { params })
  return data
}

export async function getSummary(): Promise<AdminPlayOpsSummary> {
  const { data } = await apiClient.get<AdminPlayOpsSummary>('/admin/play/summary')
  return data
}

export async function listTeams(params: { status?: 'active' | 'archived' | 'all'; q?: string; page?: number; page_size?: number } = {}): Promise<AdminPlayTeamList> {
  const { data } = await apiClient.get<AdminPlayTeamList>('/admin/play/teams', { params })
  return data
}

export async function getTeam(id: number): Promise<AdminPlayTeamDetail> {
  const { data } = await apiClient.get<AdminPlayTeamDetail>(`/admin/play/teams/${id}`)
  return data
}

export async function getTeamSettlements(id: number): Promise<PlayTeamSettlementRecord[]> {
  const { data } = await apiClient.get<PlayTeamSettlementRecord[]>(`/admin/play/teams/${id}/settlements`)
  return data ?? []
}

export const adminPlayAPI = {
  getBlindboxPool,
  updateBlindboxPool,
  getTeamRewardSettings,
  updateTeamRewardSettings,
  listTeamRewardSettlements,
  retryTeamRewardSettlement,
  getSummary,
  getArenaLeaderboard,
  listTeams,
  getTeam,
  getTeamSettlements,
}

export default adminPlayAPI
