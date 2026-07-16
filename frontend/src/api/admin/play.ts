import { apiClient } from '../client'
import type { PlayBlindboxPool } from '../play'

export type { PlayBlindboxPool, PlayBlindboxPoolTier } from '../play'

export async function getBlindboxPool(): Promise<PlayBlindboxPool> {
  const { data } = await apiClient.get<PlayBlindboxPool>('/admin/play/blindbox/pool')
  return data
}

export async function updateBlindboxPool(pool: PlayBlindboxPool): Promise<PlayBlindboxPool> {
  const { data } = await apiClient.put<PlayBlindboxPool>('/admin/play/blindbox/pool', pool)
  return data
}

export const adminPlayAPI = {
  getBlindboxPool,
  updateBlindboxPool,
}

export default adminPlayAPI
