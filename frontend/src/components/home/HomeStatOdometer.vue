<template>
  <span class="stat-value" aria-hidden="true">
    <template v-for="col in columns" :key="col.key">
      <span v-if="col.type === 'static'" class="od-static">{{ col.ch }}</span>
      <span v-else class="od-col">
        <span
          class="od-strip"
          :class="{ 'is-animating': col.animating }"
          :style="stripStyle(col)"
          @transitionend="onStripEnd(col.key)"
        >
          <span v-for="n in stripLen" :key="n" class="od-d">{{ (n - 1) % 10 }}</span>
        </span>
      </span>
    </template>
    <span class="stat-unit">{{ unit }}</span>
  </span>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import {
  ODOMETER_DEFAULT_SPIN_TAIL,
  ODOMETER_STRIP_LEN,
  type OdometerColumnState,
  type OdometerDigitState,
  syncColumnsFromValue,
} from '@/utils/odometerColumns'

const props = withDefaults(
  defineProps<{
    value: string
    unit: string
    active?: boolean
    spinTail?: number
  }>(),
  {
    active: false,
    spinTail: ODOMETER_DEFAULT_SPIN_TAIL,
  },
)

const stripLen = ODOMETER_STRIP_LEN
const columns = ref<OdometerColumnState[]>([])
let entered = false

function stripStyle(col: OdometerDigitState) {
  return {
    transform: `translate3d(0, ${-col.offset}em, 0)`,
    transitionDuration: col.animating ? `${col.durationMs}ms` : '0ms',
    transitionDelay: col.animating ? `${col.delayMs}ms` : '0ms',
  }
}

function onStripEnd(key: string) {
  columns.value = columns.value.map((col) => {
    if (col.type !== 'digit' || col.key !== key) return col
    return { ...col, animating: false }
  })
}

function applyValue(entrance: boolean) {
  columns.value = syncColumnsFromValue(columns.value, props.value, props.spinTail, {
    animateTail: props.active,
    entrance,
  })
}

async function runEntrance() {
  columns.value = syncColumnsFromValue([], props.value, props.spinTail, {
    animateTail: false,
    entrance: false,
  })
  columns.value = columns.value.map((col) =>
    col.type === 'digit' ? { ...col, offset: 0, animating: false, durationMs: 0, delayMs: 0 } : col,
  )
  await nextTick()
  requestAnimationFrame(() => {
    columns.value = syncColumnsFromValue(columns.value, props.value, props.spinTail, {
      animateTail: true,
      entrance: true,
    })
  })
}

watch(
  () => props.value,
  () => {
    if (entered) {
      applyValue(false)
      return
    }
    applyValue(false)
  },
  { immediate: true },
)

watch(
  () => props.active,
  (active) => {
    if (!active || entered) return
    entered = true
    void runEntrance()
  },
  { immediate: true },
)
</script>

<style scoped>
.od-strip {
  display: block;
  transform: translate3d(0, 0, 0);
  transition-property: none;
  will-change: auto;
}

.od-strip.is-animating {
  transition-property: transform;
  transition-timing-function: cubic-bezier(0.16, 1, 0.3, 1);
  will-change: transform;
}
</style>
