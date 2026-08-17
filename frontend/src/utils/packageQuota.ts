import type { PackageEntitlement } from '@/types'

export type PackageQuotaDimension = 'request' | 'amount' | 'token'

export interface PackageQuotaRow {
  dimension: PackageQuotaDimension
  used: number
  limit: number
}

export function packageQuotaRows(entitlement: PackageEntitlement): PackageQuotaRow[] {
  const rows: PackageQuotaRow[] = []

  if (entitlement.request_limit != null) {
    rows.push({ dimension: 'request', used: entitlement.request_used, limit: entitlement.request_limit })
  }
  if (entitlement.amount_limit_usd != null) {
    rows.push({ dimension: 'amount', used: entitlement.amount_used_usd, limit: entitlement.amount_limit_usd })
  }
  if (entitlement.token_limit != null) {
    rows.push({ dimension: 'token', used: entitlement.token_used, limit: entitlement.token_limit })
  }

  return rows
}
