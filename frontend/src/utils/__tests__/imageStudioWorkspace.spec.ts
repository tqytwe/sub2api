import { describe, expect, it } from 'vitest'
import type { ImageStudioCatalog } from '@/api/imageStudio'
import {
  flattenImageStudioTemplates,
  isImageStudioPromptValid,
  resolveInitialImageStudioTemplate,
} from '@/utils/imageStudioWorkspace'

const catalog: ImageStudioCatalog = {
  intents: [
    {
      id: 'commerce',
      label: { zh: '电商', en: 'Commerce' },
      templates: [
        { id: 'white', label: { zh: '白底', en: 'White' }, defaults: { size: '1024x1024', count: 1 } },
      ],
    },
    {
      id: 'creative',
      label: { zh: '创意', en: 'Creative' },
      templates: [
        { id: 'free', label: { zh: '自由', en: 'Free' }, defaults: { size: '1536x1024', count: 1 } },
      ],
    },
  ],
}

describe('image studio workspace helpers', () => {
  it('flattens intent templates in catalog order', () => {
    expect(flattenImageStudioTemplates(catalog).map((item) => item.template.id)).toEqual(['white', 'free'])
  })

  it('restores the preferred template and falls back to the first template', () => {
    expect(resolveInitialImageStudioTemplate(catalog, 'free')?.intent.id).toBe('creative')
    expect(resolveInitialImageStudioTemplate(catalog, 'missing')?.template.id).toBe('white')
  })

  it('requires a non-whitespace prompt', () => {
    expect(isImageStudioPromptValid('  ')).toBe(false)
    expect(isImageStudioPromptValid('matte black headphones')).toBe(true)
  })
})
