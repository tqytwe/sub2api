import { apiClient } from './client'
import type {
  CouponPaymentQuote,
  CouponPaymentQuoteRequest,
  UserCouponPage,
  UserCouponStatus,
} from '@/types/coupon'

export const couponAPI = {
  getMyCoupons(params: { page?: number; page_size?: number; status?: UserCouponStatus } = {}) {
    return apiClient.get<UserCouponPage>('/coupons/me', { params })
  },

  quotePaymentCoupon(input: CouponPaymentQuoteRequest) {
    return apiClient.post<CouponPaymentQuote>('/payment/coupons/quote', input)
  },
}

export default couponAPI
