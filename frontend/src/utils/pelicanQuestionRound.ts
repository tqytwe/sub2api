import { ref } from 'vue'
import { apiClient } from '@/api/client'
import type { PelicanTestConfig } from '@/types'

// One tab shares a prepared question across accounts for comparison. It is not persisted.
export const pelicanQuestionRound = ref<PelicanTestConfig | null>(null)
let owner: number | null = null
let requestSequence = 0

export async function preparePelicanQuestionRound(userId: number | null, signal?: AbortSignal) {
  const request = ++requestSequence
  const { data } = await apiClient.get<PelicanTestConfig>('/admin/accounts/pelican-question', { signal })
  if (signal?.aborted || request !== requestSequence) throw new DOMException('Aborted', 'AbortError')
  if (!data.pelican_question || !data.actual_prompt) throw new Error('Invalid question snapshot')
  owner = userId
  pelicanQuestionRound.value = data
  return data
}

export function currentPelicanQuestionRound(userId: number | null) {
  return owner === userId ? pelicanQuestionRound.value : null
}
