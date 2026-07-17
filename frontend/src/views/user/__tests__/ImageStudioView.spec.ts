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
  const activeJobCount = ref(0)
  const mode = ref<'create' | 'edit'>('create')
  const supportsCreate = ref(true)
  const supportsEdit = ref(true)
  const selectedModelOption = ref({
    id: 'gpt-image-1.5',
    display_name: 'GPT Image 1.5',
    supported_backgrounds: ['auto', 'opaque', 'transparent'],
    supported_output_formats: ['png', 'jpeg', 'webp'],
    supported_input_fidelities: ['low', 'high'],
    input_fidelity_mode: 'selectable',
    output_compression: { min: 0, max: 100, formats: ['jpeg', 'webp'] },
  })
  return {
    bootstrapping: ref(false),
    capabilitiesReady: ref(true),
    capabilityError: ref(''),
    generating: ref(false),
    polling: ref(false),
    pollNotice: ref(''),
    catalog: ref({ intents: [] }),
    capabilities: ref(null),
    selectedTemplate: ref(selectedTemplate),
    userPrompt,
    accentColor: ref('#1a1a1a'),
    size: ref('1024x1024'),
    aspect: ref('1:1'),
    tier: ref('1K'),
    count: ref(1),
    expertOpen,
    expertPrompt,
    mode,
    supportsCreate,
    supportsEdit,
    operationSupported: computed(() =>
      mode.value === 'create' ? supportsCreate.value : supportsEdit.value),
    referenceUploads: ref([]),
    uploadingReferences: computed(() => false),
    editReferencesReady: computed(() => mode.value === 'create'),
    maxReferenceImages: ref(4),
    apiKeyId: ref(8),
    apiKeys: ref([{ id: 8, name: 'Images' }]),
    availableModels: ref([selectedModelOption.value]),
    selectedModel: ref('gpt-image-1.5'),
    selectedModelOption,
    quality: ref(''),
    background: ref('auto'),
    outputFormat: ref('webp'),
    outputCompression: ref(85),
    inputFidelity: ref('high'),
    showBackground: ref(true),
    showOutputFormat: ref(true),
    showOutputCompression: ref(true),
    showInputFidelity: ref(true),
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
    activeJobs: ref([]),
    galleryError: ref(''),
    galleryLoading: ref(false),
    galleryPage: ref(1),
    galleryPageSize: 12,
    galleryTotal: ref(0),
    galleryPages: ref(0),
    errorMsg: ref(''),
    autoCleanup: ref(false),
    showFirstWin: ref(false),
    latestJob: ref(null),
    activeJobCount,
    atActiveJobLimit: computed(() => activeJobCount.value >= 2),
    cancelingJobIds: ref(new Set<string>()),
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
    onAutoCleanupChange: vi.fn(),
    openPreview: vi.fn(),
    closePreview: vi.fn(),
    regenerateFromJob: vi.fn(),
    addReferenceFiles: vi.fn(),
    retryReference: vi.fn(),
    removeReference: vi.fn(),
    generate: state.generate,
    cancelJob: vi.fn(),
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

  it('keeps generation available for the second active slot and disables the third', async () => {
    workspace.userPrompt.value = 'valid prompt'
    workspace.activeJobCount.value = 1
    const wrapper = mountView()
    const generateButton = wrapper.get('button.btn-primary.w-full')

    expect(generateButton.attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).toContain('imageStudio.activeJobs')

    workspace.activeJobCount.value = 2
    await flushPromises()
    expect(generateButton.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('imageStudio.activeJobLimit')
  })

  it('fails closed and shows the reason when capabilities are unavailable', () => {
    workspace.userPrompt.value = 'valid prompt'
    workspace.capabilitiesReady.value = false
    workspace.capabilityError.value = 'imageStudio.loadCapabilitiesFailed'

    const wrapper = mountView()

    expect(wrapper.get('button.btn-primary.w-full').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('imageStudio.loadCapabilitiesFailed')
  })

  it('shows create and edit modes and forwards selected reference files', async () => {
    const wrapper = mountView()
    const modeButtons = wrapper.findAll('[data-testid^="image-mode-"]')

    expect(modeButtons).toHaveLength(2)
    await modeButtons[1].trigger('click')
    expect(workspace.mode.value).toBe('edit')
    await flushPromises()

    const file = new File(['test'], 'reference.png', { type: 'image/png' })
    const input = wrapper.get('[data-testid="reference-input"]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [file] })
    await input.trigger('change')
    expect(workspace.addReferenceFiles).toHaveBeenCalledWith([file])
  })

  it('disables model operations that are not supported', async () => {
    workspace.supportsEdit.value = false
    const wrapper = mountView()

    expect(wrapper.get('[data-testid="image-mode-create"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="image-mode-edit"]').attributes('disabled')).toBeDefined()

    workspace.supportsCreate.value = false
    workspace.supportsEdit.value = true
    workspace.mode.value = 'edit'
    await flushPromises()

    expect(wrapper.get('[data-testid="image-mode-create"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="image-mode-edit"]').attributes('disabled')).toBeUndefined()
  })

  it('renders advanced controls only when the selected model capability allows them', async () => {
    workspace.mode.value = 'edit'
    const wrapper = mountView()

    expect(wrapper.get('[data-testid="background-select"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="output-format-select"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="output-compression-input"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="input-fidelity-select"]').exists()).toBe(true)

    workspace.showBackground.value = false
    workspace.showOutputFormat.value = false
    workspace.showOutputCompression.value = false
    workspace.showInputFidelity.value = false
    await flushPromises()

    expect(wrapper.find('[data-testid="background-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="output-format-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="output-compression-input"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="input-fidelity-select"]').exists()).toBe(false)
  })

  it('switches to works only after polling starts and returns to create on failure', async () => {
    let finish!: (value: boolean) => void
    state.generate.mockReturnValue(new Promise<boolean>((resolve) => { finish = resolve }))
    workspace.userPrompt.value = 'valid prompt'
    const wrapper = mountView()
    const tabs = wrapper.findAll('.grid.grid-cols-2 button')

    await wrapper.get('button.btn-primary.w-full').trigger('click')
    expect(tabs[0].classes()).toContain('bg-white')
    expect(tabs[1].classes()).not.toContain('bg-white')

    workspace.polling.value = true
    await flushPromises()
    expect(tabs[1].classes()).toContain('bg-white')

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

  it('shows history pagination and requests the adjacent 12-item page', async () => {
    workspace.jobs.value = [
      {
        id: 'job-featured',
        template_id: 'commerce-white',
        size: '1024x1024',
        count: 1,
        status: 'completed',
        estimated_cost: 0.1,
        created_at: '2026-07-16T00:00:00Z',
        assets: [{ id: 'asset-1', sort_order: 0 }],
      },
      {
        id: 'job-history',
        template_id: 'commerce-white',
        size: '1536x1024',
        count: 1,
        status: 'partial',
        estimated_cost: 0.1,
        created_at: '2026-07-15T00:00:00Z',
        assets: [{ id: 'asset-2', sort_order: 0 }],
      },
    ]
    workspace.galleryPage.value = 2
    workspace.galleryPages.value = 3
    workspace.galleryTotal.value = 25
    const wrapper = mountView()

    expect(wrapper.text()).toContain('imageStudio.pageStatus')
    await wrapper.get('button[aria-label="imageStudio.previousPage"]').trigger('click')
    expect(state.refreshJobs).toHaveBeenCalledWith(1)

    await wrapper.get('button[aria-label="imageStudio.nextPage"]').trigger('click')
    expect(state.refreshJobs).toHaveBeenCalledWith(3)
  })

  it('does not present a second-page job as the latest featured result', () => {
    workspace.jobs.value = [{
      id: 'job-page-2',
      template_id: 'commerce-white',
      size: '1024x1024',
      count: 1,
      status: 'completed',
      estimated_cost: 0.1,
      created_at: '2026-07-15T00:00:00Z',
      assets: [{ id: 'asset-page-2', sort_order: 0 }],
    }]
    workspace.latestJob.value = workspace.jobs.value[0]
    workspace.galleryPage.value = 2
    workspace.galleryPages.value = 3

    const wrapper = mountView()
	const galleries = wrapper.findAll('image-studio-gallery-stub')

	expect(galleries).toHaveLength(1)
	expect(galleries[0].attributes('featured')).not.toBe('true')
  })

  it('renders active jobs separately while keeping all 12 history entries on page one', () => {
    workspace.activeJobs.value = [{
      id: 'job-active',
      template_id: 'commerce-white',
      size: '1024x1024',
      count: 1,
      status: 'running',
      estimated_cost: 0.1,
      created_at: '2026-07-17T00:00:00Z',
      assets: [],
    }]
    workspace.jobs.value = Array.from({ length: 12 }, (_, index) => ({
      id: `job-history-${index + 1}`,
      template_id: 'commerce-white',
      size: '1024x1024',
      count: 1,
      status: 'completed',
      estimated_cost: 0.1,
      created_at: '2026-07-16T00:00:00Z',
      assets: [{ id: `asset-${index + 1}`, sort_order: 0 }],
    }))
    workspace.galleryPage.value = 1
    workspace.galleryTotal.value = 12

    const wrapper = mountView()
    const galleries = wrapper.findAllComponents({ name: 'ImageStudioGallery' })

    expect(galleries).toHaveLength(2)
    expect(galleries[0].props('jobs')).toHaveLength(1)
    expect(galleries[0].props('jobs')[0].id).toBe('job-active')
    expect(galleries[0].props('featured')).toBe(true)
    expect(galleries[1].props('jobs')).toHaveLength(12)
  })

  it('renders only the exact 12 history entries on later pages', () => {
    workspace.activeJobs.value = [{
      id: 'job-active',
      template_id: 'commerce-white',
      size: '1024x1024',
      count: 1,
      status: 'running',
      estimated_cost: 0.1,
      created_at: '2026-07-17T00:00:00Z',
      assets: [],
    }]
    workspace.jobs.value = Array.from({ length: 12 }, (_, index) => ({
      id: `job-page-2-${index + 1}`,
      template_id: 'commerce-white',
      size: '1024x1024',
      count: 1,
      status: 'completed',
      estimated_cost: 0.1,
      created_at: '2026-07-15T00:00:00Z',
      assets: [{ id: `asset-page-2-${index + 1}`, sort_order: 0 }],
    }))
    workspace.galleryPage.value = 2
    workspace.galleryPages.value = 3
    workspace.galleryTotal.value = 25

    const wrapper = mountView()
    const galleries = wrapper.findAllComponents({ name: 'ImageStudioGallery' })

    expect(galleries).toHaveLength(1)
    expect(galleries[0].props('jobs')).toHaveLength(12)
    expect(galleries[0].props('featured')).not.toBe(true)
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
    scrollHeight.mockRestore()
  })
})
