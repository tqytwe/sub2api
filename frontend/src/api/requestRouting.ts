// PR359 openai_compatible_http_text_v1 contract. Keep observations distinct from vetoes.
export const routingCoverageV1 = 'openai_compatible_http_text_v1'
export const routingStatuses = ['complete', 'truncated', 'pending', 'incomplete', 'not_recorded', 'unsupported_coverage'] as const
export type RoutingStatus = typeof routingStatuses[number]
export interface RoutingEvent {
  selection_no: number
  stage: string
  reason: string
  account_id?: number
  value: number
  at: string
}
export interface RoutingEvidence {
  request_id: string
  coverage_version: string
  status: RoutingStatus
  selection_count: number
  event_count: number
  veto_count: number
  omitted_count: number
  incomplete: boolean
  source_population_complete: boolean
  events: RoutingEvent[]
}
const vetoReasonsV1 = new Set([
  'grok_team_model_rate_limit', 'grok_model_quota_block', 'grok_free_quota_soft_gate',
  'quota_auto_reset_pending_5h', 'quota_auto_reset_pending_7d', 'quota_auto_reset_credit_check_5h', 'quota_auto_reset_credit_check_7d',
  'profit_threshold', 'profit_invalid_account_rate', 'account_nil', 'account_missing', 'account_read_failed',
  'account_model_not_owned', 'platform_mismatch', 'not_schedulable', 'model_rate_limited', 'quota_auto_pause',
  'quota_auto_pause_5h', 'quota_auto_pause_7d', 'quota_auto_pause_retry_after', 'quota_auto_pause_requests', 'quota_auto_pause_tokens',
  'model_not_supported', 'model_not_allowed_in_group', 'model_not_supported_in_group', 'responses_tools_protocol_mismatch',
  'capability_mismatch', 'compact_unsupported', 'excluded', 'runtime_blocked', 'privacy_not_set', 'transport_incompatible',
  'shadow_parent_unhealthy', 'channel_upstream_restricted', 'channel_restricted', 'channel_pricing_restricted',
  'proxy_stream_quarantined', 'group_mismatch', 'sticky_unhealthy', 'sticky_escape_ttft', 'sticky_escape_error_rate',
  'known_full', 'acquire_busy', 'acquire_failed', 'probe_budget_exhausted', 'db_recheck_budget_exhausted', 'wait_queue_full',
  'wait_failed', 'wait_plan_missing', 'selection_missing', 'previous_response_http_incompatible', 'scheduling_threshold',
])
export const isRoutingVeto = (version: string, event: RoutingEvent) => version === routingCoverageV1 && vetoReasonsV1.has(event.reason)
const count = (value: unknown): value is number => typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
const code = (value: unknown): value is string => typeof value === 'string' && /^[a-z][a-z0-9_]{0,95}$/.test(value)
const object = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value)
export function parseRoutingEvidence(value: unknown, requestID: string): RoutingEvidence {
  if (!object(value) || value.request_id !== requestID ||
    !(value.coverage_version === '' || code(value.coverage_version)) ||
    !routingStatuses.includes(value.status as RoutingStatus) ||
    !['selection_count', 'event_count', 'veto_count', 'omitted_count'].every(key => count(value[key])) ||
    (value.event_count as number) > 128 || typeof value.incomplete !== 'boolean' || typeof value.source_population_complete !== 'boolean' ||
    !Array.isArray(value.events) || value.events.length > 128 || !value.events.every(event =>
      object(event) && count(event.selection_no) && event.selection_no > 0 && code(event.stage) && code(event.reason) &&
      (event.account_id === undefined || (count(event.account_id) && event.account_id > 0)) && count(event.value) &&
      typeof event.at === 'string' && event.at.length <= 64 && Number.isFinite(Date.parse(event.at)))) {
    throw new Error('Unsupported routing evidence response')
  }
  // Do not require event_count === events.length: pending/lost batches are valid evidence gaps.
  return value as unknown as RoutingEvidence
}
