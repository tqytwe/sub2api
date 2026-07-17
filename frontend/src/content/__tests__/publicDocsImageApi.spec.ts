import { describe, expect, it } from 'vitest'
import {
  PUBLIC_DOC_CONTENT_ZH,
  normalizePublicDocLocation,
} from '@/content/public-docs'

const imageApiPageIds = ['text-to-image-api', 'batch-image-api', 'async-image-tasks']

describe('public image API documentation placement', () => {
  it('places image API guides under deployment instead of tutorials', () => {
    const tutorial = PUBLIC_DOC_CONTENT_ZH.find((category) => category.id === 'tutorial')
    const deploy = PUBLIC_DOC_CONTENT_ZH.find((category) => category.id === 'deploy')

    expect(tutorial?.pages.map((page) => page.id)).not.toEqual(
      expect.arrayContaining(imageApiPageIds),
    )
    expect(deploy?.pages.slice(0, imageApiPageIds.length).map((page) => page.id)).toEqual(
      imageApiPageIds,
    )
  })

  it.each(imageApiPageIds)('redirects the legacy tutorial link for %s', (pageId) => {
    expect(normalizePublicDocLocation('tutorial', pageId)).toEqual({
      catId: 'deploy',
      pageId,
    })
  })

  it('documents the three real image response contracts and multipart boundary', () => {
    const imagePage = PUBLIC_DOC_CONTENT_ZH
      .find((category) => category.id === 'deploy')
      ?.pages.find((page) => page.id === 'text-to-image-api')
    const asyncPage = PUBLIC_DOC_CONTENT_ZH
      .find((category) => category.id === 'deploy')
      ?.pages.find((page) => page.id === 'async-image-tasks')

    expect(imagePage?.html).toContain('同步 Base64 返回')
    expect(imagePage?.html).toContain('同步 URL 返回')
    expect(imagePage?.html).toContain('data[].b64_json')
    expect(imagePage?.html).toContain('data[].url')
    expect(imagePage?.html).toContain('同步长连接 524')
    expect(imagePage?.html).toContain('每个 multipart 文件或文本字段最多 <strong>20 MiB</strong>')
    expect(imagePage?.html).toContain('413 invalid_request_error')

    expect(asyncPage?.html).toContain('/v1/images/generations/async')
    expect(asyncPage?.html).toContain('/v1/images/tasks/')
    expect(asyncPage?.html).toContain('默认使用服务器持久卷')
    expect(asyncPage?.html).toContain('/v1/images/task-assets/')
    expect(asyncPage?.html).toContain('"status": "processing"')
    expect(asyncPage?.html).toContain('"status": "completed"')
    expect(asyncPage?.html).toContain('"status": "failed"')
    expect(asyncPage?.html).toContain('成功提交和成功轮询响应都带')
  })
})
