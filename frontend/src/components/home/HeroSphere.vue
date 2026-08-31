<template>
  <div ref="rootEl" class="hs-root" aria-hidden="true">
    <canvas ref="canvasMainEl" class="hs-canvas" />
  </div>
</template>

<script setup lang="ts">
import type { FeatureCollection } from 'geojson'
import { geoGraticule, geoOrthographic, geoPath } from 'd3-geo'
import { onMounted, onUnmounted, ref } from 'vue'

const ANIMATION_MS = 1600
const MAX_FRAME_INTERVAL_MS = 50
const DESKTOP_PARTICLES = 240

const emit = defineEmits<{ reveal: [] }>()

const rootEl = ref<HTMLElement | null>(null)
const canvasMainEl = ref<HTMLCanvasElement | null>(null)

interface GlobePalette {
  graticule: string
  landFill: string
  coast: string
  river: string
  lakeFill: string
  lakeStroke: string
  rim: string
  backLand: string
  backCoast: string
  particle: string
}

const lightPalette: GlobePalette = {
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

function canvasThemeColor(tone: 'ink' | 'paper' | 'darkBase', alpha: number) {
  const rgb = tone === 'ink' ? '10,10,10' : tone === 'paper' ? '245,245,244' : '20,20,20'
  // design-governance-allow: raw-color - Canvas2D cannot resolve CSS semantic tokens; this reviewed art-only helper maps the existing ink/paper home tokens to alpha-safe paint values.
  return `rgba(${rgb},${alpha})`
}

const darkPalette: GlobePalette = {
  graticule: canvasThemeColor('paper', 0.16),
  landFill: canvasThemeColor('paper', 0.06),
  coast: canvasThemeColor('paper', 0.60),
  river: canvasThemeColor('paper', 0.38),
  lakeFill: canvasThemeColor('darkBase', 0.90),
  lakeStroke: canvasThemeColor('paper', 0.42),
  rim: canvasThemeColor('paper', 0.50),
  backLand: canvasThemeColor('paper', 0.05),
  backCoast: canvasThemeColor('paper', 0.14),
  particle: canvasThemeColor('paper', 1)
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
let lastFrameAt = -Infinity
let animationStarted = false
let animationComplete = false
let isIntersecting = true
let isUnmounted = false
let staticMode = false
let enhancementHandle: number | ReturnType<typeof setTimeout> | null = null
let enhancementUsesIdleCallback = false
let enhancementsStarted = false
let observer: IntersectionObserver | null = null
let themeObserver: MutationObserver | null = null
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

function mainGlobeCenter(canvas: CanvasCtx) {
  // Keep the poster legible in the first viewport without competing with the CTA/status stack.
  if (isMobile()) {
    return { x: canvas.w * 0.86, y: canvas.h * 0.48 }
  }
  return { x: canvas.w * 0.82, y: canvas.h * 0.83 }
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

function paletteForCurrentTheme() {
  return document.documentElement.classList.contains('dark') ? darkPalette : lightPalette
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
  const palette = paletteForCurrentTheme()

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
  const palette = paletteForCurrentTheme()
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
  const center = mainGlobeCenter(canvas)
  const radius = globeRadius()
  drawGlobe(canvas, center.x, center.y, radius)
  if (!staticMode) {
    drawParticles(canvas, radius, 0.025 * progress, center.y)
  }
}

function redrawForThemeChange() {
  if (!mainCtx || isUnmounted) return
  const elapsed = animationComplete || staticMode
    ? ANIMATION_MS
    : animationStarted
      ? Math.min(Math.max(0, performance.now() - animationStart), ANIMATION_MS)
      : 0
  drawFrame(elapsed)
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
  if (timestamp - lastFrameAt >= MAX_FRAME_INTERVAL_MS || elapsed >= ANIMATION_MS) {
    drawFrame(Math.min(elapsed, ANIMATION_MS))
    lastFrameAt = timestamp
  }
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
    lastFrameAt = -Infinity
  }
  raf = window.requestAnimationFrame(frame)
}

function handleVisibilityChange() {
  if (document.hidden) {
    stopAnimation()
    return
  }
  startAnimation()
  scheduleEnhancements()
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
  return isMobile()
    || window.matchMedia('(pointer: coarse)').matches
    || window.matchMedia('(hover: none)').matches
    || window.matchMedia('(prefers-reduced-motion: reduce)').matches
    || navigatorWithHints.connection?.saveData === true
    || (navigator.hardwareConcurrency > 0 && navigator.hardwareConcurrency <= 4)
    || (typeof navigatorWithHints.deviceMemory === 'number' && navigatorWithHints.deviceMemory <= 4)
}

async function loadEnhancements() {
	if (isUnmounted || document.hidden || !isIntersecting || staticMode) return
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
  if (staticMode || isUnmounted || document.hidden || !isIntersecting || enhancementsStarted || enhancementHandle !== null) return

  const run = () => {
    enhancementHandle = null
    if (isUnmounted || document.hidden || !isIntersecting || staticMode) return
    enhancementsStarted = true
    void Promise.all([loadEarth('land50'), loadEarth('coast50'), loadEnhancements()]).then(([loadedLand, loadedCoast]) => {
      if (isUnmounted) return
      land = loadedLand
      coast = loadedCoast
      if (animationComplete) drawFrame(ANIMATION_MS)
    })
  }

  if (typeof window.requestIdleCallback === 'function') {
    enhancementUsesIdleCallback = true
    enhancementHandle = window.requestIdleCallback(run, { timeout: 2500 })
    return
  }
  enhancementUsesIdleCallback = false
  enhancementHandle = window.setTimeout(run, 1200)
}

onMounted(() => {
  mainCtx = makeCtx(canvasMainEl.value)
  staticMode = shouldUseStaticMode()
  particles = staticMode ? [] : buildParticles(DESKTOP_PARTICLES)
  resize()
  // The poster is independent of optional geography assets and must not delay the CTA.
  drawFrame(staticMode ? ANIMATION_MS : 0)
  emit('reveal')

  window.addEventListener('resize', resize)
  document.addEventListener('visibilitychange', handleVisibilityChange)

  if (typeof MutationObserver === 'function') {
    themeObserver = new MutationObserver((mutations) => {
      if (mutations.some((mutation) => mutation.attributeName === 'class')) {
        redrawForThemeChange()
      }
    })
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  }

  if (typeof IntersectionObserver === 'function' && rootEl.value) {
    observer = new IntersectionObserver((entries) => {
      const entry = entries[0]
      if (!entry) return
      isIntersecting = entry.isIntersecting
      if (isIntersecting) {
        startAnimation()
        scheduleEnhancements()
      } else {
        stopAnimation()
      }
    })
    observer.observe(rootEl.value)
  }

  if (staticMode) return
  startAnimation()
  scheduleEnhancements()
})

onUnmounted(() => {
  isUnmounted = true
  stopAnimation()
  observer?.disconnect()
  themeObserver?.disconnect()
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
