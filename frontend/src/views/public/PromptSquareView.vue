<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
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
} from '@/api/prompts'
import {
  DEFAULT_PROMPT_FILTERS,
  openPromptInImageStudio,
  readPromptFilters,
  toPromptQuery,
  type PromptFiltersState,
} from '@/utils/promptLibrary'
import { copyPromptText } from '@/utils/promptLibraryClipboard'
import PromptCard from '@/components/prompt/PromptCard.vue'
import PromptFilters from '@/components/prompt/PromptFilters.vue'
import Pagination from '@/components/common/Pagination.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import Icon from '@/components/icons/Icon.vue'
import '@/components/prompt/prompt-library.css'
import { applyPromptPageMetadata, clearPromptPageMetadata } from '@/utils/promptPageMetadata'
import { isEnglishLocale } from '@/utils/localizedPublicSettings'
import { setLocale } from '@/i18n'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const { t, locale } = useI18n()

const filters = ref<PromptFiltersState>(readPromptFilters(route.query))
const categories = ref<PromptCategory[]>([])
const result = ref<PromptPagination>({
  items: [],
  total: 0,
  page: 1,
  page_size: 24,
  pages: 0,
})
const loading = ref(true)
const loadFailed = ref(false)
const busyIds = ref(new Set<string>())
let queryTimer: ReturnType<typeof setTimeout> | null = null
let requestId = 0

const isEnglishPromptLocale = computed(() => isEnglishLocale(locale.value))
type PromptQuickEntryKind = 'featured' | 'latest' | 'popular' | 'no-reference' | 'favorite'
const promptQuickEntries: { key: PromptQuickEntryKind; labelKey: string }[] = [
  { key: 'featured', labelKey: 'promptLibrary.quickFeatured' },
  { key: 'latest', labelKey: 'promptLibrary.quickLatest' },
  { key: 'popular', labelKey: 'promptLibrary.quickPopular' },
  { key: 'no-reference', labelKey: 'promptLibrary.quickNoReference' },
  { key: 'favorite', labelKey: 'promptLibrary.quickFavorite' },
]

function querySignature(value: Record<string, unknown>): string {
  return JSON.stringify(
    Object.entries(value)
      .filter(([, item]) => item !== undefined && item !== '')
      .sort(([a], [b]) => a.localeCompare(b)),
  )
}

async function loadCategories() {
  if (isEnglishPromptLocale.value) {
    categories.value = []
    return
  }
  try {
    categories.value = await listPromptCategories()
  } catch {
    categories.value = []
  }
}

async function loadPrompts() {
  if (isEnglishPromptLocale.value) {
    requestId += 1
    loading.value = false
    loadFailed.value = false
    result.value = { items: [], total: 0, page: filters.value.page, page_size: 24, pages: 0 }
    return
  }
  const currentRequest = ++requestId
  loading.value = true
  loadFailed.value = false
  try {
    const response = await listPrompts({
      ...filters.value,
      reference: filters.value.reference || undefined,
      featured: filters.value.featured || undefined,
      favorite: filters.value.favorite || undefined,
      page_size: 24,
    })
    if (currentRequest === requestId) result.value = response
  } catch {
    if (currentRequest === requestId) {
      loadFailed.value = true
      result.value = { items: [], total: 0, page: filters.value.page, page_size: 24, pages: 0 }
    }
  } finally {
    if (currentRequest === requestId) loading.value = false
  }
}

async function syncFiltersToUrl() {
  const nextQuery = toPromptQuery(filters.value)
  if (querySignature(nextQuery as Record<string, unknown>) === querySignature(route.query)) return
  await router.replace({ query: nextQuery })
}

function scheduleUrlSync() {
  if (queryTimer) clearTimeout(queryTimer)
  queryTimer = setTimeout(() => {
    void syncFiltersToUrl()
  }, 240)
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
  void syncFiltersToUrl()
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
    appStore.showError(t('promptLibrary.favoriteFailed'))
  } finally {
    busyIds.value.delete(prompt.id)
  }
}

async function handleCopy(prompt: PromptSummary) {
  try {
    const text = prompt.prompt_template || (await getPrompt(prompt.id)).prompt_template
    await copyPromptText(text)
    appStore.showSuccess(t('promptLibrary.copySuccess'))
  } catch {
    appStore.showError(t('promptLibrary.copyFailed'))
  }
}

function handleDetails(prompt: PromptSummary) {
  void router.push(`/prompts/${encodeURIComponent(prompt.id)}`)
}

function goBack() {
  if (window.history.length > 1) {
    router.back()
    return
  }
  void router.push(isEnglishPromptLocale.value ? '/en' : '/home')
}

async function showCurrentPromptLibrary() {
  await setLocale('zh')
  await router.replace({ path: '/prompts', query: route.query })
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
    await openPromptInImageStudio(prompt.id, usePrompt, router)
  } catch {
    appStore.showError(t('promptLibrary.useFailed'))
  } finally {
    busyIds.value.delete(prompt.id)
  }
}

function handlePage(page: number) {
  filters.value = { ...filters.value, page }
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

watch(
  () => route.query,
  (query) => {
    filters.value = readPromptFilters(query)
    void loadPrompts()
  },
  { deep: true, immediate: true },
)

watch(filters, scheduleUrlSync, { deep: true })

watch(isEnglishPromptLocale, (isEnglish) => {
  if (isEnglish) {
    requestId += 1
    loading.value = false
    loadFailed.value = false
    result.value = { items: [], total: 0, page: filters.value.page, page_size: 24, pages: 0 }
    categories.value = []
    return
  }
  void loadCategories()
  void loadPrompts()
})

onMounted(() => {
  applyPromptPageMetadata({
    title: t('promptLibrary.metaTitle'),
    description: t('promptLibrary.metaDescription'),
    path: '/prompts',
    kind: 'square',
  })
  void loadCategories()
})

onBeforeUnmount(() => {
  if (queryTimer) clearTimeout(queryTimer)
  clearPromptPageMetadata()
})
</script>

<template>
  <div class="prompt-library-page">
    <header class="prompt-library-header">
      <div class="prompt-library-header-inner">
        <button type="button" class="prompt-library-home-link" :aria-label="t('promptLibrary.backAria')" @click="goBack">
          <Icon name="arrowLeft" size="sm" />
          {{ t('promptLibrary.back') }}
        </button>
        <PublicPageToolbar />
      </div>
    </header>

    <main v-if="isEnglishPromptLocale" class="prompt-library-main">
      <section class="prompt-empty">
        <Icon name="search" size="xl" />
        <p class="prompt-library-eyebrow">{{ t('promptLibrary.englishPendingEyebrow') }}</p>
        <h1>{{ t('promptLibrary.englishPendingTitle') }}</h1>
        <p>{{ t('promptLibrary.englishPendingBody') }}</p>
        <button
          type="button"
          class="prompt-primary-button"
          @click="showCurrentPromptLibrary"
        >
          {{ t('promptLibrary.switchToDefaultLanguage') }}
        </button>
      </section>
    </main>

    <main v-else class="prompt-library-main">
      <section class="prompt-library-intro">
        <div>
          <p class="prompt-library-eyebrow">{{ t('promptLibrary.eyebrow') }}</p>
          <h1>{{ t('promptLibrary.title') }}</h1>
          <p>{{ t('promptLibrary.description') }}</p>
        </div>
      </section>

      <nav class="prompt-quick-links" :aria-label="t('promptLibrary.quickLinksAria')">
        <button
          v-for="entry in promptQuickEntries"
          :key="entry.key"
          type="button"
          class="prompt-quick-link"
          :class="{ 'is-active': isQuickEntryActive(entry.key) }"
          @click="applyQuickEntry(entry.key)"
        >
          {{ t(entry.labelKey) }}
        </button>
      </nav>

      <PromptFilters v-model="filters" :categories="categories" @apply="syncFiltersToUrl" />

      <div class="prompt-result-bar">
        <span>{{ loading ? t('promptLibrary.loading') : t('promptLibrary.resultCount', { count: result.total }) }}</span>
        <label>
          <span class="sr-only">{{ t('promptLibrary.sortLabel') }}</span>
          <select
            :value="filters.sort"
            :aria-label="t('promptLibrary.sortLabel')"
            @change="filters = { ...filters, sort: ($event.target as HTMLSelectElement).value as PromptFiltersState['sort'], page: 1 }"
          >
            <option value="featured">{{ t('promptLibrary.sortFeatured') }}</option>
            <option value="latest">{{ t('promptLibrary.sortLatest') }}</option>
            <option value="popular">{{ t('promptLibrary.sortPopular') }}</option>
          </select>
        </label>
      </div>

      <div v-if="loading" class="prompt-loading-grid" :aria-label="t('promptLibrary.loading')">
        <div v-for="index in 8" :key="index" class="prompt-loading-item"></div>
      </div>

      <section v-else-if="loadFailed" class="prompt-error">
        <Icon name="exclamationCircle" size="xl" />
        <h2>{{ t('promptLibrary.loadFailedTitle') }}</h2>
        <p>{{ t('promptLibrary.loadFailedBody') }}</p>
        <button type="button" class="prompt-primary-button" @click="loadPrompts">{{ t('promptLibrary.reload') }}</button>
      </section>

      <section v-else-if="result.items.length === 0" class="prompt-empty">
        <Icon name="search" size="xl" />
        <h2>{{ t('promptLibrary.emptyTitle') }}</h2>
        <p>{{ t('promptLibrary.emptyBody') }}</p>
        <button
          type="button"
          class="prompt-reset-button"
          @click="filters = { ...DEFAULT_PROMPT_FILTERS }"
        >
          {{ t('promptLibrary.clearFilters') }}
        </button>
      </section>

      <section v-else class="prompt-masonry" :aria-label="t('promptLibrary.listAria')">
        <PromptCard
          v-for="prompt in result.items"
          :key="prompt.id"
          :prompt="prompt"
          :busy="busyIds.has(prompt.id)"
          @favorite="handleFavorite"
          @copy="handleCopy"
          @details="handleDetails"
          @use="handleUse"
        />
      </section>

      <div v-if="!loading && result.total > result.page_size" class="prompt-pagination">
        <Pagination
          :total="result.total"
          :page="filters.page"
          :page-size="result.page_size"
          :show-page-size-selector="false"
          @update:page="handlePage"
        />
      </div>
    </main>
  </div>
</template>
