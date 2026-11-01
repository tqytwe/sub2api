<template>
  <section class="card" :aria-busy="loading || saving" aria-labelledby="model-billing-title">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 id="model-billing-title" class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.modelBilling.title') }}</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.modelBilling.description') }}</p>
    </div>
    <div class="space-y-4 p-6">
      <p v-if="loading" role="status" class="text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</p>
      <div v-if="error" role="alert" class="space-y-2 text-sm text-red-600 dark:text-red-400">
        <p>{{ error }}</p>
        <button v-if="!loaded || conflict" data-testid="reload-config" type="button" class="btn btn-secondary" :disabled="loading" @click="load">
          {{ t(conflict ? 'admin.settings.modelBilling.reloadDiscard' : 'common.retry') }}
        </button>
      </div>
      <form v-if="loaded" novalidate @submit.prevent="save">
        <fieldset :disabled="loading || saving" class="min-w-0 space-y-5">
          <Toggle v-model="enabled" :disabled="loading || saving" :label="t('admin.settings.modelBilling.enabled')" :description="t('admin.settings.modelBilling.enabledHint')" />
          <p class="text-sm text-gray-600 dark:text-gray-400">{{ t('admin.settings.modelBilling.matchingHint') }}</p>
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.settings.modelBilling.rules') }} ({{ rules.length }}/100)</h3>
            <button data-testid="add-rule" type="button" class="btn btn-secondary" :disabled="rules.length >= 100" @click="addRule">{{ t('admin.settings.modelBilling.addRule') }}</button>
          </div>
          <p v-if="!rules.length" class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.modelBilling.empty') }}</p>
          <p v-if="submitted && enabled && !rules.length" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ t('admin.settings.modelBilling.rulesRequired') }}</p>
          <div v-for="(rule, index) in rules" :key="rule.key" class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
            <div class="flex items-center justify-between gap-3">
              <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.settings.modelBilling.rule') }} {{ index + 1 }}</h4>
              <button type="button" class="btn btn-secondary btn-sm" :aria-label="`${t('admin.settings.modelBilling.removeRule')} ${index + 1}`" @click="rules.splice(index, 1)">{{ t('common.delete') }}</button>
            </div>
            <div class="grid gap-4 md:grid-cols-3">
              <div class="min-w-0">
                <label :for="`model-billing-model-${rule.key}`" class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.settings.modelBilling.model') }}</label>
                <input :id="`model-billing-model-${rule.key}`" v-model="rule.model" data-testid="rule-model" type="text" maxlength="200" class="input w-full" placeholder="gpt-6*" :aria-invalid="submitted && !!ruleErrors[index]?.model" :aria-describedby="`model-billing-model-help-${rule.key}`" />
                <p :id="`model-billing-model-help-${rule.key}`" class="mt-1 text-xs" :class="submitted && ruleErrors[index]?.model ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'">{{ t(submitted && ruleErrors[index]?.model ? 'admin.settings.modelBilling.invalidModel' : 'admin.settings.modelBilling.modelHint') }}</p>
              </div>
              <div class="min-w-0">
                <label :for="`model-billing-rate-${rule.key}`" class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.settings.modelBilling.multiplier') }}</label>
                <input :id="`model-billing-rate-${rule.key}`" v-model.number="rule.multiplier" data-testid="rule-multiplier" type="number" min="1" max="1000" step="any" class="input w-full" :aria-invalid="submitted && !!ruleErrors[index]?.multiplier" :aria-describedby="`model-billing-rate-help-${rule.key}`" />
                <p :id="`model-billing-rate-help-${rule.key}`" class="mt-1 text-xs" :class="submitted && ruleErrors[index]?.multiplier ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'">{{ t('admin.settings.modelBilling.multiplierHint') }}</p>
              </div>
              <div class="min-w-0">
                <label :for="`model-billing-scope-${rule.key}`" class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.settings.modelBilling.scope') }}</label>
                <select :id="`model-billing-scope-${rule.key}`" v-model="rule.scope" data-testid="rule-scope" class="input w-full" :aria-invalid="submitted && !!ruleErrors[index]?.groups" :aria-describedby="`model-billing-scope-help-${rule.key}`">
                  <option value="all">{{ t('admin.settings.modelBilling.allGroups') }}</option>
                  <option value="selected">{{ t('admin.settings.modelBilling.selectedGroups') }}</option>
                </select>
                <p :id="`model-billing-scope-help-${rule.key}`" class="mt-1 text-xs" :class="submitted && ruleErrors[index]?.groups ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'">{{ t(submitted && ruleErrors[index]?.groups ? 'admin.settings.modelBilling.groupsRequired' : 'admin.settings.modelBilling.scopeHint') }}</p>
              </div>
            </div>
            <fieldset v-if="rule.scope === 'selected'" class="space-y-2">
              <legend class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.settings.modelBilling.selectedGroups') }}</legend>
              <div class="grid max-h-48 gap-2 overflow-y-auto sm:grid-cols-2 lg:grid-cols-3">
                <label v-for="group in groups" :key="group.id" class="flex min-w-0 items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
                  <input v-model="rule.groupIds" type="checkbox" :value="group.id" class="mt-1" />
                  <span class="min-w-0 break-words">{{ group.name }} <span class="text-gray-500">#{{ group.id }}</span></span>
                </label>
                <label v-for="id in missingGroups(rule)" :key="id" class="flex items-start gap-2 text-sm text-red-600 dark:text-red-400">
                  <input v-model="rule.groupIds" type="checkbox" :value="id" class="mt-1" />
                  <span>#{{ id }} · {{ t('admin.settings.modelBilling.unavailableGroup') }}</span>
                </label>
              </div>
              <p v-if="!groups.length" class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.modelBilling.noGroups') }}</p>
            </fieldset>
          </div>
          <div class="flex flex-wrap items-center justify-end gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
            <p v-if="saved" role="status" class="text-sm text-green-600 dark:text-green-400">{{ t('admin.settings.modelBilling.saved') }}</p>
            <button data-testid="save-config" type="submit" class="btn btn-primary" :disabled="conflict">{{ t(saving ? 'common.saving' : 'common.save') }}</button>
          </div>
        </fieldset>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { getModelBillingConfig, saveModelBillingConfig, type ModelBillingSnapshot } from '@/api/admin/modelBilling'
import { getAll } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

interface EditableRule {
  key: number
  model: string
  multiplier: number | string
  scope: 'all' | 'selected'
  groupIds: number[]
}

const { t } = useI18n()
const loading = ref(true)
const loaded = ref(false)
const saving = ref(false)
const saved = ref(false)
const submitted = ref(false)
const conflict = ref(false)
const error = ref('')
const enabled = ref(false)
const revision = ref('')
const rules = ref<EditableRule[]>([])
const groups = ref<AdminGroup[]>([])
let nextKey = 0

const activeIds = computed(() => new Set(groups.value.map(group => group.id)))
const ruleErrors = computed(() => {
  const names = rules.value.map(rule => rule.model.trim().toLowerCase())
  return rules.value.map((rule, index) => ({
    model: !/^[a-zA-Z0-9_./:-]+\*?$/.test(names[index]!) || names[index]!.length > 200 || names.indexOf(names[index]!) !== index,
    multiplier: typeof rule.multiplier !== 'number' || !Number.isFinite(rule.multiplier) || rule.multiplier < 1 || rule.multiplier > 1000,
    groups: rule.scope === 'selected' && (!rule.groupIds.length || (enabled.value && rule.groupIds.some(id => !activeIds.value.has(id)))),
  }))
})

watch([enabled, rules], () => { saved.value = false }, { deep: true })

function missingGroups(rule: EditableRule) {
  return rule.groupIds.filter(id => !activeIds.value.has(id))
}

function applySnapshot(snapshot: ModelBillingSnapshot) {
  enabled.value = snapshot.config.enabled
  rules.value = (snapshot.config.rules ?? []).map(rule => ({ key: nextKey++, model: rule.model, multiplier: rule.multiplier, scope: rule.group_ids?.length ? 'selected' : 'all', groupIds: [...(rule.group_ids ?? [])] }))
  revision.value = snapshot.revision
  submitted.value = false
  conflict.value = false
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [snapshot, activeGroups] = await Promise.all([getModelBillingConfig(), getAll()])
    groups.value = activeGroups
    applySnapshot(snapshot)
    loaded.value = true
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.settings.modelBilling.loadFailed'))
  } finally {
    loading.value = false
  }
}

function addRule() {
  if (rules.value.length < 100) rules.value.push({ key: nextKey++, model: '', multiplier: 1, scope: 'all', groupIds: [] })
}

async function save() {
  if (!loaded.value || loading.value || saving.value || conflict.value) return
  submitted.value = true
  saved.value = false
  error.value = ''
  if ((enabled.value && !rules.value.length) || rules.value.length > 100 || ruleErrors.value.some(rule => rule.model || rule.multiplier || rule.groups)) return
  saving.value = true
  try {
    const snapshot = await saveModelBillingConfig({
      config: { enabled: enabled.value, rules: rules.value.map(rule => ({ model: rule.model.trim(), multiplier: Number(rule.multiplier), group_ids: rule.scope === 'all' ? [] : [...rule.groupIds] })) },
      revision: revision.value,
    })
    applySnapshot(snapshot)
    await nextTick()
    saved.value = true
  } catch (err) {
    const status = err as { status?: number; response?: { status?: number } }
    conflict.value = status.status === 409 || status.response?.status === 409
    error.value = conflict.value ? t('admin.settings.modelBilling.conflict') : extractApiErrorMessage(err, t('admin.settings.modelBilling.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
