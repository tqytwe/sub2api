import type { CouponAudience, CouponClaimPolicy, CouponCampaignPreview, CouponCampaignResult, CouponSummary, CouponEvent } from '@/types/couponCampaign'
import { apiClient } from '../client'
import type {
  CouponBatchIssueInput,
  CouponIssueBatch,
  CouponRewardActivity,
  CouponRewardPoolInput,
  CouponRewardPoolVersion,
  CouponTemplate,
  CouponTemplateInput,
  UserCouponPage,
} from '@/types/coupon'
import type { BasePaginationResponse } from '@/types'

const basePath = '/admin/promo-codes/coupons'

export const adminCouponAPI = {
  listClaimPolicies(ids: number[]) {
    return apiClient.get<CouponClaimPolicy[]>(`${basePath}/claim-policies`, { params: { ids: ids.join(',') } })
  },
  getClaimPolicy(id: number) {
    return apiClient.get<CouponClaimPolicy>(`${basePath}/templates/${id}/claim-policy`)
  },
  saveClaimPolicy(id: number, policy: CouponClaimPolicy) {
    return apiClient.put<CouponClaimPolicy>(`${basePath}/templates/${id}/claim-policy`, policy)
  },
  previewCampaign(template_id: number, audience: CouponAudience) {
    return apiClient.post<CouponCampaignPreview>(`${basePath}/campaigns/preview`, { template_id, audience })
  },
  executeCampaign(token: string) {
    return apiClient.post<CouponCampaignResult>(`${basePath}/campaigns/execute`, { token })
  },
  summary(params: Record<string, unknown>) {
    return apiClient.get<CouponSummary>(`${basePath}/summary`, { params })
  },
  events(id: number) {
    return apiClient.get<CouponEvent[]>(`${basePath}/user-coupons/${id}/events`)
  },

  listTemplates(params: { page?: number; page_size?: number; status?: string; search?: string } = {}) {
    return apiClient.get<BasePaginationResponse<CouponTemplate>>(`${basePath}/templates`, { params })
  },

  createTemplate(input: CouponTemplateInput) {
    return apiClient.post<CouponTemplate>(`${basePath}/templates`, input)
  },

  updateTemplate(id: number, input: CouponTemplateInput) {
    return apiClient.put<CouponTemplate>(`${basePath}/templates/${id}`, input)
  },

  deleteTemplate(id: number) {
    return apiClient.delete(`${basePath}/templates/${id}`)
  },

  listUserCoupons(params: { page?: number; page_size?: number; user_id?: number; user?: string; template_id?: number; source?: string; status?: string; issued_from?: string; issued_to?: string } = {}) {
    return apiClient.get<UserCouponPage>(`${basePath}/user-coupons`, { params })
  },

  issueBatch(input: CouponBatchIssueInput) {
    return apiClient.post<CouponIssueBatch>(`${basePath}/user-coupons/batches`, input)
  },

  listBatches(params: { page?: number; page_size?: number; template_id?: number } = {}) {
    return apiClient.get<BasePaginationResponse<CouponIssueBatch>>(`${basePath}/batches`, { params })
  },

  voidUserCoupon(id: number, reason: string) {
    return apiClient.post(`${basePath}/user-coupons/${id}/void`, { reason })
  },

  listPools(activity?: CouponRewardActivity) {
    return apiClient.get<CouponRewardPoolVersion[]>(`${basePath}/pools`, { params: activity ? { activity } : undefined })
  },

  createPool(input: CouponRewardPoolInput) {
    return apiClient.post<CouponRewardPoolVersion>(`${basePath}/pools`, input)
  },

  updatePool(id: number, input: CouponRewardPoolInput) {
    return apiClient.put<CouponRewardPoolVersion>(`${basePath}/pools/${id}`, input)
  },

  deletePool(id: number) {
    return apiClient.delete(`${basePath}/pools/${id}`)
  },

  publishPool(id: number) {
    return apiClient.post<CouponRewardPoolVersion>(`${basePath}/pools/${id}/publish`)
  },
}

export default adminCouponAPI
