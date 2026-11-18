<template>
  <AppLayout>
    <div class="min-w-0 space-y-5">
      <header>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.priorityScheduling.title') }}</h1>
        <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.priorityScheduling.description') }}</p>
      </header>

      <p class="border border-primary-200 bg-primary-50 p-4 text-sm leading-6 text-primary-800 dark:border-primary-800 dark:bg-primary-950/30 dark:text-primary-200">{{ t('admin.priorityScheduling.scope') }}</p>
      <div v-if="error" role="alert" class="flex items-center justify-between gap-3 border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-900/20 dark:text-red-200">
        <span>{{ error }}</span>
        <button v-if="!config" class="btn btn-secondary" :disabled="loading" @click="load">{{ t('common.retry') }}</button>
      </div>
      <p v-if="notice" role="status" class="border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-900 dark:bg-green-900/20 dark:text-green-200">{{ notice }}</p>
      <p v-if="loading && !config" role="status" aria-live="polite">{{ t('admin.priorityScheduling.loading') }}</p>

      <form v-if="config" class="space-y-5" @submit.prevent="save">
        <fieldset class="min-w-0 space-y-5" :disabled="saving">
          <section class="border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
            <label class="block">
              <span class="field-title">{{ t('admin.priorityScheduling.penaltyMode') }}</span>
              <select v-model="config.latency_penalty_mode" class="input w-full" data-testid="latency-penalty-mode">
                <option value="deprioritize">{{ t('admin.priorityScheduling.modes.deprioritize') }}</option>
                <option value="stop">{{ t('admin.priorityScheduling.modes.stop') }}</option>
              </select>
              <small class="mt-2 block text-sm leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.priorityScheduling.modeHint') }}</small>
            </label>
            <label class="mt-4 block">
              <span class="field-title">{{ t('admin.priorityScheduling.cacheWeight') }}</span>
              <input v-model.number="config.cache_weight" class="input w-full" type="number" min="0" max="100" step="1" data-testid="cache-weight">
              <small class="mt-2 block text-sm leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.priorityScheduling.cacheWeightHint') }}</small>
            </label>
            <dl class="mt-4 grid gap-3 border-t border-gray-100 pt-4 text-sm dark:border-dark-700 sm:grid-cols-3">
              <div><dt class="text-gray-500">{{ t('admin.priorityScheduling.configStatus') }}</dt><dd class="mt-1 font-medium">{{ config.enabled ? t('admin.priorityScheduling.enabled') : t('admin.priorityScheduling.disabled') }}</dd></div>
              <div><dt class="text-gray-500">{{ t('admin.priorityScheduling.penaltyDuration') }}</dt><dd class="mt-1 font-medium">{{ config.latency_penalty_minutes ?? '—' }} {{ t('admin.priorityScheduling.minutes') }}</dd></div>
              <div><dt class="text-gray-500">{{ t('admin.priorityScheduling.measurement') }}</dt><dd class="mt-1 font-medium">{{ config.latency_measurement || 'legacy_forward' }} · P{{ config.latency_percentile ?? 90 }}</dd></div>
            </dl>
          </section>
          <button class="btn btn-primary" type="submit" :disabled="saving || loading">{{ t(saving ? 'common.saving' : 'common.save') }}</button>
        </fieldset>
      </form>

      <section class="border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <header class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-4 dark:border-dark-700">
          <div><h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.priorityScheduling.recent') }}</h2><p class="mt-1 text-sm text-gray-500">{{ t('admin.priorityScheduling.snapshotScope') }}</p></div>
          <button class="btn btn-secondary" :disabled="refreshing" @click="refresh"><!-- design-governance-allow: continuous-motion - user-triggered refresh feedback stops when the request settles and honors reduced-motion. --><Icon name="refresh" size="md" :class="refreshing ? 'animate-spin motion-reduce:animate-none' : ''" />{{ t('common.refresh') }}</button>
        </header>
        <p v-if="snapshotError" role="alert" class="m-4 border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-900/20 dark:text-red-200">{{ snapshotError }}</p>
        <p v-if="!snapshot && !refreshing" class="p-8 text-center text-sm text-gray-500">{{ t(snapshotError ? 'admin.priorityScheduling.snapshotFailed' : 'admin.priorityScheduling.empty') }}</p>
        <div v-if="snapshot" class="p-4">
          <p class="mb-3 text-xs text-gray-500">{{ new Date(snapshot.at).toLocaleString() }} · {{ t('admin.priorityScheduling.model') }}: {{ snapshot.model }} · {{ t('admin.priorityScheduling.endpoint') }}: {{ snapshot.inbound_endpoint || '—' }} · {{ t('admin.priorityScheduling.group') }}: {{ snapshot.group_id ?? '—' }}</p>
          <p v-if="snapshot.history_status && snapshot.history_status !== 'ready'" class="mb-3 text-sm text-amber-700 dark:text-amber-400">{{ t('admin.priorityScheduling.historyUnavailable', { status: snapshot.history_status }) }}</p>
          <DataTable :columns="columns" :data="snapshot.candidates" row-key="account_id">
            <template #cell-account="{ row }"><span class="font-medium text-gray-900 dark:text-white">{{ row.account_name }} <span class="font-normal text-gray-500">#{{ row.account_id }}</span></span></template>
            <template #cell-state="{ row }">
              <span v-if="row.latency_stopped" class="font-medium text-red-700 dark:text-red-300">{{ t('admin.priorityScheduling.stopped') }}</span>
              <span v-else>{{ t(`admin.priorityScheduling.tiers.${row.tier}`) }}</span>
              <small v-if="row.latency_penalty_until" class="block text-gray-500">{{ t('admin.priorityScheduling.penaltyUntil') }}: {{ new Date(row.latency_penalty_until).toLocaleString() }}</small>
            </template>
            <template #cell-latency="{ row }">{{ row.avg_ttft_ms != null ? `${t('admin.priorityScheduling.avgTTFT')} ${Math.round(row.avg_ttft_ms)} ms` : `P${row.latency_percentile ?? config?.latency_percentile ?? 90} · ${row.selected_ttft_ms == null ? '—' : `${Math.round(row.selected_ttft_ms)} ms`}` }} · {{ row.avg_ttft_ms != null ? (row.avg_ttft_samples ?? row.samples) : row.samples }} {{ t('admin.priorityScheduling.samples') }}</template>
            <template #cell-reasons="{ row }"><span class="text-xs text-gray-500"><template v-if="row.reasons.length"><span v-for="reason in row.reasons" :key="reason" class="mr-2">{{ t(`admin.priorityScheduling.reasons.${reason}`) }}</span></template><template v-else>{{ t('admin.priorityScheduling.noReasons') }}</template></span></template>
            <template #empty><p class="p-8 text-center text-sm text-gray-500">{{ t('admin.priorityScheduling.emptyCandidates') }}</p></template>
          </DataTable>
        </div>
      </section>

      <section class="border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
        <header class="flex flex-wrap items-center justify-between gap-3">
          <div><h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.priorityScheduling.analysis') }}</h2><p class="mt-1 text-sm text-gray-500">{{ t('admin.priorityScheduling.snapshotScope') }}</p></div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-secondary" type="button" :disabled="modelsLoading" data-testid="load-models" @click="loadModels"><Icon name="refresh" size="md" />{{ t('admin.priorityScheduling.loadModels') }}</button>
            <button class="btn btn-secondary" type="button" :disabled="recomputing" data-testid="recompute" @click="recompute"><Icon name="refresh" size="md" />{{ t('admin.priorityScheduling.recompute') }}</button>
          </div>
        </header>
        <div class="mt-4 flex flex-wrap items-end gap-3">
          <label class="min-w-[220px] flex-1"><span class="field-title">{{ t('admin.priorityScheduling.group') }}</span><select v-model="analysisGroup" class="input w-full" data-testid="analysis-group"><option :value="undefined">{{ t('admin.priorityScheduling.allGroups') }}</option><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option></select></label>
          <label class="min-w-[220px] flex-1"><span class="field-title">{{ t('admin.priorityScheduling.analysisModel') }}</span><select v-model="analysisModel" class="input w-full" data-testid="analysis-model"><option value="">{{ t('admin.priorityScheduling.chooseModel') }}</option><option v-for="model in models" :key="model" :value="model">{{ model }}</option></select></label>
          <button class="btn btn-primary" type="button" :disabled="!analysisModel || analyzing" data-testid="run-analysis" @click="runAnalysis">{{ t('admin.priorityScheduling.runAnalysis') }}</button>
        </div>
        <p v-if="groupsError" role="alert" class="mt-3 text-sm text-red-700 dark:text-red-300">{{ groupsError }}</p>
        <p v-if="analysisError" role="alert" class="mt-3 text-sm text-red-700 dark:text-red-300">{{ analysisError }}</p>
        <div v-if="recomputeResult" class="mt-4 border border-gray-200 p-4 dark:border-dark-700" data-testid="recompute-result">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.priorityScheduling.recomputeResult') }}</h3>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.priorityScheduling.recomputeScope') }}</p>
          <ul class="mt-3 divide-y divide-gray-100 dark:divide-dark-700">
            <li v-for="row in recomputeResult.diff" :key="row.account_id" class="flex flex-wrap justify-between gap-2 py-2 text-sm">
              <span>{{ row.account_name }} <span class="text-gray-500">#{{ row.account_id }}</span></span>
              <span class="tabular-nums">{{ row.old_score.toFixed(1) }} → {{ row.new_score.toFixed(1) }} · #{{ row.old_rank || '—' }} → #{{ row.new_rank || '—' }}</span>
            </li>
          </ul>
        </div>
        <div v-if="analysis" class="mt-4 overflow-x-auto">
          <DataTable :columns="analysisColumns" :data="analysis.snapshot.candidates" row-key="account_id">
            <template #cell-account="{ row }"><span class="font-medium">{{ row.account_name }}</span></template>
            <template #cell-score="{ row }">{{ row.score.toFixed(1) }}</template>
            <template #cell-performance="{ row }">{{ row.avg_tps == null ? '—' : `${row.avg_tps.toFixed(1)} TPS` }} · {{ row.avg_ttft_ms == null ? '—' : `${Math.round(row.avg_ttft_ms)} ms` }} · {{ row.cache_percent == null ? '—' : `${row.cache_percent.toFixed(1)}%` }}</template>
            <template #empty><p class="p-6 text-center text-sm text-gray-500">{{ t('admin.priorityScheduling.emptyCandidates') }}</p></template>
          </DataTable>
        </div>
        <p v-else-if="modelsLoaded && !analysisError" class="mt-4 text-sm text-gray-500">{{ models.length ? t('admin.priorityScheduling.analysisEmpty') : t('admin.priorityScheduling.noModels') }}</p>
        <div v-if="analysis?.latency" class="mt-5 overflow-x-auto">
          <h3 class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.priorityScheduling.latencySeries') }}</h3>
          <table class="w-full whitespace-nowrap text-left text-sm"><thead><tr class="border-b border-gray-200 dark:border-dark-700"><th class="py-2 pr-4 font-medium">{{ t('admin.priorityScheduling.account') }}</th><th class="py-2 pr-4 text-right font-medium">{{ t('admin.priorityScheduling.samples') }}</th><th class="py-2 pr-4 text-right font-medium">{{ t('admin.priorityScheduling.peakBucketP90') }}</th><th class="py-2 pr-4 text-right font-medium">{{ t('admin.priorityScheduling.slowSamples') }}</th><th class="py-2 text-right font-medium">{{ t('admin.priorityScheduling.errorSamples') }}</th></tr></thead><tbody><tr v-for="row in analysis.latency.accounts" :key="row.account_id" class="border-b border-gray-100 dark:border-dark-700"><td class="py-2 pr-4">{{ row.account_name }}</td><td class="py-2 pr-4 text-right tabular-nums">{{ row.samples }}</td><td class="py-2 pr-4 text-right tabular-nums">{{ row.samples ? `${Math.round(row.peak_bucket_p90_ttft_ms)} ms` : '—' }}</td><td class="py-2 pr-4 text-right tabular-nums">{{ row.slow_samples }}</td><td class="py-2 text-right tabular-nums">{{ row.error_samples ?? '—' }}</td></tr></tbody></table>
        </div>
      </section>

      <section class="border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
        <header class="flex flex-wrap items-center justify-between gap-3">
          <div><h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.priorityScheduling.stickyBindings') }}</h2><p class="mt-1 text-sm text-gray-500">{{ t('admin.priorityScheduling.stickyHint') }}</p></div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-secondary" type="button" :disabled="bindingsLoading || !analysisGroup" data-testid="load-bindings" @click="loadBindings"><Icon name="refresh" size="md" />{{ t('admin.priorityScheduling.loadBindings') }}</button>
            <button class="btn btn-secondary" type="button" :disabled="!bindings || bindings.total === 0 || clearing" data-testid="clear-bindings" @click="confirmingClear = true">{{ t('admin.priorityScheduling.clearBindings') }}</button>
          </div>
        </header>
        <p v-if="bindingsError" class="mt-3 text-sm text-red-700 dark:text-red-300" role="alert">{{ bindingsError }}</p>
        <p v-if="confirmingClear" class="mt-3 flex flex-wrap items-center gap-3 border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
          <span class="flex-1">{{ t('admin.priorityScheduling.confirmClear') }}</span>
          <button class="btn btn-primary" type="button" :disabled="clearing" data-testid="confirm-clear-bindings" @click="clearBindings">{{ t('common.confirm') }}</button>
          <button class="btn btn-secondary" type="button" :disabled="clearing" @click="confirmingClear = false">{{ t('common.cancel') }}</button>
        </p>
        <div v-if="bindings" class="mt-4 overflow-x-auto">
          <p class="mb-2 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.priorityScheduling.bindingTotal', { total: bindings.total }) }}</p>
          <table class="w-full text-left text-sm"><thead><tr class="border-b border-gray-200 dark:border-dark-700"><th class="py-2 pr-4 font-medium">{{ t('admin.priorityScheduling.account') }}</th><th class="py-2 text-right font-medium">{{ t('admin.priorityScheduling.bindingCount') }}</th></tr></thead><tbody><tr v-for="row in bindings.by_account" :key="row.account_id" class="border-b border-gray-100 dark:border-dark-700"><td class="py-2 pr-4">#{{ row.account_id }}</td><td class="py-2 text-right tabular-nums">{{ row.count }}</td></tr><tr v-if="bindings.by_account.length === 0"><td colspan="2" class="py-4 text-center text-gray-500">{{ t('admin.priorityScheduling.noBindings') }}</td></tr></tbody></table>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import type { AdminGroup } from '@/types'
import * as priorityAPI from '@/api/admin/priorityScheduling'
import * as groupsAPI from '@/api/admin/groups'

const { t } = useI18n()
const config = ref<priorityAPI.PrioritySchedulingConfig | null>(null)
const snapshot = ref<priorityAPI.PrioritySnapshot | null>(null)
const loading = ref(false)
const saving = ref(false)
const refreshing = ref(false)
const groups = ref<AdminGroup[]>([])
const analysisGroup = ref<number | undefined>(undefined)
const models = ref<string[]>([])
const analysisModel = ref('')
const analysis = ref<priorityAPI.PriorityAnalysis | null>(null)
const recomputeResult = ref<priorityAPI.PrioritySchedulingRecomputeResult | null>(null)
const modelsLoaded = ref(false)
const modelsLoading = ref(false)
const analyzing = ref(false)
const recomputing = ref(false)
const analysisError = ref('')
const groupsError = ref('')
const bindings = ref<priorityAPI.StickyBindings | null>(null)
const bindingsLoading = ref(false)
const clearing = ref(false)
const confirmingClear = ref(false)
const bindingsError = ref('')
const error = ref('')
const snapshotError = ref('')
const notice = ref('')
const columns = computed<Column[]>(() => [
  { key: 'account', label: t('admin.priorityScheduling.account') },
  { key: 'state', label: t('admin.priorityScheduling.state') },
  { key: 'latency', label: t('admin.priorityScheduling.latency') },
  { key: 'reasons', label: t('admin.priorityScheduling.reasonsTitle') },
])
const analysisColumns = computed<Column[]>(() => [
  { key: 'account', label: t('admin.priorityScheduling.account') },
  { key: 'score', label: 'Score' },
  { key: 'performance', label: t('admin.priorityScheduling.performance') },
])
let alive = true
let generation = 0
let modelGeneration = 0
let analysisGeneration = 0
let bindingGeneration = 0

async function load() {
  const current = ++generation
  loading.value = true
  error.value = ''
  snapshotError.value = ''
  try {
    const result = await priorityAPI.getPriorityConfig()
    if (!alive || current !== generation) return
    config.value = { ...result, latency_penalty_mode: result.latency_penalty_mode || 'deprioritize' }
    await refresh()
  } catch {
    if (alive && current === generation) error.value = t('admin.priorityScheduling.loadFailed')
  } finally {
    if (alive && current === generation) loading.value = false
  }
}

async function refresh() {
  refreshing.value = true
  snapshotError.value = ''
  try {
    snapshot.value = await priorityAPI.getPrioritySnapshot()
  } catch {
    snapshot.value = null
    snapshotError.value = t('admin.priorityScheduling.snapshotLoadFailed')
  } finally {
    refreshing.value = false
  }
}

async function save() {
  if (!config.value || saving.value) return
  const payload = { ...config.value }
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const saved = await priorityAPI.savePriorityConfig(payload)
    config.value = { ...saved, latency_penalty_mode: saved.latency_penalty_mode || 'deprioritize' }
    notice.value = t('admin.priorityScheduling.saved')
    await refresh()
  } catch {
    error.value = t('admin.priorityScheduling.saveFailed')
  } finally {
    saving.value = false
  }
}

async function loadModels() {
  const current = ++modelGeneration
  modelsLoading.value = true
  analysisError.value = ''
  try {
    const result = await priorityAPI.getPriorityModels(analysisGroup.value)
    if (!alive || current !== modelGeneration) return
    models.value = result
    if (!analysisModel.value) analysisModel.value = models.value[0] || ''
    modelsLoaded.value = true
    analysis.value = null
  } catch {
    if (alive && current === modelGeneration) analysisError.value = t('admin.priorityScheduling.actionFailed')
  } finally {
    if (alive && current === modelGeneration) modelsLoading.value = false
  }
}

async function runAnalysis() {
  if (!analysisModel.value) return
  const current = ++analysisGeneration
  analyzing.value = true
  analysisError.value = ''
  try {
    const result = await priorityAPI.getPriorityAnalysis(analysisModel.value, 60, analysisGroup.value)
    if (!alive || current !== analysisGeneration) return
    analysis.value = result
  } catch {
    if (alive && current === analysisGeneration) analysisError.value = t('admin.priorityScheduling.actionFailed')
  } finally {
    if (alive && current === analysisGeneration) analyzing.value = false
  }
}

async function recompute() {
  recomputing.value = true
  analysisError.value = ''
  try {
    recomputeResult.value = await priorityAPI.recomputePriorityScheduling()
  } catch {
    analysisError.value = t('admin.priorityScheduling.actionFailed')
  } finally {
    recomputing.value = false
  }
}

async function loadGroups() {
  try {
    groups.value = await groupsAPI.getAll('openai')
    groupsError.value = ''
  } catch {
    groups.value = []
    groupsError.value = t('admin.priorityScheduling.groupsLoadFailed')
  }
}

async function loadBindings() {
  if (!analysisGroup.value) return
  const current = ++bindingGeneration
  bindingsLoading.value = true
  bindingsError.value = ''
  try {
    const result = await priorityAPI.getStickyBindings(analysisGroup.value)
    if (current !== bindingGeneration) return
    bindings.value = result
  } catch {
    if (current === bindingGeneration) bindingsError.value = t('admin.priorityScheduling.actionFailed')
  } finally {
    if (current === bindingGeneration) bindingsLoading.value = false
  }
}

async function clearBindings() {
  if (!analysisGroup.value || clearing.value) return
  clearing.value = true
  bindingsError.value = ''
  try {
    await priorityAPI.clearStickyBindings(analysisGroup.value)
    confirmingClear.value = false
    await loadBindings()
  } catch {
    bindingsError.value = t('admin.priorityScheduling.actionFailed')
  } finally {
    clearing.value = false
  }
}

watch(analysisGroup, async () => {
  modelGeneration++
  analysisGeneration++
  bindingGeneration++
  analyzing.value = false
  analysis.value = null
  models.value = []
  analysisModel.value = ''
  modelsLoaded.value = false
  bindings.value = null
  confirmingClear.value = false
  await loadModels()
})

onMounted(async () => { await load(); await loadGroups(); await loadModels() })
onBeforeUnmount(() => { alive = false; generation++ })
</script>
