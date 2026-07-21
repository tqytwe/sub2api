<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-3">
          <div class="flex flex-wrap items-center gap-3">
            <input
              v-model="planFilters.keyword"
              type="search"
              class="input min-w-[220px] flex-1"
              :placeholder="t('payment.admin.searchPlans')"
            />
            <Select v-model="planFilters.platform" :options="storefrontPlatformFilterOptions" class="w-40" />
            <Select v-model="planFilters.category" :options="storefrontCategoryFilterOptions" class="w-36" />
            <Select v-model="planFilters.sale" :options="saleFilterOptions" class="w-32" />
            <Select v-model="planFilters.featured" :options="featuredFilterOptions" class="w-32" />
            <button @click="resetPlanFilters" class="btn btn-secondary">{{ t('payment.admin.resetFilters') }}</button>
            <div class="ml-auto flex items-center gap-2">
              <button @click="loadPlans" :disabled="plansLoading" class="btn btn-secondary" :title="t('common.refresh')">
                <Icon name="refresh" size="md" :class="plansLoading ? 'animate-spin' : ''" />
              </button>
              <button @click="openPlanEdit(null)" class="btn btn-primary">{{ t('payment.admin.createPlan') }}</button>
            </div>
          </div>
          <div v-if="selectedPlanIds.length > 0" class="flex flex-wrap items-center gap-2 rounded-lg border border-primary-100 bg-primary-50/70 p-3 dark:border-primary-500/20 dark:bg-primary-500/10">
            <span class="mr-1 text-sm font-medium text-primary-700 dark:text-primary-300">{{ t('payment.admin.selectedPlans', { count: selectedPlanIds.length }) }}</span>
            <button class="btn btn-secondary btn-sm" :disabled="batchUpdating" @click="batchUpdateSelectedPlans({ for_sale: true })">{{ t('payment.admin.batchOnSale') }}</button>
            <button class="btn btn-secondary btn-sm" :disabled="batchUpdating" @click="batchUpdateSelectedPlans({ for_sale: false })">{{ t('payment.admin.batchOffSale') }}</button>
            <button class="btn btn-secondary btn-sm" :disabled="batchUpdating" @click="batchUpdateSelectedPlans({ storefront_featured: true })">{{ t('payment.admin.batchFeatured') }}</button>
            <button class="btn btn-secondary btn-sm" :disabled="batchUpdating" @click="batchUpdateSelectedPlans({ storefront_featured: false })">{{ t('payment.admin.batchUnfeatured') }}</button>
            <div class="flex items-center gap-2">
              <Select v-model="batchPlatform" :options="storefrontPlatformBatchOptions" class="w-36" />
              <button class="btn btn-secondary btn-sm" :disabled="batchUpdating" @click="batchUpdateSelectedPlans({ storefront_platform: batchPlatform })">{{ t('payment.admin.batchSetPlatform') }}</button>
            </div>
            <div class="flex items-center gap-2">
              <Select v-model="batchCategory" :options="storefrontCategoryBatchOptions" class="w-32" />
              <button class="btn btn-secondary btn-sm" :disabled="batchUpdating" @click="batchUpdateSelectedPlans({ storefront_category: batchCategory })">{{ t('payment.admin.batchSetCategory') }}</button>
            </div>
          </div>
          <p class="text-xs text-gray-500 dark:text-dark-400">
            {{ t('payment.admin.filteredPlansSummary', { count: filteredPlans.length, total: plans.length }) }}
          </p>
        </div>
      </div>

      <!-- Plans Table -->
      <DataTable
        :columns="planColumns"
        :data="filteredPlans"
        :loading="plansLoading"
        selectable
        row-key="id"
        :selected-keys="selectedPlanIds"
        :selection-label="t('payment.admin.selectPlan')"
        @update:selected-keys="selectedPlanIds = $event"
      >
        <template #cell-cover_image_url="{ value, row }">
          <div class="h-12 w-20 overflow-hidden rounded-md border border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-800">
            <img
              v-if="value"
              :src="value"
              :alt="row.product_name || row.name"
              class="h-full w-full object-cover"
            />
            <div v-else class="flex h-full w-full items-center justify-center text-[10px] font-medium text-gray-400 dark:text-dark-400">
              {{ t('payment.admin.noCover') }}
            </div>
          </div>
        </template>
        <template #cell-name="{ value, row }">
          <div class="min-w-0">
            <span class="block truncate text-sm font-medium" :class="getPlanNameClass(row.group_id)">{{ row.product_name || value }}</span>
            <span v-if="row.product_name" class="block truncate text-xs text-gray-400 dark:text-dark-400">{{ value }}</span>
          </div>
        </template>
        <template #cell-group_id="{ value }">
          <span v-if="isGroupMissing(value)" class="text-sm">
            <span class="text-gray-400">#{{ value }}</span>
            <span class="ml-1 badge badge-danger">{{ t('payment.admin.groupMissing') }}</span>
          </span>
          <GroupBadge
            v-else-if="getGroup(value)"
            :name="getGroup(value)!.name"
            :platform="getGroup(value)!.platform"
            :rate-multiplier="getGroup(value)!.rate_multiplier"
          />
          <span v-else class="text-sm text-gray-400">-</span>
        </template>
        <template #cell-storefront_platform="{ value, row }">
          <span :class="['rounded-md border px-2 py-0.5 text-xs font-medium', platformBadgeClass(planStorefrontPlatform(row) || value || '')]">
            {{ planStorefrontPlatformLabel(row) }}
          </span>
        </template>
        <template #cell-storefront_category="{ row }">
          <span class="rounded-md border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300">
            {{ planStorefrontCategoryLabel(row) }}
          </span>
        </template>
        <template #cell-storefront_badge="{ row }">
          <div class="flex flex-wrap items-center gap-1">
            <span v-if="row.storefront_featured" class="badge badge-success text-[10px]">{{ t('payment.planCard.featured') }}</span>
            <span v-if="row.storefront_badge" class="rounded bg-gray-900 px-1.5 py-0.5 text-[10px] font-semibold text-white dark:bg-white dark:text-gray-900">{{ row.storefront_badge }}</span>
            <span v-if="!row.storefront_featured && !row.storefront_badge" class="text-xs text-gray-400">-</span>
          </div>
        </template>
        <template #cell-price="{ value, row }">
          <div class="text-sm">
            <span class="font-medium text-gray-900 dark:text-white">{{ planCurrencySymbol(row.currency) }}{{ (value ?? 0).toFixed(2) }}</span>
            <span v-if="row.currency" class="ml-1 text-xs text-gray-400">{{ row.currency }}</span>
            <span v-if="row.original_price" class="ml-1 text-xs text-gray-400 line-through">{{ planCurrencySymbol(row.currency) }}{{ row.original_price.toFixed(2) }}</span>
          </div>
        </template>
        <template #cell-validity_days="{ value, row }">
          <span class="text-sm">{{ value }} {{ t('payment.admin.' + (row.validity_unit || 'days')) }}</span>
        </template>
        <template #cell-for_sale="{ value, row }">
          <button
            type="button"
            :class="[
              'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              value ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'
            ]"
            @click="toggleForSale(row)"
          >
            <span :class="[
              'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              value ? 'translate-x-4' : 'translate-x-0'
            ]" />
          </button>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-2">
            <button @click="openPlanEdit(row)" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400">
              <Icon name="edit" size="sm" />
              <span class="text-xs">{{ t('common.edit') }}</span>
            </button>
            <button @click="confirmDeletePlan(row)" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400">
              <Icon name="trash" size="sm" />
              <span class="text-xs">{{ t('common.delete') }}</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Plan Edit Dialog -->
    <PlanEditDialog :show="showPlanDialog" :plan="editingPlan" :groups="groups" :payment-config="paymentConfig" @close="showPlanDialog = false" @saved="loadPlans" />

    <ConfirmDialog :show="showDeletePlanDialog" :title="t('payment.admin.deletePlan')" :message="t('payment.admin.deletePlanConfirm')" :confirm-text="t('common.delete')" danger @confirm="handleDeletePlan" @cancel="showDeletePlanDialog = false" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { AdminPaymentConfig } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import adminAPI from '@/api/admin'
import type { SubscriptionPlan } from '@/types/payment'
import type { AdminGroup } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlanEditDialog from './PlanEditDialog.vue'
import { currencySymbol } from '@/components/payment/currency'
import { platformBadgeClass, platformLabel, platformTextClass } from '@/utils/platformColors'

const { t } = useI18n()
const appStore = useAppStore()

function planCurrencySymbol(currency?: string): string {
  return currencySymbol(currency || 'USD')
}

// ==================== Groups ====================

const groups = ref<AdminGroup[]>([])
const paymentConfig = ref<AdminPaymentConfig | null>(null)

async function loadGroups() {
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch { /* ignore */ }
}

async function loadPaymentConfig() {
  try {
    const res = await adminPaymentAPI.getConfig()
    paymentConfig.value = res.data
  } catch { /* preview only */ }
}

function getGroup(id: number): AdminGroup | undefined {
  return groups.value.find(g => g.id === id)
}

function isGroupMissing(id: number): boolean {
  return id > 0 && !groups.value.find(g => g.id === id)
}

function getPlanNameClass(groupId: number): string {
  const group = getGroup(groupId)
  return group ? platformTextClass(group.platform) : 'text-gray-900 dark:text-white'
}


// ==================== Plans ====================

const plansLoading = ref(false)
const plans = ref<SubscriptionPlan[]>([])
const showPlanDialog = ref(false)
const showDeletePlanDialog = ref(false)
const editingPlan = ref<SubscriptionPlan | null>(null)
const deletingPlanId = ref<number | null>(null)
const selectedPlanIds = ref<Array<string | number>>([])
const batchUpdating = ref(false)
const batchPlatform = ref('openai')
const batchCategory = ref('pro')

const planFilters = reactive({
  keyword: '',
  platform: 'all',
  category: 'all',
  sale: 'all',
  featured: 'all',
})

type StorefrontCategory = 'daily' | 'credit' | 'pro' | 'team' | 'enterprise' | 'image'
const storefrontCategories: StorefrontCategory[] = ['daily', 'credit', 'pro', 'team', 'enterprise', 'image']

const storefrontPlatformBatchOptions = computed(() => [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: t('payment.planShelf.platforms.anthropic') },
  { value: 'gemini', label: 'Gemini' },
  { value: 'grok', label: 'Grok' },
  { value: 'image', label: t('payment.planShelf.platforms.image') },
  { value: 'team', label: t('payment.planShelf.platforms.team') },
])

const storefrontPlatformFilterOptions = computed(() => [
  { value: 'all', label: t('payment.planShelf.platforms.all') },
  ...storefrontPlatformBatchOptions.value,
])

const storefrontCategoryBatchOptions = computed(() => storefrontCategories.map(value => ({
  value,
  label: storefrontCategoryLabel(value),
})))

const storefrontCategoryFilterOptions = computed(() => [
  { value: 'all', label: t('payment.planShelf.categories.all') },
  ...storefrontCategoryBatchOptions.value,
])

const saleFilterOptions = computed(() => [
  { value: 'all', label: t('payment.admin.allSaleStatuses') },
  { value: 'on', label: t('payment.admin.onSale') },
  { value: 'off', label: t('payment.admin.offSale') },
])

const featuredFilterOptions = computed(() => [
  { value: 'all', label: t('payment.admin.allFeaturedStatuses') },
  { value: 'featured', label: t('payment.planCard.featured') },
  { value: 'normal', label: t('payment.admin.notFeatured') },
])

const planColumns = computed((): Column[] => [
  { key: 'id', label: 'ID' },
  { key: 'cover_image_url', label: t('payment.admin.coverImage') },
  { key: 'name', label: t('payment.admin.planName') },
  { key: 'group_id', label: t('payment.admin.group') },
  { key: 'storefront_platform', label: t('payment.admin.storefrontPlatform') },
  { key: 'storefront_category', label: t('payment.admin.storefrontCategory') },
  { key: 'storefront_badge', label: t('payment.admin.storefrontBadge') },
  { key: 'price', label: t('payment.admin.price') },
  { key: 'validity_days', label: t('payment.admin.validity') },
  { key: 'for_sale', label: t('payment.admin.forSale') },
  { key: 'sort_order', label: t('payment.admin.sortOrder') },
  { key: 'actions', label: t('common.actions') },
])

async function loadPlans() {
  plansLoading.value = true
  try {
    const res = await adminPaymentAPI.getPlans()
    // Backend returns features as newline-separated string; parse to array
    plans.value = (res.data || []).map((p: Omit<SubscriptionPlan, 'features'> & { features: string | string[] }) => ({
      ...p,
      features: typeof p.features === 'string'
        ? p.features.split('\n').map((f: string) => f.trim()).filter(Boolean)
        : (p.features || []),
    }))
    selectedPlanIds.value = selectedPlanIds.value.filter(id => plans.value.some(plan => plan.id === Number(id)))
  }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { plansLoading.value = false }
}

function openPlanEdit(plan: SubscriptionPlan | null) {
  editingPlan.value = plan
  showPlanDialog.value = true
}


/** Quick toggle for_sale from the list */
async function toggleForSale(plan: SubscriptionPlan) {
  try {
    await adminPaymentAPI.updatePlan(plan.id, { for_sale: !plan.for_sale })
    plan.for_sale = !plan.for_sale
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

function inferStorefrontCategory(plan: SubscriptionPlan): StorefrontCategory {
  const name = `${plan.product_name || ''} ${plan.name || ''}`.toLowerCase()
  if (plan.validity_days === 1 || name.includes('日卡') || name.includes('daily')) return 'daily'
  if (name.includes('团队') || name.includes('team')) return 'team'
  if (name.includes('企业') || name.includes('enterprise')) return 'enterprise'
  if (name.includes('额度') || name.includes('credit')) return 'credit'
  if (name.includes('图片') || name.includes('image')) return 'image'
  return 'pro'
}

function planStorefrontPlatform(plan: SubscriptionPlan): string {
  return plan.storefront_platform?.trim() || plan.group_platform || getGroup(plan.group_id)?.platform || ''
}

function planStorefrontCategory(plan: SubscriptionPlan): StorefrontCategory {
  const category = plan.storefront_category?.trim().toLowerCase()
  return storefrontCategories.includes(category as StorefrontCategory)
    ? category as StorefrontCategory
    : inferStorefrontCategory(plan)
}

function storefrontCategoryLabel(category: string): string {
  switch (category) {
    case 'daily': return t('payment.planShelf.categories.daily')
    case 'credit': return t('payment.planShelf.categories.credit')
    case 'team': return t('payment.planShelf.categories.team')
    case 'enterprise': return t('payment.planShelf.categories.enterprise')
    case 'image': return t('payment.planShelf.categories.image')
    default: return t('payment.planShelf.categories.pro')
  }
}

function planStorefrontPlatformLabel(plan: SubscriptionPlan): string {
  const platform = planStorefrontPlatform(plan)
  if (platform === 'anthropic') return t('payment.planShelf.platforms.anthropic')
  if (platform === 'image') return t('payment.planShelf.platforms.image')
  if (platform === 'team') return t('payment.planShelf.platforms.team')
  return platformLabel(platform)
}

function planStorefrontCategoryLabel(plan: SubscriptionPlan): string {
  return storefrontCategoryLabel(planStorefrontCategory(plan))
}

function planMatchesPlatformFilter(plan: SubscriptionPlan): boolean {
  if (planFilters.platform === 'all') return true
  const platform = planStorefrontPlatform(plan)
  const category = planStorefrontCategory(plan)
  if (planFilters.platform === 'team') return platform === 'team' || category === 'team' || category === 'enterprise'
  if (planFilters.platform === 'image') return platform === 'image' || category === 'image'
  return platform === planFilters.platform
}

const filteredPlans = computed(() => {
  const keyword = planFilters.keyword.trim().toLowerCase()
  return plans.value.filter(plan => {
    if (keyword) {
      const haystack = `${plan.name} ${plan.product_name || ''} ${plan.description || ''}`.toLowerCase()
      if (!haystack.includes(keyword)) return false
    }
    if (!planMatchesPlatformFilter(plan)) return false
    if (planFilters.category !== 'all' && planStorefrontCategory(plan) !== planFilters.category) return false
    if (planFilters.sale === 'on' && !plan.for_sale) return false
    if (planFilters.sale === 'off' && plan.for_sale) return false
    if (planFilters.featured === 'featured' && !plan.storefront_featured) return false
    if (planFilters.featured === 'normal' && plan.storefront_featured) return false
    return true
  })
})

function resetPlanFilters() {
  planFilters.keyword = ''
  planFilters.platform = 'all'
  planFilters.category = 'all'
  planFilters.sale = 'all'
  planFilters.featured = 'all'
}

async function batchUpdateSelectedPlans(payload: Record<string, unknown>) {
  const ids = selectedPlanIds.value.map(id => Number(id)).filter(id => Number.isFinite(id))
  if (ids.length === 0 || batchUpdating.value) return
  batchUpdating.value = true
  try {
    await Promise.all(ids.map(id => adminPaymentAPI.updatePlan(id, payload)))
    appStore.showSuccess(t('payment.admin.batchUpdateSuccess', { count: ids.length }))
    selectedPlanIds.value = []
    await loadPlans()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    batchUpdating.value = false
  }
}

function confirmDeletePlan(plan: SubscriptionPlan) { deletingPlanId.value = plan.id; showDeletePlanDialog.value = true }
async function handleDeletePlan() {
  if (!deletingPlanId.value) return
  try { await adminPaymentAPI.deletePlan(deletingPlanId.value); appStore.showSuccess(t('common.deleted')); showDeletePlanDialog.value = false; loadPlans() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

// ==================== Lifecycle ====================

onMounted(() => {
  loadGroups()
  loadPaymentConfig()
  loadPlans()
})
</script>
