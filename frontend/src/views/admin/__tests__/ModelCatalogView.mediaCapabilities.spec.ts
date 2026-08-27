import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ModelCatalogView from '@/views/admin/ModelCatalogView.vue'

const { listCatalog, saveCatalogEntry, getAll, showError } = vi.hoisted(() => ({
  listCatalog: vi.fn(),
  saveCatalogEntry: vi.fn(),
  getAll: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/modelCatalog', () => ({
  default: {
    listCatalog,
    saveCatalogEntry,
    deleteCatalogEntry: vi.fn(),
    batchVisibility: vi.fn(),
    batchGroups: vi.fn(),
    createSyncJob: vi.fn(),
    getSyncJob: vi.fn(),
    listDiscoveries: vi.fn(),
    importDiscoveries: vi.fn(),
  },
}))

vi.mock('@/api/admin/groups', () => ({ default: { getAll } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const AppLayoutStub = defineComponent({ template: '<main><slot /></main>' })
const TablePageLayoutStub = defineComponent({ template: '<section><slot name="filters" /><slot name="table" /></section>' })
const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: `
    <section>
      <article v-for="row in data" :key="row.id">
        <slot name="cell-media_capabilities" :row="row" />
        <slot name="cell-actions" :row="row" />
      </article>
    </section>
  `,
})
const BaseDialogStub = defineComponent({
  props: { show: Boolean, title: String },
  emits: ['close'],
  template: '<section v-if="show"><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>',
})

function mountView() {
  return mount(ModelCatalogView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        BaseDialog: BaseDialogStub,
        Toggle: true,
        GroupSelector: true,
        Icon: true,
        RouterLink: true,
      },
    },
  })
}

function editButton(wrapper: ReturnType<typeof mountView>) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text() === 'common.edit')
  if (!button) throw new Error('model catalog edit action was not rendered')
  return button
}

function inputForLabel(wrapper: ReturnType<typeof mountView>, key: string) {
  const label = wrapper.findAll('label').find((candidate) => candidate.text().includes(key))
  if (!label) throw new Error(`input label ${key} was not rendered`)
  return label.get('input')
}

describe('ModelCatalogView media capability declaration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getAll.mockResolvedValue([])
    listCatalog.mockResolvedValue([{
      id: 1,
      model_name: 'existing-chat-model',
      platform: 'openai',
      display_name: null,
      use_case: null,
      sort_order: 1,
      visible_public: false,
      visible_auth: true,
      featured: false,
      group_ids: null,
      media_capabilities: null,
      official_input_price: null,
      official_output_price: null,
      official_cache_read_price: null,
      official_cache_write_price: null,
      official_source: '',
      official_updated_at: null,
      official_input_manual: false,
      official_output_manual: false,
      official_cache_read_manual: false,
      official_cache_write_manual: false,
      source_updated_at: null,
      created_at: '2026-08-26T00:00:00Z',
      updated_at: '2026-08-26T00:00:00Z',
    }])
  })

  it('keeps legacy rows undeclared until an administrator explicitly enables media capabilities', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.modelCatalog.mediaCapabilities.undeclared')
    await editButton(wrapper).trigger('click')
    await flushPromises()

    const toggle = wrapper.get('[data-testid="model-media-capabilities-toggle"]')
    expect((toggle.element as HTMLInputElement).checked).toBe(false)
    await toggle.setValue(true)

    expect(wrapper.text()).toContain('admin.modelCatalog.fields.mediaCapabilitiesVersion')
    expect(wrapper.text()).toContain('admin.modelCatalog.mediaCapabilities.image')
    wrapper.unmount()
  })

  it('blocks an empty declaration instead of silently saving a model with no modality', async () => {
    const wrapper = mountView()
    await flushPromises()
    await editButton(wrapper).trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="model-media-capabilities-toggle"]').setValue(true)
    const save = wrapper.findAll('button').find((button) => button.text() === 'common.save')
    expect(save).toBeDefined()
    await save!.trigger('click')

    expect(showError).toHaveBeenCalledWith('admin.modelCatalog.mediaCapabilities.validation.modalities_required')
    expect(saveCatalogEntry).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('blocks an operation alias before it reaches the catalog API', async () => {
    const wrapper = mountView()
    await flushPromises()
    await editButton(wrapper).trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="model-media-capabilities-toggle"]').setValue(true)
    await inputForLabel(wrapper, 'admin.modelCatalog.fields.mediaCapabilitiesVersion').setValue('v1')
    await inputForLabel(wrapper, 'admin.modelCatalog.fields.mediaCapabilitiesAdapter').setValue('sensenova')
    await inputForLabel(wrapper, 'admin.modelCatalog.mediaCapabilities.image').setValue(true)
    await inputForLabel(wrapper, 'admin.modelCatalog.fields.imageOperations').setValue('generation')

    const save = wrapper.findAll('button').find((button) => button.text() === 'common.save')
    expect(save).toBeDefined()
    await save!.trigger('click')

    expect(showError).toHaveBeenCalledWith('admin.modelCatalog.mediaCapabilities.validation.image_operations_invalid')
    expect(saveCatalogEntry).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('preserves a future server modality and featured flag when saving an existing declaration', async () => {
    listCatalog.mockResolvedValueOnce([{
      id: 2,
      model_name: 'provider-future-model',
      platform: 'openai',
      display_name: null,
      use_case: null,
      sort_order: 2,
      visible_public: false,
      visible_auth: true,
      featured: true,
      group_ids: null,
      media_capabilities: {
        version: 'v1',
        adapter: 'provider-adapter',
        modalities: ['image', 'embedding'],
        image: { operations: ['create'] },
      },
      official_input_price: null,
      official_output_price: null,
      official_cache_read_price: null,
      official_cache_write_price: null,
      official_source: '',
      official_updated_at: null,
      official_input_manual: false,
      official_output_manual: false,
      official_cache_read_manual: false,
      official_cache_write_manual: false,
      source_updated_at: null,
      created_at: '2026-08-26T00:00:00Z',
      updated_at: '2026-08-26T00:00:00Z',
    }])

    const wrapper = mountView()
    await flushPromises()
    await editButton(wrapper).trigger('click')
    await flushPromises()

    const save = wrapper.findAll('button').find((button) => button.text() === 'common.save')
    expect(save).toBeDefined()
    await save!.trigger('click')
    await flushPromises()

    expect(saveCatalogEntry).toHaveBeenCalledWith(expect.objectContaining({
      featured: true,
      media_capabilities: expect.objectContaining({
        modalities: ['image', 'embedding'],
      }),
    }))
    wrapper.unmount()
  })
})
