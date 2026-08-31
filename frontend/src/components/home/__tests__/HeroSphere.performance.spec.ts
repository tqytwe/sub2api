import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
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
  let idleCallbacks: Map<number, IdleRequestCallback>
  let nextIdle: number
  let intersectionCallback: IntersectionObserverCallback | undefined
  let context: CanvasRenderingContext2D

  beforeEach(() => {
    vi.restoreAllMocks()
    document.documentElement.classList.remove('dark')
    sessionStorage.clear()
    sessionStorage.setItem('jd-home-intro-seen', '1')
    rafCallbacks = new Map()
    nextRaf = 0
    idleCallbacks = new Map()
    nextIdle = 0
    intersectionCallback = undefined

    context = canvasContext()
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(context)
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
      const id = ++nextRaf
      rafCallbacks.set(id, callback)
      return id
    })
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((id) => {
      rafCallbacks.delete(id)
    })
    Object.defineProperty(window, 'requestIdleCallback', {
      configurable: true,
      value: vi.fn((callback: IdleRequestCallback) => {
        const id = ++nextIdle
        idleCallbacks.set(id, callback)
        return id
      }),
    })
    Object.defineProperty(window, 'cancelIdleCallback', {
      configurable: true,
      value: vi.fn((id: number) => {
        idleCallbacks.delete(id)
      }),
    })
    vi.spyOn(performance, 'now').mockReturnValue(0)
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    Object.defineProperty(navigator, 'hardwareConcurrency', { configurable: true, value: 8 })
    Object.defineProperty(navigator, 'deviceMemory', { configurable: true, value: 8 })
    Object.defineProperty(navigator, 'connection', { configurable: true, value: { saveData: false } })
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1280 })
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

  it('limits the desktop enhancement to a short, 20fps animation', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      json: async () => emptyFeatureCollection,
    } as Response)))
    const wrapper = mount(HeroSphere)
    await flushPromises()

    for (const timestamp of [0, 50, 800, 1600]) {
      const entry = [...rafCallbacks.entries()][0]
      expect(entry).toBeTruthy()
      rafCallbacks.delete(entry[0])
      entry[1](timestamp)
    }

    expect(rafCallbacks.size).toBe(0)
    wrapper.unmount()
  })

  it('uses a static poster on mobile before downloading geography', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    const fetchMock = vi.fn(async () => ({
      ok: true,
      json: async () => emptyFeatureCollection,
    } as Response))
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(HeroSphere)
    await flushPromises()

    expect(window.requestAnimationFrame).not.toHaveBeenCalled()
    expect(fetchMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('uses a visible dark-theme palette and redraws the poster after a theme switch', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    const wrapper = mount(HeroSphere)
    await flushPromises()

    const lightRim = context.strokeStyle
    document.documentElement.classList.add('dark')
    await flushPromises()

    expect(context.strokeStyle).not.toBe(lightRim)
    expect(context.strokeStyle).toContain('245,245,244')
    wrapper.unmount()
  })

  it('keeps the poster geometry visible in the first viewport without centering it behind the hero controls', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/components/home/HeroSphere.vue'), 'utf8')

    expect(source).toContain('canvas.w * 0.86')
    expect(source).toContain('canvas.h * 0.48')
    expect(source).toContain('canvas.w * 0.82')
    expect(source).toContain('canvas.h * 0.83')
    expect(source).not.toContain('canvas.h * 1.2')
  })

  it('defers every geography request until the idle enhancement callback', async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      json: async () => emptyFeatureCollection,
    } as Response))
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(HeroSphere)
    await flushPromises()

    expect(fetchMock).not.toHaveBeenCalled()
    const idleEntry = [...idleCallbacks.entries()][0]
    expect(idleEntry).toBeTruthy()
    idleCallbacks.delete(idleEntry[0])
    idleEntry[1]({ didTimeout: false, timeRemaining: () => 50 })
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledTimes(4)
    expect(fetchMock.mock.calls.map(([path]) => path).sort()).toEqual([
      '/earth/coast50.lod.json',
      '/earth/lakes50.lod.json',
      '/earth/land50.lod.json',
      '/earth/rivers50.lod.json',
    ])
    wrapper.unmount()
  })

  it('uses the static poster for coarse-pointer desktop sessions', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1280 })
    vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
      matches: query === '(pointer: coarse)',
    } as MediaQueryList))
    const fetchMock = vi.fn(async () => ({
      ok: true,
      json: async () => emptyFeatureCollection,
    } as Response))
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(HeroSphere)
    await flushPromises()

    expect(window.requestAnimationFrame).not.toHaveBeenCalled()
    expect(fetchMock).not.toHaveBeenCalled()
    expect(idleCallbacks.size).toBe(0)
    wrapper.unmount()
  })

  it('does not fetch when a queued idle callback runs after unmount', async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      json: async () => emptyFeatureCollection,
    } as Response))
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(HeroSphere)
    await flushPromises()
    const idleEntry = [...idleCallbacks.entries()][0]
    expect(idleEntry).toBeTruthy()
    wrapper.unmount()
    idleEntry[1]({ didTimeout: false, timeRemaining: () => 50 })
    await flushPromises()

    expect(fetchMock).not.toHaveBeenCalled()
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
