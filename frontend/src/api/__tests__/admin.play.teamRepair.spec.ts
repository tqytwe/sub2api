import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { post },
}))

import { repairTeamMember, type AdminTeamMemberRepairInput } from '@/api/admin/play'

const input: AdminTeamMemberRepairInput = {
  user_id: 42,
  operation: 'add',
  reason: 'repair missing team membership',
}

describe('admin Play team repair API', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    localStorage.setItem('auth_user', JSON.stringify({ id: 7 }))
    post.mockReset()
    post.mockResolvedValue({
      data: {
        status: 'added',
        team_id: 9,
        user_id: 42,
        effective_at: '2026-07-20T08:00:00+08:00',
        warnings: [],
      },
    })
    vi.spyOn(globalThis.crypto, 'randomUUID')
      .mockReturnValue('11111111-1111-4111-8111-111111111111')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('always sends an administrator-scoped Idempotency-Key', async () => {
    await repairTeamMember(9, input)

    expect(post).toHaveBeenCalledWith('/admin/play/teams/9/members', input, {
      headers: {
        'Idempotency-Key': 'play-team-repair-7-9-42-11111111-1111-4111-8111-111111111111',
      },
    })
    expect(sessionStorage.length).toBe(0)
  })

  it('reuses the same key after an ambiguous failure and clears it after success', async () => {
    post.mockRejectedValueOnce(new Error('network timeout'))
    await expect(repairTeamMember(9, input)).rejects.toThrow('network timeout')
    const firstKey = post.mock.calls[0][2].headers['Idempotency-Key']
    expect(sessionStorage.length).toBe(1)

    await repairTeamMember(9, input)

    expect(post.mock.calls[1][2].headers['Idempotency-Key']).toBe(firstKey)
    expect(sessionStorage.length).toBe(0)
  })

  it('does not reuse a repair key across administrators', async () => {
    post.mockRejectedValueOnce(new Error('first admin timeout'))
    await expect(repairTeamMember(9, input)).rejects.toThrow('first admin timeout')
    const firstKey = post.mock.calls[0][2].headers['Idempotency-Key']

    localStorage.setItem('auth_user', JSON.stringify({ id: 8 }))
    vi.mocked(globalThis.crypto.randomUUID)
      .mockReturnValueOnce('22222222-2222-4222-8222-222222222222')
    await repairTeamMember(9, input)

    expect(post.mock.calls[1][2].headers['Idempotency-Key']).not.toBe(firstKey)
    expect(post.mock.calls[1][2].headers['Idempotency-Key']).toContain('play-team-repair-8-9-42-')
  })
})
