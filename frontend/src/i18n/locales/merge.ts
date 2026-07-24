export type LocaleNode = Record<string, unknown>

export function isLocaleNode(value: unknown): value is LocaleNode {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function mergeLocaleMessages<T extends LocaleNode>(base: T, updates: LocaleNode): T {
  const merged: LocaleNode = { ...base }
  for (const [key, value] of Object.entries(updates)) {
    const current = merged[key]
    merged[key] = isLocaleNode(current) && isLocaleNode(value)
      ? mergeLocaleMessages(current, value)
      : value
  }
  return merged as T
}
