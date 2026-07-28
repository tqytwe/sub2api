import type { BasePaginationResponse } from './index'

export type CouponBenefitType = 'fixed_amount' | 'percentage'
export type CouponScope = 'balance' | 'subscription'
export type CouponValidityMode = 'relative_days' | 'end_of_day' | 'end_of_month' | 'fixed'
export type CouponTemplateStatus = 'draft' | 'active' | 'paused' | 'archived'
export type UserCouponStatus = 'available' | 'locked' | 'used' | 'expired' | 'voided'
export type CouponIssueSource = 'blindbox' | 'quiz' | 'checkin' | 'admin_batch' | 'manual' | 'compensation'
export type CouponRewardActivity = 'blindbox' | 'quiz' | 'checkin'
export type CouponRewardPoolStatus = 'draft' | 'published' | 'retired'

export interface CouponTermsSnapshot {
  template_id: number
  template_version: number
  template_key: string
  name: string
  description?: string
  benefit_type: CouponBenefitType
  benefit_value: number
  max_discount_amount?: number | null
  currency: string
  applicable_scopes: CouponScope[]
  minimum_order_amount: number
  eligible_plan_ids: number[]
  validity_mode: CouponValidityMode
  validity_days?: number
  rules?: Record<string, unknown>
}

export interface CouponTemplate extends Omit<CouponTermsSnapshot, 'template_id' | 'template_version'> {
  id: number
  key: string
  version: number
  status: CouponTemplateStatus
  fixed_expires_at?: string | null
  valid_from?: string | null
  total_issue_limit?: number | null
  issued_count: number
  created_at: string
  updated_at: string
}

export interface UserCoupon {
  id: number
  template_id: number
  template_name?: string
  user_id: number
  user_email?: string
  user_name?: string
  status: UserCouponStatus
  terms_snapshot: CouponTermsSnapshot
  source: CouponIssueSource
  source_ref?: string
  issue_batch_id?: number | null
  issued_at: string
  valid_from: string
  expires_at: string
  locked_order_id?: number | null
  locked_at?: string | null
  used_order_id?: number | null
  used_order_no?: string
  used_order_type?: string
  used_order_status?: string
  used_order_amount?: number
  used_order_pay_amount?: number
  used_order_discount_amount?: number
  used_order_currency?: string
  used_at?: string | null
  voided_at?: string | null
  void_reason?: string
  created_at: string
  updated_at: string
}

export interface CouponQuote {
  user_coupon_id: number
  template_id: number
  original_amount: number
  discount_amount: number
  payable_amount: number
  currency: string
}

export interface CouponEligibility {
  coupon: UserCoupon
  quote: CouponQuote
}

export interface CouponEligibleListResponse {
  items: CouponEligibility[]
}

export interface CouponEligibilityRequest {
  scope: CouponScope
  amount: number
  plan_id?: number
  currency?: string
}

export interface CouponPaymentQuoteRequest {
  coupon_id: number
  payment_type: string
  order_type: 'balance' | 'subscription'
  amount: number
  plan_id?: number
}

export interface CouponPaymentQuote {
  user_coupon_id: number
  template_id: number
  list_amount: number
  gateway_base_amount: number
  discount_amount: number
  fee_amount: number
  pay_amount: number
  payment_currency: string
  qualifying_recharge_amount: number
}

export type UserCouponPage = BasePaginationResponse<UserCoupon>

export interface CouponIssueBatch {
  id: number
  template_id: number
  template_name?: string
  source: CouponIssueSource
  requested_count: number
  issued_count: number
  failed_count: number
  status: 'completed' | 'failed'
  created_at: string
  completed_at?: string | null
}

export interface CouponRewardPoolEntry {
  id?: number
  pool_version_id?: number
  template_id: number
  template_name?: string
  weight_bp: number
  enabled: boolean
  starts_at?: string | null
  ends_at?: string | null
  stock_cap?: number | null
  issued_count?: number
  per_user_issue_limit?: number | null
  sort_order: number
}

export interface CouponRewardPoolVersion {
  id: number
  activity: CouponRewardActivity
  version: string
  status: CouponRewardPoolStatus
  coupon_weight_bp: number
  redeem_code_weight_bp?: number
  balance_weight_bp: number
  reward_config?: CouponRewardPoolConfig
  fallback_template_id: number
  entries: CouponRewardPoolEntry[]
  published_at?: string | null
  created_at: string
  updated_at: string
}

export interface CouponTemplateInput {
  key: string
  name: string
  description?: string
  status: CouponTemplateStatus
  benefit_type: CouponBenefitType
  benefit_value: number
  max_discount_amount?: number | null
  currency: string
  applicable_scopes: CouponScope[]
  minimum_order_amount: number
  eligible_plan_ids: number[]
  validity_mode: CouponValidityMode
  validity_days?: number
  fixed_expires_at?: string | null
  valid_from?: string | null
  total_issue_limit?: number | null
  rules?: Record<string, unknown>
}

export interface CouponBatchIssueInput {
  template_id: number
  user_ids: number[]
  source?: CouponIssueSource
  idempotency_key: string
  metadata?: Record<string, unknown>
}

export interface CouponRewardPoolInput {
  activity: CouponRewardActivity
  version: string
  status: CouponRewardPoolStatus
  coupon_weight_bp: number
  redeem_code_weight_bp?: number
  balance_weight_bp: number
  reward_config?: CouponRewardPoolConfig
  fallback_template_id: number
  entries: CouponRewardPoolEntry[]
}

export interface CouponRewardPoolConfig {
  balance_entries?: BalanceRewardPoolEntry[]
  redeem_entries?: RedeemRewardPoolEntry[]
}

export interface BalanceRewardPoolEntry {
  name?: string
  amount: number
  weight_bp: number
  enabled: boolean
  per_user_day_limit?: number
}

export interface RedeemRewardPoolEntry {
  name?: string
  batch_name?: string
  code_type?: string
  weight_bp: number
  enabled: boolean
  delivery_mode?: 'issue_code' | 'auto_redeem' | string
}
