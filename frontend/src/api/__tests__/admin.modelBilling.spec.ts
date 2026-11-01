import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put } = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get, put } }))

import { getModelBillingConfig, saveModelBillingConfig } from '../admin/modelBilling'

describe('admin model billing config API', () => {
  beforeEach(() => vi.clearAllMocks())

  it('reads the dedicated config and opaque revision', async () => {
    const snapshot = { config: { enabled: false, rules: null }, revision: '' }
    get.mockResolvedValue({ data: snapshot })
    await expect(getModelBillingConfig()).resolves.toEqual(snapshot)
    expect(get).toHaveBeenCalledWith('/admin/model-billing/config')
  })

  it('saves the explicit group scope together with the loaded revision', async () => {
    const snapshot = { config: { enabled: true, rules: [{ model: 'gpt-6*', multiplier: 2.5, group_ids: [7] }] }, revision: 'r1' }
    put.mockResolvedValue({ data: { ...snapshot, revision: 'r2' } })
    await expect(saveModelBillingConfig(snapshot)).resolves.toEqual({ ...snapshot, revision: 'r2' })
    expect(put).toHaveBeenCalledWith('/admin/model-billing/config', snapshot)
  })

  it('propagates a conflict without retrying or replacing the revision', async () => {
    const conflict = { status: 409, code: 'MODEL_BILLING_REVISION_CONFLICT' }
    put.mockRejectedValue(conflict)
    await expect(saveModelBillingConfig({ config: { enabled: false, rules: [] }, revision: 'stale' })).rejects.toBe(conflict)
    expect(put).toHaveBeenCalledTimes(1)
  })
})
