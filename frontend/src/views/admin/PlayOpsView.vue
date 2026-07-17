<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.playOps.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2 self-start" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('admin.playOps.refresh') }}
        </button>
      </div>

      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-6">
        <div v-for="card in statCards" :key="card.label" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
          <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ card.value }}</p>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_420px]">
        <div class="space-y-6">
          <section class="card">
            <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.arenaTitle') }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.playOps.arenaHint') }}</p>
              </div>
              <div class="inline-flex rounded border border-gray-200 p-1 text-sm dark:border-dark-700">
                <button type="button" class="rounded px-3 py-1" :class="arenaPeriodType === 'daily' ? activeTabClass : idleTabClass" @click="switchArena('daily')">
                  {{ t('admin.playOps.daily') }}
                </button>
                <button type="button" class="rounded px-3 py-1" :class="arenaPeriodType === 'monthly' ? activeTabClass : idleTabClass" @click="switchArena('monthly')">
                  {{ t('admin.playOps.monthly') }}
                </button>
              </div>
            </div>
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                  <tr>
                    <th class="px-4 py-3">{{ t('admin.playOps.rank') }}</th>
                    <th class="px-4 py-3">{{ t('admin.playOps.user') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.tokens') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.estimatedReward') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="row in arena?.rows || []" :key="row.user_id">
                    <td class="px-4 py-3 font-medium">#{{ row.rank }}</td>
                    <td class="px-4 py-3">
                      <div>{{ row.display_name }}</div>
                      <div v-if="row.email" class="text-xs text-gray-500">{{ row.email }}</div>
                    </td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ formatNumber(row.token_sum) }}</td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ formatMoney(row.estimated_reward) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="card">
            <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.teamsTitle') }}</h2>
              <div class="flex flex-col gap-2 sm:flex-row">
                <input v-model="query" type="search" class="input sm:w-72" :placeholder="t('admin.playOps.searchPlaceholder')" @keyup.enter="loadTeams" />
                <select v-model="status" class="input sm:w-32" @change="loadTeams">
                  <option value="active">{{ t('admin.playOps.statusActive') }}</option>
                  <option value="archived">{{ t('admin.playOps.statusArchived') }}</option>
                  <option value="all">{{ t('admin.playOps.statusAll') }}</option>
                </select>
              </div>
            </div>
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                  <tr>
                    <th class="px-4 py-3">{{ t('admin.playOps.team') }}</th>
                    <th class="px-4 py-3">{{ t('admin.playOps.captain') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.members') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.spend') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.pool') }}</th>
                    <th class="px-4 py-3"></th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="team in teams.items" :key="team.id">
                    <td class="px-4 py-3">
                      <div class="font-medium text-gray-900 dark:text-white">{{ team.name }}</div>
                      <div class="text-xs text-gray-500">{{ team.invite_code }}</div>
                    </td>
                    <td class="px-4 py-3">
                      <div>{{ team.captain_display_name }}</div>
                      <div class="text-xs text-gray-500">{{ team.captain_email }}</div>
                    </td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ team.member_count }}</td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ formatMoney(team.team_spend) }}</td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ formatMoney(team.estimated_pool) }}</td>
                    <td class="px-4 py-3 text-right">
                      <button type="button" class="btn btn-secondary btn-sm" @click="selectTeam(team.id)">
                        {{ t('admin.playOps.details') }}
                      </button>
                    </td>
                  </tr>
                  <tr v-if="!teams.items.length">
                    <td colspan="6" class="px-4 py-8 text-center text-gray-500">{{ loading ? t('admin.playOps.loading') : t('admin.playOps.noTeams') }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <aside class="card self-start">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.detailTitle') }}</h2>
          </div>
          <div v-if="selectedTeam" class="space-y-5 p-5">
            <div>
              <h3 class="text-base font-semibold">{{ selectedTeam.team.name }}</h3>
              <p class="text-sm text-gray-500">{{ t('admin.playOps.inviteCode') }}: {{ selectedTeam.team.invite_code }}</p>
            </div>
            <div>
              <h4 class="mb-2 text-sm font-semibold">{{ t('admin.playOps.memberContributions') }}</h4>
              <div class="space-y-2">
                <div v-for="member in selectedTeam.team.members" :key="member.user_id" class="rounded border border-gray-200 p-3 text-sm dark:border-dark-700">
                  <div class="flex justify-between gap-3">
                    <span class="min-w-0">
                      <span class="block truncate font-medium">{{ member.display_name }}</span>
                      <span v-if="member.email" class="block truncate text-xs text-gray-500">{{ member.email }}</span>
                    </span>
                    <span>{{ formatMoney(member.spend) }}</span>
                  </div>
                  <div class="mt-1 flex justify-between gap-3 text-xs text-gray-500">
                    <span>{{ t('admin.playOps.memberTokensLine', { tokens: formatNumber(member.token_sum), pct: member.spend_pct }) }}</span>
                    <span>{{ t('admin.playOps.memberEstimated') }} {{ formatMoney(member.estimated_reward) }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div>
              <h4 class="mb-2 text-sm font-semibold">{{ t('admin.playOps.settlements') }}</h4>
              <div v-if="!selectedTeam.settlements.length" class="text-sm text-gray-500">{{ t('admin.playOps.noSettlements') }}</div>
              <div v-for="record in selectedTeam.settlements" :key="record.settlement.id" class="mb-3 rounded border border-gray-200 p-3 text-sm dark:border-dark-700">
                <div class="flex justify-between gap-3 font-medium">
                  <span>{{ record.settlement.period_start.slice(0, 7) }}</span>
                  <span>{{ formatMoney(record.settlement.pool_amount) }} · {{ record.settlement.status }}</span>
                </div>
                <div v-for="allocation in record.allocations" :key="allocation.id" class="mt-2 flex justify-between gap-3 text-xs text-gray-500">
                  <span class="min-w-0">
                    <span class="block truncate">{{ allocation.display_name || `#${allocation.user_id}` }}</span>
                    <span v-if="allocation.email" class="block truncate">{{ allocation.email }}</span>
                  </span>
                  <span>{{ formatMoney(allocation.reward_amount) }} · {{ allocation.payout_status }}</span>
                </div>
              </div>
            </div>
          </div>
          <div v-else class="p-5 text-sm text-gray-500">{{ t('admin.playOps.noTeams') }}</div>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import adminPlayAPI, { type AdminArenaLeaderboard, type AdminPlayOpsSummary, type AdminPlayTeamDetail, type AdminPlayTeamList } from '@/api/admin/play'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const arenaPeriodType = ref<'daily' | 'monthly'>('monthly')
const arena = ref<AdminArenaLeaderboard | null>(null)
const teams = ref<AdminPlayTeamList>({ items: [], total: 0, page: 1, page_size: 20 })
const summary = ref<AdminPlayOpsSummary | null>(null)
const selectedTeam = ref<AdminPlayTeamDetail | null>(null)
const query = ref('')
const status = ref<'active' | 'archived' | 'all'>('active')

const activeTabClass = 'bg-primary-600 text-white'
const idleTabClass = 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'

const statCards = computed(() => {
  const arenaBudget = arenaPeriodType.value === 'daily'
    ? summary.value?.daily_arena_reward_budget
    : summary.value?.monthly_arena_reward_budget
  return [
    { label: t('admin.playOps.totalTeams'), value: formatNumber(summary.value?.total_teams) },
    { label: t('admin.playOps.activeTeams'), value: formatNumber(summary.value?.active_teams) },
    { label: t('admin.playOps.monthSpend'), value: formatMoney(summary.value?.month_spend) },
    { label: t('admin.playOps.estimatedPool'), value: formatMoney(summary.value?.estimated_shared_pool) },
    { label: t('admin.playOps.arenaBudget'), value: formatMoney(arenaBudget) },
    { label: t('admin.playOps.pendingFailedSettlements'), value: formatNumber(summary.value?.pending_failed_settlements) },
  ]
})

async function load() {
  loading.value = true
  try {
    const [summaryData, arenaData, teamData] = await Promise.all([
      adminPlayAPI.getSummary(),
      adminPlayAPI.getArenaLeaderboard({ period_type: arenaPeriodType.value, limit: 20 }),
      adminPlayAPI.listTeams({ status: status.value, q: query.value, page: 1, page_size: 50 }),
    ])
    summary.value = summaryData
    arena.value = arenaData
    teams.value = teamData
    if (!selectedTeam.value && teamData.items[0]) {
      await selectTeam(teamData.items[0].id)
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.playOps.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function loadTeams() {
  try {
    teams.value = await adminPlayAPI.listTeams({ status: status.value, q: query.value, page: 1, page_size: 50 })
    if (selectedTeam.value && !teams.value.items.some((team) => team.id === selectedTeam.value?.team.id)) {
      selectedTeam.value = null
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.playOps.loadFailed')))
  }
}

async function switchArena(periodType: 'daily' | 'monthly') {
  arenaPeriodType.value = periodType
  arena.value = await adminPlayAPI.getArenaLeaderboard({ period_type: periodType, limit: 20 })
}

async function selectTeam(id: number) {
  selectedTeam.value = await adminPlayAPI.getTeam(id)
}

function formatNumber(value: number | string | undefined) {
  return Number(value || 0).toLocaleString()
}

function formatMoney(value: number | string | undefined) {
  return `$${Number(value || 0).toFixed(2)}`
}

onMounted(load)
</script>
