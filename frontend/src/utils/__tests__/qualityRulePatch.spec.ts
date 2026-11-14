import { describe, expect, it } from 'vitest'
import type { PelicanTestConfig, ScheduledTestPlan } from '@/types'
import { buildQualityRulePatch, copyQualityRuleConfig, qualityActionPayload, qualityRuleFieldValue } from '../qualityRulePatch'

const config = (): PelicanTestConfig => ({ rule_name: 'Synthetic rule', question_kind: 'text', prompt: 'say 21', model_ids: ['a', 'b'], parallel_count: 2, quality: { action: 'disable_scheduling', expected_answer: '21', auto_restore: true, judge: { kind: 'expected' }, trigger_on_upstream_5xx: true, retest_rest_minutes: 90, oauth_401_max_rounds: 6 } })
const plan = (): ScheduledTestPlan => ({ id: 1, account_id: 7, model_id: 'a', cron_expression: '*/30 * * * *', enabled: true, max_results: 100, auto_recover: false, last_run_at: null, next_run_at: null, created_at: '', updated_at: 'version-1', pelican_config: config() })
const draft = () => ({ model_id: 'c', cron_expression: '0 * * * *', enabled: false, max_results: 150, pelican_config: { ...config(), model_ids: ['c', 'd'], quality: { ...config().quality, trigger_on_upstream_5xx: false, retest_rest_minutes: 120 } } })

describe('quality rule runtime input contracts', () => {
  it('copies reusable 5xx/rest/multimodel inputs without result state or source mutation', () => {
    const source = config()
    Object.assign(source, { response_text: 'not reusable', quality_state: { owner: 99 } })
    const copy = copyQualityRuleConfig(source)
    expect(copy.quality).toMatchObject({ trigger_on_upstream_5xx: true, retest_rest_minutes: 90, oauth_401_max_rounds: 6 })
    expect(copy).not.toHaveProperty('response_text')
    expect(copy).not.toHaveProperty('quality_state')
    copy.model_ids!.push('other')
    expect(source.model_ids).toEqual(['a', 'b'])
  })

  it.each([0, 1, 10080])('accepts %s minutes for an action', minutes => {
    const result = qualityActionPayload({ ...config().quality!, retest_rest_minutes: minutes })
    expect(result.retest_rest_minutes ?? 0).toBe(minutes)
    if (!minutes) expect(result).not.toHaveProperty('retest_rest_minutes')
  })

  it.each([-1, 1.5, 10081, NaN, Infinity])('rejects invalid action rest %s', value => {
    expect(() => qualityActionPayload({ ...config().quality!, retest_rest_minutes: value })).toThrow('admin.qualityOps.invalidRetestRest')
  })

  it('clears rest for observation and never turns a 5xx trigger off implicitly', () => {
    const value = qualityActionPayload({ ...config().quality!, action: 'observe_only', retest_rest_minutes: 90 })
    expect(value.retest_rest_minutes).toBeUndefined()
    expect(value.trigger_on_upstream_5xx).toBe(true)
    expect(config().quality!.retest_rest_minutes).toBe(90)
  })

  it('owns rest under the action checkbox and trigger under its own checkbox', () => {
    const source = draft(), target = plan()
    expect(buildQualityRulePatch(target, source, ['enabled'])).toEqual({ enabled: false, expected_updated_at: 'version-1' })
    const restore = buildQualityRulePatch(target, source, ['restore'])
    expect(restore.pelican_config!.quality).toMatchObject({ retest_rest_minutes: 90, trigger_on_upstream_5xx: true })
    const action = buildQualityRulePatch(target, source, ['action'])
    expect(action.pelican_config!.quality).toMatchObject({ retest_rest_minutes: 120, trigger_on_upstream_5xx: true, oauth_401_max_rounds: 6 })
    const trigger = buildQualityRulePatch(target, source, ['trigger'])
    expect(trigger.pelican_config!.quality).toMatchObject({ retest_rest_minutes: 90, trigger_on_upstream_5xx: false })
    expect(qualityRuleFieldValue(target, 'action')).not.toEqual(qualityRuleFieldValue({ ...target, pelican_config: source.pelican_config }, 'action'))
  })

  it('preserves each bulk target own model and policy state and applies explicit observation cleanup', () => {
    const target = plan()
    target.pelican_config!.model_ids = ['target-model']
    target.pelican_config!.quality!.retest_rest_minutes = 20
    const source = draft()
    source.pelican_config.quality.action = 'observe_only'
    const result = buildQualityRulePatch(target, source, ['action'])
    expect(result.pelican_config!.model_ids).toEqual(['target-model'])
    expect(result.pelican_config!.quality!.retest_rest_minutes).toBeUndefined()
    expect(result.pelican_config!.quality!.trigger_on_upstream_5xx).toBe(true)
    expect(target.pelican_config!.quality!.retest_rest_minutes).toBe(20)
  })
})
