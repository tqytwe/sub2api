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
export {
  PUBLIC_DOC_TREE,
  PUBLIC_DOC_TREE_EN,
  normalizePublicDocLocation,
  type PublicDocCategory,
  type PublicDocLocale,
  type PublicDocPage,
} from './public-docs-tree'
import { PUBLIC_DOC_TREE, PUBLIC_DOC_TREE_EN, type PublicDocLocale } from './public-docs-tree'

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
