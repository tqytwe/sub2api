import { describe, expect, it } from 'vitest'
import { jisudengHomeEn } from '@/i18n/locales/jisudeng-home.en'
import { jisudengHomeZh } from '@/i18n/locales/jisudeng-home.zh'

describe('home truthfulness copy', () => {
  it('describes the deployed multipart field limit instead of a false request limit', () => {
    expect(jisudengHomeZh.image.desc).toContain('每个 multipart 字段最大 20 MiB')
    expect(jisudengHomeEn.image.desc).toContain('20 MiB per multipart field')
    expect(jisudengHomeZh.image.desc).not.toContain('30 MiB')
    expect(jisudengHomeEn.image.desc).not.toContain('30 MiB')
  })

  it('has visible freshness labels for the real stats snapshot', () => {
    expect(jisudengHomeZh.stats.through).toBe('最近运营样本结束于 {time}')
    expect(jisudengHomeZh.stats.computed).toContain('{time}')
    expect(jisudengHomeZh.stats.stale).toBeTruthy()
    expect(jisudengHomeEn.stats.through).toBe('Latest operations sample ended {time}')
    expect(jisudengHomeEn.stats.computed).toContain('{time}')
    expect(jisudengHomeEn.stats.stale).toBeTruthy()
  })
})
