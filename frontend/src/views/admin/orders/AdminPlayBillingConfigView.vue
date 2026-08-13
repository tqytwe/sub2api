<template>
  <AppLayout>
    <div class="space-y-4">
      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="min-w-0 space-y-2">
            <p class="text-xs font-semibold uppercase tracking-wide text-primary-600 dark:text-primary-300">
              {{ t('payment.admin.playBilling.eyebrow') }}
            </p>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('payment.admin.playBilling.title') }}
            </h2>
            <p class="text-sm leading-6 text-gray-600 dark:text-dark-300">
              {{ t('payment.admin.playBilling.description') }}
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadAll">
              <!-- design-governance-allow: continuous-motion - existing admin refresh spinner is transient, user-triggered, and stops when the request finishes. -->
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin motion-reduce:animate-none' : ''" />
              <span>{{ t('common.refresh') }}</span>
            </button>
            <button class="btn btn-primary" :disabled="saving" data-test="save-play-billing" @click="saveMappings">
              <Icon name="save" size="sm" />
              <span>{{ saving ? t('common.saving') : t('common.save') }}</span>
            </button>
          </div>
        </div>

        <div class="mt-4 grid gap-3 md:grid-cols-3">
          <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
              {{ t('payment.admin.playBilling.packageName') }}
            </p>
            <p class="mt-1 break-all text-sm font-semibold text-gray-900 dark:text-white">
              {{ config?.package_name || '-' }}
            </p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
              {{ t('payment.admin.playBilling.serviceAccount') }}
            </p>
            <p class="mt-1">
              <span
                :class="[
                  'inline-flex rounded-full px-2 py-0.5 text-xs font-semibold',
                  config?.service_account_configured
                    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
                    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
                ]"
              >
                {{ config?.service_account_configured ? t('payment.admin.playBilling.configured') : t('payment.admin.playBilling.notConfigured') }}
              </span>
            </p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
              {{ t('payment.admin.playBilling.enabledProducts') }}
            </p>
            <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
              {{ enabledCount }} / {{ mappings.length }}
            </p>
          </div>
        </div>

        <div class="mt-4 rounded-lg border border-blue-100 bg-blue-50 p-3 text-sm leading-6 text-blue-800 dark:border-blue-900/50 dark:bg-blue-950/30 dark:text-blue-200">
          {{ t('payment.admin.playBilling.consoleBoundary') }}
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('payment.admin.playBilling.products') }}
            </h3>
            <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">
              {{ t('payment.admin.playBilling.productsHint') }}
            </p>
          </div>
          <button class="btn btn-secondary" type="button" data-test="add-play-billing-product" @click="addMapping">
            <Icon name="plus" size="sm" />
            <span>{{ t('payment.admin.playBilling.addProduct') }}</span>
          </button>
        </div>

        <DataTable
          class="mt-4"
          :columns="columns"
          :data="mappings"
          :loading="loading"
          :sticky-first-column="false"
          :sticky-actions-column="false"
          row-key="local_id"
        >
          <template #empty>
            <div class="flex flex-col items-center">
              <Icon name="inbox" size="xl" class="mb-3 h-10 w-10 text-gray-400 dark:text-dark-500" />
              <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                {{ t('payment.admin.playBilling.empty') }}
              </p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ t('payment.admin.playBilling.emptyHint') }}
              </p>
            </div>
          </template>

          <template #cell-enabled="{ row }">
            <Toggle
              v-model="row.enabled"
              :aria-label="t('payment.admin.playBilling.enabled')"
              :disabled="saving"
            />
          </template>

          <template #cell-product_id="{ row }">
            <div class="w-full min-w-0 space-y-1 md:min-w-[220px]">
              <input
                v-model.trim="row.product_id"
                class="input min-w-0"
                :class="fieldError(row, 'product_id') ? 'border-red-500 focus:border-red-500 focus:ring-red-500' : ''"
                :placeholder="t('payment.admin.playBilling.productIdPlaceholder')"
                :disabled="saving"
                data-test="play-billing-product-id"
              />
              <p v-if="fieldError(row, 'product_id')" class="text-xs text-red-500">
                {{ fieldError(row, 'product_id') }}
              </p>
            </div>
          </template>

          <template #cell-product_type="{ row }">
            <Select
              v-model="row.product_type"
              :options="productTypeOptions"
              class="w-32"
              :disabled="saving"
              @change="normalizeProductType(row)"
            />
          </template>

          <template #cell-order_type="{ row }">
            <Select
              v-model="row.order_type"
              :options="orderTypeOptions"
              class="w-36"
              :disabled="saving"
            />
          </template>

          <template #cell-title="{ row }">
            <div class="w-full min-w-0 space-y-1 md:min-w-[200px]">
              <input
                v-model.trim="row.title"
                class="input min-w-0"
                :placeholder="t('payment.admin.playBilling.titlePlaceholder')"
                :disabled="saving"
              />
              <input
                v-model.trim="row.formatted_price"
                class="input min-w-0"
                :placeholder="t('payment.admin.playBilling.pricePlaceholder')"
                :disabled="saving"
              />
            </div>
          </template>

          <template #cell-entitlement="{ row }">
            <div class="w-full min-w-0 space-y-2 md:min-w-[240px]">
              <template v-if="row.order_type === 'balance'">
                <input
                  v-model.number="row.amount"
                  type="number"
                  min="0"
                  step="0.01"
                  class="input min-w-0"
                  :class="fieldError(row, 'amount') ? 'border-red-500 focus:border-red-500 focus:ring-red-500' : ''"
                  :placeholder="t('payment.admin.playBilling.amountPlaceholder')"
                  :disabled="saving"
                  data-test="play-billing-amount"
                />
                <p v-if="fieldError(row, 'amount')" class="text-xs text-red-500">
                  {{ fieldError(row, 'amount') }}
                </p>
              </template>
              <template v-else>
                <Select
                  v-model="row.plan_id"
                  :options="planOptions"
                  class="w-full"
                  searchable
                  :placeholder="t('payment.admin.playBilling.selectPlan')"
                  :disabled="saving || plansLoading"
                />
                <p v-if="fieldError(row, 'plan_id')" class="text-xs text-red-500">
                  {{ fieldError(row, 'plan_id') }}
                </p>
              </template>
            </div>
          </template>

          <template #cell-pay_amount="{ row }">
            <div class="w-full min-w-0 space-y-1 md:min-w-[150px]">
              <input
                v-model.number="row.pay_amount"
                type="number"
                min="0"
                step="0.01"
                class="input min-w-0"
                :placeholder="t('payment.admin.playBilling.payAmountPlaceholder')"
                :disabled="saving"
              />
              <input
                v-model.trim="row.currency"
                class="input min-w-0 uppercase"
                maxlength="3"
                :placeholder="t('payment.admin.playBilling.currencyPlaceholder')"
                :disabled="saving"
              />
            </div>
          </template>

          <template #cell-consumable="{ row }">
            <Toggle
              v-model="row.consumable"
              :aria-label="t('payment.admin.playBilling.consumable')"
              :disabled="saving || row.product_type !== 'inapp'"
            />
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white"
                :aria-label="t('common.copy')"
                @click="duplicateMapping(row)"
              >
                <Icon name="copy" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-lg p-1.5 text-red-500 hover:bg-red-50 hover:text-red-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500/40 dark:hover:bg-red-900/20"
                :aria-label="t('common.delete')"
                @click="removeMapping(row.local_id)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>
        </DataTable>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { PlayBillingProductMapping, PlayBillingProductType, SubscriptionPlan } from '@/types/payment'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'

type EditablePlayBillingMapping = PlayBillingProductMapping & {
  local_id: string
  enabled: boolean
  consumable: boolean
  product_type: PlayBillingProductType
  order_type: 'balance' | 'subscription'
}

type FieldKey = 'product_id' | 'amount' | 'plan_id'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const plansLoading = ref(false)
const plans = ref<SubscriptionPlan[]>([])
const config = ref<Awaited<ReturnType<typeof adminPaymentAPI.getPlayBillingConfig>>['data'] | null>(null)
const mappings = ref<EditablePlayBillingMapping[]>([])
const validationErrors = ref<Record<string, Partial<Record<FieldKey, string>>>>({})

const productTypeOptions = computed(() => [
  { value: 'inapp', label: t('payment.admin.playBilling.productTypeInapp') },
  { value: 'subs', label: t('payment.admin.playBilling.productTypeSubs') },
])

const orderTypeOptions = computed(() => [
  { value: 'balance', label: t('payment.admin.balanceOrder') },
  { value: 'subscription', label: t('payment.admin.subscriptionOrder') },
])

const planOptions = computed(() => [
  { value: 0, label: t('payment.admin.playBilling.selectPlan') },
  ...plans.value.map(plan => ({
    value: plan.id,
    label: planLabel(plan),
  })),
])

const columns = computed((): Column[] => [
  { key: 'enabled', label: t('payment.admin.playBilling.enabled'), class: 'min-w-[72px]' },
  { key: 'product_id', label: t('payment.admin.playBilling.productId'), class: 'min-w-[260px]' },
  { key: 'product_type', label: t('payment.admin.playBilling.productType'), class: 'min-w-[140px]' },
  { key: 'order_type', label: t('payment.admin.playBilling.orderType'), class: 'min-w-[150px]' },
  { key: 'title', label: t('payment.admin.playBilling.display'), class: 'min-w-[260px]' },
  { key: 'entitlement', label: t('payment.admin.playBilling.entitlement'), class: 'min-w-[280px]' },
  { key: 'pay_amount', label: t('payment.admin.playBilling.payAmount'), class: 'min-w-[200px]' },
  { key: 'consumable', label: t('payment.admin.playBilling.consumable'), class: 'min-w-[96px]' },
  { key: 'actions', label: t('common.actions'), class: 'min-w-[96px]' },
])

const enabledCount = computed(() => mappings.value.filter(item => item.enabled).length)

function localId(): string {
  return `play-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

function toEditable(mapping: PlayBillingProductMapping): EditablePlayBillingMapping {
  const productType = mapping.product_type === 'subs' ? 'subs' : 'inapp'
  const orderType = mapping.order_type === 'subscription' ? 'subscription' : 'balance'
  return {
    ...mapping,
    local_id: localId(),
    product_type: productType,
    order_type: orderType,
    enabled: mapping.enabled !== false,
    consumable: mapping.consumable ?? productType === 'inapp',
    currency: (mapping.currency || 'USD').toUpperCase(),
    amount: mapping.amount || 0,
    pay_amount: mapping.pay_amount || 0,
    plan_id: mapping.plan_id || 0,
  }
}

function toPayload(mapping: EditablePlayBillingMapping): PlayBillingProductMapping {
  const payload: PlayBillingProductMapping = {
    product_id: mapping.product_id.trim(),
    product_type: mapping.product_type,
    order_type: mapping.order_type,
    title: mapping.title?.trim() || undefined,
    description: mapping.description?.trim() || undefined,
    formatted_price: mapping.formatted_price?.trim() || undefined,
    pay_amount: Number(mapping.pay_amount) > 0 ? Number(mapping.pay_amount) : undefined,
    currency: (mapping.currency || 'USD').trim().toUpperCase(),
    enabled: mapping.enabled,
  }
  if (mapping.order_type === 'balance') {
    payload.amount = Number(mapping.amount) || 0
    payload.consumable = mapping.consumable
  } else {
    payload.plan_id = Number(mapping.plan_id) || 0
    payload.consumable = mapping.product_type === 'inapp' ? mapping.consumable : false
  }
  return payload
}

function planLabel(plan: SubscriptionPlan): string {
  const name = plan.product_name?.trim() || plan.name
  const price = Number.isFinite(plan.price) ? `${plan.currency || 'USD'} ${Number(plan.price).toFixed(2)}` : ''
  return `${name} #${plan.id}${price ? ` · ${price}` : ''}`
}

async function loadConfig() {
  const res = await adminPaymentAPI.getPlayBillingConfig()
  config.value = res.data
  mappings.value = (res.data.products || []).map(toEditable)
  validationErrors.value = {}
}

async function loadPlans() {
  plansLoading.value = true
  try {
    const res = await adminPaymentAPI.getPlans()
    plans.value = (res.data || []).map((plan: Omit<SubscriptionPlan, 'features'> & { features: string | string[] }) => ({
      ...plan,
      features: typeof plan.features === 'string'
        ? plan.features.split('\n').map(feature => feature.trim()).filter(Boolean)
        : (plan.features || []),
    }))
  } catch {
    plans.value = []
  } finally {
    plansLoading.value = false
  }
}

async function loadAll() {
  loading.value = true
  try {
    await Promise.all([loadConfig(), loadPlans()])
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function addMapping() {
  mappings.value.unshift(toEditable({
    product_id: '',
    product_type: 'inapp',
    order_type: 'balance',
    amount: 0,
    pay_amount: 0,
    currency: 'USD',
    consumable: true,
    enabled: true,
  }))
}

function duplicateMapping(row: EditablePlayBillingMapping) {
  mappings.value.unshift({
    ...row,
    local_id: localId(),
    product_id: `${row.product_id}_copy`,
    enabled: false,
  })
}

function removeMapping(localID: string) {
  mappings.value = mappings.value.filter(item => item.local_id !== localID)
  delete validationErrors.value[localID]
}

function normalizeProductType(row: EditablePlayBillingMapping) {
  if (row.product_type === 'subs') {
    row.consumable = false
  } else if (row.consumable == null) {
    row.consumable = true
  }
}

function fieldError(row: EditablePlayBillingMapping, field: FieldKey): string {
  return validationErrors.value[row.local_id]?.[field] || ''
}

function validateMappings(): boolean {
  const errors: Record<string, Partial<Record<FieldKey, string>>> = {}
  const seen = new Set<string>()
  for (const row of mappings.value) {
    const rowErrors: Partial<Record<FieldKey, string>> = {}
    const productID = row.product_id.trim()
    const key = `${row.product_type}:${productID.toLowerCase()}`
    if (!productID) {
      rowErrors.product_id = t('payment.admin.playBilling.validation.productRequired')
    } else if (seen.has(key)) {
      rowErrors.product_id = t('payment.admin.playBilling.validation.duplicateProduct')
    } else {
      seen.add(key)
    }
    if (row.order_type === 'balance' && !(Number(row.amount) > 0)) {
      rowErrors.amount = t('payment.admin.playBilling.validation.amountRequired')
    }
    if (row.order_type === 'subscription' && !(Number(row.plan_id) > 0)) {
      rowErrors.plan_id = t('payment.admin.playBilling.validation.planRequired')
    }
    if (Object.keys(rowErrors).length > 0) {
      errors[row.local_id] = rowErrors
    }
  }
  validationErrors.value = errors
  return Object.keys(errors).length === 0
}

async function saveMappings() {
  if (!validateMappings()) {
    appStore.showError(t('payment.admin.playBilling.validation.fixBeforeSave'))
    return
  }
  saving.value = true
  try {
    const res = await adminPaymentAPI.updatePlayBillingConfig({
      products: mappings.value.map(toPayload),
    })
    config.value = res.data
    mappings.value = (res.data.products || []).map(toEditable)
    validationErrors.value = {}
    appStore.showSuccess(t('payment.admin.playBilling.saved'))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void loadAll()
})
</script>
