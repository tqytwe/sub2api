import type {
  ImageStudioCatalog,
  ImageStudioIntent,
  ImageStudioModelOption,
  ImageStudioTemplate,
} from '@/api/imageStudio'

export interface ImageStudioTemplateSelection {
  intent: ImageStudioIntent
  template: ImageStudioTemplate
}

export interface ImageStudioKeyOption {
  id: number
  name: string
}

export interface ImageStudioKeySelection {
  key: ImageStudioKeyOption
  models: ImageStudioModelOption[]
}

export interface ImageStudioKeyPage {
  items: Array<ImageStudioKeyOption & { status: string }>
  pages: number
}

export const IMAGE_STUDIO_PROMPT_LIMIT = 8000

export type ImageStudioPromptError = 'required' | 'too_long'

export function countImageStudioCodePoints(value: string): number {
  return Array.from(value).length
}

export function validateImageStudioPrompt(
  prompt: string,
  options: { required?: boolean } = {},
): ImageStudioPromptError | null {
  if (!prompt.trim()) return options.required === false ? null : 'required'
  if (countImageStudioCodePoints(prompt) > IMAGE_STUDIO_PROMPT_LIMIT) return 'too_long'
  return null
}

export function resizeImageStudioTextarea(
  textarea: HTMLTextAreaElement,
  options: { mobile?: boolean; viewportHeight?: number; desktopMaxHeight?: number } = {},
): number {
  const mobile = options.mobile
    ?? (typeof window !== 'undefined' && window.matchMedia('(max-width: 1023px)').matches)
  const viewportHeight = options.viewportHeight
    ?? (
      typeof window !== 'undefined'
        ? (window.visualViewport?.height || window.innerHeight)
        : 800
    )
  const desktopMaxHeight = options.desktopMaxHeight ?? 320
  const maxHeight = mobile ? Math.floor(viewportHeight * 0.42) : desktopMaxHeight

  textarea.style.height = 'auto'
  const height = Math.min(textarea.scrollHeight, maxHeight)
  textarea.style.height = `${height}px`
  textarea.style.overflowY = textarea.scrollHeight > maxHeight ? 'auto' : 'hidden'
  return height
}

export async function loadAllActiveImageStudioKeys(
  loadPage: (page: number, pageSize: number) => Promise<ImageStudioKeyPage>,
): Promise<ImageStudioKeyOption[]> {
  const pageSize = 100
  const keys: ImageStudioKeyOption[] = []
  let page = 1
  let pages = 1
  do {
    const result = await loadPage(page, pageSize)
    keys.push(...result.items
      .filter((key) => key.status === 'active')
      .map((key) => ({ id: key.id, name: key.name || `Key #${key.id}` })))
    pages = Math.max(1, result.pages)
    page += 1
  } while (page <= pages)
  return keys
}

export async function findFirstImageStudioKeyWithModels(
  keys: ImageStudioKeyOption[],
  loadModels: (keyId: number) => Promise<ImageStudioModelOption[]>,
): Promise<ImageStudioKeySelection | null> {
  for (const key of keys) {
    try {
      const models = await loadModels(key.id)
      if (models.length > 0) return { key, models }
    } catch {
      // A key may belong to a group where image generation is unavailable.
    }
  }
  return null
}

export function flattenImageStudioTemplates(catalog: ImageStudioCatalog | null): ImageStudioTemplateSelection[] {
  return (catalog?.intents ?? []).flatMap((intent) =>
    intent.templates.map((template) => ({ intent, template })),
  )
}

export function resolveInitialImageStudioTemplate(
  catalog: ImageStudioCatalog | null,
  preferredTemplateId?: string | null,
): ImageStudioTemplateSelection | null {
  const options = flattenImageStudioTemplates(catalog)
  if (!options.length) return null
  return options.find((option) => option.template.id === preferredTemplateId) ?? options[0]
}

export function isImageStudioPromptValid(prompt: string): boolean {
  return validateImageStudioPrompt(prompt) === null
}
