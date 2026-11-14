import { beforeEach, describe, expect, it, vi } from 'vitest'

const { put } = vi.hoisted(() => ({ put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { put } }))
import { apiKeysAPI } from '@/api/admin/apiKeys'
import type { CreateApiKeyRequest, UpdateApiKeyRequest } from '@/types'

type PolicyField = 'model_mapping' | 'reasoning_effort' | 'allow_long_context' | 'context_token_limit'
const userCreateHasNoPolicy: Extract<keyof CreateApiKeyRequest, PolicyField> extends never ? true : false = true
const userUpdateHasNoPolicy: Extract<keyof UpdateApiKeyRequest, PolicyField> extends never ? true : false = true

describe('administrator API key model policy', () => {
  beforeEach(() => put.mockReset())

  it('writes only the four policy fields through the existing admin endpoint and preserves explicit zero/false', async () => {
    const policy = { model_mapping: { 'public-model': 'target-model' }, reasoning_effort: 'high', allow_long_context: false, context_token_limit: 0 }
    const result = { api_key: { id: 17, ...policy }, auto_granted_group_access: false }
    put.mockResolvedValue({ data: result })
    expect(await apiKeysAPI.updateApiKeyModelPolicy(17, policy)).toEqual(result)
    expect(put).toHaveBeenCalledTimes(1)
    expect(put).toHaveBeenCalledWith('/admin/api-keys/17', policy)
    expect(userCreateHasNoPolicy).toBe(true)
    expect(userUpdateHasNoPolicy).toBe(true)
  })

  it('retains the existing group-update contract', async () => {
    put.mockResolvedValue({ data: { api_key: { id: 17 }, auto_granted_group_access: false } })
    await apiKeysAPI.updateApiKeyGroup(17, null)
    expect(put).toHaveBeenCalledTimes(1)
    expect(put).toHaveBeenCalledWith('/admin/api-keys/17', { group_id: 0 })
  })
})
