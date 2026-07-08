<template>
  <transition name="whycard">
    <div v-if="active !== null" class="whycard wc" :style="styleObj">
      <div class="wc-tag"><span>WHY</span><span>{{ activeLabel }}</span></div>
      <div class="wc-stage">
        <template v-if="active === 0">
          <div class="g2-stage">
            <svg class="g2-rails" viewBox="0 0 372 172" aria-hidden="true">
              <defs>
                <path id="lane1" d="M34 46 C90 34, 126 34, 188 86 S286 138, 340 52" />
                <path id="lane2" d="M34 84 C92 72, 132 74, 188 86 S286 120, 340 118" />
                <path id="lane3" d="M34 126 C92 118, 140 110, 188 86 S286 80, 340 28" />
              </defs>
              <path d="M34 46 C90 34, 126 34, 188 86 S286 138, 340 52" class="g2-rail" fill="none" />
              <path d="M34 84 C92 72, 132 74, 188 86 S286 120, 340 118" class="g2-rail" fill="none" />
              <path d="M34 126 C92 118, 140 110, 188 86 S286 80, 340 28" class="g2-rail" fill="none" />
            </svg>
            <div v-for="(m, i) in modelIcons" :key="m.name" class="g2-model" :style="{ top: `${28 + i * 40}px`, '--i': i } as any">
              <span class="g2-model-disc"><svg viewBox="0 0 24 24"><path :d="m.path" /></svg></span>
              <span class="g2-model-name">{{ m.name }}</span>
            </div>
            <div class="g2-core">
              <div class="g2-ring" />
              <div class="g2-ripple" />
              <div class="g2-ripple g2-ripple-2" />
              <div class="g2-nucleus">AI</div>
              <div class="g2-core-label">multi-model gateway</div>
            </div>
            <div class="g2-agent">
              <div class="g2-agent-disc">A</div>
              <div class="g2-model-name">Agent</div>
            </div>
          </div>
        </template>
        <template v-else-if="active === 1">
          <div class="rel-panel rel-bad">
            <div class="rel-head"><span class="rel-name">single route</span><span class="rel-badge rel-badge-err">risky</span></div>
            <div class="rel-lines"><span class="rel-line rel-line-dead" /><span class="rel-line rel-line-dead" /></div>
            <div class="rel-retry">fallback retry</div>
          </div>
          <div class="rel-panel rel-good">
            <div class="rel-head"><span class="rel-name">redundant route</span><span class="rel-badge rel-badge-ok">stable</span></div>
            <div class="rel-lines"><span class="rel-line rel-line-run" /><span class="rel-line rel-line-run" /></div>
            <div class="rel-done">99.97% availability</div>
          </div>
        </template>
        <template v-else-if="active === 2">
          <div class="prv-panel">
            <div class="prv-head"><span>privacy</span><span class="prv-id">local-only</span></div>
            <div class="prv-row"><span class="prv-k">logs</span><span class="prv-v">kept inside your stack</span></div>
            <div class="prv-row"><span class="prv-k">keys</span><span class="prv-v prv-secret"><i>••••••••••</i><b>masked end-to-end</b></span></div>
            <div class="prv-row"><span class="prv-k">policy</span><span class="prv-v">least privilege by default</span></div>
            <div class="prv-pills"><span class="prv-pill">no exfil</span><span class="prv-pill">audit trail</span></div>
          </div>
        </template>
        <template v-else-if="active === 3">
          <div class="ins-rail">
            <div class="ins-card ins-step1"><div class="ins-card-tag">STEP 01</div><div class="ins-type">open the gateway<span class="ins-caret" /></div></div>
            <div class="ins-arrow">→</div>
            <div class="ins-card ins-step2"><div class="ins-card-tag">STEP 02</div><div class="ins-key">paste key</div></div>
            <div class="ins-arrow">→</div>
            <div class="ins-card ins-step3"><div class="ins-card-tag">STEP 03</div><div class="ins-ok"><span class="ins-ok-dot" />ready</div></div>
          </div>
        </template>
        <template v-else-if="active === 4">
          <div class="bil-panel">
            <div class="bil-row" v-for="(row, i) in billingRows" :key="row.model" :style="{ '--i': i } as any"><span class="bil-model">{{ row.model }}</span><span class="bil-tk">{{ row.token }}</span><span class="bil-amt">{{ row.amount }}</span></div>
            <div class="bil-total" :style="{ '--i': billingRows.length } as any"><span>month total</span><span class="bil-amt-total">{{ billingTotal }}</span></div>
          </div>
        </template>
        <template v-else>
          <div class="wal-panel">
            <div class="wal-toast">wallet synced</div>
            <div class="wal-label">wallet balance</div>
            <div class="wal-roll"><span class="wal-vals"><b>12.48</b><b>18.36</b><b>24.92</b></span></div>
            <div class="wal-bar"><span class="wal-bar-fill" /></div>
            <div class="wal-pills"><span class="wal-pill">top up</span><span class="wal-pill">withdraw</span><span class="wal-pill">history</span></div>
          </div>
        </template>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { MODEL_ICONS } from './model-icons'

const props = defineProps<{ active: number | null; x: number; y: number }>()
const modelIcons = MODEL_ICONS.slice(0, 4)
const billingRows = [
  { model: 'Claude', token: '12.4k', amount: '$2.40' },
  { model: 'GPT', token: '8.1k', amount: '$1.12' },
  { model: 'Gemini', token: '3.2k', amount: '$0.46' }
]
const billingTotal = '$3.98'
const labels = ['gateway','reliability','privacy','instant','billing','wallet']
const activeLabel = computed(() => (props.active === null ? '' : labels[props.active] || 'panel'))
const styleObj = computed(() => ({ transform: `translate3d(${props.x}px, ${props.y}px, 0)` }))
</script>
