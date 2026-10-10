import { describe, expect, it } from 'vitest'
import { buildGroupAllowedModelsPayload, groupAllowedModelsFromAccount, groupAllowedModelsChanged, groupAllowedModelsError } from '../groupAllowedModels'

describe('account group policy replacement contract', () => {
  it('copies detail bindings without mutating the server response', () => {
    const account = { account_groups: [{ group_id: 1, allowed_models: ['gpt-5.5'] }, { group_id: 2, allowed_models: null }] }
    const result = groupAllowedModelsFromAccount(account)
    result[1].push('gpt-5.3-*')
    expect(account.account_groups[0].allowed_models).toEqual(['gpt-5.5'])
    expect(result[2]).toBeUndefined()
  })
  it('submits every surviving policy, normalizes lists, and drops removed groups', () => {
    expect(buildGroupAllowedModelsPayload([1, 2, 3], { 1: [' gpt-5.5 ', 'gpt-5.5', ''], 2: ['gpt-5.3-*'], 4: ['removed'] })).toEqual({ 1: ['gpt-5.5'], 2: ['gpt-5.3-*'] })
  })
  it('does not overwrite policies on unrelated saves or membership-only changes', () => {
    expect(groupAllowedModelsChanged([1, 2], { 1: ['gpt-5.5'] }, { 1: ['gpt-5.5'] })).toBe(false)
    expect(groupAllowedModelsChanged([2], { 1: ['removed'] }, { 1: ['removed'] })).toBe(false)
  })
  it('clearing one group retains the other policy; clearing all sends {}', () => {
    expect(groupAllowedModelsChanged([1, 2], { 1: [], 2: ['keep'] }, { 1: ['old'], 2: ['keep'] })).toBe(true)
    expect(buildGroupAllowedModelsPayload([1, 2], { 1: [], 2: ['keep'] })).toEqual({ 2: ['keep'] })
    expect(buildGroupAllowedModelsPayload([1], { 1: [] })).toEqual({})
  })
  it('validates the backend byte and normalized count bounds', () => {
    expect(groupAllowedModelsError(['中'.repeat(67)])).toBe('nameTooLong')
    expect(groupAllowedModelsError(Array.from({ length: 501 }, (_, i) => `model-${i}`))).toBe('tooMany')
    expect(groupAllowedModelsError(Array(501).fill('gpt-*'))).toBeNull()
  })
})
