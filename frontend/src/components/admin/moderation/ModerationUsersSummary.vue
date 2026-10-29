<template>
  <section class="space-y-4" data-test="moderation-user-summary">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.moderationUsers.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.description') }}</p>
      </div>
      <div class="flex flex-wrap items-end gap-2">
        <form class="flex flex-wrap items-end gap-2" @submit.prevent="search">
          <label class="block text-sm text-gray-700 dark:text-gray-300">
            {{ t('admin.moderationUsers.uid') }}
            <input v-model.trim="uidInput" type="number" min="1" step="1" class="input mt-1 w-36" data-test="uid-search" />
          </label>
          <label class="block text-sm text-gray-700 dark:text-gray-300">
            {{ t('admin.moderationUsers.stateFilter') }}
            <select v-model="filter" class="input mt-1" data-test="state-filter" @change="search">
              <option value="all">{{ t('admin.moderationUsers.all') }}</option>
              <option value="enforced">{{ t('admin.moderationUsers.enforced') }}</option>
              <option value="exempt">{{ t('admin.moderationUsers.exempt') }}</option>
              <option value="auto_ban_exempt">{{ t('admin.moderationUsers.autoBanExempt') }}</option>
              <option value="keyword_exempt">{{ t('admin.moderationUsers.keywordExempt') }}</option>
              <option value="api_audit_exempt">{{ t('admin.moderationUsers.apiAuditExempt') }}</option>
              <option value="email_exempt">{{ t('admin.moderationUsers.emailExempt') }}</option>
            </select>
          </label>
          <button type="submit" class="btn btn-primary" data-test="search-users" :disabled="busy || !uidIsValid">
            {{ t('common.search') }}
          </button>
        </form>
        <button type="button" class="btn btn-secondary" data-test="refresh-users" :disabled="busy" @click="load">
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <p v-else-if="busy" role="status" class="text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</p>
    <p v-else-if="items.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.empty') }}</p>
    <div v-else class="overflow-x-auto rounded-md border border-gray-200 dark:border-dark-700">
      <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
        <thead class="bg-gray-50 dark:bg-dark-800">
          <tr>
            <th class="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.uid') }}</th>
            <th class="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.status') }}</th>
            <th class="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.count') }}</th>
            <th class="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.reason') }}</th>
            <th class="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-800">
          <tr v-for="item in items" :key="item.user_id">
            <td class="whitespace-nowrap px-4 py-3 font-medium text-gray-900 dark:text-white">{{ item.user_id }}</td>
            <td class="whitespace-nowrap px-4 py-3 text-gray-700 dark:text-gray-300">{{ item.status }}</td>
            <td class="whitespace-nowrap px-4 py-3 text-gray-700 dark:text-gray-300">{{ item.current_count }}</td>
            <td class="max-w-sm break-words px-4 py-3 text-gray-700 dark:text-gray-300">{{ item.reason || '—' }}</td>
            <td class="whitespace-nowrap px-4 py-3 text-right">
              <button type="button" class="btn btn-secondary btn-sm" :data-test="`open-user-${item.user_id}`" :disabled="actionBusy" @click="openUser(item.user_id)">
                {{ t('admin.moderationUsers.details') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="total > pageSize" class="flex items-center justify-end gap-3 text-sm">
      <span class="text-gray-500 dark:text-gray-400">{{ page }} / {{ pageCount }} · {{ total }}</span>
      <button type="button" class="btn btn-secondary btn-sm" data-test="previous-users-page" :disabled="busy || page <= 1" @click="changePage(page - 1)">{{ t('pagination.previous') }}</button>
      <button type="button" class="btn btn-secondary btn-sm" data-test="next-users-page" :disabled="busy || page >= pageCount" @click="changePage(page + 1)">{{ t('pagination.next') }}</button>
    </div>

    <BaseDialog :show="selectedUserID !== null" :title="t('admin.moderationUsers.details')" width="narrow" :close-on-escape="!actionBusy" :show-close-button="!actionBusy" @close="closeUser">
      <div class="mb-3 flex justify-end">
        <button type="button" class="btn btn-secondary btn-sm" data-test="refresh-user-detail" :disabled="detailBusy || actionBusy" @click="refreshDetail">{{ t('common.refresh') }}</button>
      </div>
      <div v-if="detailBusy" role="status" class="py-6 text-sm text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="detailError && !detail" role="alert" class="py-2 text-sm text-red-600 dark:text-red-300">{{ detailError }}</div>
      <div v-else-if="detail" class="space-y-4">
        <p v-if="detailError" role="alert" class="text-sm text-red-600 dark:text-red-300">{{ detailError }}</p>
        <dl class="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.uid') }}</dt><dd class="text-gray-900 dark:text-white">{{ detail.user_id }}</dd>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.status') }}</dt><dd class="text-gray-900 dark:text-white">{{ detail.status }}</dd>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.count') }}</dt><dd class="text-gray-900 dark:text-white">{{ detail.current_count }}</dd>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.generation') }}</dt><dd class="text-gray-900 dark:text-white">{{ detail.generation }}</dd>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.ownership') }}</dt><dd class="text-gray-900 dark:text-white">{{ detail.owned_disable ? t('admin.moderationUsers.moderationOwned') : t('admin.moderationUsers.otherSource') }}</dd>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.reason') }}</dt><dd class="break-words text-gray-900 dark:text-white">{{ detail.reason || '—' }}</dd>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.actor') }}</dt><dd class="text-gray-900 dark:text-white">{{ detail.actor_id ?? '—' }}</dd>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.moderationUsers.updatedAt') }}</dt><dd class="text-gray-900 dark:text-white">{{ detail.updated_at ? new Date(detail.updated_at).toLocaleString() : '—' }}</dd>
        </dl>

        <div class="border-t border-gray-200 pt-4 dark:border-dark-700">
          <h3 class="font-medium text-gray-900 dark:text-white">{{ t('admin.moderationUsers.exemptions') }}</h3>
          <div class="mt-3 grid gap-2 sm:grid-cols-2">
            <label v-for="item in exemptionOptions" :key="item.key" class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="exemptions[item.key]" type="checkbox" class="mt-1" :data-test="`exemption-${item.key}`" :disabled="actionBusy" />
              <span>{{ t(item.label) }}</span>
            </label>
          </div>
          <p class="mt-3 text-sm text-gray-700 dark:text-gray-300">{{ canUnban ? t('admin.moderationUsers.unbanScope') : t('admin.moderationUsers.otherSource') }}</p>
          <form class="mt-3 space-y-3" @submit.prevent="applyAction('exemption')">
            <label class="block text-sm text-gray-700 dark:text-gray-300">
              {{ t('admin.moderationUsers.reason') }}
              <textarea v-model="reason" class="input mt-1 min-h-20 w-full" maxlength="500" data-test="unban-reason" :disabled="actionBusy" />
            </label>
            <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="confirmed" type="checkbox" class="mt-1" data-test="unban-confirm" :disabled="actionBusy" />
              <span>{{ t('admin.moderationUsers.confirm', { userId: detail.user_id }) }}</span>
            </label>
            <p v-if="detailError" role="alert" class="text-sm text-red-600 dark:text-red-300">{{ detailError }}</p>
            <p v-if="saved" role="status" class="text-sm text-emerald-700 dark:text-emerald-300">{{ t('admin.moderationUsers.saved') }}</p>
            <div class="flex flex-wrap gap-2">
              <button type="button" class="btn btn-secondary" data-test="save-exemptions" :disabled="!exemptionsChanged || !canSubmitAction || actionBusy" :aria-busy="actionBusy" @click="applyAction('exemption')">{{ t('admin.moderationUsers.saveExemptions') }}</button>
              <button type="button" class="btn btn-secondary" data-test="reset-count" :disabled="!canSubmitAction || actionBusy" @click="applyAction('reset')">{{ t('admin.moderationUsers.reset') }}</button>
              <button v-if="canUnban" type="button" class="btn btn-primary" data-test="unban-user" :disabled="!canSubmitAction || actionBusy" :aria-busy="actionBusy" @click="applyAction('unban')">{{ actionBusy ? t('common.processing') : t('admin.moderationUsers.unban') }}</button>
            </div>
          </form>
        </div>
      </div>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { getModerationUser, listModerationUsers, manageModerationUser, type ModerationUserFilter, type ModerationUserState } from '@/api/admin/riskControl'

const { t } = useI18n()
const pageSize = 20
const page = ref(1)
const total = ref(0)
const items = ref<ModerationUserState[]>([])
const uidInput = ref('')
const appliedUserID = ref<number | undefined>()
const filter = ref<ModerationUserFilter>('all')
const busy = ref(false)
const error = ref('')
const selectedUserID = ref<number | null>(null)
const detail = ref<ModerationUserState | null>(null)
const detailBusy = ref(false)
const actionBusy = ref(false)
const detailError = ref('')
const reason = ref('')
const confirmed = ref(false)
const saved = ref(false)
const exemptions = ref({ auto_ban_exempt: false, keyword_exempt: false, api_audit_exempt: false, email_exempt: false })
const exemptionOptions = [
  { key: 'auto_ban_exempt' as const, label: 'admin.moderationUsers.autoBanExempt' },
  { key: 'keyword_exempt' as const, label: 'admin.moderationUsers.keywordExempt' },
  { key: 'api_audit_exempt' as const, label: 'admin.moderationUsers.apiAuditExempt' },
  { key: 'email_exempt' as const, label: 'admin.moderationUsers.emailExempt' },
]
let listRequest = 0
let detailRequest = 0

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const uidIsValid = computed(() => uidInput.value === '' || (Number.isSafeInteger(Number(uidInput.value)) && Number(uidInput.value) > 0))
const canUnban = computed(() => detail.value?.status === 'disabled' && detail.value.owned_disable === true)
const exemptionsChanged = computed(() => Boolean(detail.value && exemptionOptions.some(({ key }) => exemptions.value[key] !== detail.value?.[key])))
const canSubmitAction = computed(() => reason.value.trim().length > 0 && [...reason.value.trim()].length <= 500 && confirmed.value)

function errorMessage(err: unknown) {
  const value = err as { response?: { data?: { message?: string } }; message?: string }
  return value.response?.data?.message || value.message || t('admin.moderationUsers.failed')
}

async function load() {
  const request = ++listRequest
  busy.value = true
  error.value = ''
  try {
    const result = await listModerationUsers({
      page: page.value,
      page_size: pageSize,
      state: filter.value,
      ...(appliedUserID.value ? { user_id: appliedUserID.value } : {}),
    })
    if (request === listRequest) {
      items.value = result.items
      total.value = result.total
    }
  } catch (err) {
    if (request === listRequest) {
      error.value = errorMessage(err)
      items.value = []
      total.value = 0
    }
  } finally {
    if (request === listRequest) busy.value = false
  }
}

function search() {
  if (!uidIsValid.value) return
  appliedUserID.value = uidInput.value ? Number(uidInput.value) : undefined
  page.value = 1
  void load()
}

function changePage(value: number) {
  page.value = value
  void load()
}

async function openUser(userID: number) {
  if (actionBusy.value) return
  selectedUserID.value = userID
  detail.value = null
  detailError.value = ''
  saved.value = false
  reason.value = ''
  confirmed.value = false
  await loadDetail(userID)
}

async function loadDetail(userID: number) {
  const request = ++detailRequest
  detailBusy.value = true
  detailError.value = ''
  try {
    const state = await getModerationUser(userID)
    if (request === detailRequest && selectedUserID.value === userID) {
      if (state.user_id !== userID) throw new Error(t('admin.moderationUsers.readbackFailed'))
      readDetail(state)
    }
  } catch (err) {
    if (request === detailRequest) detailError.value = errorMessage(err)
  } finally {
    if (request === detailRequest) detailBusy.value = false
  }
}

function refreshDetail() {
  if (selectedUserID.value !== null && !actionBusy.value) void loadDetail(selectedUserID.value)
}

function closeUser() {
  if (actionBusy.value) return
  selectedUserID.value = null
  detail.value = null
  detailError.value = ''
}

function readDetail(state: ModerationUserState) {
  detail.value = state
  exemptions.value = {
    auto_ban_exempt: state.auto_ban_exempt,
    keyword_exempt: state.keyword_exempt,
    api_audit_exempt: state.api_audit_exempt,
    email_exempt: state.email_exempt,
  }
}

async function applyAction(action: 'unban' | 'reset' | 'exemption') {
  const current = detail.value
  if (!current || (action === 'unban' && !canUnban.value) || (action === 'exemption' && !exemptionsChanged.value) || !canSubmitAction.value || actionBusy.value) return
  const targetUserID = current.user_id
  const targetRequest = detailRequest
  const requestedExemptions = { ...exemptions.value }
  actionBusy.value = true
  detailError.value = ''
  saved.value = false
  let writeError: unknown
  try {
    await manageModerationUser(current.user_id, {
      action,
      version: current.version,
      reason: reason.value.trim(),
      confirm: confirmed.value,
      ...(action === 'exemption' ? requestedExemptions : {}),
    })
  } catch (err) {
    writeError = err
  }

  try {
    const verified = await getModerationUser(targetUserID)
    if (selectedUserID.value !== targetUserID || detailRequest !== targetRequest) return
    if (verified.user_id !== targetUserID) throw new Error(t('admin.moderationUsers.readbackFailed'))
    readDetail(verified)
    if (writeError) throw writeError
    if (action === 'unban' && (verified.status !== 'active' || verified.owned_disable)) {
      throw new Error(t('admin.moderationUsers.readbackFailed'))
    }
    if (action === 'reset' && verified.current_count !== 0) throw new Error(t('admin.moderationUsers.readbackFailed'))
    if (action === 'exemption' && exemptionOptions.some(({ key }) => verified[key] !== requestedExemptions[key])) {
      throw new Error(t('admin.moderationUsers.readbackFailed'))
    }
    reason.value = ''
    confirmed.value = false
    saved.value = true
    await load()
  } catch (err) {
    if (selectedUserID.value === targetUserID && detailRequest === targetRequest) {
      detailError.value = writeError ? errorMessage(writeError) : errorMessage(err)
    }
  } finally {
    if (selectedUserID.value === targetUserID && detailRequest === targetRequest) {
      confirmed.value = false
      actionBusy.value = false
    }
  }
}

onMounted(load)
</script>
