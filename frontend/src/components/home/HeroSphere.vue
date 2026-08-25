<template>
  <div ref="rootEl" class="hs-root" aria-hidden="true">
    <canvas ref="canvasMainEl" class="hs-canvas" />
  </div>
</template>

<script setup lang="ts">
import type { FeatureCollection } from 'geojson'
import { geoGraticule, geoOrthographic, geoPath } from 'd3-geo'
import { onMounted, onUnmounted, ref } from 'vue'

const ANIMATION_MS = 4000
const MOBILE_PARTICLES = 360
const DESKTOP_PARTICLES = 650

const emit = defineEmits<{ reveal: [] }>()

const rootEl = ref<HTMLElement | null>(null)
const canvasMainEl = ref<HTMLCanvasElement | null>(null)

const palette = {
  graticule: 'rgba(10,10,10,0.09)',
  landFill: 'rgba(10,10,10,0.045)',
  coast: 'rgba(10,10,10,0.66)',
  river: 'rgba(10,10,10,0.34)',
  lakeFill: '#ffffff',
  lakeStroke: 'rgba(10,10,10,0.42)',
  rim: 'rgba(10,10,10,0.38)',
  backLand: 'rgba(10,10,10,0.05)',
  backCoast: 'rgba(10,10,10,0.10)',
  particle: '#0a0a0a'
}

const projection = geoOrthographic().clipAngle(90).precision(1)
const backProjection = geoOrthographic().clipAngle(90).reflectX(true).precision(1)
const graticule = geoGraticule()
const sphere = { type: 'Sphere' as const }

let land: FeatureCollection | null = null
let coast: FeatureCollection | null = null
let rivers: FeatureCollection | null = null
let lakes: FeatureCollection | null = null
let mainCtx: CanvasCtx | null = null
let raf = 0
let animationStart = 0
let animationStarted = false
let animationComplete = false
let isIntersecting = true
let isUnmounted = false
let staticMode = false
let enhancementHandle: number | ReturnType<typeof setTimeout> | null = null
let enhancementUsesIdleCallback = false
let observer: IntersectionObserver | null = null
let particles: Array<{ x: number; y: number; z: number }> = []

interface CanvasCtx {
  ctx: CanvasRenderingContext2D
  pf: ReturnType<typeof geoPath>
  pb: ReturnType<typeof geoPath>
  w: number
  h: number
}

function clamp(value: number, min = 0, max = 1) {
  return Math.min(max, Math.max(min, value))
}

function smoothstep(value: number) {
  const progress = clamp(value)
  return progress * progress * (3 - 2 * progress)
}

function isMobile() {
  return window.innerWidth < 768
}

function globeRadius() {
  const width = window.innerWidth
  const base = isMobile()
    ? Math.min(Math.max(width * 0.62, 200), 300)
    : Math.min(Math.max(width * 0.38, 280), 470)
  return (base / 3) * 1.32
}

function mainGlobeCenterY(canvas: CanvasCtx) {
  return isMobile() ? canvas.h * 0.9 : canvas.h * 1.2
}

function makeCtx(canvas: HTMLCanvasElement | null): CanvasCtx | null {
  if (!canvas) return null
  const ctx = canvas.getContext('2d')
  if (!ctx) return null
  return {
    ctx,
    pf: geoPath(projection, ctx),
    pb: geoPath(backProjection, ctx),
    w: 0,
    h: 0
  }
}

function resize() {
  if (!mainCtx || !canvasMainEl.value) return
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  const parent = canvasMainEl.value.parentElement
  mainCtx.w = parent?.clientWidth || window.innerWidth
  mainCtx.h = parent?.clientHeight || window.innerHeight
  canvasMainEl.value.width = Math.round(mainCtx.w * dpr)
  canvasMainEl.value.height = Math.round(mainCtx.h * dpr)
  mainCtx.ctx.setTransform(dpr, 0, 0, dpr, 0, 0)

  if (staticMode || animationComplete) drawFrame(ANIMATION_MS)
}

async function loadEarth(name: string): Promise<FeatureCollection | null> {
  try {
    const response = await fetch(`/earth/${name}.lod.json`)
    if (!response.ok) return null
    return await response.json() as FeatureCollection
  } catch {
    return null
  }
}

function drawGlobe(canvas: CanvasCtx, cx: number, cy: number, radius: number) {
  projection.translate([cx, cy]).scale(radius)
  backProjection.translate([cx, cy]).scale(radius)
  const { ctx, pf, pb } = canvas

  if (land) {
    ctx.beginPath()
    pb(land)
    ctx.fillStyle = palette.backLand
    ctx.fill()
  }
  if (coast) {
    ctx.beginPath()
    pb(coast)
    ctx.strokeStyle = palette.backCoast
    ctx.lineWidth = 0.7
    ctx.stroke()
  }

  ctx.beginPath()
  pf(graticule())
  ctx.strokeStyle = palette.graticule
  ctx.lineWidth = 0.6
  ctx.stroke()

  if (land) {
    ctx.beginPath()
    pf(land)
    ctx.fillStyle = palette.landFill
    ctx.fill()
  }
  if (lakes) {
    ctx.beginPath()
    pf(lakes)
    ctx.fillStyle = palette.lakeFill
    ctx.fill()
    ctx.strokeStyle = palette.lakeStroke
    ctx.lineWidth = 0.5
    ctx.stroke()
  }
  if (rivers) {
    ctx.beginPath()
    for (const feature of rivers.features) {
      if ((feature.properties?.sr ?? 0) <= 8) pf(feature)
    }
    ctx.strokeStyle = palette.river
    ctx.lineWidth = 0.55
    ctx.stroke()
  }
  if (coast) {
    ctx.beginPath()
    pf(coast)
    ctx.strokeStyle = palette.coast
    ctx.lineWidth = 0.9
    ctx.stroke()
  }

  ctx.beginPath()
  pf(sphere)
  ctx.strokeStyle = palette.rim
  ctx.lineWidth = 1
  ctx.stroke()
}

function drawParticles(canvas: CanvasCtx, radius: number, alpha: number, cy: number) {
  if (!particles.length || alpha <= 0) return
  const { ctx, w } = canvas
  const cx = w / 2
  const rotation = projection.rotate()[0] * Math.PI / 180
  const cosY = Math.cos(rotation)
  const sinY = Math.sin(rotation)

  ctx.fillStyle = palette.particle
  for (const particle of particles) {
    const x = particle.x * cosY + particle.z * sinY
    const z = -particle.x * sinY + particle.z * cosY
    const depth = (z + 1) / 2
    ctx.globalAlpha = alpha * (0.2 + depth * 0.8)
    ctx.beginPath()
    ctx.arc(cx + x * radius, cy - particle.y * radius, 0.45 + depth * 0.7, 0, Math.PI * 2)
    ctx.fill()
  }
  ctx.globalAlpha = 1
}

function drawFrame(elapsed: number) {
  if (!mainCtx) return
  const progress = smoothstep(elapsed / ANIMATION_MS)
  const rotation = 18 * progress
  projection.rotate([-rotation, -18])
  backProjection.rotate([-rotation + 180, -18])

  const canvas = mainCtx
  canvas.ctx.clearRect(0, 0, canvas.w, canvas.h)
  const centerY = mainGlobeCenterY(canvas)
  const radius = globeRadius()
  drawGlobe(canvas, canvas.w / 2, centerY, radius)
  if (!staticMode) {
    drawParticles(canvas, radius, 0.025 * progress, centerY)
  }
}

function stopAnimation() {
  if (!raf) return
  window.cancelAnimationFrame(raf)
  raf = 0
}

function frame(timestamp: number) {
  raf = 0
  if (document.hidden || !isIntersecting || isUnmounted) return

  const elapsed = Math.max(0, timestamp - animationStart)
  drawFrame(Math.min(elapsed, ANIMATION_MS))
  if (elapsed >= ANIMATION_MS) {
    animationComplete = true
    return
  }
  raf = window.requestAnimationFrame(frame)
}

function startAnimation() {
  if (staticMode || animationComplete || raf || document.hidden || !isIntersecting || isUnmounted) return
  if (!animationStarted) {
    animationStarted = true
    animationStart = performance.now()
  }
  raf = window.requestAnimationFrame(frame)
}

function handleVisibilityChange() {
  if (document.hidden) {
    stopAnimation()
    return
  }
  startAnimation()
}

function buildParticles(count: number) {
  const golden = Math.PI * (3 - Math.sqrt(5))
  return Array.from({ length: count }, (_, index) => {
    const y = 1 - (index / Math.max(1, count - 1)) * 2
    const radius = Math.sqrt(Math.max(0, 1 - y * y))
    const theta = golden * index
    return { x: Math.cos(theta) * radius, y, z: Math.sin(theta) * radius }
  })
}

function shouldUseStaticMode() {
  const navigatorWithHints = navigator as Navigator & {
    connection?: { saveData?: boolean }
    deviceMemory?: number
  }
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
    || navigatorWithHints.connection?.saveData === true
    || (navigator.hardwareConcurrency > 0 && navigator.hardwareConcurrency <= 4)
    || (typeof navigatorWithHints.deviceMemory === 'number' && navigatorWithHints.deviceMemory <= 4)
}

async function loadEnhancements() {
  const [loadedRivers, loadedLakes] = await Promise.all([
    loadEarth('rivers50'),
    loadEarth('lakes50')
  ])
  if (isUnmounted) return
  rivers = loadedRivers
  lakes = loadedLakes
  if (staticMode || animationComplete) drawFrame(ANIMATION_MS)
}

function scheduleEnhancements() {
  if (staticMode || isUnmounted) return
  if (typeof window.requestIdleCallback === 'function') {
    enhancementUsesIdleCallback = true
    enhancementHandle = window.requestIdleCallback(() => {
      enhancementHandle = null
      void loadEnhancements()
    }, { timeout: 2500 })
    return
  }
  enhancementUsesIdleCallback = false
  enhancementHandle = window.setTimeout(() => {
    enhancementHandle = null
    void loadEnhancements()
  }, 1200)
}

onMounted(() => {
  mainCtx = makeCtx(canvasMainEl.value)
  staticMode = shouldUseStaticMode()
  particles = staticMode ? [] : buildParticles(isMobile() ? MOBILE_PARTICLES : DESKTOP_PARTICLES)
  resize()
  emit('reveal')

  window.addEventListener('resize', resize)
  document.addEventListener('visibilitychange', handleVisibilityChange)

  if (typeof IntersectionObserver === 'function' && rootEl.value) {
    observer = new IntersectionObserver((entries) => {
      const entry = entries[0]
      if (!entry) return
      isIntersecting = entry.isIntersecting
      if (isIntersecting) startAnimation()
      else stopAnimation()
    })
    observer.observe(rootEl.value)
  }

  if (staticMode) drawFrame(ANIMATION_MS)
  else startAnimation()

  void Promise.all([loadEarth('land50'), loadEarth('coast50')]).then(([loadedLand, loadedCoast]) => {
    if (isUnmounted) return
    land = loadedLand
    coast = loadedCoast
    if (staticMode || animationComplete) drawFrame(ANIMATION_MS)
    scheduleEnhancements()
  })
})

onUnmounted(() => {
  isUnmounted = true
  stopAnimation()
  observer?.disconnect()
  window.removeEventListener('resize', resize)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  if (enhancementHandle !== null) {
    if (enhancementUsesIdleCallback && typeof window.cancelIdleCallback === 'function') {
      window.cancelIdleCallback(enhancementHandle as number)
    } else {
      clearTimeout(enhancementHandle)
    }
  }
})
</script>
