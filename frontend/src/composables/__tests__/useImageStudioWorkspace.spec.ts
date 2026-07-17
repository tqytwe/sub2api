import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useImageStudioWorkspace } from '@/composables/useImageStudioWorkspace'
import {
  getStudioPendingJobsForUser,
  getStudioPendingJobForUser,
  getStudioPendingJobId,
  loadStudioDraft,
  setStudioPendingJobId,
} from '@/utils/imageStudioSession'

const mocks = vi.hoisted(() => ({
  getTemplates: vi.fn(),
  getCapabilities: vi.fn(),
  generate: vi.fn(),
  poll: vi.fn(),
  listJobs: vi.fn(),
  listActiveJobs: vi.fn(),
  cancel: vi.fn(),
  deleteReference: vi.fn(),
  estimate: vi.fn(),
  listModels: vi.fn(),
  uploadReference: vi.fn(),
  routerPush: vi.fn(),
  refreshUser: vi.fn(),
  markStudioFirstWin: vi.fn(),
  trackGrowthEvent: vi.fn(),
  trackQuestCompleteOnce: vi.fn(),
  setAuthUser: vi.fn(),
  auth: {
    user: { id: 42, balance: 10 },
    refreshUser: vi.fn(),
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.routerPush }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'en' },
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const authStore = reactive(mocks.auth)
  mocks.setAuthUser.mockImplementation((user) => {
    authStore.user = user
  })
  return {
    useAuthStore: () => authStore,
  }
})

vi.mock('@/api/keys', () => ({
  keysAPI: {
    list: vi.fn().mockResolvedValue({
      items: [{ id: 8, name: 'Images', status: 'active' }],
      pages: 1,
    }),
  },
}))

vi.mock('@/api/play', () => ({
  default: {
    getPlayHub: vi.fn().mockResolvedValue({ growth: { total_recharged: 10 } }),
  },
}))

vi.mock('@/utils/growthAnalytics', () => ({
  getStudioAutoCleanup: () => false,
  getStudioLastTemplate: () => null,
  hasStudioFirstWin: () => true,
  markStudioFirstWin: mocks.markStudioFirstWin,
  setStudioAutoCleanup: vi.fn(),
  setStudioLastTemplate: vi.fn(),
  trackGrowthEvent: mocks.trackGrowthEvent,
  trackQuestCompleteOnce: mocks.trackQuestCompleteOnce,
}))

vi.mock('@/api/imageStudio', () => ({
  isImageStudioJobActive: (job: { status: string }) =>
    job.status === 'pending' || job.status === 'running',
  isImageStudioJobTerminal: (job: { status: string }) =>
    job.status !== 'pending' && job.status !== 'running',
  default: {
    getImageStudioTemplates: (...args: unknown[]) => mocks.getTemplates(...args),
    getImageStudioCapabilities: (...args: unknown[]) => mocks.getCapabilities(...args),
    listImageStudioModels: (...args: unknown[]) => mocks.listModels(...args),
    estimateImageStudio: (...args: unknown[]) => mocks.estimate(...args),
    listImageStudioJobs: (...args: unknown[]) => mocks.listJobs(...args),
    getActiveImageStudioJobs: (...args: unknown[]) => mocks.listActiveJobs(...args),
    generateImageStudio: (...args: unknown[]) => mocks.generate(...args),
    pollImageStudioJob: (...args: unknown[]) => mocks.poll(...args),
    cancelImageStudioJob: (...args: unknown[]) => mocks.cancel(...args),
    uploadImageStudioReference: (...args: unknown[]) => mocks.uploadReference(...args),
    deleteImageStudioReference: (...args: unknown[]) => mocks.deleteReference(...args),
    deleteImageStudioJob: vi.fn(),
  },
}))

async function mountWorkspace() {
  let workspace!: ReturnType<typeof useImageStudioWorkspace>
  const wrapper = mount(defineComponent({
    setup() {
      workspace = useImageStudioWorkspace()
      return () => h('div')
    },
  }))
  await flushPromises()
  mountedWrappers.push(wrapper)
  return { wrapper, workspace }
}

const mountedWrappers: Array<ReturnType<typeof mount>> = []

function completedJob() {
  return {
    id: 'job-1',
    template_id: 'commerce-white',
    size: '1024x1024',
    count: 1,
    status: 'completed',
    estimated_cost: 0.1,
    created_at: '2026-07-16T00:00:00Z',
    assets: [{ id: 'asset-1', sort_order: 0 }],
  }
}

function completedJobWithId(id: string) {
  return {
    ...completedJob(),
    id,
    assets: [{ id: `asset-${id}`, sort_order: 0 }],
  }
}

function pendingJob(id: string) {
  return {
    ...completedJob(),
    id,
    status: 'pending',
    assets: [],
  }
}

function jobPage(
  pageJobs: ReturnType<typeof completedJob>[] = [],
  overrides: Partial<{
    total: number
    page: number
    page_size: number
    pages: number
  }> = {},
) {
  return {
    jobs: pageJobs,
    total: overrides.total ?? pageJobs.length,
    page: overrides.page ?? 1,
    page_size: overrides.page_size ?? 12,
    pages: overrides.pages ?? (pageJobs.length ? 1 : 0),
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

describe('useImageStudioWorkspace prompt UX', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    mocks.setAuthUser({ id: 42, balance: 10 })
    mocks.auth.refreshUser = mocks.refreshUser
    mocks.getTemplates.mockResolvedValue({
      intents: [{
        id: 'commerce',
        label: { zh: '电商', en: 'Commerce' },
        templates: [{
          id: 'commerce-white',
          label: { zh: '白底', en: 'White' },
          defaults: { size: '1024x1024', count: 1 },
        }],
      }],
    })
    mocks.getCapabilities.mockResolvedValue({
      aspects: [{ id: '1:1', label: { zh: '方形', en: 'Square' } }],
      tiers: [{ id: '1K', label: { zh: '1K', en: '1K' } }],
      size_options: [{ aspect: '1:1', tier: '1K', size: '1024x1024', billing_tier: '1K' }],
    })
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      writable: true,
      value: vi.fn(() => 'blob:reference'),
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      writable: true,
      value: vi.fn(),
    })
    mocks.listJobs.mockResolvedValue(jobPage())
    mocks.listActiveJobs.mockResolvedValue([])
    mocks.listModels.mockResolvedValue([{
      id: 'gpt-image-1',
      display_name: 'GPT Image 1',
      operations: ['create', 'edit'],
      supported_sizes: ['1024x1024'],
      max_reference_images: 4,
    }])
    mocks.estimate.mockResolvedValue({
      estimated_cost: 0.1,
      balance: 10,
      sufficient: true,
      model: 'gpt-image-1',
      count: 1,
      size: '1024x1024',
    })
    mocks.generate.mockResolvedValue({ job: pendingJob('job-1') })
    mocks.poll.mockResolvedValue(completedJob())
    mocks.cancel.mockImplementation(async (id: string) => ({
      ...pendingJob(id),
      status: 'cancelled',
    }))
    mocks.uploadReference.mockResolvedValue({
      id: 'ref-1',
      filename: 'reference.png',
      content_type: 'image/png',
      byte_size: 4,
      expires_at: '2026-07-23T00:00:00Z',
    })
    mocks.deleteReference.mockResolvedValue(undefined)
  })

  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
    vi.useRealTimers()
  })

  it('restores and autosaves only the current user draft fields', async () => {
    localStorage.setItem('image_studio_draft:v1:user:42', JSON.stringify({
      version: 1,
      userId: 42,
      savedAt: Date.now(),
      draft: {
        userPrompt: 'restored prompt',
        expertPrompt: 'restored expert',
        expertOpen: true,
        templateId: 'commerce-white',
        accentColor: '#abcdef',
        aspect: '1:1',
        tier: '1K',
        count: 1,
      },
    }))
    const { workspace } = await mountWorkspace()

    expect(workspace.userPrompt.value).toBe('restored prompt')
    expect(workspace.expertPrompt.value).toBe('restored expert')
    expect(workspace.expertOpen.value).toBe(true)
    expect(workspace.accentColor.value).toBe('#abcdef')

    vi.useFakeTimers()
    try {
      workspace.userPrompt.value = 'updated prompt'
      await flushPromises()
      expect(loadStudioDraft(42)?.userPrompt).toBe('restored prompt')

      await vi.advanceTimersByTimeAsync(300)
      expect(loadStudioDraft(42)?.userPrompt).toBe('updated prompt')
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not enter polling until the generation request is accepted', async () => {
    let accept!: (value: unknown) => void
    let finishPolling!: (value: unknown) => void
    mocks.generate.mockReturnValueOnce(new Promise((resolve) => { accept = resolve }))
    mocks.poll.mockReturnValueOnce(new Promise((resolve) => { finishPolling = resolve }))
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'valid prompt'

    const generation = workspace.generate()
    expect(workspace.generating.value).toBe(true)
    expect(workspace.polling.value).toBe(false)

    accept({ job: { ...completedJob(), status: 'pending', assets: [] } })
    await flushPromises()
    expect(workspace.polling.value).toBe(true)

    finishPolling(completedJob())
    await generation
    await flushPromises()
  })

  it('uploads references before submitting an edit job and sends only private reference ids', async () => {
    const createObjectURL = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:reference-1')
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const { workspace } = await mountWorkspace()
    workspace.mode.value = 'edit'
    const file = new File(['test'], 'reference.png', { type: 'image/png' })

    await workspace.addReferenceFiles([file])

    expect(mocks.uploadReference).toHaveBeenCalledWith(file, expect.any(AbortSignal))
    expect(workspace.referenceUploads.value).toMatchObject([{
      previewUrl: 'blob:reference-1',
      status: 'ready',
      referenceId: 'ref-1',
    }])
    workspace.userPrompt.value = 'replace the background'
    await expect(workspace.generate()).resolves.toBe(true)
    expect(mocks.generate).toHaveBeenCalledWith(
      expect.objectContaining({
        mode: 'edit',
        reference_ids: ['ref-1'],
      }),
      expect.any(String),
    )

    workspace.removeReference(workspace.referenceUploads.value[0].localId)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:reference-1')
    expect(mocks.deleteReference).not.toHaveBeenCalledWith('ref-1')
    createObjectURL.mockRestore()
    revokeObjectURL.mockRestore()
  })

  it('refreshes edit estimates only after references are ready and includes their ids', async () => {
    const { workspace } = await mountWorkspace()
    mocks.estimate.mockClear()

    workspace.mode.value = 'edit'
    await flushPromises()
    expect(mocks.estimate).not.toHaveBeenCalled()
    expect(workspace.estimate.value).toBeNull()

    await workspace.addReferenceFiles([
      new File(['test'], 'reference.png', { type: 'image/png' }),
    ])
    await flushPromises()

    expect(mocks.estimate).toHaveBeenLastCalledWith(expect.objectContaining({
      reference_ids: ['ref-1'],
    }))
    expect(workspace.estimate.value?.estimated_cost).toBe(0.1)

    workspace.removeReference(workspace.referenceUploads.value[0].localId)
    await flushPromises()
    expect(workspace.estimate.value).toBeNull()
  })

  it('deletes a ready reference when the user removes it before submission', async () => {
    const { workspace } = await mountWorkspace()
    workspace.mode.value = 'edit'
    await workspace.addReferenceFiles([
      new File(['test'], 'reference.png', { type: 'image/png' }),
    ])

    workspace.removeReference(workspace.referenceUploads.value[0].localId)
    await flushPromises()

    expect(mocks.deleteReference).toHaveBeenCalledTimes(1)
    expect(mocks.deleteReference).toHaveBeenCalledWith('ref-1')
  })

  it('deletes discarded ready references when the model limit shrinks or mode returns to create', async () => {
    const { workspace } = await mountWorkspace()
    workspace.mode.value = 'edit'
    workspace.referenceUploads.value = ['ref-1', 'ref-2', 'ref-3'].map((referenceId, index) => ({
      localId: `local-${index + 1}`,
      file: new File(['test'], `${referenceId}.png`, { type: 'image/png' }),
      previewUrl: `blob:${referenceId}`,
      status: 'ready' as const,
      referenceId,
      error: '',
    }))
    workspace.availableModels.value = [{
      id: 'one-reference-model',
      display_name: 'One reference model',
      operations: ['create', 'edit'],
      supported_sizes: ['1024x1024'],
      max_reference_images: 1,
    }]

    workspace.selectedModel.value = 'one-reference-model'
    await flushPromises()

    expect(workspace.referenceUploads.value.map((item) => item.referenceId)).toEqual(['ref-1'])
    expect(mocks.deleteReference).toHaveBeenCalledWith('ref-2')
    expect(mocks.deleteReference).toHaveBeenCalledWith('ref-3')

    workspace.mode.value = 'create'
    await flushPromises()
    expect(mocks.deleteReference).toHaveBeenCalledWith('ref-1')
  })

  it('enforces model operations and treats a missing edit reference limit as unsupported', async () => {
    const { workspace } = await mountWorkspace()
    workspace.availableModels.value = [{
      id: 'create-only',
      display_name: 'Create only',
      operations: ['create'],
      supported_sizes: ['1024x1024'],
      max_reference_images: 0,
    }]
    workspace.selectedModel.value = 'create-only'
    await flushPromises()

    expect(workspace.supportsCreate.value).toBe(true)
    expect(workspace.supportsEdit.value).toBe(false)
    expect(workspace.maxReferenceImages.value).toBe(0)
    workspace.mode.value = 'edit'
    await flushPromises()
    expect(workspace.mode.value).toBe('create')

    mocks.uploadReference.mockClear()
    await workspace.addReferenceFiles([
      new File(['test'], 'reference.png', { type: 'image/png' }),
    ])
    expect(mocks.uploadReference).not.toHaveBeenCalled()

    workspace.availableModels.value = [{
      id: 'edit-only',
      display_name: 'Edit only',
      operations: ['edit'],
      supported_sizes: ['1024x1024'],
      max_reference_images: 1,
    }]
    workspace.selectedModel.value = 'edit-only'
    await flushPromises()

    expect(workspace.supportsCreate.value).toBe(false)
    expect(workspace.supportsEdit.value).toBe(true)
    expect(workspace.mode.value).toBe('edit')
    expect(workspace.maxReferenceImages.value).toBe(1)
  })

  it('fails closed when a model does not declare supported operations', async () => {
    const { workspace } = await mountWorkspace()
    workspace.availableModels.value = [{
      id: 'missing-operations',
      display_name: 'Missing operations',
      supported_sizes: ['1024x1024'],
      max_reference_images: 4,
    }]
    workspace.selectedModel.value = 'missing-operations'
    await flushPromises()

    expect(workspace.supportsCreate.value).toBe(false)
    expect(workspace.supportsEdit.value).toBe(false)
    expect(workspace.operationSupported.value).toBe(false)
    workspace.userPrompt.value = 'try a model with incomplete capabilities'
    await expect(workspace.generate()).resolves.toBe(false)
    expect(workspace.errorMsg.value).toBe('imageStudio.operationUnsupported')
  })

  it('deletes unclaimed ready references when the workspace unmounts', async () => {
    const { wrapper, workspace } = await mountWorkspace()
    workspace.mode.value = 'edit'
    workspace.referenceUploads.value = [{
      localId: 'local-unmount',
      file: new File(['test'], 'reference.png', { type: 'image/png' }),
      previewUrl: 'blob:unmount',
      status: 'ready',
      referenceId: 'ref-unmount',
      error: '',
    }]

    wrapper.unmount()
    await flushPromises()

    expect(mocks.deleteReference).toHaveBeenCalledWith('ref-unmount')
  })

  it('keeps a failed reference upload retryable and blocks edit generation until it succeeds', async () => {
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:reference-failed')
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    mocks.uploadReference
      .mockRejectedValueOnce({ message: 'upload unavailable' })
      .mockResolvedValueOnce({
        id: 'ref-retried',
        filename: 'reference.webp',
        content_type: 'image/webp',
        byte_size: 4,
        expires_at: '2026-07-23T00:00:00Z',
      })
    const { workspace } = await mountWorkspace()
    workspace.mode.value = 'edit'
    workspace.userPrompt.value = 'edit this image'

    await workspace.addReferenceFiles([
      new File(['test'], 'reference.webp', { type: 'image/webp' }),
    ])

    expect(workspace.referenceUploads.value[0].status).toBe('failed')
    await expect(workspace.generate()).resolves.toBe(false)
    expect(mocks.generate).not.toHaveBeenCalled()

    await workspace.retryReference(workspace.referenceUploads.value[0].localId)
    expect(workspace.referenceUploads.value[0]).toMatchObject({
      status: 'ready',
      referenceId: 'ref-retried',
    })
    await expect(workspace.generate()).resolves.toBe(true)
    expect(mocks.generate).toHaveBeenCalledWith(
      expect.objectContaining({
        mode: 'edit',
        reference_ids: ['ref-retried'],
      }),
      expect.any(String),
    )
  })

  it('submits only capability-valid advanced image options and resets them across model changes', async () => {
    const { workspace } = await mountWorkspace()
    workspace.availableModels.value = [{
      id: 'gpt-image-1.5',
      display_name: 'GPT Image 1.5',
      platform: 'openai',
      operations: ['create', 'edit'],
      supported_sizes: ['1024x1024'],
      supported_qualities: ['auto', 'high'],
      supported_backgrounds: ['auto', 'opaque', 'transparent'],
      supported_output_formats: ['png', 'jpeg', 'webp'],
      supported_input_fidelities: ['low', 'high'],
      input_fidelity_mode: 'selectable',
      output_compression: { min: 0, max: 100, formats: ['jpeg', 'webp'] },
      max_reference_images: 4,
      default_quality: 'auto',
      default_background: 'auto',
      default_output_format: 'png',
      default_input_fidelity: 'low',
    }]
    workspace.selectedModel.value = 'gpt-image-1.5'
    await flushPromises()

    workspace.mode.value = 'edit'
    workspace.referenceUploads.value = [{
      localId: 'local-ready',
      file: new File(['test'], 'reference.png', { type: 'image/png' }),
      previewUrl: 'blob:ready',
      status: 'ready',
      referenceId: 'ref-ready',
      error: '',
    }]
    workspace.background.value = 'transparent'
    workspace.outputFormat.value = 'webp'
    workspace.outputCompression.value = 82
    workspace.inputFidelity.value = 'high'
    workspace.quality.value = 'high'
    workspace.userPrompt.value = 'edit with advanced settings'

    await expect(workspace.generate()).resolves.toBe(true)
    expect(mocks.generate).toHaveBeenCalledWith(
      expect.objectContaining({
        background: 'transparent',
        output_format: 'webp',
        output_compression: 82,
        input_fidelity: 'high',
        quality: 'high',
      }),
      expect.any(String),
    )

    workspace.availableModels.value = [{
      id: 'grok-imagine-image-quality',
      display_name: 'Grok Imagine',
      platform: 'grok',
      operations: ['create', 'edit'],
      sizing_kind: 'aspect_resolution',
      supported_sizes: ['1024x1024'],
      supported_qualities: ['standard'],
      supported_output_formats: ['jpeg'],
      max_reference_images: 3,
      default_quality: 'standard',
      default_output_format: 'jpeg',
    }]
    workspace.selectedModel.value = 'grok-imagine-image-quality'
    await flushPromises()

    expect(workspace.background.value).toBe('')
    expect(workspace.outputFormat.value).toBe('jpeg')
    expect(workspace.inputFidelity.value).toBe('')
    expect(workspace.quality.value).toBe('standard')
    expect(workspace.showOutputCompression.value).toBe(false)
  })

  it('clears compression left behind by a model that supported the same output format', async () => {
    const { workspace } = await mountWorkspace()
    workspace.availableModels.value = [{
      id: 'compression-model',
      display_name: 'Compression model',
      operations: ['create', 'edit'],
      supported_sizes: ['1024x1024'],
      supported_output_formats: ['jpeg'],
      default_output_format: 'jpeg',
      output_compression: { min: 0, max: 100, formats: ['jpeg'] },
    }, {
      id: 'plain-model',
      display_name: 'Plain model',
      operations: ['create', 'edit'],
      supported_sizes: ['1024x1024'],
      supported_output_formats: ['jpeg'],
      default_output_format: 'jpeg',
    }]
    workspace.selectedModel.value = 'compression-model'
    await flushPromises()
    workspace.outputCompression.value = 42

    workspace.selectedModel.value = 'plain-model'
    await flushPromises()

    expect(workspace.outputFormat.value).toBe('jpeg')
    expect(workspace.showOutputCompression.value).toBe(false)
    expect(workspace.outputCompression.value).toBe(85)
  })

  it('keeps the latest model response and loading state when API keys change quickly', async () => {
    const { workspace } = await mountWorkspace()
    const first = deferred<Array<{ id: string; display_name: string; supported_sizes: string[] }>>()
    const second = deferred<Array<{ id: string; display_name: string; supported_sizes: string[] }>>()
    mocks.listModels.mockReset()
    mocks.listModels
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)

    workspace.apiKeyId.value = 9
    await nextTick()
    workspace.apiKeyId.value = 10
    await nextTick()

    expect(workspace.loadingModels.value).toBe(true)
    first.resolve([{
      id: 'stale-model',
      display_name: 'Stale model',
      supported_sizes: ['1024x1024'],
    }])
    await flushPromises()
    expect(workspace.loadingModels.value).toBe(true)
    expect(workspace.availableModels.value).toEqual([])

    second.resolve([{
      id: 'latest-model',
      display_name: 'Latest model',
      supported_sizes: ['1024x1024'],
    }])
    await flushPromises()

    expect(workspace.loadingModels.value).toBe(false)
    expect(workspace.selectedModel.value).toBe('latest-model')
  })

  it('keeps the latest estimate and loading state when selections change quickly', async () => {
    const { workspace } = await mountWorkspace()
    workspace.availableModels.value = [{
      id: 'model-old',
      display_name: 'Old model',
      operations: ['create', 'edit'],
      supported_sizes: ['1024x1024'],
    }, {
      id: 'model-new',
      display_name: 'New model',
      operations: ['create', 'edit'],
      supported_sizes: ['1024x1024'],
    }]
    workspace.selectedModel.value = 'model-old'
    await flushPromises()

    const first = deferred<{
      estimated_cost: number
      balance: number
      sufficient: boolean
      model: string
      count: number
      size: string
    }>()
    const second = deferred<{
      estimated_cost: number
      balance: number
      sufficient: boolean
      model: string
      count: number
      size: string
    }>()
    mocks.estimate.mockReset()
    mocks.estimate
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)

    workspace.count.value = 2
    await nextTick()
    workspace.selectedModel.value = 'model-new'
    await nextTick()

    expect(workspace.loadingEstimate.value).toBe(true)
    first.resolve({
      estimated_cost: 0.2,
      balance: 10,
      sufficient: true,
      model: 'model-old',
      count: 2,
      size: '1024x1024',
    })
    await flushPromises()
    expect(workspace.loadingEstimate.value).toBe(true)
    expect(workspace.estimate.value?.model).not.toBe('model-old')

    second.resolve({
      estimated_cost: 0.3,
      balance: 10,
      sufficient: true,
      model: 'model-new',
      count: 2,
      size: '1024x1024',
    })
    await flushPromises()

    expect(workspace.loadingEstimate.value).toBe(false)
    expect(workspace.estimate.value?.model).toBe('model-new')
  })

  it('reuses the idempotency key after a failed POST and rotates it after acceptance', async () => {
    mocks.generate
      .mockRejectedValueOnce(Object.assign(new Error('Network Error'), { code: 'ERR_NETWORK' }))
      .mockResolvedValueOnce({ job: completedJob() })
      .mockResolvedValueOnce({ job: { ...completedJob(), id: 'job-2' } })
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'same submitted request'

    await expect(workspace.generate()).resolves.toBe(false)
    await expect(workspace.generate()).resolves.toBe(true)
    workspace.userPrompt.value = 'new user submission'
    await expect(workspace.generate()).resolves.toBe(true)

    const keys = mocks.generate.mock.calls.map((call) => call[1])
    expect(keys[0]).toEqual(expect.any(String))
    expect(keys[1]).toBe(keys[0])
    expect(keys[2]).not.toBe(keys[1])
  })

  it('accepts a second job after the first POST and blocks a third active job', async () => {
    mocks.generate
      .mockResolvedValueOnce({ job: pendingJob('job-1') })
      .mockResolvedValueOnce({ job: pendingJob('job-2') })
    mocks.poll.mockReturnValue(new Promise(() => {}))
    const { workspace } = await mountWorkspace()

    workspace.userPrompt.value = 'first prompt'
    await expect(workspace.generate()).resolves.toBe(true)
    expect(workspace.generating.value).toBe(false)
    expect(workspace.activeJobCount.value).toBe(1)

    workspace.userPrompt.value = 'second prompt'
    await expect(workspace.generate()).resolves.toBe(true)
    expect(workspace.activeJobCount.value).toBe(2)
    expect(workspace.atActiveJobLimit.value).toBe(true)

    workspace.userPrompt.value = 'third prompt'
    await expect(workspace.generate()).resolves.toBe(false)
    expect(mocks.generate).toHaveBeenCalledTimes(2)
  })

  it('restores and polls every active or locally pending job independently', async () => {
    setStudioPendingJobId('job-local', {
      userId: 42,
      submittedPrompt: { userPrompt: 'local pending prompt', expertPrompt: '' },
    })
    mocks.listActiveJobs.mockResolvedValue([pendingJob('job-active')])
    mocks.poll.mockImplementation(async (id: string) => {
      if (id === 'job-active') {
        return {
          ...pendingJob(id),
          status: 'failed',
          error_message: 'first failed',
        }
      }
      return {
        ...completedJob(),
        id,
        status: 'partial',
        success_count: 1,
        fail_count: 1,
      }
    })

    const { workspace } = await mountWorkspace()
    await flushPromises()

    expect(mocks.poll.mock.calls.map(([id]) => id).sort()).toEqual(['job-active', 'job-local'])
    expect(workspace.activeJobs.value).toEqual([])
    expect(workspace.errorMsg.value).toBe('first failed')
    expect(workspace.latestJob.value?.id).toBe('job-local')
    expect(workspace.latestJob.value?.status).toBe('partial')
    expect(workspace.latestJob.value?.assets).toHaveLength(1)
  })

  it('cancels one active job without clearing another pending snapshot', async () => {
    const { workspace } = await mountWorkspace()
    setStudioPendingJobId('job-1', {
      userId: 42,
      submittedPrompt: { userPrompt: 'first', expertPrompt: '' },
    })
    setStudioPendingJobId('job-2', {
      userId: 42,
      submittedPrompt: { userPrompt: 'second', expertPrompt: '' },
    })
    workspace.jobs.value = [pendingJob('job-1'), pendingJob('job-2')]

    await workspace.cancelJob('job-1')

    expect(mocks.cancel).toHaveBeenCalledWith('job-1')
    expect(workspace.jobs.value.find((job) => job.id === 'job-1')?.status).toBe('cancelled')
    expect(getStudioPendingJobsForUser(42).map((job) => job.jobId)).toEqual(['job-2'])
  })

  it('clears restored prompt drafts when a pending job completes after refresh', async () => {
    localStorage.setItem('image_studio_pending_job_id', 'job-1')
    localStorage.setItem('image_studio_pending_job_context:v1', JSON.stringify({
      version: 1,
      jobId: 'job-1',
      userId: 42,
      submittedPrompt: {
        userPrompt: 'already submitted prompt',
        expertPrompt: 'already submitted expert prompt',
      },
    }))
    localStorage.setItem('image_studio_draft:v1:user:42', JSON.stringify({
      version: 1,
      userId: 42,
      savedAt: Date.now(),
      draft: {
        userPrompt: 'already submitted prompt',
        expertPrompt: 'already submitted expert prompt',
        expertOpen: true,
        templateId: 'commerce-white',
        accentColor: '#abcdef',
        aspect: '1:1',
        tier: '1K',
        count: 1,
      },
    }))

    const { workspace } = await mountWorkspace()

    expect(workspace.userPrompt.value).toBe('')
    expect(workspace.expertPrompt.value).toBe('')
    expect(loadStudioDraft(42)?.userPrompt).toBe('')
    expect(loadStudioDraft(42)?.expertPrompt).toBe('')
  })

  it('retains restored prompt drafts when a resumed job has no assets', async () => {
    mocks.poll.mockResolvedValueOnce({ ...completedJob(), assets: [] })
    localStorage.setItem('image_studio_pending_job_id', 'job-1')
    localStorage.setItem('image_studio_pending_job_context:v1', JSON.stringify({
      version: 1,
      jobId: 'job-1',
      userId: 42,
      submittedPrompt: {
        userPrompt: 'retry this prompt',
        expertPrompt: 'retry this expert prompt',
      },
    }))
    localStorage.setItem('image_studio_draft:v1:user:42', JSON.stringify({
      version: 1,
      userId: 42,
      savedAt: Date.now(),
      draft: {
        userPrompt: 'retry this prompt',
        expertPrompt: 'retry this expert prompt',
        expertOpen: true,
        templateId: 'commerce-white',
        accentColor: '#abcdef',
        aspect: '1:1',
        tier: '1K',
        count: 1,
      },
    }))

    const { workspace } = await mountWorkspace()

    expect(workspace.errorMsg.value).toBe('imageStudio.assetsMissingHint')
    expect(workspace.userPrompt.value).toBe('retry this prompt')
    expect(workspace.expertPrompt.value).toBe('retry this expert prompt')
    expect(loadStudioDraft(42)?.userPrompt).toBe('retry this prompt')
  })

  it('retains the submitted draft when a resumed job fails', async () => {
    mocks.poll.mockResolvedValueOnce({
      ...completedJob(),
      status: 'failed',
      assets: [],
      error_message: 'provider failed',
    })
    setStudioPendingJobId('job-1', {
      userId: 42,
      submittedPrompt: {
        userPrompt: 'keep failed prompt',
        expertPrompt: 'keep failed expert',
      },
    })
    localStorage.setItem('image_studio_draft:v1:user:42', JSON.stringify({
      version: 1,
      userId: 42,
      savedAt: Date.now(),
      draft: {
        userPrompt: 'keep failed prompt',
        expertPrompt: 'keep failed expert',
        expertOpen: true,
        templateId: 'commerce-white',
        accentColor: '#abcdef',
        aspect: '1:1',
        tier: '1K',
        count: 1,
      },
    }))

    const { workspace } = await mountWorkspace()
    await flushPromises()

    expect(workspace.errorMsg.value).toBe('provider failed')
    expect(workspace.userPrompt.value).toBe('keep failed prompt')
    expect(loadStudioDraft(42)?.userPrompt).toBe('keep failed prompt')
    expect(getStudioPendingJobsForUser(42)).toEqual([])
  })

  it('retains a newer prompt when an older generation completes', async () => {
    let finishPolling!: (value: unknown) => void
    mocks.poll.mockReturnValueOnce(new Promise((resolve) => { finishPolling = resolve }))
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'submitted prompt'
    workspace.expertPrompt.value = 'submitted expert'

    const generation = workspace.generate()
    await flushPromises()
    workspace.userPrompt.value = 'new prompt while waiting'
    workspace.expertPrompt.value = 'new expert while waiting'
    await flushPromises()
    finishPolling(completedJob())

    await expect(generation).resolves.toBe(true)
    await flushPromises()
    expect(workspace.userPrompt.value).toBe('new prompt while waiting')
    expect(workspace.expertPrompt.value).toBe('new expert while waiting')
    expect(loadStudioDraft(42)?.userPrompt).toBe('new prompt while waiting')
  })

  it('snapshots advanced settings so an older job cannot clear a changed draft', async () => {
    const poll = deferred<ReturnType<typeof completedJob>>()
    mocks.poll.mockReturnValueOnce(poll.promise)
    const { workspace } = await mountWorkspace()
    workspace.availableModels.value = [{
      id: 'advanced-model',
      display_name: 'Advanced model',
      operations: ['create', 'edit'],
      supported_sizes: ['1024x1024'],
      supported_backgrounds: ['auto', 'transparent', 'opaque'],
      supported_output_formats: ['png', 'webp'],
      supported_input_fidelities: ['low', 'high'],
      input_fidelity_mode: 'selectable',
      output_compression: { min: 0, max: 100, formats: ['webp'] },
      max_reference_images: 4,
      default_background: 'auto',
      default_output_format: 'png',
      default_input_fidelity: 'low',
    }]
    workspace.selectedModel.value = 'advanced-model'
    await flushPromises()
    workspace.mode.value = 'edit'
    workspace.referenceUploads.value = [{
      localId: 'local-snapshot',
      file: new File(['test'], 'reference.png', { type: 'image/png' }),
      previewUrl: 'blob:snapshot',
      status: 'ready',
      referenceId: 'ref-snapshot',
      error: '',
    }]
    workspace.userPrompt.value = 'keep this prompt when settings change'
    workspace.background.value = 'transparent'
    workspace.outputFormat.value = 'webp'
    workspace.outputCompression.value = 82
    workspace.inputFidelity.value = 'high'

    await expect(workspace.generate()).resolves.toBe(true)
    expect(getStudioPendingJobForUser(42)?.submittedPrompt).toMatchObject({
      background: 'transparent',
      outputFormat: 'webp',
      outputCompression: 82,
      inputFidelity: 'high',
      mode: 'edit',
      referenceIds: ['ref-snapshot'],
    })

    workspace.background.value = 'opaque'
    workspace.outputCompression.value = 55
    await flushPromises()
    poll.resolve(completedJob())
    await flushPromises()

    expect(workspace.userPrompt.value).toBe('keep this prompt when settings change')
    expect(workspace.background.value).toBe('opaque')
    expect(workspace.outputCompression.value).toBe(55)
    expect(loadStudioDraft(42)?.userPrompt).toBe('keep this prompt when settings change')
  })

  it('aborts a removed upload and deletes a reference returned after cancellation', async () => {
    const upload = deferred<{
      id: string
      filename: string
      content_type: string
      byte_size: number
      expires_at: string
    }>()
    mocks.uploadReference.mockReset()
    mocks.uploadReference.mockReturnValueOnce(upload.promise)
    const { workspace } = await mountWorkspace()
    workspace.mode.value = 'edit'
    const file = new File(['test'], 'reference.png', { type: 'image/png' })

    const adding = workspace.addReferenceFiles([file])
    await nextTick()
    const signal = mocks.uploadReference.mock.calls[0][1] as AbortSignal
    const localId = workspace.referenceUploads.value[0].localId

    workspace.removeReference(localId)
    expect(signal.aborted).toBe(true)
    expect(workspace.referenceUploads.value).toEqual([])

    upload.resolve({
      id: 'ref-late',
      filename: 'reference.png',
      content_type: 'image/png',
      byte_size: 4,
      expires_at: '2026-07-23T00:00:00Z',
    })
    await adding
    await flushPromises()

    expect(mocks.deleteReference).toHaveBeenCalledWith('ref-late')
    expect(workspace.referenceUploads.value).toEqual([])
  })

  it('fails closed when image capabilities cannot be loaded', async () => {
    mocks.getCapabilities.mockRejectedValueOnce(new Error('capability service unavailable'))
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'must not submit without capability data'

    expect(workspace.capabilitiesReady.value).toBe(false)
    expect(workspace.capabilityError.value).toBe('imageStudio.loadCapabilitiesFailed')
    await expect(workspace.generate()).resolves.toBe(false)
    expect(workspace.errorMsg.value).toBe('imageStudio.loadCapabilitiesFailed')
    expect(mocks.generate).not.toHaveBeenCalled()
  })

  it('clears prompt drafts after success and retains them after a structured API failure', async () => {
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'keep settings, clear me'
    workspace.expertOpen.value = true
    workspace.expertPrompt.value = 'clear expert too'
    await flushPromises()

    await expect(workspace.generate()).resolves.toBe(true)
    await flushPromises()
    expect(workspace.userPrompt.value).toBe('')
    expect(workspace.expertPrompt.value).toBe('')
    expect(loadStudioDraft(42)?.templateId).toBe('commerce-white')

    workspace.userPrompt.value = 'retain on failure'
    workspace.expertPrompt.value = 'retain expert'
    mocks.generate.mockRejectedValueOnce({
      status: 400,
      reason: 'IMAGE_STUDIO_PROMPT_REJECTED',
      message: 'The provider rejected this prompt.',
    })

    await expect(workspace.generate()).resolves.toBe(false)
    expect(workspace.errorMsg.value).toBe('The provider rejected this prompt.')
    expect(workspace.userPrompt.value).toBe('retain on failure')
    expect(loadStudioDraft(42)?.userPrompt).toBe('retain on failure')
  })

  it('keeps generation successful when the post-success balance refresh fails', async () => {
    mocks.refreshUser.mockRejectedValueOnce(new Error('profile refresh failed'))
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'successful image prompt'

    await expect(workspace.generate()).resolves.toBe(true)
    await flushPromises()
    expect(workspace.latestJob.value?.id).toBe('job-1')
    expect(workspace.userPrompt.value).toBe('')
    expect(loadStudioDraft(42)?.userPrompt).toBe('')
    expect(workspace.errorMsg.value).toBe('')
  })

  it('keeps timeout copy visible and resumes polling slow jobs', async () => {
    vi.useFakeTimers()
    mocks.poll
      .mockRejectedValueOnce(new Error('IMAGE_STUDIO_POLL_TIMEOUT'))
      .mockResolvedValueOnce(completedJob())
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'valid prompt'

    await expect(workspace.generate()).resolves.toBe(true)
    await flushPromises()
    expect(workspace.errorMsg.value).toBe('imageStudio.pollTimeout')
    expect(workspace.errorMsg.value).not.toBe('imageStudio.generateFailed')
    expect(getStudioPendingJobId()).toBeNull()
    expect(getStudioPendingJobForUser(42)?.jobId).toBe('job-1')

    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()
    expect(mocks.poll).toHaveBeenCalledTimes(2)
    expect(workspace.latestJob.value?.id).toBe('job-1')
  })

  it('ignores a pending job snapshot owned by another signed-in user', async () => {
    localStorage.setItem('image_studio_pending_job_id', 'job-1')
    localStorage.setItem('image_studio_pending_job_context:v1', JSON.stringify({
      version: 1,
      jobId: 'job-1',
      userId: 42,
      submittedPrompt: {
        userPrompt: 'user 42 prompt',
        expertPrompt: '',
      },
    }))
    mocks.setAuthUser({ id: 7, balance: 10 })

    await mountWorkspace()

    expect(mocks.poll).not.toHaveBeenCalled()
    expect(getStudioPendingJobId()).toBe('job-1')
  })

  it('surfaces gallery loading errors and clears them after retry', async () => {
    mocks.listJobs.mockRejectedValueOnce({ message: 'Gallery service unavailable.' })
    const { workspace } = await mountWorkspace()

    expect(workspace.galleryError.value).toBe('Gallery service unavailable.')

    mocks.listJobs.mockResolvedValueOnce(jobPage())
    await workspace.refreshJobs()
    expect(workspace.galleryError.value).toBe('')
  })

  it('does not let the first gallery request block workspace bootstrapping', async () => {
    let resolveGallery!: (value: ReturnType<typeof jobPage>) => void
    mocks.listJobs.mockReturnValueOnce(new Promise((resolve) => {
      resolveGallery = resolve
    }))

    const { workspace } = await mountWorkspace()

    expect(workspace.bootstrapping.value).toBe(false)
    expect(workspace.galleryLoading.value).toBe(true)
    expect(workspace.selectedTemplate.value?.id).toBe('commerce-white')

    resolveGallery(jobPage([completedJob()]))
    await flushPromises()
    expect(workspace.galleryLoading.value).toBe(false)
    expect(workspace.jobs.value).toHaveLength(1)
  })

  it('does not let model discovery block the first workspace render', async () => {
    const models = deferred<Array<{ id: string; display_name: string }>>()
    mocks.listModels.mockReset()
    mocks.listModels.mockReturnValueOnce(models.promise)
    let workspace!: ReturnType<typeof useImageStudioWorkspace>
    const wrapper = mount(defineComponent({
      setup() {
        workspace = useImageStudioWorkspace()
        return () => h('div')
      },
    }))
    mountedWrappers.push(wrapper)

    await flushPromises()

    expect(workspace.bootstrapping.value).toBe(false)
    expect(workspace.loadingModels.value).toBe(true)
    expect(workspace.selectedTemplate.value?.id).toBe('commerce-white')

    models.resolve([{ id: 'gpt-image-1', display_name: 'GPT Image 1' }])
    await flushPromises()
    expect(workspace.loadingModels.value).toBe(false)
    expect(workspace.selectedModel.value).toBe('gpt-image-1')
  })

  it('loads history in 12-item pages and replaces the previous terminal page', async () => {
    const pageTwoJob = { ...completedJob(), id: 'job-page-2' }
    const { workspace } = await mountWorkspace()
    workspace.jobs.value = [completedJob()]
    mocks.listJobs.mockResolvedValueOnce(jobPage([pageTwoJob], {
      total: 25,
      page: 2,
      page_size: 12,
      pages: 3,
    }))

    await workspace.refreshJobs(2)

    expect(mocks.listJobs).toHaveBeenLastCalledWith(2, 12)
    expect(workspace.galleryPage.value).toBe(2)
    expect(workspace.galleryTotal.value).toBe(25)
    expect(workspace.galleryPages.value).toBe(3)
    expect(workspace.jobs.value.map((job) => job.id)).toEqual(['job-page-2'])
  })

  it('features the first displayable terminal job from page one without success side effects', async () => {
    localStorage.setItem('image_studio_draft:v1:user:42', JSON.stringify({
      version: 1,
      userId: 42,
      savedAt: Date.now(),
      draft: {
        userPrompt: 'keep this returning-user draft',
        expertPrompt: 'keep expert draft',
        expertOpen: true,
        templateId: 'commerce-white',
        accentColor: '#abcdef',
        aspect: '1:1',
        tier: '1K',
        count: 1,
      },
    }))
    const { workspace } = await mountWorkspace()
    const featured = {
      ...completedJobWithId('job-featured-history'),
      status: 'partial',
    }
    mocks.trackGrowthEvent.mockClear()
    mocks.trackQuestCompleteOnce.mockClear()
    mocks.markStudioFirstWin.mockClear()
    mocks.refreshUser.mockClear()
    mocks.listJobs.mockResolvedValueOnce(jobPage([
      { ...completedJobWithId('job-failed'), status: 'failed', assets: [] },
      { ...completedJobWithId('job-empty'), assets: [] },
      featured,
      completedJobWithId('job-later-history'),
    ]))

    await workspace.refreshJobs(1)

    expect(workspace.latestJob.value?.id).toBe('job-featured-history')
    expect(workspace.latestJob.value?.status).toBe('partial')
    expect(mocks.trackGrowthEvent).not.toHaveBeenCalled()
    expect(mocks.trackQuestCompleteOnce).not.toHaveBeenCalled()
    expect(mocks.markStudioFirstWin).not.toHaveBeenCalled()
    expect(mocks.refreshUser).not.toHaveBeenCalled()
    expect(workspace.userPrompt.value).toBe('keep this returning-user draft')
    expect(workspace.expertPrompt.value).toBe('keep expert draft')
    expect(loadStudioDraft(42)?.userPrompt).toBe('keep this returning-user draft')
  })

  it('keeps active jobs separate from every 12-item history page', async () => {
    const history = Array.from({ length: 12 }, (_, index) =>
      completedJobWithId(`job-page-2-${index + 1}`))
    mocks.listJobs.mockResolvedValueOnce(jobPage(history, {
      total: 25,
      page: 2,
      page_size: 12,
      pages: 3,
    }))
    mocks.listActiveJobs.mockResolvedValueOnce([pendingJob('job-active')])
    mocks.poll.mockReturnValueOnce(new Promise(() => {}))

    const { workspace } = await mountWorkspace()
    await flushPromises()

    expect(workspace.activeJobs.value.map((job) => job.id)).toEqual(['job-active'])
    expect(workspace.jobs.value).toHaveLength(12)
    expect(workspace.jobs.value.map((job) => job.id)).not.toContain('job-active')
  })

  it('does not regress a terminal job when a stale gallery refresh still says running', async () => {
    const { workspace } = await mountWorkspace()
    workspace.jobs.value = [completedJob()]
    mocks.listJobs.mockResolvedValueOnce(jobPage([{
      ...pendingJob('job-1'),
      status: 'running',
    }]))

    await workspace.refreshJobs()

    expect(workspace.jobs.value[0].status).toBe('completed')
    expect(workspace.jobs.value[0].assets).toHaveLength(1)
  })

  it('does not let an older submission complete over the latest featured result', async () => {
    const firstPoll = deferred<ReturnType<typeof completedJob>>()
    const secondPoll = deferred<ReturnType<typeof completedJob>>()
    mocks.generate
      .mockResolvedValueOnce({ job: pendingJob('job-older') })
      .mockResolvedValueOnce({ job: pendingJob('job-newer') })
    mocks.poll.mockImplementation((id: string) => (
      id === 'job-older' ? firstPoll.promise : secondPoll.promise
    ))
    const { workspace } = await mountWorkspace()

    workspace.userPrompt.value = 'older submission'
    await expect(workspace.generate()).resolves.toBe(true)
    workspace.userPrompt.value = 'newer submission'
    await expect(workspace.generate()).resolves.toBe(true)

    secondPoll.resolve(completedJobWithId('job-newer'))
    await flushPromises()
    firstPoll.resolve(completedJobWithId('job-older'))
    await flushPromises()

    expect(workspace.latestJob.value?.id).toBe('job-newer')
  })

  it('ignores an old generate response after the signed-in user changes', async () => {
    const oldGenerate = deferred<{ job: ReturnType<typeof pendingJob> }>()
    mocks.generate.mockReturnValueOnce(oldGenerate.promise)
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'old user submission'
    const generation = workspace.generate()

    mocks.setAuthUser({ id: 7, balance: 20 })
    await flushPromises()
    oldGenerate.resolve({ job: pendingJob('job-old-user') })
    await generation
    await flushPromises()

    expect(workspace.activeJobs.value.map((job) => job.id)).not.toContain('job-old-user')
    expect(workspace.jobs.value.map((job) => job.id)).not.toContain('job-old-user')
    expect(workspace.latestJob.value).toBeNull()
  })

  it('ignores an old poll completion after the signed-in user changes', async () => {
    const oldPoll = deferred<ReturnType<typeof completedJob>>()
    mocks.poll.mockReturnValueOnce(oldPoll.promise)
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'old user polling'
    await expect(workspace.generate()).resolves.toBe(true)

    mocks.setAuthUser({ id: 7, balance: 20 })
    await flushPromises()
    oldPoll.resolve(completedJobWithId('job-1'))
    await flushPromises()

    expect(workspace.latestJob.value).toBeNull()
    expect(workspace.activeJobs.value.map((job) => job.id)).not.toContain('job-1')
    expect(workspace.jobs.value.map((job) => job.id)).not.toContain('job-1')
  })

  it('ignores an older load response after a new user session has loaded', async () => {
    const oldTemplates = deferred<{
      intents: Array<{
        id: string
        label: { zh: string; en: string }
        templates: Array<{
          id: string
          label: { zh: string; en: string }
          defaults: { size: string; count: number }
        }>
      }>
    }>()
    mocks.getTemplates
      .mockReturnValueOnce(oldTemplates.promise)
      .mockResolvedValueOnce({
        intents: [{
          id: 'new-user',
          label: { zh: '新用户', en: 'New user' },
          templates: [{
            id: 'new-user-template',
            label: { zh: '新模板', en: 'New template' },
            defaults: { size: '1024x1024', count: 1 },
          }],
        }],
      })

    let workspace!: ReturnType<typeof useImageStudioWorkspace>
    const wrapper = mount(defineComponent({
      setup() {
        workspace = useImageStudioWorkspace()
        return () => h('div')
      },
    }))
    mountedWrappers.push(wrapper)
    await flushPromises()

    mocks.setAuthUser({ id: 7, balance: 20 })
    await flushPromises()
    expect(workspace.selectedTemplate.value?.id).toBe('new-user-template')

    oldTemplates.resolve({
      intents: [{
        id: 'old-user',
        label: { zh: '旧用户', en: 'Old user' },
        templates: [{
          id: 'old-user-template',
          label: { zh: '旧模板', en: 'Old template' },
          defaults: { size: '1024x1024', count: 1 },
        }],
      }],
    })
    await flushPromises()

    expect(workspace.selectedTemplate.value?.id).toBe('new-user-template')
  })
})
