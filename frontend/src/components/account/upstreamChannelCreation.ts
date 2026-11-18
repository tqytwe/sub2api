import { bindChannelAccounts, readChannelBindings } from '@/api/admin/upstreamChannels'

export function useUpstreamChannelCreation(
  selection: () => { channelID: number; confirmed: boolean },
  translate: (key: string) => string,
) {
  const pending = new Map<string, unknown>()

  async function run<T>(kind: string, payload: unknown, create: () => Promise<T>, accountIDs: (result: T) => number[]) {
    const { channelID, confirmed } = selection()
    if (!channelID) return create()
    if (!confirmed) throw new Error(translate('upstreamEntities.confirmCreate'))

    const key = JSON.stringify([kind, channelID, payload])
    const result = pending.has(key) ? pending.get(key) as T : await create()
    pending.set(key, result)
    const ids = accountIDs(result)
    const failedReadback = () => new Error(translate('upstreamEntities.createReadbackFailed'))
    for (let offset = 0; offset < ids.length; offset += 100) {
      const batch = ids.slice(offset, offset + 100)
      const hasAllAccounts = (bindings: { account_id: number; deleted: boolean }[]) =>
        batch.every(id => bindings.some(binding => binding.account_id === id && !binding.deleted)) && bindings.length === batch.length
      const current = await readChannelBindings(batch)
      if (!hasAllAccounts(current) || current.some(binding => binding.channel_id && binding.channel_id !== channelID)) {
        throw failedReadback()
      }
      const unbound = current.filter(binding => binding.channel_id === 0)
      if (unbound.length) {
        await bindChannelAccounts(channelID, unbound)
        const saved = await readChannelBindings(batch)
        if (!hasAllAccounts(saved) || saved.some(binding => binding.channel_id !== channelID)) throw failedReadback()
      }
    }
    pending.delete(key)
    return result
  }

  return { run, clear: () => pending.clear() }
}
