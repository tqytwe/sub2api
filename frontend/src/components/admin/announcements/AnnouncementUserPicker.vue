<template>
  <div class="space-y-2" data-testid="announcement-user-picker">
    <div class="relative" @focusout="closeResultsOnBlur">
      <input
        v-model="query"
        type="text"
        autocomplete="off"
        class="input w-full"
        :aria-label="t('admin.announcements.form.selectUsers')"
        :disabled="selectionFull"
        :placeholder="t('admin.announcements.form.searchUsersPlaceholder')"
        data-testid="announcement-user-search"
        @input="handleSearch"
        @keydown.enter="handleEnter"
        @focus="showResults = query.trim() !== ''"
      />
      <div
        v-if="showResults && (searching || searched)"
        class="absolute left-0 right-0 top-full z-10 mt-1 max-h-48 overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-500 dark:bg-dark-700"
      >
        <p v-if="searching" class="px-3 py-2 text-sm" role="status">{{ t('common.loading') }}</p>
        <p v-else-if="searchError" class="px-3 py-2 text-sm text-red-700 dark:text-red-300" role="status">
          {{ t('admin.announcements.form.userSearchFailed') }}
        </p>
        <button
          v-for="user in results"
          :key="user.id"
          type="button"
          class="flex w-full flex-wrap items-center gap-2 px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-dark-600"
          data-testid="announcement-user-option"
          @mousedown.prevent
          @click="addUser(user)"
        >
          <span class="text-gray-400">#{{ user.id }}</span>
          <span class="min-w-0 break-all text-gray-900 dark:text-white">{{ user.username || user.email }}</span>
          <span v-if="user.username" class="min-w-0 break-all text-xs text-gray-400">{{ user.email }}</span>
          <Icon v-if="modelValue.includes(user.id)" name="check" size="sm" class="ml-auto text-primary-600" />
        </button>
        <div v-if="!searching && !searchError && results.length === 0" class="px-3 py-2 text-sm text-gray-500 dark:text-dark-400">
          {{ t('admin.announcements.form.noUsersFound') }}
        </div>
      </div>
    </div>

    <div v-if="modelValue.length > 0" class="flex flex-wrap gap-2">
      <span
        v-for="id in modelValue"
        :key="id"
        class="inline-flex max-w-full items-center gap-1 rounded-lg bg-primary-50 px-2 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-300"
        data-testid="announcement-user-chip"
      >
        <span class="text-gray-400">#{{ id }}</span>
        <span class="min-w-0 break-all">{{ userLabel(id) }}</span>
        <button
          type="button"
          class="rounded hover:text-primary-900 dark:hover:text-primary-100"
          :aria-label="t('admin.announcements.form.removeUser', { id })"
          @click="removeUser(id)"
        >
          <Icon name="x" size="xs" />
        </button>
      </span>
    </div>
    <p v-else class="text-xs text-gray-500 dark:text-dark-400">
      {{ t('admin.announcements.form.noUsersSelected') }}
    </p>
    <p v-if="selectionFull" class="input-hint" role="status">
      {{ t('admin.announcements.form.userSelectionLimit', { count: maxUsers }) }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  modelValue: number[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

const { t } = useI18n()

const maxUsers = 1000
const selectionFull = computed(() => props.modelValue.length >= maxUsers)
const query = ref('')
const results = ref<AdminUser[]>([])
const searched = ref(false)
const searching = ref(false)
const searchError = ref(false)
const showResults = ref(false)

// 已选用户的展示信息；null 表示按 ID 查不到。编辑已有公告时只有 ID，按需补查。
const users = ref<Record<number, { email: string; username: string } | null>>({})
const requested = new Set<number>()
let disposed = false

const remember = (user: AdminUser) => {
  users.value[user.id] = { email: user.email, username: user.username || '' }
}

watch(
  () => props.modelValue,
  async (ids) => {
    // Resolve existing recipients in small batches instead of firing up to 1,000 requests at once.
    const missing = ids.filter((id) => !(id in users.value) && !requested.has(id))
    missing.forEach((id) => requested.add(id))
    for (let offset = 0; offset < missing.length && !disposed; offset += 8) {
      await Promise.all(missing.slice(offset, offset + 8).map(async (id) => {
        try {
          remember(await adminAPI.users.getById(id, true))
        } catch {
          users.value[id] = null
        }
      }))
    }
  },
  { immediate: true }
)

const userLabel = (id: number) => {
  const user = users.value[id]
  if (user === null) return t('admin.announcements.form.userUnavailable')
  if (!user) return '…'
  return user.username ? `${user.username} (${user.email})` : user.email
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
let searchSeq = 0

function closeResultsOnBlur(event: FocusEvent) {
  const nextFocus = event.relatedTarget
  if (!(nextFocus instanceof Node) || !(event.currentTarget as HTMLElement).contains(nextFocus)) {
    showResults.value = false
  }
}

const handleSearch = () => {
  clearTimeout(searchTimer)
  const seq = ++searchSeq
  const keyword = query.value.trim()
  results.value = []
  searched.value = false
  searchError.value = false
  searching.value = keyword !== ''
  showResults.value = keyword !== ''
  if (!keyword) return

  searchTimer = setTimeout(async () => {
    try {
      const res = await adminAPI.users.list(1, 10, { search: keyword, include_subscriptions: false })
      if (seq === searchSeq) results.value = res.items
    } catch {
      if (seq === searchSeq) searchError.value = true
    } finally {
      if (seq === searchSeq) {
        searching.value = false
        searched.value = true
      }
    }
  }, 300)
}

const addUser = (user: AdminUser) => {
  remember(user)
  if (!selectionFull.value && !props.modelValue.includes(user.id)) {
    emit('update:modelValue', [...props.modelValue, user.id])
  }
  searchSeq++
  query.value = ''
  results.value = []
  searched.value = false
  searching.value = false
  showResults.value = false
}

// 选择框在公告表单里：回车只选中第一个搜索结果，不能触发表单提交。
const handleEnter = (event: KeyboardEvent) => {
  if (event.isComposing) return
  event.preventDefault()
  if (results.value.length > 0) addUser(results.value[0])
}

const removeUser = (id: number) => {
  emit('update:modelValue', props.modelValue.filter((existing) => existing !== id))
}

onBeforeUnmount(() => {
  disposed = true
  searchSeq++
  clearTimeout(searchTimer)
})
</script>
