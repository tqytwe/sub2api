import type { PelicanTestConfig, QualityPolicy, ScheduledTestPlan, UpdateScheduledTestPlanRequest } from '@/types'

export function qualityUsesRetestRest(action?: string) {
  return action === 'remove_groups' || action === 'disable_scheduling' || action === 'remove_models' || action === 'isolate_models'
}

// The same runtime policy is submitted by account rules, templates and presets.
export function qualityActionPayload(policy: QualityPolicy): QualityPolicy {
  const next = { ...policy, remove_group_ids: policy.remove_group_ids ? [...policy.remove_group_ids] : undefined }
  const rest = qualityUsesRetestRest(policy.action) ? policy.retest_rest_minutes ?? 0 : 0
  if (!Number.isInteger(rest) || rest < 0 || rest > 10080) throw new Error('admin.qualityOps.invalidRetestRest')
  if (rest > 0) next.retest_rest_minutes = rest
  else delete next.retest_rest_minutes
  return next
}

// Allowlist reusable input fields. Result snapshots and account ownership never
// belong to a newly created rule, template or preset.
export function copyQualityRuleConfig(source: PelicanTestConfig): PelicanTestConfig & { quality: QualityPolicy } {
  const policy = source.quality || { action: 'observe_only' }
  return {
    rule_name: source.rule_name,
    question_kind: source.question_kind,
    test_channel: source.test_channel,
    prompt: source.prompt,
    reasoning_effort: source.reasoning_effort,
    parallel_count: source.parallel_count,
    model_ids: source.model_ids ? [...source.model_ids] : undefined,
    quality: qualityActionPayload({
      action: policy.action,
      expected_answer: policy.expected_answer,
      auto_restore: policy.auto_restore,
      remove_group_ids: policy.remove_group_ids,
      trigger_on_upstream_5xx: policy.trigger_on_upstream_5xx ?? false,
      retest_rest_minutes: policy.retest_rest_minutes,
      oauth_401_max_rounds: policy.oauth_401_max_rounds,
      judge: policy.judge ? { kind: policy.judge.kind, group_id: policy.judge.group_id, model_id: policy.judge.model_id, prompt: policy.judge.prompt } : undefined,
    }),
  }
}

export const qualityRuleFields = ['model', 'schedule', 'test', 'action', 'restore', 'retention', 'trigger', 'enabled'] as const
export type QualityRuleField = typeof qualityRuleFields[number]
export type QualityRuleDraft = {
  model_id: string
  cron_expression: string
  max_results?: number
  enabled: boolean
  pelican_config: PelicanTestConfig & { quality: QualityPolicy }
}

export function qualityRuleFieldValue(rule: Pick<ScheduledTestPlan, 'model_id' | 'cron_expression' | 'enabled' | 'pelican_config'> & { max_results?: number }, field: QualityRuleField): unknown {
  const config = rule.pelican_config, quality = config?.quality
  switch (field) {
    case 'model': return [...new Set([rule.model_id, ...(config?.model_ids || [])])]
    case 'schedule': return rule.cron_expression
    case 'enabled': return rule.enabled
    case 'retention': return rule.max_results
    case 'trigger': return quality?.trigger_on_upstream_5xx ?? false
    case 'action': return [quality?.action, quality?.remove_group_ids || [], quality?.retest_rest_minutes ?? 0]
    case 'restore': return quality?.auto_restore ?? false
    case 'test': return [config?.question_kind, config?.test_channel || 'account', config?.prompt, config?.reasoning_effort, config?.parallel_count, quality?.expected_answer, quality?.judge]
  }
}

// The server replaces nested config. Start from each target's own current
// config, merge explicitly selected fields, and use its existing CAS version.
export function buildQualityRulePatch(plan: ScheduledTestPlan, draft: QualityRuleDraft, fields: readonly QualityRuleField[]): UpdateScheduledTestPlanRequest {
  if (!fields.length) throw new Error('admin.qualityOps.selectBulkFields')
  const patch: UpdateScheduledTestPlanRequest = { expected_updated_at: plan.updated_at }
  if (fields.includes('model')) {
    if (!draft.model_id.trim()) throw new Error('admin.qualityOps.probeModelsInvalid')
    patch.model_id = draft.model_id.trim()
  }
  if (fields.includes('schedule')) {
    if (!draft.cron_expression.trim()) throw new Error('admin.qualityOps.scheduleRequired')
    patch.cron_expression = draft.cron_expression.trim()
  }
  if (fields.includes('retention')) {
    if (!Number.isInteger(draft.max_results) || draft.max_results! < 1 || draft.max_results! > 200) throw new Error('admin.qualityOps.invalidMaxResults')
    patch.max_results = draft.max_results
    if (!fields.includes('model') && !fields.includes('test') && patch.max_results! < (plan.pelican_config?.model_ids?.length || 1) * (plan.pelican_config?.parallel_count || 1)) throw new Error('admin.qualityOps.retentionTooSmall')
  }
  if (fields.includes('enabled')) patch.enabled = draft.enabled
  if (!fields.some(field => ['model', 'test', 'action', 'restore', 'trigger'].includes(field))) return patch
  if (!plan.pelican_config?.quality) throw new Error('admin.qualityOps.ruleConfigMissing')
  const config: PelicanTestConfig & { quality: QualityPolicy } = JSON.parse(JSON.stringify(plan.pelican_config))
  const source = draft.pelican_config
  if (fields.includes('model')) {
    config.model_ids = [...new Set([draft.model_id.trim(), ...(source.model_ids || []).map(model => model.trim())])]
    if (config.model_ids.some(model => !model || model.length > 100) || config.model_ids.length > 50) throw new Error('admin.qualityOps.probeModelsInvalid')
  }
  if (fields.includes('test')) {
    config.question_kind = source.question_kind
    config.test_channel = source.test_channel
    config.prompt = source.prompt
    config.reasoning_effort = source.reasoning_effort
    config.parallel_count = source.parallel_count
    config.quality.expected_answer = source.quality.expected_answer
    const judge = source.quality.judge
    config.quality.judge = judge ? { kind: judge.kind, group_id: judge.group_id, model_id: judge.model_id, prompt: judge.prompt } : undefined
  }
  if (fields.includes('trigger')) config.quality.trigger_on_upstream_5xx = source.quality.trigger_on_upstream_5xx ?? false
  if (fields.includes('action')) {
    config.quality = qualityActionPayload({ ...config.quality, action: source.quality.action, remove_group_ids: source.quality.remove_group_ids, retest_rest_minutes: source.quality.retest_rest_minutes })
    if (config.quality.action === 'isolate_models' && !config.quality.remove_group_ids?.length) throw new Error('admin.qualityOps.isolation.groupsRequired')
  }
  if (fields.includes('restore')) config.quality.auto_restore = source.quality.auto_restore
  if (config.quality.action === 'observe_only') delete config.quality.retest_rest_minutes
  const count = config.parallel_count || 1
  if (!Number.isInteger(count) || count < 1 || count > 32) throw new Error('admin.qualityOps.probeCountInvalid')
  const samples = (config.model_ids?.length || 1) * count
  if (samples > 100) throw new Error('admin.qualityOps.probeSamplesInvalid')
  if ((patch.max_results ?? plan.max_results) < samples) throw new Error('admin.qualityOps.retentionTooSmall')
  patch.pelican_config = config
  return patch
}
