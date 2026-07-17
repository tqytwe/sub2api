import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import { useImageStudioCapabilities } from '@/composables/useImageStudioCapabilities'
import type { ImageStudioCapabilities } from '@/api/imageStudio'

const mockCapabilities: ImageStudioCapabilities = {
  aspects: [{ id: '1:1', label: { zh: '1:1', en: '1:1' } }],
  tiers: [
    { id: '1K', label: { zh: '1K', en: '1K' } },
    { id: '2K', label: { zh: '2K', en: '2K' } },
  ],
  size_options: [
    { aspect: '1:1', tier: '1K', size: '1024x1024', billing_tier: '1K' },
    { aspect: '1:1', tier: '2K', size: '2048x2048', billing_tier: '2K' },
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

  it('does not invent a default size before capabilities are available', () => {
    const capabilities = ref<ImageStudioCapabilities | null>(null)
    const caps = useImageStudioCapabilities(() => capabilities.value, () => null)

    expect(caps.currentOption.value).toBeNull()
    expect(caps.resolvedSize.value).toBe('')
  })
})
