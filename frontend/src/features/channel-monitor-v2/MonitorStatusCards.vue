<template>
  <div data-testid="monitor-status-cards">
    <div v-if="loading && !items.length" class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
      <div v-for="i in 6" :key="i" class="h-60 rounded-lg bg-gray-100 dark:bg-dark-800" />
    </div>
    <div v-else-if="!items.length" class="card rounded-lg py-12 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('channelMonitorV2.cards.empty') }}
    </div>
    <div v-else class="space-y-6">
      <section v-for="platform in platforms" :key="platform.name" class="space-y-3" :data-platform="platform.name">
        <h2 class="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-200">
          <span class="h-2.5 w-2.5 rounded-full bg-primary-500" aria-hidden="true" />
          {{ platformLabel(platform.name, locale) }}
          <span class="text-xs font-normal text-gray-400">{{ platform.items.length }}</span>
        </h2>
        <div class="grid grid-cols-1 items-start gap-3 md:grid-cols-2 xl:grid-cols-3">
          <article v-for="row in platform.items" :key="`${row.platform}-${row.group_id}`" class="min-w-0 rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800" data-testid="monitor-group-card">
            <header class="flex items-start gap-2.5">
              <span class="status-dot mt-1" :class="`health-${row.health.overall}`" aria-hidden="true" />
              <div class="min-w-0 flex-1">
                <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="row.group_name">{{ row.group_name || `#${row.group_id}` }}</h3>
                <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">{{ platformLabel(row.platform, locale) }}</p>
              </div>
              <span class="badge" :class="healthBadge[row.health.overall]">{{ t(`channelMonitorV2.cards.health.${row.health.overall}`) }}</span>
            </header>

            <dl class="mt-4 grid grid-cols-3 gap-2">
              <div v-for="metric in metrics(row)" :key="metric.key" class="min-w-0 rounded-md bg-gray-50 px-2.5 py-2 dark:bg-dark-900/40">
                <dt class="truncate text-[10px] text-gray-500 dark:text-gray-400">{{ t(`channelMonitorV2.cards.${metric.key}`) }}</dt>
                <dd class="mt-1 truncate font-mono text-sm font-semibold tabular-nums text-gray-900 dark:text-white" :title="metric.title">{{ metric.value }}</dd>
              </div>
            </dl>

            <div class="mt-4 border-t border-gray-100 pt-3 dark:border-dark-700">
              <div class="mb-2 flex items-center justify-between gap-2 text-[10px] text-gray-500 dark:text-gray-400">
                <span>{{ t(timelineMode === 'samples' ? 'channelMonitorV2.cards.samples' : 'channelMonitorV2.cards.timeline') }}</span>
                <span>{{ t('channelMonitorV2.cards.historyCount', { count: timeline(row).filter((bar) => bar.buckets.length).length }) }}</span>
              </div>
              <p v-if="timelineMode === 'samples' && !timeline(row).length" class="text-xs text-gray-400">{{ t('channelMonitorV2.cards.noSamples') }}</p>
              <div v-else class="flex h-5 items-end gap-1" role="img" :aria-label="t('channelMonitorV2.cards.trafficHistory')">
                <span v-for="(bar, index) in timeline(row)" :key="index" class="min-w-0 flex-1 rounded-sm" :class="healthBar[bar.state]" :title="barTitle(row, bar)" />
              </div>
            </div>

            <div class="mt-4 border-t border-gray-100 pt-3 dark:border-dark-700" data-testid="candy-history">
              <h4 class="text-xs font-medium text-gray-700 dark:text-gray-200">{{ t('channelMonitorV2.candy.title') }}</h4>
              <template v-if="candyHistories(row).length">
                <div v-for="history in candyHistories(row)" :key="`${history.model}:${history.reasoning_effort}`" class="mt-2">
                  <div class="flex items-start justify-between gap-2 text-[10px]">
                    <span class="min-w-0 break-words text-gray-500 dark:text-gray-400">{{ history.model }} · {{ history.reasoning_effort }} · {{ t('channelMonitorV2.candy.cadence', { minutes: history.interval_minutes }) }}</span>
                    <span class="shrink-0" :class="candyText[candyDisplayState(history, now)]">{{ t(`channelMonitorV2.candy.states.${candyDisplayState(history, now)}`) }}</span>
                  </div>
                  <div class="mt-1.5 flex h-3 gap-px" role="group" :aria-label="t('channelMonitorV2.candy.historyCount', { count: Math.min(history.results.length, candyHistoryLimit(history)), limit: candyHistoryLimit(history) })">
                    <template v-for="(result, index) in candyHistorySlots(history)" :key="index">
                      <span v-if="result" class="min-w-0 flex-1 rounded-sm" :class="candyColor[result.verdict]" :title="candyTitle(result, history.model, history.reasoning_effort)" />
                      <span v-else class="min-w-0 flex-1 rounded-sm bg-gray-200 dark:bg-dark-600" :title="t('channelMonitorV2.candy.notChecked')" />
                    </template>
                  </div>
                  <p v-if="!history.results.length" class="mt-1 text-[10px] text-gray-400">{{ t('channelMonitorV2.candy.waiting') }}</p>
                </div>
              </template>
              <p v-else class="mt-1 text-[10px] text-gray-500 dark:text-gray-400">{{ t('channelMonitorV2.candy.disabledHint') }}</p>
            </div>

            <div v-if="showcaseGroupIds.has(row.group_id || 0)" class="mt-3 flex justify-end">
              <button
                type="button"
                class="inline-flex min-h-8 items-center gap-1.5 rounded-md border border-primary-200 px-2.5 text-xs font-medium text-primary-700 hover:bg-primary-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 dark:border-primary-800 dark:text-primary-300 dark:hover:bg-primary-900/20"
                :aria-label="t('channelMonitorV2.pelican.openHistory', { group: row.group_name || row.group_id })"
                :data-testid="`pelican-history-open-${row.group_id}`"
                @click="selectedShowcaseGroup = { id: row.group_id || 0, name: row.group_name || `#${row.group_id}` }"
              >
                <Icon name="play" size="xs" />{{ t('channelMonitorV2.pelican.open') }}
              </button>
            </div>
          </article>
        </div>
      </section>
    </div>
    <PelicanGroupHistoryDialog
      :show="selectedShowcaseGroup !== null"
      :group-id="selectedShowcaseGroup?.id || 0"
      :group-name="selectedShowcaseGroup?.name || ''"
      @close="selectedShowcaseGroup = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorCandyResult, MonitorCoverage, MonitorMatrixRow } from '@/api/channelMonitorV2'
import type { PelicanShowcaseGroup } from '@/api/pelicanShowcase'
import Icon from '@/components/icons/Icon.vue'
import PelicanGroupHistoryDialog from '@/components/user/pelican/PelicanGroupHistoryDialog.vue'
import { platformLabel } from '@/utils/platformColors'
import { formatMonitorMs, formatMonitorPercent } from './monitorFormat'
import { candyDisplayState, candyHistoryLimit, candyHistorySlots, hasMonitorSamples, monitorCandyHistories, monitorCardTimeline, monitorCardTimelineMode } from './monitorCards'

const props = defineProps<{
  items: MonitorMatrixRow[]
  coverage?: MonitorCoverage
  loading: boolean
  now: number
  timelineMode?: string
  platformOrder?: string[]
  groupOrder?: number[]
  admin?: boolean
  showcaseGroups?: PelicanShowcaseGroup[]
}>()
const { t, locale } = useI18n()
const timelineMode = computed(() => monitorCardTimelineMode(props.timelineMode))
const candyHistories = monitorCandyHistories
const showcaseGroupIds = computed(() => new Set((props.showcaseGroups || []).map((group) => group.id)))
const selectedShowcaseGroup = ref<{ id: number; name: string } | null>(null)
const healthBadge = { healthy: 'badge-success', warning: 'badge-warning', critical: 'badge-danger', unknown: 'badge-gray' }
const healthBar = { healthy: 'bg-emerald-500', warning: 'bg-amber-400', critical: 'bg-red-500', unknown: 'bg-gray-200 dark:bg-dark-600' }
const candyColor = { correct: 'bg-emerald-500', incorrect: 'bg-red-500', error: 'bg-amber-500' }
const candyText = { correct: 'text-emerald-700 dark:text-emerald-300', incorrect: 'text-red-700 dark:text-red-300', error: 'text-amber-700 dark:text-amber-300', unknown: 'text-gray-500', stale: 'text-amber-700 dark:text-amber-300' }
const platforms = computed(() => {
  const order = new Map((props.platformOrder || []).map((platform, index) => [platform, index]))
  const grouped = new Map<string, MonitorMatrixRow[]>()
  for (const row of props.items) {
    const items = grouped.get(row.platform) || []
    items.push(row)
    grouped.set(row.platform, items)
  }
  const groupOrder = new Map((props.groupOrder || []).map((id, index) => [id, index]))
  return [...grouped.entries()]
    .map(([name, items]) => ({ name, items: items.sort((a, b) => (groupOrder.get(a.group_id || 0) ?? Number.MAX_SAFE_INTEGER) - (groupOrder.get(b.group_id || 0) ?? Number.MAX_SAFE_INTEGER) || (a.group_name || '').localeCompare(b.group_name || '')) }))
    .sort((a, b) => (order.get(a.name) ?? Number.MAX_SAFE_INTEGER) - (order.get(b.name) ?? Number.MAX_SAFE_INTEGER) || a.name.localeCompare(b.name))
})
function metrics(row: MonitorMatrixRow) {
  const hasRequests = hasMonitorSamples(row.metrics)
  return [
    { key: 'availability', value: hasRequests ? formatMonitorPercent(1 - row.metrics.error_rate) : '-', title: hasRequests ? t('channelMonitorV2.metrics.errorRateValue', { value: formatMonitorPercent(row.metrics.error_rate) }) : '' },
    { key: 'cache', value: row.metrics.cache_rate_denominator > 0 ? formatMonitorPercent(row.metrics.cache_rate) : '-' },
    { key: 'latency', value: formatMonitorMs(row.metrics.ttft.p50_ms), title: t('channelMonitorV2.metrics.ttftP50') },
  ]
}
function timeline(row: MonitorMatrixRow) { return monitorCardTimeline(row, props.coverage, timelineMode.value) }
function barTitle(row: MonitorMatrixRow, bar: ReturnType<typeof timeline>[number]) {
  const time = new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'short' })
  return `${row.group_name || row.group_id} · ${time.format(bar.start)} · ${t(`channelMonitorV2.cards.health.${bar.state}`)} · ${bar.buckets.length} ${t('channelMonitorV2.cards.samples').toLowerCase()}`
}
function candyTitle(result: MonitorCandyResult, model: string, effort: string) {
  const time = new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(result.checked_at))
  return [model, effort, time, t(`channelMonitorV2.candy.states.${result.verdict}`), formatMonitorMs(result.latency_ms), props.admin ? result.answer_preview : undefined, props.admin ? result.reason : undefined].filter(Boolean).join(' · ')
}
</script>

<style scoped>
.status-dot { display: inline-block; height: .625rem; width: .625rem; flex: none; border-radius: 9999px; }
.health-healthy { background: var(--status-success); }
.health-warning { background: var(--status-warning); }
.health-critical { background: var(--status-danger); }
.health-unknown { background: var(--status-neutral); }
</style>
