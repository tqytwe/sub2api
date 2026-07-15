import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import ImageStudioGallery from '@/components/imageStudio/ImageStudioGallery.vue'
import type { ImageStudioJob } from '@/api/imageStudio'

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

describe('ImageStudioGallery', () => {
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
})
