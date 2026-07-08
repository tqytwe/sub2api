/** Public /docs route tree — cat & page ids match ?cat=&page= query params. */
export interface PublicDocPage {
  id: string
}

export interface PublicDocCategory {
  id: string
  /** i18n key under docs.categories.{id} */
  categoryKey: string
  pages: PublicDocPage[]
}

export const PUBLIC_DOC_TREE: PublicDocCategory[] = [
  {
    id: 'tutorial',
    categoryKey: 'tutorial',
    pages: [{ id: 'quickstart' }, { id: 'api-key' }, { id: 'first-request' }],
  },
  {
    id: 'vip',
    categoryKey: 'vip',
    pages: [{ id: 'recharge' }, { id: 'vip-tiers' }, { id: 'check-in' }],
  },
  {
    id: 'deploy',
    categoryKey: 'deploy',
    pages: [{ id: 'sdk-quick' }, { id: 'claude-code' }, { id: 'codex-cli' }, { id: 'gemini-cli' }],
  },
  {
    id: 'models',
    categoryKey: 'models',
    pages: [{ id: 'overview' }, { id: 'selection' }],
  },
  {
    id: 'channels',
    categoryKey: 'channels',
    pages: [{ id: 'available' }, { id: 'pricing' }],
  },
  {
    id: 'about',
    categoryKey: 'about',
    pages: [{ id: 'privacy' }, { id: 'integrity' }],
  },
]

export function findDocCategory(catId: string) {
  return PUBLIC_DOC_TREE.find((c) => c.id === catId)
}

export function findDocPage(catId: string, pageId: string) {
  const cat = findDocCategory(catId)
  return cat?.pages.find((p) => p.id === pageId)
}

export function defaultDocPageForCategory(catId: string) {
  return findDocCategory(catId)?.pages[0]?.id
}
