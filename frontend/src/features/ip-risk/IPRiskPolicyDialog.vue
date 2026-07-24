<template>
  <BaseDialog
    :show="show"
    :title="t('admin.ipRisk.policyDialog.title')"
    width="extra-wide"
    :close-on-click-outside="false"
    @close="close"
  >
    <div class="flex border-b border-gray-200 dark:border-dark-700">
      <button
        v-for="tab in tabs"
        :key="tab.value"
        type="button"
        :class="[
          '-mb-px border-b-2 px-4 py-3 text-sm font-medium transition-colors',
          activeTab === tab.value
            ? 'border-primary-500 text-primary-600 dark:text-primary-400'
            : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200',
        ]"
        @click="activeTab = tab.value"
      >
        {{ tab.label }}
      </button>
    </div>

    <div v-if="loading" class="flex min-h-[360px] items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      <Icon name="refresh" size="md" class="mr-2" />
      {{ t('common.loading') }}
    </div>

    <div v-else-if="activeTab === 'detection' && config" class="space-y-6 pt-5">
      <section class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800/60 dark:bg-amber-950/30">
        <div class="flex items-start justify-between gap-4">
          <div class="flex items-start gap-3">
            <Icon name="shield" size="md" class="mt-0.5 shrink-0 text-amber-700 dark:text-amber-300" />
            <div>
              <h4 class="text-sm font-semibold text-amber-900 dark:text-amber-100">{{ t('admin.ipRisk.policyDialog.automationTitle') }}</h4>
              <p class="mt-1 text-sm text-amber-800 dark:text-amber-200">{{ t('admin.ipRisk.policyDialog.automationHint') }}</p>
            </div>
          </div>
          <label class="inline-flex cursor-pointer items-center gap-2">
            <input
              v-model="config.auto_block_enabled"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
            <span class="text-sm font-medium text-amber-900 dark:text-amber-100">
              {{ config.auto_block_enabled ? t('common.enabled') : t('common.disabled') }}
            </span>
          </label>
        </div>
        <p v-if="config.auto_block_enabled" class="mt-3 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800/60 dark:bg-red-950/30 dark:text-red-200">
          {{ t('admin.ipRisk.policyDialog.stepUpWarning') }}
        </p>
      </section>

      <section>
        <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ipRisk.policyDialog.registrationRules') }}</h4>
        <div class="mt-3 grid gap-3 md:grid-cols-3">
          <ConfigPair
            v-model:threshold="config.registration_10m_threshold"
            v-model:score="config.registration_10m_score"
            :label="t('admin.ipRisk.policyDialog.registration10m')"
          />
          <ConfigPair
            v-model:threshold="config.registration_1h_threshold"
            v-model:score="config.registration_1h_score"
            :label="t('admin.ipRisk.policyDialog.registration1h')"
          />
          <ConfigPair
            v-model:threshold="config.registration_24h_threshold"
            v-model:score="config.registration_24h_score"
            :label="t('admin.ipRisk.policyDialog.registration24h')"
          />
        </div>
      </section>

      <section>
        <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ipRisk.policyDialog.crossSignals') }}</h4>
        <div class="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          <ConfigPair v-model:threshold="config.shared_ua_3_threshold" v-model:score="config.shared_ua_3_score" :label="t('admin.ipRisk.policyDialog.sharedUa3')" />
          <ConfigPair v-model:threshold="config.shared_ua_5_threshold" v-model:score="config.shared_ua_5_score" :label="t('admin.ipRisk.policyDialog.sharedUa5')" />
          <ConfigPair v-model:threshold="config.email_pattern_threshold" v-model:score="config.email_pattern_score" :label="t('admin.ipRisk.policyDialog.emailPattern')" />
          <ConfigPair v-model:threshold="config.shared_api_ip_threshold" v-model:score="config.shared_api_ip_score" :label="t('admin.ipRisk.policyDialog.sharedApiIp')" />
          <ConfigPair v-model:threshold="config.rapid_behavior_threshold" v-model:score="config.rapid_behavior_score" :label="t('admin.ipRisk.policyDialog.rapidBehavior')" />
          <ConfigPair v-model:threshold="config.shared_signup_code_threshold" v-model:score="config.shared_signup_code_score" :label="t('admin.ipRisk.policyDialog.signupCode')" />
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <NumberField v-model="config.auto_block_score" :label="t('admin.ipRisk.policyDialog.autoBlockScore')" :min="90" :max="100" />
        <NumberField v-model="config.auto_block_min_registrations" :label="t('admin.ipRisk.policyDialog.autoBlockUsers')" :min="5" :max="500" />
        <NumberField v-model="config.event_retention_days" :label="t('admin.ipRisk.policyDialog.eventRetention')" :min="1" :max="365" />
        <NumberField v-model="config.case_retention_days" :label="t('admin.ipRisk.policyDialog.caseRetention')" :min="1" :max="3650" />
      </section>

      <label class="flex items-start gap-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <input v-model="config.historical_backfill_enabled" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
        <span>
          <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.ipRisk.policyDialog.historyBackfill') }}</span>
          <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.policyDialog.historyBackfillHint') }}</span>
        </span>
      </label>
    </div>

    <div v-else-if="activeTab === 'policies'" class="space-y-5 pt-5">
      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ editingPolicyId ? t('admin.ipRisk.policyDialog.editPolicy') : t('admin.ipRisk.policyDialog.addPolicy') }}
            </h4>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ipRisk.policyDialog.policyHint') }}</p>
          </div>
          <button v-if="editingPolicyId" type="button" class="btn btn-ghost btn-sm" @click="resetPolicyForm">
            {{ t('common.cancel') }}
          </button>
        </div>
        <div class="mt-4 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <div>
            <label class="input-label">{{ t('admin.ipRisk.policyDialog.mode') }}</label>
            <Select v-model="policyForm.mode" :options="policyModeOptions" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.ipRisk.policyDialog.exactIp') }}</label>
            <input v-model.trim="policyForm.exact_ip" class="input font-mono" placeholder="203.0.113.8" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.ipRisk.policyDialog.cidr') }}</label>
            <input v-model.trim="policyForm.ip_network" class="input font-mono" placeholder="203.0.113.0/24" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.ipRisk.policyDialog.expiresAt') }}</label>
            <input v-model="policyForm.expires_at" type="datetime-local" class="input" />
          </div>
        </div>
        <div class="mt-4">
          <label class="input-label">{{ t('admin.ipRisk.policyDialog.reason') }}</label>
          <input v-model.trim="policyForm.reason" maxlength="500" class="input" :placeholder="t('admin.ipRisk.policyDialog.policyReasonPlaceholder')" />
        </div>
        <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
          <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="policyForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            {{ t('common.enabled') }}
          </label>
          <button type="button" class="btn btn-primary" :disabled="savingPolicy || !canSavePolicy" @click="savePolicy">
            {{ savingPolicy ? t('common.saving') : editingPolicyId ? t('common.update') : t('common.add') }}
          </button>
        </div>
      </section>

      <section>
        <div v-if="policies.length === 0" class="rounded-lg border border-dashed border-gray-300 py-10 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
          {{ t('admin.ipRisk.policyDialog.noPolicies') }}
        </div>
        <div v-else class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 text-left">{{ t('admin.ipRisk.policyDialog.mode') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.ipRisk.policyDialog.target') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.ipRisk.policyDialog.reason') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.ipRisk.policyDialog.validity') }}</th>
                <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
              <tr v-for="policy in policies" :key="policy.id">
                <td class="whitespace-nowrap px-4 py-3">
                  <span :class="['badge', policyModeClass(policy.mode)]">{{ policyModeLabel(policy.mode) }}</span>
                </td>
                <td class="whitespace-nowrap px-4 py-3 font-mono text-xs text-gray-700 dark:text-gray-200">
                  {{ policy.exact_ip || policy.ip_network || '-' }}
                </td>
                <td class="max-w-xs px-4 py-3 text-gray-600 dark:text-gray-300">{{ policy.reason }}</td>
                <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-500 dark:text-gray-400">
                  {{ policy.expires_at ? formatDateTime(policy.expires_at) : t('admin.ipRisk.policyDialog.permanent') }}
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-right">
                  <button type="button" class="btn btn-ghost btn-sm" @click="editPolicy(policy)">{{ t('common.edit') }}</button>
                  <button type="button" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" @click="removePolicy(policy)">{{ t('common.delete') }}</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <button type="button" class="btn btn-secondary" :disabled="savingConfig || savingPolicy" @click="close">
          {{ t('common.close') }}
        </button>
        <button v-if="activeTab === 'detection'" type="button" class="btn btn-primary" :disabled="!config || savingConfig" @click="saveConfig">
          {{ savingConfig ? t('common.saving') : t('admin.ipRisk.policyDialog.saveConfig') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <BaseDialog
    :show="Boolean(deleteTarget)"
    :title="t('admin.ipRisk.policyDialog.deleteTitle')"
    width="narrow"
    :close-on-click-outside="false"
    @close="closeDelete"
  >
    <div v-if="deleteTarget" class="space-y-3">
      <div class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800/60 dark:bg-red-950/30 dark:text-red-200">
        {{ t('admin.ipRisk.policyDialog.deleteHint') }}
      </div>
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
        <div class="text-xs text-gray-500 dark:text-gray-400">{{ policyModeLabel(deleteTarget.mode) }}</div>
        <div class="mt-1 font-mono text-sm font-medium text-gray-900 dark:text-white">
          {{ deleteTarget.exact_ip || deleteTarget.ip_network || '-' }}
        </div>
        <div class="mt-2 text-sm text-gray-600 dark:text-gray-300">{{ deleteTarget.reason }}</div>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" :disabled="deletingPolicy" @click="closeDelete">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-danger" :disabled="deletingPolicy" @click="confirmDelete">
          {{ deletingPolicy ? t('admin.ipRisk.policyDialog.deleting') : t('admin.ipRisk.policyDialog.confirmDelete') }}
        </button>
      </div>
    </template>
  </BaseDialog>

</template>

<script setup lang="ts">
import { computed, defineComponent, h, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { isStepUpCancelled, type StepUpController } from '@/composables/useStepUp'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import type { IPRiskPolicy, IPRiskPolicyInput, IPPolicyMode, RiskConfig } from './types'

const NumberField = defineComponent({
  props: {
    modelValue: { type: Number, required: true },
    label: { type: String, required: true },
    min: { type: Number, default: 1 },
    max: { type: Number, default: 100 },
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () => h('label', { class: 'block' }, [
      h('span', { class: 'input-label' }, props.label),
      h('input', {
        type: 'number',
        class: 'input tabular-nums',
        value: props.modelValue,
        min: props.min,
        max: props.max,
        onInput: (event: Event) => emit('update:modelValue', Number((event.target as HTMLInputElement).value)),
      }),
    ])
  },
})

const ConfigPair = defineComponent({
  props: {
    threshold: { type: Number, required: true },
    score: { type: Number, required: true },
    label: { type: String, required: true },
  },
  emits: ['update:threshold', 'update:score'],
  setup(props, { emit }) {
    return () => h('div', { class: 'rounded-lg border border-gray-200 p-3 dark:border-dark-700' }, [
      h('div', { class: 'text-sm font-medium text-gray-900 dark:text-white' }, props.label),
      h('div', { class: 'mt-3 grid grid-cols-2 gap-3' }, [
        h('label', [
          h(
            'span',
            { class: 'mb-1 block text-xs text-gray-500 dark:text-gray-400' },
            t('admin.ipRisk.policyDialog.threshold'),
          ),
          h('input', {
            type: 'number',
            class: 'input tabular-nums',
            min: 1,
            value: props.threshold,
            onInput: (event: Event) => emit('update:threshold', Number((event.target as HTMLInputElement).value)),
          }),
        ]),
        h('label', [
          h(
            'span',
            { class: 'mb-1 block text-xs text-gray-500 dark:text-gray-400' },
            t('admin.ipRisk.policyDialog.score'),
          ),
          h('input', {
            type: 'number',
            class: 'input tabular-nums',
            min: 0,
            max: 100,
            value: props.score,
            onInput: (event: Event) => emit('update:score', Number((event.target as HTMLInputElement).value)),
          }),
        ]),
      ]),
    ])
  },
})

const props = defineProps<{
  show: boolean
  stepUp: StepUpController
}>()
const emit = defineEmits<{ (event: 'close'): void; (event: 'updated'): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const activeTab = ref<'detection' | 'policies'>('detection')
const loading = ref(false)
const savingConfig = ref(false)
const savingPolicy = ref(false)
const deletingPolicy = ref(false)
const config = ref<RiskConfig | null>(null)
const policies = ref<IPRiskPolicy[]>([])
const editingPolicyId = ref<number | null>(null)
const deleteTarget = ref<IPRiskPolicy | null>(null)
const policyForm = reactive<IPRiskPolicyInput>({
  mode: 'allowlist',
  ip_network: '',
  exact_ip: '',
  reason: '',
  enabled: true,
  expires_at: null,
})

const tabs = computed(() => [
  { value: 'detection' as const, label: t('admin.ipRisk.policyDialog.detectionTab') },
  { value: 'policies' as const, label: t('admin.ipRisk.policyDialog.policiesTab') },
])

const policyModeOptions = computed(() => [
  { value: 'allowlist', label: t('admin.ipRisk.policyModes.allowlist') },
  { value: 'observe', label: t('admin.ipRisk.policyModes.observe') },
  { value: 'shared_network', label: t('admin.ipRisk.policyModes.shared_network') },
  { value: 'block_registration', label: t('admin.ipRisk.policyModes.block_registration') },
])

const canSavePolicy = computed(() =>
  Boolean((policyForm.exact_ip || policyForm.ip_network) && policyForm.reason.trim()),
)

watch(
  () => props.show,
  (show) => {
    if (show) load()
  },
)

async function load() {
  loading.value = true
  try {
    const [nextConfig, nextPolicies] = await Promise.all([
      adminAPI.ipRisk.getConfig(),
      adminAPI.ipRisk.listPolicies(),
    ])
    config.value = structuredClone(nextConfig)
    policies.value = nextPolicies
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.ipRisk.policyDialog.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  if (!config.value) return
  savingConfig.value = true
  try {
    config.value = await props.stepUp.run(() => adminAPI.ipRisk.updateConfig(config.value!))
    appStore.showSuccess(t('admin.ipRisk.policyDialog.configSaved'))
    emit('updated')
  } catch (error) {
    if (!isStepUpCancelled(error)) {
      appStore.showError(extractApiErrorMessage(error, t('admin.ipRisk.policyDialog.saveFailed')))
    }
  } finally {
    savingConfig.value = false
  }
}

async function savePolicy() {
  if (!canSavePolicy.value) return
  savingPolicy.value = true
  const payload: IPRiskPolicyInput = {
    ...policyForm,
    expires_at: policyForm.expires_at ? new Date(policyForm.expires_at).toISOString() : null,
  }
  try {
    if (editingPolicyId.value) {
      await props.stepUp.run(() => adminAPI.ipRisk.updatePolicy(editingPolicyId.value!, payload))
    } else {
      await props.stepUp.run(() => adminAPI.ipRisk.createPolicy(payload))
    }
    appStore.showSuccess(t('admin.ipRisk.policyDialog.policySaved'))
    resetPolicyForm()
    policies.value = await adminAPI.ipRisk.listPolicies()
    emit('updated')
  } catch (error) {
    if (!isStepUpCancelled(error)) {
      appStore.showError(extractApiErrorMessage(error, t('admin.ipRisk.policyDialog.policySaveFailed')))
    }
  } finally {
    savingPolicy.value = false
  }
}

function editPolicy(policy: IPRiskPolicy) {
  editingPolicyId.value = policy.id
  policyForm.mode = policy.mode
  policyForm.ip_network = policy.ip_network || ''
  policyForm.exact_ip = policy.exact_ip || ''
  policyForm.reason = policy.reason
  policyForm.enabled = policy.enabled
  policyForm.expires_at = policy.expires_at ? toLocalInput(policy.expires_at) : null
}

function removePolicy(policy: IPRiskPolicy) {
  deleteTarget.value = policy
}

function closeDelete() {
  if (deletingPolicy.value) return
  deleteTarget.value = null
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  deletingPolicy.value = true
  const policyID = deleteTarget.value.id
  try {
    await props.stepUp.run(() => adminAPI.ipRisk.deletePolicy(policyID))
    policies.value = policies.value.filter((item) => item.id !== policyID)
    appStore.showSuccess(t('admin.ipRisk.policyDialog.policyDeleted'))
    deleteTarget.value = null
    emit('updated')
  } catch (error) {
    if (!isStepUpCancelled(error)) {
      appStore.showError(extractApiErrorMessage(error, t('admin.ipRisk.policyDialog.policyDeleteFailed')))
    }
  } finally {
    deletingPolicy.value = false
  }
}

function resetPolicyForm() {
  editingPolicyId.value = null
  policyForm.mode = 'allowlist'
  policyForm.ip_network = ''
  policyForm.exact_ip = ''
  policyForm.reason = ''
  policyForm.enabled = true
  policyForm.expires_at = null
}

function close() {
  if (savingConfig.value || savingPolicy.value || deletingPolicy.value) return
  emit('close')
}

function toLocalInput(value: string) {
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

function policyModeLabel(mode: IPPolicyMode) {
  return t(`admin.ipRisk.policyModes.${mode}`)
}

function policyModeClass(mode: IPPolicyMode) {
  if (mode === 'block_registration') return 'badge-danger'
  if (mode === 'allowlist') return 'badge-success'
  if (mode === 'shared_network') return 'badge-warning'
  return 'badge-gray'
}
</script>
