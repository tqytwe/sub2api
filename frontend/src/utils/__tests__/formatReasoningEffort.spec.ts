import { beforeEach, describe, expect, it, vi } from 'vitest'

const { locale, translate } = vi.hoisted(() => ({
  locale: { value: 'zh' },
  translate: vi.fn((key: string) => key),
}))

vi.mock('@/i18n', () => ({
  getLocale: () => locale.value,
  i18n: { global: { t: translate } },
}))

import { formatReasoningEffort } from '../format'

describe('formatReasoningEffort', () => {
  beforeEach(() => {
    translate.mockClear()
  })

  it('uses the active locale for known effort values', () => {
    locale.value = 'zh'
    expect(formatReasoningEffort('medium')).toBe('usage.reasoningEffortValues.medium')

    locale.value = 'en'
    expect(formatReasoningEffort('x-high')).toBe('usage.reasoningEffortValues.xhigh')
  })

  it('does not pass an unknown backend value through as a user-visible English label', () => {
    locale.value = 'zh'
    expect(formatReasoningEffort('provider-new-tier')).toBe('common.unknown')
  })

  it('keeps omitted and explicitly minimal effort neutral', () => {
    expect(formatReasoningEffort(undefined)).toBe('-')
    expect(formatReasoningEffort('minimal')).toBe('-')
  })
})
