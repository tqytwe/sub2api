<template>
  <span class="stat-value" aria-hidden="true">
    <span v-if="stable" class="od-stable">{{ stable }}</span>
    <span v-for="col in tails" :key="col.key" class="od-col">
      <span
        class="od-strip"
        :class="{ 'is-animating': col.animating }"
        :style="stripStyle(col)"
        @transitionend="onStripEnd(col.key, $event)"
      >
        <span v-for="n in stripLen" :key="n" class="od-d">{{ (n - 1) % 10 }}</span>
      </span>
    </span>
    <span class="stat-unit">{{ unit }}</span>
  </span>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import {
  ODOMETER_DEFAULT_SPIN_TAIL,
  ODOMETER_STRIP_LEN,
  landOffset,
  type TailDigitState,
  syncTailDigits,
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
const stable = ref('')
const tails = ref<TailDigitState[]>([])
let entered = false

function stripStyle(col: TailDigitState) {
  return {
    transform: `translate3d(0, calc(${-col.offset} * 1em), 0)`,
    transitionDuration: col.animating ? `${col.durationMs}ms` : '0ms',
    transitionDelay: col.animating ? `${col.delayMs}ms` : '0ms',
  }
}

function onStripEnd(key: string, ev: TransitionEvent) {
  if (ev.propertyName !== 'transform') return
  tails.value = tails.value.map((col) => {
    if (col.key !== key) return col
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
  const next = syncTailDigits(tails.value, props.value, props.spinTail, {
    animate: props.active,
    entrance,
  })
  stable.value = next.stable
  tails.value = next.tails
}

async function runEntrance() {
  const next = syncTailDigits([], props.value, props.spinTail, {
    animate: false,
    entrance: false,
  })
  stable.value = next.stable
  tails.value = next.tails.map((col) => ({ ...col, offset: 0, animating: false }))

  await nextTick()
  requestAnimationFrame(() => {
    const rolled = syncTailDigits(tails.value, props.value, props.spinTail, {
      animate: true,
      entrance: true,
    })
    stable.value = rolled.stable
    tails.value = rolled.tails
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
  display: inline-flex;
  align-items: baseline;
  flex-wrap: nowrap;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.od-stable {
  flex-shrink: 0;
}

.od-col {
  display: block;
  flex-shrink: 0;
  width: 0.62em;
  height: 1em;
  overflow: hidden;
}

.od-d {
  display: block;
  height: 1em;
  line-height: 1em;
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
