<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import couponAPI from '@/api/coupon'
import { useAuthStore } from '@/stores/auth'
import type { CouponPaymentQuote, UserCoupon } from '@/types/coupon'
import { formatCurrency, formatDateTime } from '@/utils/format'

const props = defineProps<{
  modelValue: number | null
  amount: number
  orderType: 'balance' | 'subscription'
  planId?: number
  paymentType: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
  quote: [value: CouponPaymentQuote | null]
}>()

const { t } = useI18n()
const authStore = useAuthStore()
const coupons = ref<UserCoupon[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 50
const loading = ref(false)
const loadFailed = ref(false)
const quoting = ref(false)
const quoteError = ref('')
const currentQuote = ref<CouponPaymentQuote | null>(null)
const activeQuoteCouponID = ref<number | null>(null)
let quoteRequest = 0
let couponLoadRequest = 0

const selectedCoupon = computed(() => coupons.value.find((coupon) => coupon.id === props.modelValue) ?? null)
const canQuote = computed(() => props.amount > 0 && !!props.paymentType)
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
  quoteRequest += 1
  coupons.value = []
  total.value = 0
  page.value = 1
  loading.value = false
  loadFailed.value = false
  quoting.value = false
  quoteError.value = ''
  currentQuote.value = null
  activeQuoteCouponID.value = null
  emit('quote', null)
  if (props.modelValue !== null) emit('update:modelValue', null)
}

function termsOf(coupon: UserCoupon) {
  return coupon.terms_snapshot
}

function benefitLabel(coupon: UserCoupon): string {
  const terms = termsOf(coupon)
  if (terms.benefit_type === 'percentage') {
    const cap = terms.max_discount_amount
      ? t('coupon.selector.percentCapped', {
          percent: terms.benefit_value,
          cap: formatCurrency(terms.max_discount_amount, terms.currency),
        })
      : t('coupon.selector.percent', { percent: terms.benefit_value })
    return cap
  }
  return t('coupon.selector.fixed', { amount: formatCurrency(terms.benefit_value, terms.currency) })
}

function scopeLabel(coupon: UserCoupon): string {
  const scopes = termsOf(coupon).applicable_scopes
  if (scopes.length === 2) return t('coupon.selector.scopeBoth')
  return scopes[0] === 'subscription' ? t('coupon.selector.scopeSubscription') : t('coupon.selector.scopeRecharge')
}

function minimumLabel(coupon: UserCoupon): string {
  const terms = termsOf(coupon)
  return terms.minimum_order_amount > 0
    ? t('coupon.selector.minimum', { amount: formatCurrency(terms.minimum_order_amount, terms.currency) })
    : t('coupon.selector.noMinimum')
}

function isPendingActivation(coupon: UserCoupon): boolean {
  const validFrom = Date.parse(coupon.valid_from)
  return Number.isFinite(validFrom) && validFrom > Date.now()
}

async function loadCoupons() {
  const request = ++couponLoadRequest
  const sessionKey = couponSessionKey()
  if (!authStore.isAuthenticated) {
    coupons.value = []
    total.value = 0
    loading.value = false
    loadFailed.value = false
    return
  }
  loading.value = true
  loadFailed.value = false
  try {
    const response = await couponAPI.getMyCoupons({ page: page.value, page_size: pageSize, status: 'available' })
    if (request !== couponLoadRequest || !isCurrentCouponSession(sessionKey)) return
    coupons.value = response.data.items || []
    total.value = response.data.total || 0
    if (page.value > pages.value) {
      page.value = pages.value
      void loadCoupons()
      return
    }
    if (props.modelValue && !coupons.value.some((coupon) => coupon.id === props.modelValue)) {
      clearSelection()
      return
    }
    const initialCoupon = selectedCoupon.value
    if (initialCoupon && activeQuoteCouponID.value !== initialCoupon.id) {
      void quoteCoupon(initialCoupon)
    }
  } catch {
    if (request !== couponLoadRequest || !isCurrentCouponSession(sessionKey)) return
    coupons.value = []
    total.value = 0
    loadFailed.value = true
  } finally {
    if (request === couponLoadRequest && isCurrentCouponSession(sessionKey)) loading.value = false
  }
}

function changePage(next: number) {
  if (props.disabled || next < 1 || next > pages.value || next === page.value || loading.value) return
  clearSelection()
  page.value = next
  void loadCoupons()
}

function cancelQuote() {
  quoteRequest += 1
  quoting.value = false
  quoteError.value = ''
  currentQuote.value = null
  activeQuoteCouponID.value = null
  emit('quote', null)
}

function clearSelection() {
  cancelQuote()
  emit('update:modelValue', null)
}

async function quoteCoupon(coupon: UserCoupon) {
  if (props.disabled) return
  const request = ++quoteRequest
  const sessionKey = couponSessionKey()
  if (!authStore.isAuthenticated) return
  activeQuoteCouponID.value = coupon.id
  emit('update:modelValue', coupon.id)
  quoteError.value = ''
  currentQuote.value = null
  emit('quote', null)

  if (!canQuote.value) return
  quoting.value = true
  try {
    const response = await couponAPI.quotePaymentCoupon({
      coupon_id: coupon.id,
      payment_type: props.paymentType,
      order_type: props.orderType,
      amount: props.amount,
      plan_id: props.planId,
    })
    if (request !== quoteRequest || !isCurrentCouponSession(sessionKey)) return
    currentQuote.value = response.data
    emit('quote', response.data)
  } catch {
    if (request !== quoteRequest || !isCurrentCouponSession(sessionKey)) return
    quoteError.value = t('coupon.selector.quoteFailed')
    currentQuote.value = null
    emit('quote', null)
  } finally {
    if (request === quoteRequest && isCurrentCouponSession(sessionKey)) quoting.value = false
  }
}

watch(
  () => [props.amount, props.orderType, props.planId, props.paymentType] as const,
  () => {
    if (!selectedCoupon.value) return
    void quoteCoupon(selectedCoupon.value)
  },
)

watch(
  () => props.modelValue,
  (couponID) => {
    if (couponID !== activeQuoteCouponID.value) cancelQuote()
  },
)

onMounted(() => { void loadCoupons() })

onBeforeUnmount(() => {
  couponLoadRequest += 1
  cancelQuote()
})

watch(
  () => [authStore.isAuthenticated, authStore.user?.id, authStore.token] as const,
  () => {
    resetCouponState()
    void loadCoupons()
  },
)
</script>

<template>
  <section class="coupon-selector" :aria-busy="loading || quoting">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('coupon.selector.title') }}</h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('coupon.selector.description') }}</p>
      </div>
      <router-link to="/wallet#coupons" class="btn btn-secondary btn-sm">
        {{ t('coupon.selector.viewWallet') }}
      </router-link>
    </div>

    <p v-if="!canQuote" class="mt-4 text-sm text-gray-500 dark:text-gray-400">
      {{ t('coupon.selector.enterAmount') }}
    </p>
    <div v-else-if="loading" class="mt-4 text-sm text-gray-500 dark:text-gray-400">
      {{ t('coupon.selector.loading') }}
    </div>
    <div v-else-if="loadFailed" class="mt-4 flex flex-wrap items-center gap-3 text-sm text-rose-600 dark:text-rose-300">
      <span>{{ t('coupon.selector.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="loadCoupons">{{ t('common.retry') }}</button>
    </div>
    <p v-else-if="coupons.length === 0" class="mt-4 text-sm text-gray-500 dark:text-gray-400">
      {{ t('coupon.selector.empty') }}
    </p>
    <div v-else class="mt-4 divide-y divide-gray-100 overflow-hidden rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-600">
      <label
        v-for="coupon in coupons"
        :key="coupon.id"
        :class="[
          'flex items-start gap-3 px-4 py-3 text-sm text-gray-700 dark:text-gray-200',
          isPendingActivation(coupon) ? 'cursor-not-allowed opacity-60' : 'cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-700/50',
        ]"
      >
        <input
          class="mt-1 h-4 w-4 shrink-0"
          type="radio"
          name="payment-coupon"
          :checked="coupon.id === modelValue"
          :disabled="quoting || disabled || isPendingActivation(coupon)"
          @change="quoteCoupon(coupon)"
        />
        <span class="min-w-0 flex-1">
          <span class="flex flex-wrap items-center justify-between gap-2">
            <strong class="text-gray-900 dark:text-white">{{ coupon.template_name || coupon.terms_snapshot.name }}</strong>
            <span class="font-medium text-primary-700 dark:text-primary-300">{{ benefitLabel(coupon) }}</span>
          </span>
          <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
            {{ scopeLabel(coupon) }} · {{ minimumLabel(coupon) }} · {{ t('coupon.selector.expiresAt', { time: formatDateTime(coupon.expires_at) }) }}
          </span>
          <span v-if="isPendingActivation(coupon)" class="mt-1 block text-xs text-amber-700 dark:text-amber-200">
            {{ t('coupon.selector.availableAt', { time: formatDateTime(coupon.valid_from) }) }}
          </span>
        </span>
      </label>
    </div>

    <div v-if="pages > 1" class="mt-3 flex flex-wrap items-center justify-between gap-3 text-sm">
      <span class="text-gray-500 dark:text-gray-400">{{ t('coupon.selector.pageInfo', { page, pages }) }}</span>
      <div class="flex gap-2">
        <button type="button" class="btn btn-secondary btn-sm" data-test="coupon-page-previous" :disabled="page <= 1 || loading || disabled" @click="changePage(page - 1)">
          {{ t('common.previous') }}
        </button>
        <button type="button" class="btn btn-secondary btn-sm" data-test="coupon-page-next" :disabled="page >= pages || loading || disabled" @click="changePage(page + 1)">
          {{ t('common.next') }}
        </button>
      </div>
    </div>

    <div v-if="currentQuote" class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-gray-200 pt-3 text-sm dark:border-dark-600">
      <span class="text-gray-600 dark:text-gray-300">{{ t('coupon.selector.quoteDiscount') }}</span>
      <strong class="text-primary-700 dark:text-primary-300">-{{ formatCurrency(currentQuote.discount_amount, currentQuote.payment_currency) }}</strong>
    </div>
    <p v-if="quoting" class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t('coupon.selector.quoting') }}</p>
    <p v-else-if="quoteError" class="mt-3 text-sm text-rose-600 dark:text-rose-300">{{ quoteError }}</p>
    <button v-if="modelValue" type="button" class="btn btn-ghost btn-sm mt-3" :disabled="quoting || disabled" @click="clearSelection">
      {{ t('coupon.selector.clear') }}
    </button>
  </section>
</template>
