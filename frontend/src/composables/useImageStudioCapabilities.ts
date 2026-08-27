import { computed, ref } from 'vue'
import type { ImageStudioCapabilities, ImageStudioModelOption } from '@/api/imageStudio'

export function useImageStudioCapabilities(
  capabilities: () => ImageStudioCapabilities | null,
  selectedModel: () => ImageStudioModelOption | null | undefined,
) {
  const aspect = ref('1:1')
  const tier = ref('1K')
  const dedicatedSize = ref('')
  const customSize = ref('')
  const userTouchedSize = ref(false)

  const sizeOptions = computed(() => capabilities()?.size_options ?? [])

  const supportedSizeSet = computed(() => {
    const model = selectedModel()
    if (!model?.supported_sizes?.length) return null
    return new Set(model.supported_sizes)
  })

  const activeModel = computed(() => selectedModel() ?? null)
  const usesCustomDimensions = computed(() => (
    activeModel.value?.sizing_kind === 'custom_dimensions'
  ))
  const dedicatedSizeOptions = computed(() => activeModel.value?.supported_sizes ?? [])
  const usesDedicatedSizeList = computed(() => {
    if (usesCustomDimensions.value) return false
    const model = activeModel.value
    if (!model?.supported_sizes?.length) return false
    const matrixSizes = new Set(sizeOptions.value.map((option) => option.size))
    return model.sizing_kind === 'fixed' && model.supported_sizes.some((size) => !matrixSizes.has(size))
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

  function parseDimensions(value: string): { width: number; height: number } | null {
    const match = /^\s*(\d+)x(\d+)\s*$/i.exec(value)
    if (!match) return null
    const width = Number(match[1])
    const height = Number(match[2])
    if (!Number.isInteger(width) || !Number.isInteger(height)) return null
    return { width, height }
  }

  function isCustomSizeAllowed(value: string): boolean {
    const dimensions = parseDimensions(value)
    const model = activeModel.value
    if (!dimensions || !model || !usesCustomDimensions.value) return false
    const min = Number(model.min_dimension)
    const max = Number(model.max_dimension)
    const step = Number(model.dimension_step)
    const maxAspect = Number(model.max_aspect_ratio)
    if (!Number.isInteger(min) || !Number.isInteger(max) || !Number.isInteger(step) || step <= 0) return false
    if (dimensions.width < min || dimensions.height < min || dimensions.width > max || dimensions.height > max) return false
    if (dimensions.width % step !== 0 || dimensions.height % step !== 0) return false
    const aspect = Math.max(dimensions.width, dimensions.height) / Math.min(dimensions.width, dimensions.height)
    return Number.isFinite(aspect) && (!Number.isFinite(maxAspect) || maxAspect <= 0 || aspect <= maxAspect)
  }

  const customSizeValid = computed(() => !usesCustomDimensions.value || isCustomSizeAllowed(customSize.value))
  const sizeReady = computed(() => {
    if (usesCustomDimensions.value) return customSizeValid.value
    if (usesDedicatedSizeList.value) return dedicatedSizeOptions.value.includes(dedicatedSize.value)
    return !!currentOption.value && !currentOption.value.disabled
  })
  const resolvedSize = computed(() => {
    if (usesCustomDimensions.value) return customSizeValid.value ? customSize.value : ''
    if (usesDedicatedSizeList.value) return dedicatedSize.value
    return currentOption.value?.size ?? ''
  })

  function setFromSize(size: string) {
    if (usesCustomDimensions.value) {
      if (isCustomSizeAllowed(size)) customSize.value = size
      return
    }
    if (usesDedicatedSizeList.value) {
      if (dedicatedSizeOptions.value.includes(size)) dedicatedSize.value = size
      return
    }
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

  function selectDedicatedSize(next: string) {
    if (!dedicatedSizeOptions.value.includes(next)) return
    userTouchedSize.value = true
    dedicatedSize.value = next
  }

  function setCustomSize(next: string): boolean {
    if (!isCustomSizeAllowed(next)) return false
    userTouchedSize.value = true
    customSize.value = next
    return true
  }

  function ensureSelectableTier() {
    if (usesCustomDimensions.value) {
      const defaultSize = activeModel.value?.default_size ?? ''
      if (!isCustomSizeAllowed(customSize.value) && isCustomSizeAllowed(defaultSize)) {
        customSize.value = defaultSize
      }
      return
    }
    if (usesDedicatedSizeList.value) {
      const defaultSize = activeModel.value?.default_size ?? ''
      if (!dedicatedSizeOptions.value.includes(dedicatedSize.value)) {
        dedicatedSize.value = dedicatedSizeOptions.value.includes(defaultSize)
          ? defaultSize
          : dedicatedSizeOptions.value[0] ?? ''
      }
      return
    }
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
    dedicatedSize,
    customSize,
    userTouchedSize,
    sizeOptions,
    selectableOptions,
    currentOption,
    usesDedicatedSizeList,
    dedicatedSizeOptions,
    usesCustomDimensions,
    customSizeValid,
    sizeReady,
    resolvedSize,
    setFromSize,
    applyTemplateDefault,
    selectAspect,
    selectTier,
    selectDedicatedSize,
    setCustomSize,
    ensureSelectableTier,
    resetUserTouched,
  }
}
