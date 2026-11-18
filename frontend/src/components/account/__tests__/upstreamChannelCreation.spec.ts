import { expect, it, vi } from 'vitest'
import { useUpstreamChannelCreation } from '../upstreamChannelCreation'
import { bindChannelAccounts, readChannelBindings } from '@/api/admin/upstreamChannels'

vi.mock('@/api/admin/upstreamChannels', () => ({ bindChannelAccounts: vi.fn(), readChannelBindings: vi.fn() }))

const row = (channel_id = 0) => ({ account_id: 8, account_name: 'imported', deleted: false, channel_id, revision: 1 })
const selection = () => ({ channelID: 3, confirmed: true })
const translate = (key: string) => key

it('resumes assigning a created account after a failed binding without creating it twice', async () => {
  const assign = useUpstreamChannelCreation(selection, translate)
  const create = vi.fn().mockResolvedValue({ id: 8 })
  vi.mocked(readChannelBindings).mockReset().mockResolvedValueOnce([row()]).mockResolvedValueOnce([row()]).mockResolvedValueOnce([row(3)])
  vi.mocked(bindChannelAccounts).mockReset().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce([row(3)])
  const submit = () => assign.run('pat', { name: 'imported' }, create, ({ id }) => [id])

  await expect(submit()).rejects.toThrow('offline')
  await expect(submit()).resolves.toEqual({ id: 8 })
  expect(create).toHaveBeenCalledTimes(1)
  expect(bindChannelAccounts).toHaveBeenLastCalledWith(3, [row()])
})

it('requires confirmation and never reassigns an account bound to another channel', async () => {
  const create = vi.fn().mockResolvedValue({ id: 8 })
  const assign = useUpstreamChannelCreation(() => ({ channelID: 3, confirmed: false }), translate)
  await expect(assign.run('pat', {}, create, ({ id }) => [id])).rejects.toThrow('upstreamEntities.confirmCreate')
  expect(create).not.toHaveBeenCalled()

  vi.mocked(readChannelBindings).mockReset().mockResolvedValue([row(4)])
  vi.mocked(bindChannelAccounts).mockClear()
  const confirmed = useUpstreamChannelCreation(selection, translate)
  await expect(confirmed.run('pat', {}, create, ({ id }) => [id])).rejects.toThrow('upstreamEntities.createReadbackFailed')
  expect(bindChannelAccounts).not.toHaveBeenCalled()
})
