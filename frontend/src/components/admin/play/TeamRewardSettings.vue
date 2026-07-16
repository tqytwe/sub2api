<template>
  <section class="card">
    <div class="flex flex-col gap-3 border-b border-gray-100 px-6 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ text('团队共享奖励', 'Team shared rewards') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ text('按月实际消费比例分配余额奖励', 'Monthly balance rewards split by actual spend') }}
        </p>
      </div>
      <Toggle v-if="settings" v-model="settings.enabled" />
    </div>

    <div v-if="loading && !settings" class="flex min-h-36 items-center justify-center">
      <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
    </div>
    <div v-else-if="settings" class="space-y-5 p-6">
      <div class="grid gap-4 sm:grid-cols-2">
        <label>
          <span class="input-label">{{ text('月度奖池上限', 'Monthly pool cap') }}</span>
          <input v-model="settings.cap" data-testid="team-reward-cap" type="number" min="0.00000001" step="0.01" class="input" />
        </label>
        <label>
          <span class="input-label">{{ text('开始月份', 'Start month') }}</span>
          <input v-model="settings.start_month" data-testid="team-reward-start-month" type="month" class="input" />
        </label>
      </div>

      <div class="overflow-x-auto rounded border border-gray-200 dark:border-dark-700">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3 text-left text-xs text-gray-500">{{ text('消费阈值', 'Spend threshold') }}</th>
              <th class="px-4 py-3 text-left text-xs text-gray-500">{{ text('返还比例', 'Reward rate') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="(tier, index) in settings.tiers" :key="index">
              <td class="px-4 py-3">
                <input v-model="tier.threshold" :data-testid="`team-threshold-${index}`" type="number" min="0.00000001" step="0.01" class="input" />
              </td>
              <td class="px-4 py-3">
                <input v-model="tier.rate" :data-testid="`team-rate-${index}`" type="number" min="0.00000001" max="1" step="0.01" class="input" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="flex justify-end">
        <button type="button" class="btn btn-primary inline-flex items-center gap-2" :disabled="saving" @click="save">
          <Icon :name="saving ? 'refresh' : 'check'" size="sm" :class="{ 'animate-spin': saving }" />
          {{ text('保存共享奖励', 'Save shared rewards') }}
        </button>
      </div>

      <div class="border-t border-gray-100 pt-5 dark:border-dark-700">
        <div class="mb-3 flex items-center justify-between">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ text('最近结算', 'Recent settlements') }}</h3>
          <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ text('刷新', 'Refresh') }}</button>
        </div>
        <p v-if="settlements.length === 0" class="text-sm text-gray-500">{{ text('暂无结算', 'No settlements') }}</p>
        <div v-else class="space-y-2">
          <div v-for="record in settlements" :key="record.settlement.id" class="flex flex-wrap items-center justify-between gap-3 rounded border border-gray-200 px-3 py-2 text-sm dark:border-dark-700">
            <span>#{{ record.settlement.id }} · {{ record.settlement.period_start.slice(0, 7) }} · ${{ Number(record.settlement.pool_amount).toFixed(2) }}</span>
            <div class="flex items-center gap-2">
              <span>{{ record.settlement.status }}</span>
              <button
                v-if="record.settlement.status !== 'completed'"
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="retrying === record.settlement.id"
                @click="retry(record.settlement.id)"
              >
                {{ text('重试', 'Retry') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import adminPlayAPI, { type TeamRewardSettings } from '@/api/admin/play'
import type { PlayTeamSettlementRecord } from '@/api/play'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { locale } = useI18n()
const appStore = useAppStore()
const settings = ref<TeamRewardSettings | null>(null)
const settlements = ref<PlayTeamSettlementRecord[]>([])
const loading = ref(false)
const saving = ref(false)
const retrying = ref<number | null>(null)
const isZh = computed(() => locale.value.startsWith('zh'))
const text = (zh: string, en: string) => (isZh.value ? zh : en)

async function load() {
  loading.value = true
  try {
    const [config, history] = await Promise.all([
      adminPlayAPI.getTeamRewardSettings(),
      adminPlayAPI.listTeamRewardSettlements(),
    ])
    settings.value = config
    settlements.value = history
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, text('加载团队奖励失败', 'Failed to load team rewards')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!settings.value || saving.value) return
  saving.value = true
  try {
    settings.value = await adminPlayAPI.updateTeamRewardSettings(settings.value)
    appStore.showSuccess(text('团队奖励已保存', 'Team rewards saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, text('保存失败', 'Save failed')))
  } finally {
    saving.value = false
  }
}

async function retry(id: number) {
  retrying.value = id
  try {
    await adminPlayAPI.retryTeamRewardSettlement(id)
    appStore.showSuccess(text('已执行重试', 'Retry completed'))
    await load()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, text('重试失败', 'Retry failed')))
  } finally {
    retrying.value = null
  }
}

onMounted(load)
</script>
