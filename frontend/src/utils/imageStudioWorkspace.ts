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
  return prompt.trim().length > 0
}
