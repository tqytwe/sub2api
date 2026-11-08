<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { AffiliateOperationsOverview } from '@/api/admin/affiliateOperations'
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)
const props = defineProps<{ rows: AffiliateOperationsOverview['trend'] }>()
const { t } = useI18n()
const data = computed(() => ({
  labels: props.rows.map(row => row.date),
  datasets: [
    // design-governance-allow: raw-color - Centralized canvas strokes for the two operational count series, with text labels and table alternatives.
    { label: t('admin.affiliateOperations.newInvites'), data: props.rows.map(row => row.invited_count), borderColor: 'rgb(59, 130, 246)', tension: .2 },
    // design-governance-allow: raw-color - Centralized canvas strokes for the two operational count series, with text labels and table alternatives.
    { label: t('admin.affiliateOperations.effectivePaid'), data: props.rows.map(row => row.effective_paid_count), borderColor: 'rgb(16, 185, 129)', tension: .2 },
  ],
}))
const options = { responsive: true, maintainAspectRatio: false, animation: false as const, scales: { y: { beginAtZero: true, ticks: { precision: 0 } } } }
</script>

<template>
  <div v-if="rows.length" class="space-y-3">
    <div class="h-64"><Line :data="data" :options="options" :aria-label="t('admin.affiliateOperations.dailyTrend')" /></div>
    <details class="text-sm"><summary class="cursor-pointer text-primary-600 dark:text-primary-400">{{ t('admin.affiliateOperations.trendTable') }}</summary>
      <div class="mt-3 overflow-x-auto"><table class="w-full text-sm"><thead><tr class="text-left"><th class="p-2">{{ t('admin.affiliateOperations.date') }}</th><th class="p-2">{{ t('admin.affiliateOperations.newInvites') }}</th><th class="p-2">{{ t('admin.affiliateOperations.effectivePaid') }}</th></tr></thead><tbody><tr v-for="row in rows" :key="row.date"><td class="p-2">{{ row.date }}</td><td class="p-2 tabular-nums">{{ row.invited_count }}</td><td class="p-2 tabular-nums">{{ row.effective_paid_count }}</td></tr></tbody></table></div>
    </details>
  </div>
  <p v-else class="py-8 text-center text-sm text-gray-500">{{ t('admin.affiliateOperations.noData') }}</p>
</template>
