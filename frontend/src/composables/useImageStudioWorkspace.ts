import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { keysAPI } from '@/api/keys'
import imageStudioAPI, {
  type ImageStudioAsset,
  type ImageStudioCapabilities,
  type ImageStudioCatalog,
  type ImageStudioEstimate,
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
  clearStudioPromptDraft,
  clearStudioPendingJobForUser,
  getStudioPendingJobForUser,
  getStudioPendingJobSubmittedPrompt,
  loadStudioDraft,
  saveStudioDraft,
  setStudioPendingJobId,
  type ImageStudioSubmittedPrompt,
} from '@/utils/imageStudioSession'
import {
  findFirstImageStudioKeyWithModels,
  isImageStudioPromptValid,
  loadAllActiveImageStudioKeys,
  resolveInitialImageStudioTemplate,
  validateImageStudioPrompt,
} from '@/utils/imageStudioWorkspace'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'

export function useImageStudioWorkspace() {
  const { t, locale } = useI18n()
  const router = useRouter()
  const authStore = useAuthStore()

  const bootstrapping = ref(true)
  const generating = ref(false)
  const polling = ref(false)
  const catalog = ref<ImageStudioCatalog | null>(null)
  const capabilities = ref<ImageStudioCapabilities | null>(null)
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
  const galleryError = ref('')
  const errorMsg = ref('')
  const pollNotice = ref('')
  const autoCleanup = ref(getStudioAutoCleanup())
  const showFirstWin = ref(false)
  const latestJob = ref<ImageStudioJob | null>(null)
  const totalRecharged = ref(0)
  const previewAsset = ref<ImageStudioAsset | null>(null)
  const previewJobId = ref('')
  const previewIndex = ref(0)
  const draftReady = ref(false)
  let draftSaveTimer: ReturnType<typeof setTimeout> | null = null
  let pendingDraftSave: { userId: number; draft: ReturnType<typeof currentDraft> } | null = null

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
  const hasApiKeys = computed(() => apiKeys.value.length > 0)
  const showAccentColor = computed(() => selectedTemplate.value !== null)
  const showQuality = computed(() => (selectedModelOption.value?.supported_qualities?.length ?? 0) > 0)
  const promptError = computed(() => validateImageStudioPrompt(userPrompt.value))
  const expertPromptError = computed(() =>
    expertOpen.value
      ? validateImageStudioPrompt(expertPrompt.value, { required: false })
      : null,
  )
  const promptValid = computed(() => promptError.value === null)
  const expertPromptValid = computed(() => expertPromptError.value === null)
  const draftUserId = computed(() => authStore.user?.id ?? null)

  function labelFor(obj?: { zh: string; en: string }) {
    if (!obj) return ''
    return locale.value.startsWith('zh') ? obj.zh : obj.en
  }

  function applyQuickStart() {
    const lastId = getStudioLastTemplate()
    if (!lastId) return false
    const selection = resolveInitialImageStudioTemplate(catalog.value, lastId)
    if (!selection || selection.template.id !== lastId) return false
    selectedTemplate.value = selection.template
    sizeCaps.applyTemplateDefault(selection.template.defaults.size, true)
    count.value = isNewUser.value ? 1 : Math.min(selection.template.defaults.count, maxCount.value)
    return true
  }

  function applyDefaultTemplate() {
    const selection = resolveInitialImageStudioTemplate(catalog.value)
    if (!selection) return false
    selectedTemplate.value = selection.template
    sizeCaps.applyTemplateDefault(selection.template.defaults.size, true)
    count.value = isNewUser.value ? 1 : Math.min(selection.template.defaults.count, maxCount.value)
    return true
  }

  function currentDraft() {
    return {
      userPrompt: userPrompt.value,
      expertPrompt: expertPrompt.value,
      expertOpen: expertOpen.value,
      templateId: selectedTemplate.value?.id ?? null,
      accentColor: accentColor.value,
      aspect: sizeCaps.aspect.value,
      tier: sizeCaps.tier.value,
      count: count.value,
    }
  }

  function persistDraft() {
    flushScheduledDraft()
    const userId = draftUserId.value
    if (!draftReady.value || !userId) return
    saveStudioDraft(userId, currentDraft())
  }

  function flushScheduledDraft() {
    if (draftSaveTimer) clearTimeout(draftSaveTimer)
    draftSaveTimer = null
    if (!pendingDraftSave) return
    saveStudioDraft(pendingDraftSave.userId, pendingDraftSave.draft)
    pendingDraftSave = null
  }

  function scheduleDraftPersist() {
    const userId = draftUserId.value
    if (!draftReady.value || !userId) return
    pendingDraftSave = { userId, draft: currentDraft() }
    if (draftSaveTimer) clearTimeout(draftSaveTimer)
    draftSaveTimer = setTimeout(flushScheduledDraft, 300)
  }

  function restoreDraft() {
    const userId = draftUserId.value
    if (!userId) return false
    const draft = loadStudioDraft(userId)
    if (!draft) return false

    const template = resolveInitialImageStudioTemplate(catalog.value, draft.templateId)
    if (template && template.template.id === draft.templateId) {
      selectedTemplate.value = template.template
    }
    userPrompt.value = draft.userPrompt
    expertPrompt.value = draft.expertPrompt
    expertOpen.value = draft.expertOpen
    accentColor.value = draft.accentColor
    sizeCaps.aspect.value = draft.aspect
    sizeCaps.tier.value = draft.tier
    sizeCaps.userTouchedSize.value = true
    count.value = Math.min(maxCount.value, Math.max(1, draft.count))
    return true
  }

  function clearPromptDraftAfterSuccess(
    submittedUserId: number,
    submittedPrompt: ImageStudioSubmittedPrompt | null,
  ) {
    flushScheduledDraft()
    if (!submittedPrompt) return
    if (!clearStudioPromptDraft(submittedUserId, submittedPrompt)) return
    if (
      draftUserId.value === submittedUserId
      && userPrompt.value === submittedPrompt.userPrompt
      && expertPrompt.value === submittedPrompt.expertPrompt
    ) {
      userPrompt.value = ''
      expertPrompt.value = ''
      persistDraft()
    }
  }

  function refreshUserAfterImageSuccess() {
    void Promise.resolve()
      .then(() => authStore.refreshUser())
      .catch(() => {})
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
      applyModels(models)
    } catch {
      modelError.value = t('imageStudio.loadModelsFailed')
    } finally {
      loadingModels.value = false
    }
  }

  function applyModels(models: ImageStudioModelOption[]) {
    availableModels.value = models
    selectedModel.value = models[0]?.id ?? ''
    quality.value = models[0]?.default_quality ?? models[0]?.supported_qualities?.[0] ?? ''
    if (models[0]?.default_size && !selectedTemplate.value) {
      sizeCaps.applyTemplateDefault(models[0].default_size, true)
    }
    sizeCaps.ensureSelectableTier()
    if (!models.length) modelError.value = t('imageStudio.noModels')
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
      galleryError.value = ''
    } catch (err: unknown) {
      galleryError.value = extractApiErrorMessage(err, t('imageStudio.galleryLoadFailed'))
    }
  }

  async function resumePendingJob(jobId: string) {
    const submittedUserId = draftUserId.value
    if (!submittedUserId) return
    const submittedPrompt = getStudioPendingJobSubmittedPrompt(submittedUserId, jobId)
    let reachedTerminalState = false
    polling.value = true
    pollNotice.value = t('imageStudio.polling')
    try {
      const job = await imageStudioAPI.pollImageStudioJob(jobId)
      reachedTerminalState = true
      if (job.status === 'failed') {
        errorMsg.value = job.error_message || t('imageStudio.generateFailed')
        await loadModels()
        sizeCaps.ensureSelectableTier()
        return
      }
      latestJob.value = job
      jobs.value = [job, ...jobs.value.filter((j) => j.id !== job.id)]
      if (!job.assets?.length) {
        errorMsg.value = t('imageStudio.assetsMissingHint')
        return
      }
      clearPromptDraftAfterSuccess(submittedUserId, submittedPrompt)
      refreshUserAfterImageSuccess()
    } catch (err: unknown) {
      const code = err instanceof Error ? err.message : ''
      if (code === 'IMAGE_STUDIO_POLL_TIMEOUT') {
        errorMsg.value = t('imageStudio.pollTimeout')
      } else {
        errorMsg.value = extractApiErrorMessage(err, t('imageStudio.generateFailed'))
      }
    } finally {
      polling.value = false
      if (reachedTerminalState) {
        clearStudioPendingJobForUser(submittedUserId, jobId)
      }
      void refreshJobs()
    }
  }

  async function load() {
    const isRefresh = !!catalog.value
    if (!isRefresh) bootstrapping.value = true
    errorMsg.value = ''
    try {
      const [tpl, caps, activeKeys, jobResult, hub, activeJob] = await Promise.all([
        imageStudioAPI.getImageStudioTemplates(),
        imageStudioAPI.getImageStudioCapabilities().catch(() => null),
        loadAllActiveImageStudioKeys((page, pageSize) =>
          keysAPI.list(page, pageSize, { status: 'active' })),
        imageStudioAPI.listImageStudioJobs(12)
          .then((value) => ({ value, error: null as unknown }))
          .catch((error: unknown) => ({ value: [] as ImageStudioJob[], error })),
        playAPI.getPlayHub().catch(() => null),
        imageStudioAPI.getActiveImageStudioJob().catch(() => null),
      ])
      totalRecharged.value = hub?.growth?.total_recharged ?? 0
      catalog.value = tpl
      capabilities.value = caps
      apiKeys.value = activeKeys
      let initialModels: ImageStudioModelOption[] | null = null
      if (apiKeys.value.length && !apiKeyId.value) {
        const selection = await findFirstImageStudioKeyWithModels(
          apiKeys.value,
          imageStudioAPI.listImageStudioModels,
        )
        apiKeyId.value = selection?.key.id ?? apiKeys.value[0].id
        initialModels = selection?.models ?? null
      }
      jobs.value = jobResult.value
      galleryError.value = jobResult.error
        ? extractApiErrorMessage(jobResult.error, t('imageStudio.galleryLoadFailed'))
        : ''
      if (!isRefresh && !applyQuickStart()) applyDefaultTemplate()
      if (!isRefresh) restoreDraft()
      if (initialModels) applyModels(initialModels)
      else await loadModels()

      const pendingId = draftUserId.value
        ? getStudioPendingJobForUser(draftUserId.value)?.jobId
        : null
      const resumeId = pendingId || (activeJob && (activeJob.status === 'pending' || activeJob.status === 'running') ? activeJob.id : null)
      if (resumeId) {
        await resumePendingJob(resumeId)
      }
    } catch {
      errorMsg.value = t('imageStudio.loadFailed')
    } finally {
      draftReady.value = true
      bootstrapping.value = false
    }
  }

  watch([selectedTemplate, size, count, apiKeyId, selectedModel], refreshEstimate)
  watch(apiKeyId, () => {
    if (!bootstrapping.value) void loadModels()
  })
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
  watch(
    [
      userPrompt,
      expertPrompt,
      expertOpen,
      selectedTemplate,
      accentColor,
      sizeCaps.aspect,
      sizeCaps.tier,
      count,
    ],
    scheduleDraftPersist,
  )

  function pickTemplate(tpl: ImageStudioTemplate) {
    for (const intent of catalog.value?.intents ?? []) {
      if (intent.templates.some((item) => item.id === tpl.id)) {
        trackGrowthEvent('image_studio_intent_select', { intent_id: intent.id })
        break
      }
    }
    trackGrowthEvent('image_studio_template_select', { template_id: tpl.id })
    selectedTemplate.value = tpl
    sizeCaps.applyTemplateDefault(tpl.defaults.size)
    count.value = isNewUser.value ? 1 : Math.min(tpl.defaults.count, maxCount.value)
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
      selectedTemplate.value = tpl
      break
    }
    sizeCaps.setFromSize(job.size)
    sizeCaps.userTouchedSize.value = true
    count.value = Math.min(job.count, maxCount.value)
    errorMsg.value = ''
    pollNotice.value = ''
    void refreshEstimate()
  }

  async function generate(): Promise<boolean> {
    if (!isImageStudioPromptValid(userPrompt.value)) {
      errorMsg.value = promptError.value === 'too_long'
        ? t('imageStudio.promptTooLong')
        : t('imageStudio.promptRequired')
      return false
    }
    if (!expertPromptValid.value) {
      errorMsg.value = t('imageStudio.expertPromptTooLong')
      return false
    }
    if (!selectedTemplate.value || !apiKeyId.value || !selectedModel.value) return false
    const submittedUserId = draftUserId.value
    if (!submittedUserId) return false
    if (estimate.value && !estimate.value.sufficient) {
      trackGrowthEvent('image_studio_insufficient_balance', { balance: estimate.value.balance })
      router.push('/purchase?return=/image-studio')
      return false
    }
    trackGrowthEvent('image_studio_generate_click', {
      template_id: selectedTemplate.value.id,
      estimated_cost: estimate.value?.estimated_cost,
      size: size.value,
    })
    generating.value = true
    errorMsg.value = ''
    pollNotice.value = ''
    const submittedPrompt: ImageStudioSubmittedPrompt = {
      userPrompt: userPrompt.value,
      expertPrompt: expertPrompt.value,
    }
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
        expert_prompt: expertOpen.value && expertPrompt.value.trim() ? expertPrompt.value : null,
        api_key_id: apiKeyId.value,
        retain_days: autoCleanup.value ? 7 : 0,
      })
      setStudioLastTemplate(selectedTemplate.value.id)
      setStudioPendingJobId(result.job.id, {
        userId: submittedUserId,
        submittedPrompt,
      })
      polling.value = true
      pollNotice.value = t('imageStudio.polling')
      const job = await imageStudioAPI.pollImageStudioJob(result.job.id)
      clearStudioPendingJobForUser(submittedUserId, result.job.id)
      if (job.status === 'failed') {
        errorMsg.value = job.error_message || t('imageStudio.generateFailed')
        trackGrowthEvent('image_studio_generate_fail', {
          template_id: selectedTemplate.value.id,
          reason: job.error_message || 'job_failed',
        })
        await loadModels()
        sizeCaps.ensureSelectableTier()
        return false
      }
      if (!job.assets?.length) {
        errorMsg.value = t('imageStudio.assetsMissingHint')
        latestJob.value = job
        jobs.value = [job, ...jobs.value.filter((j) => j.id !== job.id)]
        pollNotice.value = ''
        return false
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
      pollNotice.value = ''
      trackGrowthEvent('image_studio_result_view', { job_id: job.id, count: job.count })
      if (!hasStudioFirstWin()) {
        markStudioFirstWin()
        showFirstWin.value = true
      }
      clearPromptDraftAfterSuccess(submittedUserId, submittedPrompt)
      refreshUserAfterImageSuccess()
      return true
    } catch (err: unknown) {
      const code = err instanceof Error ? err.message : ''
      if (code === 'IMAGE_STUDIO_POLL_TIMEOUT') {
        errorMsg.value = t('imageStudio.pollTimeout')
      } else {
        errorMsg.value = extractApiErrorMessage(err, t('imageStudio.generateFailed'))
      }
      trackGrowthEvent('image_studio_generate_fail', {
        template_id: selectedTemplate.value.id,
        reason: extractApiErrorCode(err) || code || 'request_failed',
      })
      persistDraft()
      return false
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

  onMounted(() => {
    trackGrowthEvent('image_studio_workspace_view')
    void load()
  })
  onBeforeUnmount(flushScheduledDraft)

  return {
    bootstrapping,
    generating,
    polling,
    pollNotice,
    catalog,
    capabilities,
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
    galleryError,
    errorMsg,
    autoCleanup,
    showFirstWin,
    latestJob,
    balance,
    hasApiKeys,
    showAccentColor,
    showQuality,
    promptValid,
    promptError,
    expertPromptValid,
    expertPromptError,
    maxCount,
    isNewUser,
    previewAsset,
    previewJobId,
    previewIndex,
    labelFor,
    pickTemplate,
    onAutoCleanupChange,
    openPreview,
    closePreview,
    regenerateFromJob,
    generate,
    removeJob,
    onAspectChange,
    onTierChange,
    refreshJobs,
    load,
  }
}
