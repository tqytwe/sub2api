import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import HeroSphere from '../HeroSphere.vue'

const emptyFeatureCollection = { type: 'FeatureCollection', features: [] }

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function canvasContext() {
  return {
    setTransform: vi.fn(),
    clearRect: vi.fn(),
    beginPath: vi.fn(),
    closePath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    bezierCurveTo: vi.fn(),
    arc: vi.fn(),
    fill: vi.fn(),
    stroke: vi.fn(),
    globalAlpha: 1,
    fillStyle: '',
    strokeStyle: '',
    lineWidth: 1,
  } as unknown as CanvasRenderingContext2D
}

describe('HeroSphere bounded rendering', () => {
  let rafCallbacks: Map<number, FrameRequestCallback>
  let nextRaf: number
  let intersectionCallback: IntersectionObserverCallback | undefined

  beforeEach(() => {
    vi.restoreAllMocks()
    sessionStorage.clear()
    sessionStorage.setItem('jd-home-intro-seen', '1')
    rafCallbacks = new Map()
    nextRaf = 0
    intersectionCallback = undefined

    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(canvasContext())
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
      const id = ++nextRaf
      rafCallbacks.set(id, callback)
      return id
    })
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((id) => {
      rafCallbacks.delete(id)
    })
    vi.spyOn(performance, 'now').mockReturnValue(0)
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    Object.defineProperty(navigator, 'hardwareConcurrency', { configurable: true, value: 8 })
    Object.defineProperty(navigator, 'deviceMemory', { configurable: true, value: 8 })
    Object.defineProperty(navigator, 'connection', { configurable: true, value: { saveData: false } })
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    vi.stubGlobal('IntersectionObserver', class {
      constructor(callback: IntersectionObserverCallback) {
        intersectionCallback = callback
      }
      observe() {}
      unobserve() {}
      disconnect() {}
      takeRecords() { return [] }
      root = null
      rootMargin = ''
      thresholds = []
    })
  })

  it('initializes and reveals the canvas without waiting for geography', async () => {
    const earth = deferred<Response>()
    vi.stubGlobal('fetch', vi.fn(() => earth.promise))

    const wrapper = mount(HeroSphere)

    expect(HTMLCanvasElement.prototype.getContext).toHaveBeenCalled()
    expect(wrapper.emitted('reveal')).toHaveLength(1)

    earth.resolve({ ok: true, json: async () => emptyFeatureCollection } as Response)
    await flushPromises()
    wrapper.unmount()
  })

  it('does not start a frame loop in reduced-motion mode', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: true } as MediaQueryList)
    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      json: async () => emptyFeatureCollection,
    } as Response)))

    const wrapper = mount(HeroSphere)
    await flushPromises()

    expect(window.requestAnimationFrame).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('stops scheduling frames after drawing the final frame', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      json: async () => emptyFeatureCollection,
    } as Response)))
    const wrapper = mount(HeroSphere)
    await flushPromises()

    for (const timestamp of [0, 500, 1500, 4000]) {
      const entry = [...rafCallbacks.entries()][0]
      expect(entry).toBeTruthy()
      rafCallbacks.delete(entry[0])
      entry[1](timestamp)
    }

    expect(rafCallbacks.size).toBe(0)
    wrapper.unmount()
  })

  it('stops immediately when hidden or outside the viewport', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      json: async () => emptyFeatureCollection,
    } as Response)))
    const wrapper = mount(HeroSphere)
    await flushPromises()
    expect(rafCallbacks.size).toBeGreaterThan(0)

    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    document.dispatchEvent(new Event('visibilitychange'))
    expect(rafCallbacks.size).toBe(0)

    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    intersectionCallback?.([
      { target: wrapper.element, isIntersecting: false } as IntersectionObserverEntry,
    ], {} as IntersectionObserver)
    expect(rafCallbacks.size).toBe(0)
    wrapper.unmount()
  })
})
