<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import couponAPI from '@/api/coupon'
import type { UserCoupon } from '@/types/coupon'
import { couponErrorKey } from '@/utils/couponErrors'
const props = defineProps<{ disabled?: boolean }>()
const emit = defineEmits<{ claimed: [coupon: UserCoupon] }>()
const { t } = useI18n()
const auth = useAuthStore()
let generation = 0
watch(
  () => [auth.isAuthenticated, auth.user?.id, auth.token],
  () => {
    generation++
    code.value = ''
    busy.value = false
    error.value = ''
    success.value = false
  },
)
onBeforeUnmount(() => {
  generation++
})
const code = ref(''),
  busy = ref(false),
  error = ref(''),
  success = ref(false)
async function claim() {
  if (busy.value || props.disabled || !code.value.trim()) return
  const request = ++generation
  busy.value = true
  error.value = ''
  success.value = false
  try {
    const { data } = await couponAPI.claimCode(code.value.trim().toUpperCase())
    if (request !== generation) return
    emit('claimed', data)
    success.value = true
    code.value = ''
  } catch (e) {
    if (request !== generation) return
    error.value = t(couponErrorKey(e))
  } finally {
    if (request === generation) busy.value = false
  }
}
</script>
<template>
  <form class="my-4 space-y-2" :aria-busy="busy" @submit.prevent="claim">
    <label class="grid gap-1 text-sm"
      ><span class="input-label">{{ t('couponCampaign.enterCode') }}</span
      ><input
        v-model="code"
        class="input uppercase"
        maxlength="64"
        autocomplete="off"
        :disabled="busy || disabled"
        :aria-invalid="!!error"
    /></label>
    <button type="submit" class="btn btn-secondary" :disabled="busy || disabled || !code.trim()">
      {{ t(busy ? 'common.processing' : 'couponCampaign.claim') }}
    </button>
    <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <p v-if="success" role="status" class="text-sm text-emerald-700 dark:text-emerald-300">
      {{ t('couponCampaign.claimed') }}
    </p>
  </form>
</template>
