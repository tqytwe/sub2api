import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpsImageRuntimeHealth from '../OpsImageRuntimeHealth.vue'

const mockGetImageRuntimesHealth = vi.fn()

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getImageRuntimesHealth: (...args: any[]) => mockGetImageRuntimesHealth(...args)
  }
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, any>) => {
        if (!params) return key
        return `${key}:${Object.values(params).join(',')}`
      }
    })
  }
})

const sampleHealth = {
  checked_at: '2026-07-18T12:00:00Z',
  gateway_async: {
    enabled: true,
    ready: true,
    storage: 'local',
    storage_ready: true,
    queue: 'redis',
    queue_enabled: true,
    database_ready: false,
    redis_ready: true,
    worker_running: true,
    backlog: { ready: 0, delayed: 0, active: 1 }
  },
  batch: {
    enabled: false,
    ready: false,
    storage: 'postgresql_and_provider_managed',
    storage_ready: true,
    queue: 'redis',
    queue_enabled: true,
    database_ready: true,
    redis_ready: true,
    worker_running: true,
    backlog: { ready: 5, delayed: 2, active: 1 },
    oldest_task: {
      id: 'imgbatch_oldest',
      status: 'running',
      created_at: '2026-07-18T11:00:00Z'
    },
    recent_error: {
      code: 'PROVIDER_STATUS_FAILED',
      message: 'provider failed',
      created_at: '2026-07-18T11:30:00Z'
    },
    stale_balance_holds: 3,
    settlement_retries: 4,
    provider_failures: 5,
    result_cleanup_pending: 6
  },
  image_studio: {
    enabled: true,
    ready: false,
    storage: 'postgresql_private_assets',
    storage_ready: true,
    queue: 'postgresql_leases',
    queue_enabled: true,
    database_ready: true,
    redis_ready: false,
    worker_running: false,
    backlog: { ready: 2, delayed: 0, active: 0 }
  }
}

describe('OpsImageRuntimeHealth', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetImageRuntimesHealth.mockResolvedValue(sampleHealth)
  })

  it('distinguishes ready, draining, and not-ready runtimes with operational details', async () => {
    const wrapper = mount(OpsImageRuntimeHealth)
    await flushPromises()

    expect(wrapper.get('[data-test="image-runtime-gateway_async"]').attributes('data-state')).toBe('ready')
    expect(wrapper.get('[data-test="image-runtime-batch"]').attributes('data-state')).toBe('draining')
    expect(wrapper.get('[data-test="image-runtime-image_studio"]').attributes('data-state')).toBe('not-ready')
    expect(wrapper.get('[data-test="image-runtime-batch"]').text()).toContain('imgbatch_oldest')
    expect(wrapper.get('[data-test="image-runtime-batch"]').text()).toContain('PROVIDER_STATUS_FAILED')
    expect(wrapper.get('[data-test="image-runtime-batch"]').text()).toContain('provider failed')
    expect(wrapper.get('[data-test="image-runtime-batch"]').text()).toContain('admin.ops.imageRuntimes.metrics.staleHolds')
    expect(wrapper.get('[data-test="image-runtime-batch"]').text()).toContain('admin.ops.imageRuntimes.metrics.cleanupPending')
  })

  it('refreshes manually and when the dashboard refresh token changes', async () => {
    const wrapper = mount(OpsImageRuntimeHealth, {
      props: { refreshToken: 0 }
    })
    await flushPromises()
    expect(mockGetImageRuntimesHealth).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-test="image-runtimes-refresh"]').trigger('click')
    await flushPromises()
    expect(mockGetImageRuntimesHealth).toHaveBeenCalledTimes(2)

    await wrapper.setProps({ refreshToken: 1 })
    await flushPromises()
    expect(mockGetImageRuntimesHealth).toHaveBeenCalledTimes(3)
  })

  it('keeps the panel visible when health loading fails', async () => {
    mockGetImageRuntimesHealth.mockRejectedValueOnce(new Error('runtime offline'))

    const wrapper = mount(OpsImageRuntimeHealth)
    await flushPromises()

    expect(wrapper.get('[data-test="image-runtimes-error"]').text()).toContain('admin.ops.imageRuntimes.loadFailed')
  })
})
