<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import PublicContentLayout from '@/components/layout/PublicContentLayout.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import { fetchPublicStatusSummary, type PublicStatusSummaryResponse } from '@/api/publicStatus'
import { formatHomeStatsTimestamp, formatHomeStatRequests, formatHomeStatUptime, formatPublicStatusLatency, freshnessFromSnapshot } from '@/utils/homeLiveStats'
import { localizedSiteName } from '@/utils/localizedPublicSettings'
import { useAppStore } from '@/stores'
import '@/styles/public-pages.css'

const { t, locale } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const summary = ref<PublicStatusSummaryResponse | null>(null)
const loading = ref(true)
const nowMs = ref(Date.now())
let freshnessTimer: ReturnType<typeof setInterval> | null = null

const isEnglish = computed(() => route.path === '/en/status' || route.path.startsWith('/en/status/'))
const siteName = computed(() => localizedSiteName(appStore.cachedPublicSettings?.site_name || appStore.siteName, locale.value))
const effectiveFreshness = computed(() => freshnessFromSnapshot(summary.value, nowMs.value))
const statusLabel = computed(() => {
  if (effectiveFreshness.value === 'unavailable') return t('publicStatus.unavailable')
  return effectiveFreshness.value === 'delayed' ? t('publicStatus.dataDelayed') : t('publicStatus.dataNormal')
})
const statusClass = computed(() => `is-${effectiveFreshness.value}`)
const siteLogo = computed(() => appStore.siteLogo || '')
const p50 = computed(() => formatPublicStatusLatency(summary.value?.ttft.p50_ms ?? null))
const p95 = computed(() => formatPublicStatusLatency(summary.value?.ttft.p95_ms ?? null))

function formatMeasurementWindow(windowStart: string | null | undefined, windowEnd: string | null | undefined): string {
  const start = formatHomeStatsTimestamp(windowStart ?? null, locale.value)
  const end = formatHomeStatsTimestamp(windowEnd ?? null, locale.value)
  if (!start || !end) return t('publicStatus.windowUnavailable')
  return t('publicStatus.window', { start, end })
}

const metricItems = computed(() => {
  const data = summary.value
  return [
    {
      key: 'requests',
      label: t('publicStatus.recordedRequests'),
      value: formatHomeStatRequests(data?.total_requests ?? null),
      unit: data ? '+' : '',
      sample: '',
    },
    {
      key: 'availability',
      label: t('publicStatus.availability'),
      value: formatHomeStatUptime(data?.availability.value_pct ?? null),
      unit: data?.availability.value_pct == null ? '' : '%',
      sample: data ? t('publicStatus.samples', { count: formatHomeStatRequests(data.availability.sample_count) }) : '',
      window: data ? formatMeasurementWindow(data.availability.window_start, data.availability.window_end) : '',
    },
    {
      key: 'ttft-p50',
      label: t('publicStatus.ttftP50'),
      value: p50.value.value,
      unit: p50.value.unit,
      sample: data ? t('publicStatus.samples', { count: formatHomeStatRequests(data.ttft.sample_count) }) : '',
      window: data ? formatMeasurementWindow(data.ttft.window_start, data.ttft.window_end) : '',
    },
    {
      key: 'ttft-p95',
      label: t('publicStatus.ttftP95'),
      value: p95.value.value,
      unit: p95.value.unit,
      sample: data ? t('publicStatus.samples', { count: formatHomeStatRequests(data.ttft.sample_count) }) : '',
      window: data ? formatMeasurementWindow(data.ttft.window_start, data.ttft.window_end) : '',
    },
  ]
})

const dataThrough = computed(() => formatHomeStatsTimestamp(summary.value?.data_through ?? null, locale.value))

async function load() {
  loading.value = true
  summary.value = await fetchPublicStatusSummary()
  nowMs.value = Date.now()
  loading.value = false
}

onMounted(() => {
  void load()
  // A cached response must not keep the page green after its business
  // watermark crosses the delayed or unavailable threshold.
  freshnessTimer = setInterval(() => {
    nowMs.value = Date.now()
  }, 60_000)
})

onBeforeUnmount(() => {
  if (freshnessTimer) clearInterval(freshnessTimer)
})
</script>

<template>
  <PublicContentLayout
    :site-name="siteName"
    :site-logo="siteLogo"
    :home-route="isEnglish ? '/en' : '/'"
    frame="reading"
  >
    <main class="public-status-main">
      <div class="public-status-heading">
        <div>
          <p class="public-status-eyebrow">{{ t('publicStatus.eyebrow') }}</p>
          <h1>{{ t('publicStatus.title') }}</h1>
          <p class="public-status-lede">{{ t('publicStatus.lede') }}</p>
        </div>
        <div class="public-status-state" :class="statusClass" role="status" aria-live="polite">
          <span class="public-status-dot" aria-hidden="true" />
          {{ statusLabel }}
        </div>
      </div>

      <section class="public-status-grid" :aria-busy="loading">
        <article v-for="item in metricItems" :key="item.key" class="public-status-metric" data-testid="status-metric">
          <p>{{ item.label }}</p>
          <strong>{{ item.value }}<small>{{ item.unit }}</small></strong>
          <span v-if="item.sample">{{ item.sample }}</span>
          <span v-if="item.window" class="public-status-window" data-testid="status-window">{{ item.window }}</span>
        </article>
      </section>

      <div class="public-status-meta">
        <span v-if="dataThrough">{{ t('publicStatus.through', { time: dataThrough }) }}</span>
        <span v-else>{{ t('publicStatus.noWatermark') }}</span>
        <button type="button" class="public-status-retry" :disabled="loading" @click="load">{{ t('publicStatus.retry') }}</button>
      </div>

      <p class="public-status-note">{{ t('publicStatus.note') }}</p>
      <SupportFloatingCard />
    </main>
  </PublicContentLayout>
</template>

<style scoped>
.public-status-main { color: var(--text-primary); }
.public-status-heading { display:flex; align-items:flex-end; justify-content:space-between; gap:24px; }
.public-status-eyebrow { margin:0 0 12px; color:var(--text-muted); font-size:12px; font-weight:650; letter-spacing:.12em; text-transform:uppercase; }
.public-status-heading h1 { margin:0; font-size:32px; line-height:40px; }
.public-status-lede { max-inline-size:58ch; margin:10px 0 0; color:var(--text-secondary); }
.public-status-state { display:inline-flex; align-items:center; gap:8px; flex:0 0 auto; padding:8px 12px; border:1px solid var(--status-success-border); border-radius:999px; color:var(--status-success-text); background:var(--status-success-surface); font-weight:650; }
.public-status-state.is-delayed { border-color:var(--status-warning-border); color:var(--status-warning-text); background:var(--status-warning-surface); }
.public-status-state.is-unavailable { border-color:var(--border-default); color:var(--text-muted); background:var(--surface-panel); }
.public-status-dot { inline-size:8px; block-size:8px; border-radius:50%; background:currentColor; }
.public-status-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:12px; margin-top:32px; }
.public-status-metric { min-block-size:150px; padding:20px; border:1px solid var(--border-default); border-radius:8px; background:var(--surface-panel); }
.public-status-metric p { min-block-size:42px; margin:0; color:var(--text-secondary); font-size:13px; }
.public-status-metric strong { display:block; margin-top:20px; font-size:30px; line-height:36px; font-variant-numeric:tabular-nums; }
.public-status-metric strong small { margin-inline-start:2px; font-size:16px; font-weight:600; }
.public-status-metric span { display:block; margin-top:8px; color:var(--text-muted); font-size:12px; }
.public-status-metric .public-status-window { margin-top:4px; overflow-wrap:anywhere; }
.public-status-meta { display:flex; align-items:center; justify-content:space-between; gap:16px; margin-top:16px; color:var(--text-muted); font-size:12px; }
.public-status-retry { min-block-size:40px; padding:0 14px; border:1px solid var(--border-default); border-radius:6px; background:var(--surface-panel); color:var(--text-primary); font:inherit; font-weight:600; cursor:pointer; }
.public-status-retry:disabled { cursor:not-allowed; opacity:.6; }
.public-status-retry:focus-visible { outline:2px solid var(--border-focus); outline-offset:2px; }
.public-status-note { margin:48px 0 0; padding-top:20px; border-top:1px solid var(--border-default); color:var(--text-muted); font-size:12px; }
/* design-governance-allow: page-shell-ownership - viewport media query only controls status content; PublicContentLayout owns the route shell. */
@media (max-width: 767px) {
  .public-status-main { padding: 0; }
  .public-status-heading { align-items:flex-start; flex-direction:column; }
  .public-status-heading h1 { font-size:28px; line-height:36px; }
  .public-status-grid { grid-template-columns:repeat(2,minmax(0,1fr)); margin-top:24px; }
  .public-status-metric { min-block-size:136px; padding:16px; }
  .public-status-metric strong { margin-top:12px; font-size:24px; }
}
/* design-governance-allow: page-shell-ownership - narrow breakpoint changes only the inner metric grid, not route width or scrolling. */
@media (max-width: 420px) { .public-status-grid { grid-template-columns:1fr; } }
</style>
