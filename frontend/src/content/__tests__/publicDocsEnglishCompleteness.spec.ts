import { describe, expect, it } from 'vitest'

import {
  PUBLIC_DOC_CONTENT_EN,
  defaultDocPageIdEn,
  findDocContentEn,
} from '@/content/public-docs-data.en'
import { PUBLIC_DOC_CONTENT_ZH } from '@/content/public-docs-data.zh'
import { PUBLIC_DOC_TREE } from '@/content/public-docs-tree'

const CJK_RE = /[\u3400-\u9fff\uf900-\ufaff]/

function flatten(value: unknown): string {
  if (typeof value === 'string') return value
  if (Array.isArray(value)) return value.map(flatten).join('\n')
  if (value && typeof value === 'object') return Object.values(value).map(flatten).join('\n')
  return ''
}

describe('English public documentation coverage', () => {
  it('mirrors the Chinese documentation category and page structure', () => {
    expect(PUBLIC_DOC_CONTENT_EN.map((category) => category.id)).toEqual(
      PUBLIC_DOC_TREE.map((category) => category.id),
    )

    for (const treeCategory of PUBLIC_DOC_TREE) {
      const enCategory = PUBLIC_DOC_CONTENT_EN.find((category) => category.id === treeCategory.id)
      expect(enCategory?.pages.map((page) => page.id), treeCategory.id).toEqual(
        treeCategory.pages.map((page) => page.id),
      )
    }
  })

  it('keeps the neutral documentation tree aligned with the Chinese source content', () => {
    expect(PUBLIC_DOC_TREE.map((category) => category.id)).toEqual(
      PUBLIC_DOC_CONTENT_ZH.map((category) => category.id),
    )

    for (const zhCategory of PUBLIC_DOC_CONTENT_ZH) {
      const treeCategory = PUBLIC_DOC_TREE.find((category) => category.id === zhCategory.id)
      expect(treeCategory?.pages.map((page) => page.id), zhCategory.id).toEqual(
        zhCategory.pages.map((page) => page.id),
      )
    }
  })

  it('has a reachable English page for every mirrored article', () => {
    for (const category of PUBLIC_DOC_CONTENT_EN) {
      expect(defaultDocPageIdEn(category.id), category.id).toBe(category.pages[0]?.id)
      for (const page of category.pages) {
        expect(findDocContentEn(category.id, page.id), `${category.id}/${page.id}`).toBe(page)
      }
    }
  })

  it('does not leak CJK text into the generated English documentation payload', () => {
    expect(flatten(PUBLIC_DOC_CONTENT_EN)).not.toMatch(CJK_RE)
  })
})
