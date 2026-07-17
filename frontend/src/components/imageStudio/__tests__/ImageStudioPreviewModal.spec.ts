import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ImageStudioAsset } from '@/api/imageStudio'
import ImageStudioPreviewModal from '@/components/imageStudio/ImageStudioPreviewModal.vue'

const fetchImageStudioAssetBlobMock = vi.hoisted(() => vi.fn())
const originalCreateObjectURL = window.URL.createObjectURL
const originalRevokeObjectURL = window.URL.revokeObjectURL
const createObjectURLMock = vi.fn<(blob: Blob) => string>()
const revokeObjectURLMock = vi.fn()

vi.mock('@/api/imageStudio', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/imageStudio')>()
  return {
    ...actual,
    default: {
      ...actual.default,
      fetchImageStudioAssetBlob: fetchImageStudioAssetBlobMock,
    },
  }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function abortableDeferred<T>(signal: AbortSignal) {
  const request = deferred<T>()
  const promise = new Promise<T>((resolve, reject) => {
    request.promise.then(resolve, reject)
    signal.addEventListener(
      'abort',
      () => reject(new DOMException('The operation was aborted.', 'AbortError')),
      { once: true },
    )
  })
  return { ...request, promise }
}

function managedAsset(id: string): ImageStudioAsset {
  return {
    id,
    sort_order: 0,
    preview_url: `/api/v1/image-studio/assets/${id}/content`,
  }
}

function mountPreview(asset: ImageStudioAsset) {
  return mount(ImageStudioPreviewModal, {
    attachTo: document.body,
    props: {
      asset,
      jobId: 'job-preview',
      index: 0,
    },
    global: {
      plugins: [createPinia()],
      stubs: { Icon: true },
    },
  })
}

describe('ImageStudioPreviewModal', () => {
  beforeEach(() => {
    fetchImageStudioAssetBlobMock.mockReset()
    createObjectURLMock.mockReset()
    revokeObjectURLMock.mockReset()
    window.URL.createObjectURL = createObjectURLMock as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = revokeObjectURLMock as typeof window.URL.revokeObjectURL
  })

  afterEach(() => {
    document.body.innerHTML = ''
    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
  })

  it('keeps asset B visible when the earlier asset A request resolves last', async () => {
    const requestA = deferred<Blob>()
    const requestB = deferred<Blob>()
    const blobA = new Blob(['asset-a'], { type: 'image/png' })
    const blobB = new Blob(['asset-b'], { type: 'image/png' })
    fetchImageStudioAssetBlobMock.mockImplementation((id: string) => (
      id === 'asset-a' ? requestA.promise : requestB.promise
    ))
    createObjectURLMock.mockImplementation((blob) => (
      blob === blobA ? 'blob:asset-a' : 'blob:asset-b'
    ))
    const wrapper = mountPreview(managedAsset('asset-a'))

    await wrapper.setProps({ asset: managedAsset('asset-b') })
    requestB.resolve(blobB)
    await flushPromises()
    expect(document.body.querySelector('img')?.getAttribute('src')).toBe('blob:asset-b')

    requestA.resolve(blobA)
    await flushPromises()

    expect(document.body.querySelector('img')?.getAttribute('src')).toBe('blob:asset-b')
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:asset-a')
    wrapper.unmount()
  })

  it('aborts the previous HTTP request immediately when switching assets', async () => {
    const requests = new Map<string, ReturnType<typeof abortableDeferred<Blob>>>()
    const signals = new Map<string, AbortSignal>()
    fetchImageStudioAssetBlobMock.mockImplementation((
      id: string,
      _mode: string,
      signal: AbortSignal,
    ) => {
      signals.set(id, signal)
      const request = abortableDeferred<Blob>(signal)
      requests.set(id, request)
      return request.promise
    })
    createObjectURLMock.mockReturnValue('blob:asset-b')
    const wrapper = mountPreview(managedAsset('asset-a'))
    await flushPromises()

    expect(signals.get('asset-a')?.aborted).toBe(false)
    await wrapper.setProps({ asset: managedAsset('asset-b') })

    expect(signals.get('asset-a')?.aborted).toBe(true)
    expect(signals.get('asset-b')?.aborted).toBe(false)
    requests.get('asset-b')?.resolve(new Blob(['asset-b'], { type: 'image/png' }))
    await flushPromises()
    expect(document.body.querySelector('img')?.getAttribute('src')).toBe('blob:asset-b')

    wrapper.unmount()
  })

  it('aborts the active HTTP request immediately when unmounting', async () => {
    let requestSignal: AbortSignal | undefined
    fetchImageStudioAssetBlobMock.mockImplementation((
      _id: string,
      _mode: string,
      signal: AbortSignal,
    ) => {
      requestSignal = signal
      return abortableDeferred<Blob>(signal).promise
    })
    const wrapper = mountPreview(managedAsset('asset-active'))
    await flushPromises()

    expect(requestSignal?.aborted).toBe(false)
    wrapper.unmount()

    expect(requestSignal?.aborted).toBe(true)
    expect(document.body.querySelector('img')).toBeNull()
  })

  it('revokes object URLs when replacing the preview and when unmounting', async () => {
    const blobA = new Blob(['asset-a'], { type: 'image/png' })
    const blobB = new Blob(['asset-b'], { type: 'image/png' })
    fetchImageStudioAssetBlobMock
      .mockResolvedValueOnce(blobA)
      .mockResolvedValueOnce(blobB)
    createObjectURLMock
      .mockReturnValueOnce('blob:asset-a')
      .mockReturnValueOnce('blob:asset-b')
    const wrapper = mountPreview(managedAsset('asset-a'))
    await flushPromises()

    await wrapper.setProps({ asset: managedAsset('asset-b') })
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:asset-a')
    await flushPromises()
    expect(document.body.querySelector('img')?.getAttribute('src')).toBe('blob:asset-b')

    wrapper.unmount()
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:asset-b')
  })

  it('revokes an object URL created by a response that arrives after unmount', async () => {
    const request = deferred<Blob>()
    fetchImageStudioAssetBlobMock.mockReturnValue(request.promise)
    createObjectURLMock.mockReturnValue('blob:late-preview')
    const wrapper = mountPreview(managedAsset('asset-late'))

    wrapper.unmount()
    request.resolve(new Blob(['late'], { type: 'image/png' }))
    await flushPromises()

    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:late-preview')
    expect(document.body.querySelector('img')).toBeNull()
  })
})
