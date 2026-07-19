<script setup lang="ts">
import { computed } from 'vue'
import { normalizeVIPColorKey } from '@/utils/vipColors'

const props = withDefaults(defineProps<{
  open: boolean
  title: string
  amount: string
  subtitle?: string
  details?: string[]
  vipLabel?: string
  colorKey?: string
  variant?: 'standard' | 'jackpot' | 'settlement'
  primaryLabel?: string
  secondaryLabel?: string
}>(), {
  subtitle: '',
  details: () => [],
  vipLabel: '',
  colorKey: 'neutral',
  variant: 'standard',
  primaryLabel: '',
  secondaryLabel: '',
})

const emit = defineEmits<{
  close: []
  primary: []
  secondary: []
}>()

const tone = computed(() => normalizeVIPColorKey(props.colorKey))
const pieces = Array.from({ length: 24 }, (_, index) => ({
  id: index,
  x: 8 + ((index * 17) % 84),
  delay: (index % 8) * 0.055,
  drift: ((index % 5) - 2) * 18,
  spin: index % 2 === 0 ? 1 : -1,
}))

function pieceStyle(piece: typeof pieces[number]) {
  return {
    left: `${piece.x}%`,
    animationDelay: `${piece.delay}s`,
    '--reward-drift': `${piece.drift}px`,
    '--reward-spin': `${piece.spin}`,
  }
}
</script>

<template>
  <div
    v-if="open"
    class="reward-celebration-overlay"
    :class="[`tone-${tone}`, `variant-${variant}`]"
    role="dialog"
    aria-modal="true"
  >
    <div class="reward-celebration-backdrop" @click="emit('close')" />
    <div class="reward-confetti" aria-hidden="true">
      <span
        v-for="piece in pieces"
        :key="piece.id"
        :style="pieceStyle(piece)"
      />
    </div>
    <section class="reward-celebration-card">
      <button type="button" class="reward-close" aria-label="Close" @click="emit('close')">x</button>
      <div class="reward-box-stage" aria-hidden="true">
        <div class="reward-beam" />
        <div class="reward-box">
          <span class="reward-box-lid" />
          <span class="reward-box-body" />
        </div>
      </div>
      <span v-if="vipLabel" class="reward-vip-badge">{{ vipLabel }}</span>
      <p class="reward-title">{{ title }}</p>
      <strong class="reward-amount">{{ amount }}</strong>
      <p v-if="subtitle" class="reward-subtitle">{{ subtitle }}</p>
      <ul v-if="details.length" class="reward-detail-list">
        <li v-for="detail in details" :key="detail">{{ detail }}</li>
      </ul>
      <div class="reward-actions">
        <button v-if="primaryLabel" type="button" class="play-btn play-btn-primary" @click="emit('primary')">
          {{ primaryLabel }}
        </button>
        <button v-if="secondaryLabel" type="button" class="play-btn play-btn-secondary" @click="emit('secondary')">
          {{ secondaryLabel }}
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.reward-celebration-overlay {
  --reward-main: #64748b;
  --reward-soft: rgba(100, 116, 139, 0.18);
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 20px;
}

.reward-celebration-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(15, 23, 42, 0.46);
  backdrop-filter: blur(5px);
}

.tone-emerald { --reward-main: #059669; --reward-soft: rgba(5, 150, 105, 0.18); }
.tone-sky { --reward-main: #0284c7; --reward-soft: rgba(2, 132, 199, 0.18); }
.tone-indigo { --reward-main: #4f46e5; --reward-soft: rgba(79, 70, 229, 0.18); }
.tone-amber { --reward-main: #b7791f; --reward-soft: rgba(183, 121, 31, 0.2); }
.tone-gold { --reward-main: #c47d36; --reward-soft: rgba(196, 125, 54, 0.22); }

.reward-celebration-card {
  position: relative;
  width: min(420px, 100%);
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--reward-main) 38%, var(--line));
  border-radius: 8px;
  background: var(--card);
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.24);
  padding: 28px;
  text-align: center;
}

.reward-celebration-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 50% 0%, var(--reward-soft), transparent 58%);
  pointer-events: none;
}

.reward-close {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 1;
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid var(--line);
  border-radius: 999px;
  color: var(--muted);
  background: var(--card);
}

.reward-confetti {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
}

.reward-confetti span {
  position: absolute;
  top: -18px;
  width: 8px;
  height: 14px;
  border-radius: 2px;
  background: var(--reward-main);
  opacity: 0.9;
  animation: reward-confetti-fall 1.35s ease-out forwards;
}

.reward-confetti span:nth-child(3n) {
  background: #f59e0b;
}

.reward-confetti span:nth-child(4n) {
  background: #10b981;
}

.reward-box-stage {
  position: relative;
  width: 116px;
  height: 96px;
  margin: 0 auto 12px;
}

.reward-beam {
  position: absolute;
  inset: 8px 18px 18px;
  background: linear-gradient(180deg, var(--reward-soft), transparent);
  clip-path: polygon(50% 0, 100% 100%, 0 100%);
  animation: reward-beam-rise 0.72s ease-out both;
}

.reward-box {
  position: absolute;
  left: 22px;
  right: 22px;
  bottom: 8px;
  height: 54px;
}

.reward-box-lid,
.reward-box-body {
  position: absolute;
  left: 0;
  width: 100%;
  border: 2px solid color-mix(in srgb, var(--reward-main) 72%, #ffffff);
  background: color-mix(in srgb, var(--reward-main) 16%, var(--card));
}

.reward-box-lid {
  top: 0;
  height: 18px;
  transform-origin: 12px 20px;
  animation: reward-lid-pop 0.55s ease-out both;
}

.reward-box-body {
  bottom: 0;
  height: 38px;
}

.reward-vip-badge {
  position: relative;
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  margin-bottom: 8px;
  border-radius: 999px;
  padding: 3px 10px;
  color: #fff;
  background: var(--reward-main);
  font-size: 12px;
  font-weight: 800;
}

.reward-title {
  position: relative;
  margin: 0 0 8px;
  color: var(--text);
  font-size: 15px;
  font-weight: 700;
}

.reward-amount {
  position: relative;
  display: block;
  color: var(--reward-main);
  font-size: clamp(32px, 7vw, 46px);
  line-height: 1;
}

.variant-jackpot .reward-amount {
  letter-spacing: 0;
}

.reward-subtitle,
.reward-detail-list {
  position: relative;
}

.reward-subtitle {
  margin: 12px 0 0;
  color: var(--muted);
}

.reward-detail-list {
  display: grid;
  gap: 6px;
  margin: 16px 0 0;
  padding: 0;
  list-style: none;
  color: var(--text);
  font-size: 13px;
}

.reward-actions {
  position: relative;
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-top: 20px;
  flex-wrap: wrap;
}

@keyframes reward-confetti-fall {
  from {
    transform: translate3d(0, 0, 0) rotate(0deg);
  }
  to {
    transform: translate3d(var(--reward-drift), 72vh, 0) rotate(calc(var(--reward-spin) * 420deg));
  }
}

@keyframes reward-lid-pop {
  from {
    transform: translateY(8px) rotate(0deg);
  }
  to {
    transform: translate(-12px, -30px) rotate(-18deg);
  }
}

@keyframes reward-beam-rise {
  from {
    opacity: 0;
    transform: translateY(18px) scaleY(0.5);
  }
  to {
    opacity: 1;
    transform: translateY(0) scaleY(1);
  }
}

@media (prefers-reduced-motion: reduce) {
  .reward-confetti,
  .reward-beam {
    display: none;
  }

  .reward-box-lid {
    animation: none;
    transform: translate(-10px, -18px) rotate(-12deg);
  }
}
</style>
