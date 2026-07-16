import { apiClient } from './client'

export interface PlayCheckinStatus {
  enabled: boolean
  checked_in_today: boolean
  reward_amount: number
  server_date: string
  streak_count?: number
  next_milestone_days?: number
  next_milestone_bonus?: number
  can_makeup?: boolean
  makeup_date?: string
  recharge_boost_active?: boolean
  boost_checkin_multiplier?: number
}

export interface PlayCheckinResult {
  reward_amount: number
  balance_added: number
  server_date: string
  streak_count?: number
  milestone_bonus?: number
}

export interface PlayArenaPeriod {
  id: number
  name: string
  start_at: string
  end_at: string
  status: string
}

export interface PlayArenaCurrent {
  enabled: boolean
  period?: PlayArenaPeriod
  token_sum?: number
  display_token_sum?: number
  rank?: number
  tokens_to_prev_rank?: number
  recharge_boost_active?: boolean
  arena_score_multiplier?: number
  campaign_active?: boolean
}

export interface PlayArenaScore {
  rank: number
  user_id: number
  display_name: string
  avatar_url?: string
  token_sum: number
}

export interface PlayArenaLeaderboard {
  enabled: boolean
  period?: PlayArenaPeriod
  rows: PlayArenaScore[]
}

export interface PlayBlindboxStatus {
  enabled: boolean
  cost_amount: number
  pool?: PlayBlindboxPool
  daily_limit: number
  effective_limit?: number
  opens_today: number
  can_open: boolean
  server_date: string
  recharge_boost_active?: boolean
  campaign_active?: boolean
}

export interface PlayBlindboxPoolTier {
  amount: number
  weight: number
}

export interface PlayBlindboxPool {
  version: string
  cost: number
  rtp_cap: number
  tiers: PlayBlindboxPoolTier[]
}

export interface PlayBlindboxPoolResponse {
  enabled: boolean
  pool: PlayBlindboxPool
}

export interface PlayBlindboxOpenResult {
  cost_amount: number
  reward_amount: number
  net_amount: number
  opens_today: number
  server_date: string
  pool_version: string
  open_source: string
}

export interface PlayBlindboxRecentWin {
  user: string
  reward: number
  when: string
}

export interface PlayQuizQuestion {
  id: number
  prompt: string
  options: string[]
}

export interface PlayQuizToday {
  enabled: boolean
  questions: PlayQuizQuestion[]
  already_submitted: boolean
  previous_score?: number
  previous_total?: number
  previous_reward?: number
  reward_per_correct: number
  server_date: string
}

export interface PlayQuizSubmitResult {
  score: number
  total: number
  reward_amount: number
  server_date: string
}

export interface PlayTeamMember {
  user_id: number
  display_name: string
  avatar_url?: string
  joined_at: string
  token_sum: number
  token_pct: number
}

export interface PlayTeamAffiliateInfo {
  enabled: boolean
  token_threshold: number
  milestone_reached: boolean
  tokens_to_milestone?: number
  captain_bonus?: number
  captain_bonus_granted?: boolean
}

export interface PlayTeamSummary {
  id: number
  name: string
  invite_code: string
  captain_id: number
  member_count: number
  token_sum: number
  members: PlayTeamMember[]
  affiliate?: PlayTeamAffiliateInfo
}

export interface PlayTeamMe {
  enabled: boolean
  team?: PlayTeamSummary
}

export interface PlayVIPStatus {
  tier: number
  label: string
  perks?: string[]
  next_tier?: number
  next_label?: string
  next_min_recharge?: number
  amount_to_next?: number
}

export interface PlayCampaignRules {
  recharge_bonus_pct?: number
  blindbox_extra_opens?: number
  arena_score_multiplier?: number
  name_i18n?: Record<string, string>
}

export interface PlayCampaignSummary {
  id: number
  name: string
  start_at: string
  end_at: string
  rules: PlayCampaignRules
}

export interface PlayHubGrowth {
  balance: number
  total_recharged: number
  first_recharge_eligible: boolean
  balance_low_warning: boolean
  balance_low_threshold?: number
  recharge_multiplier: number
  payment_enabled: boolean
  campaign_recharge_bonus_pct?: number
  vip?: PlayVIPStatus
}

export interface PlayHubImageStudio {
  enabled: boolean
  images_today: number
  has_completed_job: boolean
}

export interface PlayQuestTask {
  key: string
  label?: string
  completed: boolean
  energy: number
  cta_route?: string
}

export interface PlayQuestToday {
  enabled: boolean
  energy: number
  level: number
  energy_to_next_level: number
  tasks: PlayQuestTask[]
  server_date: string
}

export interface PlayHubSummary {
  any_enabled: boolean
  pending_actions: number
  growth: PlayHubGrowth
  campaigns?: PlayCampaignSummary[]
  image_studio?: PlayHubImageStudio
  quests?: PlayQuestToday
  checkin?: PlayCheckinStatus
  arena?: PlayArenaCurrent
  daily_arena?: PlayArenaCurrent
  blindbox?: PlayBlindboxStatus
  quiz?: PlayQuizToday
  team?: PlayTeamMe
}

export async function getPlayHub(): Promise<PlayHubSummary> {
  const { data } = await apiClient.get<PlayHubSummary>('/play/hub')
  return data
}

export async function getActiveCampaigns(): Promise<PlayCampaignSummary[]> {
  const { data } = await apiClient.get<PlayCampaignSummary[]>('/play/campaigns/active')
  return data
}

export async function getCheckinStatus(): Promise<PlayCheckinStatus> {
  const { data } = await apiClient.get<PlayCheckinStatus>('/play/checkin/status')
  return data
}

export async function checkin(): Promise<PlayCheckinResult> {
  const { data } = await apiClient.post<PlayCheckinResult>('/play/checkin')
  return data
}

export async function checkinMakeup(): Promise<PlayCheckinResult> {
  const { data } = await apiClient.post<PlayCheckinResult>('/play/checkin/makeup')
  return data
}

export async function getArenaCurrent(): Promise<PlayArenaCurrent> {
  const { data } = await apiClient.get<PlayArenaCurrent>('/play/arena/current')
  return data
}

export async function getArenaDailyCurrent(): Promise<PlayArenaCurrent> {
  const { data } = await apiClient.get<PlayArenaCurrent>('/play/arena/daily/current')
  return data
}

export async function getArenaDailyLeaderboard(limit = 50): Promise<PlayArenaLeaderboard> {
  const { data } = await apiClient.get<PlayArenaLeaderboard>('/play/arena/daily/leaderboard', {
    params: { limit },
  })
  return data
}

export async function getQuestsToday(): Promise<PlayQuestToday> {
  const { data } = await apiClient.get<PlayQuestToday>('/play/quests/today')
  return data
}

export async function getArenaLeaderboard(limit = 50): Promise<PlayArenaLeaderboard> {
  const { data } = await apiClient.get<PlayArenaLeaderboard>('/play/arena/leaderboard', {
    params: { limit },
  })
  return data
}

export async function getBlindboxStatus(): Promise<PlayBlindboxStatus> {
  const { data } = await apiClient.get<PlayBlindboxStatus>('/play/blindbox/status')
  return data
}

export async function getBlindboxPool(): Promise<PlayBlindboxPoolResponse> {
  const { data } = await apiClient.get<PlayBlindboxPoolResponse>('/play/blindbox/pool')
  return data
}

export async function getBlindboxRecentWins(): Promise<PlayBlindboxRecentWin[]> {
  const { data } = await apiClient.get<PlayBlindboxRecentWin[]>('/play/blindbox/recent')
  return data ?? []
}

export async function openBlindbox(idempotencyKey?: string): Promise<PlayBlindboxOpenResult> {
  const headers = idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined
  const { data } = await apiClient.post<PlayBlindboxOpenResult>('/play/blindbox/open', {}, { headers })
  return data
}

export async function getQuizToday(): Promise<PlayQuizToday> {
  const { data } = await apiClient.get<PlayQuizToday>('/play/quiz/today')
  return data
}

export async function submitQuiz(answers: { question_id: number; choice_index: number }[]): Promise<PlayQuizSubmitResult> {
  const { data } = await apiClient.post<PlayQuizSubmitResult>('/play/quiz/submit', { answers })
  return data
}

export async function getTeamMe(): Promise<PlayTeamMe> {
  const { data } = await apiClient.get<PlayTeamMe>('/play/teams/me')
  return data
}

export async function createTeam(name: string): Promise<PlayTeamSummary> {
  const { data } = await apiClient.post<PlayTeamSummary>('/play/teams', { name })
  return data
}

export async function joinTeam(inviteCode: string): Promise<PlayTeamSummary> {
  const { data } = await apiClient.post<PlayTeamSummary>('/play/teams/join', { invite_code: inviteCode })
  return data
}

export const playAPI = {
  getPlayHub,
  getActiveCampaigns,
  getCheckinStatus,
  checkin,
  checkinMakeup,
  getArenaCurrent,
  getArenaLeaderboard,
  getArenaDailyCurrent,
  getArenaDailyLeaderboard,
  getQuestsToday,
  getBlindboxPool,
  getBlindboxStatus,
  openBlindbox,
  getBlindboxRecentWins,
  getQuizToday,
  submitQuiz,
  getTeamMe,
  createTeam,
  joinTeam,
}

export default playAPI
