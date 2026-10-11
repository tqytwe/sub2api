<template>
  <section class="space-y-3" :aria-label="t('requestLedger.routing.title')" :aria-busy="loading" data-testid="ledger-section-routing">
    <h3 class="text-base font-semibold">{{ t('requestLedger.routing.title') }}</h3>
    <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.coverageNotice') }}</p>
    <p v-if="loading" role="status" class="text-sm">{{ t('common.loading') }}</p>
    <div v-if="error" role="alert" class="space-y-2 text-sm text-red-600 dark:text-red-400">
      <p>{{ t('requestLedger.routing.error') }}</p>
      <button type="button" class="btn btn-secondary" data-action="retry" :disabled="loading" @click="read">{{ t('requestLedger.retry') }}</button>
    </div>
    <template v-if="evidence">
      <p role="status" :data-routing-status="evidence.status" class="text-sm" :class="['incomplete', 'truncated', 'pending'].includes(evidence.status) ? 'text-amber-700 dark:text-amber-400' : 'text-gray-700 dark:text-gray-300'">{{ t(`requestLedger.routing.states.${evidence.status}`) }}</p>
      <p v-if="evidence.coverage_version && evidence.coverage_version !== routingCoverageV1" class="text-sm text-amber-700 dark:text-amber-400">{{ t('requestLedger.routing.unknownVersion') }}</p>
      <dl class="grid gap-3 text-sm sm:grid-cols-2">
        <div><dt class="text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.version') }}</dt><dd class="break-all font-mono">{{ evidence.coverage_version || t('requestLedger.routing.noVersion') }}</dd></div>
        <div><dt class="text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.scope') }}</dt><dd>{{ t(evidence.source_population_complete ? 'requestLedger.routing.sourceComplete' : 'requestLedger.routing.prefiltered') }}</dd></div>
        <div><dt class="text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.selections') }}</dt><dd class="tabular-nums">{{ number(evidence.selection_count) }}</dd></div>
        <div><dt class="text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.eventCounts') }}</dt><dd class="tabular-nums">{{ number(evidence.event_count) }} / {{ number(evidence.events.length) }}</dd></div>
        <div><dt class="text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.vetoCount') }}</dt><dd class="tabular-nums">{{ number(evidence.veto_count) }}</dd></div>
        <div><dt class="text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.omitted') }}</dt><dd data-testid="routing-omitted" class="tabular-nums">{{ number(evidence.omitted_count) }}</dd></div>
        <div><dt class="text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.incomplete') }}</dt><dd>{{ t(evidence.incomplete ? 'requestLedger.yes' : 'requestLedger.routing.notFlagged') }}</dd></div>
        <div data-testid="routing-first-veto"><dt class="text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.firstVeto') }}</dt><dd v-if="firstVeto" class="break-words">{{ t('requestLedger.routing.selection', { number: firstVeto.selection_no }) }} · {{ label('stages', firstVeto.stage) }} · {{ account(firstVeto) }} · {{ label('reasons', firstVeto.reason) }}</dd><dd v-else>{{ t('requestLedger.routing.noRecordedVeto') }}</dd></div>
      </dl>
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.interpretation') }}</p>
      <p v-if="!evidence.events.length" class="text-sm text-gray-500 dark:text-gray-400">{{ t('requestLedger.routing.empty') }}</p>
      <ol v-else class="space-y-3" :start="pageIndex * pageSize + 1" :aria-label="t('requestLedger.routing.events')">
        <li v-for="(event, index) in visibleEvents" :key="pageIndex * pageSize + index" class="space-y-2 border-t border-gray-200 pt-3 text-sm dark:border-dark-700">
          <p class="flex flex-wrap gap-2"><span class="font-semibold">{{ t('requestLedger.routing.selection', { number: event.selection_no }) }}</span><span>{{ label('stages', event.stage) }}</span><span>{{ account(event) }}</span><span v-if="isRoutingVeto(evidence.coverage_version, event)" class="text-amber-700 dark:text-amber-400">{{ t('requestLedger.routing.veto') }}</span></p>
          <p class="break-words">{{ label('reasons', event.reason) }} <span v-if="event.reason === 'prefiltered_schedulable_pool'" class="tabular-nums">· {{ t('requestLedger.routing.poolCount', { count: event.value }) }}</span></p>
          <p class="break-all font-mono text-xs text-gray-500 dark:text-gray-400">{{ event.reason }}</p>
          <time :datetime="event.at" class="text-xs tabular-nums text-gray-500 dark:text-gray-400">{{ new Date(event.at).toLocaleString(locale) }}</time>
        </li>
      </ol>
    </template>
    <nav class="flex flex-wrap items-center gap-2 border-t border-gray-200 pt-3 text-sm dark:border-dark-700" :aria-label="`${t('requestLedger.routing.title')} · ${t('common.pagination')}`">
      <button type="button" class="btn btn-secondary" data-action="previous" :disabled="loading || pageIndex === 0" @click="pageIndex--">{{ t('pagination.previous') }}</button>
      <span v-if="evidence" role="status" data-testid="routing-page" class="tabular-nums">{{ t('requestLedger.routing.page', { start: evidence.events.length ? pageIndex * pageSize + 1 : 0, end: Math.min((pageIndex + 1) * pageSize, evidence.events.length), count: evidence.events.length }) }}</span>
      <button type="button" class="btn btn-secondary" data-action="next" :disabled="loading || !evidence || (pageIndex + 1) * pageSize >= evidence.events.length" @click="pageIndex++">{{ t('pagination.next') }}</button>
      <button type="button" class="btn btn-secondary" data-action="refresh" :disabled="loading" @click="read">{{ t('requestLedger.refreshSection') }}</button>
    </nav>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { isRoutingVeto, routingCoverageV1, type RoutingEvent, type RoutingEvidence } from '@/api/requestRouting'

const props = defineProps<{ scope?: { signal: AbortSignal; isCurrent: () => boolean }; loadEvidence: (signal: AbortSignal) => Promise<RoutingEvidence> }>()
const { t, te, locale } = useI18n()
const evidence = shallowRef<RoutingEvidence | null>(null)
const loading = ref(false), error = ref(false), pageIndex = ref(0), pageSize = 20
const visibleEvents = computed(() => evidence.value?.events.slice(pageIndex.value * pageSize, (pageIndex.value + 1) * pageSize) || [])
const firstVeto = computed(() => evidence.value?.events.find(event => isRoutingVeto(evidence.value!.coverage_version, event)))
const number = (value: number) => value.toLocaleString(locale.value)
const account = (event: RoutingEvent) => event.account_id ? t('requestLedger.routing.account', { id: event.account_id }) : t('requestLedger.routing.noAccount')
const label = (group: string, value: string) => te(`requestLedger.routing.${group}.${value}`) ? t(`requestLedger.routing.${group}.${value}`) : value
let controller: AbortController | undefined, sequence = 0, disposed = false
async function read() {
  const scope = props.scope
  if (disposed || !scope || scope.signal.aborted || !scope.isCurrent()) return
  controller?.abort(); const current = new AbortController(); controller = current
  const version = ++sequence
  const canPublish = () => !disposed && version === sequence && !current.signal.aborted && scope === props.scope && !scope.signal.aborted && scope.isCurrent()
  // A refresh is a new snapshot, never a mixture of old events and a new summary.
  evidence.value = null; pageIndex.value = 0; loading.value = true; error.value = false
  try {
    const result = await props.loadEvidence(current.signal)
    if (canPublish()) evidence.value = result
  } catch { if (canPublish()) error.value = true }
  finally { if (canPublish()) loading.value = false }
}
function invalidate() { sequence++; controller?.abort(); evidence.value = null; pageIndex.value = 0; loading.value = false; error.value = false }
watch(() => props.scope, (scope, previous) => {
  previous?.signal.removeEventListener('abort', invalidate)
  invalidate(); scope?.signal.addEventListener('abort', invalidate, { once: true })
  if (scope && !scope.signal.aborted) void read()
}, { immediate: true, flush: 'sync' })
onBeforeUnmount(() => { disposed = true; props.scope?.signal.removeEventListener('abort', invalidate); invalidate() })
</script>
