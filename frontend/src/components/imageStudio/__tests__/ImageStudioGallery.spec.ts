import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ImageStudioGallery from '@/components/imageStudio/ImageStudioGallery.vue'
import type { ImageStudioJob } from '@/api/imageStudio'

const fetchImageStudioAssetBlobMock = vi.hoisted(() => vi.fn())
const originalCreateObjectURL = window.URL.createObjectURL
const originalRevokeObjectURL = window.URL.revokeObjectURL
const createObjectURLMock = vi.fn(() => 'blob:managed-thumb')
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
    preview_url: '/api/v1/image-studio/assets/asset-managed/content',
  }],
}

describe('ImageStudioGallery', () => {
  beforeEach(() => {
    fetchImageStudioAssetBlobMock.mockReset()
    createObjectURLMock.mockClear()
    revokeObjectURLMock.mockClear()
    window.URL.createObjectURL = createObjectURLMock as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = revokeObjectURLMock as typeof window.URL.revokeObjectURL
  })

  afterEach(() => {
    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
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
})
