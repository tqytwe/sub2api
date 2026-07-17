import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  cancelImageStudioJob,
  deleteImageStudioReference,
  downloadImageStudioJob,
  estimateImageStudio,
  fetchImageStudioAssetBlob,
  generateImageStudio,
  getActiveImageStudioJobs,
  listImageStudioJobs,
  pollImageStudioJob,
  uploadImageStudioReference,
  type ImageStudioJob,
} from '@/api/imageStudio'

const clientMocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  delete: vi.fn(),
}))
const blobMocks = vi.hoisted(() => ({
  saveBlob: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get: clientMocks.get,
    post: clientMocks.post,
    delete: clientMocks.delete,
  },
}))

vi.mock('@/utils/imageStudioBlob', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/imageStudioBlob')>()
  return {
    ...actual,
    saveBlob: blobMocks.saveBlob,
  }
})

const job: ImageStudioJob = {
  id: 'job-1',
  template_id: 'free-create',
  size: '1024x1024',
  count: 1,
  status: 'running',
  estimated_cost: 0.08,
  created_at: '2026-07-16T00:00:00Z',
}

describe('Image Studio jobs API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('normalizes active jobs from the multi-job contract and legacy first-job field', async () => {
    clientMocks.get.mockResolvedValueOnce({
      data: { jobs: [job, { ...job, id: 'job-2' }], job },
    })

    await expect(getActiveImageStudioJobs()).resolves.toEqual([
      job,
      { ...job, id: 'job-2' },
    ])

    clientMocks.get.mockResolvedValueOnce({ data: { job } })
    await expect(getActiveImageStudioJobs()).resolves.toEqual([job])
  })

  it('requests page metadata and preserves the paged jobs contract', async () => {
    const page = {
      jobs: [{
        ...job,
        assets: [{
          id: 'asset-1',
          sort_order: 0,
          width: 1536,
          height: 1024,
          aspect_ratio: '3:2',
          thumbnail_url: '/api/v1/image-studio/assets/asset-1/thumbnail',
        }],
      }],
      total: 25,
      page: 2,
      page_size: 12,
      pages: 3,
    }
    clientMocks.get.mockResolvedValueOnce({ data: page })

    await expect(listImageStudioJobs(2, 12)).resolves.toEqual(page)
    expect(clientMocks.get).toHaveBeenCalledWith('/image-studio/jobs', {
      params: { page: 2, page_size: 12 },
    })
  })

  it('normalizes legacy raw arrays and jobs-only envelopes into page metadata', async () => {
    clientMocks.get.mockResolvedValueOnce({ data: { jobs: [job] } })
    await expect(listImageStudioJobs(1, 12)).resolves.toEqual({
      jobs: [job],
      total: 1,
      page: 1,
      page_size: 12,
      pages: 1,
    })

    clientMocks.get.mockResolvedValueOnce({ data: [job] })
    await expect(listImageStudioJobs(1, 12)).resolves.toEqual({
      jobs: [job],
      total: 1,
      page: 1,
      page_size: 12,
      pages: 1,
    })
  })

  it('returns the updated job from cancellation', async () => {
    const cancelled = { ...job, status: 'cancelled' }
    clientMocks.post.mockResolvedValueOnce({ data: cancelled })

    await expect(cancelImageStudioJob('job-1')).resolves.toEqual(cancelled)
    expect(clientMocks.post).toHaveBeenCalledWith('/image-studio/jobs/job-1/cancel')
  })

  it('sends the caller-owned idempotency key when generating', async () => {
    const result = { job }
    const body = {
      template_id: 'free-create',
      user_prompt: 'a reliable request',
      api_key_id: 8,
    }
    clientMocks.post.mockResolvedValueOnce({ data: result })

    await expect(generateImageStudio(body, 'image-studio-stable-key')).resolves.toEqual(result)
    expect(clientMocks.post).toHaveBeenCalledWith(
      '/image-studio/generate',
      body,
      { headers: { 'Idempotency-Key': 'image-studio-stable-key' } },
    )
  })

  it('includes ready private reference ids in edit estimates', async () => {
    const estimate = {
      estimated_cost: 0.18,
      balance: 10,
      sufficient: true,
      model: 'gpt-image-1',
      count: 2,
      size: '1024x1024',
    }
    clientMocks.get.mockResolvedValueOnce({ data: estimate })

    await expect(estimateImageStudio({
      template_id: 'free-create',
      size: '1024x1024',
      count: 2,
      api_key_id: 8,
      model: 'gpt-image-1',
      reference_ids: ['ref-1', 'ref-2'],
    })).resolves.toEqual(estimate)

    expect(clientMocks.get).toHaveBeenCalledWith('/image-studio/estimate', {
      params: expect.objectContaining({
        reference_ids: ['ref-1', 'ref-2'],
      }),
    })
  })

  it('uploads a private reference as multipart form data with an abort signal', async () => {
    const reference = {
      id: 'ref-1',
      filename: 'reference.png',
      content_type: 'image/png',
      byte_size: 4,
      expires_at: '2026-07-23T00:00:00Z',
    }
    clientMocks.post.mockResolvedValueOnce({ data: { reference } })
    const file = new File(['test'], 'reference.png', { type: 'image/png' })
    const controller = new AbortController()

    await expect(uploadImageStudioReference(file, controller.signal)).resolves.toEqual(reference)

    expect(clientMocks.post).toHaveBeenCalledTimes(1)
    const [path, body, config] = clientMocks.post.mock.calls[0]
    expect(path).toBe('/image-studio/references')
    expect(body).toBeInstanceOf(FormData)
    expect((body as FormData).get('image')).toBe(file)
    expect(config).toEqual({ signal: controller.signal })
  })

  it('deletes an uploaded reference by its encoded id', async () => {
    clientMocks.delete.mockResolvedValueOnce({ data: null })

    await expect(deleteImageStudioReference('ref late')).resolves.toBeUndefined()
    expect(clientMocks.delete).toHaveBeenCalledWith('/image-studio/references/ref%20late')
  })

  it('fetches protected thumbnail blobs with the caller abort signal', async () => {
    const blob = new Blob(['thumb'], { type: 'image/webp' })
    clientMocks.get.mockResolvedValueOnce({ data: blob })
    const controller = new AbortController()

    await expect(
      fetchImageStudioAssetBlob('asset 1', 'thumbnail', controller.signal),
    ).resolves.toBe(blob)
    expect(clientMocks.get).toHaveBeenCalledWith(
      '/image-studio/assets/asset%201/thumbnail',
      { responseType: 'blob', signal: controller.signal },
    )
  })

  it('passes the abort signal into each job polling request', async () => {
    const completed = { ...job, status: 'completed' }
    clientMocks.get.mockResolvedValueOnce({ data: completed })
    const controller = new AbortController()

    await expect(
      pollImageStudioJob(job.id, { signal: controller.signal }),
    ).resolves.toEqual(completed)

    expect(clientMocks.get).toHaveBeenCalledWith(
      '/image-studio/jobs/job-1',
      { signal: controller.signal },
    )
  })

  it('aborts an interval wait immediately instead of waiting for the timer', async () => {
    vi.useFakeTimers()
    const controller = new AbortController()
    clientMocks.get.mockResolvedValueOnce({ data: job })
    let settled = false
    let rejection: unknown

    try {
      const polling = pollImageStudioJob(job.id, {
        intervalMs: 60_000,
        signal: controller.signal,
      })
      void polling.catch((error: unknown) => {
        settled = true
        rejection = error
      })

      await Promise.resolve()
      await Promise.resolve()
      expect(clientMocks.get).toHaveBeenCalledTimes(1)

      controller.abort()
      await Promise.resolve()
      await Promise.resolve()

      expect(settled).toBe(true)
      expect(rejection).toEqual(new Error('IMAGE_STUDIO_POLL_ABORTED'))
      expect(clientMocks.get).toHaveBeenCalledTimes(1)
    } finally {
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('downloads a completed job as a zip archive', async () => {
    const blob = new Blob(['zip'], { type: 'application/zip' })
    clientMocks.get.mockResolvedValueOnce({ data: blob })

    await downloadImageStudioJob('job 123456789')

    expect(clientMocks.get).toHaveBeenCalledWith(
      '/image-studio/jobs/job%20123456789/download',
      { responseType: 'blob' },
    )
    expect(blobMocks.saveBlob).toHaveBeenCalledWith(blob, 'image-studio-job 1234.zip')
  })
})
