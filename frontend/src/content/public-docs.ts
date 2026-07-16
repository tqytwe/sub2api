/** Public /docs route tree — cat & page ids match ?cat=&page= query params. */
export {
  PUBLIC_DOC_CONTENT_ZH,
  findDocContent,
  defaultDocPageId,
  type PublicDocCategoryContent,
  type PublicDocPageContent,
} from './public-docs-data.zh'

import { PUBLIC_DOC_CONTENT_ZH, defaultDocPageId, findDocContent } from './public-docs-data.zh'

export interface PublicDocPage {
  id: string
}

export interface PublicDocCategory {
  id: string
  categoryKey: string
  pages: PublicDocPage[]
}

/** Route tree derived from doc content (single source of truth). */
export const PUBLIC_DOC_TREE: PublicDocCategory[] = PUBLIC_DOC_CONTENT_ZH.map((cat) => ({
  id: cat.id,
  categoryKey: cat.id,
  pages: cat.pages.map((p) => ({ id: p.id })),
}))

export function findDocCategory(catId: string) {
  return PUBLIC_DOC_TREE.find((c) => c.id === catId)
}

export function findDocPage(catId: string, pageId: string) {
  return findDocContent(catId, pageId)
}

export function defaultDocPageForCategory(catId: string) {
  return defaultDocPageId(catId)
}

const LEGACY_TUTORIAL_PAGE_CATEGORIES: Record<string, string> = {
  'text-to-image-api': 'deploy',
  'batch-image-api': 'deploy',
}

export function normalizePublicDocLocation(catId: string, pageId: string) {
  if (catId === 'tutorial' && LEGACY_TUTORIAL_PAGE_CATEGORIES[pageId]) {
    return {
      catId: LEGACY_TUTORIAL_PAGE_CATEGORIES[pageId],
      pageId,
    }
  }
  return { catId, pageId }
}
