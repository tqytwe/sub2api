import { describe, expect, it } from 'vitest'

import { PUBLIC_DOC_CONTENT_EN } from '@/content/public-docs-data.en'
import { jisudengHomeEn } from '@/i18n/locales/jisudeng-home.en'
import { jisudengPagesEn } from '@/i18n/locales/jisudeng-pages.en'
import { resolvePublicRouteSeo } from '@/utils/routeSeo'

function flatten(value: unknown): string {
  if (typeof value === 'string') return value
  if (Array.isArray(value)) return value.map(flatten).join('\n')
  if (value && typeof value === 'object') return Object.values(value).map(flatten).join('\n')
  return ''
}

describe('English brand copy', () => {
  it('uses the approved English positioning without national framing', () => {
    const copy = [
      flatten(jisudengHomeEn),
      flatten(jisudengPagesEn),
      flatten(PUBLIC_DOC_CONTENT_EN),
      flatten(resolvePublicRouteSeo('/en')),
      flatten(resolvePublicRouteSeo('/en/models')),
      flatten(resolvePublicRouteSeo('/en/docs')),
    ].join('\n')

    expect(copy).toContain('Access DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through one OpenAI-compatible API.')
    expect(copy).toContain('Jisudeng')
    expect(copy).not.toMatch(/\bChinese\b/i)
    expect(copy).not.toMatch(/\bChinese AI\b/i)
    expect(copy).not.toMatch(/\bChina\b/i)
    expect(copy).not.toMatch(/国产|出海/)
  })
})
