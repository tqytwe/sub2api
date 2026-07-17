import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ImageStudioGallery from '@/components/imageStudio/ImageStudioGallery.vue'
import type { ImageStudioJob } from '@/api/imageStudio'

const fetchImageStudioAssetBlobMock = vi.hoisted(() => vi.fn())
const downloadImageStudioJobMock = vi.hoisted(() => vi.fn())
const originalCreateObjectURL = window.URL.createObjectURL
const originalRevokeObjectURL = window.URL.revokeObjectURL
const originalIntersectionObserver = globalThis.IntersectionObserver
const createObjectURLMock = vi.fn(() => 'blob:managed-thumb')
const revokeObjectURLMock = vi.fn()

function deferredBlob() {
  let resolve!: (blob: Blob) => void
  const promise = new Promise<Blob>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

vi.mock('@/api/imageStudio', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/imageStudio')>()
  return {
    ...actual,
    default: {
      ...actual.default,
      fetchImageStudioAssetBlob: fetchImageStudioAssetBlobMock,
      downloadImageStudioJob: downloadImageStudioJobMock,
    },
  }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

const failedJob: ImageStudioJob = {
  id: 'job-1',
  template_id: 'free-create',
  size: '1024x1024',
  count: 1,
  status: 'failed',
  estimated_cost: 0.08,
  error_message: 'provider rejected the request',
  created_at: '2026-07-15T00:00:00Z',
  assets: [],
}

const managedAssetJob: ImageStudioJob = {
  id: 'job-managed',
  template_id: 'free-create',
  size: '1024x1024',
  count: 1,
  status: 'completed',
  estimated_cost: 0.08,
  created_at: '2026-07-15T00:00:00Z',
  assets: [{
    id: 'asset-managed',
    sort_order: 0,
    thumbnail_url: '/api/v1/image-studio/assets/asset-managed/thumbnail',
    preview_url: '/api/v1/image-studio/assets/asset-managed/content',
  }],
}

const historicalManagedAssetJob: ImageStudioJob = {
  ...managedAssetJob,
  id: 'job-historical-managed',
  assets: [{
    id: 'asset-historical-managed',
    sort_order: 0,
    preview_url: '/api/v1/image-studio/assets/asset-historical-managed/content',
  }],
}

const runningJob: ImageStudioJob = {
  id: 'job-running',
  template_id: 'free-create',
  size: '1024x1024',
  count: 4,
  status: 'running',
  estimated_cost: 0.32,
  success_count: 1,
  fail_count: 1,
  created_at: '2026-07-16T00:00:00Z',
  assets: [],
}

const partialJob: ImageStudioJob = {
  id: 'job-partial',
  template_id: 'free-create',
  size: '1024x1024',
  count: 2,
  status: 'partial',
  estimated_cost: 0.16,
  success_count: 1,
  fail_count: 1,
  created_at: '2026-07-16T00:00:00Z',
  assets: [{
    id: 'asset-partial',
    sort_order: 0,
    url: 'https://cdn.example.com/partial.png',
    width: 1536,
    height: 1024,
    aspect_ratio: '3:2',
  }],
}

describe('ImageStudioGallery', () => {
  beforeEach(() => {
    fetchImageStudioAssetBlobMock.mockReset()
    downloadImageStudioJobMock.mockReset()
    downloadImageStudioJobMock.mockResolvedValue(undefined)
    createObjectURLMock.mockClear()
    revokeObjectURLMock.mockClear()
    window.URL.createObjectURL = createObjectURLMock as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = revokeObjectURLMock as typeof window.URL.revokeObjectURL
    globalThis.IntersectionObserver = undefined as unknown as typeof IntersectionObserver
  })

  afterEach(() => {
    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    globalThis.IntersectionObserver = originalIntersectionObserver
  })

  it('renders an explicit empty state', () => {
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [] },
      global: { plugins: [createPinia()] },
    })
    expect(wrapper.text()).toContain('imageStudio.galleryEmpty')
  })

  it('shows failure details and emits reuse', async () => {
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [failedJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })

    expect(wrapper.text()).toContain('provider rejected the request')
    await wrapper.get('button[aria-label="imageStudio.reuseSettings"]').trigger('click')
    expect(wrapper.emitted('regenerate')?.[0]).toEqual([failedJob])
  })

  it('does not assign a protected managed asset URL directly to an image', () => {
    fetchImageStudioAssetBlobMock.mockReturnValue(new Promise(() => {}))

    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [managedAssetJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })

    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).toContain('imageStudio.loadingPreview')
  })

  it('revokes managed thumbnail URLs when assets leave the gallery', async () => {
    fetchImageStudioAssetBlobMock.mockResolvedValue(new Blob(['image'], { type: 'image/png' }))
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [managedAssetJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })
    await flushPromises()

    await wrapper.setProps({ jobs: [] })
    await flushPromises()

    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:managed-thumb')
  })

  it('revokes a thumbnail that resolves after the gallery unmounts', async () => {
    let resolveBlob!: (blob: Blob) => void
    fetchImageStudioAssetBlobMock.mockReturnValue(new Promise<Blob>((resolve) => {
      resolveBlob = resolve
    }))
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [managedAssetJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })

    wrapper.unmount()
    resolveBlob(new Blob(['image'], { type: 'image/png' }))
    await flushPromises()

    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:managed-thumb')
  })

  it('waits for a protected thumbnail to enter the viewport before loading it', async () => {
    let callback!: IntersectionObserverCallback
    const observe = vi.fn()
    const unobserve = vi.fn()
    const disconnect = vi.fn()
    globalThis.IntersectionObserver = vi.fn((nextCallback: IntersectionObserverCallback) => {
      callback = nextCallback
      return { observe, unobserve, disconnect } as unknown as IntersectionObserver
    }) as unknown as typeof IntersectionObserver
    fetchImageStudioAssetBlobMock.mockResolvedValue(new Blob(['thumb'], { type: 'image/webp' }))

    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [managedAssetJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })

    expect(observe).toHaveBeenCalledTimes(1)
    expect(fetchImageStudioAssetBlobMock).not.toHaveBeenCalled()

    const target = observe.mock.calls[0][0] as Element
    callback([{ target, isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver)
    await flushPromises()

    expect(fetchImageStudioAssetBlobMock).toHaveBeenCalledWith('asset-managed', 'thumbnail')
    expect(wrapper.find('img[src="blob:managed-thumb"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('falls back to authenticated content for historical assets without thumbnails', async () => {
    fetchImageStudioAssetBlobMock.mockResolvedValue(new Blob(['original'], { type: 'image/png' }))

    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [historicalManagedAssetJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })
    await flushPromises()

    expect(fetchImageStudioAssetBlobMock).toHaveBeenCalledWith('asset-historical-managed', 'content')
    expect(wrapper.find('img[src="blob:managed-thumb"]').exists()).toBe(true)
  })

  it.each([
    ['a thumbnail request failure', new Error('thumbnail not found')],
    ['an empty thumbnail response', new Blob([], { type: 'image/webp' })],
    ['an invalid thumbnail response', new Blob(['{}'], { type: 'application/json' })],
  ])('falls back once to authenticated content after %s', async (_reason, thumbnailResult) => {
    if (thumbnailResult instanceof Error) {
      fetchImageStudioAssetBlobMock.mockRejectedValueOnce(thumbnailResult)
    } else {
      fetchImageStudioAssetBlobMock.mockResolvedValueOnce(thumbnailResult)
    }
    fetchImageStudioAssetBlobMock.mockResolvedValueOnce(
      new Blob(['original'], { type: 'image/png' }),
    )

    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [managedAssetJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })
    await flushPromises()

    expect(fetchImageStudioAssetBlobMock.mock.calls).toEqual([
      ['asset-managed', 'thumbnail'],
      ['asset-managed', 'content'],
    ])
    expect(wrapper.find('img[src="blob:managed-thumb"]').exists()).toBe(true)
  })

  it('attempts authenticated content only once when thumbnail and fallback both fail', async () => {
    fetchImageStudioAssetBlobMock
      .mockRejectedValueOnce(new Error('thumbnail not found'))
      .mockRejectedValueOnce(new Error('content unavailable'))

    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [managedAssetJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })
    await flushPromises()
    await wrapper.setProps({ jobs: [{ ...managedAssetJob }] })
    await flushPromises()

    expect(fetchImageStudioAssetBlobMock.mock.calls).toEqual([
      ['asset-managed', 'thumbnail'],
      ['asset-managed', 'content'],
    ])
    expect(wrapper.text()).toContain('imageStudio.previewFailed')
  })

  it('revokes a fallback object URL that resolves after unmount', async () => {
    const content = deferredBlob()
    fetchImageStudioAssetBlobMock
      .mockRejectedValueOnce(new Error('thumbnail not found'))
      .mockReturnValueOnce(content.promise)
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [managedAssetJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })
    await flushPromises()

    wrapper.unmount()
    content.resolve(new Blob(['original'], { type: 'image/png' }))
    await flushPromises()

    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:managed-thumb')
  })

  it('shows progress and cancellation for active jobs without exposing delete', async () => {
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [runningJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })

    expect(wrapper.text()).toContain('imageStudio.jobProgress')
    expect(wrapper.text()).toContain('imageStudio.itemCounts')
    expect(wrapper.find('button[aria-label="imageStudio.delete"]').exists()).toBe(false)

    await wrapper.get('button[aria-label="imageStudio.cancel"]').trigger('click')
    expect(wrapper.emitted('cancel')?.[0]).toEqual(['job-running'])
  })

  it('keeps successful assets visible at their real ratio with dimension copy', () => {
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [partialJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })

    expect(wrapper.text()).toContain('partial')
    expect(wrapper.find('img[src="https://cdn.example.com/partial.png"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="asset-frame"]').attributes('style')).toContain(
      'aspect-ratio: 1536 / 1024',
    )
    expect(wrapper.text()).toContain('imageStudio.assetDimensions')
    expect(wrapper.find('button[aria-label="imageStudio.delete"]').exists()).toBe(true)
  })

  it('prefers an asset aspect ratio over the legacy job size', () => {
    const aspectOnlyJob: ImageStudioJob = {
      ...partialJob,
      size: '1024x1024',
      assets: [{
        id: 'asset-aspect-only',
        sort_order: 0,
        url: 'https://cdn.example.com/wide.png',
        aspect_ratio: '16:9',
      }],
    }
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [aspectOnlyJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })

    expect(wrapper.get('[data-testid="asset-frame"]').attributes('style')).toContain(
      'aspect-ratio: 16 / 9',
    )
  })

  it('uses the loaded image natural dimensions instead of the legacy job size', async () => {
    const unknownDimensionsJob: ImageStudioJob = {
      ...partialJob,
      size: '1024x1024',
      assets: [{
        id: 'asset-natural-ratio',
        sort_order: 0,
        url: 'https://cdn.example.com/natural.png',
      }],
    }
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [unknownDimensionsJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })
    const imageElement = wrapper.get('img').element
    Object.defineProperty(imageElement, 'naturalWidth', { configurable: true, value: 1600 })
    Object.defineProperty(imageElement, 'naturalHeight', { configurable: true, value: 900 })

    await wrapper.get('img').trigger('load')

    expect(wrapper.get('[data-testid="asset-frame"]').attributes('style')).toContain(
      'aspect-ratio: 1600 / 900',
    )
    expect(wrapper.text()).toContain('imageStudio.assetDimensions')
  })

  it.each([
    ['completed', managedAssetJob],
    ['partial', partialJob],
  ])('downloads every %s job as a zip archive', async (_status, targetJob) => {
    const wrapper = mount(ImageStudioGallery, {
      props: { jobs: [targetJob] },
      global: {
        plugins: [createPinia()],
        stubs: { Icon: true },
      },
    })

    await wrapper.get('button[aria-label="imageStudio.downloadAll"]').trigger('click')

    expect(downloadImageStudioJobMock).toHaveBeenCalledWith(targetJob.id)
  })
})
