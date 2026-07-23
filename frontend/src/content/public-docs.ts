/** Public /docs route tree — cat & page ids match ?cat=&page= query params. */
export {
  PUBLIC_DOC_CONTENT_ZH,
  findDocContent,
  defaultDocPageId,
  type PublicDocCategoryContent,
  type PublicDocPageContent,
} from './public-docs-data.zh'
export { PUBLIC_DOC_CONTENT_EN, findDocContentEn, defaultDocPageIdEn } from './public-docs-data.en'

import { PUBLIC_DOC_CONTENT_ZH, defaultDocPageId, findDocContent } from './public-docs-data.zh'
import { PUBLIC_DOC_CONTENT_EN, defaultDocPageIdEn, findDocContentEn } from './public-docs-data.en'

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

export const PUBLIC_DOC_TREE_EN: PublicDocCategory[] = PUBLIC_DOC_CONTENT_EN.map((cat) => ({
  id: cat.id,
  categoryKey: cat.id,
  pages: cat.pages.map((p) => ({ id: p.id })),
}))

export type PublicDocLocale = 'en' | 'zh'

export function publicDocContentForLocale(locale: PublicDocLocale | string) {
  return locale === 'en' ? PUBLIC_DOC_CONTENT_EN : PUBLIC_DOC_CONTENT_ZH
}

export function publicDocTreeForLocale(locale: PublicDocLocale | string) {
  return locale === 'en' ? PUBLIC_DOC_TREE_EN : PUBLIC_DOC_TREE
}

export function findDocCategory(catId: string) {
  return PUBLIC_DOC_TREE.find((c) => c.id === catId)
}

export function findDocPage(catId: string, pageId: string) {
  return findDocContent(catId, pageId)
}

export function findDocContentForLocale(locale: PublicDocLocale | string, catId: string, pageId: string) {
  return locale === 'en' ? findDocContentEn(catId, pageId) : findDocContent(catId, pageId)
}

export function defaultDocPageForCategory(catId: string) {
  return defaultDocPageId(catId)
}

export function defaultDocPageForLocale(locale: PublicDocLocale | string, catId: string) {
  return locale === 'en' ? defaultDocPageIdEn(catId) : defaultDocPageId(catId)
}

const LEGACY_TUTORIAL_PAGE_CATEGORIES: Record<string, string> = {
  'text-to-image-api': 'deploy',
  'batch-image-api': 'deploy',
  'async-image-tasks': 'deploy',
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
