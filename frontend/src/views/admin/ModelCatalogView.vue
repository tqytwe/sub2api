<template>
  <AppLayout>
    <div class="mb-4 flex items-center gap-2 border-b border-gray-200 px-1 dark:border-dark-700">
      <button class="border-b-2 px-3 py-2 text-sm font-medium" :class="activePanel === 'models' ? 'border-gray-900 text-gray-900 dark:border-white dark:text-white' : 'border-transparent text-gray-500'" @click="activePanel = 'models'">{{ t('modelPlaza.admin.modelsTab') }}</button>
      <button class="border-b-2 px-3 py-2 text-sm font-medium" :class="activePanel === 'groups' ? 'border-gray-900 text-gray-900 dark:border-white dark:text-white' : 'border-transparent text-gray-500'" @click="activePanel = 'groups'">{{ t('modelPlaza.admin.groupsTab') }}</button>
    </div>
    <section v-show="activePanel === 'groups'" class="mb-6 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.groups.title') }}</h2>
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.groups.description') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <router-link class="btn btn-secondary" to="/admin/groups">{{ t('modelPlaza.admin.manageGroups') }}</router-link>
          <button class="btn btn-secondary" :disabled="groupLoading" @click="loadPlazaGroups">
            <Icon name="refresh" size="md" />
          </button>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[900px] text-sm">
          <thead class="border-b border-gray-200 text-left text-xs text-gray-500 dark:border-dark-700">
            <tr><th class="px-3 py-2">{{ t('admin.groups.columns.name') }}</th><th class="px-3 py-2">{{ t('admin.groups.columns.platform') }}</th><th class="px-3 py-2">{{ t('admin.groups.columns.rateMultiplier') }}</th><th class="px-3 py-2">{{ t('admin.groups.columns.subscriptionType') }}</th><th class="px-3 py-2">{{ t('admin.groups.columns.status') }}</th></tr>
          </thead>
          <tbody>
            <tr v-for="group in plazaGroups" :key="group.id" class="border-b border-gray-100 dark:border-dark-700/60">
              <td class="px-3 py-2"><div class="font-medium text-gray-900 dark:text-white">{{ group.name }}</div><div class="max-w-[360px] truncate text-xs text-gray-500">{{ group.description }}</div></td>
              <td class="px-3 py-2">{{ platformLabel(group.platform) }}</td><td class="px-3 py-2 font-mono">{{ group.rate_multiplier }}<span class="ml-1 text-xs text-gray-400">{{ t('modelPlaza.admin.displayOnly') }}</span></td>
              <td class="px-3 py-2">{{ subscriptionTypeLabel(group.subscription_type) }}</td><td class="px-3 py-2">{{ groupStatusLabel(group.status) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
    <TablePageLayout v-show="activePanel === 'models'">
      <template #filters>
        <div class="flex flex-col justify-between gap-4 lg:flex-row lg:items-start">
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <input
              v-model="searchQuery"
              type="search"
              class="input w-full sm:w-72"
              :placeholder="t('admin.modelCatalog.searchPlaceholder')"
              @input="debouncedLoad"
            />
            <input
              v-model="platformFilter"
              type="text"
              class="input w-40"
              :placeholder="t('admin.modelCatalog.platformFilter')"
              @change="loadCatalog"
            />
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadCatalog">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-secondary" :disabled="syncing" @click="startSync">
              {{ syncing ? t('admin.modelCatalog.syncing') : t('admin.modelCatalog.sync') }}
            </button>
            <button class="btn btn-secondary" @click="openDiscoveryPanel">
              {{ t('admin.modelCatalog.importDiscovery') }}
            </button>
            <button class="btn btn-primary" @click="openCreate">
              {{ t('admin.modelCatalog.add') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <div v-if="selectedIds.length" class="mb-3 flex flex-wrap gap-2">
          <button class="btn btn-secondary btn-sm" @click="batchSetPublic(true)">
            {{ t('admin.modelCatalog.batchPublicOn') }}
          </button>
          <button class="btn btn-secondary btn-sm" @click="batchSetPublic(false)">
            {{ t('admin.modelCatalog.batchPublicOff') }}
          </button>
          <button class="btn btn-secondary btn-sm" @click="openBatchGroups">
            {{ t('admin.modelCatalog.batchSetGroups') }}
          </button>
        </div>

        <DataTable :columns="columns" :data="filteredRows" :loading="loading">
          <template #cell-select="{ row }">
            <input type="checkbox" :checked="selectedIds.includes(row.id)" @change="toggleSelect(row.id)" />
          </template>
          <template #cell-model_name="{ value }">
            <span class="font-medium">{{ value }}</span>
          </template>
          <template #cell-official_input_price="{ row }">
            {{ formatPrice(row.official_input_price) }}
          </template>
          <template #cell-official_output_price="{ row }">
            {{ formatPrice(row.official_output_price) }}
          </template>
          <template #cell-group_ids="{ row }">
            <span v-if="row.group_ids == null" class="text-xs text-gray-500">{{ t('admin.modelCatalog.groupModeAuto') }}</span>
            <span v-else-if="row.group_ids.length === 0" class="text-xs text-gray-500">{{ t('admin.modelCatalog.groupModeNone') }}</span>
            <span v-else class="flex flex-wrap gap-1">
              <span v-for="id in row.group_ids" :key="id" class="rounded bg-gray-100 px-1.5 py-0.5 text-xs dark:bg-dark-700">
                {{ groupLabel(id) }}
              </span>
            </span>
          </template>
          <template #cell-visible_public="{ row }">
            <Toggle :model-value="row.visible_public" @update:model-value="(v: boolean) => patchVisibility(row, v, undefined)" />
          </template>
          <template #cell-visible_auth="{ row }">
            <Toggle :model-value="row.visible_auth" @update:model-value="(v: boolean) => patchVisibility(row, undefined, v)" />
          </template>
          <template #cell-actions="{ row }">
            <button class="btn btn-ghost btn-sm" @click="openEdit(row)">{{ t('common.edit') }}</button>
            <button class="btn btn-ghost btn-sm text-red-600" @click="removeRow(row)">{{ t('common.delete') }}</button>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <BaseDialog :show="editOpen" :title="editForm.id ? t('admin.modelCatalog.editTitle') : t('admin.modelCatalog.addTitle')" @close="editOpen = false">
      <div class="space-y-4">
        <label class="block text-sm">
          {{ t('admin.modelCatalog.fields.model') }}
          <input v-model="editForm.model_name" class="input mt-1 w-full" />
        </label>
        <label class="block text-sm">
          {{ t('admin.modelCatalog.fields.platform') }}
          <input v-model="editForm.platform" class="input mt-1 w-full" />
        </label>
        <div class="border-t border-gray-200 pt-4 dark:border-dark-700">
          <label class="block text-sm">
            {{ t('admin.modelCatalog.fields.groupMode') }}
            <select v-model="editForm.group_mode" class="input mt-1 w-full">
              <option value="selected">{{ t('admin.modelCatalog.fields.groupModeSelected') }}</option>
              <option value="auto">{{ t('admin.modelCatalog.fields.groupModeAuto') }}</option>
            </select>
          </label>
          <GroupSelector
            v-if="editForm.group_mode === 'selected'"
            v-model="editForm.group_ids"
            :groups="groups"
            searchable="auto"
            class="mt-3"
          />
          <p v-if="editForm.group_mode === 'selected'" class="mt-2 text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.modelCatalog.fields.groupModeSelectedHint') }}
          </p>
          <p v-else class="mt-2 text-xs text-amber-700 dark:text-amber-300">
            {{ t('admin.modelCatalog.fields.groupModeAutoHint') }}
          </p>
        </div>
        <label class="block text-sm">
          {{ t('admin.modelCatalog.fields.displayName') }}
          <input v-model="editForm.display_name" class="input mt-1 w-full" />
        </label>
        <label class="block text-sm">
          {{ t('admin.modelCatalog.fields.sortOrder') }}
          <input v-model.number="editForm.sort_order" type="number" class="input mt-1 w-full" />
        </label>
        <div class="flex gap-4">
          <label class="flex items-center gap-2 text-sm">
            <input v-model="editForm.visible_public" type="checkbox" />
            {{ t('admin.modelCatalog.fields.visiblePublic') }}
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input v-model="editForm.visible_auth" type="checkbox" />
            {{ t('admin.modelCatalog.fields.visibleAuth') }}
          </label>
        </div>

        <div class="grid grid-cols-2 gap-3 border-t border-gray-200 pt-4 text-sm dark:border-dark-700">
          <div>
            <span class="text-gray-500">{{ t('admin.modelCatalog.columns.officialInput') }}</span>
            <input v-model.number="editForm.official_input_price_million" type="number" min="0" step="0.01" class="input mt-1 w-full" />
          </div>
          <div>
            <span class="text-gray-500">{{ t('admin.modelCatalog.columns.officialOutput') }}</span>
            <input v-model.number="editForm.official_output_price_million" type="number" min="0" step="0.01" class="input mt-1 w-full" />
          </div>
          <div>
            <span class="text-gray-500">{{ t('admin.modelCatalog.fields.officialCacheRead') }}</span>
            <input v-model.number="editForm.official_cache_read_price_million" type="number" min="0" step="0.01" class="input mt-1 w-full" />
          </div>
          <div>
            <span class="text-gray-500">{{ t('admin.modelCatalog.fields.officialCacheWrite') }}</span>
            <input v-model.number="editForm.official_cache_write_price_million" type="number" min="0" step="0.01" class="input mt-1 w-full" />
          </div>
        </div>

      </div>
      <template #footer>
        <div class="flex gap-2 pr-14 sm:pr-0">
          <button class="btn btn-secondary" @click="editOpen = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="saving" @click="saveEdit">{{ t('common.save') }}</button>
        </div>
      </template>
    </BaseDialog>
    <BaseDialog :show="batchGroupOpen" :title="t('admin.modelCatalog.batchGroupsTitle')" @close="batchGroupOpen = false">
      <div class="space-y-4">
        <label class="block text-sm">
          {{ t('admin.modelCatalog.fields.groupMode') }}
          <select v-model="batchGroupMode" class="input mt-1 w-full">
            <option value="selected">{{ t('admin.modelCatalog.fields.groupModeSelected') }}</option>
            <option value="auto">{{ t('admin.modelCatalog.fields.groupModeAuto') }}</option>
          </select>
        </label>
        <GroupSelector
          v-if="batchGroupMode === 'selected'"
          v-model="batchGroupIDs"
          :groups="groups"
          searchable="auto"
        />
        <p v-if="batchGroupMode === 'selected'" class="text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.modelCatalog.fields.groupModeSelectedHint') }}
        </p>
        <p v-else class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.modelCatalog.fields.groupModeAutoHint') }}</p>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="batchGroupOpen = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="saving" @click="saveBatchGroups">
          {{ t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="syncResultOpen" :title="t('admin.modelCatalog.syncResultTitle')" @close="syncResultOpen = false">
      <p v-if="syncResult">
        {{ t('admin.modelCatalog.syncResult', {
          updated: syncResult.updated ?? 0,
          discovered: syncResult.discovered ?? 0,
        }) }}
      </p>
      <p v-if="syncJobError" class="mt-2 text-sm text-red-600">{{ syncJobError }}</p>
      <ul v-if="syncResult?.warnings?.length" class="mt-2 list-disc pl-5 text-sm text-amber-700">
        <li v-for="(w, i) in syncResult.warnings" :key="i">{{ w }}</li>
      </ul>
      <template #footer>
        <button class="btn btn-primary" @click="syncResultOpen = false">{{ t('common.close') }}</button>
      </template>
    </BaseDialog>

    <BaseDialog :show="discoveryOpen" :title="t('admin.modelCatalog.discoveryTitle')" width="extra-wide" @close="discoveryOpen = false">
      <div class="space-y-4">
        <div class="flex flex-wrap items-center gap-3">
          <input
            v-model="discoverySearch"
            type="search"
            class="input flex-1 min-w-[12rem]"
            :placeholder="t('admin.modelCatalog.discoverySearch')"
            @input="debouncedDiscoveryLoad"
          />
          <button class="btn btn-secondary btn-sm" @click="toggleDiscoveryPageSelect">
            {{ t('admin.modelCatalog.discoverySelectAll') }}
          </button>
          <label class="flex items-center gap-2 text-sm">
            {{ t('admin.modelCatalog.fields.groupMode') }}
            <select v-model="discoveryGroupMode" class="input h-8">
              <option value="selected">{{ t('admin.modelCatalog.fields.groupModeSelected') }}</option>
              <option value="auto">{{ t('admin.modelCatalog.fields.groupModeAuto') }}</option>
            </select>
          </label>
        </div>

        <GroupSelector
          v-if="discoveryGroupMode === 'selected'"
          v-model="discoveryGroupIDs"
          :groups="groups"
          searchable="auto"
        />
        <p v-else class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.modelCatalog.fields.groupModeAutoHint') }}</p>

        <div v-if="discoveryLoading" class="py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        <div v-else-if="discoveryRows.length === 0" class="py-8 text-center text-sm text-gray-500">
          {{ t('admin.modelCatalog.discoveryEmpty') }}
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b text-left text-gray-500">
                <th class="py-2 pr-2 w-8"></th>
                <th class="py-2 pr-3">{{ t('admin.modelCatalog.columns.model') }}</th>
                <th class="py-2 pr-3">{{ t('admin.modelCatalog.columns.platform') }}</th>
                <th class="py-2 pr-3">{{ t('admin.modelCatalog.discoverySource') }}</th>
                <th class="py-2 pr-3">{{ t('admin.modelCatalog.columns.officialInput') }}</th>
                <th class="py-2">{{ t('admin.modelCatalog.columns.officialOutput') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in discoveryRows" :key="row.id" class="border-b border-gray-100 dark:border-dark-800">
                <td class="py-2 pr-2">
                  <input type="checkbox" :checked="discoverySelectedIds.includes(row.id)" @change="toggleDiscoverySelect(row.id)" />
                </td>
                <td class="py-2 pr-3 font-medium">{{ row.model_name }}</td>
                <td class="py-2 pr-3">{{ row.platform }}</td>
                <td class="py-2 pr-3">{{ row.source }}</td>
                <td class="py-2 pr-3">{{ formatPayloadPrice(row.payload, 'input_price') }}</td>
                <td class="py-2">{{ formatPayloadPrice(row.payload, 'output_price') }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="discoveryTotal > discoveryPageSize" class="flex items-center justify-between text-sm">
          <span>{{ t('admin.modelCatalog.discoveryPage', { page: discoveryPage, total: discoveryTotalPages }) }}</span>
          <div class="flex gap-2">
            <button class="btn btn-secondary btn-sm" :disabled="discoveryPage <= 1" @click="changeDiscoveryPage(discoveryPage - 1)">
              {{ t('common.previous') }}
            </button>
            <button class="btn btn-secondary btn-sm" :disabled="discoveryPage >= discoveryTotalPages" @click="changeDiscoveryPage(discoveryPage + 1)">
              {{ t('common.next') }}
            </button>
          </div>
        </div>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="discoveryOpen = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="discoverySelectedIds.length === 0 || discoveryImporting" @click="importSelectedDiscoveries">
          {{ t('admin.modelCatalog.discoveryImportSelected', { n: discoverySelectedIds.length }) }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import Icon from '@/components/icons/Icon.vue'
import adminModelCatalogAPI, {
  type AdminCatalogRow,
  type ModelDiscovery,
  type ModelSyncJob,
} from '@/api/admin/modelCatalog'
import groupsAPI from '@/api/admin/groups'
import { formatScaled } from '@/utils/pricing'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AdminGroup } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const rows = ref<AdminCatalogRow[]>([])
const activePanel = ref<'models' | 'groups'>('models')
const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const searchQuery = ref('')
const platformFilter = ref('')
const selectedIds = ref<number[]>([])
const editOpen = ref(false)
const syncResultOpen = ref(false)
const syncResult = ref<ModelSyncJob['result'] | null>(null)
const syncJobError = ref<string | null>(null)
const groups = ref<AdminGroup[]>([])
const plazaGroups = ref<AdminGroup[]>([])
const groupLoading = ref(false)
const batchGroupOpen = ref(false)
const batchGroupMode = ref<'auto' | 'selected'>('auto')
const batchGroupIDs = ref<number[]>([])

const discoveryOpen = ref(false)
const discoveryLoading = ref(false)
const discoveryImporting = ref(false)
const discoveryRows = ref<ModelDiscovery[]>([])
const discoveryTotal = ref(0)
const discoverySearch = ref('')
const discoverySelectedIds = ref<number[]>([])
const discoveryPage = ref(1)
const discoveryPageSize = 50
const discoveryGroupMode = ref<'auto' | 'selected'>('auto')
const discoveryGroupIDs = ref<number[]>([])

const editForm = reactive({
  id: 0,
  model_name: '',
  platform: 'openai',
  display_name: '',
  sort_order: 0,
  visible_public: false,
  visible_auth: true,
  group_mode: 'auto' as 'auto' | 'selected',
  group_ids: [] as number[],
  official_input_price_million: null as number | null,
  official_output_price_million: null as number | null,
  official_cache_read_price_million: null as number | null,
  official_cache_write_price_million: null as number | null,
  official_input_manual: false,
  official_output_manual: false,
  official_cache_read_manual: false,
  official_cache_write_manual: false,
})

const originalOfficialPrices = reactive({
  input: null as number | null,
  output: null as number | null,
  cacheRead: null as number | null,
  cacheWrite: null as number | null,
})

const columns = computed(() => [
  { key: 'select', label: '', sortable: false },
  { key: 'model_name', label: t('admin.modelCatalog.columns.model'), sortable: true },
  { key: 'platform', label: t('admin.modelCatalog.columns.platform'), sortable: true },
  { key: 'group_ids', label: t('admin.modelCatalog.columns.groups'), sortable: false },
  { key: 'official_input_price', label: t('admin.modelCatalog.columns.officialInput'), sortable: false },
  { key: 'official_output_price', label: t('admin.modelCatalog.columns.officialOutput'), sortable: false },
  { key: 'visible_public', label: t('admin.modelCatalog.columns.public'), sortable: false },
  { key: 'visible_auth', label: t('admin.modelCatalog.columns.auth'), sortable: false },
  { key: 'actions', label: t('common.actions'), sortable: false },
])

const filteredRows = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter(
    (r) =>
      r.model_name.toLowerCase().includes(q) ||
      r.platform.toLowerCase().includes(q),
  )
})

const discoveryTotalPages = computed(() => Math.max(1, Math.ceil(discoveryTotal.value / discoveryPageSize)))

let debounceTimer: ReturnType<typeof setTimeout> | null = null
function debouncedLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(loadCatalog, 300)
}

let discoveryDebounceTimer: ReturnType<typeof setTimeout> | null = null
function debouncedDiscoveryLoad() {
  if (discoveryDebounceTimer) clearTimeout(discoveryDebounceTimer)
  discoveryDebounceTimer = setTimeout(() => {
    discoveryPage.value = 1
    void loadDiscoveries()
  }, 300)
}

function formatPrice(v: number | null | undefined): string {
  if (v == null) return '—'
  return formatScaled(v, 1_000_000)
}

function formatPayloadPrice(payload: Record<string, unknown>, key: string): string {
  const raw = payload[key]
  if (typeof raw !== 'number') return '—'
  return formatScaled(raw, 1_000_000)
}

function groupLabel(id: number): string {
  return groups.value.find((group) => group.id === id)?.name ?? `#${id}`
}

function subscriptionTypeLabel(value: string): string {
  return value === 'subscription' ? t('modelPlaza.admin.subscription') : t('modelPlaza.admin.standard')
}

function platformLabel(value: string): string {
  const key: Record<string, string> = {
    openai: 'platformOpenai',
    anthropic: 'platformAnthropic',
    gemini: 'platformGemini',
    antigravity: 'platformAntigravity',
    grok: 'platformGrok',
    composite: 'platformComposite'
  }
  return t(`modelPlaza.admin.${key[value] ?? 'platformComposite'}`)
}

function groupStatusLabel(value: string): string {
  return value === 'inactive' ? t('modelPlaza.admin.inactive') : t('modelPlaza.admin.active')
}

async function loadGroups() {
  try {
    groups.value = await groupsAPI.getAll()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelCatalog.groupLoadFailed')))
  }
}

async function loadCatalog() {
  loading.value = true
  try {
    rows.value = await adminModelCatalogAPI.listCatalog({
      platform: platformFilter.value || undefined,
      search: searchQuery.value || undefined,
    })
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelCatalog.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function loadDiscoveries() {
  discoveryLoading.value = true
  try {
    const result = await adminModelCatalogAPI.listDiscoveries({
      status: 'new',
      search: discoverySearch.value || undefined,
      limit: discoveryPageSize,
      offset: (discoveryPage.value - 1) * discoveryPageSize,
    })
    discoveryRows.value = result.items
    discoveryTotal.value = result.total
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelCatalog.discoveryLoadFailed')))
  } finally {
    discoveryLoading.value = false
  }
}

function openDiscoveryPanel() {
  discoveryOpen.value = true
  discoverySelectedIds.value = []
  discoveryPage.value = 1
  discoveryGroupMode.value = 'auto'
  discoveryGroupIDs.value = []
  void loadDiscoveries()
}

function toggleSelect(id: number) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  } else {
    selectedIds.value = [...selectedIds.value, id]
  }
}

function toggleDiscoverySelect(id: number) {
  if (discoverySelectedIds.value.includes(id)) {
    discoverySelectedIds.value = discoverySelectedIds.value.filter((x) => x !== id)
  } else {
    discoverySelectedIds.value = [...discoverySelectedIds.value, id]
  }
}

function toggleDiscoveryPageSelect() {
  const pageIds = discoveryRows.value.map((r) => r.id)
  const allSelected = pageIds.every((id) => discoverySelectedIds.value.includes(id))
  if (allSelected) {
    discoverySelectedIds.value = discoverySelectedIds.value.filter((id) => !pageIds.includes(id))
  } else {
    const merged = new Set([...discoverySelectedIds.value, ...pageIds])
    discoverySelectedIds.value = [...merged]
  }
}

function changeDiscoveryPage(page: number) {
  discoveryPage.value = page
  void loadDiscoveries()
}

async function importSelectedDiscoveries() {
  if (!discoverySelectedIds.value.length) {
    appStore.showError(t('admin.modelCatalog.importSelectRequired'))
    return
  }
  discoveryImporting.value = true
  try {
    const n = await adminModelCatalogAPI.importDiscoveries({
      ids: discoverySelectedIds.value,
      group_ids: discoveryGroupMode.value === 'selected' ? discoveryGroupIDs.value : null,
    })
    appStore.showSuccess(t('admin.modelCatalog.importDone', { n }))
    discoveryOpen.value = false
    discoverySelectedIds.value = []
    await loadCatalog()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelCatalog.importSelectRequired')))
  } finally {
    discoveryImporting.value = false
  }
}

function openCreate() {
  Object.assign(editForm, {
    id: 0,
    model_name: '',
    platform: 'openai',
    display_name: '',
    sort_order: rows.value.length * 10,
    visible_public: false,
    visible_auth: true,
    group_mode: 'auto',
    group_ids: [],
    official_input_manual: false,
    official_output_manual: false,
    official_cache_read_manual: false,
    official_cache_write_manual: false,
  })
  Object.assign(originalOfficialPrices, { input: null, output: null, cacheRead: null, cacheWrite: null })
  editOpen.value = true
}

function openEdit(row: AdminCatalogRow) {
  Object.assign(editForm, {
    id: row.id,
    model_name: row.model_name,
    platform: row.platform,
    display_name: row.display_name ?? '',
    sort_order: row.sort_order,
    visible_public: row.visible_public,
    visible_auth: row.visible_auth,
    group_mode: row.group_ids == null ? 'auto' : 'selected',
    group_ids: row.group_ids ?? [],
    official_input_price_million: toPerMillion(row.official_input_price),
    official_output_price_million: toPerMillion(row.official_output_price),
    official_cache_read_price_million: toPerMillion(row.official_cache_read_price),
    official_cache_write_price_million: toPerMillion(row.official_cache_write_price),
    official_input_manual: row.official_input_manual,
    official_output_manual: row.official_output_manual,
    official_cache_read_manual: row.official_cache_read_manual,
    official_cache_write_manual: row.official_cache_write_manual,
  })
  Object.assign(originalOfficialPrices, {
    input: toPerMillion(row.official_input_price),
    output: toPerMillion(row.official_output_price),
    cacheRead: toPerMillion(row.official_cache_read_price),
    cacheWrite: toPerMillion(row.official_cache_write_price),
  })
  editOpen.value = true
}

async function saveEdit() {
  saving.value = true
  try {
    await adminModelCatalogAPI.saveCatalogEntry({
      id: editForm.id,
      model_name: editForm.model_name,
      platform: editForm.platform,
      display_name: editForm.display_name || null,
      sort_order: editForm.sort_order,
      visible_public: editForm.visible_public,
      visible_auth: editForm.visible_auth,
      group_ids: editForm.group_mode === 'selected' ? editForm.group_ids : null,
      // Paid price remains owned by the channel and group billing configuration.
      // This form only controls reference prices displayed in the model plaza.
      official_input_price: fromPerMillion(editForm.official_input_price_million),
      official_output_price: fromPerMillion(editForm.official_output_price_million),
      official_cache_read_price: fromPerMillion(editForm.official_cache_read_price_million),
      official_cache_write_price: fromPerMillion(editForm.official_cache_write_price_million),
      official_input_manual: editForm.official_input_price_million !== originalOfficialPrices.input
        ? editForm.official_input_price_million != null
        : editForm.official_input_manual,
      official_output_manual: editForm.official_output_price_million !== originalOfficialPrices.output
        ? editForm.official_output_price_million != null
        : editForm.official_output_manual,
      official_cache_read_manual: editForm.official_cache_read_price_million !== originalOfficialPrices.cacheRead
        ? editForm.official_cache_read_price_million != null
        : editForm.official_cache_read_manual,
      official_cache_write_manual: editForm.official_cache_write_price_million !== originalOfficialPrices.cacheWrite
        ? editForm.official_cache_write_price_million != null
        : editForm.official_cache_write_manual,
    })
    editOpen.value = false
    await loadCatalog()
    appStore.showSuccess(t('common.saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelCatalog.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function removeRow(row: AdminCatalogRow) {
  if (!confirm(t('admin.modelCatalog.deleteConfirm', { name: row.model_name }))) return
  await adminModelCatalogAPI.deleteCatalogEntry(row.id)
  await loadCatalog()
}

async function patchVisibility(row: AdminCatalogRow, visiblePublic?: boolean, visibleAuth?: boolean) {
  await adminModelCatalogAPI.batchVisibility({
    ids: [row.id],
    visible_public: visiblePublic,
    visible_auth: visibleAuth,
  })
  await loadCatalog()
}

async function batchSetPublic(on: boolean) {
  if (!selectedIds.value.length) return
  await adminModelCatalogAPI.batchVisibility({ ids: selectedIds.value, visible_public: on })
  await loadCatalog()
}

function openBatchGroups() {
  batchGroupMode.value = 'auto'
  batchGroupIDs.value = []
  batchGroupOpen.value = true
}

async function saveBatchGroups() {
  if (!selectedIds.value.length) return
  saving.value = true
  try {
    await adminModelCatalogAPI.batchGroups({
      ids: selectedIds.value,
      group_ids: batchGroupMode.value === 'selected' ? batchGroupIDs.value : null,
    })
    batchGroupOpen.value = false
    selectedIds.value = []
    await loadCatalog()
    appStore.showSuccess(t('common.saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelCatalog.saveFailed')))
  } finally {
    saving.value = false
  }
}

function toPerMillion(value: number | null | undefined): number | null {
  return value == null ? null : value * 1_000_000
}

function fromPerMillion(value: number | string | null): number | null {
  if (value == null || value === '') return null
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric / 1_000_000 : null
}

async function pollSyncJob(id: string) {
  for (let i = 0; i < 120; i++) {
    await new Promise((r) => setTimeout(r, 1500))
    const job = await adminModelCatalogAPI.getSyncJob(id)
    if (job.status === 'succeeded' || job.status === 'failed') {
      syncResult.value = job.result ?? null
      syncJobError.value = job.status === 'failed' ? (job.error || t('admin.modelCatalog.syncFailed')) : null
      syncResultOpen.value = true
      syncing.value = false
      await loadCatalog()
      return
    }
  }
  syncing.value = false
}

async function startSync() {
  syncing.value = true
  syncJobError.value = null
  try {
    const job = await adminModelCatalogAPI.createSyncJob()
    void pollSyncJob(job.id)
  } catch (err: unknown) {
    syncing.value = false
    appStore.showError(extractApiErrorMessage(err, t('admin.modelCatalog.syncFailed')))
  }
}

async function loadPlazaGroups() {
  groupLoading.value = true
  try { plazaGroups.value = await groupsAPI.getAll() } finally { groupLoading.value = false }
}

onMounted(() => {
  void Promise.all([loadCatalog(), loadGroups(), loadPlazaGroups()])
})
</script>
