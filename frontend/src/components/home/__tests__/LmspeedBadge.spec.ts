import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import LmspeedBadge from '../LmspeedBadge.vue'

const CLAIM_BADGE_URL =
  'https://lmspeed.net/api/provider/claim-badge/2039?claim=2039-1kHJJSboOUMX0Au9G01tAaBWozH20jbC'

describe('LmspeedBadge', () => {
  it('renders the LMSpeed ownership badge with safe external-link attributes', () => {
    const wrapper = mount(LmspeedBadge)
    const link = wrapper.get('a')
    const image = wrapper.get('img')

    expect(link.attributes('href')).toBe('https://lmspeed.net/')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
    expect(link.attributes('aria-label')).toBe('Featured on LMSpeed.net')

    expect(image.attributes('src')).toBe(CLAIM_BADGE_URL)
    expect(image.attributes('alt')).toBe('Featured on LMSpeed.net')
    expect(image.attributes('width')).toBe('190')
    expect(image.attributes('height')).toBe('64')
    expect(image.attributes('decoding')).toBe('async')
    expect(image.attributes('loading')).toBeUndefined()
  })
})
