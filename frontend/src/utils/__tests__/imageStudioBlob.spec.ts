import { describe, expect, it } from 'vitest'
import { extensionForContentType, filenameForAsset, isStudioAssetApiPath } from '@/utils/imageStudioBlob'

describe('imageStudioBlob', () => {
  it('maps content types to extensions', () => {
    expect(extensionForContentType('image/jpeg')).toBe('.jpg')
    expect(extensionForContentType('image/webp')).toBe('.webp')
    expect(extensionForContentType('image/png')).toBe('.png')
  })

  it('builds filenames from job id and index', () => {
    expect(filenameForAsset('abcdef12-3456-7890-abcd-ef1234567890', 0, 'image/webp')).toBe(
      'image-studio-abcdef12-1.webp',
    )
  })

  it('detects studio asset api paths', () => {
    expect(isStudioAssetApiPath('/api/v1/image-studio/assets/uuid/content')).toBe(true)
    expect(isStudioAssetApiPath('https://cdn.example.com/a.png')).toBe(false)
  })
})
