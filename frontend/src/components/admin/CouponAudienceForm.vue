<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { CouponAudience } from '@/types/couponCampaign'
import type { AdminUser, UserAttributeDefinition } from '@/types'
const props = defineProps<{ modelValue: CouponAudience; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: CouponAudience] }>()
const { t } = useI18n()
const search = ref(''),
  idText = ref((props.modelValue.user_ids || []).join(', ')),
  inputError = ref(''),
  searching = ref(false)
const results = ref<AdminUser[]>([]),
  definitions = ref<UserAttributeDefinition[]>([])
let searchRequest = 0
onBeforeUnmount(() => {
  searchRequest++
})
function update(p: Partial<CouponAudience>) {
  emit('update:modelValue', { ...props.modelValue, ...p })
}
function mode(value: CouponAudience['mode']) {
  searchRequest++
  searching.value = false
  idText.value = ''
  results.value = []
  inputError.value = ''
  emit('update:modelValue', { mode: value })
}
function parseIds() {
  const chunks = idText.value
    .trim()
    .split(/[\s,，]+/)
    .filter(Boolean)
  if (chunks.some((x) => !/^\d+$/.test(x) || !Number.isSafeInteger(Number(x)) || Number(x) <= 0)) {
    inputError.value = t('couponCampaign.invalidIds')
    update({ user_ids: [] })
    return
  }
  inputError.value = ''
  update({ user_ids: [...new Set(chunks.map(Number))] })
}
async function find() {
  const request = ++searchRequest
  searching.value = true
  inputError.value = ''
  try {
    const page = await adminAPI.users.list(1, 20, {
      status: 'active',
      role: 'user',
      search: search.value,
      include_subscriptions: false,
    })
    if (request === searchRequest) results.value = page.items
  } catch {
    if (request === searchRequest) inputError.value = t('couponCampaign.errors.unknown')
  } finally {
    if (request === searchRequest) searching.value = false
  }
}
function select(id: number) {
  inputError.value = ''
  const ids = [...new Set([...(props.modelValue.user_ids || []), id])]
  idText.value = ids.join(', ')
  update({ user_ids: ids })
}
function updateAttribute(id: number, value: string) {
  const attrs = { ...props.modelValue.attributes }
  if (value) attrs[id] = value
  else delete attrs[id]
  update({ attributes: attrs })
}
watch(
  () => props.modelValue.user_ids,
  (ids) => {
    if (!inputError.value) idText.value = (ids || []).join(', ')
  },
)
async function attributes() {
  try {
    definitions.value = await adminAPI.userAttributes.listEnabledDefinitions()
  } catch {
    inputError.value = t('couponCampaign.errors.unknown')
  }
}
</script>
<template>
  <fieldset class="space-y-3" :disabled="disabled">
    <legend class="input-label">{{ t('couponCampaign.audience') }}</legend>
    <label class="grid gap-1 text-sm"
      ><span class="sr-only">{{ t('couponCampaign.audience') }}</span
      ><select
        class="input"
        :value="modelValue.mode"
        @change="mode(($event.target as HTMLSelectElement).value as CouponAudience['mode'])"
      >
        <option value="users">{{ t('couponCampaign.users') }}</option>
        <option value="all">{{ t('couponCampaign.all') }}</option>
        <option value="filtered">{{ t('couponCampaign.filtered') }}</option>
      </select></label
    >
    <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('couponCampaign.audienceHint') }}</p>
    <template v-if="modelValue.mode === 'users'">
      <label class="grid gap-1 text-sm"
        ><span class="input-label">{{ t('couponCampaign.ids') }}</span
        ><input v-model="idText" class="input" :aria-invalid="!!inputError" @input="parseIds"
      /></label>
      <div class="flex flex-wrap gap-2">
        <input
          v-model="search"
          class="input min-w-0 flex-1"
          :aria-label="t('couponCampaign.search')"
          :placeholder="t('couponCampaign.search')"
        /><button type="button" class="btn btn-secondary" :disabled="searching" @click="find">
          {{ t('common.search') }}
        </button>
      </div>
      <ul v-if="results.length" class="divide-y divide-gray-100 dark:divide-dark-700">
        <li
          v-for="u in results"
          :key="u.id"
          class="flex flex-wrap items-center justify-between gap-2 py-2 text-sm"
        >
          <span class="break-all">{{ u.email }} · #{{ u.id }}</span
          ><button type="button" class="btn btn-secondary btn-sm" @click="select(u.id)">
            {{ t('common.add') }}
          </button>
        </li>
      </ul>
    </template>
    <div v-if="modelValue.mode === 'filtered'" class="grid gap-3 sm:grid-cols-2">
      <label class="grid gap-1 text-sm"
        ><span class="input-label">{{ t('couponCampaign.search') }}</span
        ><input
          :value="modelValue.search"
          class="input"
          @input="update({ search: ($event.target as HTMLInputElement).value })"
      /></label>
      <label class="grid gap-1 text-sm"
        ><span class="input-label">{{ t('couponCampaign.group') }}</span
        ><input
          :value="modelValue.group_name"
          class="input"
          @input="update({ group_name: ($event.target as HTMLInputElement).value })"
      /></label>
      <label class="grid gap-1 text-sm"
        ><span class="input-label">{{ t('couponCampaign.keyGroup') }}</span
        ><input
          :value="modelValue.api_key_group_id"
          type="number"
          min="1"
          class="input"
          @input="
            update({ api_key_group_id: Number(($event.target as HTMLInputElement).value) || undefined })
          "
      /></label>
      <label class="grid gap-1 text-sm"
        ><span class="input-label">{{ t('couponCampaign.vip') }}</span
        ><input
          :value="modelValue.vip_tier"
          type="number"
          min="0"
          class="input"
          @input="
            update({
              vip_tier:
                ($event.target as HTMLInputElement).value === ''
                  ? undefined
                  : Number(($event.target as HTMLInputElement).value),
            })
          "
      /></label>
      <button type="button" class="btn btn-secondary self-end" @click="attributes">
        {{ t('couponCampaign.attributes') }}
      </button>
      <label v-for="a in definitions" :key="a.id" class="grid gap-1 text-sm"
        ><span class="input-label">{{ a.name }}</span
        ><input
          :value="modelValue.attributes?.[a.id]"
          class="input"
          @input="updateAttribute(a.id, ($event.target as HTMLInputElement).value)"
      /></label>
    </div>
    <p v-if="inputError" role="alert" class="text-sm text-red-600 dark:text-red-300">{{ inputError }}</p>
  </fieldset>
</template>
