<template>
  <div class="ctv">
    <svg class="ctv-ps5-back" viewBox="0 0 420 260" aria-hidden="true">
      <g fill="#fff" stroke="#0a0a0a" stroke-width="1.5">
        <path d="M58 86c25-30 63-37 94-22l44 21c20 10 43 15 66 15h80c18 0 32 12 32 30 0 23-19 41-42 41h-83c-14 0-28 4-40 11l-37 23c-34 21-79 16-108-13l-32-32c-18-18-19-46-4-64z" fill="#f6f4ef"/>
        <circle cx="112" cy="108" r="20" class="psk stick-hit" fill="#fff"/>
        <circle cx="304" cy="136" r="20" class="psk stick-hit" fill="#fff"/>
        <rect x="160" y="92" width="42" height="42" rx="10" class="psk psk-pad" fill="#f1efe8"/>
        <path d="M180 102v22M169 113h22" class="psk psk-sym" stroke-linecap="round"/>
        <rect x="218" y="88" width="42" height="42" rx="10" class="psk psk-solo" fill="#fff"/>
        <path d="M229 108h20" class="psk psk-sym" stroke-linecap="round"/>
      </g>
    </svg>

    <div class="ctv-body">
      <svg class="ctv-antenna" viewBox="0 0 220 48" aria-hidden="true">
        <path d="M110 44V18" />
        <path d="M76 18 110 4 144 18" />
        <circle cx="110" cy="18" r="5" class="ant-cap" fill="#fff" />
      </svg>

      <div class="ctv-inner">
        <div class="ctv-screen">
          <svg class="ctv-show" viewBox="0 0 360 220" aria-hidden="true">
            <g v-if="activeKey === 'ch1'" class="bb">
              <rect width="360" height="220" fill="#111" />
              <g transform="translate(115 40)">
                <path d="M35 18 78 0l16 20-43 18z" fill="#ffb34d" class="bb-lid" />
                <path d="M44 20 88 4" stroke="#0a0a0a" stroke-width="2" class="draw d1" />
                <circle cx="68" cy="72" r="34" fill="#fff1cc" stroke="#0a0a0a" stroke-width="2" />
                <path d="M52 70h32" stroke="#0a0a0a" stroke-width="3" stroke-linecap="round" />
                <path d="M58 58l-8-7M78 58l8-7" stroke="#0a0a0a" stroke-width="3" stroke-linecap="round" />
                <circle cx="44" cy="116" r="12" fill="#fff" opacity=".2" class="bb-spark" />
                <circle cx="98" cy="122" r="8" fill="#fff" opacity=".22" class="bb-spark" />
                <path d="M16 118c18-8 40-10 52-10" stroke="#f4d27a" stroke-width="3" fill="none" class="bb-cursor" />
                <circle cx="68" cy="72" r="46" fill="none" stroke="#fff7" stroke-width="1" class="bb-star" />
              </g>
            </g>

            <g v-else-if="activeKey === 'ch2'">
              <rect width="360" height="220" fill="#161617" />
              <g transform="translate(60 28)">
                <rect x="18" y="0" width="88" height="94" rx="14" fill="#f7f7f2" stroke="#0a0a0a" stroke-width="2" />
                <rect x="128" y="0" width="88" height="94" rx="14" fill="#f7f7f2" stroke="#0a0a0a" stroke-width="2" />
                <rect x="238" y="0" width="88" height="94" rx="14" fill="#f7f7f2" stroke="#0a0a0a" stroke-width="2" />
                <path d="M32 28h60M32 48h60M32 68h28" stroke="#0a0a0a" stroke-width="3" class="draw k1" />
                <path d="M142 28h60M142 48h60M142 68h28" stroke="#0a0a0a" stroke-width="3" class="draw k2" />
                <path d="M252 28h60M252 48h60M252 68h28" stroke="#0a0a0a" stroke-width="3" class="draw k3" />
                <path d="M44 108c24 18 35 25 56 25M154 108c24 18 35 25 56 25M264 108c24 18 35 25 56 25" stroke="#8d8d84" stroke-width="2" stroke-dasharray="6 7" class="draw k4" />
              </g>
            </g>

            <g v-else-if="activeKey === 'ch3'">
              <rect width="360" height="220" fill="#101114" />
              <g transform="translate(50 34)">
                <circle cx="130" cy="70" r="30" fill="#fff" stroke="#0a0a0a" stroke-width="2" />
                <circle cx="54" cy="118" r="18" fill="#fff" stroke="#0a0a0a" stroke-width="2" />
                <circle cx="206" cy="118" r="18" fill="#fff" stroke="#0a0a0a" stroke-width="2" />
                <circle cx="130" cy="152" r="18" fill="#fff" stroke="#0a0a0a" stroke-width="2" />
                <path d="M76 114h108M130 100v34M76 114 112 84M184 114 148 84M54 118 120 152M206 118 140 152" stroke="#7b7d86" stroke-width="2" class="rb-l1" />
                <path d="M130 70h54M130 70H76" stroke="#7b7d86" stroke-width="2" class="rb-l2" />
                <path d="M54 118h34M206 118h-34" stroke="#7b7d86" stroke-width="2" class="rb-l3" />
                <g fill="#0a0a0a">
                  <circle cx="130" cy="70" r="7" />
                  <circle cx="54" cy="118" r="5" />
                  <circle cx="206" cy="118" r="5" />
                  <circle cx="130" cy="152" r="5" />
                </g>
              </g>
            </g>

            <g v-else>
              <rect width="360" height="220" fill="#12110f" />
              <g transform="translate(58 30)">
                <path d="M40 120c0-40 28-62 64-62s64 22 64 62" stroke="#f0d89a" stroke-width="3" fill="none" class="tfs1" />
                <path d="M24 118h168" stroke="#0a0a0a" stroke-width="2" class="tfs2" />
                <path d="M24 146h168" stroke="#0a0a0a" stroke-width="2" class="tfs3" />
                <circle cx="66" cy="118" r="16" fill="#fff1ca" stroke="#0a0a0a" stroke-width="2" class="tf-coin c1" />
                <circle cx="110" cy="118" r="16" fill="#fff1ca" stroke="#0a0a0a" stroke-width="2" class="tf-coin c2" />
                <circle cx="154" cy="118" r="16" fill="#fff1ca" stroke="#0a0a0a" stroke-width="2" class="tf-coin c3" />
                <path d="M50 92l16-16 14 14 18-20 14 16 16-14" stroke="#0a0a0a" stroke-width="2" fill="none" class="tf-crown" />
                <circle cx="130" cy="68" r="40" fill="none" stroke="#fff3" stroke-width="1" class="tf-pulse" />
              </g>
            </g>
          </svg>
          <div class="ctv-scanlines" />
          <div class="ctv-beam" />
          <div class="ctv-vignette" />
          <div class="ctv-noise" />
          <div class="ctv-ch">{{ currentName }}</div>
          <div class="ctv-hint">{{ currentHint }}</div>
        </div>

        <aside class="ctv-panel">
          <div class="ctv-panel-tag">CHANNEL</div>
          <div class="ctv-list">
            <button v-for="(ch, idx) in channels" :key="ch.key" class="ctv-pick" :class="{ active: activeIndex === idx }" @click="pick(idx)">
              <span class="ctv-pick-no">0{{ idx + 1 }}</span>
              <span class="ctv-pick-name">{{ ch.name }}</span>
            </button>
          </div>
          <router-link :to="activeDestination" class="ctv-enter">
            {{ enterLabel }}
            <span aria-hidden="true">→</span>
          </router-link>
          <button class="ctv-knob" :style="knobStyle" @click="nextChannel" :aria-label="nextLabel">
            <span class="ctv-knob-disc"><span class="ctv-knob-pointer" /></span>
          </button>
          <div class="ctv-next">{{ nextLabel }}</div>
          <div class="ctv-grill"><i /><i /><i /></div>
        </aside>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { CHANNEL_DESTINATIONS } from '@/content/play-features'
import playAPI from '@/api/play'

const { t } = useI18n()
const authStore = useAuthStore()
const activeIndex = ref(0)
const hubPending = ref(0)

const activeKey = computed(() => `ch${activeIndex.value + 1}`)
const currentName = computed(() => t(`home.jisudeng.channels.${activeKey.value}.name`))
const currentHint = computed(() => {
  const base = t(`home.jisudeng.channels.${activeKey.value}.hint`)
  if (authStore.isAuthenticated && hubPending.value > 0) {
    return `${base} · ${t('playHub.channelPending', { count: hubPending.value })}`
  }
  return base
})
const nextLabel = computed(() => t('home.jisudeng.channels.next'))
const enterLabel = computed(() => t('home.jisudeng.channels.enter'))
const knobStyle = computed(() => ({ transform: `rotate(${activeIndex.value * 90}deg)` }))

const channels = [1, 2, 3, 4].map((n) => ({ key: `ch${n}`, name: t(`home.jisudeng.channels.ch${n}.name`) }))

const activeDestination = computed(() => {
  const dest = CHANNEL_DESTINATIONS[activeIndex.value]
  return dest?.route ?? '/home'
})

async function loadHubPending() {
  if (!authStore.isAuthenticated) return
  try {
    const hub = await playAPI.getPlayHub()
    hubPending.value = hub.pending_actions
  } catch {
    hubPending.value = 0
  }
}

function pick(idx: number) { activeIndex.value = idx }
function nextChannel() { activeIndex.value = (activeIndex.value + 1) % channels.length }

onMounted(loadHubPending)
</script>
