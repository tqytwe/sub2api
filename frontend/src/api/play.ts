import { apiClient } from './client'

export interface PlayCheckinStatus {
  enabled: boolean
  eligible?: boolean
  ineligible_reason?: string
  checked_in_today: boolean
  reward_amount: number
  coupon_pool_ready?: boolean
  coupon_weight_bp?: number
  redeem_code_weight_bp?: number
  balance_weight_bp?: number
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
  reward_type?: PlayRewardType
  coupon?: PlayCouponReward
  redeem_code?: PlayRedeemCodeReward
  coupon_pool_version?: string
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
  estimated_reward?: number
  recharge_boost_active?: boolean
  arena_score_multiplier?: number
  campaign_active?: boolean
}

export interface PlayArenaScore {
  rank: number
  // Public boards deliberately never return a database user ID. An empty
  // display_name with anonymous=true is localized by the consuming surface.
  display_name?: string
  anonymous?: boolean
  avatar_url?: string
  token_sum: number
  is_mine?: boolean
}

export interface PlayArenaLeaderboard {
  enabled: boolean
  period?: PlayArenaPeriod
  rows: PlayArenaScore[]
}

export interface PlayArenaDailyRewardSummary {
  enabled: boolean
  recent?: PlayArenaDailyRecentRewardSummary
  current?: PlayArenaDailyCurrentRewardEstimate
}

export interface PlayArenaDailyRecentRewardSummary {
  period?: PlayArenaPeriod
  settled_at?: string
  paid_today: boolean
  winners_count: number
  total_amount: number
  winners: PlayArenaDailyRewardWinner[]
}

export interface PlayArenaDailyRewardWinner {
  rank: number
  display_name?: string
  anonymous?: boolean
  avatar_url?: string
  token_sum: number
  amount: number
}

export interface PlayArenaDailyCurrentRewardEstimate {
  period?: PlayArenaPeriod
  rows: PlayArenaDailyRewardEstimateRow[]
}

export interface PlayArenaDailyRewardEstimateRow {
  rank: number
  display_name?: string
  anonymous?: boolean
  avatar_url?: string
  token_sum: number
  estimated_reward: number
}

export interface PlayArenaMonthlyRewardSummary {
  enabled: boolean
  period?: PlayArenaPeriod
  settled_at?: string
  winners_count: number
  total_amount: number
  winners: PlayArenaMonthlyRewardWinner[]
}

export interface PlayArenaMonthlyRewardWinner {
  rank: number
  display_name?: string
  anonymous?: boolean
  avatar_url?: string
  amount: number
  paid_at?: string
}

export interface PlayArenaSeasonHistoryWinner extends PlayArenaScore {
  reward_amount: number
  payout_status: 'paid' | string
  paid_at?: string
}

export interface PlayArenaSeasonHistory {
  period?: PlayArenaPeriod
  winners_count: number
  total_amount: number
  winners: PlayArenaSeasonHistoryWinner[]
}

export interface PlayArenaSeasonOverview {
  enabled: boolean
  period?: PlayArenaPeriod
  current: PlayArenaCurrent
  rows: PlayArenaScore[]
  reward_tiers: PlayArenaSettlementTier[]
  history: PlayArenaSeasonHistory[]
}

export interface PlayArenaSettlementTier {
  rank_max: number
  amount: number
}

export interface PlayTeamRewardShowcase {
  winners: PlayTeamRewardShowcaseWinner[]
}

export interface PlayTeamRewardShowcaseWinner {
  settlement_month: string
  team_name: string
  display_name: string
  avatar_url?: string
  amount: number
  paid_at?: string
}

export interface PlayBlindboxStatus {
  enabled: boolean
  coupon_pool_ready?: boolean
  coupon_prizes?: PlayCouponPrizePreview[]
  coupon_weight_bp?: number
  redeem_code_weight_bp?: number
  balance_weight_bp?: number
  cost_amount: number
  pool?: PlayBlindboxPool
  current_pool?: PlayBlindboxPool
  next_pool?: PlayBlindboxPool
  vip_tier?: PlayVIPStatus
  expected_reward?: number
  next_expected_reward?: number
  pool_version?: string
  rtp_cap?: number
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
  coupon_pool_ready?: boolean
  coupon_prizes?: PlayCouponPrizePreview[]
  coupon_weight_bp?: number
  balance_weight_bp?: number
  pool: PlayBlindboxPool
  current_pool?: PlayBlindboxPool
  next_pool?: PlayBlindboxPool
  vip_tier?: PlayVIPStatus
  expected_reward?: number
  next_expected_reward?: number
  pool_version?: string
  rtp_cap?: number
}

export interface PlayBlindboxOpenResult {
  cost_amount: number
  reward_amount: number
  net_amount: number
  reward_type?: PlayRewardType
  coupon?: PlayCouponReward
  redeem_code?: PlayRedeemCodeReward
  coupon_pool_version?: string
  opens_today: number
  server_date: string
  pool_version: string
  open_source: string
  vip_tier?: PlayVIPStatus
  expected_reward?: number
  rtp_cap?: number
}

export type PlayRewardType = 'none' | 'balance' | 'coupon' | 'redeem_code'

export interface PlayCouponPrizePreview {
  template_id: number
  name: string
  weight_bp: number
  tier: 'common' | 'standard' | 'rare' | 'jackpot' | string
}

export interface PlayCouponReward {
  user_coupon_id: number
  template_id: number
  name: string
  benefit_type: 'fixed_amount' | 'percentage'
  benefit_value: number
  max_discount_amount?: number | null
  currency: string
  applicable_scopes: ('balance' | 'subscription')[]
  minimum_order_amount: number
  valid_from: string
  expires_at: string
}

export interface PlayRedeemCodeReward {
  id: number
  code: string
  type: string
  value: number
  status: string
  batch_name?: string
  issued_at?: string | null
  expires_at?: string | null
  reward_pool_version?: string
}

export interface PlayBlindboxRecentWin {
  user: string
  reward: number
  reward_type?: PlayRewardType
  coupon_name?: string
  when: string
}

export interface PlayQuizQuestion {
  id: number
  prompt: string
  options: string[]
}

export interface PlayQuizToday {
  enabled: boolean
  coupon_pool_ready?: boolean
  questions: PlayQuizQuestion[]
  already_submitted: boolean
  previous_score?: number
  previous_total?: number
  previous_reward?: number
  previous_reward_type?: PlayRewardType
  previous_coupon?: PlayCouponReward
  previous_redeem_code?: PlayRedeemCodeReward
  previous_coupon_pool_version?: string
  reward_per_correct: number
  server_date: string
}

export interface PlayQuizSubmitResult {
  score: number
  total: number
  reward_amount: number
  reward_type?: PlayRewardType
  coupon?: PlayCouponReward
  redeem_code?: PlayRedeemCodeReward
  coupon_pool_version?: string
  server_date: string
}

export interface PlayTeamMember {
  user_id: number
  display_name: string
  email?: string
  avatar_url?: string
  joined_at: string
  token_sum: number
  token_pct: number
  spend: string
  spend_pct: number
  estimated_reward: string
  latest_settlement_month?: string
  latest_actual_reward?: string
  latest_payout_status?: 'pending' | 'processing' | 'paid' | 'failed'
  latest_paid_at?: string
}

export interface TeamRewardTier {
  threshold: string
  rate: string
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
  invite_code?: string
  is_recruiting?: boolean
  captain_id: number
  member_count: number
  token_sum: number
  members: PlayTeamMember[]
  affiliate?: PlayTeamAffiliateInfo
  current_month: string
  team_spend: string
  reached_threshold: string
  reward_rate: string
  next_threshold: string
  estimated_pool: string
  reward_cap: string
  reward_tiers: TeamRewardTier[]
}

export interface PlayTeamMe {
  enabled: boolean
  team?: PlayTeamSummary
}

export interface PlayTeamRewardAllocation {
  id: number
  settlement_id: number
  user_id: number
  display_name?: string
  avatar_url?: string
  email?: string
  contribution: string
  ratio: string
  reward_amount: string
  payout_status: 'pending' | 'processing' | 'paid' | 'failed'
  paid_at?: string
  last_error?: string
}

export interface PlayTeamSettlement {
  id: number
  team_id: number
  period_start: string
  window_start: string
  window_end: string
  team_spend: string
  reached_threshold: string
  reward_rate: string
  pool_amount: string
  cap_amount: string
  status: 'pending' | 'processing' | 'completed' | 'partial' | 'failed'
  last_error?: string
  completed_at?: string
}

export interface PlayTeamSettlementRecord {
  settlement: PlayTeamSettlement
  allocations: PlayTeamRewardAllocation[]
}

export interface PlayUserTeamSettlementRecord {
  settlement_id: number
  team_id: number
  team_name: string
  settlement_month: string
  team_spend: string
  pool_amount: string
  settlement_status: PlayTeamSettlement['status']
  personal_contribution: string
  personal_ratio: string
  personal_reward: string
  payout_status: PlayTeamRewardAllocation['payout_status']
  paid_at?: string
}

export type PlayTeamSettlementHistoryRecord = PlayTeamSettlementRecord | PlayUserTeamSettlementRecord

export interface PlayVIPStatus {
  tier: number
  label: string
  recharge_bonus_pct: number
  color_key: string
  perks?: string[]
  next_tier?: number
  next_label?: string
  next_min_recharge?: number
  amount_to_next?: number
}

export interface PlayVIPTier {
  tier: number
  label: string
  min_recharge: number
  recharge_bonus_pct: number
  color_key: string
  perks?: string[]
}

export interface PlayCampaignRules {
  recharge_bonus_pct?: number
  blindbox_extra_opens?: number
  arena_score_multiplier?: number
  name_i18n?: Record<string, string>
  campaign_type?: 'benefit_overlay' | 'new_user_growth' | 'hybrid'
  referral_campaign_id?: number
  qualification_metric?: 'net_recharge' | 'actual_consumption'
  reward_tiers?: PlayCampaignRewardTier[]
  require_invite?: boolean
  legacy_rebate_policy?: 'exclude' | 'stack'
}

export interface PlayCampaignRewardTier {
  tier: number
  required_amount: number
  reward_amount: number
  currency: 'CNY'
}

export interface PlayNewUserGrowthRewardProgress extends PlayCampaignRewardTier {
  reward_id?: number
  status?: 'claimable' | 'claimed_frozen' | 'available' | 'expired' | 'revoked' | 'debt_review' | 'resolved'
}

export interface PlayNewUserGrowthProgress {
  eligible: boolean
  referral_campaign_id: number
  referral_version: number
  qualification_metric: 'net_recharge' | 'actual_consumption'
  qualified_amount: number
  rewards: PlayNewUserGrowthRewardProgress[]
}

export interface PlayCampaignAudience {
  all?: boolean
  ordinary?: boolean
  member?: boolean
  vip_tiers?: number[]
  registered_within_days?: number
}

export interface PlayCampaignSummary {
  id: number
  name: string
  start_at: string
  end_at: string
  rules: PlayCampaignRules
  new_user_growth?: PlayNewUserGrowthProgress
}

export interface PlayTeamLeaderboardEntry {
  rank: number
  team_id: number
  team_name: string
  member_count: number
  monthly_spend: string
  estimated_pool: string
  gap_to_previous: string
  is_mine: boolean
}

export interface PlayTeamLeaderboard {
  rows: PlayTeamLeaderboardEntry[]
  month: string
  total_teams: number
}

// Public team competition data is team-only by contract. It must not contain
// invite codes, member identities, personal spend, or payout allocations.
export interface PlayTeamDirectoryEntry {
  team_id: number
  team_name: string
  member_count: number
  member_capacity: number
  monthly_spend: string
  estimated_pool: string
  accepting_applications: boolean
}

export interface PlayTeamDirectory {
  month: string
  rows: PlayTeamDirectoryEntry[]
}

export interface PlayTeamPublicLeaderboardEntry {
  rank: number
  team_id: number
  team_name: string
  member_count: number
  monthly_spend: string
  estimated_pool: string
  gap_to_previous: string
}

export interface PlayTeamPublicLeaderboard {
  month: string
  total_teams: number
  rows: PlayTeamPublicLeaderboardEntry[]
}

export interface PlayTeamSeason {
  id: number
  month: string
  window_start?: string
  window_end?: string
  rules?: Record<string, unknown>
  status: string
  frozen_at?: string
  settled_at?: string
}

export interface PlayTeamSeasonRanking {
  rank: number
  team_id: number
  team_name: string
  member_count: number
  team_spend: string
  reached_threshold?: string
  reward_rate?: string
  pool_amount: string
  paid_amount: string
  settlement_status: string
}

export interface PlayTeamSeasonDetail {
  season: PlayTeamSeason
  total_teams: number
  rows: PlayTeamSeasonRanking[]
}

export interface PlayTeamJoinApplication {
  id: number
  team_id: number
  // This field is intentionally never rendered. A future backend may attach a
  // masked display field for captains, but the immutable lifecycle ID remains
  // solely an action target.
  applicant_user_id?: number
  applicant_display_name?: string
  applicant_avatar_url?: string
  status: 'pending' | 'approved' | 'rejected' | 'withdrawn' | 'expired' | string
  message?: string
  requested_at: string
  sla_due_at: string
  expires_at: string
  handled_at?: string
  decision_note?: string
}

export interface PlayTeamInvite {
  invite_code: string
  expires_at: string
  rotated_at?: string
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
  vip_tiers?: PlayVIPTier[]
  membership_paid_amount?: number
  is_member: boolean
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

export async function getTeamLeaderboard(): Promise<PlayTeamLeaderboard> {
  const { data } = await apiClient.get<PlayTeamLeaderboard>('/play/teams/leaderboard')
  return data
}

export async function getTeamDirectory(limit = 20): Promise<PlayTeamDirectory> {
  const { data } = await apiClient.get<PlayTeamDirectory>('/play/teams/directory', { params: { limit } })
  return data
}

export async function getTeamPublicLeaderboard(limit = 50): Promise<PlayTeamPublicLeaderboard> {
  const { data } = await apiClient.get<PlayTeamPublicLeaderboard>('/play/teams/leaderboard/public', { params: { limit } })
  return data
}

export async function getTeamSeasons(limit = 12): Promise<PlayTeamSeason[]> {
  const { data } = await apiClient.get<PlayTeamSeason[]>('/play/teams/seasons', { params: { limit } })
  return data ?? []
}

export async function getTeamSeason(month: string, limit = 10): Promise<PlayTeamSeasonDetail> {
  const { data } = await apiClient.get<PlayTeamSeasonDetail>(`/play/teams/seasons/${encodeURIComponent(month)}`, { params: { limit } })
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

// The aggregation reads one selected Farm period. History is opt-in so a
// ranking tab can render promptly while immutable payout proof loads only when
// the user expands it.
export async function getArenaSeasonOverview(
  period: 'daily' | 'monthly',
  includeHistory = false,
): Promise<PlayArenaSeasonOverview> {
  const { data } = await apiClient.get<PlayArenaSeasonOverview>('/play/arena/overview', {
    params: {
      period,
      include_history: includeHistory ? '1' : '0',
    },
  })
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

export async function getArenaDailyRewardSummary(): Promise<PlayArenaDailyRewardSummary> {
  const { data } = await apiClient.get<PlayArenaDailyRewardSummary>('/play/arena/daily/reward-summary')
  return data
}

export async function getArenaRewardSummary(): Promise<PlayArenaMonthlyRewardSummary> {
  const { data } = await apiClient.get<PlayArenaMonthlyRewardSummary>('/play/arena/reward-summary')
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

export async function applyToTeam(teamID: number, message = ''): Promise<PlayTeamJoinApplication> {
  const { data } = await apiClient.post<PlayTeamJoinApplication>('/play/teams/applications', {
    team_id: teamID,
    message,
  })
  return data
}

export async function getTeamMyApplications(limit = 20): Promise<PlayTeamJoinApplication[]> {
  const { data } = await apiClient.get<PlayTeamJoinApplication[]>('/play/teams/applications/me', { params: { limit } })
  return data ?? []
}

export async function getTeamCaptainApplications(limit = 50): Promise<PlayTeamJoinApplication[]> {
  const { data } = await apiClient.get<PlayTeamJoinApplication[]>('/play/teams/applications', { params: { limit } })
  return data ?? []
}

export async function decideTeamApplication(applicationID: number, decision: 'approve' | 'reject', note = ''): Promise<PlayTeamJoinApplication> {
  const { data } = await apiClient.post<PlayTeamJoinApplication>(`/play/teams/applications/${applicationID}/decision`, {
    decision,
    note,
  })
  return data
}

export async function rotateTeamInvite(): Promise<PlayTeamInvite> {
  const { data } = await apiClient.post<PlayTeamInvite>('/play/teams/invite/rotate')
  return data
}

export async function setTeamRecruiting(recruiting: boolean): Promise<{ recruiting: boolean }> {
  const { data } = await apiClient.put<{ recruiting: boolean }>('/play/teams/recruiting', { recruiting })
  return data
}

export async function leaveTeam(): Promise<void> {
  await apiClient.post('/play/teams/leave')
}

export async function transferTeam(targetUserId: number): Promise<void> {
  await apiClient.post('/play/teams/transfer', { target_user_id: targetUserId })
}

export async function removeTeamMember(targetUserId: number): Promise<void> {
  await apiClient.post('/play/teams/remove', { target_user_id: targetUserId })
}

export async function getTeamSettlements(): Promise<PlayTeamSettlementHistoryRecord[]> {
  const { data } = await apiClient.get<PlayTeamSettlementHistoryRecord[]>('/play/teams/settlements')
  return data ?? []
}

export async function getTeamRewardShowcase(): Promise<PlayTeamRewardShowcase> {
  const { data } = await apiClient.get<PlayTeamRewardShowcase>('/play/teams/reward-showcase')
  return data ?? { winners: [] }
}

export const playAPI = {
  getPlayHub,
  getActiveCampaigns,
  getCheckinStatus,
  checkin,
  checkinMakeup,
  getArenaCurrent,
  getArenaSeasonOverview,
  getArenaLeaderboard,
  getArenaDailyCurrent,
  getArenaDailyLeaderboard,
  getArenaDailyRewardSummary,
  getArenaRewardSummary,
  getQuestsToday,
  getBlindboxPool,
  getBlindboxStatus,
  openBlindbox,
  getBlindboxRecentWins,
  getQuizToday,
  submitQuiz,
  getTeamMe,
  getTeamLeaderboard,
  getTeamDirectory,
  getTeamPublicLeaderboard,
  getTeamSeasons,
  getTeamSeason,
  createTeam,
  joinTeam,
  applyToTeam,
  getTeamMyApplications,
  getTeamCaptainApplications,
  decideTeamApplication,
  rotateTeamInvite,
  setTeamRecruiting,
  leaveTeam,
  transferTeam,
  removeTeamMember,
  getTeamSettlements,
  getTeamRewardShowcase,
}

export default playAPI
