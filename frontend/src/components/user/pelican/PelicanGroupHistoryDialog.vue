<template>
  <BaseDialog :show="show" :title="t('pelicanShowcase.history.title', { group: groupName })" width="full" content-class="h-[90dvh]" body-class="flex min-h-0 flex-col !overflow-hidden" @close="emit('close')">
    <div class="grid min-h-0 flex-1 gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]" data-testid="pelican-group-history">
      <section class="flex min-h-0 min-w-0 flex-col gap-3 border-b border-gray-200 pb-3 lg:border-b-0 lg:border-r lg:pb-0 lg:pr-4 dark:border-dark-700">
        <label class="field-label mt-0" for="pelican-history-model">{{ t('pelicanShowcase.history.model') }}</label>
        <select id="pelican-history-model" v-model="selectedModel" class="input w-full" :disabled="loading || !models.length" data-testid="pelican-history-model">
          <option v-for="model in models" :key="model" :value="model">{{ model }}</option>
        </select>
        <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('pelicanShowcase.history.hint') }}</p>
        <div v-if="loading" class="space-y-2" role="status" aria-live="polite">
        <div v-for="i in 4" :key="i" class="h-16 rounded-md bg-gray-100 dark:bg-dark-800" />
        </div>
        <p v-else-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }} <button type="button" class="underline" @click="load">{{ t('channelMonitorV2.pelican.retry') }}</button></p>
        <p v-else-if="!filteredItems.length" class="text-sm text-gray-500 dark:text-gray-400" data-testid="pelican-history-empty">{{ t('pelicanShowcase.history.empty') }}</p>
        <ol v-else class="min-h-0 space-y-2 overflow-y-auto" :aria-label="t('pelicanShowcase.history.runs')">
          <li v-for="item in filteredItems" :key="item.id">
            <button
              type="button"
              class="w-full min-w-0 rounded-md border px-3 py-2 text-left focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
              :class="selectedItem?.id === item.id ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20' : 'border-gray-200 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800'"
              :data-result-id="item.id"
              @click="selectItem(item)"
            >
              <span class="flex items-center justify-between gap-2 text-xs">
                <span class="font-medium text-gray-900 dark:text-white">{{ dateLabel(item.generated_at) }}</span>
                <span class="text-gray-500 dark:text-gray-400">{{ durationLabel(item.latency_ms) }}</span>
              </span>
              <span class="mt-1 block text-[11px] text-gray-500 dark:text-gray-400">{{ item.model_id }} · {{ item.reasoning_effort }}</span>
            </button>
          </li>
        </ol>
      </section>

      <section class="flex min-h-0 flex-col" aria-live="polite" :aria-busy="previewLoading">
        <div v-if="selectedItem" class="mb-3 flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span>{{ t('pelicanShowcase.history.selected', { model: selectedItem.model_id }) }} · {{ dateLabel(selectedItem.generated_at) }}</span>
          <span>{{ t('pelicanShowcase.history.duration', { duration: durationLabel(selectedItem.latency_ms) }) }}</span>
        </div>
        <div class="min-h-0 flex-1 overflow-hidden rounded-md border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-900" data-testid="pelican-preview-stage">
          <div v-if="previewLoading" class="flex h-full items-center justify-center text-sm text-gray-500" role="status">{{ t('pelicanShowcase.history.previewLoading') }}</div>
          <p v-else-if="previewError" role="alert" class="p-4 text-sm text-red-600 dark:text-red-400">{{ previewError }}</p>
          <PelicanArtworkPreview v-else-if="previewHtml" :html="previewHtml" :title="t('pelicanShowcase.history.previewTitle', { model: selectedItem?.model_id || '' })" />
          <p v-else class="flex h-full items-center justify-center p-4 text-sm text-gray-500 dark:text-gray-400">{{ t('pelicanShowcase.history.chooseRun') }}</p>
        </div>
      </section>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getShowcase, getShowcaseItem, type PelicanShowcaseItem } from '@/api/pelicanShowcase'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PelicanArtworkPreview from './PelicanArtworkPreview.vue'

const props = defineProps<{ show: boolean; groupId: number; groupName: string }>()
const emit = defineEmits<{ close: [] }>()
const { t, locale } = useI18n()
const items = ref<PelicanShowcaseItem[]>([])
const selectedModel = ref('')
const selectedItem = ref<PelicanShowcaseItem | null>(null)
const previewHtml = ref('')
const loading = ref(false)
const previewLoading = ref(false)
const error = ref('')
const previewError = ref('')
const models = computed(() => [...new Set(items.value.map((item) => item.model_id))])
const filteredItems = computed(() => items.value.filter((item) => item.model_id === selectedModel.value))
let request = 0

function dateLabel(value: string) {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(value))
}

function durationLabel(value: number) {
  return `${(value / 1000).toFixed(2)} s`
}

async function load() {
  const current = ++request
  loading.value = true
  error.value = ''
  previewHtml.value = ''
  selectedItem.value = null
  try {
    const view = await getShowcase({ groupId: props.groupId })
    if (current !== request) return
    const group = view.groups.find((item) => item.id === props.groupId)
    items.value = group?.items ?? []
    selectedModel.value = models.value[0] ?? ''
    loading.value = false
    if (filteredItems.value.length) await selectItem(filteredItems.value[0])
  } catch {
    if (current === request) error.value = t('pelicanShowcase.history.loadFailed')
  } finally {
    if (current === request) loading.value = false
  }
}

async function selectItem(item: PelicanShowcaseItem) {
  const current = ++request
  selectedItem.value = item
  previewHtml.value = ''
  previewError.value = ''
  previewLoading.value = true
  try {
    const full = await getShowcaseItem(item.id)
    if (current !== request || full.group_id !== props.groupId) return
    previewHtml.value = full.response_text ?? ''
    if (!previewHtml.value) previewError.value = t('pelicanShowcase.history.invalidArtwork')
  } catch {
    if (current === request) previewError.value = t('pelicanShowcase.history.previewFailed')
  } finally {
    if (current === request) previewLoading.value = false
  }
}

watch(() => props.show, (show) => {
  if (show) void load()
  else request++
}, { immediate: true })

watch(selectedModel, () => {
  if (selectedItem.value && selectedItem.value.model_id !== selectedModel.value && filteredItems.value.length) {
    void selectItem(filteredItems.value[0])
  }
})
</script>
