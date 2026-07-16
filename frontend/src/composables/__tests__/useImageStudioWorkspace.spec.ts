import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useImageStudioWorkspace } from '@/composables/useImageStudioWorkspace'
import {
  getStudioPendingJobForUser,
  getStudioPendingJobId,
  loadStudioDraft,
} from '@/utils/imageStudioSession'

const mocks = vi.hoisted(() => ({
  generate: vi.fn(),
  poll: vi.fn(),
  listJobs: vi.fn(),
  routerPush: vi.fn(),
  refreshUser: vi.fn(),
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

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => mocks.auth,
}))

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
  markStudioFirstWin: vi.fn(),
  setStudioAutoCleanup: vi.fn(),
  setStudioLastTemplate: vi.fn(),
  trackGrowthEvent: vi.fn(),
  trackQuestCompleteOnce: vi.fn(),
}))

vi.mock('@/api/imageStudio', () => ({
  default: {
    getImageStudioTemplates: vi.fn().mockResolvedValue({
      intents: [{
        id: 'commerce',
        label: { zh: '电商', en: 'Commerce' },
        templates: [{
          id: 'commerce-white',
          label: { zh: '白底', en: 'White' },
          defaults: { size: '1024x1024', count: 1 },
        }],
      }],
    }),
    getImageStudioCapabilities: vi.fn().mockResolvedValue({
      aspects: [{ id: '1:1', label: { zh: '方形', en: 'Square' } }],
      tiers: [{ id: '1K', label: { zh: '1K', en: '1K' } }],
      size_options: [{ aspect: '1:1', tier: '1K', size: '1024x1024', billing_tier: '1K' }],
    }),
    listImageStudioModels: vi.fn().mockResolvedValue([{
      id: 'gpt-image-1',
      display_name: 'GPT Image 1',
      supported_sizes: ['1024x1024'],
    }]),
    estimateImageStudio: vi.fn().mockResolvedValue({
      estimated_cost: 0.1,
      balance: 10,
      sufficient: true,
      model: 'gpt-image-1',
      count: 1,
      size: '1024x1024',
    }),
    listImageStudioJobs: (...args: unknown[]) => mocks.listJobs(...args),
    getActiveImageStudioJob: vi.fn().mockResolvedValue(null),
    generateImageStudio: (...args: unknown[]) => mocks.generate(...args),
    pollImageStudioJob: (...args: unknown[]) => mocks.poll(...args),
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

describe('useImageStudioWorkspace prompt UX', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    mocks.auth.user = { id: 42, balance: 10 }
    mocks.auth.refreshUser = mocks.refreshUser
    mocks.listJobs.mockResolvedValue([])
    mocks.generate.mockResolvedValue({ job: { ...completedJob(), status: 'pending', assets: [] } })
    mocks.poll.mockResolvedValue(completedJob())
  })

  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
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
    expect(workspace.userPrompt.value).toBe('new prompt while waiting')
    expect(workspace.expertPrompt.value).toBe('new expert while waiting')
    expect(loadStudioDraft(42)?.userPrompt).toBe('new prompt while waiting')
  })

  it('clears prompt drafts after success and retains them after a structured API failure', async () => {
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'keep settings, clear me'
    workspace.expertOpen.value = true
    workspace.expertPrompt.value = 'clear expert too'
    await flushPromises()

    await expect(workspace.generate()).resolves.toBe(true)
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
    expect(workspace.latestJob.value?.id).toBe('job-1')
    expect(workspace.userPrompt.value).toBe('')
    expect(loadStudioDraft(42)?.userPrompt).toBe('')
    expect(workspace.errorMsg.value).toBe('')
  })

  it('keeps timeout copy visible instead of replacing it with a generic failure', async () => {
    mocks.poll.mockRejectedValueOnce(new Error('IMAGE_STUDIO_POLL_TIMEOUT'))
    const { workspace } = await mountWorkspace()
    workspace.userPrompt.value = 'valid prompt'

    await expect(workspace.generate()).resolves.toBe(false)
    expect(workspace.errorMsg.value).toBe('imageStudio.pollTimeout')
    expect(workspace.errorMsg.value).not.toBe('imageStudio.generateFailed')
    expect(getStudioPendingJobId()).toBe('job-1')
    expect(getStudioPendingJobForUser(42)?.jobId).toBe('job-1')
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
    mocks.auth.user = { id: 7, balance: 10 }

    await mountWorkspace()

    expect(mocks.poll).not.toHaveBeenCalled()
    expect(getStudioPendingJobId()).toBe('job-1')
  })

  it('surfaces gallery loading errors and clears them after retry', async () => {
    mocks.listJobs.mockRejectedValueOnce({ message: 'Gallery service unavailable.' })
    const { workspace } = await mountWorkspace()

    expect(workspace.galleryError.value).toBe('Gallery service unavailable.')

    mocks.listJobs.mockResolvedValueOnce([])
    await workspace.refreshJobs()
    expect(workspace.galleryError.value).toBe('')
  })
})
