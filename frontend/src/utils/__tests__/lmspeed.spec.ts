import { describe, expect, it } from 'vitest'

import { buildLmspeedSpeedTestUrl, normalizeLmspeedBaseUrl, resolveLmspeedLocale } from '@/utils/lmspeed'

describe('lmspeed utils', () => {
  it.each([
    ['https://api.example.com', 'https://api.example.com/v1'],
    ['https://api.example.com/', 'https://api.example.com/v1'],
    ['https://api.example.com/v1', 'https://api.example.com/v1'],
    ['https://api.example.com/v1/', 'https://api.example.com/v1'],
  ])('normalizeLmspeedBaseUrl(%s) -> %s', (input, expected) => {
    expect(normalizeLmspeedBaseUrl(input)).toBe(expected)
  })

  it('builds a zh LMSpeed speed-test link with encoded endpoint and key', () => {
    const url = buildLmspeedSpeedTestUrl({
      baseUrl: ' https://api.example.com/ ',
      apiKey: 'sk-test/key with spaces',
      locale: 'zh',
    })

    const parsed = new URL(url)
    expect(parsed.origin).toBe('https://lmspeed.net')
    expect(parsed.pathname).toBe('/zh')
    expect(parsed.searchParams.get('baseUrl')).toBe('https://api.example.com/v1')
    expect(parsed.searchParams.get('apiKey')).toBe('sk-test/key with spaces')
    expect(parsed.searchParams.has('modelId')).toBe(false)
  })

  it('defaults to the English LMSpeed root when no locale is provided', () => {
    const url = buildLmspeedSpeedTestUrl({
      baseUrl: 'https://api.example.com',
      apiKey: 'sk-test',
    })

    const parsed = new URL(url)
    expect(parsed.origin).toBe('https://lmspeed.net')
    expect(parsed.pathname).toBe('/')
  })

  it('adds modelId only when provided', () => {
    const withModel = buildLmspeedSpeedTestUrl({
      baseUrl: 'https://api.example.com/v1',
      apiKey: 'sk-test',
      modelId: 'free:Qwen3-30B-A3B',
    })
    expect(new URL(withModel).searchParams.get('modelId')).toBe('free:Qwen3-30B-A3B')

    const withoutModel = buildLmspeedSpeedTestUrl({
      baseUrl: 'https://api.example.com/v1',
      apiKey: 'sk-test',
      modelId: '   ',
    })
    expect(new URL(withoutModel).searchParams.has('modelId')).toBe(false)
  })

  it.each([
    ['zh', 'zh'],
    ['zh-CN', 'zh'],
    ['en', 'en'],
    ['en-US', 'en'],
    ['ru-RU', 'ru'],
    ['fr-FR', 'en'],
    [null, 'en'],
  ] as const)('maps app locale %s to LMSpeed locale %s', (input, expected) => {
    expect(resolveLmspeedLocale(input)).toBe(expected)
  })
})
