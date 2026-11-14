import { beforeEach, describe, expect, it, vi } from 'vitest'
import { schedulableTimersAPI } from '../schedulableTimers'

const { get, post, remove } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), remove: vi.fn() }))
vi.mock('../../client', () => ({ apiClient: { get, post, delete: remove } }))
beforeEach(() => { vi.resetAllMocks() })

describe('account scheduling timer HTTP contract', () => {
  it('unwraps administrator snapshots and concrete execution previews', async () => {
    const snapshot = { tasks: [], history: [], server_now: '2026-11-14T00:00:00Z', poll_seconds: 30, open_grace_seconds: 900, max_delay_minutes: 527040 }
    get.mockResolvedValue({ data: snapshot })
    expect(await schedulableTimersAPI.list(7)).toBe(snapshot)
    expect(get).toHaveBeenCalledWith('/admin/accounts/7/schedulable-timers')
    const input = { mode: 'daily' as const, time: '08:00', timezone: 'Asia/Shanghai' }
    const preview = { execute_at: '2026-11-15T00:00:00Z', timezone: input.timezone, local_date: '2026-11-15', local_time: input.time, offset: '+08:00', server_now: snapshot.server_now }
    post.mockResolvedValue({ data: preview })
    expect(await schedulableTimersAPI.preview(7, input)).toBe(preview)
    expect(post).toHaveBeenCalledWith('/admin/accounts/7/schedulable-timers/preview', input)
  })

  it('preserves one-shot confirmation and compare-and-set versions', async () => {
    const input = { mode: 'absolute' as const, action: 'close' as const, hours: 0, minutes: 0, date: '2026-11-15', time: '01:00', timezone: 'Asia/Shanghai', expected_version: 3, confirmed_at: '2026-11-14T17:00:00Z' }
    const task = { action: 'close', version: 4 }
    post.mockResolvedValue({ data: task })
    expect(await schedulableTimersAPI.save(7, input)).toBe(task)
    expect(post).toHaveBeenCalledWith('/admin/accounts/7/schedulable-timers', input)
  })

  it('saves both daily actions atomically and cancels only the selected version', async () => {
    const input = { open_time: '08:00', close_time: '01:00', timezone: 'Asia/Shanghai', expected_open_version: 3, expected_close_version: 5 }
    post.mockResolvedValue({ data: { tasks: [] } })
    expect(await schedulableTimersAPI.saveDailyWindow(7, input)).toEqual({ tasks: [] })
    expect(post).toHaveBeenCalledWith('/admin/accounts/7/schedulable-timers/daily-window', input)
    await schedulableTimersAPI.cancel(7, 'close', 5)
    expect(remove).toHaveBeenCalledWith('/admin/accounts/7/schedulable-timers/close', { params: { version: 5 } })
  })
})
