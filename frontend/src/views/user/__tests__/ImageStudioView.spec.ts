import { flushPromises, mount } from '@vue/test-utils'
import { computed, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ImageStudioView from '@/views/user/ImageStudioView.vue'

const state = vi.hoisted(() => ({
  generate: vi.fn(),
  refreshJobs: vi.fn(),
}))

function workspaceStub() {
  const selectedTemplate = {
    id: 'commerce-white',
    label: { zh: '白底', en: 'White' },
    description: { zh: '描述', en: 'Description' },
    defaults: { size: '1024x1024', count: 1 },
  }
  const userPrompt = ref('')
  const expertPrompt = ref('')
  const expertOpen = ref(false)
  return {
    bootstrapping: ref(false),
    generating: ref(false),
    polling: ref(false),
    pollNotice: ref(''),
    catalog: ref({ intents: [] }),
    capabilities: ref(null),
    selectedTemplate: ref(selectedTemplate),
    promptReference: ref(null),
    promptReferenceError: ref(''),
    promptVariableValues: ref({}),
    userPrompt,
    accentColor: ref('#1a1a1a'),
    size: ref('1024x1024'),
    aspect: ref('1:1'),
    tier: ref('1K'),
    count: ref(1),
    expertOpen,
    expertPrompt,
    apiKeyId: ref(8),
    apiKeys: ref([{ id: 8, name: 'Images' }]),
    availableModels: ref([{ id: 'gpt-image-1', display_name: 'GPT Image 1' }]),
    selectedModel: ref('gpt-image-1'),
    selectedModelOption: ref({ id: 'gpt-image-1', display_name: 'GPT Image 1' }),
    quality: ref(''),
    loadingModels: ref(false),
    modelError: ref(''),
    estimateError: ref(''),
    estimate: ref({
      estimated_cost: 0.1,
      balance: 10,
      sufficient: true,
      model: 'gpt-image-1',
      count: 1,
      size: '1024x1024',
    }),
    jobs: ref([]),
    galleryError: ref(''),
    errorMsg: ref(''),
    autoCleanup: ref(false),
    showFirstWin: ref(false),
    latestJob: ref(null),
    balance: computed(() => 10),
    hasApiKeys: computed(() => true),
    showAccentColor: computed(() => true),
    showQuality: computed(() => false),
    promptValid: computed(() => userPrompt.value.trim().length > 0 && [...userPrompt.value].length <= 8000),
    expertPromptValid: computed(() =>
      !expertOpen.value || !expertPrompt.value.trim() || [...expertPrompt.value].length <= 8000),
    maxCount: computed(() => 4),
    isNewUser: computed(() => false),
    previewAsset: ref(null),
    previewJobId: ref(''),
    previewIndex: ref(0),
    labelFor: (value?: { en: string }) => value?.en || '',
    pickTemplate: vi.fn(),
    applyPromptVariables: vi.fn(),
    clearPromptReference: vi.fn(),
    saveCreationRecipe: vi.fn(),
    onAutoCleanupChange: vi.fn(),
    openPreview: vi.fn(),
    closePreview: vi.fn(),
    regenerateFromJob: vi.fn(),
    generate: state.generate,
    removeJob: vi.fn(),
    onAspectChange: vi.fn(),
    onTierChange: vi.fn(),
    refreshJobs: state.refreshJobs,
  }
}

let workspace: ReturnType<typeof workspaceStub>

vi.mock('@/composables/useImageStudioWorkspace', () => ({
  useImageStudioWorkspace: () => workspace,
}))

vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: { imageStudio: 'image_studio' },
  isFeatureFlagEnabled: () => true,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

function mountView() {
  return mount(ImageStudioView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        ImageStudioGallery: true,
        ImageStudioPreviewModal: true,
        ImageStudioSizePicker: true,
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('ImageStudioView prompt UX', () => {
  beforeEach(() => {
    workspace = workspaceStub()
    state.generate.mockReset()
    state.refreshJobs.mockReset()
  })

  it('shows code-point counters and inline errors without native UTF-16 maxlength clipping', async () => {
    state.generate.mockResolvedValue(false)
    const wrapper = mountView()
    const textareas = wrapper.findAll('textarea')
    const mainPrompt = textareas[0]
    const expertPrompt = textareas[1]

    expect(mainPrompt.attributes('maxlength')).toBeUndefined()
    expect(expertPrompt.attributes('maxlength')).toBeUndefined()

    await mainPrompt.setValue('😀')
    expect(wrapper.text()).toContain('1 / 8000')

    await mainPrompt.setValue('')
    await mainPrompt.trigger('blur')
    expect(wrapper.text()).toContain('imageStudio.promptRequired')

    workspace.expertOpen.value = true
    await expertPrompt.setValue(' ')
    await expertPrompt.trigger('blur')
    expect(wrapper.text()).not.toContain('imageStudio.expertPromptTooLong')
    expect(wrapper.get('button.btn-primary.w-full').attributes('disabled')).toBeDefined()

    await mainPrompt.setValue('valid prompt')
    await expertPrompt.setValue('😀'.repeat(8001))
    await expertPrompt.trigger('blur')
    expect(wrapper.text()).toContain('imageStudio.expertPromptTooLong')
    expect(wrapper.get('button.btn-primary.w-full').attributes('disabled')).toBeDefined()
  })

  it('switches to works only after polling starts and returns to create on failure', async () => {
    let finish!: (value: boolean) => void
    state.generate.mockReturnValue(new Promise<boolean>((resolve) => { finish = resolve }))
    workspace.userPrompt.value = 'valid prompt'
    const wrapper = mountView()
    const tabs = wrapper.findAll('.grid.grid-cols-4 button')

    await wrapper.get('button.btn-primary.w-full').trigger('click')
    expect(tabs[0].classes()).toContain('bg-white')
    expect(tabs[2].classes()).not.toContain('bg-white')

    workspace.polling.value = true
    await flushPromises()
    expect(tabs[2].classes()).toContain('bg-white')

    workspace.polling.value = false
    finish(false)
    await flushPromises()
    expect(tabs[0].classes()).toContain('bg-white')

    workspace.polling.value = true
    await flushPromises()
    workspace.errorMsg.value = 'imageStudio.pollTimeout'
    await flushPromises()
    expect(tabs[0].classes()).toContain('bg-white')
    expect(wrapper.get('[data-testid="mobile-generation-error"]').text()).toContain(
      'imageStudio.pollTimeout',
    )
  })

  it('shows a retryable gallery error instead of the empty state', async () => {
    workspace.galleryError.value = 'Gallery service unavailable.'
    const wrapper = mountView()

    expect(wrapper.text()).toContain('Gallery service unavailable.')
    expect(wrapper.text()).not.toContain('imageStudio.galleryEmpty')
    await wrapper.get('[data-testid="retry-gallery"]').trigger('click')
    expect(state.refreshJobs).toHaveBeenCalledTimes(1)
  })

  it('keeps a successful result visible when a background gallery refresh fails', () => {
    workspace.jobs.value = [{
      id: 'job-1',
      template_id: 'commerce-white',
      size: '1024x1024',
      count: 1,
      status: 'completed',
      estimated_cost: 0.1,
      created_at: '2026-07-16T00:00:00Z',
      assets: [{ id: 'asset-1', sort_order: 0 }],
    }]
    workspace.galleryError.value = 'Gallery service unavailable.'
    const wrapper = mountView()

    expect(wrapper.find('image-studio-gallery-stub').exists()).toBe(true)
    expect(wrapper.find('[data-testid="retry-gallery"]').exists()).toBe(false)
  })

  it('remeasures a restored expert prompt when advanced settings opens and the viewport changes', async () => {
    const originalMatchMedia = window.matchMedia
    window.matchMedia = vi.fn().mockReturnValue({
      matches: true,
      media: '(max-width: 1023px)',
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })
    workspace.expertPrompt.value = 'restored expert prompt'
    const wrapper = mountView()
    const mainPrompt = wrapper.findAll('textarea')[0].element
    const expertPrompt = wrapper.findAll('textarea')[1].element
    Object.defineProperty(mainPrompt, 'scrollHeight', { configurable: true, value: 600 })
    Object.defineProperty(expertPrompt, 'scrollHeight', { configurable: true, value: 600 })

    const details = wrapper.get('details')
    ;(details.element as HTMLDetailsElement).open = true
    await details.trigger('toggle')
    await flushPromises()

    expect(workspace.expertOpen.value).toBe(true)
    expect(expertPrompt.style.height).not.toBe('')
    expect(expertPrompt.style.overflowY).toBe('auto')

    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 500 })
    window.dispatchEvent(new window.Event('resize'))
    await flushPromises()
    expect(mainPrompt.style.height).toBe('210px')
    expect(expertPrompt.style.height).toBe('210px')
    wrapper.unmount()
    window.matchMedia = originalMatchMedia
  })

  it('remeasures restored prompts after the loading shell unmounts', async () => {
    workspace.bootstrapping.value = true
    workspace.userPrompt.value = 'restored long prompt'
    const wrapper = mountView()
    const scrollHeight = vi.spyOn(HTMLTextAreaElement.prototype, 'scrollHeight', 'get')
      .mockReturnValue(480)

    workspace.bootstrapping.value = false
    await flushPromises()
    const mainPrompt = wrapper.find('textarea').element
    const expectedMaxHeight = window.matchMedia('(max-width: 1023px)').matches
      ? Math.floor(window.innerHeight * 0.42)
      : 320

    expect(mainPrompt.style.height).toBe(`${expectedMaxHeight}px`)
    expect(mainPrompt.style.overflowY).toBe('auto')
    wrapper.unmount()
    scrollHeight.mockRestore()
  })

  it('shows the 极速蹬 library reference and Chinese variable controls', async () => {
    workspace.promptReference.value = {
      prompt_id: '123',
      version: 4,
      title: '夏日饮品海报',
      prompt_template: 'Create a poster for {{product}}.',
      variables: [
        { name: 'product', label: '产品名称', description: '填写要展示的商品' },
      ],
      recommended_models: [],
      recommended_sizes: [],
      reference_requirement: 'none',
    }
    workspace.promptVariableValues.value = { product: '气泡水' }

    const wrapper = mountView()

    expect(wrapper.text()).toContain('来自极速蹬提示词库')
    expect(wrapper.text()).toContain('夏日饮品海报')
    expect(wrapper.text()).toContain('产品名称')
    expect(wrapper.text()).toContain('智能改写')
    expect(wrapper.text()).toContain('保存为创作配方')
    wrapper.unmount()
  })
})
