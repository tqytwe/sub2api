<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import SupportContactPanel from '@/components/common/SupportContactPanel.vue'
import '@/styles/support-floating.css'

const props = withDefaults(defineProps<{
  hideOnMobile?: boolean
}>(), {
  hideOnMobile: false,
})

const PEEK_KEY = 'support_fab_peeked'
const OFFSET_KEY = 'support_fab_offset'

const { t } = useI18n()
const appStore = useAppStore()

const panelOpen = ref(false)
const isDragging = ref(false)
const isPeek = ref(false)
const fabRef = ref<HTMLElement | null>(null)
const offset = ref({ x: 0, y: 0 })

const hasSupportContact = computed(() => appStore.supportContact.contacts.length > 0)

const fabStyle = computed(() => {
  if (!offset.value.x && !offset.value.y) return undefined
  return {
    transform: `translate(${offset.value.x}px, ${offset.value.y}px)`
  }
})

let peekTimer: number | null = null
let peekHideTimer: number | null = null
let dragStart = { x: 0, y: 0, ox: 0, oy: 0 }
let moved = false
let lastSupportRefreshAt = 0

function restoreOffset() {
  try {
    const raw = localStorage.getItem(OFFSET_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw) as { x?: number; y?: number }
    offset.value = { x: parsed.x ?? 0, y: parsed.y ?? 0 }
  } catch {
    /* ignore */
  }
}

function saveOffset() {
  try {
    localStorage.setItem(OFFSET_KEY, JSON.stringify(offset.value))
  } catch {
    /* ignore */
  }
}

function schedulePeek() {
  try {
    if (sessionStorage.getItem(PEEK_KEY)) return
  } catch {
    /* ignore */
  }
  peekTimer = window.setTimeout(() => {
    isPeek.value = true
    try {
      sessionStorage.setItem(PEEK_KEY, '1')
    } catch {
      /* ignore */
    }
    peekHideTimer = window.setTimeout(() => {
      isPeek.value = false
    }, 3600)
  }, 900)
}

function clearPeekTimers() {
  if (peekTimer) window.clearTimeout(peekTimer)
  if (peekHideTimer) window.clearTimeout(peekHideTimer)
  peekTimer = null
  peekHideTimer = null
}

async function refreshSupportContactIfNeeded() {
  const now = Date.now()
  if (now - lastSupportRefreshAt < 30_000) return
  lastSupportRefreshAt = now
  await appStore.fetchPublicSettings(true)
}

function togglePanel() {
  if (moved) {
    moved = false
    return
  }
  panelOpen.value = !panelOpen.value
  if (panelOpen.value) {
    void refreshSupportContactIfNeeded()
  }
}

function closePanel() {
  panelOpen.value = false
}

function onPointerDown(event: PointerEvent) {
  if (panelOpen.value) return
  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
  isDragging.value = true
  moved = false
  dragStart = {
    x: event.clientX,
    y: event.clientY,
    ox: offset.value.x,
    oy: offset.value.y
  }
}

function onPointerMove(event: PointerEvent) {
  if (!isDragging.value) return
  const dx = event.clientX - dragStart.x
  const dy = event.clientY - dragStart.y
  if (Math.abs(dx) > 4 || Math.abs(dy) > 4) moved = true
  offset.value = { x: dragStart.ox + dx, y: dragStart.oy + dy }
}

function onPointerUp(event: PointerEvent) {
  if (!isDragging.value) return
  const target = event.currentTarget as HTMLElement
  if (target.hasPointerCapture(event.pointerId)) {
    target.releasePointerCapture(event.pointerId)
  }
  isDragging.value = false
  saveOffset()
}

onMounted(() => {
  restoreOffset()
  schedulePeek()
})

onBeforeUnmount(() => {
  clearPeekTimers()
})
</script>

<template>
  <div
    v-if="hasSupportContact"
    ref="fabRef"
    class="support-fab"
    :class="{ 'is-open': panelOpen, 'support-fab--mobile-hidden': props.hideOnMobile }"
    :style="fabStyle"
  >
    <Transition name="support-pop">
      <div v-if="panelOpen" class="support-panel" role="dialog" aria-modal="true">
        <button type="button" class="support-close" :aria-label="t('common.close')" @click="closePanel">
          <span aria-hidden="true">×</span>
        </button>
        <SupportContactPanel
          v-if="hasSupportContact"
          :config="appStore.supportContact"
          compact
        />
      </div>
    </Transition>

    <button
      type="button"
      class="support-trigger"
      :class="{ 'is-dragging': isDragging, 'is-peek': isPeek }"
      :aria-expanded="panelOpen"
      :aria-label="t('support.trigger')"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @click="togglePanel"
    >
      <span class="support-trigger-orb">
        <svg
          class="support-trigger-icon"
          viewBox="0 0 24 24"
          width="20"
          height="20"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M4 12a8 8 0 0 1 16 0v5a3 3 0 0 1-3 3h-2" />
          <rect x="3" y="11" width="4" height="6" rx="2" />
          <rect x="17" y="11" width="4" height="6" rx="2" />
        </svg>
      </span>
      <span class="support-trigger-text">{{ t('support.trigger') }}</span>
    </button>
  </div>
</template>
