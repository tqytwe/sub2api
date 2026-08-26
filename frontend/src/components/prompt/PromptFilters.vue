<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
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

const { t } = useI18n()
const drawerOpen = ref(false)
const drawerFilters = ref<PromptFiltersState>({ ...props.modelValue })
let previousBodyOverflow = ''

const filterDimensions: PromptCategoryDimension[] = ['purpose', 'style', 'subject', 'model', 'size']

function dimensionLabel(dimension: PromptCategoryDimension): string {
  return t(`promptLibrary.filters.${dimension}`)
}

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
        <span class="sr-only">{{ t('promptLibrary.filters.searchLabel') }}</span>
        <Icon name="search" size="sm" />
        <input
          :value="modelValue.q"
          type="search"
          :placeholder="t('promptLibrary.filters.searchPlaceholder')"
          @input="updateFilter('q', ($event.target as HTMLInputElement).value)"
        />
      </label>
      <button
        type="button"
        class="prompt-mobile-filter-button"
        :aria-label="t('promptLibrary.filters.open')"
        @click="openDrawer"
      >
        <Icon name="filter" size="sm" />
        {{ t('common.filter') }}
      </button>
    </div>

    <div class="prompt-desktop-filters" :aria-label="t('promptLibrary.filters.groupLabel')">
      <label v-for="dimension in filterDimensions" :key="dimension">
        <span>{{ dimensionLabel(dimension) }}</span>
        <select
          :value="modelValue[dimension]"
          @change="updateFilter(dimension, ($event.target as HTMLSelectElement).value)"
        >
          <option value="">{{ t('promptLibrary.filters.all') }}</option>
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
        <span>{{ t('promptLibrary.filters.reference') }}</span>
        <select
          :value="modelValue.reference"
          @change="updateFilter('reference', ($event.target as HTMLSelectElement).value as PromptFiltersState['reference'])"
        >
          <option value="">{{ t('promptLibrary.filters.all') }}</option>
          <option value="none">{{ t('promptLibrary.filters.none') }}</option>
          <option value="optional">{{ t('promptLibrary.filters.optional') }}</option>
          <option value="required">{{ t('promptLibrary.filters.required') }}</option>
        </select>
      </label>
      <button type="button" class="prompt-reset-button" @click="resetFilters">{{ t('promptLibrary.filters.reset') }}</button>
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
            <h2 id="prompt-filter-title">{{ t('promptLibrary.filters.title') }}</h2>
            <button
              type="button"
              class="prompt-icon-button"
              :aria-label="t('promptLibrary.filters.close')"
              :title="t('promptLibrary.filters.close')"
              @click="closeDrawer"
            >
              <Icon name="x" size="md" />
            </button>
          </header>
          <div class="prompt-filter-drawer-body">
            <label v-for="dimension in filterDimensions" :key="dimension">
              <span>{{ dimensionLabel(dimension) }}</span>
              <select
                :value="drawerFilters[dimension]"
                @change="updateDrawerFilter(dimension, ($event.target as HTMLSelectElement).value)"
              >
                <option value="">{{ t('promptLibrary.filters.all') }}</option>
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
              <span>{{ t('promptLibrary.filters.reference') }}</span>
              <select
                :value="drawerFilters.reference"
                @change="updateDrawerFilter('reference', ($event.target as HTMLSelectElement).value as PromptFiltersState['reference'])"
              >
                <option value="">{{ t('promptLibrary.filters.all') }}</option>
                <option value="none">{{ t('promptLibrary.filters.none') }}</option>
                <option value="optional">{{ t('promptLibrary.filters.optional') }}</option>
                <option value="required">{{ t('promptLibrary.filters.required') }}</option>
              </select>
            </label>
          </div>
          <footer>
            <button type="button" class="prompt-reset-button" @click="resetDrawerFilters">{{ t('promptLibrary.filters.reset') }}</button>
            <button type="button" class="prompt-primary-button" @click="closeAndApply">{{ t('promptLibrary.filters.apply') }}</button>
          </footer>
        </div>
      </div>
    </Teleport>
  </div>
</template>
