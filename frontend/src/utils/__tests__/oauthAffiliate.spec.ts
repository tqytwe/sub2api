import { beforeEach, describe, expect, it, vi } from 'vitest'
import { attributeReferralCampaign } from '@/api/referralCampaign'
import {
  buildRegisterInviteLink,
  clearAffiliateReferralCode,
  clearOAuthAffiliateCode,
  loadReferralCampaignToken,
  loadAffiliateReferralCode,
  loadOAuthAffiliateCode,
  resolveAffiliateReferralCode,
  storeReferralCampaignToken,
  storeAffiliateReferralCode,
  storeOAuthAffiliateCode,
  tryAttributeReferralCampaign
} from '@/utils/oauthAffiliate'

vi.mock('@/api/referralCampaign', () => ({
  attributeReferralCampaign: vi.fn()
}))

const attributeReferralCampaignMock = vi.mocked(attributeReferralCampaign)

describe('oauthAffiliate', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    attributeReferralCampaignMock.mockReset()
    vi.useRealTimers()
  })

  it('persists affiliate referral code across pages', () => {
    expect(resolveAffiliateReferralCode(' 5579J7CFG9PF ')).toBe('5579J7CFG9PF')
    expect(loadAffiliateReferralCode()).toBe('5579J7CFG9PF')
    expect(resolveAffiliateReferralCode()).toBe('5579J7CFG9PF')
  })

  it('expires stale affiliate referral code', () => {
    const now = Date.UTC(2026, 0, 1)
    storeAffiliateReferralCode('AFF123', now)

    expect(loadAffiliateReferralCode(now + 30 * 24 * 60 * 60 * 1000 - 1)).toBe('AFF123')
    expect(loadAffiliateReferralCode(now + 30 * 24 * 60 * 60 * 1000 + 1)).toBe('')
    expect(localStorage.getItem('affiliate_referral_code')).toBeNull()
  })

  it('keeps oauth transient code separate from persistent referral code', () => {
    storeAffiliateReferralCode('PERSISTED')
    storeOAuthAffiliateCode('OAUTH')

    expect(loadAffiliateReferralCode()).toBe('PERSISTED')
    expect(loadOAuthAffiliateCode()).toBe('OAUTH')

    clearOAuthAffiliateCode()
    expect(loadOAuthAffiliateCode()).toBe('')
    expect(loadAffiliateReferralCode()).toBe('PERSISTED')

    clearAffiliateReferralCode()
    expect(loadAffiliateReferralCode()).toBe('')
  })

  it('builds register links that carry both affiliate and team codes', () => {
    expect(buildRegisterInviteLink('XRFP2MCTF4DS', '8895eab6')).toBe(
      `${window.location.origin}/register?ref=XRFP2MCTF4DS&team=8895EAB6`
    )
  })

  it('keeps a campaign token when authenticated attribution fails', async () => {
    storeReferralCampaignToken('signed-campaign-token')
    attributeReferralCampaignMock.mockRejectedValueOnce(new Error('temporary failure'))

    await tryAttributeReferralCampaign()

    expect(attributeReferralCampaignMock).toHaveBeenCalledWith('signed-campaign-token')
    expect(loadReferralCampaignToken()).toBe('signed-campaign-token')
  })

  it('clears a campaign token only after attribution succeeds', async () => {
    storeReferralCampaignToken('signed-campaign-token')
    attributeReferralCampaignMock.mockResolvedValueOnce()

    await tryAttributeReferralCampaign()

    expect(attributeReferralCampaignMock).toHaveBeenCalledWith('signed-campaign-token')
    expect(loadReferralCampaignToken()).toBe('')
  })
})
