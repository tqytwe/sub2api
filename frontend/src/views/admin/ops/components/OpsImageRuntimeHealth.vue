<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  opsAPI,
  type OpsImageRuntimeComponentHealth,
  type OpsImageRuntimesHealth
} from '@/api/admin/ops'
import Icon from '@/components/icons/Icon.vue'

type RuntimeKey = 'gateway_async' | 'batch' | 'image_studio'
type RuntimeState = 'ready' | 'draining' | 'disabled' | 'not-ready'
type RuntimeCheck = {
  key: string
  label: string
  ready: boolean
}

const props = withDefaults(defineProps<{
  refreshToken?: number
}>(), {
  refreshToken: 0
})

const { t } = useI18n()
const loading = ref(false)
const errorMessage = ref('')
const health = ref<OpsImageRuntimesHealth | null>(null)

const runtimeDefinitions: Array<{
  key: RuntimeKey
  titleKey: string
  icon: 'server' | 'cube' | 'sparkles'
}> = [
  {
    key: 'gateway_async',
    titleKey: 'admin.ops.imageRuntimes.names.gatewayAsync',
    icon: 'server'
  },
  {
    key: 'batch',
    titleKey: 'admin.ops.imageRuntimes.names.batch',
    icon: 'cube'
  },
  {
    key: 'image_studio',
    titleKey: 'admin.ops.imageRuntimes.names.imageStudio',
    icon: 'sparkles'
  }
]

const runtimes = computed(() => {
  if (!health.value) return []
  return runtimeDefinitions.map((definition) => ({
    ...definition,
    health: health.value![definition.key]
  }))
})

function runtimeState(key: RuntimeKey, runtime: OpsImageRuntimeComponentHealth): RuntimeState {
  if (runtime.ready) return 'ready'
  if (key !== 'image_studio' && !runtime.enabled && runtime.queue_enabled) {
    const storageReady = key === 'batch' ? runtime.database_ready : runtime.storage_ready
    if (storageReady && runtime.redis_ready && runtime.worker_running) return 'draining'
    return 'not-ready'
  }
  if (!runtime.enabled) return 'disabled'
  return 'not-ready'
}

function runtimeStateLabel(state: RuntimeState): string {
  const keys: Record<RuntimeState, string> = {
    ready: 'admin.ops.imageRuntimes.states.ready',
    draining: 'admin.ops.imageRuntimes.states.draining',
    disabled: 'admin.ops.imageRuntimes.states.disabled',
    'not-ready': 'admin.ops.imageRuntimes.states.notReady'
  }
  return t(keys[state])
}

function runtimeStateClass(state: RuntimeState): string {
  const classes: Record<RuntimeState, string> = {
    ready: 'text-emerald-700 dark:text-emerald-300',
    draining: 'text-amber-700 dark:text-amber-300',
    disabled: 'text-gray-500 dark:text-gray-400',
    'not-ready': 'text-red-700 dark:text-red-300'
  }
  return classes[state]
}

function statusIcon(ready: boolean): 'checkCircle' | 'xCircle' {
  return ready ? 'checkCircle' : 'xCircle'
}

function statusIconClass(ready: boolean): string {
  return ready
    ? 'text-emerald-600 dark:text-emerald-400'
    : 'text-red-500 dark:text-red-400'
}

function runtimeChecks(key: RuntimeKey, runtime: OpsImageRuntimeComponentHealth): RuntimeCheck[] {
  const checks: RuntimeCheck[] = [
    {
      key: 'api',
      label: t('admin.ops.imageRuntimes.checks.api'),
      ready: runtime.enabled
    },
    {
      key: 'queue',
      label: t('admin.ops.imageRuntimes.checks.queue'),
      ready: runtime.queue_enabled
    }
  ]
  if (key === 'gateway_async') {
    checks.push(
      {
        key: 'storage',
        label: t('admin.ops.imageRuntimes.checks.storage'),
        ready: runtime.storage_ready
      },
      {
        key: 'redis',
        label: t('admin.ops.imageRuntimes.checks.redis'),
        ready: runtime.redis_ready
      }
    )
  } else if (key === 'batch') {
    checks.push(
      {
        key: 'database',
        label: t('admin.ops.imageRuntimes.checks.database'),
        ready: runtime.database_ready
      },
      {
        key: 'redis',
        label: t('admin.ops.imageRuntimes.checks.redis'),
        ready: runtime.redis_ready
      }
    )
  } else {
    checks.push(
      {
        key: 'storage',
        label: t('admin.ops.imageRuntimes.checks.storage'),
        ready: runtime.storage_ready
      },
      {
        key: 'database',
        label: t('admin.ops.imageRuntimes.checks.database'),
        ready: runtime.database_ready
      }
    )
  }
  checks.push({
    key: 'worker',
    label: t('admin.ops.imageRuntimes.checks.worker'),
    ready: runtime.worker_running
  })
  return checks
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function hasBatchMetrics(runtime: OpsImageRuntimeComponentHealth): boolean {
  return [
    runtime.stale_balance_holds,
    runtime.settlement_retries,
    runtime.provider_failures,
    runtime.result_cleanup_pending
  ].some((value) => Number(value || 0) > 0)
}

async function fetchHealth() {
  loading.value = true
  errorMessage.value = ''
  try {
    health.value = await opsAPI.getImageRuntimesHealth()
  } catch (err) {
    console.error('[OpsImageRuntimeHealth] Failed to load image runtime health', err)
    errorMessage.value = t('admin.ops.imageRuntimes.loadFailed')
  } finally {
    loading.value = false
  }
}

watch(
  () => props.refreshToken,
  (next, previous) => {
    if (next !== previous) void fetchHealth()
  }
)

onMounted(fetchHealth)
</script>

<template>
  <section class="border-y border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
    <header class="flex min-h-14 items-center justify-between gap-4 px-4 py-3 sm:px-5">
      <div class="min-w-0">
        <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.ops.imageRuntimes.title') }}
        </h2>
        <p v-if="health?.checked_at" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.imageRuntimes.checkedAt', { time: formatDate(health.checked_at) }) }}
        </p>
      </div>
      <button
        type="button"
        class="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-gray-200 text-gray-600 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-dark-800"
        :disabled="loading"
        :title="t('admin.ops.imageRuntimes.refresh')"
        :aria-label="t('admin.ops.imageRuntimes.refresh')"
        data-test="image-runtimes-refresh"
        @click="fetchHealth"
      >
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
      </button>
    </header>

    <div
      v-if="errorMessage"
      data-test="image-runtimes-error"
      class="border-t border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-300"
    >
      {{ errorMessage }}
    </div>

    <div
      v-if="health"
      class="grid grid-cols-1 divide-y divide-gray-200 border-t border-gray-200 dark:divide-dark-700 dark:border-dark-700 lg:grid-cols-3 lg:divide-x lg:divide-y-0"
    >
      <article
        v-for="runtime in runtimes"
        :key="runtime.key"
        :data-test="`image-runtime-${runtime.key}`"
        :data-state="runtimeState(runtime.key, runtime.health)"
        class="min-w-0 px-4 py-4 sm:px-5"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="flex min-w-0 items-center gap-2.5">
            <Icon :name="runtime.icon" size="md" class="shrink-0 text-gray-500 dark:text-gray-400" />
            <h3 class="truncate text-sm font-medium text-gray-900 dark:text-white">
              {{ t(runtime.titleKey) }}
            </h3>
          </div>
          <span
            class="flex shrink-0 items-center gap-1.5 text-xs font-medium"
            :class="runtimeStateClass(runtimeState(runtime.key, runtime.health))"
          >
            <span class="h-1.5 w-1.5 rounded-full bg-current" />
            {{ runtimeStateLabel(runtimeState(runtime.key, runtime.health)) }}
          </span>
        </div>

        <div class="mt-3 break-all font-mono text-[11px] text-gray-500 dark:text-gray-400">
          {{ runtime.health.storage }} · {{ runtime.health.queue }}
        </div>

        <dl class="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
          <div
            v-for="check in runtimeChecks(runtime.key, runtime.health)"
            :key="check.key"
            class="flex min-w-0 items-center justify-between gap-2"
          >
            <dt class="text-gray-500 dark:text-gray-400">{{ check.label }}</dt>
            <dd>
              <Icon
                :name="statusIcon(check.ready)"
                size="sm"
                :class="statusIconClass(check.ready)"
              />
            </dd>
          </div>
        </dl>

        <div class="mt-4 grid grid-cols-3 border-y border-gray-100 py-3 text-center dark:border-dark-800">
          <div>
            <div class="text-base font-semibold text-gray-900 dark:text-white">{{ runtime.health.backlog.ready }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.imageRuntimes.backlog.ready') }}</div>
          </div>
          <div>
            <div class="text-base font-semibold text-gray-900 dark:text-white">{{ runtime.health.backlog.delayed }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.imageRuntimes.backlog.delayed') }}</div>
          </div>
          <div>
            <div class="text-base font-semibold text-gray-900 dark:text-white">{{ runtime.health.backlog.active }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.imageRuntimes.backlog.active') }}</div>
          </div>
        </div>

        <div v-if="runtime.health.oldest_task" class="mt-3 text-xs">
          <div class="text-gray-500 dark:text-gray-400">{{ t('admin.ops.imageRuntimes.oldestTask') }}</div>
          <div class="mt-1 break-all font-mono text-gray-800 dark:text-gray-200">
            {{ runtime.health.oldest_task.id }}
          </div>
          <div class="mt-0.5 text-gray-500 dark:text-gray-400">
            {{ runtime.health.oldest_task.status }} · {{ formatDate(runtime.health.oldest_task.created_at) }}
          </div>
        </div>

        <div
          v-if="runtime.health.recent_error"
          class="mt-3 border-l-2 border-red-400 pl-3 text-xs text-red-700 dark:text-red-300"
        >
          <div v-if="runtime.health.recent_error.code" class="break-all font-mono font-medium">
            {{ runtime.health.recent_error.code }}
          </div>
          <div class="mt-0.5 break-words">{{ runtime.health.recent_error.message }}</div>
          <div class="mt-0.5 text-red-500 dark:text-red-400">
            {{ formatDate(runtime.health.recent_error.created_at) }}
          </div>
        </div>

        <dl
          v-if="hasBatchMetrics(runtime.health)"
          class="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 border-t border-gray-100 pt-3 text-xs dark:border-dark-800"
        >
          <div>
            <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.ops.imageRuntimes.metrics.staleHolds') }}</dt>
            <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ runtime.health.stale_balance_holds || 0 }}</dd>
          </div>
          <div>
            <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.ops.imageRuntimes.metrics.settlementRetries') }}</dt>
            <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ runtime.health.settlement_retries || 0 }}</dd>
          </div>
          <div>
            <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.ops.imageRuntimes.metrics.providerFailures') }}</dt>
            <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ runtime.health.provider_failures || 0 }}</dd>
          </div>
          <div>
            <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.ops.imageRuntimes.metrics.cleanupPending') }}</dt>
            <dd class="mt-0.5 font-medium text-gray-900 dark:text-white">{{ runtime.health.result_cleanup_pending || 0 }}</dd>
          </div>
        </dl>
      </article>
    </div>

    <div
      v-else-if="loading"
      class="grid min-h-40 grid-cols-1 animate-pulse border-t border-gray-200 dark:border-dark-700 lg:grid-cols-3"
    >
      <div v-for="index in 3" :key="index" class="space-y-4 border-gray-200 px-5 py-5 dark:border-dark-700 lg:border-r lg:last:border-r-0">
        <div class="h-4 w-36 rounded bg-gray-200 dark:bg-dark-700" />
        <div class="h-16 rounded bg-gray-100 dark:bg-dark-800" />
      </div>
    </div>
  </section>
</template>
