<template>
  <transition name="whycard">
    <div v-if="active !== null" class="whycard wc" :style="styleObj">
      <div class="wc-tag"><span>{{ t('home.jisudeng.features.hoverCard.tag') }}</span><span>{{ activeLabel }}</span></div>
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
              <div class="g2-core-label">{{ t('home.jisudeng.features.hoverCard.gateway.title') }}</div>
            </div>
            <div class="g2-agent">
              <div class="g2-agent-disc">A</div>
              <div class="g2-model-name">{{ t('home.jisudeng.features.hoverCard.gateway.agent') }}</div>
            </div>
          </div>
        </template>
        <template v-else-if="active === 1">
          <div class="rel-panel rel-bad">
            <div class="rel-head"><span class="rel-name">{{ t('home.jisudeng.features.hoverCard.reliability.singleRoute') }}</span><span class="rel-badge rel-badge-err">{{ t('home.jisudeng.features.hoverCard.reliability.risky') }}</span></div>
            <div class="rel-lines"><span class="rel-line rel-line-dead" /><span class="rel-line rel-line-dead" /></div>
            <div class="rel-retry">{{ t('home.jisudeng.features.hoverCard.reliability.fallbackRetry') }}</div>
          </div>
          <div class="rel-panel rel-good">
            <div class="rel-head"><span class="rel-name">{{ t('home.jisudeng.features.hoverCard.reliability.redundantRoute') }}</span><span class="rel-badge rel-badge-ok">{{ t('home.jisudeng.features.hoverCard.reliability.stable') }}</span></div>
            <div class="rel-lines"><span class="rel-line rel-line-run" /><span class="rel-line rel-line-run" /></div>
            <div class="rel-done">{{ t('home.jisudeng.features.hoverCard.reliability.availability') }}</div>
          </div>
        </template>
        <template v-else-if="active === 2">
          <div class="prv-panel">
            <div class="prv-head"><span>{{ t('home.jisudeng.features.hoverCard.privacy.title') }}</span><span class="prv-id">{{ t('home.jisudeng.features.hoverCard.privacy.localOnly') }}</span></div>
            <div class="prv-row"><span class="prv-k">{{ t('home.jisudeng.features.hoverCard.privacy.logs') }}</span><span class="prv-v">{{ t('home.jisudeng.features.hoverCard.privacy.logsValue') }}</span></div>
            <div class="prv-row"><span class="prv-k">{{ t('home.jisudeng.features.hoverCard.privacy.keys') }}</span><span class="prv-v prv-secret"><i>••••••••••</i><b>{{ t('home.jisudeng.features.hoverCard.privacy.masked') }}</b></span></div>
            <div class="prv-row"><span class="prv-k">{{ t('home.jisudeng.features.hoverCard.privacy.policy') }}</span><span class="prv-v">{{ t('home.jisudeng.features.hoverCard.privacy.policyValue') }}</span></div>
            <div class="prv-pills"><span class="prv-pill">{{ t('home.jisudeng.features.hoverCard.privacy.noExfiltration') }}</span><span class="prv-pill">{{ t('home.jisudeng.features.hoverCard.privacy.auditTrail') }}</span></div>
          </div>
        </template>
        <template v-else-if="active === 3">
          <div class="ins-rail">
            <div class="ins-card ins-step1"><div class="ins-card-tag">{{ t('home.jisudeng.features.hoverCard.instant.step', { number: '01' }) }}</div><div class="ins-type">{{ t('home.jisudeng.features.hoverCard.instant.openGateway') }}<span class="ins-caret" /></div></div>
            <div class="ins-arrow">→</div>
            <div class="ins-card ins-step2"><div class="ins-card-tag">{{ t('home.jisudeng.features.hoverCard.instant.step', { number: '02' }) }}</div><div class="ins-key">{{ t('home.jisudeng.features.hoverCard.instant.pasteKey') }}</div></div>
            <div class="ins-arrow">→</div>
            <div class="ins-card ins-step3"><div class="ins-card-tag">{{ t('home.jisudeng.features.hoverCard.instant.step', { number: '03' }) }}</div><div class="ins-ok"><span class="ins-ok-dot" />{{ t('home.jisudeng.features.hoverCard.instant.ready') }}</div></div>
          </div>
        </template>
        <template v-else-if="active === 4">
          <div class="bil-panel">
            <div class="bil-row" v-for="(row, i) in billingRows" :key="row.model" :style="{ '--i': i } as any"><span class="bil-model">{{ row.model }}</span><span class="bil-tk">{{ row.token }}</span><span class="bil-amt">{{ row.amount }}</span></div>
            <div class="bil-total" :style="{ '--i': billingRows.length } as any"><span>{{ t('home.jisudeng.features.hoverCard.billing.monthTotal') }}</span><span class="bil-amt-total">{{ billingTotal }}</span></div>
          </div>
        </template>
        <template v-else>
          <div class="wal-panel">
            <div class="wal-toast">{{ t('home.jisudeng.features.hoverCard.wallet.synced') }}</div>
            <div class="wal-label">{{ t('home.jisudeng.features.hoverCard.wallet.balance') }}</div>
            <div class="wal-roll"><span class="wal-vals"><b>12.48</b><b>18.36</b><b>24.92</b></span></div>
            <div class="wal-bar"><span class="wal-bar-fill" /></div>
            <div class="wal-pills"><span class="wal-pill">{{ t('home.jisudeng.features.hoverCard.wallet.topUp') }}</span><span class="wal-pill">{{ t('home.jisudeng.features.hoverCard.wallet.withdraw') }}</span><span class="wal-pill">{{ t('home.jisudeng.features.hoverCard.wallet.history') }}</span></div>
          </div>
        </template>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { MODEL_ICONS } from './model-icons'

const props = defineProps<{ active: number | null; x: number; y: number }>()
const { t } = useI18n()
const modelIcons = MODEL_ICONS.slice(0, 4)
const billingRows = [
  { model: 'Claude', token: '12.4k', amount: '$2.40' },
  { model: 'GPT', token: '8.1k', amount: '$1.12' },
  { model: 'Gemini', token: '3.2k', amount: '$0.46' }
]
const billingTotal = '$3.98'
const activeLabel = computed(() => {
  switch (props.active) {
    case 0:
      return t('home.jisudeng.features.hoverCard.labels.gateway')
    case 1:
      return t('home.jisudeng.features.hoverCard.labels.reliability')
    case 2:
      return t('home.jisudeng.features.hoverCard.labels.privacy')
    case 3:
      return t('home.jisudeng.features.hoverCard.labels.instant')
    case 4:
      return t('home.jisudeng.features.hoverCard.labels.billing')
    case 5:
      return t('home.jisudeng.features.hoverCard.labels.wallet')
    default:
      return props.active === null ? '' : t('home.jisudeng.features.hoverCard.labels.fallback')
  }
})
const styleObj = computed(() => ({ transform: `translate3d(${props.x}px, ${props.y}px, 0)` }))
</script>
