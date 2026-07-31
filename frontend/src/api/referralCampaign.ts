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
  }
  enrollment?: { campaign_id: number; user_id: number; enrolled_at: string }
  tiers: Array<{ tier: number; required_invites: number; reward_amount: number; currency: string }>
  invited_count: number
  qualified_count: number
  rewards: ReferralCampaignReward[]
  ranking?: { rank: number; email_masked: string; qualified_count: number; reward_amount: number; is_me?: boolean }
  leaderboard: Array<{ rank: number; email_masked: string; qualified_count: number; reward_amount: number; is_me?: boolean }>
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
