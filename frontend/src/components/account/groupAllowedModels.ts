/** Policies are a full replacement snapshot, not a per-group PATCH. */
export type GroupAllowedModels = Record<number, string[]>

export function groupAllowedModelsFromAccount(account: { account_groups?: { group_id: number; allowed_models?: string[] | null }[] } | null | undefined): GroupAllowedModels {
  const limits: GroupAllowedModels = {}
  for (const binding of account?.account_groups ?? []) {
    if (binding.allowed_models?.length) limits[binding.group_id] = [...binding.allowed_models]
  }
  return limits
}

const normalize = (models: string[]) => [...new Set(models.map(model => model.trim()).filter(Boolean))]

/** Missing/empty entries mean unrestricted; include every surviving restriction. */
export function buildGroupAllowedModelsPayload(groupIds: number[], limits: GroupAllowedModels): GroupAllowedModels {
  const payload: GroupAllowedModels = {}
  for (const id of groupIds) {
    const models = normalize(limits[id] ?? [])
    if (models.length) payload[id] = models
  }
  return payload
}

export function groupAllowedModelsChanged(groupIds: number[], current: GroupAllowedModels, saved: GroupAllowedModels): boolean {
  return JSON.stringify(buildGroupAllowedModelsPayload(groupIds, current)) !== JSON.stringify(buildGroupAllowedModelsPayload(groupIds, saved))
}

export function groupAllowedModelsError(models: string[]): 'tooMany' | 'nameTooLong' | null {
  const normalized = normalize(models)
  if (normalized.length > 500) return 'tooMany'
  // The Go validator measures bytes, including multibyte model names.
  if (normalized.some(model => new TextEncoder().encode(model).length > 200)) return 'nameTooLong'
  return null
}
