<template>
  <div class="hs-root" aria-hidden="true">
    <div v-if="introActive" ref="backdropEl" class="hs-backdrop" />
    <canvas v-if="introActive" ref="canvasIntroEl" class="hs-canvas hs-intro" />
    <canvas ref="canvasMainEl" class="hs-canvas" />
    <div class="hs-stage" :class="{ 'is-fixed': introActive }">
      <div
        v-for="(model, idx) in visibleModels"
        :key="model.name"
        :ref="(el) => setLogoRef(el as HTMLElement | null, idx)"
        class="hs-logo"
      >
        <span class="hs-logo-badge">
          <svg viewBox="0 0 24 24"><path :d="model.path" /></svg>
        </span>
        <span class="hs-logo-name">{{ model.name }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { FeatureCollection } from 'geojson'
import { geoDistance, geoGraticule, geoOrthographic, geoPath } from 'd3-geo'
import { onMounted, onUnmounted, ref } from 'vue'
import { MODEL_ICONS } from './model-icons'

const INTRO_KEY = 'jd-home-intro-seen'
const INTRO_MS = 1900
const TRANSITION_MS = 1150
const REVEAL_MS = 2150
const PARTICLE_MS = 2150
const SPIN_START = 600
const SPIN_END = 1100

const emit = defineEmits<{ reveal: [] }>()

const introActive = ref(!hasSeenIntro())
const backdropEl = ref<HTMLElement | null>(null)
const canvasIntroEl = ref<HTMLCanvasElement | null>(null)
const canvasMainEl = ref<HTMLCanvasElement | null>(null)
const logoRefs = ref<(HTMLElement | null)[]>([])

const visibleModels = MODEL_ICONS.slice(0, typeof window !== 'undefined' && window.innerWidth < 768 ? 6 : 8)

function hasSeenIntro() {
  try {
    return sessionStorage.getItem(INTRO_KEY) === '1'
  } catch {
    return false
  }
}

function markIntroSeen() {
  try {
    sessionStorage.setItem(INTRO_KEY, '1')
  } catch {
    /* ignore */
  }
}

function setLogoRef(el: HTMLElement | null, idx: number) {
  logoRefs.value[idx] = el
}

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

let introCtx: CanvasCtx | null = null
let mainCtx: CanvasCtx | null = null
let raf = 0
let startTs = 0
let lastTs = 0
let spin = 0
let scrollUnlocked = false
let introDone = false
let particles: Array<{ x: number; y: number; z: number; delay: number; scatter: number }> = []

interface CanvasCtx {
  ctx: CanvasRenderingContext2D
  pf: ReturnType<typeof geoPath>
  pb: ReturnType<typeof geoPath>
  w: number
  h: number
}

const anchorPoints = [
  { lat: 27, lon: 0 },
  { lat: -16, lon: 45 },
  { lat: 7, lon: 90 },
  { lat: -27, lon: 135 },
  { lat: 19, lon: 180 },
  { lat: -5, lon: 225 },
  { lat: 30, lon: 270 },
  { lat: -22, lon: 315 }
]

function clamp(n: number, lo = 0, hi = 1) {
  return Math.min(hi, Math.max(lo, n))
}

function easeOutCubic(t: number) {
  return 1 - (1 - t) ** 3
}

function smoothstep(t: number) {
  const p = clamp(t)
  return p * p * (3 - 2 * p)
}

function isMobile() {
  return window.innerWidth < 768
}

function globeRadius() {
  const w = window.innerWidth
  const base = isMobile()
    ? Math.min(Math.max(w * 0.62, 200), 300)
    : Math.min(Math.max(w * 0.38, 280), 470)
  return (base / 3) * 1.32
}

function mainGlobeCenterY(c: CanvasCtx) {
  return isMobile() ? c.h * 0.9 : c.h * 1.2
}

function introRadius() {
  return Math.min(window.innerWidth, window.innerHeight) * 0.46
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
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  if (introCtx && canvasIntroEl.value) {
    introCtx.w = window.innerWidth
    introCtx.h = window.innerHeight
    canvasIntroEl.value.width = Math.round(introCtx.w * dpr)
    canvasIntroEl.value.height = Math.round(introCtx.h * dpr)
    introCtx.ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  }
  if (mainCtx && canvasMainEl.value) {
    const parent = canvasMainEl.value.parentElement
    mainCtx.w = parent?.clientWidth ?? window.innerWidth
    mainCtx.h = parent?.clientHeight ?? window.innerHeight
    canvasMainEl.value.width = Math.round(mainCtx.w * dpr)
    canvasMainEl.value.height = Math.round(mainCtx.h * dpr)
    mainCtx.ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  }
}

function loadEarth(name: string) {
  return fetch(`/earth/${name}.lod.json`).then((r) => (r.ok ? r.json() : null))
}

function drawGlobe(
  c: CanvasCtx,
  cx: number,
  cy: number,
  r: number,
  alpha: number,
  backOnly = false
) {
  if (alpha < 0.004) return
  projection.translate([cx, cy]).scale(r)
  backProjection.translate([cx, cy]).scale(r)
  const { ctx, pf, pb } = c
  ctx.globalAlpha = alpha

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

  if (!backOnly && land) {
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
    for (const f of rivers.features) {
      if ((f.properties?.sr ?? 0) <= 8) pf(f)
    }
    ctx.strokeStyle = palette.river
    ctx.lineWidth = 0.55
    ctx.stroke()
  }
  if (!backOnly && coast) {
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
  ctx.globalAlpha = 1
}

function projectPoint(lon: number, lat: number) {
  const rot = projection.rotate()
  const dist = geoDistance([lon, lat], [-rot[0], -rot[1]])
  const pt = projection([lon, lat])
  return {
    x: pt?.[0] ?? 0,
    y: pt?.[1] ?? 0,
    depth: Math.cos(dist)
  }
}

function drawParticles(c: CanvasCtx, r: number, alpha: number, cy = c.h / 2) {
  if (alpha < 0.01 || !particles.length) return
  const { ctx, w } = c
  const cx = w / 2
  const cosY = Math.cos((spin * Math.PI) / 180)
  const sinY = Math.sin((spin * Math.PI) / 180)
  const tilt = (-18 * Math.PI) / 180
  const cosX = Math.cos(tilt)
  const sinX = Math.sin(tilt)
  const k = 4.6

  ctx.fillStyle = palette.particle
  for (const p of particles) {
    const t = clamp((performance.now() - startTs - PARTICLE_MS - p.delay * SPIN_START) / SPIN_END)
    const eased = easeOutCubic(t)
    const scale = p.scatter + (1 - p.scatter) * eased
    const x1 = p.x * cosY + p.z * sinY
    const y1 = -p.x * sinY + p.z * cosY
    const y2 = p.y * cosX - y1 * sinX
    const z2 = p.y * sinX + y1 * cosX
    const depth = (z2 + 1) / 2
    const persp = k / (k + z2)
    const dotAlpha = (0.12 + depth * 0.78) * alpha * eased
    if (dotAlpha < 0.008) continue
    ctx.globalAlpha = dotAlpha
    ctx.beginPath()
    ctx.arc(cx + x1 * r * scale * persp, cy - y2 * r * scale * persp, (0.55 + depth * 0.95) * scale, 0, Math.PI * 2)
    ctx.fill()
  }
  ctx.globalAlpha = 1
}

function updateLogos(elapsed: number) {
  const diag = Math.hypot(window.innerWidth, window.innerHeight)
  for (let i = 0; i < visibleModels.length; i++) {
    const el = logoRefs.value[i]
    if (!el) continue
    const anchor = anchorPoints[i]
    let x = 0
    let y = 0
    let scale = 1
    let opacity = 0

    if (elapsed < INTRO_MS) {
      const t = clamp((elapsed - 300 - i * 45) / 320)
      if (t <= 0) {
        el.style.opacity = '0'
        continue
      }
      const p = projectPoint(anchor.lon, anchor.lat)
      const depth = (p.depth + 1) / 2
      x = p.x
      y = p.y
      scale = (0.72 + 0.42 * depth) * (0.3 + 0.7 * t)
      opacity = t * (0.3 + 0.7 * depth)
    } else if (elapsed < PARTICLE_MS + 400) {
      const t = clamp((elapsed - INTRO_MS) / 560)
      const p = projectPoint(anchor.lon, anchor.lat)
      const depth = (p.depth + 1) / 2
      const scatter = t * t * t
      x = p.x + p.x * diag * 0.0008 * scatter
      y = p.y + p.y * diag * 0.0008 * scatter
      scale = (0.72 + 0.42 * depth) * (1 + 0.3 * scatter)
      opacity = (0.3 + 0.7 * depth) * (1 - t)
    } else {
      el.style.opacity = '0'
      continue
    }

    el.style.transform = `translate3d(${x}px, ${y}px, 0) translate(-50%, -50%) scale(${scale})`
    el.style.opacity = String(opacity)
  }
}

function frame(ts: number) {
  raf = requestAnimationFrame(frame)
  const elapsed = ts - startTs
  const dt = Math.min(50, ts - lastTs || 16)
  lastTs = ts

  const spinning = elapsed >= INTRO_MS && elapsed < INTRO_MS + TRANSITION_MS + PARTICLE_MS + SPIN_END
  if (spinning || elapsed < INTRO_MS + TRANSITION_MS) {
    spin += (elapsed < INTRO_MS ? 9 : 2.9) * (dt / 1000)
    projection.rotate([-spin, -18])
    backProjection.rotate([-spin + 180, -18])
  }

  if (!scrollUnlocked && elapsed >= REVEAL_MS) {
    scrollUnlocked = true
    document.documentElement.style.overflow = ''
    document.body.style.overflow = ''
    if (backdropEl.value) backdropEl.value.style.pointerEvents = 'none'
    emit('reveal')
  }

  if (!introDone && elapsed >= INTRO_MS + TRANSITION_MS + PARTICLE_MS + SPIN_END) {
    introDone = true
    introActive.value = false
    markIntroSeen()
    resize()
  }

  if (introActive.value && introCtx) {
    const c = introCtx
    c.ctx.clearRect(0, 0, c.w, c.h)
    if (elapsed < INTRO_MS) {
      drawGlobe(c, c.w / 2, c.h / 2, introRadius(), 0.85)
    } else {
      const fade = 1 - smoothstep((elapsed - INTRO_MS) / TRANSITION_MS)
      drawGlobe(c, c.w / 2, c.h / 2, introRadius(), 0.85 * fade, true)
      if (backdropEl.value) backdropEl.value.style.opacity = String(1 - smoothstep((elapsed - INTRO_MS * 0.1) / (TRANSITION_MS * 0.75)))
    }
  }

  if (mainCtx) {
    const c = mainCtx
    c.ctx.clearRect(0, 0, c.w, c.h)
    if (elapsed >= PARTICLE_MS) {
      const appear = smoothstep((elapsed - PARTICLE_MS) / (SPIN_END + SPIN_START))
      const cy = mainGlobeCenterY(c)
      drawGlobe(c, c.w / 2, cy, globeRadius(), appear)
      drawParticles(c, globeRadius(), 0.13 * (isMobile() ? 0.24 : 0.3) * appear, cy)
    }
  }

  updateLogos(elapsed)
}

function buildParticles(count: number) {
  const golden = Math.PI * (3 - Math.sqrt(5))
  const out: typeof particles = []
  for (let i = 0; i < count; i++) {
    const y = 1 - (i / (count - 1)) * 2
    const radius = Math.sqrt(Math.max(0, 1 - y * y))
    const theta = golden * i
    out.push({
      x: Math.cos(theta) * radius,
      y,
      z: Math.sin(theta) * radius,
      delay: (i * 0.61803) % 1,
      scatter: 2.2 + ((i * 0.3819) % 1) * 2.6
    })
  }
  return out
}

onMounted(async () => {
  [land, coast, rivers, lakes] = await Promise.all([
    loadEarth('land50'),
    loadEarth('coast50'),
    loadEarth('rivers50'),
    loadEarth('lakes50')
  ])

  introCtx = makeCtx(canvasIntroEl.value)
  mainCtx = makeCtx(canvasMainEl.value)
  particles = buildParticles(isMobile() ? 950 : 1700)

  if (!introCtx && !mainCtx) {
    scrollUnlocked = true
    introDone = true
    introActive.value = false
    emit('reveal')
    return
  }

  resize()

  if (introActive.value) {
    document.documentElement.style.overflow = 'hidden'
    document.body.style.overflow = 'hidden'
    startTs = performance.now()
  } else {
    scrollUnlocked = true
    introDone = true
    emit('reveal')
    startTs = performance.now() - PARTICLE_MS
  }

  lastTs = performance.now()
  window.addEventListener('resize', resize)
  raf = requestAnimationFrame(frame)
})

onUnmounted(() => {
  cancelAnimationFrame(raf)
  window.removeEventListener('resize', resize)
  document.documentElement.style.overflow = ''
  document.body.style.overflow = ''
})
</script>
