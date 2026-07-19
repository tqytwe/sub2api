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
    const batchPage = PUBLIC_DOC_CONTENT_ZH
      .find((category) => category.id === 'deploy')
      ?.pages.find((page) => page.id === 'batch-image-api')

    expect(imagePage?.html).toContain('同步 Base64 返回')
    expect(imagePage?.html).toContain('同步 URL 返回')
    expect(imagePage?.html).toContain('data[].b64_json')
    expect(imagePage?.html).toContain('data[].url')
    expect(imagePage?.html).toContain('/v1/images/results/{result_id}/{index}')
    expect(imagePage?.html).toContain('requested_n')
    expect(imagePage?.html).toContain('requested_size')
    expect(imagePage?.html).toContain('UTF-8 BOM')
    expect(imagePage?.html).toContain('IMAGE_RESPONSE_FORMAT_INVALID')
    expect(imagePage?.html).toContain('同步长连接 524')
    expect(imagePage?.html).toContain('每个 multipart 文件或文本字段最多 <strong>20 MiB</strong>')
    expect(imagePage?.html).toContain('413 invalid_request_error')

    expect(asyncPage?.html).toContain('/v1/images/generations/async')
    expect(asyncPage?.html).toContain('/v1/images/tasks/')
    expect(asyncPage?.html).toContain('Redis 持久队列')
    expect(asyncPage?.html).toContain('/v1/images/task-assets/')
    expect(asyncPage?.html).toContain('"status": "queued"')
    expect(asyncPage?.html).toContain('"status": "processing"')
    expect(asyncPage?.html).toContain('"status": "completed"')
    expect(asyncPage?.html).toContain('"status": "failed"')
    expect(asyncPage?.html).toContain('Idempotency-Key')
    expect(asyncPage?.html).toContain('最多 255 字节')
    expect(asyncPage?.html).toContain('IMAGE_TASK_IDEMPOTENCY_KEY_INVALID')
    expect(asyncPage?.html).toContain('IMAGE_PROMPT_REQUIRED')
    expect(asyncPage?.html).toContain('IMAGE_ASYNC_NOT_READY')
    expect(asyncPage?.html).toContain('成功提交和成功轮询响应都带')
    expect(asyncPage?.html).toContain('processing 的任务会转为 failed')
    expect(asyncPage?.html).toContain('不会重放可能已经计费的上游请求')
    expect(asyncPage?.html).not.toContain('陈旧 processing lease 会重新入队')

    expect(imagePage?.html).toContain('一次多张使用同步 Images')
    expect(imagePage?.html).toContain('n=1-10')
    expect(imagePage?.html).toContain('多个 prompt 使用 Batch Image')

    expect(batchPage?.html).toContain('GET  https://api.jisudeng.com/v1/images/batches/models')
    expect(batchPage?.html).toContain('POST https://api.jisudeng.com/v1/images/batches')
    expect(batchPage?.html).toContain('GET  https://api.jisudeng.com/v1/images/batches/{id}/download')
    expect(batchPage?.html).toContain('DELETE https://api.jisudeng.com/v1/images/batches/{id}')
    expect(batchPage?.html).toContain('BATCH_IMAGE_NOT_READY')
    expect(batchPage?.html).toContain('BATCH_IMAGE_DISABLED')
    expect(batchPage?.html).toContain('5 个独立 items')
    expect(batchPage?.html).toContain('10 个独立 items')
    expect(batchPage?.html).toContain('output_count')
    expect(batchPage?.html).toContain('output_image_count')
    expect(batchPage?.html).toContain('单 item 最多 4')
  })
})
