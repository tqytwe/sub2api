<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import {
  consumeInternalSpeedTestPayload,
  fetchSpeedTestModels,
  normalizeSpeedTestBaseUrl,
  runSpeedTestChatCompletion,
  type SpeedTestModel,
} from '@/utils/internalSpeedTest'

type RunStatus = 'queued' | 'running' | 'success' | 'error' | 'cancelled'

interface RunState {
  id: number
  prompt: string
  status: RunStatus
  outputText: string
  firstTokenLatencyMs: number | null
  totalMs: number | null
  outputTokens: number
  tokensPerSecond: number
  error: string
}

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const hasPayload = ref(false)
const keyName = ref('')
const apiKey = ref('')
const baseUrl = ref('')
const models = ref<SpeedTestModel[]>([])
const selectedModel = ref('')
const loadingModels = ref(false)
const modelsError = ref('')
const running = ref(false)
const testCount = ref(5)
const runs = ref<RunState[]>([])

let abortController: AbortController | null = null

const countOptions = [1, 3, 5]
const promptKeys = [
  'keys.speedTest.prompts.concise',
  'keys.speedTest.prompts.reasoning',
  'keys.speedTest.prompts.translation',
  'keys.speedTest.prompts.summary',
  'keys.speedTest.prompts.code',
]

const normalizedBaseUrl = computed(() => normalizeSpeedTestBaseUrl(baseUrl.value))
const completedRuns = computed(() => runs.value.filter((run) => run.status === 'success'))
const averageFirstTokenMs = computed(() => {
  const values = completedRuns.value
    .map((run) => run.firstTokenLatencyMs)
    .filter((value): value is number => typeof value === 'number')
  if (!values.length) return null
  return values.reduce((sum, value) => sum + value, 0) / values.length
})
const averageRate = computed(() => {
  const values = completedRuns.value.map((run) => run.tokensPerSecond).filter((value) => value > 0)
  if (!values.length) return null
  return values.reduce((sum, value) => sum + value, 0) / values.length
})
const totalOutputTokens = computed(() => completedRuns.value.reduce((sum, run) => sum + run.outputTokens, 0))
const canStart = computed(() =>
  hasPayload.value &&
  !running.value &&
  !loadingModels.value &&
  Boolean(selectedModel.value) &&
  !modelsError.value
)

function formatMs(value: number | null): string {
  if (value === null || Number.isNaN(value)) return '-'
  return `${Math.round(value)} ms`
}

function formatRate(value: number | null): string {
  if (value === null || Number.isNaN(value)) return '-'
  return `${value.toFixed(2)} ${t('keys.speedTest.tokensPerSecond')}`
}

function statusLabel(status: RunStatus): string {
  return t(`keys.speedTest.status.${status}`)
}

function statusClasses(status: RunStatus): string {
  switch (status) {
    case 'success':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'error':
      return 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
    case 'cancelled':
      return 'border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300'
    case 'running':
      return 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-300'
    default:
      return 'border-gray-200 bg-white text-gray-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-400'
  }
}

function describeError(error: unknown): string {
  if (error instanceof DOMException && error.name === 'AbortError') {
    return t('keys.speedTest.cancelled')
  }
  return error instanceof Error && error.message ? error.message : t('common.error')
}

async function loadModels() {
  if (!apiKey.value || !baseUrl.value) return
  abortController?.abort()
  abortController = new AbortController()
  loadingModels.value = true
  modelsError.value = ''
  try {
    const items = await fetchSpeedTestModels({
      apiKey: apiKey.value,
      baseUrl: baseUrl.value,
      signal: abortController.signal,
    })
    models.value = items
    selectedModel.value = items[0]?.id ?? ''
    if (!items.length) {
      modelsError.value = t('keys.speedTest.noModels')
    }
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    models.value = []
    selectedModel.value = ''
    modelsError.value = describeError(error)
    appStore.showError(t('keys.speedTest.loadModelsFailed'))
  } finally {
    loadingModels.value = false
  }
}

function buildRuns(): RunState[] {
  const prompts = promptKeys.map((key) => t(key))
  return Array.from({ length: testCount.value }, (_, index) => ({
    id: index + 1,
    prompt: prompts[index % prompts.length],
    status: 'queued',
    outputText: '',
    firstTokenLatencyMs: null,
    totalMs: null,
    outputTokens: 0,
    tokensPerSecond: 0,
    error: '',
  }))
}

async function startTests() {
  if (!canStart.value) return
  abortController?.abort()
  abortController = new AbortController()
  runs.value = buildRuns()
  running.value = true

  try {
    for (const run of runs.value) {
      if (abortController.signal.aborted) {
        run.status = 'cancelled'
        continue
      }

      run.status = 'running'
      try {
        const result = await runSpeedTestChatCompletion({
          apiKey: apiKey.value,
          baseUrl: baseUrl.value,
          model: selectedModel.value,
          prompt: run.prompt,
          signal: abortController.signal,
          onDelta: (content) => {
            run.outputText += content
          },
        })
        run.outputText = result.outputText
        run.firstTokenLatencyMs = result.firstTokenLatencyMs
        run.totalMs = result.totalMs
        run.outputTokens = result.outputTokens
        run.tokensPerSecond = result.tokensPerSecond
        run.status = 'success'
      } catch (error) {
        run.error = describeError(error)
        run.status = abortController.signal.aborted ? 'cancelled' : 'error'
        if (abortController.signal.aborted) break
      }
    }
  } finally {
    running.value = false
  }
}

function cancelTests() {
  abortController?.abort()
  for (const run of runs.value) {
    if (run.status === 'running' || run.status === 'queued') {
      run.status = 'cancelled'
    }
  }
  running.value = false
}

function goBackToKeys() {
  apiKey.value = ''
  router.push('/keys')
}

onMounted(() => {
  const payload = consumeInternalSpeedTestPayload()
  if (!payload) {
    hasPayload.value = false
    return
  }

  hasPayload.value = true
  apiKey.value = payload.apiKey
  baseUrl.value = payload.baseUrl
  keyName.value = payload.keyName || t('keys.speedTest.currentKey')
  void loadModels()
})

onBeforeUnmount(() => {
  abortController?.abort()
  apiKey.value = ''
})
</script>

<template>
  <AppLayout>
    <div class="space-y-6">
      <section
        v-if="!hasPayload"
        class="rounded-lg border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div class="flex min-w-0 items-start gap-3">
            <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
              <Icon name="key" size="md" :stroke-width="2" />
            </span>
            <div class="min-w-0">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('keys.speedTest.missing.title') }}
              </h2>
              <p class="mt-1 text-sm leading-5 text-gray-600 dark:text-gray-400">
                {{ t('keys.speedTest.missing.description') }}
              </p>
            </div>
          </div>
          <button type="button" class="btn btn-primary flex-shrink-0" @click="goBackToKeys">
            <Icon name="arrowLeft" size="sm" class="mr-1.5" />
            {{ t('keys.speedTest.backToKeys') }}
          </button>
        </div>
      </section>

      <template v-else>
        <section class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div class="flex min-w-0 items-start gap-3">
              <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-blue-600 text-white dark:bg-blue-500">
                <Icon name="bolt" size="md" :stroke-width="2" />
              </span>
              <div class="min-w-0">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('keys.speedTest.title') }}
                </h2>
                <p class="mt-1 text-sm leading-5 text-gray-600 dark:text-gray-400">
                  {{ t('keys.speedTest.description') }}
                </p>
              </div>
            </div>
            <button type="button" class="btn btn-secondary flex-shrink-0" @click="goBackToKeys">
              <Icon name="arrowLeft" size="sm" class="mr-1.5" />
              {{ t('keys.speedTest.backToKeys') }}
            </button>
          </div>

          <div class="mt-5 grid gap-4 md:grid-cols-3">
            <label class="block">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-200">
                {{ t('keys.speedTest.modelLabel') }}
              </span>
              <select
                v-model="selectedModel"
                class="mt-1 block h-10 w-full rounded-lg border border-gray-300 bg-white px-3 text-sm text-gray-900 focus-visible:border-primary-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 disabled:cursor-not-allowed disabled:bg-gray-100 dark:border-dark-600 dark:bg-dark-900 dark:text-white dark:disabled:bg-dark-800"
                :disabled="loadingModels || running || models.length === 0"
              >
                <option v-if="loadingModels" value="">{{ t('keys.speedTest.loadingModels') }}</option>
                <option v-for="model in models" :key="model.id" :value="model.id">{{ model.id }}</option>
              </select>
            </label>

            <label class="block">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-200">
                {{ t('keys.speedTest.countLabel') }}
              </span>
              <select
                v-model.number="testCount"
                class="mt-1 block h-10 w-full rounded-lg border border-gray-300 bg-white px-3 text-sm text-gray-900 focus-visible:border-primary-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 disabled:cursor-not-allowed disabled:bg-gray-100 dark:border-dark-600 dark:bg-dark-900 dark:text-white dark:disabled:bg-dark-800"
                :disabled="running"
              >
                <option v-for="count in countOptions" :key="count" :value="count">
                  {{ t('keys.speedTest.countOption', { count }) }}
                </option>
              </select>
            </label>

            <div class="min-w-0">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-200">
                {{ t('keys.speedTest.keyLabel') }}
              </p>
              <p class="mt-1 truncate rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200">
                {{ keyName }}
              </p>
            </div>
          </div>

          <div class="mt-4 min-w-0 rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
            <span class="font-medium">{{ t('keys.speedTest.baseUrlLabel') }}</span>
            <code class="ml-1 break-all font-mono">{{ normalizedBaseUrl }}</code>
          </div>

          <p v-if="modelsError" class="mt-3 text-sm text-red-600 dark:text-red-300">
            {{ modelsError }}
          </p>

          <div class="mt-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-end">
            <button
              type="button"
              class="btn btn-secondary justify-center"
              :disabled="loadingModels || running"
              @click="loadModels"
            >
              <Icon name="refresh" size="sm" class="mr-1.5" />
              {{ t('keys.speedTest.reloadModels') }}
            </button>
            <button
              type="button"
              class="btn btn-primary justify-center"
              :disabled="!canStart"
              @click="startTests"
            >
              <Icon name="play" size="sm" class="mr-1.5" />
              {{ t('keys.speedTest.start') }}
            </button>
            <button
              v-if="running"
              type="button"
              class="btn btn-secondary justify-center"
              @click="cancelTests"
            >
              <Icon name="x" size="sm" class="mr-1.5" />
              {{ t('keys.speedTest.cancel') }}
            </button>
          </div>
        </section>

        <section class="grid gap-4 md:grid-cols-3">
          <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('keys.speedTest.avgFirstToken') }}</p>
            <p class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
              {{ formatMs(averageFirstTokenMs) }}
            </p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('keys.speedTest.avgRate') }}</p>
            <p class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
              {{ formatRate(averageRate) }}
            </p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('keys.speedTest.totalOutputTokens') }}</p>
            <p class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
              {{ totalOutputTokens }}
            </p>
          </div>
        </section>

        <section class="space-y-3">
          <div class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('keys.speedTest.resultsTitle') }}
              </h2>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                {{ t('keys.speedTest.resultsDescription') }}
              </p>
            </div>
          </div>

          <div
            v-if="runs.length === 0"
            class="rounded-lg border border-dashed border-gray-300 bg-white p-6 text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400"
          >
            {{ t('keys.speedTest.emptyResults') }}
          </div>

          <article
            v-for="run in runs"
            :key="run.id"
            class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800"
          >
            <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('keys.speedTest.promptLabel', { index: run.id }) }}
                  </span>
                  <span
                    class="inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium"
                    :class="statusClasses(run.status)"
                  >
                    {{ statusLabel(run.status) }}
                  </span>
                </div>
                <p class="mt-1 text-sm leading-5 text-gray-600 dark:text-gray-400">
                  {{ run.prompt }}
                </p>
              </div>
              <dl class="grid flex-shrink-0 grid-cols-3 gap-3 text-right text-sm">
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('keys.speedTest.firstToken') }}</dt>
                  <dd class="font-medium tabular-nums text-gray-900 dark:text-white">{{ formatMs(run.firstTokenLatencyMs) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('keys.speedTest.totalTime') }}</dt>
                  <dd class="font-medium tabular-nums text-gray-900 dark:text-white">{{ formatMs(run.totalMs) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('keys.speedTest.rate') }}</dt>
                  <dd class="font-medium tabular-nums text-gray-900 dark:text-white">{{ formatRate(run.tokensPerSecond || null) }}</dd>
                </div>
              </dl>
            </div>

            <p v-if="run.error" class="mt-3 text-sm text-red-600 dark:text-red-300">
              {{ run.error }}
            </p>

            <details v-if="run.outputText" class="mt-3">
              <summary class="cursor-pointer text-sm font-medium text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-primary-300">
                {{ t('keys.speedTest.output') }}
              </summary>
              <pre class="mt-2 overflow-x-auto whitespace-pre-wrap rounded-lg bg-gray-950 p-3 text-sm leading-5 text-gray-100"><code>{{ run.outputText }}</code></pre>
            </details>
          </article>
        </section>
      </template>
    </div>
  </AppLayout>
</template>
