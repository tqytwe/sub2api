<template>
  <div
    ref="rootRef"
    class="term"
    role="img"
    :aria-label="t('home.jisudeng.onboard.terminalAria')"
  >
    <div class="term-bar">
      <span class="term-dot term-dot-r" />
      <span class="term-dot term-dot-y" />
      <span class="term-dot term-dot-g" />
      <span class="term-bar-title">{{ t('home.jisudeng.onboard.terminalTitle') }}</span>
    </div>
    <div class="term-body">
      <div
        v-for="(line, idx) in lines"
        :key="`${cycle}-${idx}`"
        class="term-line"
        :class="`is-${line.kind}`"
      >
        <span v-if="line.kind === 'cmd'" class="term-ps">$</span>
        <span v-else-if="line.kind === 'ok'" class="term-mark-ok">✓</span>
        <span v-else-if="line.kind === 'ask'" class="term-mark-ask">?</span>
        <span class="term-text">{{ line.text }}</span>
        <span v-if="idx === lines.length - 1 && running" class="term-cursor" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

type LineKind = 'cmd' | 'ok' | 'ask' | 'out'

interface TermLine {
  kind: LineKind
  text: string
  typed?: boolean
  pause: number
  phase: number
}

const emit = defineEmits<{ phase: [number] }>()
const { t } = useI18n()

const script = computed<TermLine[]>(() => [
  {
    kind: 'cmd',
    text: 'curl -fsSL https://jisudeng.com/d/****** | bash',
    phase: 1,
    typed: true,
    pause: 750
  },
  { kind: 'ok', text: t('home.jisudeng.onboard.terminalEnv'), phase: 2, pause: 460 },
  { kind: 'ask', text: t('home.jisudeng.onboard.terminalPasteKey'), phase: 2, typed: true, pause: 520 },
  { kind: 'ask', text: t('home.jisudeng.onboard.terminalModel'), phase: 2, pause: 600 },
  { kind: 'ok', text: t('home.jisudeng.onboard.terminalConfig'), phase: 2, pause: 400 },
  { kind: 'ok', text: t('home.jisudeng.onboard.terminalSelfCheck'), phase: 2, pause: 950 },
  { kind: 'cmd', text: t('home.jisudeng.onboard.terminalIntroPrompt'), phase: 3, typed: true, pause: 650 },
  {
    kind: 'out',
    text: t('home.jisudeng.onboard.terminalIntroReply'),
    phase: 3,
    typed: true,
    pause: 3000
  }
])

const rootRef = ref<HTMLElement | null>(null)
const lines = ref<{ kind: LineKind; text: string }[]>([])
const running = ref(false)
const cycle = ref(0)

let step = 0
let charIdx = 0
let timer: ReturnType<typeof setTimeout> | null = null
let observer: IntersectionObserver | null = null

function schedule(fn: () => void, ms: number) {
  timer = setTimeout(fn, ms)
}

function tick() {
  if (!running.value) return

  const activeScript = script.value
  if (step >= activeScript.length) {
    schedule(() => {
      lines.value = []
      step = 0
      charIdx = 0
      cycle.value++
      emit('phase', 1)
      tick()
    }, 700)
    return
  }

  const item = activeScript[step]

  if (charIdx === 0) {
    emit('phase', item.phase)
    lines.value = [...lines.value, { kind: item.kind, text: item.typed ? '' : item.text }]
    if (!item.typed) {
      step++
      schedule(tick, item.pause)
      return
    }
  }

  if (charIdx < item.text.length) {
    charIdx++
    const last = lines.value[lines.value.length - 1]
    lines.value = [
      ...lines.value.slice(0, -1),
      { ...last, text: item.text.slice(0, charIdx) }
    ]
    schedule(tick, item.kind === 'out' ? 48 : 26)
    return
  }

  step++
  charIdx = 0
  schedule(tick, item.pause)
}

function setRunning(value: boolean) {
  if (running.value === value) return
  running.value = value
  if (timer) clearTimeout(timer)
  if (value) {
    charIdx = 0
    if (step < script.value.length && lines.value.length > step) {
      lines.value = lines.value.slice(0, step)
    }
    tick()
  }
}

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => entries.forEach((e) => setRunning(e.isIntersecting)),
    { threshold: 0.3 }
  )
  if (rootRef.value) observer.observe(rootRef.value)
})

onUnmounted(() => {
  if (timer) clearTimeout(timer)
  observer?.disconnect()
})
</script>
