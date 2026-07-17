import { describe, expect, it, vi } from 'vitest'
import type { ImageStudioCatalog, ImageStudioModelOption } from '@/api/imageStudio'
import {
  IMAGE_STUDIO_PROMPT_LIMIT,
  countImageStudioCodePoints,
  findFirstImageStudioKeyWithModels,
  flattenImageStudioTemplates,
  isImageStudioPromptValid,
  loadAllActiveImageStudioKeys,
  resizeImageStudioTextarea,
  resolveInitialImageStudioTemplate,
  validateImageStudioPrompt,
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

  it('counts Unicode code points instead of UTF-16 code units', () => {
    expect(countImageStudioCodePoints('a😀b')).toBe(3)
    expect(countImageStudioCodePoints('😀'.repeat(IMAGE_STUDIO_PROMPT_LIMIT))).toBe(
      IMAGE_STUDIO_PROMPT_LIMIT,
    )
  })

  it('applies the same 8000-code-point validation boundary to prompts', () => {
    expect(validateImageStudioPrompt('')).toBe('required')
    expect(validateImageStudioPrompt(' \n\t')).toBe('required')
    expect(validateImageStudioPrompt(' \n\t', { required: false })).toBeNull()
    expect(validateImageStudioPrompt('😀'.repeat(IMAGE_STUDIO_PROMPT_LIMIT))).toBeNull()
    expect(validateImageStudioPrompt('😀'.repeat(IMAGE_STUDIO_PROMPT_LIMIT + 1))).toBe('too_long')
  })

  it('grows textareas until the mobile viewport cap and then scrolls internally', () => {
    const textarea = document.createElement('textarea')
    Object.defineProperty(textarea, 'scrollHeight', { configurable: true, value: 900 })

    expect(resizeImageStudioTextarea(textarea, { mobile: true, viewportHeight: 1000 })).toBe(420)
    expect(textarea.style.height).toBe('420px')
    expect(textarea.style.overflowY).toBe('auto')

    Object.defineProperty(textarea, 'scrollHeight', { configurable: true, value: 180 })
    expect(resizeImageStudioTextarea(textarea, { mobile: true, viewportHeight: 1000 })).toBe(180)
    expect(textarea.style.overflowY).toBe('hidden')
  })

  it('uses the visual viewport height when a mobile keyboard shrinks it', () => {
    const descriptor = Object.getOwnPropertyDescriptor(window, 'visualViewport')
    Object.defineProperty(window, 'visualViewport', {
      configurable: true,
      value: { height: 400 },
    })
    const textarea = document.createElement('textarea')
    Object.defineProperty(textarea, 'scrollHeight', { configurable: true, value: 900 })

    expect(resizeImageStudioTextarea(textarea, { mobile: true })).toBe(168)
    expect(textarea.style.height).toBe('168px')

    if (descriptor) Object.defineProperty(window, 'visualViewport', descriptor)
    else delete (window as { visualViewport?: VisualViewport }).visualViewport
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

  it('loads every API key page and keeps only active keys', async () => {
    const loadPage = vi.fn(async (page: number) => ({
      items: page === 1
        ? [
            { id: 10, name: 'Disabled', status: 'inactive' },
            { id: 11, name: 'First active', status: 'active' },
          ]
        : [{ id: 12, name: 'Later active', status: 'active' }],
      pages: 2,
    }))

    await expect(loadAllActiveImageStudioKeys(loadPage)).resolves.toEqual([
      { id: 11, name: 'First active' },
      { id: 12, name: 'Later active' },
    ])
    expect(loadPage.mock.calls).toEqual([[1, 100], [2, 100]])
  })
})
