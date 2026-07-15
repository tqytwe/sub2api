import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { keysAPI } from '@/api/keys'
import imageStudioAPI, {
  type ImageStudioAsset,
  type ImageStudioCapabilities,
  type ImageStudioCatalog,
  type ImageStudioEstimate,
  type ImageStudioIntent,
  type ImageStudioJob,
  type ImageStudioModelOption,
  type ImageStudioTemplate,
} from '@/api/imageStudio'
import playAPI from '@/api/play'
import { useAuthStore } from '@/stores/auth'
import { useImageStudioCapabilities } from '@/composables/useImageStudioCapabilities'
import {
  getStudioAutoCleanup,
  getStudioLastTemplate,
  hasStudioFirstWin,
  markStudioFirstWin,
  setStudioAutoCleanup,
  setStudioLastTemplate,
  trackGrowthEvent,
  trackQuestCompleteOnce,
} from '@/utils/growthAnalytics'
import {
  clearStudioPendingJobId,
  getStudioPendingJobId,
  setStudioPendingJobId,
} from '@/utils/imageStudioSession'

export function useImageStudioWizard() {
  const { t, locale } = useI18n()
  const router = useRouter()
  const authStore = useAuthStore()

  const bootstrapping = ref(true)
  const generating = ref(false)
  const polling = ref(false)
  const step = ref(1)
  const catalog = ref<ImageStudioCatalog | null>(null)
  const capabilities = ref<ImageStudioCapabilities | null>(null)
  const selectedIntent = ref<ImageStudioIntent | null>(null)
  const selectedTemplate = ref<ImageStudioTemplate | null>(null)
  const userPrompt = ref('')
  const accentColor = ref('#1a1a1a')
  const count = ref(1)
  const expertOpen = ref(false)
  const expertPrompt = ref('')
  const apiKeyId = ref<number | null>(null)
  const apiKeys = ref<Array<{ id: number; name: string }>>([])
  const availableModels = ref<ImageStudioModelOption[]>([])
  const selectedModel = ref('')
  const quality = ref('')
  const loadingModels = ref(false)
  const modelError = ref('')
  const estimateError = ref('')
  const estimate = ref<ImageStudioEstimate | null>(null)
  const jobs = ref<ImageStudioJob[]>([])
  const errorMsg = ref('')
  const pollNotice = ref('')
  const autoCleanup = ref(getStudioAutoCleanup())
  const showFirstWin = ref(false)
  const latestJob = ref<ImageStudioJob | null>(null)
  const totalRecharged = ref(0)
  const previewAsset = ref<ImageStudioAsset | null>(null)
  const previewJobId = ref('')
  const previewIndex = ref(0)

  const selectedModelOption = computed(() =>
    availableModels.value.find((m) => m.id === selectedModel.value) ?? null,
  )

  const sizeCaps = useImageStudioCapabilities(
    () => capabilities.value,
    () => selectedModelOption.value,
  )

  const size = sizeCaps.resolvedSize

  const isNewUser = computed(() => totalRecharged.value <= 0)
  const maxCount = computed(() => (isNewUser.value ? 1 : 4))
  const balance = computed(() => authStore.user?.balance ?? estimate.value?.balance ?? 0)
  const balanceLow = computed(() => balance.value <= 0)
  const hasApiKeys = computed(() => apiKeys.value.length > 0)
  const showAccentColor = computed(() => selectedIntent.value?.id === 'ecommerce')
  const showQuality = computed(() => (selectedModelOption.value?.supported_qualities?.length ?? 0) > 0)

  function labelFor(obj?: { zh: string; en: string }) {
    if (!obj) return ''
    return locale.value.startsWith('zh') ? obj.zh : obj.en
  }

  function applyQuickStart() {
    const lastId = getStudioLastTemplate()
    if (!lastId || !catalog.value) return false
    for (const intent of catalog.value.intents) {
      const tpl = intent.templates.find((x) => x.id === lastId)
      if (tpl) {
        selectedIntent.value = intent
        selectedTemplate.value = tpl
        sizeCaps.applyTemplateDefault(tpl.defaults.size, true)
        count.value = isNewUser.value ? 1 : Math.min(tpl.defaults.count, maxCount.value)
        step.value = 3
        return true
      }
    }
    return false
  }

  async function loadModels() {
    modelError.value = ''
    availableModels.value = []
    selectedModel.value = ''
    quality.value = ''
    if (!apiKeyId.value) return
    loadingModels.value = true
    try {
      const models = await imageStudioAPI.listImageStudioModels(apiKeyId.value)
      availableModels.value = models
      selectedModel.value = models[0]?.id ?? ''
      quality.value = models[0]?.default_quality ?? models[0]?.supported_qualities?.[0] ?? ''
      if (models[0]?.default_size) {
        sizeCaps.applyTemplateDefault(models[0].default_size, true)
      }
      sizeCaps.ensureSelectableTier()
      if (!models.length) modelError.value = t('imageStudio.noModels')
    } catch {
      modelError.value = t('imageStudio.loadModelsFailed')
    } finally {
      loadingModels.value = false
    }
  }

  async function refreshEstimate() {
    estimateError.value = ''
    if (!selectedTemplate.value || !apiKeyId.value || !selectedModel.value) {
      estimate.value = null
      return
    }
    try {
      estimate.value = await imageStudioAPI.estimateImageStudio({
        template_id: selectedTemplate.value.id,
        size: size.value,
        count: count.value,
        api_key_id: apiKeyId.value,
        model: selectedModel.value,
      })
    } catch {
      estimate.value = null
      estimateError.value = t('imageStudio.estimateFailed')
    }
  }

  async function refreshJobs() {
    try {
      jobs.value = await imageStudioAPI.listImageStudioJobs(12)
    } catch {
      // keep gallery on silent failure
    }
  }

  async function resumePendingJob(jobId: string) {
    polling.value = true
    pollNotice.value = t('imageStudio.polling')
    try {
      const job = await imageStudioAPI.pollImageStudioJob(jobId)
      if (job.status === 'failed') {
        errorMsg.value = job.error_message || t('imageStudio.generateFailed')
        await loadModels()
        sizeCaps.ensureSelectableTier()
        return
      }
      latestJob.value = job
      jobs.value = [job, ...jobs.value.filter((j) => j.id !== job.id)]
      step.value = 4
      await authStore.refreshUser()
    } catch (err: unknown) {
      const code = err instanceof Error ? err.message : ''
      if (code === 'IMAGE_STUDIO_POLL_TIMEOUT') {
        pollNotice.value = t('imageStudio.pollTimeout')
      } else {
        errorMsg.value = t('imageStudio.generateFailed')
      }
    } finally {
      polling.value = false
      clearStudioPendingJobId()
      void refreshJobs()
    }
  }

  async function load() {
    const isRefresh = !!catalog.value
    if (!isRefresh) bootstrapping.value = true
    errorMsg.value = ''
    try {
      const [tpl, caps, keyPage, jobList, hub, activeJob] = await Promise.all([
        imageStudioAPI.getImageStudioTemplates(),
        imageStudioAPI.getImageStudioCapabilities().catch(() => null),
        keysAPI.list(1, 20),
        imageStudioAPI.listImageStudioJobs(12).catch(() => []),
        playAPI.getPlayHub().catch(() => null),
        imageStudioAPI.getActiveImageStudioJob().catch(() => null),
      ])
      totalRecharged.value = hub?.growth?.total_recharged ?? 0
      catalog.value = tpl
      capabilities.value = caps
      apiKeys.value = (keyPage.items ?? []).map((k) => ({ id: k.id, name: k.name || `Key #${k.id}` }))
      if (apiKeys.value.length && !apiKeyId.value) apiKeyId.value = apiKeys.value[0].id
      jobs.value = jobList
      if (!isRefresh) applyQuickStart()
      await loadModels()

      const pendingId = getStudioPendingJobId()
      const resumeId = pendingId || (activeJob && (activeJob.status === 'pending' || activeJob.status === 'running') ? activeJob.id : null)
      if (resumeId) {
        await resumePendingJob(resumeId)
      }
    } catch {
      errorMsg.value = t('imageStudio.loadFailed')
    } finally {
      bootstrapping.value = false
    }
  }

  watch([selectedTemplate, size, count, apiKeyId, selectedModel], refreshEstimate)
  watch(apiKeyId, () => { void loadModels() })
  watch(maxCount, (max) => { if (count.value > max) count.value = max })
  watch(selectedModel, (modelId) => {
    const model = availableModels.value.find((m) => m.id === modelId)
    if (!model) return
    if (!quality.value || !(model.supported_qualities || []).includes(quality.value)) {
      quality.value = model.default_quality ?? model.supported_qualities?.[0] ?? ''
    }
    sizeCaps.ensureSelectableTier()
  })
  watch([sizeCaps.aspect, sizeCaps.tier], ([aspect, tier]) => {
    trackGrowthEvent('image_studio_size_change', { aspect, tier, resolved_size: size.value })
  })

  function pickIntent(intent: ImageStudioIntent) {
    trackGrowthEvent('image_studio_intent_select', { intent_id: intent.id })
    selectedIntent.value = intent
    selectedTemplate.value = intent.templates[0] ?? null
    if (selectedTemplate.value) {
      sizeCaps.applyTemplateDefault(selectedTemplate.value.defaults.size)
      count.value = isNewUser.value ? 1 : Math.min(selectedTemplate.value.defaults.count, maxCount.value)
    }
    step.value = 2
  }

  function pickTemplate(tpl: ImageStudioTemplate) {
    selectedTemplate.value = tpl
    sizeCaps.applyTemplateDefault(tpl.defaults.size)
    count.value = isNewUser.value ? 1 : Math.min(tpl.defaults.count, maxCount.value)
    step.value = 3
  }

  function goToStep(target: number) {
    if (polling.value || generating.value) return
    if (target < step.value) step.value = target
  }

  function goBack() {
    if (polling.value || generating.value) return
    if (step.value === 4) { step.value = 3; return }
    if (step.value === 3) { step.value = 2; return }
    if (step.value === 2) step.value = 1
  }

  function startOver() {
    if (polling.value || generating.value) return
    step.value = 1
    selectedIntent.value = null
    selectedTemplate.value = null
    latestJob.value = null
    errorMsg.value = ''
    pollNotice.value = ''
    sizeCaps.resetUserTouched()
  }

  function onAutoCleanupChange() {
    setStudioAutoCleanup(autoCleanup.value)
  }

  function openPreview(asset: ImageStudioAsset, jobId: string, index: number) {
    previewAsset.value = asset
    previewJobId.value = jobId
    previewIndex.value = index
  }

  function closePreview() {
    previewAsset.value = null
    previewJobId.value = ''
    previewIndex.value = 0
  }

  function regenerateFromJob(job: ImageStudioJob) {
    trackGrowthEvent('image_studio_regenerate_same', { template_id: job.template_id, size: job.size })
    if (!catalog.value) return
    for (const intent of catalog.value.intents) {
      const tpl = intent.templates.find((x) => x.id === job.template_id)
      if (!tpl) continue
      selectedIntent.value = intent
      selectedTemplate.value = tpl
      break
    }
    sizeCaps.setFromSize(job.size)
    sizeCaps.userTouchedSize.value = true
    count.value = Math.min(job.count, maxCount.value)
    errorMsg.value = ''
    pollNotice.value = ''
    step.value = 3
    void refreshEstimate()
  }

  async function generate() {
    if (!selectedTemplate.value || !apiKeyId.value || !selectedModel.value) return
    if (estimate.value && !estimate.value.sufficient) {
      trackGrowthEvent('image_studio_insufficient_balance', { balance: estimate.value.balance })
      router.push('/purchase?return=/image-studio')
      return
    }
    trackGrowthEvent('image_studio_generate_click', {
      template_id: selectedTemplate.value.id,
      estimated_cost: estimate.value?.estimated_cost,
      size: size.value,
    })
    generating.value = true
    polling.value = true
    errorMsg.value = ''
    pollNotice.value = t('imageStudio.polling')
    try {
      const result = await imageStudioAPI.generateImageStudio({
        template_id: selectedTemplate.value.id,
        user_prompt: userPrompt.value,
        accent_color: accentColor.value,
        size: size.value,
        aspect: sizeCaps.aspect.value,
        tier: sizeCaps.tier.value,
        quality: quality.value || undefined,
        count: count.value,
        model: selectedModel.value,
        expert_prompt: expertOpen.value ? expertPrompt.value : null,
        api_key_id: apiKeyId.value,
        retain_days: autoCleanup.value ? 7 : 0,
      })
      setStudioLastTemplate(selectedTemplate.value.id)
      setStudioPendingJobId(result.job.id)
      const job = await imageStudioAPI.pollImageStudioJob(result.job.id)
      clearStudioPendingJobId()
      if (job.status === 'failed') {
        errorMsg.value = job.error_message || t('imageStudio.generateFailed')
        await loadModels()
        sizeCaps.ensureSelectableTier()
        return
      }
      trackGrowthEvent('image_studio_generate_success', {
        template_id: job.template_id,
        actual_cost: job.actual_cost ?? job.estimated_cost,
        count: job.count,
        size: job.size,
      })
      trackQuestCompleteOnce('image_generate')
      latestJob.value = job
      jobs.value = [job, ...jobs.value.filter((j) => j.id !== job.id)]
      step.value = 4
      pollNotice.value = ''
      await authStore.refreshUser()
      if (!hasStudioFirstWin()) {
        markStudioFirstWin()
        showFirstWin.value = true
      }
    } catch (err: unknown) {
      clearStudioPendingJobId()
      const code = err instanceof Error ? err.message : ''
      if (code === 'IMAGE_STUDIO_POLL_TIMEOUT') {
        pollNotice.value = t('imageStudio.pollTimeout')
      } else {
        errorMsg.value = t('imageStudio.generateFailed')
      }
    } finally {
      generating.value = false
      polling.value = false
      void refreshJobs()
    }
  }

  async function removeJob(id: string) {
    await imageStudioAPI.deleteImageStudioJob(id)
    jobs.value = jobs.value.filter((j) => j.id !== id)
    if (latestJob.value?.id === id) latestJob.value = null
  }

  function onAspectChange(value: string) {
    sizeCaps.selectAspect(value)
  }

  function onTierChange(value: string) {
    sizeCaps.selectTier(value)
  }

  onMounted(load)

  return {
    bootstrapping,
    generating,
    polling,
    pollNotice,
    step,
    catalog,
    capabilities,
    selectedIntent,
    selectedTemplate,
    userPrompt,
    accentColor,
    size,
    aspect: sizeCaps.aspect,
    tier: sizeCaps.tier,
    count,
    expertOpen,
    expertPrompt,
    apiKeyId,
    apiKeys,
    availableModels,
    selectedModel,
    selectedModelOption,
    quality,
    loadingModels,
    modelError,
    estimateError,
    estimate,
    jobs,
    errorMsg,
    autoCleanup,
    showFirstWin,
    latestJob,
    balance,
    balanceLow,
    hasApiKeys,
    showAccentColor,
    showQuality,
    maxCount,
    isNewUser,
    previewAsset,
    previewJobId,
    previewIndex,
    labelFor,
    pickIntent,
    pickTemplate,
    goToStep,
    goBack,
    startOver,
    onAutoCleanupChange,
    openPreview,
    closePreview,
    regenerateFromJob,
    generate,
    removeJob,
    onAspectChange,
    onTierChange,
    load,
  }
}
