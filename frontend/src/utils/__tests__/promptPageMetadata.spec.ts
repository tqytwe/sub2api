import { afterEach, describe, expect, it } from 'vitest'
import { applyPromptPageMetadata, clearPromptPageMetadata } from '@/utils/promptPageMetadata'

describe('提示词页面元数据', () => {
  afterEach(() => {
    document.head.innerHTML = ''
  })

  it('sets Chinese canonical, sharing, and 极速蹬 publisher metadata', () => {
    applyPromptPageMetadata({
      title: '电影感人像',
      description: '用于制作电影感人物视觉。',
      path: '/prompts/123',
      image: 'https://img.example/portrait.jpg',
      kind: 'detail',
    })

    expect(document.title).toBe('电影感人像 - 极速蹬提示词库')
    expect(document.querySelector('link[rel="canonical"]')?.getAttribute('href'))
      .toBe(`${window.location.origin}/prompts/123`)
    expect(document.querySelector('meta[property="og:site_name"]')?.getAttribute('content'))
      .toBe('极速蹬提示词库')
    const structured = JSON.parse(
      document.querySelector('#jisudeng-prompt-structured-data')?.textContent || '{}',
    )
    expect(structured.publisher.name).toBe('极速蹬')
  })

  it('restores the title and existing meta or canonical values on cleanup', () => {
    document.title = '原页面标题'
    document.head.innerHTML += [
      '<meta name="description" content="原页面描述">',
      '<meta property="og:title" content="原分享标题">',
      '<link rel="canonical" href="https://example.com/original">',
    ].join('')

    applyPromptPageMetadata({
      title: '电影感人像',
      description: '用于制作电影感人物视觉。',
      path: '/prompts/123',
      kind: 'detail',
    })
    clearPromptPageMetadata()

    expect(document.title).toBe('原页面标题')
    expect(document.querySelector('meta[name="description"]')?.getAttribute('content'))
      .toBe('原页面描述')
    expect(document.querySelector('meta[property="og:title"]')?.getAttribute('content'))
      .toBe('原分享标题')
    expect(document.querySelector('link[rel="canonical"]')?.getAttribute('href'))
      .toBe('https://example.com/original')
    expect(document.querySelector('#jisudeng-prompt-structured-data')).toBeNull()
  })
})
