import { describe, expect, it } from 'vitest'
import { resolveCampaignDisplayName } from '@/utils/playCampaign'
import type { PlayCampaignSummary } from '@/api/play'

const campaign: PlayCampaignSummary = {
  id: 1,
  name: '开服福利周',
  start_at: '2026-01-01T00:00:00Z',
  end_at: '2026-12-31T00:00:00Z',
  rules: {
    recharge_bonus_pct: 10,
    name_i18n: { zh: '开服福利周', en: 'Launch perks week' },
  },
}

describe('resolveCampaignDisplayName', () => {
  it('returns English title for en locale', () => {
    expect(resolveCampaignDisplayName(campaign, 'en')).toBe('Launch perks week')
  })

  it('returns Chinese title for zh locale', () => {
    expect(resolveCampaignDisplayName(campaign, 'zh')).toBe('开服福利周')
  })

  it('falls back to campaign.name when name_i18n is missing', () => {
    expect(resolveCampaignDisplayName({ ...campaign, rules: {} }, 'en')).toBe('开服福利周')
  })
})
