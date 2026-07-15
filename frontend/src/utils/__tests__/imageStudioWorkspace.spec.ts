import { describe, expect, it, vi } from 'vitest'
import type { ImageStudioCatalog, ImageStudioModelOption } from '@/api/imageStudio'
import {
  findFirstImageStudioKeyWithModels,
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

  it('selects the first API key whose group exposes image models', async () => {
    const imageModel: ImageStudioModelOption = { id: 'gpt-image-1', display_name: 'GPT Image 1' }
    const loadModels = vi.fn(async (keyId: number) => {
      if (keyId === 10) throw new Error('IMAGE_STUDIO_IMAGE_NOT_ALLOWED')
      if (keyId === 11) return []
      return [imageModel]
    })

    const selection = await findFirstImageStudioKeyWithModels([
      { id: 10, name: 'Default' },
      { id: 11, name: 'Text only' },
      { id: 12, name: 'Images' },
      { id: 13, name: 'Unused' },
    ], loadModels)

    expect(selection).toEqual({ key: { id: 12, name: 'Images' }, models: [imageModel] })
    expect(loadModels.mock.calls.map(([keyId]) => keyId)).toEqual([10, 11, 12])
  })

  it('returns null when no API key exposes image models', async () => {
    const loadModels = vi.fn()
      .mockRejectedValueOnce(new Error('not allowed'))
      .mockResolvedValueOnce([])

    await expect(findFirstImageStudioKeyWithModels([
      { id: 10, name: 'Default' },
      { id: 11, name: 'Text only' },
    ], loadModels)).resolves.toBeNull()
  })
})
