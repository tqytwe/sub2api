<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import couponAPI from '@/api/coupon'
import { useAuthStore } from '@/stores/auth'
import type { UserCoupon, UserCouponStatus } from '@/types/coupon'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { localizedEnumOrUnknown } from '@/utils/localizedEnum'
import Icon from '@/components/icons/Icon.vue'

type CouponWalletTab = Extract<UserCouponStatus, 'available' | 'locked' | 'used' | 'expired' | 'voided'>

const { t } = useI18n()
const authStore = useAuthStore()
const activeTab = ref<CouponWalletTab>('available')
const coupons = ref<UserCoupon[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const failed = ref(false)
let couponLoadRequest = 0
let suppressCouponReload = false

const tabs = computed(() => [
  { value: 'available' as const, label: t('coupon.wallet.tabs.available') },
  { value: 'locked' as const, label: t('coupon.wallet.tabs.locked') },
  { value: 'used' as const, label: t('coupon.wallet.tabs.used') },
  { value: 'expired' as const, label: t('coupon.wallet.tabs.expired') },
  { value: 'voided' as const, label: t('coupon.wallet.tabs.voided') },
])
const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

function couponSessionKey(): string {
  if (!authStore.isAuthenticated) return 'guest'
  return `user:${authStore.user?.id ?? ''}:token:${authStore.token ?? ''}`
}

function isCurrentCouponSession(sessionKey: string): boolean {
  return sessionKey === couponSessionKey()
}

function resetCouponState() {
  couponLoadRequest += 1
  coupons.value = []
  total.value = 0
  loading.value = false
  failed.value = false
  suppressCouponReload = true
  activeTab.value = 'available'
  page.value = 1
  void nextTick(() => {
    suppressCouponReload = false
    void loadCoupons()
  })
}

function termsOf(coupon: UserCoupon) {
  return coupon.terms_snapshot
}

function benefitLabel(coupon: UserCoupon): string {
  const terms = termsOf(coupon)
  if (terms.benefit_type === 'percentage') {
    return terms.max_discount_amount
      ? t('coupon.wallet.percentCapped', { percent: terms.benefit_value, cap: formatCurrency(terms.max_discount_amount, terms.currency) })
      : t('coupon.wallet.percent', { percent: terms.benefit_value })
  }
  return t('coupon.wallet.fixed', { amount: formatCurrency(terms.benefit_value, terms.currency) })
}

function scopeLabel(coupon: UserCoupon): string {
  const scopes = termsOf(coupon).applicable_scopes
  if (scopes.length === 2) return t('coupon.wallet.scopeBoth')
  return scopes[0] === 'subscription' ? t('coupon.wallet.scopeSubscription') : t('coupon.wallet.scopeRecharge')
}

function couponStatusLabel(coupon: UserCoupon): string {
  return localizedEnumOrUnknown(t, `coupon.wallet.status.${coupon.status}`)
}

function statusClass(status: UserCouponStatus): string {
  if (status === 'available') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-200'
  if (status === 'locked') return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-200'
  if (status === 'used') return 'bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-200'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
}

function isPendingActivation(coupon: UserCoupon): boolean {
  const validFrom = Date.parse(coupon.valid_from)
  return Number.isFinite(validFrom) && validFrom > Date.now()
}

function canUseCoupon(coupon: UserCoupon): boolean {
  return coupon.status === 'available' && !isPendingActivation(coupon)
}

async function loadCoupons() {
  const request = ++couponLoadRequest
  const sessionKey = couponSessionKey()
  if (!authStore.isAuthenticated) {
    coupons.value = []
    total.value = 0
    loading.value = false
    failed.value = false
    return
  }
  loading.value = true
  failed.value = false
  try {
    const response = await couponAPI.getMyCoupons({
      page: page.value,
      page_size: pageSize,
      status: activeTab.value,
    })
    if (request !== couponLoadRequest || !isCurrentCouponSession(sessionKey)) return
    coupons.value = response.data.items || []
    total.value = response.data.total || 0
  } catch {
    if (request !== couponLoadRequest || !isCurrentCouponSession(sessionKey)) return
    coupons.value = []
    total.value = 0
    failed.value = true
  } finally {
    if (request === couponLoadRequest && isCurrentCouponSession(sessionKey)) loading.value = false
  }
}

function selectTab(tab: CouponWalletTab) {
  if (activeTab.value === tab) return
  activeTab.value = tab
  page.value = 1
}

function changePage(next: number) {
  if (next < 1 || next > pages.value || loading.value) return
  page.value = next
}

watch([activeTab, page], () => {
  if (!suppressCouponReload) void loadCoupons()
})
onMounted(() => { void loadCoupons() })

onBeforeUnmount(() => {
  couponLoadRequest += 1
})

watch(
  () => [authStore.isAuthenticated, authStore.user?.id, authStore.token] as const,
  () => {
    resetCouponState()
  },
)
</script>

<template>
  <section id="coupons" class="card min-w-0 overflow-hidden">
    <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('coupon.wallet.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('coupon.wallet.description') }}</p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm inline-flex items-center gap-2 self-start" :disabled="loading" @click="loadCoupons">
        <Icon name="refresh" size="sm" />
        {{ t('common.refresh') }}
      </button>
    </div>

    <div class="border-b border-gray-100 px-5 pt-3 dark:border-dark-700">
      <div class="flex gap-1 overflow-x-auto" role="tablist" :aria-label="t('coupon.wallet.title')">
        <button
          v-for="tab in tabs"
          :key="tab.value"
          type="button"
          role="tab"
          :aria-selected="activeTab === tab.value"
          :class="[
            'min-h-10 shrink-0 border-b-2 px-3 text-sm font-medium transition-colors',
            activeTab === tab.value
              ? 'border-primary-600 text-primary-700 dark:border-primary-400 dark:text-primary-300'
              : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200',
          ]"
          @click="selectTab(tab.value)"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <div class="divide-y divide-gray-100 dark:divide-dark-700" :aria-busy="loading">
      <div v-if="loading" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('coupon.wallet.loading') }}</div>
      <div v-else-if="failed" class="px-5 py-10 text-center text-sm text-rose-600 dark:text-rose-300">
        {{ t('coupon.wallet.loadFailed') }}
      </div>
      <div v-else-if="coupons.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('coupon.wallet.empty') }}
      </div>
      <article v-for="coupon in coupons" :key="coupon.id" class="grid min-w-0 gap-3 px-5 py-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="break-words text-base font-semibold text-gray-900 dark:text-white">{{ coupon.template_name || coupon.terms_snapshot.name }}</h3>
            <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(coupon.status)">{{ isPendingActivation(coupon) ? t('coupon.wallet.pending') : couponStatusLabel(coupon) }}</span>
          </div>
          <p class="mt-1 text-sm font-medium text-primary-700 dark:text-primary-300">{{ benefitLabel(coupon) }}</p>
          <p class="mt-1 break-words text-xs text-gray-500 dark:text-gray-400">
            {{ scopeLabel(coupon) }} · {{ t('coupon.wallet.minimum', { amount: formatCurrency(coupon.terms_snapshot.minimum_order_amount, coupon.terms_snapshot.currency) }) }}
          </p>
          <p class="mt-1 break-words text-xs text-gray-500 dark:text-gray-400">
            {{ t('coupon.wallet.expiresAt', { time: formatDateTime(coupon.expires_at) }) }}
          </p>
          <p v-if="isPendingActivation(coupon)" class="mt-1 break-words text-xs text-amber-700 dark:text-amber-200">
            {{ t('coupon.wallet.availableAt', { time: formatDateTime(coupon.valid_from) }) }}
          </p>
          <p v-if="coupon.status === 'locked'" class="mt-1 break-words text-xs text-amber-700 dark:text-amber-200">
            {{ t('coupon.wallet.lockedHint') }}
          </p>
        </div>
        <div v-if="canUseCoupon(coupon)" class="flex flex-wrap gap-2 sm:justify-end">
          <router-link
            v-if="coupon.terms_snapshot.applicable_scopes.includes('balance')"
            :to="{ path: '/purchase', query: { tab: 'recharge' } }"
            class="btn btn-secondary btn-sm"
          >
            {{ t('coupon.wallet.useRecharge') }}
          </router-link>
          <router-link
            v-if="coupon.terms_snapshot.applicable_scopes.includes('subscription')"
            :to="{ path: '/purchase', query: { tab: 'subscription' } }"
            class="btn btn-primary btn-sm"
          >
            {{ t('coupon.wallet.useSubscription') }}
          </router-link>
        </div>
        <p v-else-if="coupon.status === 'available' && isPendingActivation(coupon)" class="text-xs text-amber-700 dark:text-amber-200">{{ t('coupon.wallet.pendingHint') }}</p>
      </article>
    </div>

    <div v-if="total > pageSize" class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 px-5 py-3 text-sm dark:border-dark-700">
      <span class="text-gray-500 dark:text-gray-400">{{ t('coupon.wallet.pageInfo', { page, pages }) }}</span>
      <div class="flex gap-2">
        <button type="button" class="btn btn-secondary btn-sm" :disabled="page <= 1 || loading" @click="changePage(page - 1)">{{ t('common.previous') }}</button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="page >= pages || loading" @click="changePage(page + 1)">{{ t('common.next') }}</button>
      </div>
    </div>
  </section>
</template>
