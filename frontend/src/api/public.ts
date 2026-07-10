/**
 * Unauthenticated public API endpoints.
 */

import { apiClient } from './client'
import type { UserAvailableChannel } from './channels'

export async function getPublicModels(options?: { signal?: AbortSignal }): Promise<UserAvailableChannel[]> {
  const { data } = await apiClient.get<UserAvailableChannel[]>('/public/models', {
    signal: options?.signal,
  })
  return data ?? []
}

export const publicAPI = { getPublicModels }

export default publicAPI
