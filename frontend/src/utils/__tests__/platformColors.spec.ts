import { describe, expect, it } from 'vitest'

import { platformLabel } from '../platformColors'

describe('platformLabel', () => {
  it('localizes system-owned image and team platform categories without changing provider names', () => {
    expect(platformLabel('image', 'zh')).toBe('图片')
    expect(platformLabel('team', 'zh')).toBe('团队企业')
    expect(platformLabel('image', 'en')).toBe('Image')
    expect(platformLabel('team', 'en')).toBe('Team / Enterprise')
    expect(platformLabel('sensenova-u1.5-lite', 'en')).toBe('sensenova-u1.5-lite')
  })
})
