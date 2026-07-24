<template>
  <component
    :is="isPopup ? 'main' : AppLayout"
    :class="isPopup ? 'bg-primary-50 px-4 py-8 dark:bg-dark-950' : ''"
  >
    <component
      :is="isPopup ? PageFrame : 'div'"
      :frame="isPopup ? 'compact' : undefined"
      :class="isPopup ? '' : 'py-8'"
    >
      <CompactStatusPanel
        v-if="loading"
        :title="t('payment.stripePay')"
        :description="t('payment.qr.payInNewWindowHint')"
        tone="primary"
        loading
      />

      <CompactStatusPanel
        v-else-if="initError"
        :title="t('payment.stripeLoadFailed')"
        :description="initError"
        icon="exclamationCircle"
        tone="danger"
      >
        <template #actions>
          <button class="btn btn-primary w-full sm:w-auto" @click="router.push('/purchase')">
            <Icon name="creditCard" size="md" />
            {{ t('payment.result.backToRecharge') }}
          </button>
        </template>
      </CompactStatusPanel>

      <template v-else>
        <CompactStatusPanel
          v-if="wechatQrUrl"
          :title="t('payment.qr.scanWxpay')"
          :description="t('payment.qr.scanWxpayHint')"
          icon="creditCard"
          tone="primary"
        >
          <template #details>
            <div v-if="order" class="rounded-lg border border-gray-200 bg-white p-4 text-center dark:border-dark-700 dark:bg-dark-800">
              <p class="text-sm font-medium text-gray-500 dark:text-dark-400">{{ t('payment.actualPay') }}</p>
              <p class="mt-1 text-2xl font-semibold tabular-nums text-gray-950 dark:text-white">
                {{ formatGatewayAmount(order.pay_amount) }}
              </p>
            </div>
            <div class="mt-4 flex justify-center">
              <div class="relative rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-800/60 dark:bg-emerald-950/30">
                <img :src="wechatQrUrl" alt="WeChat Pay QR" class="h-56 w-56 rounded-md object-contain" />
                <div class="pointer-events-none absolute inset-0 flex items-center justify-center">
                  <span class="rounded-full bg-emerald-600 p-2 text-white shadow-sm ring-2 ring-white dark:ring-dark-900">
                    <Icon name="creditCard" size="sm" />
                  </span>
                </div>
              </div>
            </div>
            <p class="mt-4 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ t('payment.qr.waitingPayment') }}
            </p>
          </template>
        </CompactStatusPanel>

        <CompactStatusPanel
          v-else-if="redirecting"
          :title="t('payment.stripePay')"
          :description="t('payment.qr.payInNewWindowHint')"
          tone="primary"
          loading
        >
          <template #details>
            <div v-if="order" class="rounded-lg border border-gray-200 bg-white p-4 text-center dark:border-dark-700 dark:bg-dark-800">
              <p class="text-sm font-medium text-gray-500 dark:text-dark-400">{{ t('payment.actualPay') }}</p>
              <p class="mt-1 text-2xl font-semibold tabular-nums text-gray-950 dark:text-white">
                {{ formatGatewayAmount(order.pay_amount) }}
              </p>
            </div>
          </template>
        </CompactStatusPanel>

        <CompactStatusPanel
          v-else-if="stripeSuccess"
          :title="t('payment.result.success')"
          :description="t('payment.stripeSuccessProcessing')"
          icon="checkCircle"
          tone="success"
        />

        <CompactStatusPanel
          v-else-if="showPaymentElement"
          :title="t('payment.stripePay')"
          :description="t('payment.qr.payInNewWindowHint')"
          icon="creditCard"
          tone="primary"
        >
          <template #details>
            <div v-if="order" class="mb-4 rounded-lg border border-gray-200 bg-white p-4 text-center dark:border-dark-700 dark:bg-dark-800">
              <p class="text-sm font-medium text-gray-500 dark:text-dark-400">{{ t('payment.actualPay') }}</p>
              <p class="mt-1 text-2xl font-semibold tabular-nums text-gray-950 dark:text-white">
                {{ formatGatewayAmount(order.pay_amount) }}
              </p>
            </div>
            <div id="stripe-payment-element" class="min-h-[200px]"></div>
            <p v-if="stripeError" class="mt-4 text-sm text-red-600 dark:text-red-400">{{ stripeError }}</p>
          </template>
          <template #actions>
            <button
              class="btn btn-stripe w-full py-3 text-base sm:w-auto"
              :disabled="stripeSubmitting || !stripeReady"
              @click="handleGenericPay"
            >
              <LoadingSpinner v-if="stripeSubmitting" size="sm" color="secondary" />
              <Icon v-else name="creditCard" size="md" />
              {{ stripeSubmitting ? t('common.processing') : t('payment.stripePay') }}
            </button>
            <button class="btn btn-secondary w-full sm:w-auto" @click="router.push('/purchase')">
              <Icon name="arrowLeft" size="md" />
              {{ t('payment.result.backToRecharge') }}
            </button>
          </template>
        </CompactStatusPanel>

        <CompactStatusPanel
          v-if="stripeError && !showPaymentElement"
          :title="t('payment.result.failed')"
          :description="stripeError"
          icon="exclamationCircle"
          tone="danger"
        >
          <template #actions>
            <button class="btn btn-secondary w-full sm:w-auto" @click="router.push('/purchase')">
              <Icon name="arrowLeft" size="md" />
              {{ t('payment.result.backToRecharge') }}
            </button>
          </template>
        </CompactStatusPanel>
      </template>
    </component>
  </component>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { usePaymentStore } from '@/stores/payment'
import { paymentAPI } from '@/api/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { isMobileDevice } from '@/utils/device'
import { formatPaymentAmount, normalizePaymentCurrency } from '@/components/payment/currency'
import { PAYMENT_RECOVERY_STORAGE_KEY, readPaymentRecoverySnapshot } from '@/components/payment/paymentFlow'
import { isPaymentSuccessStatus } from '@/components/payment/orderUtils'
import type { PaymentOrder } from '@/types/payment'
import type { Stripe, StripeElements } from '@stripe/stripe-js'
import AppLayout from '@/components/layout/AppLayout.vue'
import CompactStatusPanel from '@/components/common/CompactStatusPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import PageFrame from '@/components/layout/PageFrame.vue'

const i18n = useI18n()
const { t } = i18n
const route = useRoute()
const router = useRouter()
const paymentStore = usePaymentStore()

// 弹窗模式：指定支付宝或微信方式时跳过 AppLayout
const isPopup = computed(() => !!route.query.method)

const loading = ref(true)
const initError = ref('')
const stripeError = ref('')
const stripeSubmitting = ref(false)
const stripeSuccess = ref(false)
const stripeReady = ref(false)
const order = ref<PaymentOrder | null>(null)
const currency = ref('CNY')
const wechatQrUrl = ref('')
const redirecting = ref(false)
const showPaymentElement = ref(false)

let stripeInstance: Stripe | null = null
let elementsInstance: StripeElements | null = null
let redirectTimer: ReturnType<typeof setTimeout> | null = null

onMounted(async () => {
  const orderId = Number(route.query.order_id)
  const clientSecret = String(route.query.client_secret || '')
  const method = String(route.query.method || '')
  const resumeToken = typeof route.query.resume_token === 'string' ? route.query.resume_token : undefined

  if (!orderId || !clientSecret) {
    loading.value = false
    initError.value = t('payment.stripeMissingParams')
    return
  }

  try {
    if (typeof window !== 'undefined') {
      const restored = readPaymentRecoverySnapshot(
        window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY),
        { resumeToken },
      )
      if (restored?.orderId === orderId) {
        currency.value = normalizePaymentCurrency(restored.currency)
      }
    }
    const res = await paymentAPI.getOrder(orderId)
    order.value = res.data
    if (res.data.currency) {
      currency.value = normalizePaymentCurrency(res.data.currency)
    }

    await paymentStore.fetchConfig()
    const publishableKey = paymentStore.config?.stripe_publishable_key
    if (!publishableKey) { initError.value = t('payment.stripeNotConfigured'); return }

    const { loadStripe } = await import('@stripe/stripe-js/pure')
    const stripe = await loadStripe(publishableKey)
    if (!stripe) { initError.value = t('payment.stripeLoadFailed'); return }

    stripeInstance = stripe
    loading.value = false

    // 指定方式直接确认，无需渲染完整 Payment Element
    if (method === 'alipay') {
      await confirmAlipay(stripe, clientSecret, orderId)
    } else if (method === 'wechat_pay') {
      await confirmWechatPay(stripe, clientSecret)
    } else {
      // 未指定方式时渲染完整 Payment Element
      showPaymentElement.value = true
      await nextTick()
      mountPaymentElement(stripe, clientSecret)
    }
  } catch (err: unknown) {
    initError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('payment.stripeLoadFailed'))
  } finally {
    loading.value = false
  }
})

const localeCode = computed(() => {
  const raw = i18n.locale as unknown
  if (typeof raw === 'string') return raw
  if (raw && typeof raw === 'object' && 'value' in raw) {
    return String((raw as { value?: string }).value || '')
  }
  return undefined
})

function formatGatewayAmount(value: number): string {
  return formatPaymentAmount(value, currency.value, localeCode.value)
}

async function confirmAlipay(stripe: Stripe, clientSecret: string, orderId: number) {
  redirecting.value = true
  const returnUrl = window.location.origin + '/payment/result?order_id=' + orderId + '&status=success'
  const { error } = await stripe.confirmAlipayPayment(clientSecret, { return_url: returnUrl })
  if (error) {
    redirecting.value = false
    stripeError.value = error.message || t('payment.result.failed')
  }
  // 无错误时 Stripe 会自动跳转
}

async function confirmWechatPay(stripe: Stripe, clientSecret: string) {
  const { paymentIntent, error } = await (stripe as Stripe & {
    confirmWechatPayPayment: (cs: string, opts: Record<string, unknown>) => Promise<{ paymentIntent?: { status: string; next_action?: { wechat_pay_display_qr_code?: { image_data_url?: string } } }; error?: { message?: string } }>
  }).confirmWechatPayPayment(clientSecret, {
    payment_method_options: { wechat_pay: { client: isMobileDevice() ? 'mobile_web' : 'web' } },
  })

  if (error) {
    stripeError.value = error.message || t('payment.result.failed')
    return
  }

  // 从 next_action 中提取二维码
  const qrData = paymentIntent?.next_action?.wechat_pay_display_qr_code?.image_data_url
  if (qrData) {
    wechatQrUrl.value = qrData
    // 轮询支付完成状态
    startPolling()
  } else if (paymentIntent?.status === 'succeeded') {
    stripeSuccess.value = true
    scheduleClose()
  } else {
    stripeError.value = t('payment.result.failed')
  }
}

function mountPaymentElement(stripe: Stripe, clientSecret: string) {
  const isDark = document.documentElement.classList.contains('dark')
  const elements = stripe.elements({
    clientSecret,
    appearance: { theme: isDark ? 'night' : 'stripe', variables: { borderRadius: '8px' } },
  })
  elementsInstance = elements
  const paymentElement = elements.create('payment', {
    layout: 'tabs',
    paymentMethodOrder: ['alipay', 'wechat_pay', 'card', 'link'],
  } as Record<string, unknown>)
  paymentElement.mount('#stripe-payment-element')
  paymentElement.on('ready', () => { stripeReady.value = true })
}

async function handleGenericPay() {
  if (!stripeInstance || !elementsInstance || stripeSubmitting.value) return
  stripeSubmitting.value = true
  stripeError.value = ''
  try {
    const { error } = await stripeInstance.confirmPayment({
      elements: elementsInstance,
      confirmParams: {
        return_url: window.location.origin + '/payment/result?order_id=' + route.query.order_id + '&status=success',
      },
      redirect: 'if_required',
    })
    if (error) {
      stripeError.value = error.message || t('payment.result.failed')
    } else {
      stripeSuccess.value = true
      scheduleClose()
    }
  } catch (err: unknown) {
    stripeError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('payment.result.failed'))
  } finally {
    stripeSubmitting.value = false
  }
}

let pollTimer: ReturnType<typeof setInterval> | null = null

function startPolling() {
  const orderId = Number(route.query.order_id)
  if (!orderId) return
  pollTimer = setInterval(async () => {
    const o = await paymentStore.pollOrderStatus(orderId)
    if (!o) return
    if (isPaymentSuccessStatus(o.status)) {
      if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
      stripeSuccess.value = true
      wechatQrUrl.value = ''
      scheduleClose()
    }
  }, 3000)
}

function scheduleClose() {
  if (window.opener) {
    redirectTimer = setTimeout(() => { window.close() }, 2000)
  } else {
    redirectTimer = setTimeout(() => {
      router.push({ path: '/payment/result', query: { order_id: String(route.query.order_id || ''), status: 'success' } })
    }, 2000)
  }
}

onUnmounted(() => {
  if (redirectTimer) clearTimeout(redirectTimer)
  if (pollTimer) clearInterval(pollTimer)
})
</script>
