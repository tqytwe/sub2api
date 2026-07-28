import { describe, expect, it } from 'vitest'
import { jisudengPagesEn } from '@/i18n/locales/jisudeng-pages.en'
import { jisudengPagesZh } from '@/i18n/locales/jisudeng-pages.zh'

describe('Jisudeng page locale contracts', () => {
  it('translates the Image Studio route subtitle in both locales', () => {
    expect(jisudengPagesZh.imageStudio.subtitle).toBeTruthy()
    expect(jisudengPagesEn.imageStudio.subtitle).toBeTruthy()
  })

  it('keeps Android download page copy complete in both locales', () => {
    for (const locale of [jisudengPagesZh.androidDownload, jisudengPagesEn.androidDownload]) {
      expect(locale.title).toBeTruthy()
      expect(locale.downloadApk).toBeTruthy()
      expect(locale.scanTitle).toBeTruthy()
      expect(locale.packageInfo).toBeTruthy()
      expect(locale.features).toHaveLength(4)
      expect(locale.defaultNote).toBeTruthy()
    }
  })

  it('keeps prompt validation, saved-draft, and gallery retry copy in sync', () => {
    for (const locale of [jisudengPagesZh.imageStudio, jisudengPagesEn.imageStudio]) {
      expect(locale.promptTooLong).toContain('8000')
      expect(locale.expertPromptTooLong).toContain('8000')
      expect(locale.settingsRetained).toBeTruthy()
      expect(locale.galleryLoadFailed).toBeTruthy()
      expect(locale.loadCapabilitiesFailed).toBeTruthy()
      expect(locale.retryGallery).toBeTruthy()
      expect(locale.activeJobs).toBeTruthy()
      expect(locale.jobProgress).toBeTruthy()
      expect(locale.itemCounts).toBeTruthy()
      expect(locale.cancel).toBeTruthy()
      expect(locale.cancelFailed).toBeTruthy()
      expect(locale.assetDimensions).toBeTruthy()
      expect(locale.downloadAll).toBeTruthy()
      expect(locale.previousPage).toBeTruthy()
      expect(locale.nextPage).toBeTruthy()
      expect(locale.pageStatus).toBeTruthy()
      expect(locale.background).toBeTruthy()
      expect(locale.backgroundOptions.auto).toBeTruthy()
      expect(locale.backgroundOptions.opaque).toBeTruthy()
      expect(locale.backgroundOptions.transparent).toBeTruthy()
      expect(locale.outputFormat).toBeTruthy()
      expect(locale.outputCompression).toBeTruthy()
      expect(locale.inputFidelity).toBeTruthy()
      expect(locale.inputFidelityOptions.low).toBeTruthy()
      expect(locale.inputFidelityOptions.high).toBeTruthy()
      expect(locale.status.partial).toBeTruthy()
      expect(locale.status.cancelled).toBeTruthy()
    }
  })

  it('describes the quiz balance branch as one full completion reward in both locales', () => {
    expect(jisudengPagesZh.quiz.rewardHint).toContain('${amount}')
    expect(jisudengPagesZh.quiz.rewardHint).toContain('得分大于 0')
    expect(jisudengPagesZh.quiz.rewardHint).not.toContain('每题')
    expect(jisudengPagesZh.playHub.quizPending).toContain('${reward}')
    expect(jisudengPagesZh.playHub.quizPending).not.toContain('每题')
    expect(jisudengPagesZh.play.quizQuest.subtitle).toContain('得分大于 0')
    expect(jisudengPagesZh.play.quizQuest.steps.join(' ')).not.toContain('按正确题数')

    expect(jisudengPagesEn.quiz.rewardHint).toContain('${amount}')
    expect(jisudengPagesEn.quiz.rewardHint).toMatch(/score above zero/i)
    expect(jisudengPagesEn.quiz.rewardHint).not.toMatch(/per correct/i)
    expect(jisudengPagesEn.playHub.quizPending).toContain('${reward}')
    expect(jisudengPagesEn.playHub.quizPending).not.toMatch(/per correct/i)
    expect(jisudengPagesEn.play.quizQuest.subtitle).toMatch(/score above zero/i)
    expect(jisudengPagesEn.play.quizQuest.steps.join(' ')).not.toMatch(/correct answers/i)
  })
})
