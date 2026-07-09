/** Format affiliate rebate percent for marketing copy (supports fractional rates like 0.1%). */
export function formatAffiliatePct(pct: number): string {
  if (!Number.isFinite(pct) || pct <= 0) return '0'
  if (pct < 1) {
    const fixed = pct.toFixed(1)
    return fixed.endsWith('.0') ? fixed.slice(0, -2) : fixed
  }
  return String(Math.round(pct))
}
