import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelOperationsView from '../ChannelOperationsView.vue'

const { listChannels, getCatalog, getSnapshot, mutate } = vi.hoisted(() => ({
  listChannels: vi.fn(),
  getCatalog: vi.fn(),
  getSnapshot: vi.fn(),
  mutate: vi.fn(),
}))

vi.mock('@/api/admin/channels', () => ({ list: listChannels, default: { list: listChannels } }))
vi.mock('@/api/admin/channelOperations', () => ({ getCatalog, getSnapshot, mutate }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const baseCatalog = {
  revision: 7,
  upstreams: [
    { id: 31, name: 'North', members: 1 },
    { id: 32, name: 'South', members: 0 },
  ],
  upstream_total: 2,
  accounts: [
    { id: 91, name: 'alpha', platform: 'openai', status: 'active', schedulable: true, concurrency: 4, upstream_id: 0, eligible: true },
    { id: 92, name: 'beta', platform: 'anthropic', status: 'active', schedulable: true, concurrency: 2, upstream_id: 31, eligible: true },
  ],
  total: 2,
}

const baseSnapshot = {
  scope: { channel_id: 5, upstream_id: 0, account_id: 0, group_id: 0, model: '', model_scope: 'public', dimension: 'channel' },
  usage: { status: 'ready', data: { attribution: 'current_membership', latency_measurement: 'legacy_forward', summary: { requests: 4, samples: 3, unknown_samples: 1, duration_samples: 2, p50_ms: 100, p90_ms: 200, p95_ms: 250, duration_p90_ms: 500, latest_sample_at: null }, total: 21, rows: [{ dimension_id: 31, name: 'North', requests: 4, samples: 3, unknown_samples: 1, duration_samples: 2, p50_ms: 100, p90_ms: 200, p95_ms: 250, duration_p90_ms: 500, latest_sample_at: null }], lineage: [], lineage_total: 0, error_rate: null, error_scope: 'not available' }, queried_at: '' },
  quality: { status: 'query_failed', data: null, error: 'query failed', queried_at: '' },
  errors: { status: 'unsupported_scope', data: null, error: 'unsupported', queried_at: '' },
  capacity: { status: 'ready', data: { source: 'accounts', scope: 'current', accounts: 1, schedulable: 1, configured_concurrency: 4 }, queried_at: '' },
  from: '2026-10-27T00:00:00.000Z', until: '2026-10-28T00:00:00.000Z', model_scope: 'public',
}

function mountView() {
  return mount(ChannelOperationsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: { template: '<section><slot name="filters" /><slot name="actions" /><slot name="table" /><slot name="pagination" /></section>' },
        BaseDialog: { props: ['show', 'title'], template: '<div v-if="show" role="dialog"><h2>{{ title }}</h2><slot /></div>' },
        ConfirmDialog: {
          props: ['show', 'title', 'message'],
          emits: ['cancel', 'confirm'],
          template: '<div v-if="show" role="alertdialog"><h2>{{ title }}</h2><p>{{ message }}</p><button @click="$emit(\'cancel\')">Cancel</button><button @click="$emit(\'confirm\')">Confirm</button></div>',
        },
        Icon: true,
        Pagination: true,
      },
    },
  })
}

async function readyView() {
  listChannels.mockResolvedValue({ items: [{ id: 5, name: 'Primary' }], total: 1 })
  getCatalog.mockResolvedValue(structuredClone(baseCatalog))
  getSnapshot.mockResolvedValue(structuredClone(baseSnapshot))
  const wrapper = mountView()
  await flushPromises()
  return wrapper
}

describe('ChannelOperationsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mutate.mockResolvedValue({ revision: 8 })
  })

  it('renders live membership and assigns an eligible account using the current revision', async () => {
    const wrapper = await readyView()
    expect(wrapper.text()).toContain('alpha')
    expect(wrapper.text()).toContain('North')

    await wrapper.findAll('button').find((button) => button.text().includes('channelOperations.assign'))!.trigger('click')
    await flushPromises()
    const dialog = wrapper.get('[role="dialog"]')
    await dialog.find('button.btn-primary').trigger('click')
    await flushPromises()

    expect(mutate).toHaveBeenCalledWith(5, expect.objectContaining({
      action: 'assign',
      revision: 7,
      account_id: 91,
      upstream_id: 31,
      expected_upstream_id: 0,
    }))
  })

  it('loads API snapshot scopes and preserves unavailable sections as unknown', async () => {
    const wrapper = await readyView()
    await wrapper.findAll('button').find((button) => button.text().includes('channelOperations.statisticsView'))!.trigger('click')
    await flushPromises()

    expect(getSnapshot).toHaveBeenCalledWith(expect.objectContaining({
      channel_id: 5,
      upstream_id: 0,
      account_id: 0,
      dimension: 'channel',
      model_scope: 'public',
      page: 1,
    }), expect.any(AbortSignal))
    expect(wrapper.text()).toContain('channelOperations.statsQueryFailed')
    expect(wrapper.text()).toContain('channelOperations.unsupportedScope')
    expect(wrapper.text()).toContain('channelOperations.p90')
    expect(wrapper.text()).not.toContain('quality: 0')

    await wrapper.findAll('button').find((button) => button.text().includes('channelOperations.drilldown'))!.trigger('click')
    await flushPromises()
    expect(getSnapshot).toHaveBeenLastCalledWith(expect.objectContaining({ upstream_id: 31, dimension: 'upstream' }), expect.any(AbortSignal))
  })

  it('pages snapshot rows without changing the aggregate request scope', async () => {
    const wrapper = await readyView()
    await wrapper.findAll('button').find((button) => button.text().includes('channelOperations.statisticsView'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('1 / 2')

    await wrapper.findAll('button').find((button) => button.text() === 'pagination.next')!.trigger('click')
    await flushPromises()
    expect(getSnapshot).toHaveBeenLastCalledWith(expect.objectContaining({
      page: 2,
      page_size: 20,
      channel_id: 5,
      dimension: 'channel',
    }), expect.any(AbortSignal))
    expect(wrapper.text()).toContain('admin.channelOperations.requests4')
  })

  it('requires confirmation for reassignment and sends the expected prior upstream', async () => {
    const wrapper = await readyView()
    await wrapper.findAll('button').find((button) => button.text().includes('channelOperations.reassign'))!.trigger('click')
    await flushPromises()
    const dialog = wrapper.get('[role="dialog"]')
    await dialog.find('select').setValue(32)
    await dialog.find('button.btn-primary').trigger('click')
    await flushPromises()

    expect(mutate).not.toHaveBeenCalled()
    const confirmation = wrapper.get('[role="alertdialog"]')
    await confirmation.findAll('button')[0].trigger('click')
    expect(mutate).not.toHaveBeenCalled()

    await wrapper.findAll('button').find((button) => button.text().includes('channelOperations.reassign'))!.trigger('click')
    await flushPromises()
    await wrapper.get('[role="dialog"]').find('select').setValue(32)
    await wrapper.get('[role="dialog"]').find('button.btn-primary').trigger('click')
    await flushPromises()
    await wrapper.get('[role="alertdialog"]').findAll('button')[1].trigger('click')
    await flushPromises()

    expect(mutate).toHaveBeenCalledWith(5, expect.objectContaining({
      action: 'assign',
      revision: 7,
      account_id: 92,
      upstream_id: 32,
      expected_upstream_id: 31,
      confirm: true,
    }))
  })

  it('refreshes after a revision conflict and exposes an explicit retry state', async () => {
    const wrapper = await readyView()
    mutate.mockRejectedValueOnce({ status: 409 })

    await wrapper.findAll('button').find((button) => button.text().includes('channelOperations.detach'))!.trigger('click')
    await flushPromises()
    await wrapper.get('[role="alertdialog"]').findAll('button')[1].trigger('click')
    await flushPromises()

    expect(mutate).toHaveBeenCalledWith(5, expect.objectContaining({
      action: 'detach',
      revision: 7,
      account_id: 92,
      expected_upstream_id: 31,
      confirm: true,
    }))
    expect(getCatalog).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('admin.channelOperations.conflict')
  })
})
