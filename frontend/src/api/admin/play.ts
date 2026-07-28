import { apiClient } from "../client";
import type {
  PlayArenaPeriod,
  PlayBlindboxPool,
  PlayCampaignRules,
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
  enabled: boolean;
  created_at: string;
}

export interface AdminPlayCampaignInput {
  name: string;
  start_at: string;
  end_at: string;
  rules: PlayCampaignRules;
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
  listTeams,
  getTeam,
  getTeamSettlements,
  listTeamMemberCandidates,
  repairTeamMember,
  listTeamEvents,
  listMobileFeedback,
  getMobileFeedback,
  updateMobileFeedback,
  listQuizQuestions,
  createQuizQuestion,
  updateQuizQuestion,
  deleteQuizQuestion,
};

export default adminPlayAPI;
