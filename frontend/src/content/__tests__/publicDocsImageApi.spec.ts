import { describe, expect, it } from 'vitest'
import {
  PUBLIC_DOC_CONTENT_ZH,
  normalizePublicDocLocation,
} from '@/content/public-docs'

const imageApiPageIds = ['text-to-image-api', 'batch-image-api']

describe('public image API documentation placement', () => {
  it('places image API guides under deployment instead of tutorials', () => {
    const tutorial = PUBLIC_DOC_CONTENT_ZH.find((category) => category.id === 'tutorial')
    const deploy = PUBLIC_DOC_CONTENT_ZH.find((category) => category.id === 'deploy')

    expect(tutorial?.pages.map((page) => page.id)).not.toEqual(
      expect.arrayContaining(imageApiPageIds),
    )
    expect(deploy?.pages.slice(0, 2).map((page) => page.id)).toEqual(imageApiPageIds)
  })

  it.each(imageApiPageIds)('redirects the legacy tutorial link for %s', (pageId) => {
    expect(normalizePublicDocLocation('tutorial', pageId)).toEqual({
      catId: 'deploy',
      pageId,
    })
  })
})
