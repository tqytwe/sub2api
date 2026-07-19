import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { keysAPI } from '@/api/keys'
import imageStudioAPI, {
  isImageStudioJobActive,
  isImageStudioJobTerminal,
  type ImageStudioAsset,
  type ImageStudioCapabilities,
  type ImageStudioCatalog,
  type ImageStudioEstimate,
  type ImageStudioGenerateRequest,
  type ImageStudioJob,
  type ImageStudioModelOption,
  type ImageStudioReference,
  type ImageStudioTemplate,
} from '@/api/imageStudio'
import type { PromptUseResult } from '@/api/prompts'
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
  clearStudioPendingJobId,
  clearStudioPendingJobForUser,
  getStudioPendingJobId,
  getStudioPendingJobsForUser,
  getStudioPendingJobSubmittedPrompt,
  loadStudioDraft,
  saveStudioDraft,
  setStudioPendingJobId,
  type ImageStudioSubmittedPrompt,
} from '@/utils/imageStudioSession'
import {
  isImageStudioPromptValid,
  resolveInitialImageStudioTemplate,
  validateImageStudioPrompt,
} from '@/utils/imageStudioWorkspace'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import {
  initialPromptVariableValues,
  loadPromptLibraryHandoff,
  renderPromptLibraryTemplate,
  type PromptLibraryHandoff,
} from '@/utils/promptLibraryHandoff'
import { savePromptRecipe } from '@/utils/promptRecipe'

type ImageStudioMode = 'create' | 'edit'
type ImageStudioReferenceUploadStatus = 'uploading' | 'ready' | 'failed'

export interface ImageStudioReferenceUpload {
  localId: string
  file: File
  previewUrl: string
  status: ImageStudioReferenceUploadStatus
  referenceId?: string
  reference?: ImageStudioReference
  error?: string
}

const IMAGE_STUDIO_REFERENCE_LIMIT = 4
let imageStudioReferenceSequence = 0

function createImageStudioIdempotencyKey(): string {
  const requestId = typeof globalThis.crypto?.randomUUID === 'function'
    ? globalThis.crypto.randomUUID()
    : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
  return `image-studio-generate-${requestId}`
}

export function useImageStudioWorkspace() {
  const { t, locale } = useI18n()
  const router = useRouter()
  const route = useRoute()
  const authStore = useAuthStore()

  const bootstrapping = ref(true)
  const generating = ref(false)
  const pollingJobIds = ref<Set<string>>(new Set())
  const cancelingJobIds = ref<Set<string>>(new Set())
  const catalog = ref<ImageStudioCatalog | null>(null)
  const capabilities = ref<ImageStudioCapabilities | null>(null)
  const capabilitiesLoading = ref(false)
  const capabilityError = ref('')
  const selectedTemplate = ref<ImageStudioTemplate | null>(null)
  const promptReference = ref<PromptLibraryHandoff | null>(null)
  const promptReferenceError = ref('')
  const promptVariableValues = ref<Record<string, string>>({})
  const promptPrivacyMode = ref(false)
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
  const background = ref('')
  const outputFormat = ref('')
  const outputCompression = ref(85)
  const inputFidelity = ref('')
  const mode = ref<ImageStudioMode>('create')
  const referenceUploads = ref<ImageStudioReferenceUpload[]>([])
  const loadingModels = ref(false)
  const loadingEstimate = ref(false)
  const modelError = ref('')
  const estimateError = ref('')
  const estimate = ref<ImageStudioEstimate | null>(null)
  const jobs = ref<ImageStudioJob[]>([])
  const activeJobs = ref<ImageStudioJob[]>([])
  const galleryError = ref('')
  const galleryLoading = ref(false)
  const galleryLoaded = ref(false)
  const galleryPage = ref(1)
  const galleryPageSize = 12
  const galleryTotal = ref(0)
  const galleryPages = ref(0)
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
  const pollControllers = new Map<string, AbortController>()
  const pollRetryTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const referenceUploadControllers = new Map<string, AbortController>()
  const claimedReferenceIds = new Set<string>()
  const protectedReferenceIds = new Set<string>()
  const referenceCleanupIds = new Set<string>()
  const jobSubmissionSequences = new Map<string, number>()
  let galleryRequestSequence = 0
  let loadRequestSequence = 0
  let modelRequestSequence = 0
  let loadingModelsApiKeyId = 0
  let estimateRequestSequence = 0
  let submissionSequence = 0
  let latestJobSubmissionSequence = 0
  let generateAttempt: { fingerprint: string; idempotencyKey: string } | null = null
  let mounted = false
  let disposed = false

  const selectedModelOption = computed(() =>
    availableModels.value.find((m) => m.id === selectedModel.value) ?? null,
  )

  const sizeCaps = useImageStudioCapabilities(
    () => capabilities.value,
    () => selectedModelOption.value,
  )

  const size = sizeCaps.resolvedSize
  const capabilitiesReady = computed(() =>
    !!capabilities.value
    && !capabilitiesLoading.value
    && !capabilityError.value
    && !!sizeCaps.currentOption.value
    && !!size.value,
  )

  const isNewUser = computed(() => totalRecharged.value <= 0)
  const maxCount = computed(() => (isNewUser.value ? 1 : 4))
  const balance = computed(() => authStore.user?.balance ?? estimate.value?.balance ?? 0)
  const hasApiKeys = computed(() => apiKeys.value.length > 0)
  const showAccentColor = computed(() => selectedTemplate.value !== null)
  const showQuality = computed(() => (selectedModelOption.value?.supported_qualities?.length ?? 0) > 0)
  const showBackground = computed(() =>
    (selectedModelOption.value?.supported_backgrounds?.length ?? 0) > 0,
  )
  const showOutputFormat = computed(() =>
    (selectedModelOption.value?.supported_output_formats?.length ?? 0) > 1,
  )
  const showOutputCompression = computed(() => {
    const compression = selectedModelOption.value?.output_compression
    return !!compression?.formats?.includes(outputFormat.value)
  })
  const showInputFidelity = computed(() =>
    mode.value === 'edit'
    && selectedModelOption.value?.input_fidelity_mode === 'selectable'
    && (selectedModelOption.value?.supported_input_fidelities?.length ?? 0) > 0,
  )
  function modelSupportsOperation(
    model: ImageStudioModelOption | null | undefined,
    operation: ImageStudioMode,
  ) {
    if (!model) return false
    if (!model.operations?.length) return false
    return model.operations.includes(operation)
  }
  const maxReferenceImages = computed(() => {
    const value = Number(selectedModelOption.value?.max_reference_images)
    return Number.isInteger(value) && value > 0
      ? Math.min(IMAGE_STUDIO_REFERENCE_LIMIT, value)
      : 0
  })
  const supportsCreate = computed(() =>
    modelSupportsOperation(selectedModelOption.value, 'create'),
  )
  const supportsEdit = computed(() =>
    modelSupportsOperation(selectedModelOption.value, 'edit') && maxReferenceImages.value > 0,
  )
  const operationSupported = computed(() =>
    mode.value === 'create' ? supportsCreate.value : supportsEdit.value,
  )
  const promptError = computed(() => validateImageStudioPrompt(userPrompt.value))
  const expertPromptError = computed(() =>
    expertOpen.value
      ? validateImageStudioPrompt(expertPrompt.value, { required: false })
      : null,
  )
  const promptValid = computed(() => promptError.value === null)
  const expertPromptValid = computed(() => expertPromptError.value === null)
  const draftUserId = computed(() => authStore.user?.id ?? null)
  let sessionEpoch = 0
  let sessionUserId = draftUserId.value
  const polling = computed(() => pollingJobIds.value.size > 0)
  const activeJobCount = computed(() => activeJobs.value.length)
  const atActiveJobLimit = computed(() => activeJobCount.value >= 2)
  const uploadingReferences = computed(() =>
    referenceUploads.value.some((item) => item.status === 'uploading'),
  )
  const activeReferenceUploads = computed(() =>
    referenceUploads.value.filter((item) => item.status !== 'failed'),
  )
  const readyReferenceUploads = computed(() =>
    referenceUploads.value.filter((item) => item.status === 'ready' && !!item.referenceId),
  )
  const referenceSlotCount = computed(() => activeReferenceUploads.value.length)
  const readyReferenceCount = computed(() => readyReferenceUploads.value.length)
  const editReferencesReady = computed(() =>
    mode.value === 'create'
    || (
      supportsEdit.value
      && !uploadingReferences.value
      && readyReferenceCount.value > 0
      && readyReferenceCount.value <= maxReferenceImages.value
    ),
  )
  const estimateReferenceIds = computed(() => (
    mode.value === 'edit'
      ? readyReferenceUploads.value.map((item) => item.referenceId as string)
      : []
  ))

  interface ImageStudioSessionScope {
    epoch: number
    userId: number | null
  }

  function sessionScope(): ImageStudioSessionScope {
    const userId = draftUserId.value
    if (userId !== sessionUserId) resetSessionState(userId)
    return { epoch: sessionEpoch, userId }
  }

  function isCurrentSession(scope: ImageStudioSessionScope) {
    return (
      !disposed
      && scope.epoch === sessionEpoch
      && scope.userId === sessionUserId
      && scope.userId === draftUserId.value
    )
  }

  function stopSessionAsyncWork() {
    for (const controller of pollControllers.values()) controller.abort()
    pollControllers.clear()
    for (const timer of pollRetryTimers.values()) clearTimeout(timer)
    pollRetryTimers.clear()
    pollingJobIds.value = new Set()
    cancelingJobIds.value = new Set()
    for (const controller of referenceUploadControllers.values()) controller.abort()
    referenceUploadControllers.clear()
  }

  function resetSessionState(nextUserId: number | null) {
    if (nextUserId === sessionUserId) return
    flushScheduledDraft()
    sessionEpoch += 1
    sessionUserId = nextUserId
    loadRequestSequence += 1
    galleryRequestSequence += 1
    modelRequestSequence += 1
    estimateRequestSequence += 1
    stopSessionAsyncWork()
    discardReferenceUploads(referenceUploads.value)
    referenceUploads.value = []
    claimedReferenceIds.clear()
    jobSubmissionSequences.clear()
    latestJobSubmissionSequence = 0
    generateAttempt = null
    bootstrapping.value = true
    generating.value = false
    catalog.value = null
    capabilities.value = null
    capabilitiesLoading.value = false
    capabilityError.value = ''
    selectedTemplate.value = null
    userPrompt.value = ''
    expertPrompt.value = ''
    expertOpen.value = false
    accentColor.value = '#1a1a1a'
    count.value = 1
    apiKeyId.value = null
    apiKeys.value = []
    availableModels.value = []
    selectedModel.value = ''
    promptReference.value = null
    promptReferenceError.value = ''
    promptVariableValues.value = {}
    promptPrivacyMode.value = false
    quality.value = ''
    background.value = ''
    outputFormat.value = ''
    outputCompression.value = 85
    inputFidelity.value = ''
    mode.value = 'create'
    loadingModels.value = false
    loadingEstimate.value = false
    modelError.value = ''
    estimateError.value = ''
    estimate.value = null
    jobs.value = []
    activeJobs.value = []
    latestJob.value = null
    galleryError.value = ''
    galleryLoading.value = false
    galleryLoaded.value = false
    galleryPage.value = 1
    galleryTotal.value = 0
    galleryPages.value = 0
    errorMsg.value = ''
    pollNotice.value = ''
    previewAsset.value = null
    previewJobId.value = ''
    previewIndex.value = 0
    draftReady.value = false
    pendingDraftSave = null
    sizeCaps.resetUserTouched()
    sizeCaps.setFromSize('')
  }

  function labelFor(obj?: { zh: string; en: string }) {
    if (!obj) return ''
    return locale.value.startsWith('zh') ? obj.zh : obj.en
  }

  function applyPromptVariables() {
    if (!promptReference.value) return
    userPrompt.value = renderPromptLibraryTemplate(
      promptReference.value.prompt_template,
      promptReference.value.variables,
      promptVariableValues.value,
    )
  }

  function applyPromptReferenceRecommendations() {
    const reference = promptReference.value
    if (!reference) return
    const recommendedModel = reference.recommended_models.find((model) =>
      availableModels.value.some((option) => option.id === model))
    if (recommendedModel) {
      selectedModel.value = recommendedModel
      applySelectedModelDefaults(
        availableModels.value.find((option) => option.id === recommendedModel) ?? null,
      )
    }
    const recommendedSize = reference.recommended_sizes.find((item) =>
      capabilities.value?.size_options.some((option) => option.size === item))
    if (recommendedSize) {
      sizeCaps.setFromSize(recommendedSize)
      sizeCaps.userTouchedSize.value = true
    }
  }

  function loadPromptReference() {
    if (!route.query.prompt && !route.query.version) return
    const reference = loadPromptLibraryHandoff(route.query)
    if (!reference) {
      promptReferenceError.value = t('imageStudio.promptReferenceUnavailable')
      return
    }
    promptReference.value = reference
    promptPrivacyMode.value = true
    promptReferenceError.value = ''
    promptVariableValues.value = initialPromptVariableValues(reference.variables)
    applyPromptVariables()
    applyPromptReferenceRecommendations()
  }

  function applyPromptUseResult(payload: PromptUseResult) {
    const reference: PromptLibraryHandoff = {
      prompt_id: payload.prompt_id,
      version: payload.version,
      title: payload.title,
      prompt_template: payload.prompt_template,
      variables: payload.variables,
      recommended_models: payload.recommended_models,
      recommended_sizes: payload.recommended_sizes,
      reference_requirement: payload.reference_requirement,
    }
    promptReference.value = reference
    promptPrivacyMode.value = true
    promptReferenceError.value = ''
    promptVariableValues.value = initialPromptVariableValues(reference.variables)
    applyPromptVariables()
    applyPromptReferenceRecommendations()
    void router.replace({
      path: '/image-studio',
      query: {
        prompt: reference.prompt_id,
        version: String(reference.version),
      },
    })
  }

  function clearPromptReference() {
    userPrompt.value = ''
    expertPrompt.value = ''
    promptReference.value = null
    promptReferenceError.value = ''
    promptVariableValues.value = {}
    promptPrivacyMode.value = false
    void router.replace({ path: '/image-studio', query: {} })
  }

  function saveCreationRecipe() {
    const reference = promptReference.value
    if (!reference) return false
    savePromptRecipe({
      promptId: reference.prompt_id,
      promptVersion: reference.version,
      title: reference.title,
      model: selectedModel.value,
      size: size.value,
      quality: quality.value,
      variables: reference.variables,
    })
    return true
  }

  function firstSupported(
    current: string,
    supported: string[] | undefined,
    fallback: string | undefined,
  ) {
    if (current && supported?.includes(current)) return current
    if (fallback && supported?.includes(fallback)) return fallback
    return supported?.[0] ?? ''
  }

  function applyInputFidelityDefault(
    model: ImageStudioModelOption | null | undefined,
    nextMode: ImageStudioMode,
  ) {
    if (
      nextMode !== 'edit'
      || model?.input_fidelity_mode !== 'selectable'
      || !model.supported_input_fidelities?.length
    ) {
      inputFidelity.value = ''
      return
    }
    inputFidelity.value = firstSupported(
      inputFidelity.value,
      model.supported_input_fidelities,
      model.default_input_fidelity,
    )
  }

  function normalizeOutputCompression(model: ImageStudioModelOption | null | undefined) {
    const limits = model?.output_compression
    if (!limits?.formats?.includes(outputFormat.value)) {
      outputCompression.value = 85
      return
    }
    const min = Number.isFinite(limits.min) ? limits.min : 0
    const max = Number.isFinite(limits.max) ? limits.max : 100
    const lower = Math.min(min, max)
    const upper = Math.max(min, max)
    const current = Number.isFinite(outputCompression.value) ? outputCompression.value : 85
    outputCompression.value = Math.min(upper, Math.max(lower, current))
  }

  function ensureSupportedMode() {
    if (mode.value === 'edit' && !supportsEdit.value && supportsCreate.value) {
      mode.value = 'create'
      return
    }
    if (mode.value === 'create' && !supportsCreate.value && supportsEdit.value) {
      mode.value = 'edit'
    }
  }

  function applySelectedModelDefaults(model: ImageStudioModelOption | null) {
    if (!model) {
      quality.value = ''
      background.value = ''
      outputFormat.value = ''
      outputCompression.value = 85
      inputFidelity.value = ''
      return
    }
    quality.value = firstSupported(
      quality.value,
      model.supported_qualities,
      model.default_quality,
    )
    background.value = firstSupported(
      background.value,
      model.supported_backgrounds,
      model.default_background,
    )
    outputFormat.value = firstSupported(
      outputFormat.value,
      model.supported_output_formats,
      model.default_output_format,
    )
    normalizeOutputCompression(model)
    applyInputFidelityDefault(model, mode.value)
    ensureSupportedMode()
    trimReferenceUploadsToLimit()
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
    if (!draftReady.value || !userId || promptPrivacyMode.value) return
    saveStudioDraft(userId, currentDraft())
  }

  function flushScheduledDraft() {
    if (draftSaveTimer) clearTimeout(draftSaveTimer)
    draftSaveTimer = null
    if (!pendingDraftSave) return
    if (promptPrivacyMode.value) {
      pendingDraftSave = null
      return
    }
    saveStudioDraft(pendingDraftSave.userId, pendingDraftSave.draft)
    pendingDraftSave = null
  }

  function scheduleDraftPersist() {
    const userId = draftUserId.value
    if (!draftReady.value || !userId || promptPrivacyMode.value) return
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
    if (!submittedPrompt || !matchesCurrentSubmission(submittedPrompt)) return
    if (!clearStudioPromptDraft(submittedUserId, submittedPrompt)) return
    if (draftUserId.value === submittedUserId) {
      userPrompt.value = ''
      expertPrompt.value = ''
      persistDraft()
    }
  }

  function matchesCurrentSubmission(submitted: ImageStudioSubmittedPrompt) {
    const current = currentSubmissionSnapshot()
    return Object.entries(submitted).every(([key, value]) => (
      value === undefined
      || (
        Array.isArray(value)
          ? (
              Array.isArray(current[key as keyof ImageStudioSubmittedPrompt])
              && value.length === (
                current[key as keyof ImageStudioSubmittedPrompt] as unknown[]
              ).length
              && value.every((item, index) => (
                item === (
                  current[key as keyof ImageStudioSubmittedPrompt] as unknown[]
                )[index]
              ))
            )
          : current[key as keyof ImageStudioSubmittedPrompt] === value
      )
    ))
  }

  function currentSubmissionSnapshot(): ImageStudioSubmittedPrompt {
    const referenceIds = mode.value === 'edit'
      ? readyReferenceUploads.value.map((item) => item.referenceId as string)
      : []
    return {
      userPrompt: userPrompt.value,
      expertPrompt: expertPrompt.value,
      expertOpen: expertOpen.value,
      templateId: selectedTemplate.value?.id ?? null,
      accentColor: accentColor.value,
      aspect: sizeCaps.aspect.value,
      tier: sizeCaps.tier.value,
      count: count.value,
      model: selectedModel.value,
      quality: quality.value,
      apiKeyId: apiKeyId.value ?? undefined,
      background: showBackground.value ? background.value : '',
      outputFormat: showOutputFormat.value ? outputFormat.value : '',
      outputCompression: showOutputCompression.value ? outputCompression.value : null,
      inputFidelity: showInputFidelity.value ? inputFidelity.value : '',
      mode: mode.value,
      referenceIds,
    }
  }

  function refreshUserAfterImageSuccess() {
    void Promise.resolve()
      .then(() => authStore.refreshUser())
      .catch(() => {})
  }

  function createReferencePreview(file: File) {
    if (typeof URL.createObjectURL !== 'function') return ''
    return URL.createObjectURL(file)
  }

  function revokeReferencePreview(previewUrl: string) {
    if (previewUrl && typeof URL.revokeObjectURL === 'function') {
      URL.revokeObjectURL(previewUrl)
    }
  }

  function replaceReferenceUpload(localId: string, update: Partial<ImageStudioReferenceUpload>) {
    referenceUploads.value = referenceUploads.value.map((item) =>
      item.localId === localId ? { ...item, ...update } : item,
    )
  }

  async function deleteReferenceQuietly(referenceId: string) {
    if (!referenceId || referenceCleanupIds.has(referenceId)) return
    referenceCleanupIds.add(referenceId)
    try {
      await imageStudioAPI.deleteImageStudioReference(referenceId)
    } catch {
      // Reference cleanup is compensating and must not block the workspace.
    }
  }

  function abortReferenceUpload(localId: string) {
    const controller = referenceUploadControllers.get(localId)
    if (!controller) return
    controller.abort()
    referenceUploadControllers.delete(localId)
  }

  function discardReferenceUpload(item: ImageStudioReferenceUpload) {
    abortReferenceUpload(item.localId)
    revokeReferencePreview(item.previewUrl)
    const referenceId = item.referenceId
    if (
      item.status === 'ready'
      && referenceId
      && !claimedReferenceIds.has(referenceId)
      && !protectedReferenceIds.has(referenceId)
    ) {
      void deleteReferenceQuietly(referenceId)
    }
  }

  function discardReferenceUploads(items: ImageStudioReferenceUpload[]) {
    for (const item of items) discardReferenceUpload(item)
  }

  function trimReferenceUploadsToLimit() {
    const kept: ImageStudioReferenceUpload[] = []
    const removed: ImageStudioReferenceUpload[] = []
    let activeCount = 0
    for (const item of referenceUploads.value) {
      if (item.status === 'failed') {
        kept.push(item)
        continue
      }
      if (activeCount < maxReferenceImages.value) {
        activeCount += 1
        kept.push(item)
        continue
      }
      removed.push(item)
    }
    if (!removed.length) return
    discardReferenceUploads(removed)
    referenceUploads.value = kept
  }

  function settleSubmissionReferences(
    referenceIds: string[],
    accepted: boolean,
  ) {
    for (const referenceId of referenceIds) {
      protectedReferenceIds.delete(referenceId)
      if (accepted) {
        claimedReferenceIds.add(referenceId)
        continue
      }
      const stillDisplayed = referenceUploads.value.some(
        (item) => item.referenceId === referenceId,
      )
      if (!stillDisplayed && !claimedReferenceIds.has(referenceId)) {
        void deleteReferenceQuietly(referenceId)
      }
    }
  }

  async function uploadReferenceItem(item: ImageStudioReferenceUpload) {
    const scope = sessionScope()
    abortReferenceUpload(item.localId)
    const controller = new AbortController()
    referenceUploadControllers.set(item.localId, controller)
    replaceReferenceUpload(item.localId, {
      status: 'uploading',
      referenceId: undefined,
      reference: undefined,
      error: '',
    })
    try {
      const reference = await imageStudioAPI.uploadImageStudioReference(
        item.file,
        controller.signal,
      )
      if (!isCurrentSession(scope)) {
        await deleteReferenceQuietly(reference.id)
        return
      }
      const isCurrent = referenceUploadControllers.get(item.localId) === controller
      const stillPresent = referenceUploads.value.some(
        (candidate) => candidate.localId === item.localId,
      )
      if (controller.signal.aborted || !isCurrent || !stillPresent) {
        await deleteReferenceQuietly(reference.id)
        return
      }
      replaceReferenceUpload(item.localId, {
        status: 'ready',
        referenceId: reference.id,
        reference,
        error: '',
      })
    } catch (err: unknown) {
      if (
        controller.signal.aborted
        || referenceUploadControllers.get(item.localId) !== controller
        || !referenceUploads.value.some((candidate) => candidate.localId === item.localId)
      ) {
        return
      }
      replaceReferenceUpload(item.localId, {
        status: 'failed',
        referenceId: undefined,
        reference: undefined,
        error: extractApiErrorMessage(err, t('imageStudio.referenceUploadFailed')),
      })
    } finally {
      if (referenceUploadControllers.get(item.localId) === controller) {
        referenceUploadControllers.delete(item.localId)
      }
    }
  }

  async function addReferenceFiles(files: File[]) {
    if (!supportsEdit.value) {
      errorMsg.value = t('imageStudio.operationUnsupported')
      return
    }
    const available = Math.max(0, maxReferenceImages.value - referenceSlotCount.value)
    const accepted = files.slice(0, available)
    if (accepted.length < files.length) {
      errorMsg.value = t('imageStudio.referenceLimit')
    }
    const items = accepted.map<ImageStudioReferenceUpload>((file) => ({
      localId: `reference-${Date.now()}-${++imageStudioReferenceSequence}`,
      file,
      previewUrl: createReferencePreview(file),
      status: 'uploading',
    }))
    referenceUploads.value = [...referenceUploads.value, ...items]
    await Promise.all(items.map(uploadReferenceItem))
  }

  async function retryReference(localId: string) {
    const item = referenceUploads.value.find((candidate) => candidate.localId === localId)
    if (!item || item.status === 'uploading') return
    await uploadReferenceItem(item)
  }

  function removeReference(localId: string) {
    const item = referenceUploads.value.find((candidate) => candidate.localId === localId)
    if (!item) return
    discardReferenceUpload(item)
    referenceUploads.value = referenceUploads.value.filter((candidate) => candidate.localId !== localId)
  }

  async function loadModels() {
    const scope = sessionScope()
    const requestId = ++modelRequestSequence
    const requestedApiKeyId = apiKeyId.value
    modelError.value = ''
    availableModels.value = []
    selectedModel.value = ''
    quality.value = ''
    if (!requestedApiKeyId) {
      loadingModelsApiKeyId = 0
      loadingModels.value = false
      return
    }
    loadingModelsApiKeyId = requestedApiKeyId
    loadingModels.value = true
    try {
      const models = await imageStudioAPI.listImageStudioModels(requestedApiKeyId)
      if (
        !isCurrentSession(scope)
        || requestId !== modelRequestSequence
        || apiKeyId.value !== requestedApiKeyId
      ) return
      applyModels(models)
    } catch {
      if (
        !isCurrentSession(scope)
        || requestId !== modelRequestSequence
        || apiKeyId.value !== requestedApiKeyId
      ) return
      modelError.value = t('imageStudio.loadModelsFailed')
    } finally {
      if (isCurrentSession(scope) && requestId === modelRequestSequence) {
        loadingModelsApiKeyId = 0
        loadingModels.value = false
      }
    }
  }

  function applyModels(models: ImageStudioModelOption[]) {
    availableModels.value = models
    selectedModel.value = models[0]?.id ?? ''
    applySelectedModelDefaults(models[0] ?? null)
    applyPromptReferenceRecommendations()
    if (models[0]?.default_size && !selectedTemplate.value) {
      sizeCaps.applyTemplateDefault(models[0].default_size, true)
    }
    sizeCaps.ensureSelectableTier()
    if (!models.length) modelError.value = t('imageStudio.noModels')
  }

  async function refreshEstimate() {
    const scope = sessionScope()
    const requestId = ++estimateRequestSequence
    estimateError.value = ''
    const templateId = selectedTemplate.value?.id
    const requestedApiKeyId = apiKeyId.value
    const model = selectedModel.value
    if (
      !templateId
      || !requestedApiKeyId
      || !model
      || !size.value
      || capabilityError.value
      || !operationSupported.value
      || (mode.value === 'edit' && !editReferencesReady.value)
    ) {
      estimate.value = null
      loadingEstimate.value = false
      return
    }
    loadingEstimate.value = true
    estimate.value = null
    try {
      const result = await imageStudioAPI.estimateImageStudio({
        template_id: templateId,
        size: size.value,
        count: count.value,
        api_key_id: requestedApiKeyId,
        model,
        reference_ids: estimateReferenceIds.value.length
          ? estimateReferenceIds.value
          : undefined,
      })
      if (!isCurrentSession(scope) || requestId !== estimateRequestSequence) return
      estimate.value = result
    } catch {
      if (!isCurrentSession(scope) || requestId !== estimateRequestSequence) return
      estimate.value = null
      estimateError.value = t('imageStudio.estimateFailed')
    } finally {
      if (isCurrentSession(scope) && requestId === estimateRequestSequence) {
        loadingEstimate.value = false
      }
    }
  }

  function setPollingJob(jobId: string, active: boolean) {
    const next = new Set(pollingJobIds.value)
    if (active) next.add(jobId)
    else next.delete(jobId)
    pollingJobIds.value = next
    pollNotice.value = next.size > 0 ? t('imageStudio.polling') : ''
  }

  function setCancelingJob(jobId: string, active: boolean) {
    const next = new Set(cancelingJobIds.value)
    if (active) next.add(jobId)
    else next.delete(jobId)
    cancelingJobIds.value = next
  }

  function mergeJobSnapshot(existing: ImageStudioJob | undefined, incoming: ImageStudioJob) {
    if (!existing) return incoming
    if (isImageStudioJobTerminal(existing) && isImageStudioJobActive(incoming)) {
      return existing
    }
    return {
      ...existing,
      ...incoming,
      assets: incoming.assets ?? existing.assets,
      items: incoming.items ?? existing.items,
    }
  }

  function upsertHistoryJob(job: ImageStudioJob, front = true, insert = true) {
    const existing = jobs.value.find((item) => item.id === job.id)
    if (!existing && !insert) return
    const merged = mergeJobSnapshot(existing, job)
    const remaining = jobs.value.filter((item) => item.id !== job.id)
    jobs.value = front ? [merged, ...remaining] : [...remaining, merged]
  }

  function upsertActiveJob(job: ImageStudioJob) {
    if (isImageStudioJobTerminal(job)) {
      activeJobs.value = activeJobs.value.filter((item) => item.id !== job.id)
      upsertHistoryJob(job, true, false)
      return
    }
    const existing = activeJobs.value.find((item) => item.id === job.id)
      ?? jobs.value.find((item) => item.id === job.id)
    const merged = mergeJobSnapshot(existing, job)
    activeJobs.value = [
      merged,
      ...activeJobs.value.filter((item) => item.id !== job.id),
    ]
  }

  function mergeJobList(listedJobs: ImageStudioJob[]) {
    const currentById = new Map(jobs.value.map((job) => [job.id, job]))
    if (latestJob.value) currentById.set(latestJob.value.id, latestJob.value)
    for (const job of activeJobs.value) currentById.set(job.id, job)
    jobs.value = listedJobs.map((job) => mergeJobSnapshot(currentById.get(job.id), job))
  }

  function mergeActiveJobs(incomingJobs: ImageStudioJob[]) {
    for (const job of [...incomingJobs].reverse()) upsertActiveJob(job)
  }

  function shouldFeatureJob(job: ImageStudioJob, jobSequence: number) {
    if (jobSequence > latestJobSubmissionSequence) return true
    if (jobSequence < latestJobSubmissionSequence) return false
    if (!latestJob.value || latestJob.value.id === job.id) return true
    const currentTime = Date.parse(latestJob.value.created_at)
    const incomingTime = Date.parse(job.created_at)
    if (!Number.isFinite(incomingTime)) return false
    if (!Number.isFinite(currentTime)) return true
    return incomingTime > currentTime
  }

  async function handleTerminalJob(
    job: ImageStudioJob,
    submittedUserId: number | null,
    submittedPrompt: ImageStudioSubmittedPrompt | null,
    scope: ImageStudioSessionScope,
    jobSequence = 0,
  ) {
    if (!isCurrentSession(scope)) return
    upsertActiveJob(job)
    if (getStudioPendingJobId() === job.id) clearStudioPendingJobId()
    if (submittedUserId) clearStudioPendingJobForUser(submittedUserId, job.id)
    jobSubmissionSequences.delete(job.id)

    if (job.status === 'failed') {
      errorMsg.value = job.error_message || t('imageStudio.generateFailed')
      trackGrowthEvent('image_studio_generate_fail', {
        template_id: job.template_id,
        reason: job.error_message || 'job_failed',
      })
      return
    }
    if (job.status === 'cancelled') return

    if (!job.assets?.length) {
      errorMsg.value = t('imageStudio.assetsMissingHint')
      return
    }
    if (shouldFeatureJob(job, jobSequence)) {
      latestJobSubmissionSequence = jobSequence
      latestJob.value = job
    }

    trackGrowthEvent('image_studio_generate_success', {
      template_id: job.template_id,
      actual_cost: job.actual_cost ?? job.estimated_cost,
      count: job.count,
      size: job.size,
      status: job.status,
    })
    trackQuestCompleteOnce('image_generate')
    trackGrowthEvent('image_studio_result_view', { job_id: job.id, count: job.count })
    if (!hasStudioFirstWin()) {
      markStudioFirstWin()
      showFirstWin.value = true
    }
    if (submittedUserId) {
      clearPromptDraftAfterSuccess(submittedUserId, submittedPrompt)
    }
    refreshUserAfterImageSuccess()
  }

  function startPollingJob(
    jobId: string,
    jobSequence = jobSubmissionSequences.get(jobId) ?? 0,
    scope = sessionScope(),
  ) {
    if (!isCurrentSession(scope) || pollControllers.has(jobId)) return
    const submittedUserId = scope.userId
    const submittedPrompt = submittedUserId
      ? getStudioPendingJobSubmittedPrompt(submittedUserId, jobId)
      : null
    const controller = new AbortController()
    pollControllers.set(jobId, controller)
    setPollingJob(jobId, true)

    void (async () => {
      try {
        const job = await imageStudioAPI.pollImageStudioJob(jobId, { signal: controller.signal })
        if (controller.signal.aborted || !isCurrentSession(scope)) return
        await handleTerminalJob(job, submittedUserId, submittedPrompt, scope, jobSequence)
      } catch (err: unknown) {
        if (!isCurrentSession(scope)) return
        const code = err instanceof Error ? err.message : ''
        if (code === 'IMAGE_STUDIO_POLL_ABORTED') return
        if (code === 'IMAGE_STUDIO_POLL_TIMEOUT') {
          errorMsg.value = t('imageStudio.pollTimeout')
          if (!pollRetryTimers.has(jobId)) {
            const timer = setTimeout(() => {
              pollRetryTimers.delete(jobId)
              if (isCurrentSession(scope)) startPollingJob(jobId, jobSequence, scope)
            }, 1000)
            pollRetryTimers.set(jobId, timer)
          }
        } else {
          errorMsg.value = extractApiErrorMessage(err, t('imageStudio.generateFailed'))
        }
      } finally {
        if (isCurrentSession(scope) && pollControllers.get(jobId) === controller) {
          pollControllers.delete(jobId)
          setPollingJob(jobId, false)
          if (galleryLoaded.value) void refreshJobs()
        }
      }
    })()
  }

  async function refreshJobs(
    page = galleryPage.value,
    scope = sessionScope(),
  ) {
    if (!isCurrentSession(scope)) return
    const requestedPage = Math.max(1, Math.floor(page))
    const requestId = ++galleryRequestSequence
    galleryLoading.value = true
    try {
      const result = await imageStudioAPI.listImageStudioJobs(requestedPage, galleryPageSize)
      if (!isCurrentSession(scope) || requestId !== galleryRequestSequence) return
      mergeJobList(result.jobs)
      for (const job of result.jobs) {
        if (!isImageStudioJobActive(job)) continue
        upsertActiveJob(job)
        startPollingJob(job.id, jobSubmissionSequences.get(job.id) ?? 0, scope)
      }
      galleryLoaded.value = true
      galleryPage.value = result.page
      galleryTotal.value = result.total
      galleryPages.value = result.pages
      galleryError.value = ''
    } catch (err: unknown) {
      if (!isCurrentSession(scope) || requestId !== galleryRequestSequence) return
      galleryError.value = extractApiErrorMessage(err, t('imageStudio.galleryLoadFailed'))
    } finally {
      if (isCurrentSession(scope) && requestId === galleryRequestSequence) {
        galleryLoading.value = false
      }
    }
  }

  async function ensureGalleryLoaded(page = 1) {
    if (galleryLoaded.value && galleryPage.value === page) return
    await refreshJobs(page)
  }

  async function restoreActiveJobs(scope = sessionScope()) {
    if (!isCurrentSession(scope)) return
    try {
      const restoredJobs = await imageStudioAPI.getActiveImageStudioJobs()
      if (!isCurrentSession(scope)) return
      mergeActiveJobs(restoredJobs)
      for (const job of restoredJobs.filter(isImageStudioJobActive)) {
        startPollingJob(job.id, jobSubmissionSequences.get(job.id) ?? 0, scope)
      }
    } catch {
      // The paged gallery and local pending snapshots can still restore the workspace.
    }
  }

  async function load() {
    const scope = sessionScope()
    const requestId = ++loadRequestSequence
    if (!isCurrentSession(scope)) return
    const isRefresh = !!catalog.value
    if (!isRefresh) bootstrapping.value = true
    errorMsg.value = ''
    capabilitiesLoading.value = true
    capabilityError.value = ''
    if (isRefresh && galleryLoaded.value) void refreshJobs(galleryPage.value, scope)
    void restoreActiveJobs(scope)
    try {
      const [tpl, capabilityResult, activeKeyPage, hub] = await Promise.all([
        imageStudioAPI.getImageStudioTemplates(),
        imageStudioAPI.getImageStudioCapabilities()
          .then((value) => ({ value, failed: false }))
          .catch(() => ({ value: null, failed: true })),
        keysAPI.list(1, 100, { status: 'active' }),
        playAPI.getPlayHub().catch(() => null),
      ])
      if (!isCurrentSession(scope) || requestId !== loadRequestSequence) return
      totalRecharged.value = hub?.growth?.total_recharged ?? 0
      catalog.value = tpl
      capabilities.value = capabilityResult.value
      capabilityError.value = capabilityResult.failed
        ? t('imageStudio.loadCapabilitiesFailed')
        : ''
      apiKeys.value = activeKeyPage.items
        .filter((key) => key.status === 'active')
        .map((key) => ({ id: key.id, name: key.name || `Key #${key.id}` }))
      if (apiKeys.value.length && !apiKeyId.value) {
        apiKeyId.value = apiKeys.value[0].id
      }
      if (!isRefresh && !applyQuickStart()) applyDefaultTemplate()
      if (!isRefresh) restoreDraft()
      if (!isRefresh) loadPromptReference()
      void loadModels()

      const pendingJobs = scope.userId
        ? getStudioPendingJobsForUser(scope.userId)
        : []
      for (const jobId of new Set(pendingJobs.map((job) => job.jobId))) {
        startPollingJob(jobId, jobSubmissionSequences.get(jobId) ?? 0, scope)
      }
    } catch {
      if (!isCurrentSession(scope) || requestId !== loadRequestSequence) return
      errorMsg.value = t('imageStudio.loadFailed')
    } finally {
      if (isCurrentSession(scope) && requestId === loadRequestSequence) {
        capabilitiesLoading.value = false
        draftReady.value = true
        bootstrapping.value = false
      }
    }
  }

  watch(
    [selectedTemplate, size, count, apiKeyId, selectedModel, mode, estimateReferenceIds],
    refreshEstimate,
  )
  watch(apiKeyId, () => {
    if (!bootstrapping.value && loadingModelsApiKeyId !== apiKeyId.value) void loadModels()
  })
  watch(maxCount, (max) => { if (count.value > max) count.value = max })
  watch(selectedModel, (modelId) => {
    const model = availableModels.value.find((m) => m.id === modelId)
    if (!model) return
    applySelectedModelDefaults(model)
    sizeCaps.ensureSelectableTier()
  })
  watch(mode, (nextMode) => {
    if (!operationSupported.value) {
      ensureSupportedMode()
      return
    }
    applyInputFidelityDefault(selectedModelOption.value, nextMode)
    if (nextMode === 'create') {
      discardReferenceUploads(referenceUploads.value)
      referenceUploads.value = []
    } else if (referenceSlotCount.value > maxReferenceImages.value) {
      trimReferenceUploadsToLimit()
    }
  })
  watch(draftUserId, (nextUserId) => {
    if (nextUserId === sessionUserId) return
    resetSessionState(nextUserId)
    if (mounted && !disposed) void load()
  }, { flush: 'sync' })
  watch(outputFormat, () => {
    normalizeOutputCompression(selectedModelOption.value)
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
    const scope = sessionScope()
    if (!isCurrentSession(scope)) return false
    if (generating.value || atActiveJobLimit.value) {
      if (atActiveJobLimit.value) errorMsg.value = t('imageStudio.activeJobLimit')
      return false
    }
    if (!capabilitiesReady.value) {
      errorMsg.value = capabilityError.value || t('imageStudio.loadCapabilitiesFailed')
      return false
    }
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
    if (!operationSupported.value) {
      errorMsg.value = t('imageStudio.operationUnsupported')
      return false
    }
    if (!editReferencesReady.value) {
      errorMsg.value = uploadingReferences.value
        ? t('imageStudio.referenceUploading')
        : t('imageStudio.referenceRequired')
      return false
    }
    const template = selectedTemplate.value
    const selectedApiKeyId = apiKeyId.value
    const model = selectedModel.value
    if (!template || !selectedApiKeyId || !model) return false
    const submittedUserId = scope.userId
    if (!submittedUserId) return false
    if (estimate.value && !estimate.value.sufficient) {
      trackGrowthEvent('image_studio_insufficient_balance', { balance: estimate.value.balance })
      router.push('/purchase?return=/image-studio')
      return false
    }
    trackGrowthEvent('image_studio_generate_click', {
      template_id: template.id,
      estimated_cost: estimate.value?.estimated_cost,
      size: size.value,
    })
    generating.value = true
    errorMsg.value = ''
    pollNotice.value = ''
    const currentSubmissionSequence = ++submissionSequence
    const submittedPrompt = currentSubmissionSnapshot()
    const promptReferenceSnapshot = promptReference.value
    const submittedReferenceIds = submittedPrompt.mode === 'edit'
      ? [...(submittedPrompt.referenceIds ?? [])]
      : []
    for (const referenceId of submittedReferenceIds) protectedReferenceIds.add(referenceId)
    const requestBody: ImageStudioGenerateRequest = {
      template_id: template.id,
      prompt_id: promptReferenceSnapshot ? Number(promptReferenceSnapshot.prompt_id) : undefined,
      prompt_version: promptReferenceSnapshot?.version,
      user_prompt: submittedPrompt.userPrompt,
      accent_color: submittedPrompt.accentColor,
      size: size.value,
      aspect: submittedPrompt.aspect,
      tier: submittedPrompt.tier,
      quality: submittedPrompt.quality || undefined,
      background: submittedPrompt.background || undefined,
      output_format: submittedPrompt.outputFormat || undefined,
      output_compression: typeof submittedPrompt.outputCompression === 'number'
        ? submittedPrompt.outputCompression
        : undefined,
      input_fidelity: submittedPrompt.inputFidelity || undefined,
      count: submittedPrompt.count,
      model: submittedPrompt.model || model,
      expert_prompt: submittedPrompt.expertOpen && submittedPrompt.expertPrompt.trim()
        ? submittedPrompt.expertPrompt
        : null,
      api_key_id: selectedApiKeyId,
      retain_days: autoCleanup.value ? 7 : 0,
      mode: submittedPrompt.mode,
      reference_ids: submittedPrompt.mode === 'edit'
        ? submittedPrompt.referenceIds
        : undefined,
    }
    const fingerprint = JSON.stringify(requestBody)
    if (!generateAttempt || generateAttempt.fingerprint !== fingerprint) {
      generateAttempt = {
        fingerprint,
        idempotencyKey: createImageStudioIdempotencyKey(),
      }
    }
    const idempotencyKey = generateAttempt.idempotencyKey
    try {
      const result = await imageStudioAPI.generateImageStudio(requestBody, idempotencyKey)
      settleSubmissionReferences(submittedReferenceIds, true)
      if (promptReferenceSnapshot) {
        setStudioPendingJobId(result.job.id)
      } else {
        setStudioPendingJobId(result.job.id, {
          userId: submittedUserId,
          submittedPrompt,
        })
      }
      if (!isCurrentSession(scope)) return true
      generateAttempt = null
      setStudioLastTemplate(template.id)
      jobSubmissionSequences.set(result.job.id, currentSubmissionSequence)
      upsertActiveJob(result.job)
      if (isImageStudioJobTerminal(result.job)) {
        await handleTerminalJob(
          result.job,
          promptReferenceSnapshot ? null : submittedUserId,
          promptReferenceSnapshot ? null : submittedPrompt,
          scope,
          currentSubmissionSequence,
        )
      } else {
        startPollingJob(result.job.id, currentSubmissionSequence, scope)
      }
      if (promptReferenceSnapshot) {
        userPrompt.value = ''
        expertPrompt.value = ''
      }
      return true
    } catch (err: unknown) {
      settleSubmissionReferences(submittedReferenceIds, false)
      if (!isCurrentSession(scope)) return false
      const code = err instanceof Error ? err.message : ''
      errorMsg.value = extractApiErrorMessage(err, t('imageStudio.generateFailed'))
      trackGrowthEvent('image_studio_generate_fail', {
        template_id: template.id,
        reason: extractApiErrorCode(err) || code || 'request_failed',
      })
      persistDraft()
      return false
    } finally {
      if (isCurrentSession(scope)) {
        generating.value = false
        if (galleryLoaded.value) void refreshJobs()
      }
    }
  }

  async function cancelJob(id: string) {
    const scope = sessionScope()
    const current = activeJobs.value.find((job) => job.id === id)
      ?? jobs.value.find((job) => job.id === id)
    if (!current || !isImageStudioJobActive(current) || cancelingJobIds.value.has(id)) return
    setCancelingJob(id, true)
    try {
      const job = await imageStudioAPI.cancelImageStudioJob(id)
      if (!isCurrentSession(scope)) return
      if (isImageStudioJobTerminal(job)) {
        pollControllers.get(id)?.abort()
        const retryTimer = pollRetryTimers.get(id)
        if (retryTimer) clearTimeout(retryTimer)
        pollRetryTimers.delete(id)
        const submittedUserId = scope.userId
        const submittedPrompt = submittedUserId
          ? getStudioPendingJobSubmittedPrompt(submittedUserId, id)
          : null
        await handleTerminalJob(
          job,
          submittedUserId,
          submittedPrompt,
          scope,
          jobSubmissionSequences.get(id) ?? 0,
        )
      } else {
        upsertActiveJob(job)
      }
    } catch (err: unknown) {
      if (!isCurrentSession(scope)) return
      errorMsg.value = extractApiErrorMessage(err, t('imageStudio.cancelFailed'))
    } finally {
      if (isCurrentSession(scope)) setCancelingJob(id, false)
    }
  }

  async function removeJob(id: string) {
    const scope = sessionScope()
    const current = activeJobs.value.find((job) => job.id === id)
      ?? jobs.value.find((job) => job.id === id)
    if (current && !isImageStudioJobTerminal(current)) return
    await imageStudioAPI.deleteImageStudioJob(id)
    if (!isCurrentSession(scope)) return
    jobs.value = jobs.value.filter((j) => j.id !== id)
    if (latestJob.value?.id === id) latestJob.value = null
    galleryTotal.value = Math.max(0, galleryTotal.value - 1)
    galleryPages.value = galleryTotal.value === 0
      ? 0
      : Math.ceil(galleryTotal.value / galleryPageSize)
    const nextPage = Math.min(galleryPage.value, Math.max(1, galleryPages.value))
    void refreshJobs(nextPage)
  }

  function onAspectChange(value: string) {
    sizeCaps.selectAspect(value)
  }

  function onTierChange(value: string) {
    sizeCaps.selectTier(value)
  }

  onMounted(() => {
    mounted = true
    trackGrowthEvent('image_studio_workspace_view')
    void load()
  })
  onBeforeUnmount(() => {
    disposed = true
    mounted = false
    flushScheduledDraft()
    sessionEpoch += 1
    loadRequestSequence += 1
    galleryRequestSequence += 1
    modelRequestSequence += 1
    estimateRequestSequence += 1
    discardReferenceUploads(referenceUploads.value)
    referenceUploads.value = []
    stopSessionAsyncWork()
  })

  return {
    bootstrapping,
    generating,
    polling,
    pollingJobIds,
    cancelingJobIds,
    pollNotice,
    catalog,
    capabilities,
    capabilitiesLoading,
    capabilitiesReady,
    capabilityError,
    selectedTemplate,
    promptReference,
    promptReferenceError,
    promptVariableValues,
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
    background,
    outputFormat,
    outputCompression,
    inputFidelity,
    mode,
    supportsCreate,
    supportsEdit,
    operationSupported,
    referenceUploads,
    uploadingReferences,
    editReferencesReady,
    referenceSlotCount,
    readyReferenceCount,
    maxReferenceImages,
    loadingModels,
    loadingEstimate,
    modelError,
    estimateError,
    estimate,
    jobs,
    activeJobs,
    galleryError,
    galleryLoading,
    galleryLoaded,
    galleryPage,
    galleryPageSize,
    galleryTotal,
    galleryPages,
    errorMsg,
    autoCleanup,
    showFirstWin,
    latestJob,
    activeJobCount,
    atActiveJobLimit,
    balance,
    hasApiKeys,
    showAccentColor,
    showQuality,
    showBackground,
    showOutputFormat,
    showOutputCompression,
    showInputFidelity,
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
    applyPromptUseResult,
    applyPromptVariables,
    clearPromptReference,
    saveCreationRecipe,
    onAutoCleanupChange,
    openPreview,
    closePreview,
    regenerateFromJob,
    addReferenceFiles,
    retryReference,
    removeReference,
    generate,
    cancelJob,
    removeJob,
    onAspectChange,
    onTierChange,
    ensureGalleryLoaded,
    refreshJobs,
    load,
  }
}
