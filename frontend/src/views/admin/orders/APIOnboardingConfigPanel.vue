<template>
  <section data-test="api-onboarding-config-panel" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.apiOnboarding.title') }}</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('payment.admin.apiOnboarding.hint') }}</p>
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

    <div v-else class="space-y-4">
      <div class="grid gap-3 lg:grid-cols-[160px_minmax(0,1fr)_minmax(0,1.4fr)]">
        <label class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-600 dark:border-dark-700 dark:text-dark-300">
          <input v-model="config.enabled" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          {{ t('payment.admin.apiOnboarding.enabled') }}
        </label>
        <div>
          <label class="input-label">{{ t('payment.admin.apiOnboarding.panelTitle') }}</label>
          <input v-model="config.title" type="text" class="input" :placeholder="t('payment.admin.apiOnboarding.panelTitlePlaceholder')" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.admin.apiOnboarding.panelSubtitle') }}</label>
          <input v-model="config.subtitle" type="text" class="input" :placeholder="t('payment.admin.apiOnboarding.panelSubtitlePlaceholder')" />
        </div>
      </div>

      <div class="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
        <div class="rounded-lg border border-gray-200 dark:border-dark-700">
          <div class="flex items-center justify-between gap-2 border-b border-gray-200 px-3 py-2 dark:border-dark-700">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.apiOnboarding.cards') }}</h3>
            <button type="button" class="btn btn-secondary btn-sm" @click="addItem">
              <Icon name="plus" size="sm" />
              {{ t('payment.admin.apiOnboarding.addCard') }}
            </button>
          </div>
          <div class="space-y-2 p-3">
            <div
              v-for="(item, index) in config.items"
              :key="item.id"
              class="w-full rounded-lg border p-3 text-left transition-colors"
              :class="selectedItemId === item.id ? 'border-primary-300 bg-primary-50/70 dark:border-primary-500/50 dark:bg-primary-500/10' : 'border-gray-200 bg-white hover:border-primary-200 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-primary-500/40'"
              @click="selectedItemId = item.id"
            >
              <div class="flex items-start gap-2">
                <div class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-300">
                  <Icon :name="itemIcon(item.cta)" size="sm" />
                </div>
                <div class="min-w-0 flex-1">
                  <input v-model="item.title" type="text" class="input h-9" :placeholder="t('payment.admin.apiOnboarding.cardTitlePlaceholder')" @click.stop />
                  <div class="mt-2 flex items-center justify-between gap-3 text-xs text-gray-500 dark:text-dark-400">
                    <label class="inline-flex items-center gap-2">
                      <input v-model="item.enabled" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" @click.stop />
                      {{ t('payment.admin.apiOnboarding.showCard') }}
                    </label>
                    <span>{{ ctaLabel(item.cta) }}</span>
                  </div>
                </div>
                <div class="flex shrink-0 items-center gap-1">
                  <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 disabled:opacity-40 dark:text-gray-300 dark:hover:bg-dark-700" :aria-label="t('payment.admin.moveUp')" :disabled="index === 0" @click.stop="moveItem(index, -1)">
                    <Icon name="arrowUp" size="sm" />
                  </button>
                  <button type="button" class="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 disabled:opacity-40 dark:text-gray-300 dark:hover:bg-dark-700" :aria-label="t('payment.admin.moveDown')" :disabled="index === config.items.length - 1" @click.stop="moveItem(index, 1)">
                    <Icon name="arrowDown" size="sm" />
                  </button>
                  <button type="button" class="rounded-md p-1.5 text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10" :aria-label="t('common.delete')" @click.stop="removeItem(item.id)">
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </div>
            </div>
            <p v-if="config.items.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('payment.admin.apiOnboarding.noCards') }}</p>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div v-if="selectedItem" class="space-y-4">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ selectedItem.title || t('payment.admin.apiOnboarding.editCard') }}</h3>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('payment.admin.apiOnboarding.editHint') }}</p>
              </div>
              <span class="rounded-md border border-gray-200 bg-gray-50 px-2 py-1 text-xs font-medium text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300">
                {{ selectedItem.enabled ? t('common.enabled') : t('common.disabled') }}
              </span>
            </div>

            <div class="grid gap-3 md:grid-cols-2">
              <div>
                <label class="input-label">{{ t('payment.admin.apiOnboarding.cardTitle') }}</label>
                <input v-model="selectedItem.title" type="text" class="input" :placeholder="t('payment.admin.apiOnboarding.cardTitlePlaceholder')" />
              </div>
              <div>
                <label class="input-label">{{ t('payment.admin.apiOnboarding.badge') }}</label>
                <input v-model="selectedItem.badge" type="text" class="input" :placeholder="t('payment.admin.apiOnboarding.badgePlaceholder')" />
              </div>
            </div>

            <div>
              <label class="input-label">{{ t('payment.admin.apiOnboarding.description') }}</label>
              <textarea v-model="selectedItem.description" class="input min-h-[88px] resize-y" :placeholder="t('payment.admin.apiOnboarding.descriptionPlaceholder')" />
            </div>

            <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <div>
                <label class="input-label">{{ t('payment.admin.apiOnboarding.ctaLabel') }}</label>
                <select v-model="selectedItem.cta" class="input">
                  <option v-for="option in ctaOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
              </div>
              <div>
                <label class="input-label">{{ t('payment.admin.apiOnboarding.audienceLabel') }}</label>
                <select v-model="selectedItem.audience" class="input">
                  <option value="new_users">{{ t('payment.admin.apiOnboarding.audience.newUsers') }}</option>
                  <option value="all_users">{{ t('payment.admin.apiOnboarding.audience.allUsers') }}</option>
                </select>
              </div>
              <div>
                <label class="input-label">{{ t('payment.admin.apiOnboarding.groupLabel') }}</label>
                <select v-model.number="selectedItem.group_id" class="input">
                  <option :value="0">{{ t('payment.admin.apiOnboarding.noGroup') }}</option>
                  <option v-for="group in groups" :key="group.id" :value="group.id">{{ groupLabel(group) }}</option>
                </select>
              </div>
              <div>
                <label class="input-label">{{ t('payment.admin.apiOnboarding.minBalance') }}</label>
                <input v-model.number="selectedItem.min_balance" type="number" min="0" step="0.01" class="input" />
              </div>
            </div>

            <div>
              <label class="input-label">{{ t('payment.admin.apiOnboarding.planLabel') }}</label>
              <select v-model.number="selectedItem.plan_id" class="input">
                <option :value="0">{{ t('payment.admin.apiOnboarding.noPlan') }}</option>
                <option v-for="plan in plans" :key="plan.id" :value="plan.id">{{ planLabel(plan) }}</option>
              </select>
              <p class="input-hint">{{ t('payment.admin.apiOnboarding.planHint') }}</p>
            </div>
          </div>
          <p v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('payment.admin.apiOnboarding.selectCardFirst') }}</p>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { APIOnboardingConfig, APIOnboardingCTA, APIOnboardingItem, AdminGroup } from '@/types'
import type { SubscriptionPlan } from '@/types/payment'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  plans: SubscriptionPlan[]
  groups: AdminGroup[]
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const selectedItemId = ref('')
const config = reactive<APIOnboardingConfig>({
  enabled: false,
  title: '',
  subtitle: '',
  items: [],
})

const ctaOptions = computed(() => [
  { value: 'create_key', label: t('payment.admin.apiOnboarding.cta.createKey') },
  { value: 'recharge', label: t('payment.admin.apiOnboarding.cta.recharge') },
  { value: 'buy_plan', label: t('payment.admin.apiOnboarding.cta.buyPlan') },
  { value: 'open_docs', label: t('payment.admin.apiOnboarding.cta.openDocs') },
])

const selectedItem = computed(() => config.items.find(item => item.id === selectedItemId.value) || null)

function replaceConfig(next: APIOnboardingConfig) {
  config.enabled = next.enabled === true
  config.title = next.title || ''
  config.subtitle = next.subtitle || ''
  config.items.splice(0, config.items.length, ...(next.items || []).map(normalizeItemForEdit))
  selectedItemId.value = config.items[0]?.id || ''
}

function normalizeItemForEdit(item: APIOnboardingItem): APIOnboardingItem {
  return {
    id: item.id || createLocalId('onboarding'),
    title: item.title || '',
    description: item.description || '',
    badge: item.badge || '',
    enabled: item.enabled !== false,
    sort_order: item.sort_order || 0,
    group_id: normalizeOptionalId(item.group_id),
    plan_id: normalizeOptionalId(item.plan_id),
    min_balance: Math.max(0, Number(item.min_balance || 0)),
    cta: normalizeCTA(item.cta),
    audience: item.audience === 'all_users' ? 'all_users' : 'new_users',
  }
}

function normalizeOptionalId(value: unknown): number | null {
  const id = Number(value || 0)
  return Number.isFinite(id) && id > 0 ? id : null
}

function createLocalId(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`
}

function normalizeCTA(value: string): APIOnboardingCTA {
  return ['create_key', 'recharge', 'buy_plan', 'open_docs'].includes(value) ? value as APIOnboardingCTA : 'create_key'
}

function ctaLabel(value: string): string {
  return ctaOptions.value.find(option => option.value === value)?.label || t('payment.admin.apiOnboarding.cta.createKey')
}

function itemIcon(value: string): 'key' | 'creditCard' | 'dollar' | 'book' {
  switch (value) {
    case 'buy_plan':
      return 'creditCard'
    case 'recharge':
      return 'dollar'
    case 'open_docs':
      return 'book'
    default:
      return 'key'
  }
}

function groupLabel(group: AdminGroup): string {
  return `${group.name} #${group.id}`
}

function planLabel(plan: SubscriptionPlan): string {
  return `${plan.product_name?.trim() || plan.name} #${plan.id}`
}

async function loadConfig() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getAPIOnboardingConfig()
    replaceConfig(res.data)
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function addItem() {
  const item: APIOnboardingItem = {
    id: createLocalId('onboarding'),
    title: t('payment.admin.apiOnboarding.newCardTitle'),
    description: '',
    badge: '',
    enabled: true,
    sort_order: config.items.length + 1,
    group_id: null,
    plan_id: null,
    min_balance: 0,
    cta: 'create_key',
    audience: 'new_users',
  }
  config.items.push(item)
  selectedItemId.value = item.id
}

function removeItem(id: string) {
  const index = config.items.findIndex(item => item.id === id)
  if (index === -1) return
  config.items.splice(index, 1)
  selectedItemId.value = config.items[Math.min(index, config.items.length - 1)]?.id || ''
}

function moveItem(index: number, direction: -1 | 1) {
  const next = index + direction
  if (next < 0 || next >= config.items.length) return
  const [item] = config.items.splice(index, 1)
  config.items.splice(next, 0, item)
}

function payload(): APIOnboardingConfig {
  return {
    enabled: config.enabled,
    title: config.title.trim(),
    subtitle: config.subtitle.trim(),
    items: config.items.map((item, index) => ({
      ...item,
      id: item.id.trim() || createLocalId('onboarding'),
      title: item.title.trim(),
      description: item.description.trim(),
      badge: item.badge.trim(),
      sort_order: index + 1,
      group_id: normalizeOptionalId(item.group_id),
      plan_id: normalizeOptionalId(item.plan_id),
      min_balance: Math.max(0, Number(item.min_balance || 0)),
      cta: normalizeCTA(item.cta),
      audience: item.audience === 'all_users' ? 'all_users' : 'new_users',
    })),
  }
}

function validateConfig(next: APIOnboardingConfig): boolean {
  const groupIds = new Set(props.groups.map(group => group.id))
  const planIds = new Set(props.plans.map(plan => plan.id))
  const titles = new Set<string>()
  for (const item of next.items) {
    if (!item.enabled) continue
    if (!item.title) {
      appStore.showError(t('payment.admin.apiOnboarding.validation.titleRequired'))
      return false
    }
    const titleKey = item.title.toLowerCase()
    if (titles.has(titleKey)) {
      appStore.showError(t('payment.admin.apiOnboarding.validation.duplicateTitle'))
      return false
    }
    titles.add(titleKey)
    if (item.group_id && !groupIds.has(item.group_id)) {
      appStore.showError(t('payment.admin.apiOnboarding.validation.groupMissing'))
      return false
    }
    if (item.plan_id && !planIds.has(item.plan_id)) {
      appStore.showError(t('payment.admin.apiOnboarding.validation.planMissing'))
      return false
    }
    if (item.cta === 'buy_plan' && !item.plan_id) {
      appStore.showError(t('payment.admin.apiOnboarding.validation.planRequired'))
      return false
    }
  }
  return true
}

async function saveConfig() {
  const next = payload()
  if (!validateConfig(next)) return
  saving.value = true
  try {
    const res = await adminPaymentAPI.updateAPIOnboardingConfig(next)
    replaceConfig(res.data)
    appStore.showSuccess(t('payment.admin.apiOnboarding.saved'))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    saving.value = false
  }
}

watch(
  () => [props.plans.map(plan => plan.id).join(','), props.groups.map(group => group.id).join(',')],
  () => {
    const groupIds = new Set(props.groups.map(group => group.id))
    const planIds = new Set(props.plans.map(plan => plan.id))
    config.items.forEach((item) => {
      if (item.group_id && !groupIds.has(item.group_id)) item.group_id = null
      if (item.plan_id && !planIds.has(item.plan_id)) item.plan_id = null
    })
  }
)

onMounted(loadConfig)
</script>
