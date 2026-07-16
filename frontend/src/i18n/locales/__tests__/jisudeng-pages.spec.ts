import { describe, expect, it } from 'vitest'
import { jisudengPagesEn } from '@/i18n/locales/jisudeng-pages.en'
import { jisudengPagesZh } from '@/i18n/locales/jisudeng-pages.zh'

describe('Jisudeng page locale contracts', () => {
  it('translates the Image Studio route subtitle in both locales', () => {
    expect(jisudengPagesZh.imageStudio.subtitle).toBeTruthy()
    expect(jisudengPagesEn.imageStudio.subtitle).toBeTruthy()
  })

  it('keeps prompt validation, saved-draft, and gallery retry copy in sync', () => {
    for (const locale of [jisudengPagesZh.imageStudio, jisudengPagesEn.imageStudio]) {
      expect(locale.promptTooLong).toContain('8000')
      expect(locale.expertPromptTooLong).toContain('8000')
      expect(locale.settingsRetained).toBeTruthy()
      expect(locale.galleryLoadFailed).toBeTruthy()
      expect(locale.retryGallery).toBeTruthy()
    }
  })
})
