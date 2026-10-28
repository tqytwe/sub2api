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
          <p class="mb-3 text-xs text-gray-500">{{ new Date(snapshot.at).toLocaleString() }} · {{ t('admin.priorityScheduling.model') }}: {{ snapshot.model }} · {{ t('admin.priorityScheduling.group') }}: {{ snapshot.group_id ?? '—' }}</p>
          <p v-if="snapshot.history_status && snapshot.history_status !== 'ready'" class="mb-3 text-sm text-amber-700 dark:text-amber-400">{{ t('admin.priorityScheduling.historyUnavailable', { status: snapshot.history_status }) }}</p>
          <DataTable :columns="columns" :data="snapshot.candidates" row-key="account_id">
            <template #cell-account="{ row }"><span class="font-medium text-gray-900 dark:text-white">{{ row.account_name }} <span class="font-normal text-gray-500">#{{ row.account_id }}</span></span></template>
            <template #cell-state="{ row }">
              <span v-if="row.latency_stopped" class="font-medium text-red-700 dark:text-red-300">{{ t('admin.priorityScheduling.stopped') }}</span>
              <span v-else>{{ t(`admin.priorityScheduling.tiers.${row.tier}`) }}</span>
              <small v-if="row.latency_penalty_until" class="block text-gray-500">{{ t('admin.priorityScheduling.penaltyUntil') }}: {{ new Date(row.latency_penalty_until).toLocaleString() }}</small>
            </template>
            <template #cell-latency="{ row }">P{{ row.latency_percentile ?? config?.latency_percentile ?? 90 }} · {{ row.selected_ttft_ms == null ? '—' : `${Math.round(row.selected_ttft_ms)} ms` }} · {{ row.samples }} {{ t('admin.priorityScheduling.samples') }}</template>
            <template #cell-reasons="{ row }"><span v-for="reason in row.reasons" :key="reason" class="mr-2 text-xs text-gray-500">{{ t(`admin.priorityScheduling.reasons.${reason}`) }}</span></template>
            <template #empty><p class="p-8 text-center text-sm text-gray-500">{{ t('admin.priorityScheduling.emptyCandidates') }}</p></template>
          </DataTable>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import * as priorityAPI from '@/api/admin/priorityScheduling'

const { t } = useI18n()
const config = ref<priorityAPI.PrioritySchedulingConfig | null>(null)
const snapshot = ref<priorityAPI.PrioritySnapshot | null>(null)
const loading = ref(false)
const saving = ref(false)
const refreshing = ref(false)
const error = ref('')
const snapshotError = ref('')
const notice = ref('')
const columns = computed<Column[]>(() => [
  { key: 'account', label: t('admin.priorityScheduling.account') },
  { key: 'state', label: t('admin.priorityScheduling.state') },
  { key: 'latency', label: t('admin.priorityScheduling.latency') },
  { key: 'reasons', label: t('admin.priorityScheduling.reasonsTitle') },
])
let alive = true
let generation = 0

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

onMounted(load)
onBeforeUnmount(() => { alive = false; generation++ })
</script>
