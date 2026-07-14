<template>
  <AppLayout>
    <TablePageLayout>
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
            <button class="btn btn-secondary" @click="importFromDiscovery">
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
          <button class="btn btn-secondary btn-sm" @click="batchMultiply(0.8)">
            {{ t('admin.modelCatalog.batchMultiply', { mult: '0.8' }) }}
          </button>
          <button class="btn btn-secondary btn-sm" @click="fillLiteLLM">
            {{ t('admin.modelCatalog.fillLiteLLM') }}
          </button>
        </div>

        <DataTable :columns="columns" :data="filteredRows" :loading="loading">
          <template #cell-select="{ row }">
            <input type="checkbox" :checked="selectedIds.includes(row.id)" @change="toggleSelect(row.id)" />
          </template>
          <template #cell-model_name="{ value }">
            <span class="font-medium">{{ value }}</span>
          </template>
          <template #cell-input_price="{ row }">
            {{ formatPrice(row.input_price) }}
          </template>
          <template #cell-output_price="{ row }">
            {{ formatPrice(row.output_price) }}
          </template>
          <template #cell-visible_public="{ row }">
            <Toggle :model-value="row.visible_public" @update:model-value="(v: boolean) => patchVisibility(row, v, undefined)" />
          </template>
          <template #cell-actions="{ row }">
            <button class="btn btn-ghost btn-sm" @click="openEdit(row)">{{ t('common.edit') }}</button>
            <button class="btn btn-ghost btn-sm text-red-600" @click="removeRow(row)">{{ t('common.delete') }}</button>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <BaseDialog :show="editOpen" :title="editForm.id ? t('admin.modelCatalog.editTitle') : t('admin.modelCatalog.addTitle')" @close="editOpen = false">
      <div class="space-y-3">
        <label class="block text-sm">
          {{ t('admin.modelCatalog.fields.model') }}
          <input v-model="editForm.model_name" class="input mt-1 w-full" />
        </label>
        <label class="block text-sm">
          {{ t('admin.modelCatalog.fields.platform') }}
          <input v-model="editForm.platform" class="input mt-1 w-full" />
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
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="editOpen = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="saving" @click="saveEdit">{{ t('common.save') }}</button>
      </template>
    </BaseDialog>

    <BaseDialog :show="syncResultOpen" :title="t('admin.modelCatalog.syncResultTitle')" @close="syncResultOpen = false">
      <p v-if="syncResult">
        {{ t('admin.modelCatalog.syncResult', {
          updated: syncResult.updated ?? 0,
          discovered: syncResult.discovered ?? 0,
        }) }}
      </p>
      <ul v-if="syncResult?.warnings?.length" class="mt-2 list-disc pl-5 text-sm text-amber-700">
        <li v-for="(w, i) in syncResult.warnings" :key="i">{{ w }}</li>
      </ul>
      <template #footer>
        <button class="btn btn-primary" @click="syncResultOpen = false">{{ t('common.close') }}</button>
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
import Icon from '@/components/icons/Icon.vue'
import adminModelCatalogAPI, {
  type SiteModelCatalogEntry,
  type ModelSyncJob,
} from '@/api/admin/modelCatalog'
import { formatScaled } from '@/utils/pricing'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const rows = ref<SiteModelCatalogEntry[]>([])
const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const searchQuery = ref('')
const platformFilter = ref('')
const selectedIds = ref<number[]>([])
const editOpen = ref(false)
const syncResultOpen = ref(false)
const syncResult = ref<ModelSyncJob['result'] | null>(null)

const editForm = reactive({
  id: 0,
  model_name: '',
  platform: 'openai',
  sort_order: 0,
  visible_public: false,
  visible_auth: true,
})

const columns = computed(() => [
  { key: 'select', label: '', sortable: false },
  { key: 'model_name', label: t('admin.modelCatalog.columns.model'), sortable: true },
  { key: 'platform', label: t('admin.modelCatalog.columns.platform'), sortable: true },
  { key: 'input_price', label: t('admin.modelCatalog.columns.input'), sortable: false },
  { key: 'output_price', label: t('admin.modelCatalog.columns.output'), sortable: false },
  { key: 'visible_public', label: t('admin.modelCatalog.columns.public'), sortable: false },
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

let debounceTimer: ReturnType<typeof setTimeout> | null = null
function debouncedLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(loadCatalog, 300)
}

function formatPrice(v: number | null): string {
  if (v == null) return '—'
  return `$${formatScaled(v, 1_000_000)}`
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

function toggleSelect(id: number) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  } else {
    selectedIds.value = [...selectedIds.value, id]
  }
}

function openCreate() {
  Object.assign(editForm, {
    id: 0,
    model_name: '',
    platform: 'openai',
    sort_order: rows.value.length * 10,
    visible_public: false,
    visible_auth: true,
  })
  editOpen.value = true
}

function openEdit(row: SiteModelCatalogEntry) {
  Object.assign(editForm, {
    id: row.id,
    model_name: row.model_name,
    platform: row.platform,
    sort_order: row.sort_order,
    visible_public: row.visible_public,
    visible_auth: row.visible_auth,
  })
  editOpen.value = true
}

async function saveEdit() {
  saving.value = true
  try {
    await adminModelCatalogAPI.saveCatalogEntry({ ...editForm })
    editOpen.value = false
    await loadCatalog()
    appStore.showSuccess(t('common.saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelCatalog.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function removeRow(row: SiteModelCatalogEntry) {
  if (!confirm(t('admin.modelCatalog.deleteConfirm', { name: row.model_name }))) return
  await adminModelCatalogAPI.deleteCatalogEntry(row.id)
  await loadCatalog()
}

async function patchVisibility(row: SiteModelCatalogEntry, visiblePublic: boolean, visibleAuth?: boolean) {
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

async function batchMultiply(mult: number) {
  if (!selectedIds.value.length) return
  await adminModelCatalogAPI.batchPrices({ ids: selectedIds.value, multiplier: mult })
  await loadCatalog()
}

async function fillLiteLLM() {
  const ids = selectedIds.value.length ? selectedIds.value : undefined
  const n = await adminModelCatalogAPI.fillFromLiteLLM(ids)
  appStore.showSuccess(t('admin.modelCatalog.fillDone', { n }))
  await loadCatalog()
}

async function importFromDiscovery() {
  const n = await adminModelCatalogAPI.importDiscoveries({})
  appStore.showSuccess(t('admin.modelCatalog.importDone', { n }))
  await loadCatalog()
}

async function pollSyncJob(id: string) {
  for (let i = 0; i < 120; i++) {
    await new Promise((r) => setTimeout(r, 1500))
    const job = await adminModelCatalogAPI.getSyncJob(id)
    if (job.status === 'succeeded' || job.status === 'failed') {
      syncResult.value = job.result ?? null
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
  try {
    const job = await adminModelCatalogAPI.createSyncJob()
    void pollSyncJob(job.id)
  } catch (err: unknown) {
    syncing.value = false
    appStore.showError(extractApiErrorMessage(err, t('admin.modelCatalog.syncFailed')))
  }
}

onMounted(loadCatalog)
</script>
