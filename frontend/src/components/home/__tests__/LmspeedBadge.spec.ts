import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'

import LmspeedBadge from '../LmspeedBadge.vue'

const CLAIM_BADGE_URL =
  'https://lmspeed.net/api/provider/claim-badge/2039?claim=2039-1kHJJSboOUMX0Au9G01tAaBWozH20jbC'

describe('LmspeedBadge', () => {
  it.each([
    ['zh', '极速蹬已被 LMSpeed.net 收录'],
    ['en', 'Jisudeng is listed on LMSpeed.net'],
  ])('renders the LMSpeed ownership badge with safe external-link attributes in %s', (locale, label) => {
    const i18n = createI18n({
      legacy: false,
      locale,
      messages: {
        zh: { home: { jisudeng: { footer: { lmspeedBadgeAlt: () => '极速蹬已被 LMSpeed.net 收录' } } } },
        en: { home: { jisudeng: { footer: { lmspeedBadgeAlt: () => 'Jisudeng is listed on LMSpeed.net' } } } },
      },
    })
    const wrapper = mount(LmspeedBadge, {
      global: { plugins: [i18n] },
    })
    const link = wrapper.get('a')
    const image = wrapper.get('img')

    expect(link.attributes('href')).toBe('https://lmspeed.net/')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer nofollow')
    expect(link.attributes('aria-label')).toBe(label)

    expect(image.attributes('src')).toBe(CLAIM_BADGE_URL)
    expect(image.attributes('alt')).toBe(label)
    expect(image.attributes('width')).toBe('190')
    expect(image.attributes('height')).toBe('64')
    expect(image.attributes('decoding')).toBe('async')
    expect(image.attributes('loading')).toBeUndefined()
  })
})
