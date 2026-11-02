<template>
  <section class="mx-auto w-full max-w-6xl space-y-5 px-1 py-2 sm:px-2">
    <header
      class="page-header mb-0 flex flex-wrap items-center justify-between gap-3 rounded-lg bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6"
    >
      <div class="min-w-0">
        <h2 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
          <span class="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400">
            <Icon name="chart" size="sm" />
          </span>
          {{ t('channelMonitorV2.settings.title') }}
        </h2>
        <p class="page-description mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('channelMonitorV2.settings.description') }}
        </p>
      </div>
      <button
        v-if="draft"
        type="button"
        class="btn btn-primary"
        :disabled="saving || !dirty || !configValid"
        @click="save"
      >
        <Icon name="check" size="sm" />
        {{ t('channelMonitorV2.settings.save') }}
      </button>
    </header>

    <div
      v-if="!systemModeV2"
      class="rounded-lg border border-amber-200 bg-amber-50/90 px-4 py-3 text-sm text-amber-900 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-100"
      role="status"
    >
      {{
        t('channelMonitorV2.settings.modeBanner', {
          mode: systemModeLabel,
          modeV2: t('channelMonitorV2.settings.modeV2'),
        })
      }}
      <router-link class="ml-1 font-medium underline" to="/admin/settings">{{ t('admin.settings.tabs.features') }}</router-link>
    </div>

    <div
      v-if="loading"
      class="card flex min-h-[200px] items-center justify-center !rounded-lg !border-0 text-sm text-gray-400 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700"
    >
      <span>{{ t('channelMonitorV2.settings.loading') }}</span>
    </div>

    <div
      v-else-if="configLoadError"
      class="card flex min-h-[200px] flex-col items-center justify-center gap-3 !rounded-lg !border-0 px-5 py-8 text-center shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
      role="alert"
    >
      <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('channelMonitorV2.settings.loadFailed') }}</p>
      <p class="max-w-lg text-xs text-gray-500 dark:text-dark-400">{{ configLoadError }}</p>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        data-testid="channel-monitor-v2-config-retry"
        @click="load"
      >
        <Icon name="refresh" size="sm" />
        {{ t('channelMonitorV2.settings.retryConfig') }}
      </button>
    </div>

    <template v-else-if="draft">
      <div class="card divide-y divide-gray-100 !rounded-lg !border-0 shadow-sm ring-1 ring-gray-900/5 dark:divide-dark-700 dark:!bg-dark-800 dark:ring-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
          <div>
            <strong class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorV2.settings.enableTitle') }}</strong>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
              {{ t('channelMonitorV2.settings.enableHint') }}
            </p>
          </div>
          <Toggle v-model="draft.enabled" />
        </div>
        <div class="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
          <div>
            <strong class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorV2.settings.refreshTitle') }}</strong>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('channelMonitorV2.settings.refreshHint') }}</p>
          </div>
          <div class="tabs inline-flex w-auto" role="group" :aria-label="t('channelMonitorV2.settings.refreshAria')">
            <button
              type="button"
              class="tab"
              :class="draft.refresh_interval_seconds === 60 ? 'tab-active' : ''"
              @click="draft.refresh_interval_seconds = 60"
            >
              {{ t('channelMonitorV2.settings.refreshIntervalMinutes', { minutes: 1 }) }}
            </button>
            <button
              type="button"
              class="tab"
              :class="draft.refresh_interval_seconds === 300 ? 'tab-active' : ''"
              @click="draft.refresh_interval_seconds = 300"
            >
              {{ t('channelMonitorV2.settings.refreshIntervalMinutes', { minutes: 5 }) }}
            </button>
          </div>
        </div>
        <div class="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
          <div>
            <strong class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorV2.settings.cardTimelineTitle') }}</strong>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('channelMonitorV2.settings.cardTimelineHint') }}</p>
          </div>
          <div class="tabs inline-flex w-auto" role="group" :aria-label="t('channelMonitorV2.settings.cardTimelineTitle')" data-testid="monitor-v2-card-timeline">
            <button type="button" class="tab" :class="draft.card_timeline_mode !== 'samples' ? 'tab-active' : ''" data-testid="monitor-v2-timeline-windows" @click="draft.card_timeline_mode = 'windows'">{{ t('channelMonitorV2.cards.timelineMode.windows') }}</button>
            <button type="button" class="tab" :class="draft.card_timeline_mode === 'samples' ? 'tab-active' : ''" data-testid="monitor-v2-timeline-samples" @click="draft.card_timeline_mode = 'samples'">{{ t('channelMonitorV2.cards.timelineMode.samples') }}</button>
          </div>
        </div>
      </div>

      <div class="card overflow-hidden !rounded-lg !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700">
        <div class="card-header !py-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorV2.settings.platformsTitle') }}</h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
            {{ t('channelMonitorV2.settings.platformsHint') }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2 border-b border-gray-100 px-5 py-3 dark:border-dark-700">
          <select v-model="selectedCandidate" class="input min-w-[12rem] flex-1" data-testid="monitor-v2-platform-select"><option value="">{{ t('channelMonitorV2.settings.selectPlatform') }}</option><option v-for="option in addCandidates" :key="option.value" :value="option.value">{{ option.label }}</option></select>
          <input v-model="customPlatform" class="input min-w-[12rem] flex-1" maxlength="64" data-testid="monitor-v2-platform-custom" :placeholder="t('channelMonitorV2.settings.customPlatform')" @keydown.enter.prevent="addPlatform" />
          <button type="button" class="btn btn-secondary btn-sm" data-testid="monitor-v2-add-platform" @click="addPlatform"><Icon name="plus" size="xs" />{{ t('channelMonitorV2.settings.addPlatform') }}</button>
        </div>
        <div class="divide-y divide-gray-100 dark:divide-dark-700">
          <div
            v-for="(platform, index) in draft.platforms"
            :key="platform.platform"
            class="flex flex-wrap items-center gap-3 px-5 py-3"
            data-testid="monitor-v2-platform-row"
            :data-platform="platform.platform"
          >
            <Toggle v-model="platform.enabled" />
            <div class="w-36 min-w-0">
              <strong class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ platformLabel(platform.platform) }}</strong>
              <small class="block truncate font-mono text-[10px] text-gray-400">{{ platform.platform }}</small>
            </div>
            <input
              class="input min-w-[12rem] flex-1"
              :value="platform.models.join(', ')"
              type="text"
              :placeholder="t('channelMonitorV2.settings.modelsPlaceholder')"
              @change="setModels(platform, $event)"
            />
            <span
              class="badge shrink-0"
              :class="platform.models.length ? 'badge-gray' : 'badge badge-primary'"
            >
              {{ platform.models.length ? t('channelMonitorV2.settings.badgeOther') : t('channelMonitorV2.settings.badgeAllModels') }}
            </span>
            <button v-if="!isFactoryPlatform(platform.platform)" type="button" class="icon-btn hover:text-red-500" :disabled="platform.enabled" :title="platform.enabled ? t('channelMonitorV2.settings.deleteDisabled') : t('channelMonitorV2.settings.deletePlatform')" data-testid="monitor-v2-delete-platform" @click="removePlatform(platform.platform)"><Icon name="trash" size="xs" /></button>
            <div class="flex shrink-0 items-center gap-1">
              <button type="button" class="icon-btn" :disabled="index === 0" :title="t('channelMonitorV2.settings.moveUp')" data-testid="monitor-v2-platform-up" @click="movePlatform(index, -1)"><Icon name="arrowUp" size="xs" /></button>
              <button type="button" class="icon-btn" :disabled="index === draft.platforms.length - 1" :title="t('channelMonitorV2.settings.moveDown')" data-testid="monitor-v2-platform-down" @click="movePlatform(index, 1)"><Icon name="arrowDown" size="xs" /></button>
            </div>
          </div>
        </div>
      </div>

      <div class="card overflow-hidden !rounded-lg !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700" data-testid="monitor-v2-group-order">
        <div class="card-header !py-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorV2.settings.groupOrderTitle') }}</h3>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('channelMonitorV2.settings.groupOrderHint') }}</p>
        </div>
        <ol class="divide-y divide-gray-100 dark:divide-dark-700">
          <li v-for="(group, index) in orderedGroups" :key="group.id" class="flex items-center gap-3 px-5 py-2.5 text-sm">
            <span class="w-8 text-right tabular-nums text-gray-400">{{ index + 1 }}</span>
            <span class="min-w-0 flex-1 truncate text-gray-800 dark:text-gray-100">{{ group.name }} <small class="text-gray-400">{{ group.platform }}</small></span>
            <button type="button" class="icon-btn" :disabled="index === 0" :title="t('channelMonitorV2.settings.moveUp')" @click="moveGroup(index, -1)"><Icon name="arrowUp" size="xs" /></button>
            <button type="button" class="icon-btn" :disabled="index === orderedGroups.length - 1" :title="t('channelMonitorV2.settings.moveDown')" @click="moveGroup(index, 1)"><Icon name="arrowDown" size="xs" /></button>
          </li>
          <li v-if="!orderedGroups.length" class="px-5 py-4 text-sm text-gray-400">{{ t('channelMonitorV2.settings.groupsLoading') }}</li>
        </ol>
      </div>

      <MonitorCandySettings
        v-model="draft.candy_probes"
        v-model:history-limit="draft.candy_history_limit"
        :groups="candyGroups"
      />
      <p v-if="!configValid" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200" role="alert">
        {{ t('channelMonitorV2.candy.invalidConfig') }}
      </p>

      <div class="card overflow-hidden !rounded-lg !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700">
        <div class="card-header flex flex-wrap items-center justify-between gap-2 !py-3">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorV2.settings.groupsTitle') }}</h3>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
              {{
                draft.group_ids.length
                  ? t('channelMonitorV2.settings.groupsSelected', { count: draft.group_ids.length })
                  : t('channelMonitorV2.settings.groupsAll')
              }}
            </p>
          </div>
          <button
            v-if="draft.group_ids.length"
            type="button"
            class="btn btn-ghost btn-sm"
            @click="draft.group_ids = []"
          >
            {{ t('channelMonitorV2.settings.groupsAll') }}
          </button>
        </div>
        <div class="max-h-[min(40vh,280px)] overflow-y-auto px-3 py-2 sm:px-4" :aria-busy="groupsLoading">
          <div
            v-if="groupsLoadError"
            class="mb-2 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50/90 px-3 py-2.5 text-sm text-amber-900 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-100"
            role="alert"
          >
            <div class="min-w-0">
              <p class="font-medium">{{ t('channelMonitorV2.settings.groupsLoadFailed') }}</p>
              <p class="mt-0.5 text-xs">{{ t('channelMonitorV2.settings.groupsLoadFailedHint') }}</p>
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-sm shrink-0"
              data-testid="channel-monitor-v2-groups-retry"
              :disabled="groupsLoading"
              @click="loadGroups"
            >
              <Icon name="refresh" size="sm" />
              {{ t('channelMonitorV2.settings.retryGroups') }}
            </button>
          </div>
          <p v-else-if="groupsLoading" class="empty-state py-8 text-sm text-gray-400" aria-live="polite">
            {{ t('channelMonitorV2.settings.groupsLoading') }}
          </p>
          <div v-else class="grid grid-cols-1 gap-1 sm:grid-cols-2">
            <label
              v-for="group in groups"
              :key="group.id"
              class="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition hover:bg-gray-50 dark:hover:bg-dark-800/60"
            >
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500/40"
                :checked="draft.group_ids.includes(group.id)"
                @change="toggleGroup(group.id)"
              />
              <span class="min-w-0 flex-1 truncate font-medium text-gray-800 dark:text-gray-100">{{ group.name }}</span>
              <small class="shrink-0 text-xs text-gray-400">{{ platformLabel(group.platform) }} · {{ t('channelMonitorV2.filters.groupId', { id: group.id }) }}</small>
            </label>
          </div>
          <p v-if="!groupsLoadError && !groupsLoading && groups.length === 0" class="empty-state py-8 text-sm text-gray-400">{{ t('channelMonitorV2.settings.groupsEmpty') }}</p>
        </div>
      </div>

      <div class="card overflow-hidden !rounded-lg !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700">
        <div class="card-header !py-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorV2.settings.errorsTitle') }}</h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
            {{ t('channelMonitorV2.settings.errorsHint') }}
          </p>
        </div>
        <div class="max-h-[min(40vh,320px)] overflow-y-auto px-3 py-2 sm:px-4">
          <div class="grid grid-cols-1 gap-1 sm:grid-cols-2">
            <label
              v-for="category in errorCategories"
              :key="category"
              class="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition hover:bg-gray-50 dark:hover:bg-dark-800/60"
            >
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500/40"
                :checked="isCategoryIgnored(category)"
                @change="toggleIgnoredCategory(category)"
              />
              <span class="min-w-0 flex-1 truncate font-medium text-gray-800 dark:text-gray-100">
                {{ categoryLabel(category) }}
              </span>
              <small class="shrink-0 font-mono text-[10px] text-gray-400">{{ category }}</small>
            </label>
          </div>
        </div>
        <div class="border-t border-gray-100 px-5 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
          {{
            t('channelMonitorV2.settings.ignoredSummary', {
              ignored: draft.ignored_error_categories?.length || 0,
              counted: countedErrorCategoryCount,
            })
          }}
        </div>
      </div>

      <div class="card overflow-hidden !rounded-lg !border-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700">
        <div class="card-header !py-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('channelMonitorV2.settings.healthTitle') }}</h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
            {{ t('channelMonitorV2.settings.healthHint') }}
          </p>
        </div>
        <div class="grid grid-cols-1 gap-4 px-5 py-4 sm:grid-cols-2 lg:grid-cols-4">
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.minimumSample') }}</span>
            <input v-model.number="draft.health_thresholds.minimum_sample" class="input" type="number" min="1" max="10000" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.warningError') }}</span>
            <input v-model.number="warningErrorPercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.criticalError') }}</span>
            <input v-model.number="criticalErrorPercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.targetTtft') }}</span>
            <input v-model.number="draft.health_thresholds.target_ttft_ms" class="input" type="number" min="1" step="100" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.warningTtft') }}</span>
            <input v-model.number="draft.health_thresholds.warning_ttft_ms" class="input" type="number" min="1" step="100" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.criticalTtft') }}</span>
            <input v-model.number="draft.health_thresholds.critical_ttft_ms" class="input" type="number" min="1" step="100" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.warningCache') }}</span>
            <input v-model.number="warningCachePercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.criticalCache') }}</span>
            <input v-model.number="criticalCachePercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
        </div>
      </div>

      <div class="space-y-2">
        <div class="rounded-lg border border-primary-200 bg-primary-50/80 px-4 py-3 text-sm text-primary-900 dark:border-primary-800/50 dark:bg-primary-900/20 dark:text-primary-100">
          <template v-if="namedModelCount === 0">
            {{ t('channelMonitorV2.settings.namedModelsEmpty') }}
          </template>
          <template v-else>
            {{ t('channelMonitorV2.settings.namedModelsCount', { count: namedModelCount }) }}
          </template>
        </div>
        <div class="rounded-lg border border-gray-200 bg-gray-50/80 px-4 py-3 text-xs text-gray-600 dark:border-dark-600 dark:bg-dark-800/50 dark:text-gray-300">
          <p class="font-medium text-gray-800 dark:text-gray-100">{{ t('channelMonitorV2.settings.userContractTitle') }}</p>
          <ul class="mt-1.5 list-disc space-y-0.5 pl-4">
            <li>{{ t('channelMonitorV2.settings.userContract.health') }}</li>
            <li>{{ t('channelMonitorV2.settings.userContract.trend') }}</li>
            <li>{{ t('channelMonitorV2.settings.userContract.latency') }}</li>
            <li>{{ t('channelMonitorV2.settings.userContract.models') }}</li>
          </ul>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import MonitorCandySettings from './MonitorCandySettings.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getChannelMonitorMode, isChannelMonitorV2Mode } from '@/utils/featureFlags'
import { localizedEnumOrUnknown } from '@/utils/localizedEnum'
import { platformLabel as localizedPlatformLabel } from '@/utils/platformColors'
import { GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'
import {
  getConfig,
  updateConfig,
  MONITOR_ERROR_CATEGORIES,
  type MonitorConfig,
} from '@/api/channelMonitorV2'
import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'

const { t, locale } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const draft = ref<MonitorConfig | null>(null)
const original = ref('')
const groups = ref<AdminGroup[]>([])
const configLoadError = ref<string | null>(null)
const groupsLoadError = ref<string | null>(null)
const groupsLoading = ref(false)
const selectedCandidate = ref('')
const customPlatform = ref('')
const factoryPlatforms = ['anthropic', 'openai', 'grok', 'kiro', 'gemini', 'antigravity']
const platformPattern = /^[a-z0-9][a-z0-9_-]{0,63}$/
const candyGroups = computed(() => groups.value.filter((group) => group.status === 'active').map((group) => ({ id: group.id, name: group.name, platform: group.platform || '' })))
const orderedGroups = computed(() => {
  const order = draft.value?.group_order || []
  const byId = new Map(groups.value.map((group) => [group.id, group]))
  const listed = order.map((id) => byId.get(id)).filter((group): group is AdminGroup => Boolean(group))
  const seen = new Set(listed.map((group) => group.id))
  return [...listed, ...groups.value.filter((group) => !seen.has(group.id)).sort((a, b) => a.name.localeCompare(b.name))]
})

const dirty = computed(() => (draft.value ? JSON.stringify(draft.value) !== original.value : false))
const configValid = computed(() => {
  const probes = draft.value?.candy_probes || []
  if (!draft.value || probes.length > 64) return false
  if (draft.value.candy_history_limit != null && (!Number.isInteger(draft.value.candy_history_limit) || draft.value.candy_history_limit < 15 || draft.value.candy_history_limit > 100)) return false
  const knownGroups = new Map(groups.value.map((group) => [group.id, group]))
  const seen = new Set<string>()
  for (const probe of probes) {
    const group = knownGroups.get(probe.group_id)
    if (!group || (probe.enabled && group.status !== 'active') || !probe.model.trim() || !Number.isInteger(probe.interval_minutes) || probe.interval_minutes < 1 || probe.interval_minutes > 1440) return false
    const id = `${probe.group_id}\0${probe.model.trim().toLowerCase()}\0${(probe.reasoning_effort || 'medium').trim().toLowerCase()}`
    if (seen.has(id)) return false
    seen.add(id)
  }
  return true
})
const namedModelCount = computed(
  () => draft.value?.platforms.filter((p) => p.enabled).reduce((sum, p) => sum + p.models.length, 0) || 0
)
const addCandidates = computed(() => {
  const configured = new Set(draft.value?.platforms.map((item) => item.platform.trim().toLowerCase()) || [])
  const seen = new Set<string>()
  const options: Array<{ value: string; label: string }> = []
  const add = (value: string, label?: string) => {
    const id = value.trim().toLowerCase()
    if (!id || configured.has(id) || seen.has(id) || !platformPattern.test(id)) return
    seen.add(id)
    options.push({ value: id, label: label || platformLabel(id) })
  }
  for (const item of GROUP_PLATFORM_OPTIONS) add(item.value, item.label)
  for (const id of factoryPlatforms) add(id)
  for (const group of groups.value) add(group.platform)
  return options
})
const errorCategories = MONITOR_ERROR_CATEGORIES
const countedErrorCategoryCount = computed(
  () => errorCategories.length - (draft.value?.ignored_error_categories?.length || 0)
)
/** System settings mode must be v2 for aggregation to run; config remains editable for prep. */
const systemModeV2 = computed(() => isChannelMonitorV2Mode())
const systemModeLabel = computed(() => {
  if (!appStore.cachedPublicSettings?.channel_monitor_enabled) {
    return t('channelMonitorV2.settings.modeClosed')
  }
  return getChannelMonitorMode() === 'v1'
    ? t('channelMonitorV2.settings.modeV1')
    : t('channelMonitorV2.settings.modeV2')
})
const defaultThresholds = {
  minimum_sample: 50,
  warning_error_rate: 0.05,
  critical_error_rate: 0.20,
  target_ttft_ms: 3000,
  warning_ttft_ms: 3000,
  critical_ttft_ms: 10000,
  // Higher is better: below 85% watch, below 60% critical.
  warning_cache_rate: 0.85,
  critical_cache_rate: 0.60,
  error_weight: 0.60,
  ttft_weight: 0.20,
  cache_weight: 0.20,
}

/** Factory ignored categories (matches backend DefaultChannelMonitorV2IgnoredErrorCategories). */
const defaultIgnoredErrorCategories = [
  'authentication',
  'client_cancelled',
  'content_policy',
  'context_limit',
  'group_access',
  'model_unsupported',
  'not_found',
  'quota_or_balance',
] as const
function percentModel(key: 'warning_error_rate' | 'critical_error_rate' | 'warning_cache_rate' | 'critical_cache_rate') {
  return computed({
    get: () => ((draft.value?.health_thresholds?.[key] ?? defaultThresholds[key]) * 100),
    set: (value: number) => {
      if (!draft.value) return
      draft.value.health_thresholds[key] = Math.max(0, Math.min(100, Number(value) || 0)) / 100
    },
  })
}
const warningErrorPercent = percentModel('warning_error_rate')
const criticalErrorPercent = percentModel('critical_error_rate')
const warningCachePercent = percentModel('warning_cache_rate')
const criticalCachePercent = percentModel('critical_cache_rate')

function setModels(platform: MonitorConfig['platforms'][number], event: Event) {
  platform.models = [
    ...new Set(
      (event.target as HTMLInputElement).value
        .split(',')
        .map((v) => v.trim())
        .filter(Boolean)
    ),
  ].sort()
}

function toggleGroup(id: number) {
  if (!draft.value) return
  draft.value.group_ids = draft.value.group_ids.includes(id)
    ? draft.value.group_ids.filter((value) => value !== id)
    : [...draft.value.group_ids, id].sort((a, b) => a - b)
}

function moveGroup(index: number, direction: -1 | 1) {
  if (!draft.value) return
  const ids = orderedGroups.value.map((group) => group.id)
  const target = index + direction
  if (target < 0 || target >= ids.length) return
  ;[ids[index], ids[target]] = [ids[target], ids[index]]
  draft.value.group_order = ids
}

function movePlatform(index: number, direction: -1 | 1) {
  if (!draft.value) return
  const target = index + direction
  if (target < 0 || target >= draft.value.platforms.length) return
  const platforms = [...draft.value.platforms]
  ;[platforms[index], platforms[target]] = [platforms[target], platforms[index]]
  draft.value.platforms = platforms
}

function isFactoryPlatform(value: string) { return factoryPlatforms.includes(value.trim().toLowerCase()) }
function addPlatform() {
  if (!draft.value) return
  const id = (customPlatform.value.trim() || selectedCandidate.value).trim().toLowerCase()
  if (!platformPattern.test(id) || draft.value.platforms.length >= 64 || draft.value.platforms.some((item) => item.platform.toLowerCase() === id)) return
  draft.value.platforms.push({ platform: id, enabled: true, models: [] })
  selectedCandidate.value = ''
  customPlatform.value = ''
}
function removePlatform(id: string) {
  if (!draft.value || isFactoryPlatform(id)) return
  const item = draft.value.platforms.find((platform) => platform.platform === id)
  if (!item || item.enabled) return
  draft.value.platforms = draft.value.platforms.filter((platform) => platform.platform !== id)
}

function isCategoryIgnored(category: string): boolean {
  return Boolean(draft.value?.ignored_error_categories?.includes(category))
}

function toggleIgnoredCategory(category: string) {
  if (!draft.value) return
  const current = new Set(draft.value.ignored_error_categories || [])
  if (current.has(category)) current.delete(category)
  else current.add(category)
  draft.value.ignored_error_categories = [...current].sort()
}

function categoryLabel(category: string) {
  return localizedEnumOrUnknown(t, `channelMonitorV2.errorCategories.${category}`)
}

function platformLabel(value: string) {
 return localizedPlatformLabel(value, locale.value)
}

function normalizeConfig(value: MonitorConfig): MonitorConfig {
  const ignored = value.ignored_error_categories
  return {
    ...value,
    card_timeline_mode: value.card_timeline_mode === 'samples' ? 'samples' : 'windows',
    candy_probes: value.candy_probes ?? [],
    candy_history_limit: Math.min(100, Math.max(15, Math.trunc(value.candy_history_limit || 100))),
    group_order: value.group_order ?? [],
    health_thresholds: { ...defaultThresholds, ...(value.health_thresholds || {}) },
    // Preserve explicit empty arrays from the server (operator cleared all).
    ignored_error_categories: [
      ...(ignored == null ? [...defaultIgnoredErrorCategories] : ignored),
    ].sort(),
  }
}

async function load() {
  loading.value = true
  configLoadError.value = null
  try {
    const value = await getConfig()
    const normalized = normalizeConfig(value)
    draft.value = structuredClone(normalized)
    original.value = JSON.stringify(normalized)
    void loadGroups()
  } catch (error) {
    configLoadError.value = extractApiErrorMessage(error, t('channelMonitorV2.settings.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadGroups() {
  groupsLoading.value = true
  groupsLoadError.value = null
  try {
    groups.value = await adminAPI.groups.getAllIncludingInactive()
  } catch (error) {
    groupsLoadError.value = extractApiErrorMessage(error, t('channelMonitorV2.settings.groupsLoadFailed'))
  } finally {
    groupsLoading.value = false
  }
}

async function save() {
  if (!draft.value) return
  saving.value = true
  try {
    const payload = normalizeConfig(draft.value)
    const value = await updateConfig(payload)
    const normalized = normalizeConfig(value)
    draft.value = structuredClone(normalized)
    original.value = JSON.stringify(normalized)
    appStore.showSuccess(t('channelMonitorV2.settings.saveSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.settings.saveFailed')))
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
