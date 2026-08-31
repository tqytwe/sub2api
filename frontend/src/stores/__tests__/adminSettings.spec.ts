import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const apiHarness = vi.hoisted(() => ({
  getSettings: vi.fn(),
  getConfig: vi.fn(),
}))

vi.mock('@/api', () => ({
  adminAPI: {
    settings: { getSettings: (...args: unknown[]) => apiHarness.getSettings(...args) },
    payment: { getConfig: (...args: unknown[]) => apiHarness.getConfig(...args) },
  },
}))

import { useAdminSettingsStore } from '@/stores/adminSettings'

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

describe('useAdminSettingsStore request coordination', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiHarness.getSettings.mockResolvedValue({ custom_menu_items: [] })
    apiHarness.getConfig.mockResolvedValue({ data: { enabled: false } })
  })

  it('shares an in-flight request between sidebar and route guards', async () => {
    const settings = deferred<{ custom_menu_items: Array<Record<string, string>> }>()
    const payment = deferred<{ data: { enabled: boolean } }>()
    apiHarness.getSettings.mockReturnValue(settings.promise)
    apiHarness.getConfig.mockReturnValue(payment.promise)

    const store = useAdminSettingsStore()
    const first = store.fetch()
    const second = store.fetch()

    expect(apiHarness.getSettings).toHaveBeenCalledTimes(1)
    expect(apiHarness.getConfig).toHaveBeenCalledTimes(1)
    expect(store.loading).toBe(true)

    settings.resolve({
      custom_menu_items: [{ id: 'admin-docs', url: 'https://www.jisudeng.com/docs', label: 'Docs' }],
    })
    payment.resolve({ data: { enabled: false } })
    await Promise.all([first, second])

    expect(store.loaded).toBe(true)
    expect(store.loading).toBe(false)
    expect(store.customMenuItems).toHaveLength(1)
  })

  it('returns a promise even when settings are already loaded', async () => {
    const store = useAdminSettingsStore()
    await store.fetch()

    const result = store.fetch()
    expect(result).toBeInstanceOf(Promise)
    await result
    expect(apiHarness.getSettings).toHaveBeenCalledTimes(1)
  })
})
