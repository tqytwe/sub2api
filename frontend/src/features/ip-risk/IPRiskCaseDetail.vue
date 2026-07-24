<template>
  <section v-if="detail" class="flex min-h-0 flex-1 flex-col" aria-live="polite">
    <header class="border-b border-gray-200 px-4 py-4 dark:border-dark-700 sm:px-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div class="flex flex-wrap items-center gap-2">
            <span :class="['badge', levelClass(detail.case.level)]">
              {{ levelLabel(detail.case.level) }}
            </span>
            <span class="font-mono text-base font-semibold text-gray-900 dark:text-white">
              {{ detail.case.primary_ip }}
            </span>
            <span v-if="detail.case.evidence_confidence !== 'exact'" class="badge badge-warning">
              {{ confidenceLabel(detail.case.evidence_confidence) }}
            </span>
          </div>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.ipRisk.caseDetected', { time: formatDateTime(detail.case.last_detected_at) }) }}
          </p>
        </div>
        <div class="text-right">
          <div class="text-3xl font-semibold tabular-nums text-gray-900 dark:text-white">
            {{ detail.case.score }}
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.riskScore') }}</div>
        </div>
      </div>

      <div class="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200">
        <div class="flex items-start gap-2">
          <Icon name="infoCircle" size="sm" class="mt-0.5 shrink-0" />
          <div>
            <div class="font-medium">{{ recommendationTitle }}</div>
            <div class="mt-1 text-xs opacity-80">{{ recommendationDescription }}</div>
          </div>
        </div>
      </div>
    </header>

    <nav class="flex shrink-0 overflow-x-auto border-b border-gray-200 px-2 dark:border-dark-700" :aria-label="t('admin.ipRisk.detailTabs.label')">
      <button
        v-for="tab in tabs"
        :key="tab.value"
        type="button"
        :class="[
          'whitespace-nowrap border-b-2 px-3 py-3 text-sm font-medium transition-colors',
          activeTab === tab.value
            ? 'border-primary-500 text-primary-600 dark:text-primary-400'
            : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200',
        ]"
        @click="activeTab = tab.value"
      >
        {{ tab.label }}
        <span v-if="tab.count !== undefined" class="ml-1 tabular-nums text-xs">({{ tab.count }})</span>
      </button>
    </nav>

    <div class="min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
      <div v-if="activeTab === 'overview'" class="space-y-5">
        <section>
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ipRisk.evidenceContribution') }}</h4>
          <div class="mt-3 space-y-2">
            <div
              v-for="signal in detail.case.signals"
              :key="signal.code"
              class="flex items-center justify-between gap-3 rounded-lg border border-gray-200 px-3 py-2.5 dark:border-dark-700"
            >
              <div class="min-w-0">
                <div class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ signalLabel(signal.code) }}</div>
                <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.ipRisk.signalCount', { count: signal.count || 0 }) }}
                </div>
              </div>
              <span :class="signal.score >= 0 ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'" class="font-semibold tabular-nums">
                {{ signal.score >= 0 ? '+' : '' }}{{ signal.score }}
              </span>
            </div>
          </div>
        </section>

        <section class="grid gap-3 sm:grid-cols-2">
          <div v-for="metric in evidenceMetrics" :key="metric.label" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
            <div class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ metric.value }}</div>
          </div>
        </section>
      </div>

      <div v-else-if="activeTab === 'users'" class="space-y-5">
        <section v-for="group in userGroups" :key="group.relation" class="space-y-2">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ group.label }}</h4>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ group.description }}</p>
            </div>
            <button
              v-if="group.relation === 'suspected_new' && group.users.length"
              type="button"
              class="btn btn-ghost btn-sm"
              @click="toggleGroup(group.users)"
            >
              {{ areAllSelected(group.users) ? t('admin.ipRisk.clearGroup') : t('admin.ipRisk.selectRecommended') }}
            </button>
          </div>

          <div v-if="group.users.length === 0" class="rounded-lg border border-dashed border-gray-300 px-4 py-5 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
            {{ t('admin.ipRisk.noUsersInGroup') }}
          </div>
          <div v-else class="space-y-2">
            <article
              v-for="user in group.users"
              :key="user.user_id"
              class="rounded-lg border border-gray-200 p-3 dark:border-dark-700"
            >
              <div class="flex items-start gap-3">
                <input
                  type="checkbox"
                  class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  :checked="selectedSet.has(user.user_id)"
                  :disabled="user.role === 'admin'"
                  :aria-label="t('admin.ipRisk.selectUser', { email: user.email })"
                  @change="toggleUser(user.user_id)"
                />
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ user.email }}</span>
                    <span v-if="user.role === 'admin'" class="badge badge-danger">{{ t('admin.ipRisk.protectedAdmin') }}</span>
                    <span v-if="user.evidence_confidence === 'inferred'" class="badge badge-warning">{{ t('admin.ipRisk.inferred') }}</span>
                    <span v-if="user.status === 'disabled'" class="badge badge-gray">{{ t('admin.ipRisk.disabled') }}</span>
                  </div>
                  <div class="mt-1 grid gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400 sm:grid-cols-2">
                    <span>{{ t('admin.ipRisk.signupSource') }}: {{ user.signup_source || '-' }}</span>
                    <span>{{ t('admin.ipRisk.registeredAt') }}: {{ formatDateTime(user.created_at) }}</span>
                    <span>{{ t('admin.ipRisk.recharged') }}: {{ formatMoney(user.total_recharged) }}</span>
                    <span>{{ t('admin.ipRisk.balance') }}: {{ formatMoney(user.balance) }}</span>
                    <span>{{ t('admin.ipRisk.apiKeys') }}: {{ user.active_api_key_count }}/{{ user.api_key_count }}</span>
                    <span>{{ t('admin.ipRisk.userId') }}: {{ user.user_id }}</span>
                  </div>

                  <div class="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-5">
                    <div class="rounded-md bg-gray-50 px-2.5 py-2 dark:bg-dark-900/50">
                      <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.userEvidence.primaryIpRegistrations') }}</div>
                      <div class="mt-0.5 text-sm font-semibold tabular-nums text-gray-900 dark:text-white">
                        {{ user.primary_ip_registrations }}
                      </div>
                    </div>
                    <div class="rounded-md bg-gray-50 px-2.5 py-2 dark:bg-dark-900/50">
                      <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.userEvidence.sharedUaAccounts') }}</div>
                      <div class="mt-0.5 text-sm font-semibold tabular-nums text-gray-900 dark:text-white">
                        {{ user.shared_ua_account_count }}
                      </div>
                    </div>
                    <div class="rounded-md bg-gray-50 px-2.5 py-2 dark:bg-dark-900/50">
                      <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.userEvidence.giftGranted') }}</div>
                      <div class="mt-0.5 text-sm font-semibold tabular-nums text-gray-900 dark:text-white">
                        {{ formatMoney(user.gift_granted) }}
                      </div>
                    </div>
                    <div class="rounded-md bg-gray-50 px-2.5 py-2 dark:bg-dark-900/50">
                      <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.userEvidence.giftConsumed') }}</div>
                      <div class="mt-0.5 text-sm font-semibold tabular-nums text-gray-900 dark:text-white">
                        {{ formatMoney(user.gift_consumed) }}
                      </div>
                    </div>
                    <div class="rounded-md bg-gray-50 px-2.5 py-2 dark:bg-dark-900/50">
                      <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.userEvidence.giftRemaining') }}</div>
                      <div class="mt-0.5 text-sm font-semibold tabular-nums text-gray-900 dark:text-white">
                        {{ formatMoney(user.gift_remaining) }}
                      </div>
                    </div>
                  </div>

                  <details class="mt-3 rounded-lg border border-gray-100 bg-gray-50/70 dark:border-dark-700 dark:bg-dark-900/30">
                    <summary class="cursor-pointer select-none px-3 py-2 text-xs font-medium text-gray-700 dark:text-gray-200">
                      {{ t('admin.ipRisk.userEvidence.keyDetails', { active: user.active_api_key_count, total: user.api_key_count }) }}
                    </summary>
                    <div v-if="user.api_keys.length" class="divide-y divide-gray-100 border-t border-gray-100 dark:divide-dark-700 dark:border-dark-700">
                      <div v-for="key in user.api_keys" :key="key.id" class="grid gap-1 px-3 py-2.5 text-xs sm:grid-cols-[minmax(0,1fr)_auto]">
                        <div class="min-w-0">
                          <div class="flex flex-wrap items-center gap-2">
                            <span class="truncate font-medium text-gray-800 dark:text-gray-100">{{ key.name || `#${key.id}` }}</span>
                            <span :class="['badge', keyStatusClass(key.status)]">{{ key.status }}</span>
                          </div>
                          <div class="mt-1 text-gray-500 dark:text-gray-400">
                            {{ t('admin.ipRisk.userEvidence.keyCreatedAt') }}: {{ formatDateTime(key.created_at) }}
                          </div>
                        </div>
                        <div class="text-gray-500 dark:text-gray-400 sm:text-right">
                          {{ t('admin.ipRisk.userEvidence.keyLastUsedAt') }}:
                          {{ key.last_used_at ? formatDateTime(key.last_used_at) : t('admin.ipRisk.userEvidence.neverUsed') }}
                        </div>
                      </div>
                    </div>
                    <div v-else class="border-t border-gray-100 px-3 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
                      {{ t('admin.ipRisk.userEvidence.noApiKeys') }}
                    </div>
                  </details>
                </div>
              </div>
            </article>
          </div>
        </section>
      </div>

      <div v-else-if="activeTab === 'evidence'" class="space-y-3">
        <div
          v-for="signal in detail.case.signals"
          :key="signal.code"
          class="rounded-lg border border-gray-200 p-4 dark:border-dark-700"
        >
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ signalLabel(signal.code) }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ signal.family }}</div>
            </div>
            <span class="badge" :class="signal.score >= 20 ? 'badge-danger' : signal.score >= 0 ? 'badge-warning' : 'badge-success'">
              {{ signal.score >= 0 ? '+' : '' }}{{ signal.score }}
            </span>
          </div>
        </div>
      </div>

      <div v-else-if="activeTab === 'timeline'" class="space-y-3">
        <div v-if="detail.timeline.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.ipRisk.noTimeline') }}
        </div>
        <div v-for="event in detail.timeline" :key="event.id" class="flex gap-3">
          <div class="mt-1 h-2.5 w-2.5 shrink-0 rounded-full bg-primary-500"></div>
          <div class="min-w-0 border-b border-gray-100 pb-3 dark:border-dark-700">
            <div class="flex flex-wrap items-center gap-2 text-sm">
              <span class="font-medium text-gray-900 dark:text-white">{{ eventTypeLabel(event.event_type) }}</span>
              <span class="font-mono text-xs text-gray-500 dark:text-gray-400">{{ event.ip_address }}</span>
              <span v-if="event.confidence === 'inferred'" class="badge badge-warning">{{ t('admin.ipRisk.inferred') }}</span>
            </div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ formatDateTime(event.occurred_at) }}
              <span v-if="event.user_id"> · {{ t('admin.ipRisk.userId') }} {{ event.user_id }}</span>
            </div>
          </div>
        </div>
      </div>

      <div v-else class="space-y-3">
        <div v-if="detail.actions.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.ipRisk.noActions') }}
        </div>
        <article v-for="action in detail.actions" :key="action.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ actionLabel(action.action_type) }}</span>
              <span :class="['badge', actionStatusClass(action.status)]">{{ action.status }}</span>
            </div>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(action.created_at) }}</span>
          </div>
          <p class="mt-2 text-sm text-gray-600 dark:text-gray-300">{{ action.reason }}</p>
          <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
            <span>{{ t('admin.ipRisk.actionResults.completed', { count: resultNumber(action, 'completed_items') }) }}</span>
            <span v-if="resultNumber(action, 'failed_items')">
              {{ t('admin.ipRisk.actionResults.failed', { count: resultNumber(action, 'failed_items') }) }}
            </span>
            <span v-if="resultNumber(action, 'conflict_items')">
              {{ t('admin.ipRisk.actionResults.conflicts', { count: resultNumber(action, 'conflict_items') }) }}
            </span>
            <span v-if="resultNumber(action, 'skipped_items')">
              {{ t('admin.ipRisk.actionResults.skipped', { count: resultNumber(action, 'skipped_items') }) }}
            </span>
          </div>
        </article>
      </div>
    </div>

    <footer class="shrink-0 border-t border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-800 sm:p-4">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <span class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ipRisk.selectedUsers', { count: selectedUserIds.length }) }}
        </span>
        <div class="flex flex-wrap justify-end gap-2">
          <button type="button" class="btn btn-secondary btn-sm" @click="$emit('action', 'observe')">
            <Icon name="eye" size="sm" class="mr-1.5" />
            {{ t('admin.ipRisk.actions.observe') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="selectedUserIds.length === 0" @click="$emit('action', 'disable_api_keys')">
            <Icon name="key" size="sm" class="mr-1.5" />
            {{ t('admin.ipRisk.actions.disableKeys') }}
          </button>
          <button type="button" class="btn btn-danger btn-sm" :disabled="selectedUserIds.length === 0" @click="$emit('action', 'disable_users')">
            <Icon name="ban" size="sm" class="mr-1.5" />
            {{ t('admin.ipRisk.actions.disableUsers') }}
          </button>
          <button type="button" class="btn btn-primary btn-sm" @click="$emit('action', 'temporary_registration_block')">
            <Icon name="shield" size="sm" class="mr-1.5" />
            {{ t('admin.ipRisk.actions.preview') }}
          </button>
        </div>
      </div>
    </footer>
  </section>

  <div
    v-else
    data-testid="ip-risk-detail-empty"
    class="flex min-h-[360px] flex-1 items-center justify-center px-6 text-center text-sm text-gray-500 dark:text-gray-400"
  >
    {{ t('admin.ipRisk.selectCasePrompt') }}
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import type {
  RiskActionType,
  RiskCaseDetail,
  RiskLevel,
  RiskRelatedUser,
  RiskSignalCode,
  RiskUserRelation,
} from './types'

const props = defineProps<{
  detail: RiskCaseDetail | null
  selectedUserIds: number[]
}>()

const emit = defineEmits<{
  (event: 'update:selectedUserIds', value: number[]): void
  (event: 'action', value: RiskActionType): void
}>()

const { t } = useI18n()
const activeTab = ref<'overview' | 'users' | 'evidence' | 'timeline' | 'actions'>('overview')
const selectedSet = computed(() => new Set(props.selectedUserIds))

watch(
  () => props.detail?.case.id,
  () => {
    activeTab.value = 'overview'
  },
)

const tabs = computed(() => [
  { value: 'overview' as const, label: t('admin.ipRisk.detailTabs.overview') },
  { value: 'users' as const, label: t('admin.ipRisk.detailTabs.users'), count: props.detail?.users.length || 0 },
  { value: 'evidence' as const, label: t('admin.ipRisk.detailTabs.evidence'), count: props.detail?.case.signals.length || 0 },
  { value: 'timeline' as const, label: t('admin.ipRisk.detailTabs.timeline'), count: props.detail?.timeline.length || 0 },
  { value: 'actions' as const, label: t('admin.ipRisk.detailTabs.actions'), count: props.detail?.actions.length || 0 },
])

const userGroups = computed(() => {
  const definitions: Array<{ relation: RiskUserRelation; label: string; description: string }> = [
    {
      relation: 'suspected_new',
      label: t('admin.ipRisk.userGroups.suspected'),
      description: t('admin.ipRisk.userGroups.suspectedHint'),
    },
    {
      relation: 'trusted_existing',
      label: t('admin.ipRisk.userGroups.trusted'),
      description: t('admin.ipRisk.userGroups.trustedHint'),
    },
    {
      relation: 'disabled',
      label: t('admin.ipRisk.userGroups.disabled'),
      description: t('admin.ipRisk.userGroups.disabledHint'),
    },
  ]
  return definitions.map((definition) => ({
    ...definition,
    users: (props.detail?.users || []).filter((user) => user.relation_type === definition.relation),
  }))
})

const evidenceMetrics = computed(() => {
  const evidence = props.detail?.evidence
  if (!evidence) return []
  return [
    { label: t('admin.ipRisk.metrics.registration10m'), value: evidence.registration_count_10m },
    { label: t('admin.ipRisk.metrics.registration1h'), value: evidence.registration_count_1h },
    { label: t('admin.ipRisk.metrics.registration24h'), value: evidence.registration_count_24h },
    { label: t('admin.ipRisk.metrics.exactRegistrations'), value: evidence.exact_registration_count },
    { label: t('admin.ipRisk.metrics.sharedUa'), value: evidence.max_shared_ua_count },
    { label: t('admin.ipRisk.metrics.sharedApiIp'), value: evidence.shared_api_ip_user_count },
  ]
})

const recommendationTitle = computed(() => {
  if (!props.detail) return ''
  if (props.detail.case.level === 'critical') return t('admin.ipRisk.recommendations.critical')
  if (props.detail.case.level === 'severe') return t('admin.ipRisk.recommendations.severe')
  if (props.detail.case.level === 'high') return t('admin.ipRisk.recommendations.high')
  return t('admin.ipRisk.recommendations.observe')
})

const recommendationDescription = computed(() => {
  if (!props.detail) return ''
  if (props.detail.case.evidence_confidence !== 'exact') return t('admin.ipRisk.recommendations.inferredProtection')
  if (props.detail.evidence.known_shared_network) return t('admin.ipRisk.recommendations.sharedProtection')
  return t('admin.ipRisk.recommendations.previewFirst')
})

function toggleUser(userId: number) {
  const next = new Set(props.selectedUserIds)
  if (next.has(userId)) next.delete(userId)
  else next.add(userId)
  emit('update:selectedUserIds', Array.from(next))
}

function areAllSelected(users: RiskRelatedUser[]) {
  const selectable = users.filter((user) => user.role !== 'admin' && user.recommended_selected)
  return selectable.length > 0 && selectable.every((user) => selectedSet.value.has(user.user_id))
}

function toggleGroup(users: RiskRelatedUser[]) {
  const next = new Set(props.selectedUserIds)
  const recommended = users.filter((user) => user.role !== 'admin' && user.recommended_selected)
  const shouldClear = recommended.length > 0 && recommended.every((user) => next.has(user.user_id))
  recommended.forEach((user) => {
    if (shouldClear) next.delete(user.user_id)
    else next.add(user.user_id)
  })
  emit('update:selectedUserIds', Array.from(next))
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

function confidenceLabel(confidence: string) {
  return t(`admin.ipRisk.confidence.${confidence}`)
}

function signalLabel(code: RiskSignalCode) {
  return t(`admin.ipRisk.signals.${code}`)
}

function eventTypeLabel(type: string) {
  return type === 'register' ? t('admin.ipRisk.events.register') : t('admin.ipRisk.events.login')
}

function actionLabel(action: RiskActionType) {
  return t(`admin.ipRisk.actionTypes.${action}`)
}

function actionStatusClass(status: string) {
  if (status === 'completed' || status === 'rolled_back') return 'badge-success'
  if (status === 'partial') return 'badge-warning'
  if (status === 'failed') return 'badge-danger'
  return 'badge-gray'
}

function keyStatusClass(status: string) {
  if (status === 'active') return 'badge-success'
  if (status === 'disabled') return 'badge-warning'
  if (status === 'revoked' || status === 'deleted') return 'badge-danger'
  return 'badge-gray'
}

function resultNumber(action: { result?: Record<string, unknown> }, key: string) {
  const value = action.result?.[key]
  return typeof value === 'number' ? value : Number(value || 0)
}

function formatMoney(value: number) {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD' }).format(value || 0)
}
</script>
