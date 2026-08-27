<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import {
  favoritePrompt,
  getPrompt,
  listPromptCategories,
  listPrompts,
  unfavoritePrompt,
  usePrompt,
  type PromptCategory,
  type PromptPagination,
  type PromptSummary,
  type PromptUseResult,
} from '@/api/prompts'
import {
  DEFAULT_PROMPT_FILTERS,
  type PromptFiltersState,
} from '@/utils/promptLibrary'
import { copyPromptText } from '@/utils/promptLibraryClipboard'
import PromptCard from '@/components/prompt/PromptCard.vue'
import PromptFilters from '@/components/prompt/PromptFilters.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import '@/components/prompt/prompt-library.css'

const props = withDefaults(defineProps<{
  pageSize?: number
}>(), {
  pageSize: 12,
})

const emit = defineEmits<{
  use: [payload: PromptUseResult]
}>()

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const filters = ref<PromptFiltersState>({
  ...DEFAULT_PROMPT_FILTERS,
  featured: true,
})
const categories = ref<PromptCategory[]>([])
const result = ref<PromptPagination>({
  items: [],
  total: 0,
  page: 1,
  page_size: props.pageSize,
  pages: 0,
})
const loading = ref(true)
const loadFailed = ref(false)
const busyIds = ref(new Set<string>())
let requestId = 0

const quickEntries = [
  { key: 'featured', labelKey: 'promptLibrary.panel.featured' },
  { key: 'latest', labelKey: 'promptLibrary.panel.latest' },
  { key: 'popular', labelKey: 'promptLibrary.panel.popular' },
  { key: 'no-reference', labelKey: 'promptLibrary.panel.noReference' },
  { key: 'favorite', labelKey: 'promptLibrary.panel.favorites' },
] as const

async function loadCategories() {
  try {
    categories.value = await listPromptCategories()
  } catch {
    categories.value = []
  }
}

async function loadPrompts() {
  const currentRequest = ++requestId
  loading.value = true
  loadFailed.value = false
  try {
    const response = await listPrompts({
      ...filters.value,
      reference: filters.value.reference || undefined,
      featured: filters.value.featured || undefined,
      favorite: filters.value.favorite || undefined,
      page_size: props.pageSize,
    })
    if (currentRequest === requestId) result.value = response
  } catch {
    if (currentRequest === requestId) {
      loadFailed.value = true
      result.value = {
        items: [],
        total: 0,
        page: filters.value.page,
        page_size: props.pageSize,
        pages: 0,
      }
    }
  } finally {
    if (currentRequest === requestId) loading.value = false
  }
}

async function applyQuickEntry(kind: 'featured' | 'latest' | 'popular' | 'no-reference' | 'favorite') {
  if (kind === 'favorite' && !authStore.isAuthenticated) {
    await router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  const next = { ...DEFAULT_PROMPT_FILTERS }
  if (kind === 'featured') next.featured = true
  if (kind === 'latest') next.sort = 'latest'
  if (kind === 'popular') next.sort = 'popular'
  if (kind === 'no-reference') next.reference = 'none'
  if (kind === 'favorite') next.favorite = true
  filters.value = next
}

function isQuickEntryActive(kind: 'featured' | 'latest' | 'popular' | 'no-reference' | 'favorite'): boolean {
  if (kind === 'featured') return filters.value.featured
  if (kind === 'latest') return filters.value.sort === 'latest' && !filters.value.featured
  if (kind === 'popular') return filters.value.sort === 'popular' && !filters.value.featured
  if (kind === 'favorite') return filters.value.favorite
  return filters.value.reference === 'none'
}

function updatePromptInList(id: string, update: Partial<PromptSummary>) {
  const target = result.value.items.find((item) => item.id === id)
  if (target) Object.assign(target, update)
}

async function handleFavorite(prompt: PromptSummary) {
  if (busyIds.value.has(prompt.id)) return
  busyIds.value.add(prompt.id)
  try {
    if (prompt.is_favorited) {
      const state = await unfavoritePrompt(prompt.id)
      updatePromptInList(prompt.id, {
        is_favorited: state.favorited,
        favorite_count: state.favorite_count ?? prompt.favorite_count,
      })
    } else {
      const state = await favoritePrompt(prompt.id)
      updatePromptInList(prompt.id, {
        is_favorited: state.favorited,
        favorite_count: state.favorite_count ?? prompt.favorite_count,
      })
    }
  } catch {
    appStore.showError(t('promptLibrary.messages.favoriteFailed'))
  } finally {
    busyIds.value.delete(prompt.id)
  }
}

async function handleCopy(prompt: PromptSummary) {
  try {
    const text = prompt.prompt_template || (await getPrompt(prompt.id)).prompt_template
    await copyPromptText(text)
    appStore.showSuccess(t('promptLibrary.messages.copied'))
  } catch {
    appStore.showError(t('promptLibrary.messages.copyFailed'))
  }
}

async function handleUse(prompt: PromptSummary) {
  if (busyIds.value.has(prompt.id)) return
  if (!authStore.isAuthenticated) {
    await router.push({
      path: '/login',
      query: { redirect: route.fullPath },
    })
    return
  }
  busyIds.value.add(prompt.id)
  try {
    emit('use', await usePrompt(prompt.id))
  } catch {
    appStore.showError(t('promptLibrary.messages.useFailed'))
  } finally {
    busyIds.value.delete(prompt.id)
  }
}

function handlePage(page: number) {
  filters.value = { ...filters.value, page }
}

watch(filters, loadPrompts, { deep: true })

onMounted(() => {
  void loadCategories()
  void loadPrompts()
})
</script>

<template>
  <section class="prompt-studio-panel">
    <header class="prompt-studio-panel-header">
      <div>
        <h2>{{ t('promptLibrary.panel.title') }}</h2>
        <p>{{ t('promptLibrary.panel.description') }}</p>
      </div>
      <button type="button" class="prompt-primary-button" @click="applyQuickEntry('featured')">
        <Icon name="sparkles" size="sm" />
        {{ t('promptLibrary.panel.featured') }}
      </button>
    </header>

    <nav class="prompt-quick-links" :aria-label="t('promptLibrary.panel.quickLinks')">
      <button
        v-for="entry in quickEntries"
        :key="entry.key"
        type="button"
        class="prompt-quick-link"
        :class="{ 'is-active': isQuickEntryActive(entry.key as 'featured' | 'latest' | 'popular' | 'no-reference' | 'favorite') }"
        @click="applyQuickEntry(entry.key as 'featured' | 'latest' | 'popular' | 'no-reference' | 'favorite')"
      >
        {{ t(entry.labelKey) }}
      </button>
    </nav>

    <PromptFilters v-model="filters" :categories="categories" />

    <div class="prompt-result-bar">
      <span>{{ loading ? t('promptLibrary.panel.loading') : t('promptLibrary.panel.resultCount', { count: result.total }) }}</span>
      <label>
        <span class="sr-only">{{ t('promptLibrary.panel.sort') }}</span>
        <select
          :value="filters.sort"
          :aria-label="t('promptLibrary.panel.sort')"
          @change="filters = { ...filters, sort: ($event.target as HTMLSelectElement).value as PromptFiltersState['sort'], page: 1 }"
        >
          <option value="featured">{{ t('promptLibrary.panel.sortFeatured') }}</option>
          <option value="latest">{{ t('promptLibrary.panel.sortLatest') }}</option>
          <option value="popular">{{ t('promptLibrary.panel.sortPopular') }}</option>
        </select>
      </label>
    </div>

    <div v-if="loading" class="prompt-loading-grid" :aria-label="t('promptLibrary.panel.loadingLabel')">
      <div v-for="index in 6" :key="index" class="prompt-loading-item"></div>
    </div>

    <section v-else-if="loadFailed" class="prompt-error">
      <Icon name="exclamationCircle" size="lg" />
      <h2>{{ t('promptLibrary.panel.loadFailed') }}</h2>
      <p>{{ t('promptLibrary.panel.loadFailedHint') }}</p>
      <button type="button" class="prompt-primary-button" @click="loadPrompts">{{ t('promptLibrary.panel.retry') }}</button>
    </section>

    <section v-else-if="result.items.length === 0" class="prompt-empty">
      <Icon name="sparkles" size="lg" />
      <h2>{{ t('promptLibrary.panel.empty') }}</h2>
      <p>{{ t('promptLibrary.panel.emptyHint') }}</p>
      <button type="button" class="prompt-reset-button" @click="filters = { ...DEFAULT_PROMPT_FILTERS }">{{ t('promptLibrary.panel.reset') }}</button>
    </section>

    <section v-else class="prompt-masonry" :aria-label="t('promptLibrary.panel.list')">
      <PromptCard
        v-for="prompt in result.items"
        :key="prompt.id"
        :prompt="prompt"
        :busy="busyIds.has(prompt.id)"
        @favorite="handleFavorite"
        @copy="handleCopy"
        @use="handleUse"
      />
    </section>

    <div v-if="!loading && result.total > result.page_size" class="prompt-pagination">
      <Pagination
        :total="result.total"
        :page="result.page"
        :page-size="result.page_size"
        :show-page-size-selector="false"
        @update:page="handlePage"
      />
    </div>
  </section>
</template>
