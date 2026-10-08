import { describe, expect, it } from 'vitest'
import { ensureLocaleMessagesForRoute, i18n } from '@/i18n'

describe('welfare reward copy across lazy route scopes', () => {
  it.each(['zh', 'en'] as const)('does not restore fixed rebate rates when loading Affiliate in %s', async (locale) => {
    await ensureLocaleMessagesForRoute('Affiliate', locale)
    const messages = i18n.global.getLocaleMessage(locale) as { affiliate: { standard: { title: string }; campaign: { rebatePolicy: string }; growth: { rebatePolicy: string; description: string } } }
    expect(messages.affiliate.standard.title).not.toContain('10%')
    expect(messages.affiliate.campaign.rebatePolicy).not.toContain('10%')
    expect(messages.affiliate.growth.rebatePolicy).not.toContain('10%')
    expect(messages.affiliate.growth.description).not.toContain('500')
  })
})
