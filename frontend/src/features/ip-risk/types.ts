export type RiskLevel = 'low' | 'medium' | 'high' | 'severe' | 'critical'
export type RiskCaseStatus = 'open' | 'observing' | 'processing' | 'resolved' | 'ignored'
export type EvidenceConfidence = 'exact' | 'inferred' | 'mixed'
export type RiskSignalCode =
  | 'registration_10m'
  | 'registration_1h'
  | 'registration_24h'
  | 'shared_ua_3'
  | 'shared_ua_5'
  | 'email_pattern'
  | 'shared_api_ip'
  | 'rapid_key_or_gift'
  | 'shared_signup_code'
  | 'trusted_account'

export type RiskActionType =
  | 'observe'
  | 'mark_shared_network'
  | 'allowlist'
  | 'temporary_registration_block'
  | 'permanent_registration_block'
  | 'disable_api_keys'
  | 'disable_users'
  | 'resolve'
  | 'ignore'
  | 'rollback'

export type IPPolicyMode = 'allowlist' | 'observe' | 'shared_network' | 'block_registration'

export interface RiskSignal {
  code: RiskSignalCode
  family: string
  score: number
  count?: number
}

export interface RiskEvidence {
  primary_ip: string
  primary_network: string
  primary_ip_registration_count: number
  registration_count_10m: number
  registration_count_1h: number
  registration_count_24h: number
  exact_registration_count: number
  max_shared_ua_count: number
  email_pattern_account_count: number
  shared_api_ip_user_count: number
  rapid_key_or_gift_user_count: number
  shared_signup_code_count: number
  trusted_account_count: number
  all_key_evidence_exact: boolean
  allowlisted: boolean
  known_shared_network: boolean
}

export interface RiskCase {
  id: number
  primary_ip: string
  primary_network: string
  score: number
  level: RiskLevel
  status: RiskCaseStatus
  evidence_confidence: EvidenceConfidence
  signals: RiskSignal[]
  related_user_count: number
  selected_user_count: number
  auto_block_eligible: boolean
  first_detected_at: string
  last_detected_at: string
  version: number
}

export interface RiskRelatedKey {
  id: number
  name: string
  status: string
  created_at: string
  last_used_at?: string
}

export type RiskUserRelation = 'suspected_new' | 'trusted_existing' | 'disabled'

export interface RiskRelatedUser {
  user_id: number
  email: string
  username: string
  role: string
  status: string
  signup_source: string
  relation_type: RiskUserRelation
  evidence_confidence: EvidenceConfidence
  recommended_selected: boolean
  first_seen_at: string
  last_seen_at: string
  created_at: string
  total_recharged: number
  balance: number
  primary_ip_registrations: number
  shared_ua_account_count: number
  gift_granted: number
  gift_consumed: number
  gift_remaining: number
  api_key_count: number
  active_api_key_count: number
  api_keys: RiskRelatedKey[]
  evidence: Record<string, unknown>
}

export interface RiskTimelineEvent {
  id: number
  event_type: string
  user_id?: number
  ip_address: string
  confidence: EvidenceConfidence
  request_id?: string
  occurred_at: string
}

export interface RiskActionItem {
  id: number
  target_type: string
  target_id?: number
  target_ip?: string
  before_state: Record<string, unknown>
  after_state: Record<string, unknown>
  status: string
  error_message?: string
  rollback_status: string
}

export interface RiskActionRecord {
  id: number
  case_id?: number
  action_type: RiskActionType
  status: string
  actor_type: 'admin' | 'system'
  actor_user_id?: number
  reason: string
  rollback_status: string
  rollback_of_action_id?: number
  result: Record<string, unknown>
  items?: RiskActionItem[]
  created_at: string
  completed_at?: string
}

export interface RiskCaseDetail {
  case: RiskCase
  evidence: RiskEvidence
  recommended_actions: string[]
  users: RiskRelatedUser[]
  timeline: RiskTimelineEvent[]
  actions: RiskActionRecord[]
}

export interface RiskActionInput {
  action_type: RiskActionType
  user_ids: number[]
  api_key_ids: number[]
  duration_minutes: number
  reason: string
  preview_token?: string
}

export interface RiskActionPreview {
  case_id: number
  case_version: number
  action_type: RiskActionType
  user_ids: number[]
  api_key_ids: number[]
  user_count: number
  api_key_count: number
  already_disabled: number
  protected_users: number[]
  trusted_users: number[]
  inferred_users: number[]
  duration_minutes: number
  requires_step_up: boolean
  confirmation_token: string
  expires_at: string
  state_digest: string
}

export interface IPRiskOverview {
  open_cases: number
  critical_cases: number
  blocked_policies: number
  review_users: number
  last_detected_at?: string
  latest_scan_status?: string
}

export interface RiskScan {
  id: number
  scan_type: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'canceled'
  requested_by?: number
  range_start: string
  range_end: string
  progress: number
  candidate_count: number
  case_count: number
  inferred_event_count: number
  error_message?: string
  started_at?: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface RiskRuntime {
  enabled: boolean
  started: boolean
  shadow_mode: boolean
  auto_block_enabled: boolean
  historical_backfill_enabled: boolean
  degraded: boolean
  degraded_reason?: string
  evaluation_queue_size: number
  evaluation_queue_capacity: number
  last_evaluation_at?: string
  last_scan?: RiskScan
  last_error?: string
}

export interface RiskConfig {
  registration_10m_threshold: number
  registration_10m_score: number
  registration_1h_threshold: number
  registration_1h_score: number
  registration_24h_threshold: number
  registration_24h_score: number
  shared_ua_3_threshold: number
  shared_ua_3_score: number
  shared_ua_5_threshold: number
  shared_ua_5_score: number
  email_pattern_threshold: number
  email_pattern_score: number
  shared_api_ip_threshold: number
  shared_api_ip_score: number
  rapid_behavior_threshold: number
  rapid_behavior_score: number
  shared_signup_code_threshold: number
  shared_signup_code_score: number
  trusted_account_score: number
  auto_block_score: number
  auto_block_min_registrations: number
  auto_block_duration_minutes: number
  auto_block_enabled: boolean
  historical_backfill_enabled: boolean
  event_retention_days: number
  case_retention_days: number
}

export interface IPRiskPolicy {
  id: number
  mode: IPPolicyMode
  ip_network?: string
  exact_ip?: string
  reason: string
  enabled: boolean
  expires_at?: string
  created_by?: number
  source_action_id?: number
  created_at: string
  updated_at: string
}

export interface IPRiskPolicyInput {
  mode: IPPolicyMode
  ip_network: string
  exact_ip: string
  reason: string
  enabled: boolean
  expires_at?: string | null
}

export interface RiskCaseQuery {
  page: number
  page_size: number
  level?: string
  status?: string
  signal?: string
  search?: string
  range_start?: string
  range_end?: string
}
