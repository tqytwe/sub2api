export interface CouponAudience {
  mode: 'all' | 'users' | 'filtered'
  user_ids?: number[]
  search?: string
  group_name?: string
  api_key_group_id?: number
  vip_tier?: number
  attributes?: Record<number, string>
}
export interface CouponClaimPolicy {
  template_id: number
  code: string
  enabled: boolean
  starts_at: string | null
  ends_at: string | null
  claim_limit: number
  per_user_limit: number
  audience: CouponAudience
  claimed_count: number
  issued_count: number
  used_count: number
  manual_count: number
  version: number
}
export interface CouponRecipient {
  user_id: number
  reason: string
}
export interface CouponCampaignPreview {
  token: string
  template_id: number
  template_version: number
  policy_version: number
  audience: CouponAudience
  recipients: number[]
  skipped: CouponRecipient[]
  requested_count: number
  remaining_stock: number | null
  expires_at: string
}
export interface CouponCampaignResult {
  batch_id: number
  issued_count: number
  skipped: CouponRecipient[]
}
export interface CouponSummary {
  issued: number
  available: number
  used: number
  amounts: Array<{ currency: string; paid: number; discount: number; refunded: number; net_paid: number }>
}
export interface CouponEvent {
  id: number
  event_type: string
  order_id: number | null
  actor_type: string
  created_at: string
  metadata: Record<string, unknown>
}
