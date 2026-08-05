/** Normalize values coming from select controls and persisted filters. */
export function normalizeVipTier(value: unknown): number | undefined {
  if (value === null || value === undefined || value === '') {
    return undefined
  }

  const parsed = typeof value === 'number' ? value : Number(String(value).trim())
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > 99) {
    return undefined
  }
  return parsed
}
