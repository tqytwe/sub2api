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

  it('describes the real sync, gateway async, and batch image boundaries', () => {
    expect(jisudengHomeZh.sections.imageLede).toContain('同步 Images')
    expect(jisudengHomeZh.sections.imageLede).toContain('单请求异步')
    expect(jisudengHomeZh.sections.imageLede).toContain('Batch')
    expect(jisudengHomeEn.sections.imageLede).toContain('Synchronous Images')
    expect(jisudengHomeEn.sections.imageLede).toContain('single-request async')
    expect(jisudengHomeEn.sections.imageLede).toContain('Batch')
  })

  it('does not claim all image prompts are memory-only now that durable async exists', () => {
    expect(jisudengHomeZh.faq.items[0]?.a).toContain('异步')
    expect(jisudengHomeZh.faq.items[0]?.a).toContain('加密')
    expect(jisudengHomeEn.faq.items[0]?.a).toContain('Async')
    expect(jisudengHomeEn.faq.items[0]?.a).toContain('encrypted')
    expect(jisudengHomeZh.faq.items[0]?.a).not.toContain('只在单次响应周期内存在于内存')
    expect(jisudengHomeEn.faq.items[0]?.a).not.toContain('memory only')
    expect(jisudengHomeZh.manifesto.body2).toContain('异步')
    expect(jisudengHomeEn.manifesto.body2).toContain('Async')
    expect(jisudengHomeZh.manifesto.body2).not.toContain('不写日志、不入数据库')
    expect(jisudengHomeEn.manifesto.body2).not.toContain('No logs, no database')
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
