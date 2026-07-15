import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ImageStudioSizePicker from '@/components/imageStudio/ImageStudioSizePicker.vue'
import type { ImageStudioCapabilities } from '@/api/imageStudio'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
    locale: { value: 'zh-CN' },
  }),
}))

const capabilities: ImageStudioCapabilities = {
  aspects: [
    { id: '1:1', label: { zh: '正方', en: 'Square' } },
    { id: '3:2', label: { zh: '横版', en: 'Landscape' } },
  ],
  tiers: [
    { id: '1K', label: { zh: '标准', en: 'Standard' } },
    { id: '2K', label: { zh: '高清', en: 'HD' } },
  ],
  size_options: [
    { aspect: '1:1', tier: '1K', size: '1024x1024', billing_tier: '1K' },
    { aspect: '1:1', tier: '2K', size: '2048x2048', billing_tier: '2K' },
    { aspect: '3:2', tier: '1K', size: '1536x1024', billing_tier: '1K' },
  ],
}

describe('ImageStudioSizePicker', () => {
  it('emits a canonical aspect selection', async () => {
    const wrapper = mount(ImageStudioSizePicker, {
      props: { capabilities, aspect: '1:1', tier: '1K' },
    })

    await wrapper.get('button[title="横版"]').trigger('click')
    expect(wrapper.emitted('update:aspect')).toEqual([['3:2']])
  })

  it('disables aspects unsupported by the selected model', () => {
    const wrapper = mount(ImageStudioSizePicker, {
      props: {
        capabilities,
        aspect: '1:1',
        tier: '1K',
        selectedModel: {
          id: 'square-only',
          display_name: 'Square only',
          supported_sizes: ['1024x1024'],
        },
      },
    })

    expect(wrapper.get('button[title="imageStudio.optionUnsupported"]').attributes('disabled')).toBeDefined()
  })
})
