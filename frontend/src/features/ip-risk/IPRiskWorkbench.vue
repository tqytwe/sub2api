<template>
  <section class="space-y-4">
    <div
      v-if="runtime?.degraded"
      class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-800/60 dark:bg-red-950/30 dark:text-red-200"
      role="alert"
    >
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="flex items-start gap-2">
          <Icon name="exclamationTriangle" size="md" class="mt-0.5 shrink-0" />
          <div>
            <div class="font-medium">{{ t('admin.ipRisk.degradedTitle') }}</div>
            <div class="mt-1 opacity-80">{{ runtime.degraded_reason || runtime.last_error || t('admin.ipRisk.degradedFallback') }}</div>
          </div>
        </div>
        <button type="button" class="btn btn-ghost btn-sm" @click="refreshAll">{{ t('common.retry') }}</button>
      </div>
    </div>

    <div
      v-else-if="runtime && runtime.shadow_mode"
      class="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-800 dark:border-blue-800/60 dark:bg-blue-950/30 dark:text-blue-200"
    >
      <div class="flex items-start gap-2">
        <Icon name="infoCircle" size="sm" class="mt-0.5 shrink-0" />
        <span>{{ t('admin.ipRisk.shadowModeHint') }}</span>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article v-for="card in overviewCards" :key="card.label" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-3">
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ card.label }}</span>
          <Icon :name="card.icon" size="md" :class="card.iconClass" />
        </div>
        <div class="mt-3 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ card.value }}</div>
        <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ card.hint }}</div>
      </article>
    </div>

    <div class="flex flex-wrap items-center gap-3 rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-800">
      <div class="relative min-w-[220px] flex-1">
        <Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        <input
          v-model.trim="filters.search"
          class="input pl-9"
          :placeholder="t('admin.ipRisk.searchPlaceholder')"
          @input="scheduleSearch"
        />
      </div>
      <div class="w-full sm:w-40">
        <Select v-model="filters.range" :options="rangeOptions" @change="applyFilters" />
      </div>
      <div class="w-full sm:w-36">
        <Select v-model="filters.level" :options="levelOptions" @change="applyFilters" />
      </div>
      <div class="w-full sm:w-40">
        <Select v-model="filters.status" :options="statusOptions" @change="applyFilters" />
      </div>
      <div class="w-full sm:w-48">
        <Select v-model="filters.signal" :options="signalOptions" @change="applyFilters" />
      </div>
      <button type="button" class="btn btn-secondary" :disabled="scanning" @click="startScan">
        <Icon name="refresh" size="sm" class="mr-2" />
        {{ scanning ? t('admin.ipRisk.scanning') : t('admin.ipRisk.scanNow') }}
      </button>
      <button type="button" class="btn btn-secondary" @click="showPolicyDialog = true">
        <Icon name="cog" size="sm" class="mr-2" />
        {{ t('admin.ipRisk.policy') }}
      </button>
    </div>

    <div v-if="activeScan" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800" aria-live="polite">
      <div class="flex flex-wrap items-center justify-between gap-3 text-sm">
        <span class="font-medium text-gray-900 dark:text-white">{{ t('admin.ipRisk.scanProgress') }}</span>
        <span class="tabular-nums text-gray-500 dark:text-gray-400">{{ activeScan.progress }}%</span>
      </div>
      <div class="mt-2 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
        <div class="h-full rounded-full bg-primary-500 transition-[width] duration-300" :style="{ width: `${activeScan.progress}%` }"></div>
      </div>
      <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ t('admin.ipRisk.scanCandidates', { count: activeScan.candidate_count }) }}</span>
        <span>{{ t('admin.ipRisk.scanCases', { count: activeScan.case_count }) }}</span>
        <span>{{ t('admin.ipRisk.scanInferred', { count: activeScan.inferred_event_count }) }}</span>
      </div>
    </div>

    <div v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-800/60 dark:bg-red-950/30 dark:text-red-200">
      <div class="flex items-center justify-between gap-3">
        <span>{{ loadError }}</span>
        <button type="button" class="btn btn-ghost btn-sm" @click="refreshAll">{{ t('common.retry') }}</button>
      </div>
    </div>

    <div class="grid min-h-[620px] gap-4 2xl:grid-cols-[minmax(440px,0.92fr)_minmax(640px,1.35fr)]">
      <section class="flex min-h-0 flex-col overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <header class="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ipRisk.caseList') }}</h2>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.caseCount', { count: total }) }}</p>
          </div>
          <button type="button" class="btn btn-ghost btn-sm" :disabled="loadingCases" @click="loadCases">
            <Icon name="refresh" size="sm" />
          </button>
        </header>

        <div v-if="loadingCases" class="flex min-h-[420px] items-center justify-center text-sm text-gray-500 dark:text-gray-400">
          <Icon name="refresh" size="md" class="mr-2" />
          {{ t('common.loading') }}
        </div>
        <div v-else-if="cases.length === 0" class="flex min-h-[420px] flex-col items-center justify-center px-8 text-center">
          <Icon name="shield" size="xl" class="text-emerald-500" />
          <div class="mt-3 text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.ipRisk.emptyTitle') }}</div>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ hasActiveFilters ? t('admin.ipRisk.emptyFiltered') : t('admin.ipRisk.emptyHint') }}
          </p>
          <button v-if="hasActiveFilters" type="button" class="btn btn-secondary btn-sm mt-4" @click="clearFilters">
            {{ t('admin.ipRisk.clearFilters') }}
          </button>
        </div>
        <div v-else class="min-h-0 flex-1 overflow-y-auto">
          <button
            v-for="riskCase in cases"
            :key="riskCase.id"
            type="button"
            class="block w-full border-b border-gray-100 px-4 py-4 text-left transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:border-dark-700 dark:hover:bg-dark-700/60"
            :class="selectedCaseId === riskCase.id ? 'bg-primary-50 dark:bg-primary-950/20' : ''"
            @click="selectCase(riskCase)"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span :class="['badge', levelClass(riskCase.level)]">{{ levelLabel(riskCase.level) }}</span>
                  <span class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ riskCase.primary_ip }}</span>
                  <span v-if="riskCase.evidence_confidence !== 'exact'" class="badge badge-warning">{{ confidenceLabel(riskCase.evidence_confidence) }}</span>
                </div>
                <p class="mt-2 text-sm font-medium text-gray-800 dark:text-gray-100">
                  {{ caseSummary(riskCase) }}
                </p>
                <div class="mt-2 flex flex-wrap gap-1.5">
                  <span v-for="signal in riskCase.signals.slice(0, 3)" :key="signal.code" class="badge badge-gray">
                    {{ signalLabel(signal.code) }}
                  </span>
                </div>
                <div class="mt-3 flex flex-wrap items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
                  <span><Icon name="users" size="xs" class="mr-1 inline" />{{ riskCase.related_user_count }}</span>
                  <span>{{ statusLabel(riskCase.status) }}</span>
                  <span>{{ relativeTime(riskCase.last_detected_at) }}</span>
                </div>
              </div>
              <div class="text-right">
                <div class="text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ riskCase.score }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.score') }}</div>
              </div>
            </div>
          </button>
        </div>
        <Pagination
          v-if="total > 0"
          :page="page"
          :page-size="pageSize"
          :total="total"
          @update:page="changePage"
          @update:page-size="changePageSize"
        />
      </section>

      <section class="hidden min-h-0 overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800 2xl:flex">
        <div v-if="loadingDetail" class="flex flex-1 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
          <Icon name="refresh" size="md" class="mr-2" />
          {{ t('common.loading') }}
        </div>
        <IPRiskCaseDetail
          v-else
          v-model:selected-user-ids="selectedUserIds"
          :detail="detail"
          @action="openAction"
        />
      </section>
    </div>
  </section>

  <BaseDialog
    :show="showDetailDialog"
    :title="t('admin.ipRisk.caseDetail')"
    width="full"
    :close-on-click-outside="false"
    @close="showDetailDialog = false"
  >
    <div class="-m-6 flex min-h-[70vh] flex-col">
      <div v-if="loadingDetail" class="flex flex-1 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        <Icon name="refresh" size="md" class="mr-2" />
        {{ t('common.loading') }}
      </div>
      <IPRiskCaseDetail
        v-else
        v-model:selected-user-ids="selectedUserIds"
        :detail="detail"
        @action="openAction"
      />
    </div>
  </BaseDialog>

  <IPRiskActionDialog
    :show="showActionDialog"
    :detail="detail"
    :selected-user-ids="selectedUserIds"
    :initial-action="initialAction"
    :step-up="props.stepUp"
    @close="showActionDialog = false"
    @completed="handleActionCompleted"
    @stale="reloadSelectedCase"
  />

  <IPRiskPolicyDialog
    :show="showPolicyDialog"
    :step-up="props.stepUp"
    @close="showPolicyDialog = false"
    @updated="refreshAll"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { StepUpController } from '@/composables/useStepUp'
import IPRiskCaseDetail from './IPRiskCaseDetail.vue'
import IPRiskActionDialog from './IPRiskActionDialog.vue'
import IPRiskPolicyDialog from './IPRiskPolicyDialog.vue'
import type {
  EvidenceConfidence,
  IPRiskOverview,
  RiskActionRecord,
  RiskActionType,
  RiskCase,
  RiskCaseDetail,
  RiskLevel,
  RiskRuntime,
  RiskScan,
  RiskSignalCode,
} from './types'

const props = defineProps<{
  stepUp: StepUpController
}>()
const emit = defineEmits<{ (event: 'overview', count: number): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const overview = ref<IPRiskOverview | null>(null)
const runtime = ref<RiskRuntime | null>(null)
const cases = ref<RiskCase[]>([])
const detail = ref<RiskCaseDetail | null>(null)
const selectedCaseId = ref<number | null>(null)
const selectedUserIds = ref<number[]>([])
const loadingCases = ref(false)
const loadingDetail = ref(false)
const loadError = ref('')
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const showDetailDialog = ref(false)
const showActionDialog = ref(false)
const showPolicyDialog = ref(false)
const initialAction = ref<RiskActionType>('temporary_registration_block')
const viewportWidth = ref(typeof window === 'undefined' ? 1920 : window.innerWidth)
const activeScan = ref<RiskScan | null>(null)
let scanPollTimer: ReturnType<typeof setTimeout> | null = null
let searchTimer: ReturnType<typeof setTimeout> | null = null

const filters = reactive({
  search: '',
  range: '24h',
  level: '',
  status: '',
  signal: '',
})

const overviewCards = computed(() => [
  {
    label: t('admin.ipRisk.overview.openCases'),
    value: overview.value?.open_cases || 0,
    hint: t('admin.ipRisk.overview.openCasesHint'),
    icon: 'shield' as const,
    iconClass: 'text-primary-600 dark:text-primary-400',
  },
  {
    label: t('admin.ipRisk.overview.criticalCases'),
    value: overview.value?.critical_cases || 0,
    hint: t('admin.ipRisk.overview.criticalCasesHint'),
    icon: 'exclamationTriangle' as const,
    iconClass: 'text-red-600 dark:text-red-400',
  },
  {
    label: t('admin.ipRisk.overview.blocks'),
    value: overview.value?.blocked_policies || 0,
    hint: t('admin.ipRisk.overview.blocksHint'),
    icon: 'ban' as const,
    iconClass: 'text-amber-600 dark:text-amber-400',
  },
  {
    label: t('admin.ipRisk.overview.reviewUsers'),
    value: overview.value?.review_users || 0,
    hint: t('admin.ipRisk.overview.reviewUsersHint'),
    icon: 'users' as const,
    iconClass: 'text-blue-600 dark:text-blue-400',
  },
])

const rangeOptions = computed(() => [
  { value: '24h', label: t('admin.ipRisk.ranges.24h') },
  { value: '7d', label: t('admin.ipRisk.ranges.7d') },
  { value: '30d', label: t('admin.ipRisk.ranges.30d') },
  { value: '90d', label: t('admin.ipRisk.ranges.90d') },
])

const levelOptions = computed(() => [
  { value: '', label: t('admin.ipRisk.allLevels') },
  ...(['medium', 'high', 'severe', 'critical'] as RiskLevel[]).map((value) => ({
    value,
    label: levelLabel(value),
  })),
])

const statusOptions = computed(() => [
  { value: '', label: t('admin.ipRisk.allStatuses') },
  ...(['open', 'observing', 'processing', 'resolved', 'ignored'] as const).map((value) => ({
    value,
    label: statusLabel(value),
  })),
])

const signalOptions = computed(() => [
  { value: '', label: t('admin.ipRisk.allSignals') },
  ...([
    'registration_10m',
    'registration_1h',
    'registration_24h',
    'shared_ua_3',
    'shared_ua_5',
    'email_pattern',
    'shared_api_ip',
    'rapid_key_or_gift',
    'shared_signup_code',
  ] as RiskSignalCode[]).map((value) => ({
    value,
    label: signalLabel(value),
  })),
])

const hasActiveFilters = computed(() =>
  Boolean(filters.search || filters.level || filters.status || filters.signal || filters.range !== '24h'),
)

const scanning = computed(() =>
  activeScan.value?.status === 'pending' || activeScan.value?.status === 'running',
)

onMounted(() => {
  window.addEventListener('resize', handleResize)
  refreshAll()
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (scanPollTimer) clearTimeout(scanPollTimer)
  if (searchTimer) clearTimeout(searchTimer)
})

async function refreshAll() {
  loadError.value = ''
  await Promise.all([loadOverviewAndRuntime(), loadCases()])
}

async function loadOverviewAndRuntime() {
  try {
    const [nextOverview, nextRuntime] = await Promise.all([
      adminAPI.ipRisk.getOverview(),
      adminAPI.ipRisk.getRuntime(),
    ])
    overview.value = nextOverview
    runtime.value = nextRuntime
    emit('overview', nextOverview.open_cases)
    if (nextRuntime.last_scan && ['pending', 'running'].includes(nextRuntime.last_scan.status)) {
      activeScan.value = nextRuntime.last_scan
      scheduleScanPoll()
    }
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('admin.ipRisk.loadFailed'))
  }
}

async function loadCases() {
  loadingCases.value = true
  try {
    const { start, end } = dateRange(filters.range)
    const response = await adminAPI.ipRisk.listCases({
      page: page.value,
      page_size: pageSize.value,
      search: filters.search || undefined,
      level: filters.level || undefined,
      status: filters.status || undefined,
      signal: filters.signal || undefined,
      range_start: start,
      range_end: end,
    })
    cases.value = response.items
    total.value = response.total
    if (cases.value.length && !selectedCaseId.value) {
      await selectCase(cases.value[0], false)
    } else if (selectedCaseId.value && !cases.value.some((item) => item.id === selectedCaseId.value)) {
      selectedCaseId.value = null
      detail.value = null
      selectedUserIds.value = []
    }
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('admin.ipRisk.loadFailed'))
  } finally {
    loadingCases.value = false
  }
}

async function selectCase(riskCase: RiskCase, openDialog = true) {
  selectedCaseId.value = riskCase.id
  if (openDialog && viewportWidth.value < 1536) showDetailDialog.value = true
  await reloadSelectedCase()
}

async function reloadSelectedCase() {
  if (!selectedCaseId.value) return
  loadingDetail.value = true
  try {
    detail.value = await adminAPI.ipRisk.getCase(selectedCaseId.value)
    selectedUserIds.value = detail.value.users
      .filter((user) =>
        user.relation_type === 'suspected_new'
        && user.recommended_selected
        && user.role !== 'admin'
        && user.evidence_confidence === 'exact',
      )
      .map((user) => user.user_id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.ipRisk.detailLoadFailed')))
  } finally {
    loadingDetail.value = false
  }
}

function openAction(action: RiskActionType) {
  initialAction.value = action
  showActionDialog.value = true
}

async function handleActionCompleted(_record: RiskActionRecord) {
  await Promise.all([loadOverviewAndRuntime(), loadCases(), reloadSelectedCase()])
}

async function startScan() {
  const { start, end } = dateRange(filters.range)
  try {
    activeScan.value = await adminAPI.ipRisk.startScan(start, end)
    appStore.showSuccess(t('admin.ipRisk.scanStarted'))
    scheduleScanPoll()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.ipRisk.scanFailed')))
  }
}

function scheduleScanPoll() {
  if (!activeScan.value || !['pending', 'running'].includes(activeScan.value.status)) return
  if (scanPollTimer) clearTimeout(scanPollTimer)
  scanPollTimer = setTimeout(pollScan, 1500)
}

async function pollScan() {
  if (!activeScan.value) return
  try {
    activeScan.value = await adminAPI.ipRisk.getScan(activeScan.value.id)
    if (['pending', 'running'].includes(activeScan.value.status)) {
      scheduleScanPoll()
      return
    }
    if (activeScan.value.status === 'completed') {
      appStore.showSuccess(t('admin.ipRisk.scanCompleted'))
      await refreshAll()
    } else {
      appStore.showError(activeScan.value.error_message || t('admin.ipRisk.scanFailed'))
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.ipRisk.scanStatusFailed')))
  }
}

function scheduleSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(applyFilters, 300)
}

function applyFilters() {
  page.value = 1
  selectedCaseId.value = null
  detail.value = null
  loadCases()
}

function clearFilters() {
  filters.search = ''
  filters.range = '24h'
  filters.level = ''
  filters.status = ''
  filters.signal = ''
  applyFilters()
}

function changePage(value: number) {
  page.value = value
  selectedCaseId.value = null
  loadCases()
}

function changePageSize(value: number) {
  pageSize.value = value
  page.value = 1
  selectedCaseId.value = null
  loadCases()
}

function handleResize() {
  viewportWidth.value = window.innerWidth
  if (viewportWidth.value >= 1536) showDetailDialog.value = false
}

function dateRange(range: string) {
  const end = new Date()
  const duration = range === '90d' ? 90 : range === '30d' ? 30 : range === '7d' ? 7 : 1
  const start = new Date(end.getTime() - duration * 24 * 60 * 60 * 1000)
  return { start: start.toISOString(), end: end.toISOString() }
}

function levelClass(level: RiskLevel) {
  return level === 'critical' || level === 'severe'
    ? 'badge-danger'
    : level === 'high'
      ? 'badge-warning'
      : level === 'medium'
        ? 'badge-primary'
        : 'badge-gray'
}

function levelLabel(level: RiskLevel) {
  return t(`admin.ipRisk.levels.${level}`)
}

function statusLabel(status: string) {
  return t(`admin.ipRisk.statuses.${status}`)
}

function confidenceLabel(confidence: EvidenceConfidence) {
  return t(`admin.ipRisk.confidence.${confidence}`)
}

function signalLabel(signal: RiskSignalCode) {
  return t(`admin.ipRisk.signals.${signal}`)
}

function caseSummary(riskCase: RiskCase) {
  return t('admin.ipRisk.caseSummary', {
    count: riskCase.related_user_count,
    signal: riskCase.signals[0] ? signalLabel(riskCase.signals[0].code) : t('admin.ipRisk.unknownSignal'),
  })
}

function relativeTime(value: string) {
  const date = new Date(value)
  const diff = Math.max(0, Date.now() - date.getTime())
  const minutes = Math.floor(diff / 60_000)
  if (minutes < 60) return t('admin.ipRisk.relative.minutes', { count: minutes })
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t('admin.ipRisk.relative.hours', { count: hours })
  return t('admin.ipRisk.relative.days', { count: Math.floor(hours / 24) })
}
</script>
