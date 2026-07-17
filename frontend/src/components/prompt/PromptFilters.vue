<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { PromptCategory, PromptCategoryDimension } from '@/api/prompts'
import type { PromptFiltersState } from '@/utils/promptLibrary'
import { DEFAULT_PROMPT_FILTERS } from '@/utils/promptLibrary'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  modelValue: PromptFiltersState
  categories: PromptCategory[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: PromptFiltersState]
  apply: []
}>()

const drawerOpen = ref(false)
const drawerFilters = ref<PromptFiltersState>({ ...props.modelValue })
let previousBodyOverflow = ''

const categoryGroups = computed(() => {
  const groups: Record<PromptCategoryDimension, PromptCategory[]> = {
    purpose: [],
    style: [],
    subject: [],
    model: [],
    size: [],
  }
  for (const category of props.categories) {
    groups[category.dimension]?.push(category)
  }
  for (const values of Object.values(groups)) {
    values.sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
  }
  return groups
})

function updateFilter<K extends keyof PromptFiltersState>(key: K, value: PromptFiltersState[K]) {
  emit('update:modelValue', {
    ...props.modelValue,
    [key]: value,
    page: key === 'page' ? Number(value) : 1,
  })
}

function updateDrawerFilter<K extends keyof PromptFiltersState>(key: K, value: PromptFiltersState[K]) {
  drawerFilters.value = {
    ...drawerFilters.value,
    [key]: value,
    page: key === 'page' ? Number(value) : 1,
  }
}

function openDrawer() {
  drawerFilters.value = { ...props.modelValue }
  drawerOpen.value = true
}

function closeDrawer() {
  drawerOpen.value = false
}

function resetFilters() {
  emit('update:modelValue', { ...DEFAULT_PROMPT_FILTERS })
}

function resetDrawerFilters() {
  drawerFilters.value = { ...DEFAULT_PROMPT_FILTERS }
}

function closeAndApply() {
  drawerOpen.value = false
  emit('update:modelValue', { ...drawerFilters.value })
  emit('apply')
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && drawerOpen.value) closeDrawer()
}

watch(drawerOpen, (open) => {
  if (open) {
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  } else {
    document.body.style.overflow = previousBodyOverflow
  }
})

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
  if (drawerOpen.value) document.body.style.overflow = previousBodyOverflow
})
</script>

<template>
  <div class="prompt-filter-shell">
    <div class="prompt-search-row">
      <label class="prompt-search-box">
        <span class="sr-only">搜索提示词</span>
        <Icon name="search" size="sm" />
        <input
          :value="modelValue.q"
          type="search"
          placeholder="搜索标题、用途或画面描述"
          @input="updateFilter('q', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <button
        type="button"
        class="prompt-mobile-filter-button"
        aria-label="打开筛选"
        @click="openDrawer"
      >
        <Icon name="filter" size="sm" />
        筛选
      </button>
    </div>

    <div class="prompt-desktop-filters" aria-label="提示词筛选">
      <label v-for="dimension in (['purpose', 'style', 'subject', 'model', 'size'] as const)" :key="dimension">
        <span>{{ { purpose: '用途', style: '风格', subject: '主体', model: '模型', size: '尺寸' }[dimension] }}</span>
        <select
          :value="modelValue[dimension]"
          @change="updateFilter(dimension, ($event.target as HTMLSelectElement).value)"
        >
          <option value="">全部</option>
          <option
            v-for="category in categoryGroups[dimension]"
            :key="category.id"
            :value="category.slug"
          >
            {{ category.name }}
          </option>
        </select>
      </label>
      <label>
        <span>参考图</span>
        <select
          :value="modelValue.reference"
          @change="updateFilter('reference', ($event.target as HTMLSelectElement).value as PromptFiltersState['reference'])"
        >
          <option value="">全部</option>
          <option value="none">无需参考图</option>
          <option value="optional">可选参考图</option>
          <option value="required">需要参考图</option>
        </select>
      </label>
      <button type="button" class="prompt-reset-button" @click="resetFilters">清除筛选</button>
    </div>

    <Teleport to="body">
      <div
        v-if="drawerOpen"
        class="prompt-filter-overlay"
        data-testid="prompt-filter-drawer"
        role="dialog"
        aria-modal="true"
        aria-labelledby="prompt-filter-title"
        @click.self="closeDrawer"
      >
        <div class="prompt-filter-drawer">
          <header>
            <h2 id="prompt-filter-title">筛选提示词</h2>
            <button
              type="button"
              class="prompt-icon-button"
              aria-label="关闭筛选"
              title="关闭筛选"
              @click="closeDrawer"
            >
              <Icon name="x" size="md" />
            </button>
          </header>
          <div class="prompt-filter-drawer-body">
            <label v-for="dimension in (['purpose', 'style', 'subject', 'model', 'size'] as const)" :key="dimension">
              <span>{{ { purpose: '用途', style: '风格', subject: '主体', model: '模型', size: '尺寸' }[dimension] }}</span>
              <select
                :value="drawerFilters[dimension]"
                @change="updateDrawerFilter(dimension, ($event.target as HTMLSelectElement).value)"
              >
                <option value="">全部</option>
                <option
                  v-for="category in categoryGroups[dimension]"
                  :key="category.id"
                  :value="category.slug"
                >
                  {{ category.name }}
                </option>
              </select>
            </label>
            <label>
              <span>参考图</span>
              <select
                :value="drawerFilters.reference"
                @change="updateDrawerFilter('reference', ($event.target as HTMLSelectElement).value as PromptFiltersState['reference'])"
              >
                <option value="">全部</option>
                <option value="none">无需参考图</option>
                <option value="optional">可选参考图</option>
                <option value="required">需要参考图</option>
              </select>
            </label>
          </div>
          <footer>
            <button type="button" class="prompt-reset-button" @click="resetDrawerFilters">清除筛选</button>
            <button type="button" class="prompt-primary-button" @click="closeAndApply">查看结果</button>
          </footer>
        </div>
      </div>
    </Teleport>
  </div>
</template>
