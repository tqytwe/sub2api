<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.ipRisk.actionsView.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.actionsView.description') }}</p>
      </div>
      <button type="button" class="btn btn-secondary" :disabled="loading" @click="load">
        <Icon name="refresh" size="sm" class="mr-2" />
        {{ t('common.refresh') }}
      </button>
    </div>

    <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-800/60 dark:bg-red-950/30 dark:text-red-200">
      <div class="flex items-start justify-between gap-3">
        <span>{{ error }}</span>
        <button type="button" class="btn btn-ghost btn-sm" @click="load">{{ t('common.retry') }}</button>
      </div>
    </div>

    <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
      <div v-if="loading" class="flex min-h-[360px] items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        <Icon name="refresh" size="md" class="mr-2" />
        {{ t('common.loading') }}
      </div>
      <div v-else-if="actions.length === 0" class="flex min-h-[360px] flex-col items-center justify-center px-6 text-center">
        <Icon name="clipboard" size="xl" class="text-gray-400 dark:text-gray-500" />
        <div class="mt-3 text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.ipRisk.actionsView.empty') }}</div>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.actionsView.emptyHint') }}</p>
      </div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900 dark:text-gray-400">
            <tr>
              <th class="px-4 py-3 text-left">{{ t('admin.ipRisk.actionsView.action') }}</th>
              <th class="px-4 py-3 text-left">{{ t('admin.ipRisk.actionsView.actor') }}</th>
              <th class="px-4 py-3 text-left">{{ t('admin.ipRisk.actionsView.impact') }}</th>
              <th class="px-4 py-3 text-left">{{ t('admin.ipRisk.actionsView.reason') }}</th>
              <th class="px-4 py-3 text-left">{{ t('admin.ipRisk.actionsView.time') }}</th>
              <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="action in actions" :key="action.id">
              <td class="whitespace-nowrap px-4 py-3">
                <div class="font-medium text-gray-900 dark:text-white">{{ actionLabel(action.action_type) }}</div>
                <div class="mt-1 flex items-center gap-2">
                  <span :class="['badge', statusClass(action.status)]">{{ action.status }}</span>
                  <span v-if="action.case_id" class="text-xs text-gray-500 dark:text-gray-400">#{{ action.case_id }}</span>
                </div>
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-gray-600 dark:text-gray-300">
                {{ action.actor_type === 'system' ? t('admin.ipRisk.actionsView.system') : `#${action.actor_user_id || '-'}` }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                <div>{{ t('admin.ipRisk.actionsView.completed', { count: resultNumber(action, 'completed_items') }) }}</div>
                <div v-if="resultNumber(action, 'failed_items') || resultNumber(action, 'conflict_items')" class="mt-1 text-red-600 dark:text-red-400">
                  {{ t('admin.ipRisk.actionsView.failed', { count: resultNumber(action, 'failed_items') + resultNumber(action, 'conflict_items') }) }}
                </div>
                <div v-if="resultNumber(action, 'skipped_items')" class="mt-1 text-amber-600 dark:text-amber-400">
                  {{ t('admin.ipRisk.actionsView.skipped', { count: resultNumber(action, 'skipped_items') }) }}
                </div>
              </td>
              <td class="min-w-64 px-4 py-3 text-gray-600 dark:text-gray-300">{{ action.reason }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(action.created_at) }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-right">
                <button
                  v-if="action.rollback_status === 'eligible'"
                  type="button"
                  class="btn btn-secondary btn-sm"
                  @click="openRollback(action)"
                >
                  {{ t('admin.ipRisk.actionsView.rollback') }}
                </button>
                <span v-else :class="['badge', rollbackClass(action.rollback_status)]">
                  {{ rollbackLabel(action.rollback_status) }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <Pagination
        v-if="total > 0"
        :page="page"
        :page-size="pageSize"
        :total="total"
        @update:page="changePage"
        @update:page-size="changePageSize"
      />
    </div>
  </section>

  <BaseDialog
    :show="Boolean(rollbackTarget)"
    :title="t('admin.ipRisk.actionsView.rollbackTitle')"
    width="narrow"
    @close="closeRollback"
  >
    <div class="space-y-4">
      <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-200">
        {{ t('admin.ipRisk.actionsView.rollbackHint') }}
      </div>
      <div>
        <label class="input-label" for="ip-risk-rollback-reason">{{ t('admin.ipRisk.actionsView.rollbackReason') }}</label>
        <textarea id="ip-risk-rollback-reason" v-model="rollbackReason" rows="3" maxlength="1000" class="input min-h-[96px] resize-y"></textarea>
      </div>
      <div v-if="rollbackResult" class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
        {{ t('admin.ipRisk.actionsView.rollbackResult', {
          completed: resultNumber(rollbackResult, 'completed_items'),
          conflicts: resultNumber(rollbackResult, 'conflict_items'),
          skipped: resultNumber(rollbackResult, 'skipped_items'),
        }) }}
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" :disabled="rollingBack" @click="closeRollback">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-danger" :disabled="rollingBack || !rollbackReason.trim()" @click="rollback">
          {{ rollingBack ? t('admin.ipRisk.actionsView.rollingBack') : t('admin.ipRisk.actionsView.confirmRollback') }}
        </button>
      </div>
    </template>
  </BaseDialog>

</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { isStepUpCancelled, type StepUpController } from '@/composables/useStepUp'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import type { RiskActionRecord, RiskActionType } from './types'

const props = defineProps<{
  stepUp: StepUpController
}>()
const { t } = useI18n()
const appStore = useAppStore()
const actions = ref<RiskActionRecord[]>([])
const loading = ref(false)
const error = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rollbackTarget = ref<RiskActionRecord | null>(null)
const rollbackReason = ref('')
const rollingBack = ref(false)
const rollbackResult = ref<RiskActionRecord | null>(null)

onMounted(load)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const response = await adminAPI.ipRisk.listActions(page.value, pageSize.value)
    actions.value = response.items
    total.value = response.total
  } catch (loadError) {
    error.value = extractApiErrorMessage(loadError, t('admin.ipRisk.actionsView.loadFailed'))
  } finally {
    loading.value = false
  }
}

function changePage(value: number) {
  page.value = value
  load()
}

function changePageSize(value: number) {
  pageSize.value = value
  page.value = 1
  load()
}

function openRollback(action: RiskActionRecord) {
  rollbackTarget.value = action
  rollbackReason.value = ''
  rollbackResult.value = null
}

function closeRollback() {
  if (rollingBack.value) return
  rollbackTarget.value = null
  rollbackResult.value = null
}

async function rollback() {
  if (!rollbackTarget.value || !rollbackReason.value.trim()) return
  rollingBack.value = true
  try {
    rollbackResult.value = await props.stepUp.run(
      () => adminAPI.ipRisk.rollbackAction(rollbackTarget.value!.id, rollbackReason.value.trim()),
      { promptBeforeAction: true },
    )
    appStore.showSuccess(t('admin.ipRisk.actionsView.rollbackCompleted'))
    await load()
  } catch (rollbackError) {
    if (!isStepUpCancelled(rollbackError)) {
      appStore.showError(extractApiErrorMessage(rollbackError, t('admin.ipRisk.actionsView.rollbackFailed')))
    }
  } finally {
    rollingBack.value = false
  }
}

function actionLabel(action: RiskActionType) {
  return t(`admin.ipRisk.actionTypes.${action}`)
}

function statusClass(status: string) {
  if (status === 'completed' || status === 'rolled_back') return 'badge-success'
  if (status === 'partial') return 'badge-warning'
  if (status === 'failed') return 'badge-danger'
  return 'badge-gray'
}

function rollbackClass(status: string) {
  if (status === 'completed') return 'badge-success'
  if (status === 'partial' || status === 'conflict') return 'badge-warning'
  if (status === 'failed') return 'badge-danger'
  return 'badge-gray'
}

function rollbackLabel(status: string) {
  return t(`admin.ipRisk.rollbackStatus.${status}`)
}

function resultNumber(action: RiskActionRecord, key: string) {
  const value = action.result?.[key]
  return typeof value === 'number' ? value : Number(value || 0)
}
</script>
