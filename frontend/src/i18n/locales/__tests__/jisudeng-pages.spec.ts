import { describe, expect, it } from 'vitest'
import { jisudengPagesEn } from '@/i18n/locales/jisudeng-pages.en'
import { jisudengPagesZh } from '@/i18n/locales/jisudeng-pages.zh'

describe('Jisudeng page locale contracts', () => {
  it('translates the Image Studio route subtitle in both locales', () => {
    expect(jisudengPagesZh.imageStudio.subtitle).toBeTruthy()
    expect(jisudengPagesEn.imageStudio.subtitle).toBeTruthy()
  })
})
