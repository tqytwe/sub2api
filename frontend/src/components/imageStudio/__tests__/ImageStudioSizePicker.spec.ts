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

  it('renders a provider-specific fixed-size list outside the shared matrix', async () => {
    const wrapper = mount(ImageStudioSizePicker, {
      props: {
        capabilities,
        aspect: '1:1',
        tier: '1K',
        size: '2752x1536',
        selectedModel: {
          id: 'sensenova-u1-fast',
          display_name: 'SenseNova U1 Fast',
          sizing_kind: 'fixed',
          supported_sizes: ['1664x2496', '2752x1536'],
        },
      },
    })

    const select = wrapper.get('[data-testid="image-size-select"]')
    expect(select.text()).toContain('1664x2496')
    expect(select.text()).toContain('2752x1536')
    await select.setValue('1664x2496')
    expect(wrapper.emitted('update:size')).toEqual([['1664x2496']])
  })

  it('emits a combined custom dimension value for constrained models', async () => {
    const wrapper = mount(ImageStudioSizePicker, {
      props: {
        capabilities,
        aspect: '1:1',
        tier: '1K',
        size: '2048x2048',
        selectedModel: {
          id: 'sensenova-u1.5-lite',
          display_name: 'SenseNova U1.5 Lite',
          sizing_kind: 'custom_dimensions',
          min_dimension: 512,
          max_dimension: 4096,
          dimension_step: 32,
          max_aspect_ratio: 3,
        },
      },
    })

    await wrapper.get('[data-testid="image-size-width"]').setValue('2720')
    await wrapper.get('[data-testid="image-size-height"]').setValue('1536')
    expect(wrapper.emitted('update:size')?.at(-1)).toEqual(['2720x1536'])
  })
})
