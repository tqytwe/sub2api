import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useLeaderboardLiveRefresh } from '@/composables/useLeaderboardLiveRefresh'

describe('useLeaderboardLiveRefresh', () => {
  const wrappers: ReturnType<typeof mount>[] = []
  function setup(fetch: (signal: AbortSignal) => Promise<number>, enabled = ref(true), key = ref('a')) {
    let result!: ReturnType<typeof useLeaderboardLiveRefresh<number>>
    wrappers.push(mount(defineComponent({
      setup() { result = useLeaderboardLiveRefresh({ fetch, enabled, key }); return {} },
      template: '<div />',
    })))
    return { result, enabled, key }
  }
  function visible(value: boolean) {
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: value ? 'visible' : 'hidden' })
    document.dispatchEvent(new Event('visibilitychange'))
  }
  beforeEach(() => { vi.useFakeTimers(); visible(true) })
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers() })

  it('loads immediately and refreshes only the supplied lightweight request every five seconds', async () => {
    const fetch = vi.fn().mockResolvedValue(7)
    const { result } = setup(fetch)
    await flushPromises()
    expect(result.data.value).toBe(7)
    await vi.advanceTimersByTimeAsync(4999)
    expect(fetch).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('stops while hidden and refreshes immediately when visible again', async () => {
    const fetch = vi.fn().mockResolvedValue(7)
    setup(fetch)
    await flushPromises()
    visible(false)
    await vi.advanceTimersByTimeAsync(60000)
    expect(fetch).toHaveBeenCalledTimes(1)
    visible(true)
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('backs off failures by 10, 20 and 30 seconds, preserves stale data and resets after success', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(1).mockRejectedValue(new Error('offline'))
    const { result } = setup(fetch)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    expect(result.data.value).toBe(1)
    expect(result.error.value).toBeInstanceOf(Error)
    expect(result.nextRefreshMs.value).toBe(10000)
    await vi.advanceTimersByTimeAsync(10000)
    expect(result.nextRefreshMs.value).toBe(20000)
    await vi.advanceTimersByTimeAsync(20000)
    expect(result.nextRefreshMs.value).toBe(30000)
    fetch.mockResolvedValue(9)
    await vi.advanceTimersByTimeAsync(30000)
    expect(result.data.value).toBe(9)
    expect(result.error.value).toBeNull()
    expect(result.nextRefreshMs.value).toBe(5000)
  })

  it('does not overlap requests even when manually refreshed or the timer advances', async () => {
    let resolve!: (value: number) => void
    const fetch = vi.fn(() => new Promise<number>(done => { resolve = done }))
    const { result } = setup(fetch)
    void result.refresh()
    await vi.advanceTimersByTimeAsync(20000)
    expect(fetch).toHaveBeenCalledTimes(1)
    resolve(3)
    await flushPromises()
    expect(result.data.value).toBe(3)
  })

  it('invalidates a previous selection, aborts it and ignores its late response', async () => {
    let resolve!: (value: number) => void
    const fetch = vi.fn().mockImplementationOnce(() => new Promise<number>(done => { resolve = done })).mockResolvedValue(8)
    const { result, key } = setup(fetch)
    await flushPromises()
    key.value = 'b'
    await flushPromises()
    expect(fetch.mock.calls[0][0].aborted).toBe(true)
    resolve(2)
    await flushPromises()
    expect(result.data.value).toBe(8)
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('runs only for the active tab and clears its timer and listeners after unmount', async () => {
    const fetch = vi.fn().mockResolvedValue(7)
    const enabled = ref(false)
    setup(fetch, enabled)
    await flushPromises()
    expect(fetch).not.toHaveBeenCalled()
    enabled.value = true
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(1)
    enabled.value = false
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60000)
    expect(fetch).toHaveBeenCalledTimes(1)
    wrappers[0].unmount()
    visible(false); visible(true)
    await vi.advanceTimersByTimeAsync(60000)
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('ignores an unmounted request even if its transport does not honor abort', async () => {
    let resolve!: (value: number) => void
    const fetch = vi.fn((signal: AbortSignal) => new Promise<number>(done => { resolve = done; expect(signal.aborted).toBe(false) }))
    const { result } = setup(fetch)
    await flushPromises()
    wrappers[0].unmount()
    resolve(11)
    await flushPromises()
    expect(result.data.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('recovers from a synchronously thrown transport error', async () => {
    const fetch = vi.fn().mockImplementationOnce(() => { throw new Error('configuration') }).mockResolvedValue(4)
    const { result } = setup(fetch)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(10000)
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(result.data.value).toBe(4)
  })

  it('starts just one request when enabling a tab and selecting its period together', async () => {
    const enabled = ref(false)
    const key = ref('a')
    const fetch = vi.fn().mockResolvedValue(4)
    setup(fetch, enabled, key)
    enabled.value = true
    key.value = 'b'
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(1)
  })
})
