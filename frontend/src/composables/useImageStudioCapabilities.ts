import { computed, ref } from 'vue'
import type { ImageStudioCapabilities, ImageStudioModelOption } from '@/api/imageStudio'

export function useImageStudioCapabilities(
  capabilities: () => ImageStudioCapabilities | null,
  selectedModel: () => ImageStudioModelOption | null | undefined,
) {
  const aspect = ref('1:1')
  const tier = ref('1K')
  const userTouchedSize = ref(false)

  const sizeOptions = computed(() => capabilities()?.size_options ?? [])

  const supportedSizeSet = computed(() => {
    const model = selectedModel()
    if (!model?.supported_sizes?.length) return null
    return new Set(model.supported_sizes)
  })

  const selectableOptions = computed(() => {
    const supported = supportedSizeSet.value
    return sizeOptions.value.map((opt) => ({
      ...opt,
      disabled: supported ? !supported.has(opt.size) : false,
    }))
  })

  const currentOption = computed(() => {
    return (
      selectableOptions.value.find((opt) => opt.aspect === aspect.value && opt.tier === tier.value) ??
      selectableOptions.value.find((opt) => !opt.disabled) ??
      null
    )
  })

  const resolvedSize = computed(() => currentOption.value?.size ?? '')

  function setFromSize(size: string) {
    const match = sizeOptions.value.find((opt) => opt.size === size)
    if (match) {
      aspect.value = match.aspect
      tier.value = match.tier
      return
    }
    aspect.value = '1:1'
    tier.value = '1K'
  }

  function applyTemplateDefault(size: string, force = false) {
    if (!force && userTouchedSize.value) return
    setFromSize(size || '1024x1024')
  }

  function selectAspect(next: string) {
    userTouchedSize.value = true
    aspect.value = next
    ensureSelectableTier()
  }

  function selectTier(next: string) {
    userTouchedSize.value = true
    tier.value = next
    ensureSelectableTier()
  }

  function ensureSelectableTier() {
    const current = selectableOptions.value.find(
      (opt) => opt.aspect === aspect.value && opt.tier === tier.value && !opt.disabled,
    )
    if (current) return
    const fallback = selectableOptions.value.find((opt) => opt.aspect === aspect.value && !opt.disabled)
    if (fallback) {
      tier.value = fallback.tier
      return
    }
    const any = selectableOptions.value.find((opt) => !opt.disabled)
    if (any) {
      aspect.value = any.aspect
      tier.value = any.tier
    }
  }

  function resetUserTouched() {
    userTouchedSize.value = false
  }

  return {
    aspect,
    tier,
    userTouchedSize,
    sizeOptions,
    selectableOptions,
    currentOption,
    resolvedSize,
    setFromSize,
    applyTemplateDefault,
    selectAspect,
    selectTier,
    ensureSelectableTier,
    resetUserTouched,
  }
}
