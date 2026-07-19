import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import { useImageStudioCapabilities } from '@/composables/useImageStudioCapabilities'
import type { ImageStudioCapabilities } from '@/api/imageStudio'

const mockCapabilities: ImageStudioCapabilities = {
  aspects: [{ id: '1:1', label: { zh: '1:1', en: '1:1' } }],
  tiers: [
    { id: '1K', label: { zh: '1K', en: '1K' } },
    { id: '2K', label: { zh: '2K', en: '2K' } },
    { id: '3K', label: { zh: '3K', en: '3K' } },
  ],
  size_options: [
    { aspect: '1:1', tier: '1K', size: '1024x1024', billing_tier: '1K' },
    { aspect: '1:1', tier: '2K', size: '2048x2048', billing_tier: '2K' },
    { aspect: '1:1', tier: '3K', size: '3072x3072', billing_tier: '4K' },
  ],
}

describe('useImageStudioCapabilities', () => {
  it('resolves size from aspect and tier', () => {
    const capabilities = ref(mockCapabilities)
    const caps = useImageStudioCapabilities(() => capabilities.value, () => null)
    caps.aspect.value = '1:1'
    caps.tier.value = '2K'
    expect(caps.resolvedSize.value).toBe('2048x2048')
  })

  it('does not override user-selected size when applying template default', () => {
    const capabilities = ref(mockCapabilities)
    const caps = useImageStudioCapabilities(() => capabilities.value, () => null)
    caps.tier.value = '2K'
    caps.userTouchedSize.value = true
    caps.applyTemplateDefault('1024x1024')
    expect(caps.tier.value).toBe('2K')
  })

  it('filters options by model supported sizes', () => {
    const capabilities = ref(mockCapabilities)
    const caps = useImageStudioCapabilities(() => capabilities.value, () => ({
      id: 'gpt-image-1',
      display_name: 'GPT Image 1',
      supported_sizes: ['1024x1024'],
    }))
    const hd = caps.selectableOptions.value.find((opt) => opt.size === '2048x2048')
    expect(hd?.disabled).toBe(true)
  })

  it('enables 3K only when the selected model supports the Agnes size', () => {
    const capabilities = ref(mockCapabilities)
    const generic = useImageStudioCapabilities(() => capabilities.value, () => ({
      id: 'gpt-image-2',
      display_name: 'GPT Image 2',
      supported_sizes: ['1024x1024', '2048x2048'],
    }))
    const agnes = useImageStudioCapabilities(() => capabilities.value, () => ({
      id: 'agnes-image-2.1-flash',
      display_name: 'Agnes Image 2.1 Flash',
      supported_sizes: ['1024x1024', '2048x2048', '3072x3072'],
    }))

    expect(generic.selectableOptions.value.find((opt) => opt.tier === '3K')?.disabled).toBe(true)
    expect(agnes.selectableOptions.value.find((opt) => opt.tier === '3K')?.disabled).toBe(false)
  })

  it('does not invent a default size before capabilities are available', () => {
    const capabilities = ref<ImageStudioCapabilities | null>(null)
    const caps = useImageStudioCapabilities(() => capabilities.value, () => null)

    expect(caps.currentOption.value).toBeNull()
    expect(caps.resolvedSize.value).toBe('')
  })
})
