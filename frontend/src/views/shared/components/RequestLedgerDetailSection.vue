<template>
  <section class="space-y-3" :aria-label="title" :aria-busy="loading" :data-testid="`ledger-section-${section}`">
    <h3 class="text-base font-semibold">{{ title }}</h3>
    <p v-if="loading" role="status" class="text-sm">{{ t('common.loading') }}</p>
    <div v-if="error" role="alert" class="space-y-2 text-sm text-red-600 dark:text-red-400">
      <p>{{ t('requestLedger.sectionError') }}</p>
      <button type="button" class="btn btn-secondary" data-action="retry" :disabled="loading" @click="read(pendingCursor, pendingIndex)">{{ t('requestLedger.retry') }}</button>
    </div>
    <template v-if="page">
      <p v-if="!page.items.length" class="text-sm text-gray-500 dark:text-gray-400">{{ t(page.has_more ? 'requestLedger.emptyPageMore' : 'requestLedger.emptyPage') }}</p>
      <slot :page="page" :loading="loading" />
    </template>
    <nav class="flex flex-wrap items-center gap-2 border-t border-gray-200 pt-3 text-sm dark:border-dark-700" :aria-label="`${title} · ${t('common.pagination')}`">
      <button type="button" class="btn btn-secondary" data-action="previous" :disabled="loading || index === 0" @click="read(history[index - 1]!, index - 1)">{{ t('pagination.previous') }}</button>
      <span v-if="page" role="status" class="tabular-nums">{{ t('requestLedger.sectionPage', { page: index + 1, count: page.items.length }) }}</span>
      <button type="button" class="btn btn-secondary" data-action="next" :disabled="loading || !page?.has_more" @click="read(page!.next_cursor, index + 1)">{{ t('pagination.next') }}</button>
      <button type="button" class="btn btn-secondary" data-action="refresh" :disabled="loading" @click="refresh">{{ t('requestLedger.refreshSection') }}</button>
    </nav>
  </section>
</template>

<script setup lang="ts" generic="T">
import { onMounted, onUnmounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DetailSectionPage } from '@/api/requestLedger'

const props = defineProps<{ title: string; section: string; loadPage: (cursor: string, signal: AbortSignal) => Promise<DetailSectionPage<T>> }>()
const emit = defineEmits<{ change: [] }>()
const { t } = useI18n()
const page = shallowRef<DetailSectionPage<T> | null>(null)
const loading = ref(false), error = ref(false), index = ref(0)
const history = ref<string[]>([''])
let controller: AbortController | undefined
let pendingCursor = '', pendingIndex = 0

async function read(cursor: string, target: number) {
  controller?.abort()
  const current = new AbortController(); controller = current
  pendingCursor = cursor; pendingIndex = target
  loading.value = true; error.value = false
  emit('change')
  try {
    const result = await props.loadPage(cursor, current.signal)
    if (current.signal.aborted) return
    if (!Array.isArray(result.items) || result.consistency !== 'live' || typeof result.has_more !== 'boolean' || typeof result.next_cursor !== 'string' || result.has_more !== !!result.next_cursor) throw new Error('Unsupported detail page response')
    // Replace the page; refreshing never appends or mixes old/new row sets.
    page.value = result
    history.value = [...history.value.slice(0, target), cursor]
    index.value = target
  } catch {
    if (!current.signal.aborted) error.value = true
  } finally {
    if (!current.signal.aborted) loading.value = false
  }
}
function refresh() {
  history.value = ['']; index.value = 0; page.value = null
  void read('', 0)
}
onMounted(refresh)
onUnmounted(() => controller?.abort())
</script>
