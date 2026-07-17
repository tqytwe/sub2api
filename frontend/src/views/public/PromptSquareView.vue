<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
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

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

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

function querySignature(value: Record<string, unknown>): string {
  return JSON.stringify(
    Object.entries(value)
      .filter(([, item]) => item !== undefined && item !== '')
      .sort(([a], [b]) => a.localeCompare(b)),
  )
}

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
    appStore.showError('收藏操作失败，请稍后重试')
  } finally {
    busyIds.value.delete(prompt.id)
  }
}

async function handleCopy(prompt: PromptSummary) {
  try {
    const text = prompt.prompt_template || (await getPrompt(prompt.id)).prompt_template
    await copyPromptText(text)
    appStore.showSuccess('提示词已复制')
  } catch {
    appStore.showError('复制失败，请手动选择提示词')
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
  void router.push('/home')
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
    appStore.showError('暂时无法用于创作，请稍后重试')
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

onMounted(() => {
  applyPromptPageMetadata({
    title: '图像工作室 · 选提示词',
    description: '在图像工作室内按用途、风格、主体、模型和尺寸查找提示词，并用于图像创作。',
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
        <button type="button" class="prompt-library-home-link" aria-label="返回上一页" @click="goBack">
          <Icon name="arrowLeft" size="sm" />
          返回
        </button>
        <PublicPageToolbar />
      </div>
    </header>

    <main class="prompt-library-main">
      <section class="prompt-library-intro">
        <div>
          <p class="prompt-library-eyebrow">图像工作室 · 提示词库</p>
          <h1>选提示词</h1>
          <p>按用途和画面特征找到可直接创作的提示词，查看示例效果后再带入图像工作室。</p>
        </div>
      </section>

      <nav class="prompt-quick-links" aria-label="快捷入口">
        <button
          v-for="entry in [
            { key: 'featured', label: '极速蹬精选' },
            { key: 'latest', label: '最新收录' },
            { key: 'popular', label: '热门使用' },
            { key: 'no-reference', label: '无需参考图' },
            { key: 'favorite', label: '我的收藏' },
          ]"
          :key="entry.key"
          type="button"
          class="prompt-quick-link"
          :class="{ 'is-active': isQuickEntryActive(entry.key as 'featured' | 'latest' | 'popular' | 'no-reference' | 'favorite') }"
          @click="applyQuickEntry(entry.key as 'featured' | 'latest' | 'popular' | 'no-reference' | 'favorite')"
        >
          {{ entry.label }}
        </button>
      </nav>

      <PromptFilters v-model="filters" :categories="categories" @apply="syncFiltersToUrl" />

      <div class="prompt-result-bar">
        <span>{{ loading ? '正在加载提示词' : `共找到 ${result.total} 条提示词` }}</span>
        <label>
          <span class="sr-only">排序方式</span>
          <select
            :value="filters.sort"
            aria-label="排序方式"
            @change="filters = { ...filters, sort: ($event.target as HTMLSelectElement).value as PromptFiltersState['sort'], page: 1 }"
          >
            <option value="featured">精选优先</option>
            <option value="latest">最新收录</option>
            <option value="popular">热门使用</option>
          </select>
        </label>
      </div>

      <div v-if="loading" class="prompt-loading-grid" aria-label="正在加载">
        <div v-for="index in 8" :key="index" class="prompt-loading-item"></div>
      </div>

      <section v-else-if="loadFailed" class="prompt-error">
        <Icon name="exclamationCircle" size="xl" />
        <h2>提示词加载失败</h2>
        <p>网络暂时不可用，请稍后重新加载。</p>
        <button type="button" class="prompt-primary-button" @click="loadPrompts">重新加载</button>
      </section>

      <section v-else-if="result.items.length === 0" class="prompt-empty">
        <Icon name="search" size="xl" />
        <h2>没有找到匹配的提示词</h2>
        <p>可以减少筛选条件，或换一个关键词再试。</p>
        <button
          type="button"
          class="prompt-reset-button"
          @click="filters = { ...DEFAULT_PROMPT_FILTERS }"
        >
          清除全部筛选
        </button>
      </section>

      <section v-else class="prompt-masonry" aria-label="提示词列表">
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
