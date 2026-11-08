<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import StatCard from '@/components/common/StatCard.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import AffiliateTrendChart from './AffiliateTrendChart.vue'
import operations, { type AffiliateOperationsOverview, type AffiliateOperationsInvite } from '@/api/admin/affiliateOperations'
import { useLeaderboardLiveRefresh } from '@/composables/useLeaderboardLiveRefresh'
import { formatDateTime } from '@/utils/format'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const currentMonth = () => new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit' }).format(new Date())
const month = ref(typeof route.query.month === 'string' && /^\d{4}-\d{2}$/.test(route.query.month) ? route.query.month : currentMonth())
const overview = ref<AffiliateOperationsOverview | null>(null)
const rankingLive = useLeaderboardLiveRefresh({ key: month, fetch: signal => operations.getLeaderboard({ month: month.value }, signal) })
const ranking = rankingLive.data
const exceptions = ref<AffiliateOperationsInvite[]>([])
const loading = ref(false)
const errors = ref({ overview: false, exceptions: false })
let requestSequence = 0
const money = (value: number) => new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD', maximumFractionDigits: 8 }).format(value)
const stats = computed(() => overview.value ? [
  { title: t('admin.affiliateOperations.newInvites'), value: overview.value.invited_count },
  { title: t('admin.affiliateOperations.effectivePaid'), value: overview.value.effective_paid_count },
  { title: t('admin.affiliateOperations.cohortConversion'), value: `${(overview.value.conversion_rate * 100).toFixed(1)}%` },
  { title: t('admin.affiliateOperations.pending'), value: overview.value.pending_count },
] : [])
const funds = computed(() => overview.value ? [
  ['available', overview.value.available_quota], ['frozen', overview.value.frozen_quota],
  ['transferred', overview.value.transferred_amount], ['withdrawn', overview.value.withdrawn_amount],
] as const : [])
const amountColumns = computed(() => [
  { key: 'source', label: t('admin.affiliateOperations.source') }, { key: 'currency', label: t('admin.affiliateOperations.currency') },
  { key: 'gross_earned', label: t('admin.affiliateOperations.grossEarned') }, { key: 'revoked_amount', label: t('admin.affiliateOperations.revoked') },
  { key: 'net_earned', label: t('admin.affiliateOperations.netEarned') },
])
const rankingColumns = computed(() => [
  { key: 'rank', label: t('admin.affiliateOperations.rank') }, { key: 'user', label: t('admin.affiliateOperations.promoter') },
  { key: 'effective_paid_count', label: t('admin.affiliateOperations.effectivePaid') },
])
const exceptionColumns = computed(() => [
  { key: 'invitee', label: t('admin.affiliateOperations.invitee') }, { key: 'reason', label: t('admin.affiliateOperations.reason') },
  { key: 'actions', label: t('admin.affiliateOperations.actions') },
])

async function load() {
  const seq = ++requestSequence
  loading.value = true
  errors.value = { overview: false, exceptions: false }
  // Clear stale month data; each failed section has its own retry state.
  overview.value = null
  exceptions.value = []
  await Promise.allSettled([
    operations.getOverview({ month: month.value }).then(value => { if (seq === requestSequence) overview.value = value }).catch(() => { if (seq === requestSequence) errors.value.overview = true }),
    operations.listExceptions({ page: 1, page_size: 5 }).then(value => { if (seq === requestSequence) exceptions.value = value.items }).catch(() => { if (seq === requestSequence) errors.value.exceptions = true }),
  ])
  if (seq === requestSequence) loading.value = false
}
function changeMonth() {
  if (!/^\d{4}-(0[1-9]|1[0-2])$/.test(month.value)) return
  void router.replace({ query: { ...route.query, month: month.value } })
}
watch(() => route.query.month, value => {
  month.value = typeof value === 'string' && /^\d{4}-(0[1-9]|1[0-2])$/.test(value) ? value : currentMonth()
  void load()
}, { immediate: true })
</script>

<template>
  <AppLayout>
    <div class="space-y-6" :aria-busy="loading">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div><h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.affiliateOperations.title') }}</h2><p class="mt-1 text-sm text-gray-500">{{ t('admin.affiliateOperations.cohortExplanation') }}</p></div>
        <div class="flex items-end gap-2"><label class="text-sm"><span class="mb-1 block">{{ t('admin.affiliateOperations.month') }}</span><input v-model="month" type="month" class="input" :max="currentMonth()" @change="changeMonth" /></label><button type="button" class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="sm" />{{ t('common.refresh') }}</button></div>
      </div>
      <div v-if="errors.overview" class="text-sm text-red-600" role="alert">{{ t('admin.affiliateOperations.loadFailed') }} <button class="btn btn-secondary" type="button" @click="load">{{ t('common.retry') }}</button></div>
      <p v-else-if="loading && !overview" role="status">{{ t('common.loading') }}</p>
      <template v-if="overview">
        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" data-test="overview-stats"><StatCard v-for="stat in stats" :key="stat.title" :title="stat.title" :value="stat.value" /></div>
        <section class="space-y-3"><h3 class="text-base font-semibold">{{ t('admin.affiliateOperations.allTimeFunds') }}</h3><p class="text-xs text-gray-500">{{ t('admin.affiliateOperations.fundScope') }}</p><dl class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4"><div v-for="[key, amount] in funds" :key="key" class="border-b border-gray-200 py-3 dark:border-dark-700"><dt class="text-xs text-gray-500">{{ t(`admin.affiliateOperations.${key}`) }}</dt><dd class="mt-1 font-semibold tabular-nums">{{ money(amount) }}</dd></div></dl></section>
        <section v-for="scope in (['period', 'all'] as const)" :key="scope" class="space-y-3"><h3 class="text-base font-semibold">{{ t(`admin.affiliateOperations.${scope === 'period' ? 'monthEarnings' : 'allEarnings'}`) }}</h3><DataTable :columns="amountColumns" :data="scope === 'period' ? overview.period_amounts || [] : overview.amounts || []"><template #cell-source="{ row }">{{ t(`admin.affiliateOperations.sources.${row.source_type}`) }}</template><template v-for="key in ['gross_earned','revoked_amount','net_earned']" :key="key" #[`cell-${key}`]="{ row }"><span class="tabular-nums">{{ new Intl.NumberFormat(locale, { maximumFractionDigits: 8 }).format(row[key]) }}</span></template></DataTable></section>
        <section class="space-y-3"><h3 class="text-base font-semibold">{{ t('admin.affiliateOperations.dailyTrend') }}</h3><AffiliateTrendChart :rows="overview.trend" /></section>
      </template>
      <section class="space-y-3">
        <div><h3 class="text-base font-semibold">{{ t('admin.affiliateOperations.monthlyTop10') }}</h3><p class="mt-1 text-xs text-gray-500">{{ t('admin.affiliateOperations.honorOnly') }}</p></div>
        <p class="text-xs text-gray-500">{{ t('admin.affiliateOperations.liveRanking') }} <template v-if="ranking">{{ t('admin.affiliateOperations.updatedAt') }} {{ formatDateTime(ranking.updated_at) }}</template></p>
        <div v-if="rankingLive.error.value" data-test="ranking-error" class="text-sm text-red-600" role="alert">{{ t('admin.affiliateOperations.loadFailed') }} <button type="button" class="btn btn-secondary" @click="rankingLive.refresh">{{ t('common.retry') }}</button></div>
        <DataTable :columns="rankingColumns" :data="ranking?.items || []" :loading="!ranking && rankingLive.isRefreshing.value">
          <template #cell-user="{ row }"><RouterLink :data-test="`ranking-user-${row.user_id}`" :to="{ path: '/admin/affiliates/invites', query: { mode: 'promoters', user_id: row.user_id, month } }" class="text-primary-600 hover:underline dark:text-primary-400">{{ row.email || row.username || `#${row.user_id}` }}</RouterLink></template>
        </DataTable>
      </section>
      <section class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2"><h3 class="text-base font-semibold">{{ t('admin.affiliateOperations.exceptions') }} <span v-if="overview" class="text-gray-500">({{ overview.exceptions_count }})</span></h3><RouterLink :to="{ path: '/admin/affiliates/invites', query: { mode: 'promoters', exceptions: '1' } }" class="text-sm text-primary-600 hover:underline dark:text-primary-400">{{ t('admin.affiliateOperations.viewPending') }}</RouterLink></div>
        <div v-if="errors.exceptions" role="alert" class="text-sm text-red-600">{{ t('admin.affiliateOperations.loadFailed') }} <button type="button" class="btn btn-secondary" @click="load">{{ t('common.retry') }}</button></div>
        <DataTable v-else :columns="exceptionColumns" :data="exceptions" :loading="loading"><template #cell-invitee="{ row }">{{ row.invitee_email || `#${row.invitee_id}` }}</template><template #cell-reason="{ row }">{{ t(`admin.affiliateOperations.reasons.${row.reason}`) }}</template><template #cell-actions="{ row }"><RouterLink :data-test="`exception-user-${row.invitee_id}`" :to="{ path: '/admin/affiliates/invites', query: { mode: 'promoters', user_id: row.inviter_id, invitee_id: row.invitee_id, detail: 'invites' } }" class="text-primary-600 hover:underline dark:text-primary-400">{{ t('admin.affiliateOperations.inspect') }}</RouterLink></template></DataTable>
      </section>
      <p v-if="overview" class="text-xs text-gray-500">{{ t('admin.affiliateOperations.updatedAt') }} {{ formatDateTime(overview.updated_at) }}</p>
    </div>
  </AppLayout>
</template>
