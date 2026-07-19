export type VIPColorKey = 'neutral' | 'emerald' | 'sky' | 'indigo' | 'amber' | 'gold'

const allowedVIPColorKeys = new Set<VIPColorKey>(['neutral', 'emerald', 'sky', 'indigo', 'amber', 'gold'])

export function normalizeVIPColorKey(colorKey?: string | null): VIPColorKey {
  const normalized = String(colorKey || '').trim().toLowerCase()
  if (normalized === 'rose') return 'gold'
  return allowedVIPColorKeys.has(normalized as VIPColorKey)
    ? normalized as VIPColorKey
    : 'neutral'
}

export function vipTierBadgeClass(colorKey?: string | null): string {
  return `vip-tier-badge vip-tier-badge-${normalizeVIPColorKey(colorKey)}`
}
