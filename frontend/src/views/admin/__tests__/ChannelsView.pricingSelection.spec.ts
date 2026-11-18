import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelsView from '../ChannelsView.vue'
import { saveAs } from 'file-saver'
import { serializePricingBundle, serializePricingCollection } from '../channelPricingBundle'

const { list, getById, showError } = vi.hoisted(() => ({
  list: vi.fn(), getById: vi.fn(), showError: vi.fn(),
}))
vi.mock('@/api/admin', () => ({ adminAPI: {
  channels: { list, getById }, groups: { getAll: vi.fn().mockResolvedValue([]) },
  settings: { getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false }) },
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('file-saver', () => ({ saveAs: vi.fn() }))

function mountView() {
  return mount(ChannelsView, { global: { stubs: {
    AppLayout: defineComponent({ template: '<main><slot /></main>' }),
    TablePageLayout: defineComponent({ template: '<section><slot name="filters" /><slot name="table" /></section>' }),
    DataTable: defineComponent({
      props: ['data', 'selectedKeys'], emits: ['update:selectedKeys'],
      template: '<div><input v-for="row in data" :key="row.id" type="checkbox" :data-row-id="row.id" @change="$emit(\'update:selectedKeys\', [row.id])" /></div>',
    }),
    BulkEditChannelDialog: defineComponent({ props: ['show', 'channels', 'initialPricingById'], template: '<div v-if="show" data-testid="bulk-review">{{ channels.map(row => row.name).join() }}</div>' }),
    BaseDialog: defineComponent({ props:['show'],template:'<section v-if="show" data-testid="channel-editor"><slot /><slot name="footer" /></section>' }), ConfirmDialog: true, Pagination: true, EmptyState: true, Select: true,
    Icon: true, PlatformIcon: true, Toggle: true, PricingEntryCard: true,
  } } })
}

describe('ChannelsView pricing selection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const rows = [{ id: 7, name: 'Test channel', model_pricing: [] }]
    list.mockResolvedValue({ items: rows, total: 1 })
    getById.mockResolvedValue(rows[0])
  })

  it('loads a selected channel fresh before opening the bulk review dialog', async () => {
    const wrapper = mountView()
    await flushPromises()
    const bulk = wrapper.findAll('button').find(button => button.text() === 'admin.channels.bulkEdit.open')!
    expect(bulk.attributes('disabled')).toBeDefined()
    await wrapper.get('[data-row-id="7"]').setValue(true)
    await bulk.trigger('click')
    await flushPromises()
    expect(getById).toHaveBeenCalledWith(7)
    expect(wrapper.get('[data-testid="bulk-review"]').text()).toContain('Test channel')
    wrapper.unmount()
  })
  it('exports the current page when nothing is selected', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="export-channel-selection"]').trigger('click')
    await flushPromises()
    expect(saveAs).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('imports a package into an unsaved create draft', async () => {
    const wrapper = mountView()
    await flushPromises()
    const input = wrapper.get('[data-testid="list-pricing-import"]')
    const text = serializePricingBundle('Imported example', [{platform:'openai',entries:[]}], key => key)
    Object.defineProperty(input.element,'files',{configurable:true,value:[{size:text.length,text:async()=>text}]})
    await input.trigger('change')
    await flushPromises()
    expect(wrapper.get('[data-testid="channel-editor"]').text()).toContain('admin.channels.form.modelPricing')
    expect(getById).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('rejects a single-item collection whose name differs from the selected channel', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-row-id="7"]').setValue(true)
    const input = wrapper.get('[data-testid="selected-pricing-import"]')
    const text = serializePricingCollection([{name:'Different',platforms:[]}], key => key)
    Object.defineProperty(input.element,'files',{configurable:true,value:[{size:text.length,text:async()=>text}]})
    await input.trigger('change')
    await flushPromises()
    expect(wrapper.find('[data-testid="bulk-review"]').exists()).toBe(false)
    expect(showError).toHaveBeenCalledWith('admin.channels.pricingTransfer.channelMismatch')
    wrapper.unmount()
  })
  it('ignores a selected import when selection changes while its file is read', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-row-id="7"]').setValue(true)
    let finish!: (value: string) => void
    const input = wrapper.get('[data-testid="selected-pricing-import"]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [{ size: 100, text: () => new Promise<string>(resolve => { finish = resolve }) }] })
    await input.trigger('change')
    await wrapper.get('input[type="text"]').setValue('new filter')
    finish(serializePricingBundle('Example', [], key => key))
    await flushPromises()
    expect(getById).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="bulk-review"]').exists()).toBe(false)
    wrapper.unmount()
  })

})
