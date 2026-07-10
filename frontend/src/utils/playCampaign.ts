import type { PlayCampaignSummary } from '@/api/play'

function campaignLocaleKey(locale: string): 'zh' | 'en' {
  return locale.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

export function resolveCampaignDisplayName(
  campaign: PlayCampaignSummary | null | undefined,
  locale: string,
): string {
  if (!campaign) return ''
  const i18n = campaign.rules?.name_i18n
  if (i18n && typeof i18n === 'object') {
    const key = campaignLocaleKey(locale)
    return i18n[key] || i18n.en || i18n.zh || campaign.name
  }
  return campaign.name
}
