<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminCouponAPI as api } from '@/api/admin/coupon'
import type { CouponTemplate } from '@/types/coupon'
import type {
  CouponAudience,
  CouponClaimPolicy,
  CouponCampaignPreview,
  CouponCampaignResult,
} from '@/types/couponCampaign'
import CouponAudienceForm from './CouponAudienceForm.vue'
import { couponErrorKey } from '@/utils/couponErrors'
import { formatDateTime } from '@/utils/format'
const props = defineProps<{ templates: CouponTemplate[]; templateId?: number; policyMode?: boolean }>()
const emit = defineEmits<{ issued: [] }>()
const { t } = useI18n()
const template = ref(props.templateId || 0),
  audience = ref<CouponAudience>({ mode: 'users' }),
  policy = ref<CouponClaimPolicy | null>(null)
const preview = ref<CouponCampaignPreview | null>(null),
  result = ref<CouponCampaignResult | null>(null),
  busy = ref(false),
  error = ref(''),
  saved = ref(false)
let policyRequest = 0
let previewRequest = 0
watch(
  () => props.templateId,
  (id) => {
    template.value = id || 0
  },
)
watch(
  template,
  async (id) => {
    const request = ++policyRequest
    previewRequest++
    preview.value = null
    result.value = null
    saved.value = false
    policy.value = null
    error.value = ''
    if (!id) return
    try {
      const { data } = await api.getClaimPolicy(id)
      if (request === policyRequest) policy.value = data
    } catch (e) {
      if (request === policyRequest) error.value = t(couponErrorKey(e))
    }
  },
  { immediate: true },
)
watch(
  audience,
  () => {
    previewRequest++
    preview.value = null
    result.value = null
  },
  { deep: true, flush: 'sync' },
)
function localTime(v: string | null) {
  if (!v) return ''
  const d = new Date(v)
  return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
function setTime(key: 'starts_at' | 'ends_at', value: string) {
  if (policy.value) policy.value[key] = value ? new Date(value).toISOString() : null
}
async function copy() {
  if (!policy.value) return
  try {
    await navigator.clipboard.writeText(policy.value.code)
    saved.value = true
  } catch {
    error.value = t('couponCampaign.copyFailed')
  }
}
async function save() {
  if (!policy.value || busy.value) return
  busy.value = true
  error.value = ''
  saved.value = false
  try {
    const { data } = await api.saveClaimPolicy(template.value, policy.value)
    policy.value = data
    saved.value = true
  } catch (e) {
    error.value = t(couponErrorKey(e))
  } finally {
    busy.value = false
  }
}
async function prepare() {
  if (!template.value || busy.value) return
  const request = ++previewRequest
  busy.value = true
  error.value = ''
  preview.value = null
  result.value = null
  try {
    const response = await api.previewCampaign(template.value, audience.value)
    if (request === previewRequest) preview.value = response.data
  } catch (e) {
    if (request === previewRequest) error.value = t(couponErrorKey(e))
  } finally {
    busy.value = false
  }
}
async function execute() {
  if (!preview.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    result.value = (await api.executeCampaign(preview.value.token)).data
    preview.value = null
    emit('issued')
  } catch (e) {
    error.value = t(couponErrorKey(e))
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <section class="space-y-4" :aria-busy="busy">
    <template v-if="policyMode">
      <div v-if="policy" class="space-y-4">
        <div class="flex flex-wrap items-center gap-3">
          <code class="break-all text-sm">{{ policy.code }}</code
          ><button type="button" class="btn btn-secondary btn-sm" @click="copy">
            {{ t('common.copy') }}
          </button>
        </div>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{
            t('couponCampaign.policyCounts', {
              issued: policy.issued_count,
              manual: policy.manual_count,
              claimed: policy.claimed_count,
              used: policy.used_count,
            })
          }}
        </p>
        <form class="space-y-4" @submit.prevent="save">
          <fieldset class="grid gap-3 sm:grid-cols-2" :disabled="busy">
            <label class="flex items-center gap-2 text-sm sm:col-span-2"
              ><input v-model="policy.enabled" type="checkbox" />{{ t('couponCampaign.enable') }}</label
            >
            <label class="grid gap-1 text-sm"
              ><span class="input-label">{{ t('couponCampaign.claimLimit') }}</span
              ><input v-model.number="policy.claim_limit" type="number" min="0" class="input" required
            /></label>
            <label class="grid gap-1 text-sm"
              ><span class="input-label">{{ t('couponCampaign.perUserLimit') }}</span
              ><input
                v-model.number="policy.per_user_limit"
                type="number"
                min="1"
                max="10"
                class="input"
                required
            /></label>
            <label class="grid gap-1 text-sm"
              ><span class="input-label">{{ t('couponCampaign.starts') }}</span
              ><input
                :value="localTime(policy.starts_at)"
                type="datetime-local"
                class="input"
                @input="setTime('starts_at', ($event.target as HTMLInputElement).value)"
            /></label>
            <label class="grid gap-1 text-sm"
              ><span class="input-label">{{ t('couponCampaign.ends') }}</span
              ><input
                :value="localTime(policy.ends_at)"
                type="datetime-local"
                class="input"
                :required="policy.enabled"
                @input="setTime('ends_at', ($event.target as HTMLInputElement).value)"
            /></label>
          </fieldset>
          <CouponAudienceForm v-model="policy.audience" :disabled="busy" />
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('couponCampaign.policyHint') }}</p>
          <button type="submit" class="btn btn-primary" :disabled="busy">{{ t('common.save') }}</button>
        </form>
      </div>
      <p v-else-if="!error" role="status">{{ t('common.loading') }}</p>
    </template>
    <form v-else class="space-y-4" @submit.prevent="prepare">
      <label class="grid gap-1 text-sm"
        ><span class="input-label">{{ t('coupon.admin.template') }}</span
        ><select v-model.number="template" class="input" :disabled="busy" required>
          <option :value="0" disabled>{{ t('coupon.admin.selectTemplate') }}</option>
          <option v-for="v in templates.filter((v) => v.status === 'active')" :key="v.id" :value="v.id">
            {{ v.name }}
          </option>
        </select></label
      >
      <CouponAudienceForm v-model="audience" :disabled="busy" />
      <button type="submit" class="btn btn-primary" :disabled="busy || !template">
        {{ t('couponCampaign.preview') }}
      </button>
    </form>
    <div
      v-if="preview"
      class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700"
      role="status"
    >
      <details v-if="preview.recipients.length">
        <summary class="cursor-pointer text-sm">{{ t('couponCampaign.recipients') }}</summary>
        <p class="max-h-48 overflow-y-auto break-words text-sm">{{ preview.recipients.join(', ') }}</p>
      </details>
      <p>
        {{
          t('couponCampaign.previewCounts', {
            requested: preview.requested_count,
            eligible: preview.recipients.length,
            skipped: preview.skipped.length,
            stock: preview.remaining_stock ?? '∞',
          })
        }}
      </p>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('couponCampaign.previewHint', { time: formatDateTime(preview.expires_at) }) }}
      </p>
      <details v-if="preview.skipped.length">
        <summary class="cursor-pointer text-sm">{{ t('couponCampaign.skipped') }}</summary>
        <ul class="max-h-48 overflow-y-auto text-sm">
          <li v-for="v in preview.skipped" :key="v.user_id">
            #{{ v.user_id }} · {{ t('couponCampaign.reasons.' + v.reason) }}
          </li>
        </ul>
      </details>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="
          busy ||
          !preview.recipients.length ||
          (preview.remaining_stock !== null && preview.remaining_stock < preview.recipients.length)
        "
        @click="execute"
      >
        {{ t('couponCampaign.confirm', { count: preview.recipients.length }) }}
      </button>
    </div>
    <div v-if="result" role="status" class="space-y-2 text-sm">
      <p>
        {{
          t('couponCampaign.result', {
            batch: result.batch_id,
            issued: result.issued_count,
            skipped: result.skipped.length,
          })
        }}
      </p>
      <details v-if="result.skipped.length">
        <summary>{{ t('couponCampaign.skipped') }}</summary>
        <ul class="max-h-48 overflow-y-auto">
          <li v-for="v in result.skipped" :key="v.user_id">
            #{{ v.user_id }} · {{ t('couponCampaign.reasons.' + v.reason) }}
          </li>
        </ul>
      </details>
    </div>
    <p v-if="saved" role="status" class="text-sm text-emerald-700 dark:text-emerald-300">
      {{ t('common.success') }}
    </p>
    <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-300">{{ error }}</p>
  </section>
</template>
