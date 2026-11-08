<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import operations, { type AffiliatePromoter, type AffiliateOperationsInvite } from '@/api/admin/affiliateOperations'
const emit = defineEmits<{ open: [id: number, inviteeId?: number] }>()
const { t } = useI18n()
const route = useRoute()
const search = ref('')
const userId = ref(String(route.query.user_id || ''))
const onlyExceptions = ref(route.query.exceptions === '1')
const status = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<Array<AffiliatePromoter | AffiliateOperationsInvite>>([])
const loading = ref(false)
const error = ref(false)
let sequence = 0
const columns = computed(() => onlyExceptions.value ? [
  { key: 'user', label: t('admin.affiliateOperations.promoter') }, { key: 'invitee_email', label: t('admin.affiliateOperations.invitee') }, { key: 'reason', label: t('admin.affiliateOperations.reason') },
] : [
  { key: 'user', label: t('admin.affiliateOperations.promoter') }, { key: 'aff_code', label: t('admin.affiliateOperations.inviteCode') },
  { key: 'invited_count', label: t('admin.affiliateOperations.invited') }, { key: 'effective_paid_count', label: t('admin.affiliateOperations.effectivePaidTotal') },
  { key: 'available_quota', label: t('admin.affiliateOperations.available') }, { key: 'frozen_quota', label: t('admin.affiliateOperations.frozen') },
])
const number = (value: number) => new Intl.NumberFormat(undefined, { maximumFractionDigits: 8 }).format(value)
async function load() {
  const seq = ++sequence
  loading.value = true; error.value = false
  const id = Number(userId.value)
  const params = { page: page.value, page_size: pageSize.value, search: search.value.trim() || undefined, user_id: Number.isSafeInteger(id) && id > 0 ? id : undefined }
  try {
    const result = onlyExceptions.value ? await operations.listExceptions({ ...params, status: status.value || undefined }) : await operations.listPromoters(params)
    if (seq !== sequence) return
    rows.value = result.items; total.value = result.total
  } catch { if (seq === sequence) { error.value = true; rows.value = []; total.value = 0 } }
  finally { if (seq === sequence) loading.value = false }
}
function filter() { page.value = 1; void load() }
onMounted(load)
watch(() => [route.query.user_id, route.query.exceptions], () => {
  userId.value = String(route.query.user_id || '')
  onlyExceptions.value = route.query.exceptions === '1'
  filter()
})
</script>

<template>
  <section class="space-y-4" :aria-busy="loading">
    <form class="flex flex-wrap items-end gap-3" @submit.prevent="filter"><label class="text-sm">{{ t('admin.affiliateOperations.searchPromoter') }}<input v-model="search" class="input mt-1" type="search" /></label><label class="text-sm">{{ t('admin.affiliateOperations.exactUserId') }}<input v-model="userId" type="number" min="1" step="1" class="input mt-1 w-full sm:w-36" /></label><label class="flex items-center gap-2 text-sm"><input v-model="onlyExceptions" type="checkbox" @change="filter" />{{ t('admin.affiliateOperations.onlyExceptions') }}</label><select v-if="onlyExceptions" v-model="status" class="input w-full sm:w-44" :aria-label="t('admin.affiliateOperations.status')" @change="filter"><option value="">{{ t('admin.affiliateOperations.allStatuses') }}</option><option v-for="item in ['pending_evidence', 'refunded', 'risk_excluded', 'disabled']" :key="item" :value="item">{{ t(`admin.affiliateOperations.reasons.${item}`) }}</option></select><button type="submit" class="btn btn-secondary" :disabled="loading"><Icon name="search" size="sm" />{{ t('common.search') }}</button></form>
    <p class="text-xs text-gray-500">{{ t('admin.affiliateOperations.promoterScope') }}</p>
    <div v-if="error" role="alert" class="text-sm text-red-600">{{ t('admin.affiliateOperations.loadFailed') }} <button type="button" class="btn btn-secondary" @click="load">{{ t('common.retry') }}</button></div>
    <DataTable :columns="columns" :data="rows" :loading="loading"><template #cell-user="{ row }"><button type="button" class="text-primary-600 hover:underline dark:text-primary-400" @click="emit('open', row.inviter_id || row.user_id, onlyExceptions ? row.invitee_id : undefined)">{{ row.email || row.inviter_email || `#${row.user_id || row.inviter_id}` }}</button></template><template #cell-reason="{ row }">{{ t(`admin.affiliateOperations.reasons.${row.reason}`) }}</template><template #cell-available_quota="{ row }"><span class="tabular-nums">USD {{ number(row.available_quota) }}</span></template><template #cell-frozen_quota="{ row }"><span class="tabular-nums">USD {{ number(row.frozen_quota) }}</span></template></DataTable>
    <Pagination v-if="total > 0" :page="page" :page-size="pageSize" :total="total" @update:page="page = $event; load()" @update:pageSize="pageSize = $event; page = 1; load()" />
  </section>
</template>
