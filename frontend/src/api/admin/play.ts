import { apiClient } from "../client";
import type {
  PlayArenaPeriod,
  PlayBlindboxPool,
  PlayCampaignRules,
  PlayCampaignAudience,
  PlayTeamSettlementRecord,
  PlayTeamSummary,
  TeamRewardTier,
} from "../play";

export type { PlayBlindboxPool, PlayBlindboxPoolTier } from "../play";

export async function getBlindboxPool(): Promise<PlayBlindboxPool> {
  const { data } = await apiClient.get<PlayBlindboxPool>(
    "/admin/play/blindbox/pool",
  );
  return data;
}

export async function updateBlindboxPool(
  pool: PlayBlindboxPool,
): Promise<PlayBlindboxPool> {
  const { data } = await apiClient.put<PlayBlindboxPool>(
    "/admin/play/blindbox/pool",
    pool,
  );
  return data;
}

export interface TeamRewardSettings {
  enabled: boolean;
  tiers: TeamRewardTier[];
  cap: string;
  start_month: string;
}

export interface AdminArenaScore {
  rank: number;
  user_id: number;
  display_name: string;
  email?: string;
  avatar_url?: string;
  token_sum: number;
  estimated_reward: number;
}

export interface AdminArenaRewardTier {
  rank_max: number;
  amount: number;
}

export interface AdminArenaRewardSettings {
  monthly: AdminArenaRewardTier[];
  daily: AdminArenaRewardTier[];
  daily_budget: number;
}

export interface AdminArenaLeaderboard {
  period?: PlayArenaPeriod;
  rewards: AdminArenaRewardTier[];
  rows: AdminArenaScore[];
}

export interface AdminPlayTeamListItem {
  id: number;
  name: string;
  invite_code: string;
  captain_id: number;
  captain_display_name: string;
  captain_avatar_url?: string;
  captain_email?: string;
  member_count: number;
  token_sum: number;
  team_spend: string;
  estimated_pool: string;
  created_at: string;
  archived_at?: string;
}

export interface AdminPlayTeamList {
  items: AdminPlayTeamListItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface AdminPlayOpsSummary {
  total_teams: number;
  active_teams: number;
  month_spend: string;
  estimated_shared_pool: string;
  pending_failed_settlements: number;
  monthly_arena_reward_budget: number;
  daily_arena_reward_budget: number;
}

export interface AdminPlayCampaign {
  id: number;
  name: string;
  start_at: string;
  end_at: string;
  rules: PlayCampaignRules;
  audience: PlayCampaignAudience;
  enabled: boolean;
  created_at: string;
}

export interface AdminPlayCampaignInput {
  name: string;
  start_at: string;
  end_at: string;
  rules: PlayCampaignRules;
  audience: PlayCampaignAudience;
  enabled: boolean;
}

export interface AdminPlayTeamDetail {
  team: PlayTeamSummary;
  created_at: string;
  archived_at?: string;
  settlements: PlayTeamSettlementRecord[];
}

export type AdminTeamMemberOperation = "add" | "move";

export interface AdminTeamReference {
  id: number;
  name: string;
  archived_at?: string;
}

export interface AdminTeamMemberImpact {
  effective_at: string;
  user_spend: string;
  source_spend_before: string;
  source_spend_after: string;
  source_pool_before: string;
  source_pool_after: string;
  target_spend_before: string;
  target_spend_after: string;
  target_pool_before: string;
  target_pool_after: string;
}

export interface AdminTeamMemberCandidate {
  user_id: number;
  email: string;
  username: string;
  display_name: string;
  status: string;
  current_team?: AdminTeamReference;
  current_joined_at?: string;
  is_captain: boolean;
  affiliate?: {
    inviter_user_id: number;
    inviter_display_name: string;
  };
  impact: AdminTeamMemberImpact;
  blockers: string[];
  warnings: string[];
}

export interface AdminTeamMemberCandidateList {
  items: AdminTeamMemberCandidate[];
  effective_at: string;
}

export interface AdminTeamMemberRepairInput {
  user_id: number;
  operation: AdminTeamMemberOperation;
  effective_at?: string;
  reason: string;
  expected_source_team_id?: number;
}

export interface AdminTeamMemberRepairResult {
  status: "added" | "moved" | "no_op";
  team_id: number;
  user_id: number;
  source_team_id?: number;
  effective_at: string;
  warnings: string[];
}

export interface AdminTeamEvent {
  id: number;
  team_id: number;
  actor_user_id: number;
  actor_display_name: string;
  subject_user_id?: number;
  subject_display_name?: string;
  event_type: string;
  detail: Record<string, unknown>;
  created_at: string;
}

export type AdminMobileFeedbackStatus =
  "new" | "viewed" | "handled" | "deferred" | "ignored";

export interface AdminMobileFeedbackScreenshot {
  url: string;
  file_name?: string;
  content_type?: string;
  byte_size?: number;
}

export interface AdminMobileFeedback {
  id: number;
  user_id: number;
  user_email?: string;
  user_name?: string;
  title: string;
  category: string;
  content: string;
  status: AdminMobileFeedbackStatus;
  app_version: string;
  platform: string;
  device_model: string;
  android_version: string;
  system_version: string;
  group_name: string;
  group_id?: number;
  backend_url: string;
  last_error: string;
  crash_log: string;
  device_info: Record<string, unknown>;
  screenshots: AdminMobileFeedbackScreenshot[];
  admin_note: string;
  installation_id?: string;
  channel?: string;
  referrer?: string;
  version: number;
  updated_by?: number;
  status_changed_at?: string;
  context?: AdminMobileFeedbackContext;
  created_at: string;
  updated_at: string;
}

export interface AdminMobileFeedbackList {
  items: AdminMobileFeedback[];
  total: number;
  page: number;
  page_size: number;
}

export interface AdminMobileFeedbackUpdateInput {
  status: AdminMobileFeedbackStatus;
  admin_note?: string;
  expected_version: number;
}

export interface AdminMembershipOverview {
  total_members: number
  tier_counts: Array<{ tier: number; label: string; count: number }>
  net_paid_amount: string
  recent_upgrades: number
  recent_downgrades: number
}

export interface AdminMembershipListItem {
  user_id: number
  email_masked: string
  username?: string
  tier: number
  tier_label: string
  is_member: boolean
  net_paid_amount: string
  registered_at?: string
  first_paid_at?: string
  last_paid_at?: string
}

export interface AdminMembershipList {
  items: AdminMembershipListItem[]
  total: number
  page: number
  page_size: number
}

export interface AdminVIPTier {
  tier: number
  label: string
  min_recharge: number
  recharge_bonus_pct: number
  color_key: string
  perks?: string[]
}

export interface AdminVIPConfigImpact {
  version: number
  tiers: AdminVIPTier[]
  affected_users: number
  upgraded_users: number
  downgraded_users: number
}

export interface AdminMembershipDetail {
  user: AdminMembershipListItem
  contributions: Array<{
    order_id: number
    order_type: string
    paid_amount: string
    refund_amount: string
    net_amount: string
    paid_at?: string
    status: string
    updated_at: string
  }>
  tier_history: Array<{
    order_id?: number
    from_tier: number
    to_tier: number
    net_paid_before: string
    net_paid_after: string
    reason: string
    created_at: string
  }>
}

export interface AdminInviteGrowthOverview {
  invited_count: number
  qualified_count: number
  paid_invitee_count: number
  reward_unlocked: string
  reward_claimed: string
  ranking?: Array<{
    rank: number
    display_name?: string
    email_masked: string
    qualified_count: number
    net_paid_amount: string
    reward_amount: string
    is_me?: boolean
  }>
}

export type AdminReferralCampaignStatus =
  | "draft"
  | "review"
  | "approved"
  | "scheduled"
  | "running"
  | "paused"
  | "settling"
  | "closed"
  | "cancelled";

export type AdminReferralReviewType = "ops" | "finance" | "risk" | "ux";
export type AdminReferralReviewDecision = "approved" | "rejected";

export interface AdminReferralCampaign {
  id: number;
  key: string;
  name: string;
  status: AdminReferralCampaignStatus;
  version: number;
  registration_from: string;
  registration_to: string;
  starts_at: string;
  ends_at: string;
  qualification_to: string;
  claim_deadline: string;
  risk_hold_hours: number;
  pay_threshold: number;
  usage_threshold: number;
  max_enrollments: number;
  budget_total: number;
  budget_reserved: number;
  budget_paid: number;
  reward_mode: "additive" | "replace";
  created_by: number;
  approved_by?: number;
}

export interface AdminReferralCampaignTier {
  tier: number;
  required_invites: number;
  reward_amount: number;
  currency: "CNY";
}

export interface AdminReferralCampaignStats {
  campaign_id: number;
  enrolled: number;
  attributed: number;
  qualified: number;
  risk_pending: number;
  risk_rejected: number;
  rewards_reserved: number;
  rewards_claimed: number;
  rewards_expired: number;
  rewards_revoked: number;
}

export interface AdminReferralCampaignApproval {
  version: number;
  review_type: AdminReferralReviewType;
  decision: AdminReferralReviewDecision;
  reviewer_id?: number;
  note: string;
  created_at: string;
}

export interface AdminReferralCampaignDetail {
  campaign: AdminReferralCampaign;
  tiers: AdminReferralCampaignTier[];
  stats: AdminReferralCampaignStats;
  approvals: AdminReferralCampaignApproval[];
}

export interface AdminReferralCampaignInput {
  key: string;
  name: string;
  registration_from: string;
  registration_to: string;
  starts_at: string;
  ends_at: string;
  qualification_to: string;
  claim_deadline: string;
  risk_hold_hours: number;
  pay_threshold: number;
  usage_threshold: number;
  max_enrollments: number;
  budget_total: number;
  reward_mode: "additive" | "replace";
  tiers: AdminReferralCampaignTier[];
  expected_version?: number;
}

export interface AdminReferralCampaignPage {
  items: AdminReferralCampaign[];
  total: number;
  page: number;
  page_size: number;
}

export interface AdminReferralCampaignParticipant {
  user_id: number;
  email: string;
  username: string;
  enrolled_at: string;
  invited_count: number;
  qualified_count: number;
  reward_unlocked: number;
  reward_claimed: number;
}

export interface AdminReferralCampaignInvite {
  attribution_id: number;
  inviter_id: number;
  inviter_email: string;
  invitee_id: number;
  invitee_email: string;
  registered_at: string;
  status: string;
  net_paid: number;
  actual_cost: number;
  risk_status: string;
  qualification_status: string;
  qualified_at?: string;
}

export interface AdminReferralCampaignReward {
  id: number;
  campaign_id: number;
  user_id: number;
  tier: number;
  reward_type: string;
  amount: number;
  currency: string;
  status: string;
  unlock_at?: string;
  claim_deadline?: string;
  frozen_until?: string;
  version: number;
  email: string;
  username: string;
}

export interface AdminReferralPage<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface AdminAppAnalytics {
  period: string
  scans: number
  download_redirects: number
  first_launches: number
  installs: number
  registered_installs: number
  active_users: number
  dau: number
  wau: number
  mau: number
  funnel: Array<{ event: string; count: number; conversion_rate?: number }>
  versions: Array<{ version: string; platform: string; active: number; share?: number }>
}

export interface AdminMobileFeedbackContext {
  installation_id?: string
  registration_status?: string
  acquisition_source?: string
  first_launch_at?: string
  last_seen_at?: string
}

export type AdminQuizQuestionLanguage = "zh" | "en";
export type AdminQuizQuestionDifficulty = "easy" | "normal" | "hard";

export interface AdminQuizQuestion {
  id: number;
  language: AdminQuizQuestionLanguage;
  prompt: string;
  options: string[];
  correct_index: number;
  category: string;
  difficulty: AdminQuizQuestionDifficulty;
  explanation: string;
  sort_order: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface AdminQuizQuestionInput {
  language: AdminQuizQuestionLanguage;
  prompt: string;
  options: string[];
  correct_index: number;
  category: string;
  difficulty: AdminQuizQuestionDifficulty;
  explanation: string;
  sort_order: number;
  active: boolean;
}

export interface AdminQuizQuestionStats {
  total: number;
  active: number;
  inactive: number;
  zh_active: number;
  en_active: number;
  categories: string[];
  difficulties: AdminQuizQuestionDifficulty[];
}

export interface AdminQuizQuestionList {
  items: AdminQuizQuestion[];
  total: number;
  page: number;
  page_size: number;
  stats: AdminQuizQuestionStats;
}

export interface AdminQuizQuestionListParams {
  language?: AdminQuizQuestionLanguage | "";
  active?: boolean | "";
  category?: string;
  difficulty?: AdminQuizQuestionDifficulty | "";
  q?: string;
  page?: number;
  page_size?: number;
}

const teamMemberRepairOperationKeys = new Map<string, string>();

function currentAdminID(): string | null {
  try {
    const rawUser = globalThis.localStorage?.getItem("auth_user");
    if (!rawUser) return null;
    const user: unknown = JSON.parse(rawUser);
    if (!user || typeof user !== "object") return null;
    const id = (user as { id?: unknown }).id;
    return typeof id === "number" && Number.isSafeInteger(id) && id > 0
      ? String(id)
      : null;
  } catch {
    return null;
  }
}

function hashRepairPayload(input: AdminTeamMemberRepairInput): string {
  const serialized = JSON.stringify(input);
  let hash = 0x811c9dc5;
  for (let index = 0; index < serialized.length; index += 1) {
    hash ^= serialized.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return (hash >>> 0).toString(16).padStart(8, "0");
}

function teamMemberRepairOperationScope(
  teamID: number,
  input: AdminTeamMemberRepairInput,
): { adminID: string; storageKey: string } | null {
  const adminID = currentAdminID();
  if (!adminID) return null;
  const payloadHash = hashRepairPayload(input);
  return {
    adminID,
    storageKey: `sub2api:admin:play-team-repair:${adminID}:${teamID}:${input.user_id}:${payloadHash}`,
  };
}

function storedTeamMemberRepairKey(storageKey: string): string | null {
  try {
    return globalThis.sessionStorage?.getItem(storageKey) ?? null;
  } catch {
    return null;
  }
}

function storeTeamMemberRepairKey(
  storageKey: string,
  value: string | null,
): void {
  try {
    if (value) globalThis.sessionStorage?.setItem(storageKey, value);
    else globalThis.sessionStorage?.removeItem(storageKey);
  } catch {
    // The in-memory map still protects retries while this page remains open.
  }
}

export async function getTeamRewardSettings(): Promise<TeamRewardSettings> {
  const { data } = await apiClient.get<TeamRewardSettings>(
    "/admin/play/team-rewards/settings",
  );
  return data;
}

export async function updateTeamRewardSettings(
  settings: TeamRewardSettings,
): Promise<TeamRewardSettings> {
  const { data } = await apiClient.put<TeamRewardSettings>(
    "/admin/play/team-rewards/settings",
    settings,
  );
  return data;
}

export async function listTeamRewardSettlements(): Promise<
  PlayTeamSettlementRecord[]
> {
  const { data } = await apiClient.get<PlayTeamSettlementRecord[]>(
    "/admin/play/team-rewards/settlements",
  );
  return data ?? [];
}

export async function retryTeamRewardSettlement(id: number): Promise<void> {
  await apiClient.post(`/admin/play/team-rewards/settlements/${id}/retry`);
}

export async function getArenaLeaderboard(
  params: {
    period_type?: "daily" | "monthly";
    period_id?: number;
    limit?: number;
  } = {},
): Promise<AdminArenaLeaderboard> {
  const { data } = await apiClient.get<AdminArenaLeaderboard>(
    "/admin/play/arena/leaderboard",
    { params },
  );
  return data;
}

export async function getArenaRewardSettings(): Promise<AdminArenaRewardSettings> {
  const { data } = await apiClient.get<AdminArenaRewardSettings>(
    "/admin/play/arena/rewards",
  );
  return data;
}

export async function updateArenaRewardSettings(
  settings: AdminArenaRewardSettings,
): Promise<AdminArenaRewardSettings> {
  const { data } = await apiClient.put<AdminArenaRewardSettings>(
    "/admin/play/arena/rewards",
    settings,
  );
  return data;
}

export async function getSummary(): Promise<AdminPlayOpsSummary> {
  const { data } = await apiClient.get<AdminPlayOpsSummary>(
    "/admin/play/summary",
  );
  return data;
}

export async function listCampaigns(): Promise<AdminPlayCampaign[]> {
  const { data } = await apiClient.get<AdminPlayCampaign[]>(
    "/admin/play/campaigns",
  );
  return data ?? [];
}

export async function createCampaign(
  input: AdminPlayCampaignInput,
): Promise<AdminPlayCampaign> {
  const { data } = await apiClient.post<AdminPlayCampaign>(
    "/admin/play/campaigns",
    input,
  );
  return data;
}

export async function updateCampaign(
  id: number,
  input: AdminPlayCampaignInput,
): Promise<AdminPlayCampaign> {
  const { data } = await apiClient.put<AdminPlayCampaign>(
    `/admin/play/campaigns/${id}`,
    input,
  );
  return data;
}

export async function deleteCampaign(id: number): Promise<void> {
  await apiClient.delete(`/admin/play/campaigns/${id}`);
}

export async function listTeams(
  params: {
    status?: "active" | "archived" | "all";
    q?: string;
    page?: number;
    page_size?: number;
  } = {},
): Promise<AdminPlayTeamList> {
  const { data } = await apiClient.get<AdminPlayTeamList>("/admin/play/teams", {
    params,
  });
  return data;
}

export async function getTeam(id: number): Promise<AdminPlayTeamDetail> {
  const { data } = await apiClient.get<AdminPlayTeamDetail>(
    `/admin/play/teams/${id}`,
  );
  return data;
}

export async function getTeamSettlements(
  id: number,
): Promise<PlayTeamSettlementRecord[]> {
  const { data } = await apiClient.get<PlayTeamSettlementRecord[]>(
    `/admin/play/teams/${id}/settlements`,
  );
  return data ?? [];
}

export async function listTeamMemberCandidates(
  id: number,
  params: {
    q: string;
    operation: AdminTeamMemberOperation;
    effective_at?: string;
    limit?: number;
  },
): Promise<AdminTeamMemberCandidateList> {
  const { data } = await apiClient.get<AdminTeamMemberCandidateList>(
    `/admin/play/teams/${id}/member-candidates`,
    { params },
  );
  return data;
}

export async function repairTeamMember(
  id: number,
  input: AdminTeamMemberRepairInput,
): Promise<AdminTeamMemberRepairResult> {
  const scope = teamMemberRepairOperationScope(id, input);
  let idempotencyKey = scope
    ? (teamMemberRepairOperationKeys.get(scope.storageKey) ??
      storedTeamMemberRepairKey(scope.storageKey))
    : null;
  if (!idempotencyKey) {
    const requestID =
      globalThis.crypto?.randomUUID?.() ??
      `${Date.now()}-${Math.random().toString(16).slice(2)}`;
    idempotencyKey = `play-team-repair-${scope?.adminID ?? "unknown-admin"}-${id}-${input.user_id}-${requestID}`;
  }
  if (scope) {
    teamMemberRepairOperationKeys.set(scope.storageKey, idempotencyKey);
    storeTeamMemberRepairKey(scope.storageKey, idempotencyKey);
  }

  const { data } = await apiClient.post<AdminTeamMemberRepairResult>(
    `/admin/play/teams/${id}/members`,
    input,
    { headers: { "Idempotency-Key": idempotencyKey } },
  );
  if (scope) {
    teamMemberRepairOperationKeys.delete(scope.storageKey);
    storeTeamMemberRepairKey(scope.storageKey, null);
  }
  return data;
}

export async function listTeamEvents(id: number): Promise<AdminTeamEvent[]> {
  const { data } = await apiClient.get<AdminTeamEvent[]>(
    `/admin/play/teams/${id}/events`,
  );
  return data ?? [];
}

export async function listMobileFeedback(
  params: {
    status?: string;
    q?: string;
    page?: number;
    page_size?: number;
  } = {},
): Promise<AdminMobileFeedbackList> {
  const { data } = await apiClient.get<AdminMobileFeedbackList>(
    "/admin/play/mobile-feedback",
    { params },
  );
  return data;
}

export async function getMobileFeedback(
  id: number,
): Promise<AdminMobileFeedback> {
  const { data } = await apiClient.get<AdminMobileFeedback>(
    `/admin/play/mobile-feedback/${id}`,
  );
  return data;
}

export async function updateMobileFeedback(
  id: number,
  input: AdminMobileFeedbackUpdateInput,
): Promise<AdminMobileFeedback> {
  const { data } = await apiClient.patch<AdminMobileFeedback>(
    `/admin/play/mobile-feedback/${id}`,
    input,
  );
  return data;
}

export async function getMembershipOverview(): Promise<AdminMembershipOverview> {
  const { data } = await apiClient.get<AdminMembershipOverview>(
    "/admin/play/membership/overview",
  );
  return data;
}

export async function listMembershipUsers(
  params: {
    q?: string;
    tier?: number;
    member?: boolean;
    page?: number;
    page_size?: number;
  } = {},
): Promise<AdminMembershipList> {
  const { data } = await apiClient.get<AdminMembershipList>(
    "/admin/play/membership/users",
    { params },
  );
  return data;
}

export async function getMembershipUser(id: number): Promise<AdminMembershipDetail> {
  const { data } = await apiClient.get<AdminMembershipDetail>(`/admin/play/membership/users/${id}`)
  return data
}

export async function getVIPConfig(): Promise<AdminVIPConfigImpact> {
  const { data } = await apiClient.get<AdminVIPConfigImpact>("/admin/play/membership/vip-config")
  return data
}

export async function previewVIPConfig(tiers: AdminVIPTier[]): Promise<AdminVIPConfigImpact> {
  const { data } = await apiClient.post<AdminVIPConfigImpact>("/admin/play/membership/vip-config/preview", { tiers })
  return data
}

export async function publishVIPConfig(input: { tiers: AdminVIPTier[]; expected_version: number; reason: string }): Promise<AdminVIPConfigImpact> {
  const { data } = await apiClient.put<AdminVIPConfigImpact>("/admin/play/membership/vip-config", input)
  return data
}

export async function getInviteGrowthOverview(
  params: { campaign_id?: number; period?: string } = {},
): Promise<AdminInviteGrowthOverview> {
  const { data } = await apiClient.get<AdminInviteGrowthOverview>(
    "/admin/affiliates/invite-growth/overview",
    { params },
  );
  return data;
}

export async function listReferralCampaigns(
  params: { page?: number; page_size?: number; status?: string; search?: string } = {},
): Promise<AdminReferralCampaignPage> {
  const { data } = await apiClient.get<AdminReferralCampaignPage>(
    "/admin/affiliates/campaigns",
    { params },
  );
  return data;
}

export async function getReferralCampaign(id: number): Promise<AdminReferralCampaignDetail> {
  const { data } = await apiClient.get<AdminReferralCampaignDetail>(
    `/admin/affiliates/campaigns/${id}`,
  );
  return data;
}

export async function createReferralCampaign(
  input: AdminReferralCampaignInput,
): Promise<AdminReferralCampaign> {
  const { data } = await apiClient.post<AdminReferralCampaign>(
    "/admin/affiliates/campaigns",
    input,
  );
  return data;
}

export async function updateReferralCampaign(
  id: number,
  input: AdminReferralCampaignInput & { expected_version: number },
): Promise<AdminReferralCampaign> {
  const { data } = await apiClient.put<AdminReferralCampaign>(
    `/admin/affiliates/campaigns/${id}`,
    input,
  );
  return data;
}

export async function setReferralCampaignStatus(
  id: number,
  input: { expected_version: number; status: AdminReferralCampaignStatus; note?: string },
): Promise<AdminReferralCampaign> {
  const { data } = await apiClient.post<AdminReferralCampaign>(
    `/admin/affiliates/campaigns/${id}/status`,
    input,
  );
  return data;
}

export async function reviewReferralCampaign(
  id: number,
  input: {
    expected_version: number;
    review_type: AdminReferralReviewType;
    decision: AdminReferralReviewDecision;
    note?: string;
  },
): Promise<AdminReferralCampaign> {
  const { data } = await apiClient.post<AdminReferralCampaign>(
    `/admin/affiliates/campaigns/${id}/reviews`,
    input,
  );
  return data;
}

export async function listReferralCampaignParticipants(
  id: number,
  params: { page?: number; page_size?: number; search?: string } = {},
): Promise<AdminReferralPage<AdminReferralCampaignParticipant>> {
  const { data } = await apiClient.get<AdminReferralPage<AdminReferralCampaignParticipant>>(
    `/admin/affiliates/campaigns/${id}/participants`,
    { params },
  );
  return data;
}

export async function listReferralCampaignInvites(
  id: number,
  params: { page?: number; page_size?: number; search?: string; status?: string } = {},
): Promise<AdminReferralPage<AdminReferralCampaignInvite>> {
  const { data } = await apiClient.get<AdminReferralPage<AdminReferralCampaignInvite>>(
    `/admin/affiliates/campaigns/${id}/invites`,
    { params },
  );
  return data;
}

export async function listReferralCampaignRewards(
  id: number,
  params: { page?: number; page_size?: number; search?: string; status?: string } = {},
): Promise<AdminReferralPage<AdminReferralCampaignReward>> {
  const { data } = await apiClient.get<AdminReferralPage<AdminReferralCampaignReward>>(
    `/admin/affiliates/campaigns/${id}/rewards`,
    { params },
  );
  return data;
}

export async function resolveReferralRewardDebt(
  campaignID: number,
  rewardID: number,
  input: { expected_version: number; decision: "recovered" | "waived"; note: string },
): Promise<AdminReferralCampaignReward> {
  const { data } = await apiClient.post<AdminReferralCampaignReward>(
    `/admin/affiliates/campaigns/${campaignID}/rewards/${rewardID}/resolve`,
    input,
  );
  return data;
}

export async function getAppAnalytics(
  params: { period?: string; version?: string; channel?: string } = {},
): Promise<AdminAppAnalytics> {
  const { data } = await apiClient.get<AdminAppAnalytics>(
    "/admin/play/app-analytics",
    { params },
  );
  return data;
}

export async function listQuizQuestions(
  params: AdminQuizQuestionListParams = {},
): Promise<AdminQuizQuestionList> {
  const { data } = await apiClient.get<AdminQuizQuestionList>(
    "/admin/play/quiz/questions",
    { params },
  );
  return data;
}

export async function createQuizQuestion(
  input: AdminQuizQuestionInput,
): Promise<AdminQuizQuestion> {
  const { data } = await apiClient.post<AdminQuizQuestion>(
    "/admin/play/quiz/questions",
    input,
  );
  return data;
}

export async function updateQuizQuestion(
  id: number,
  input: AdminQuizQuestionInput,
): Promise<AdminQuizQuestion> {
  const { data } = await apiClient.put<AdminQuizQuestion>(
    `/admin/play/quiz/questions/${id}`,
    input,
  );
  return data;
}

export async function deleteQuizQuestion(id: number): Promise<void> {
  await apiClient.delete(`/admin/play/quiz/questions/${id}`);
}

export const adminPlayAPI = {
  getBlindboxPool,
  updateBlindboxPool,
  getTeamRewardSettings,
  updateTeamRewardSettings,
  listTeamRewardSettlements,
  retryTeamRewardSettlement,
  getSummary,
  listCampaigns,
  createCampaign,
  updateCampaign,
  deleteCampaign,
  getArenaLeaderboard,
  getArenaRewardSettings,
  updateArenaRewardSettings,
  listTeams,
  getTeam,
  getTeamSettlements,
  listTeamMemberCandidates,
  repairTeamMember,
  listTeamEvents,
  listMobileFeedback,
  getMobileFeedback,
  updateMobileFeedback,
  getMembershipOverview,
  listMembershipUsers,
  getMembershipUser,
  getVIPConfig,
  previewVIPConfig,
  publishVIPConfig,
  getInviteGrowthOverview,
  listReferralCampaigns,
  getReferralCampaign,
  createReferralCampaign,
  updateReferralCampaign,
  setReferralCampaignStatus,
  reviewReferralCampaign,
  listReferralCampaignParticipants,
  listReferralCampaignInvites,
  listReferralCampaignRewards,
  resolveReferralRewardDebt,
  getAppAnalytics,
  listQuizQuestions,
  createQuizQuestion,
  updateQuizQuestion,
  deleteQuizQuestion,
};

export default adminPlayAPI;
