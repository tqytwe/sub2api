import { apiClient } from './client'

export interface ReferralCampaignReward {
  id: number
  campaign_id: number
  tier: number
  amount: number
  currency: string
  status: string
  claim_deadline?: string
  frozen_until?: string
  version: number
}

export interface ReferralCampaignProgress {
  campaign: {
    id: number
    key: string
    name: string
    status: string
    version: number
    registration_from: string
    registration_to: string
    starts_at: string
    ends_at: string
    qualification_to: string
    claim_deadline: string
    pay_threshold: number
    usage_threshold: number
    max_enrollments: number
    risk_hold_hours: number
    reward_mode: 'additive' | 'replace'
    public_rules_md: string
    invitee_notice_md: string
    legacy_rebate_policy: 'exclude' | 'stack'
    rules_version: number
    rules_updated_at: string
  }
  enrollment?: { campaign_id: number; user_id: number; enrolled_at: string }
  tiers: Array<{ tier: number; required_invites: number; reward_amount: number; currency: string }>
  invited_count: number
  qualified_count: number
  rewards: ReferralCampaignReward[]
  ranking?: { rank: number; email_masked: string; qualified_count: number; reward_amount: number; is_me?: boolean }
  leaderboard: Array<{ rank: number; email_masked: string; qualified_count: number; reward_amount: number; is_me?: boolean }>
  unseen_update?: boolean
  attention?: string
}

export interface ReferralCampaignInvitePreview {
  campaign: ReferralCampaignProgress['campaign']
}

export async function getReferralCampaignInvitePreview(token: string): Promise<ReferralCampaignInvitePreview> {
  const { data } = await apiClient.get<ReferralCampaignInvitePreview>('/auth/referral-campaign-preview', { params: { token } })
  return data
}

export async function listReferralCampaigns(): Promise<ReferralCampaignProgress[]> {
  const { data } = await apiClient.get<ReferralCampaignProgress[]>('/user/aff/campaigns')
  return data ?? []
}

export async function enrollReferralCampaign(campaignId: number): Promise<void> {
  await apiClient.post(`/user/aff/campaigns/${campaignId}/enroll`)
}

export async function attributeReferralCampaign(token: string): Promise<void> {
  await apiClient.post('/user/aff/campaigns/attribute', { token })
}

export async function getReferralCampaignProgress(campaignId: number): Promise<ReferralCampaignProgress> {
  const { data } = await apiClient.get<ReferralCampaignProgress>(`/user/aff/campaigns/${campaignId}/progress`)
  return data
}

export async function getReferralCampaignInviteToken(campaignId: number): Promise<string> {
  const { data } = await apiClient.get<{ token: string }>(`/user/aff/campaigns/${campaignId}/invite-token`)
  return data.token
}

export async function claimReferralCampaignReward(campaignId: number, rewardId: number, campaignVersion: number): Promise<ReferralCampaignReward> {
  const { data } = await apiClient.post<ReferralCampaignReward>(`/user/aff/campaigns/${campaignId}/rewards/${rewardId}/claim`, { campaign_version: campaignVersion })
  return data
}

export async function markReferralCampaignViewed(campaignId: number): Promise<void> {
  await apiClient.post(`/user/aff/campaigns/${campaignId}/view`)
}
