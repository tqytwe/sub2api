<template>
  <section data-test="storefront-config-panel" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.storefrontConfigTitle') }}</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('payment.admin.storefrontConfigHint') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadConfig">
          <Icon name="refresh" size="sm" />
          {{ t('common.refresh') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving || loading" @click="saveConfig">
          <Icon name="save" size="sm" />
          {{ saving ? t('common.processing') : t('common.save') }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="rounded-lg border border-gray-200 px-4 py-3 text-sm font-medium text-gray-500 dark:border-dark-700 dark:text-dark-300">
        {{ t('common.loading') }}
      </div>
    </div>

    <div v-else class="space-y-5">
      <div class="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
        <div class="rounded-lg border border-gray-200 dark:border-dark-700">
          <div class="flex items-center justify-between gap-2 border-b border-gray-200 px-3 py-2 dark:border-dark-700">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.storefrontShelves') }}</h3>
            <button type="button" class="btn btn-secondary btn-sm" @click="addShelf">
              <Icon name="plus" size="sm" />
              {{ t('payment.admin.addShelf') }}
            </button>
          </div>
          <div class="space-y-2 p-3">
            <div
              v-for="(shelf, index) in config.shelves"
              :key="shelf.id"
              class="w-full rounded-lg border p-3 text-left transition-colors"
              :class="selectedShelfId === shelf.id ? 'border-primary-300 bg-primary-50/70 dark:border-primary-500/50 dark:bg-primary-500/10' : 'border-gray-200 bg-white hover:border-primary-200 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-primary-500/40'"
              @click="selectedShelfId = shelf.id"
            >
              <div class="flex items-start gap-2">
                <input v-model="shelf.label" type="text" class="input h-9 min-w-0 flex-1" :placeholder="t('payment.admin.shelfLabelPlaceholder')" @click.stop />
                <div class="flex shrink-0 items-center gap-1">
                  <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700" :aria-label="t('payment.admin.moveUp')" :disabled="index === 0" @click.stop="moveShelf(index, -1)">
                    <Icon name="arrowUp" size="sm" />
                  </button>
                  <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700" :aria-label="t('payment.admin.moveDown')" :disabled="index === config.shelves.length - 1" @click.stop="moveShelf(index, 1)">
                    <Icon name="arrowDown" size="sm" />
                  </button>
                  <button type="button" class="rounded-md p-1.5 text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10" :aria-label="t('common.delete')" @click.stop="removeShelf(shelf.id)">
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </div>
              <div class="mt-2 flex items-center justify-between gap-3 text-xs text-gray-500 dark:text-dark-400">
                <label class="inline-flex items-center gap-2">
                  <input v-model="shelf.enabled" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" @click.stop />
                  {{ t('payment.admin.showOnStorefront') }}
                </label>
                <span>{{ t('payment.admin.assignedPlanCount', { count: shelf.plan_ids.length }) }}</span>
              </div>
            </div>
            <p v-if="config.shelves.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('payment.admin.noShelves') }}</p>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ selectedShelf?.label || t('payment.admin.selectShelfFirst') }}</h3>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('payment.admin.shelfPlanAssignHint') }}</p>
            </div>
            <label v-if="selectedShelf" class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
              {{ t('payment.admin.defaultPlan') }}
              <select v-model.number="selectedShelfDefaultPlanId" class="input h-9 w-48">
                <option :value="0">{{ t('payment.admin.noDefaultPlan') }}</option>
                <option v-for="plan in selectedShelfPlans" :key="plan.id" :value="plan.id">{{ planLabel(plan) }}</option>
              </select>
            </label>
          </div>
          <div v-if="selectedShelf" class="grid gap-2 md:grid-cols-2">
            <label
              v-for="plan in plans"
              :key="plan.id"
              class="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3 text-sm hover:border-primary-200 dark:border-dark-700 dark:hover:border-primary-500/40"
              :class="selectedShelf.plan_ids.includes(plan.id) ? 'bg-primary-50/60 dark:bg-primary-500/10' : 'bg-white dark:bg-dark-900'"
            >
              <input
                type="checkbox"
                class="mt-1 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="selectedShelf.plan_ids.includes(plan.id)"
                @change="toggleShelfPlan(plan.id)"
              />
              <span class="min-w-0 flex-1">
                <span class="block truncate font-medium text-gray-900 dark:text-white">{{ planLabel(plan) }}</span>
                <span class="block text-xs text-gray-500 dark:text-dark-400">{{ planCurrencySymbol(plan.currency) }}{{ plan.price }} · {{ plan.validity_days }} {{ t('payment.admin.' + (plan.validity_unit || 'days')) }}</span>
              </span>
            </label>
          </div>
          <p v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('payment.admin.selectShelfFirst') }}</p>
        </div>
      </div>

      <div class="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
        <div class="rounded-lg border border-gray-200 dark:border-dark-700">
          <div class="flex items-center justify-between gap-2 border-b border-gray-200 px-3 py-2 dark:border-dark-700">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.storefrontTags') }}</h3>
            <button type="button" class="btn btn-secondary btn-sm" @click="addTag">
              <Icon name="plus" size="sm" />
              {{ t('payment.admin.addTag') }}
            </button>
          </div>
          <div class="space-y-2 p-3">
            <div
              v-for="(tag, index) in config.tags"
              :key="tag.id"
              class="w-full rounded-lg border p-3 text-left transition-colors"
              :class="selectedTagId === tag.id ? 'border-primary-300 bg-primary-50/70 dark:border-primary-500/50 dark:bg-primary-500/10' : 'border-gray-200 bg-white hover:border-primary-200 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-primary-500/40'"
              @click="selectedTagId = tag.id"
            >
              <div class="flex items-start gap-2">
                <input v-model="tag.label" type="text" class="input h-9 min-w-0 flex-1" :placeholder="t('payment.admin.tagLabelPlaceholder')" @click.stop />
                <select v-model="tag.tone" class="input h-9 w-24" @click.stop>
                  <option value="primary">{{ t('payment.admin.tagTonePrimary') }}</option>
                  <option value="success">{{ t('payment.admin.tagToneSuccess') }}</option>
                  <option value="warning">{{ t('payment.admin.tagToneWarning') }}</option>
                  <option value="danger">{{ t('payment.admin.tagToneDanger') }}</option>
                  <option value="info">{{ t('payment.admin.tagToneInfo') }}</option>
                  <option value="neutral">{{ t('payment.admin.tagToneNeutral') }}</option>
                </select>
                <div class="flex shrink-0 items-center gap-1">
                  <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700" :aria-label="t('payment.admin.moveUp')" :disabled="index === 0" @click.stop="moveTag(index, -1)">
                    <Icon name="arrowUp" size="sm" />
                  </button>
                  <button type="button" class="rounded-md p-1.5 text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10" :aria-label="t('common.delete')" @click.stop="removeTag(tag.id)">
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </div>
              <div class="mt-2 flex items-center justify-between gap-3 text-xs text-gray-500 dark:text-dark-400">
                <label class="inline-flex items-center gap-2">
                  <input v-model="tag.enabled" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" @click.stop />
                  {{ t('payment.admin.showTag') }}
                </label>
                <span>{{ t('payment.admin.assignedPlanCount', { count: tag.plan_ids.length }) }}</span>
              </div>
            </div>
            <p v-if="config.tags.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('payment.admin.noTags') }}</p>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="mb-3">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ selectedTag?.label || t('payment.admin.selectTagFirst') }}</h3>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('payment.admin.tagPlanAssignHint') }}</p>
          </div>
          <div v-if="selectedTag" class="grid gap-2 md:grid-cols-2">
            <label
              v-for="plan in plans"
              :key="plan.id"
              class="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3 text-sm hover:border-primary-200 dark:border-dark-700 dark:hover:border-primary-500/40"
              :class="selectedTag.plan_ids.includes(plan.id) ? 'bg-primary-50/60 dark:bg-primary-500/10' : 'bg-white dark:bg-dark-900'"
            >
              <input
                type="checkbox"
                class="mt-1 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="selectedTag.plan_ids.includes(plan.id)"
                @change="toggleTagPlan(plan.id)"
              />
              <span class="min-w-0 flex-1">
                <span class="block truncate font-medium text-gray-900 dark:text-white">{{ planLabel(plan) }}</span>
                <span class="block text-xs text-gray-500 dark:text-dark-400">{{ planCurrencySymbol(plan.currency) }}{{ plan.price }}</span>
              </span>
            </label>
          </div>
          <p v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('payment.admin.selectTagFirst') }}</p>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { PaymentStorefrontConfig, PaymentStorefrontShelf, PaymentStorefrontTag, SubscriptionPlan } from '@/types/payment'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import Icon from '@/components/icons/Icon.vue'
import { currencySymbol } from '@/components/payment/currency'

const props = defineProps<{ plans: SubscriptionPlan[] }>()
const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const selectedShelfId = ref('')
const selectedTagId = ref('')
const config = reactive<PaymentStorefrontConfig>({
  shelves: [],
  tags: [],
})

const selectedShelf = computed(() => config.shelves.find(shelf => shelf.id === selectedShelfId.value) || null)
const selectedTag = computed(() => config.tags.find(tag => tag.id === selectedTagId.value) || null)
const selectedShelfPlans = computed(() => {
  const shelf = selectedShelf.value
  if (!shelf) return []
  const ids = new Set(shelf.plan_ids)
  return props.plans.filter(plan => ids.has(plan.id))
})
const selectedShelfDefaultPlanId = computed({
  get: () => selectedShelf.value?.default_plan_id || 0,
  set: (value: number) => {
    if (!selectedShelf.value) return
    selectedShelf.value.default_plan_id = value > 0 ? value : null
  },
})

function planCurrencySymbol(currency?: string): string {
  return currencySymbol(currency || 'USD')
}

function planLabel(plan: SubscriptionPlan): string {
  return `${plan.product_name?.trim() || plan.name} #${plan.id}`
}

function replaceConfig(next: PaymentStorefrontConfig) {
  config.shelves.splice(0, config.shelves.length, ...(next.shelves || []).map(normalizeShelfForEdit))
  config.tags.splice(0, config.tags.length, ...(next.tags || []).map(normalizeTagForEdit))
  selectedShelfId.value = config.shelves[0]?.id || ''
  selectedTagId.value = config.tags[0]?.id || ''
}

function normalizeShelfForEdit(shelf: PaymentStorefrontShelf): PaymentStorefrontShelf {
  return {
    id: shelf.id || createLocalId('shelf'),
    label: shelf.label || '',
    enabled: shelf.enabled !== false,
    sort_order: shelf.sort_order || 0,
    plan_ids: normalizePlanIds(shelf.plan_ids),
    default_plan_id: shelf.default_plan_id || null,
  }
}

function normalizeTagForEdit(tag: PaymentStorefrontTag): PaymentStorefrontTag {
  return {
    id: tag.id || createLocalId('tag'),
    label: tag.label || '',
    tone: tag.tone || 'neutral',
    enabled: tag.enabled !== false,
    sort_order: tag.sort_order || 0,
    plan_ids: normalizePlanIds(tag.plan_ids),
  }
}

function normalizePlanIds(ids: number[] = []): number[] {
  if (props.plans.length === 0) {
    return Array.from(new Set(ids.map(Number).filter(id => Number.isFinite(id) && id > 0)))
  }
  const validIds = new Set(props.plans.map(plan => plan.id))
  return Array.from(new Set(ids.map(Number).filter(id => validIds.has(id))))
}

function createLocalId(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`
}

async function loadConfig() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getStorefrontConfig()
    replaceConfig(res.data)
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function addShelf() {
  const shelf: PaymentStorefrontShelf = {
    id: createLocalId('shelf'),
    label: t('payment.admin.newShelfLabel'),
    enabled: true,
    sort_order: config.shelves.length + 1,
    plan_ids: [],
    default_plan_id: null,
  }
  config.shelves.push(shelf)
  selectedShelfId.value = shelf.id
}

function addTag() {
  const tag: PaymentStorefrontTag = {
    id: createLocalId('tag'),
    label: t('payment.admin.newTagLabel'),
    tone: 'neutral',
    enabled: true,
    sort_order: config.tags.length + 1,
    plan_ids: [],
  }
  config.tags.push(tag)
  selectedTagId.value = tag.id
}

function removeShelf(id: string) {
  const index = config.shelves.findIndex(shelf => shelf.id === id)
  if (index === -1) return
  config.shelves.splice(index, 1)
  selectedShelfId.value = config.shelves[Math.min(index, config.shelves.length - 1)]?.id || ''
}

function removeTag(id: string) {
  const index = config.tags.findIndex(tag => tag.id === id)
  if (index === -1) return
  config.tags.splice(index, 1)
  selectedTagId.value = config.tags[Math.min(index, config.tags.length - 1)]?.id || ''
}

function moveShelf(index: number, direction: -1 | 1) {
  const next = index + direction
  if (next < 0 || next >= config.shelves.length) return
  const [item] = config.shelves.splice(index, 1)
  config.shelves.splice(next, 0, item)
}

function moveTag(index: number, direction: -1 | 1) {
  const next = index + direction
  if (next < 0 || next >= config.tags.length) return
  const [item] = config.tags.splice(index, 1)
  config.tags.splice(next, 0, item)
}

function toggleShelfPlan(planId: number) {
  const shelf = selectedShelf.value
  if (!shelf) return
  togglePlanId(shelf.plan_ids, planId)
  if (shelf.default_plan_id && !shelf.plan_ids.includes(shelf.default_plan_id)) {
    shelf.default_plan_id = null
  }
}

function toggleTagPlan(planId: number) {
  const tag = selectedTag.value
  if (!tag) return
  togglePlanId(tag.plan_ids, planId)
}

function togglePlanId(planIds: number[], planId: number) {
  const index = planIds.indexOf(planId)
  if (index === -1) {
    planIds.push(planId)
    return
  }
  planIds.splice(index, 1)
}

function payload(): PaymentStorefrontConfig {
  return {
    shelves: config.shelves.map((shelf, index) => ({
      ...shelf,
      label: shelf.label.trim(),
      sort_order: index + 1,
      plan_ids: normalizePlanIds(shelf.plan_ids),
      default_plan_id: shelf.default_plan_id && shelf.plan_ids.includes(shelf.default_plan_id) ? shelf.default_plan_id : null,
    })),
    tags: config.tags.map((tag, index) => ({
      ...tag,
      label: tag.label.trim(),
      tone: tag.tone || 'neutral',
      sort_order: index + 1,
      plan_ids: normalizePlanIds(tag.plan_ids),
    })),
  }
}

async function saveConfig() {
  saving.value = true
  try {
    const res = await adminPaymentAPI.updateStorefrontConfig(payload())
    replaceConfig(res.data)
    appStore.showSuccess(t('payment.admin.storefrontConfigSaved'))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    saving.value = false
  }
}

watch(() => props.plans.map(plan => plan.id).join(','), () => {
  config.shelves.forEach((shelf) => {
    shelf.plan_ids = normalizePlanIds(shelf.plan_ids)
    if (shelf.default_plan_id && !shelf.plan_ids.includes(shelf.default_plan_id)) {
      shelf.default_plan_id = null
    }
  })
  config.tags.forEach((tag) => {
    tag.plan_ids = normalizePlanIds(tag.plan_ids)
  })
})

onMounted(loadConfig)
</script>
