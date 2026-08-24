import { describe, expect, it } from 'vitest'

import { modelMatchesFamily, parseModelFamily } from '../modelFamilies'

describe('model plaza family compatibility', () => {
  it('accepts only the public model-family route values', () => {
    expect(parseModelFamily('DeepSeek')).toBe('deepseek')
    expect(parseModelFamily('glm')).toBe('glm')
    expect(parseModelFamily('gpt')).toBeNull()
  })

  it('retains the legacy family aliases when filtering plaza models', () => {
    const model = { name: 'Moonshot K2', platform: 'openai' }

    expect(modelMatchesFamily(model, 'kimi')).toBe(true)
    expect(modelMatchesFamily(model, 'qwen')).toBe(false)
  })
})
