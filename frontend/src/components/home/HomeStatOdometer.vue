<template>
  <span class="stat-value" aria-hidden="true">
    <template v-for="col in columns" :key="col.key">
      <span v-if="col.type === 'static'" class="od-static">{{ col.ch }}</span>
      <span v-else class="od-col">
        <span
          class="od-strip"
          :class="{ 'is-animating': col.animating }"
          :style="stripStyle(col)"
          @transitionend="onStripEnd(col.key, $event)"
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
  landOffset,
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
    transform: `translate3d(0, calc(-1 * ${col.offset} * var(--od-cell)), 0)`,
    transitionDuration: col.animating ? `${col.durationMs}ms` : '0ms',
    transitionDelay: col.animating ? `${col.delayMs}ms` : '0ms',
  }
}

function onStripEnd(key: string, ev: TransitionEvent) {
  if (ev.propertyName !== 'transform') return
  columns.value = columns.value.map((col) => {
    if (col.type !== 'digit' || col.key !== key) return col
    return {
      ...col,
      offset: landOffset(col.targetDigit),
      animating: false,
      durationMs: 0,
      delayMs: 0,
    }
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
    applyValue(entered)
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
.stat-value {
  --od-cell: 1.12em;
  display: inline-flex;
  align-items: baseline;
  flex-wrap: nowrap;
  max-width: 100%;
  font-variant-numeric: tabular-nums;
}

.od-static {
  flex-shrink: 0;
  line-height: var(--od-cell);
}

.od-col {
  display: block;
  flex-shrink: 0;
  width: 0.62em;
  height: var(--od-cell);
  overflow: hidden;
}

.od-d {
  display: block;
  height: var(--od-cell);
  line-height: var(--od-cell);
  text-align: center;
}

.od-strip {
  display: block;
  transform: translate3d(0, 0, 0);
  transition-property: none;
}

.od-strip.is-animating {
  transition-property: transform;
  transition-timing-function: cubic-bezier(0.16, 1, 0.3, 1);
}

.stat-unit {
  flex-shrink: 0;
  margin-left: 0.14em;
}
</style>
