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
            <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.campaignsTitle') }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignsHint') }}</p>
              </div>
              <button type="button" class="btn btn-primary inline-flex items-center gap-2 self-start" data-testid="new-campaign" @click="startCreateCampaign">
                <Icon name="plus" size="sm" />
                {{ t('admin.playOps.newCampaign') }}
              </button>
            </div>

            <form v-if="campaignFormOpen" class="border-b border-gray-100 bg-gray-50/60 px-5 py-4 dark:border-dark-700 dark:bg-dark-800/40" data-testid="campaign-form" @submit.prevent="submitCampaign">
              <div class="grid gap-4 lg:grid-cols-2">
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignName') }}</span>
                  <input v-model="campaignForm.name" class="input" data-testid="campaign-name" required maxlength="128" />
                </label>
                <label class="flex items-center gap-3 rounded border border-gray-200 bg-white px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-900">
                  <input v-model="campaignForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                  <span class="font-medium text-gray-700 dark:text-gray-200">{{ t('admin.playOps.campaignEnabled') }}</span>
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignNameZh') }}</span>
                  <input v-model="campaignForm.nameZh" class="input" maxlength="128" />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignNameEn') }}</span>
                  <input v-model="campaignForm.nameEn" class="input" maxlength="128" />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignStartAt') }}</span>
                  <input v-model="campaignForm.startAt" type="datetime-local" class="input" data-testid="campaign-start" required />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignEndAt') }}</span>
                  <input v-model="campaignForm.endAt" type="datetime-local" class="input" data-testid="campaign-end" required />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.rechargeBonusPct') }}</span>
                  <input v-model="campaignForm.rechargeBonusPct" type="number" min="0" max="1000" step="0.01" class="input" data-testid="campaign-recharge-bonus" />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.blindboxExtraOpens') }}</span>
                  <input v-model="campaignForm.blindboxExtraOpens" type="number" min="0" max="100" step="1" class="input" data-testid="campaign-blindbox-extra" />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.arenaScoreMultiplier') }}</span>
                  <input v-model="campaignForm.arenaScoreMultiplier" type="number" min="0" max="100" step="0.01" class="input" data-testid="campaign-arena-multiplier" :placeholder="t('admin.playOps.arenaMultiplierPlaceholder')" />
                </label>
              </div>
              <div class="mt-4 flex flex-col gap-2 sm:flex-row sm:justify-end">
                <button type="button" class="btn btn-secondary inline-flex items-center justify-center gap-2" :disabled="campaignSaving" @click="closeCampaignForm">
                  <Icon name="x" size="sm" />
                  {{ t('admin.playOps.cancel') }}
                </button>
                <button type="submit" class="btn btn-primary inline-flex items-center justify-center gap-2" data-testid="save-campaign" :disabled="campaignSaving">
                  <Icon name="save" size="sm" />
                  {{ campaignSaving ? t('admin.playOps.saving') : t('admin.playOps.saveCampaign') }}
                </button>
              </div>
            </form>

            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                  <tr>
                    <th class="px-4 py-3">{{ t('admin.playOps.campaign') }}</th>
                    <th class="px-4 py-3">{{ t('admin.playOps.campaignWindow') }}</th>
                    <th class="px-4 py-3">{{ t('admin.playOps.campaignRules') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.actions') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="campaign in campaigns" :key="campaign.id">
                    <td class="px-4 py-3 align-top">
                      <div class="flex flex-wrap items-center gap-2">
                        <span class="font-medium text-gray-900 dark:text-white">{{ campaign.name }}</span>
                        <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="campaignStatusClass(campaign)">
                          {{ t(`admin.playOps.campaignStatus.${campaignStatus(campaign)}`) }}
                        </span>
                      </div>
                      <div class="mt-1 text-xs text-gray-500">#{{ campaign.id }}</div>
                    </td>
                    <td class="px-4 py-3 align-top">
                      <div class="tabular-nums">{{ formatDateTime(campaign.start_at) }}</div>
                      <div class="mt-1 text-xs text-gray-500">{{ formatDateTime(campaign.end_at) }}</div>
                    </td>
                    <td class="px-4 py-3 align-top">
                      <div v-if="campaignRuleLines(campaign).length" class="space-y-1">
                        <div v-for="line in campaignRuleLines(campaign)" :key="line" class="text-gray-700 dark:text-gray-200">{{ line }}</div>
                      </div>
                      <span v-else class="text-gray-500">{{ t('admin.playOps.noCampaignRules') }}</span>
                    </td>
                    <td class="px-4 py-3 text-right align-top">
                      <div class="inline-flex items-center gap-2">
                        <button type="button" class="btn btn-secondary btn-sm inline-flex items-center gap-1" @click="startEditCampaign(campaign)">
                          <Icon name="edit" size="xs" />
                          {{ t('admin.playOps.edit') }}
                        </button>
                        <button type="button" class="btn btn-danger btn-sm inline-flex items-center gap-1" :disabled="campaignDeletingId === campaign.id" @click="deleteCampaign(campaign)">
                          <Icon name="trash" size="xs" />
                          {{ t('admin.playOps.delete') }}
                        </button>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="!campaigns.length">
                    <td colspan="4" class="px-4 py-8 text-center text-gray-500">{{ loading ? t('admin.playOps.loading') : t('admin.playOps.noCampaigns') }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

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
import adminPlayAPI, { type AdminArenaLeaderboard, type AdminPlayCampaign, type AdminPlayCampaignInput, type AdminPlayOpsSummary, type AdminPlayTeamDetail, type AdminPlayTeamList } from '@/api/admin/play'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

interface CampaignFormState {
  id?: number
  name: string
  nameZh: string
  nameEn: string
  startAt: string
  endAt: string
  enabled: boolean
  rechargeBonusPct: string
  blindboxExtraOpens: string
  arenaScoreMultiplier: string
}

const loading = ref(false)
const campaignSaving = ref(false)
const campaignDeletingId = ref<number | null>(null)
const campaignFormOpen = ref(false)
const arenaPeriodType = ref<'daily' | 'monthly'>('monthly')
const arena = ref<AdminArenaLeaderboard | null>(null)
const campaigns = ref<AdminPlayCampaign[]>([])
const teams = ref<AdminPlayTeamList>({ items: [], total: 0, page: 1, page_size: 20 })
const summary = ref<AdminPlayOpsSummary | null>(null)
const selectedTeam = ref<AdminPlayTeamDetail | null>(null)
const query = ref('')
const status = ref<'active' | 'archived' | 'all'>('active')
const campaignForm = ref<CampaignFormState>(blankCampaignForm())

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
    const [summaryData, arenaData, campaignData, teamData] = await Promise.all([
      adminPlayAPI.getSummary(),
      adminPlayAPI.getArenaLeaderboard({ period_type: arenaPeriodType.value, limit: 20 }),
      adminPlayAPI.listCampaigns(),
      adminPlayAPI.listTeams({ status: status.value, q: query.value, page: 1, page_size: 50 }),
    ])
    summary.value = summaryData
    arena.value = arenaData
    campaigns.value = campaignData
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

function blankCampaignForm(): CampaignFormState {
  const start = new Date()
  start.setMinutes(0, 0, 0)
  const end = new Date(start)
  end.setDate(end.getDate() + 7)
  return {
    name: '',
    nameZh: '',
    nameEn: '',
    startAt: toDateTimeLocal(start.toISOString()),
    endAt: toDateTimeLocal(end.toISOString()),
    enabled: true,
    rechargeBonusPct: '',
    blindboxExtraOpens: '',
    arenaScoreMultiplier: '',
  }
}

function startCreateCampaign() {
  campaignForm.value = blankCampaignForm()
  campaignFormOpen.value = true
}

function startEditCampaign(campaign: AdminPlayCampaign) {
  campaignForm.value = {
    id: campaign.id,
    name: campaign.name,
    nameZh: campaign.rules.name_i18n?.zh || '',
    nameEn: campaign.rules.name_i18n?.en || '',
    startAt: toDateTimeLocal(campaign.start_at),
    endAt: toDateTimeLocal(campaign.end_at),
    enabled: campaign.enabled,
    rechargeBonusPct: campaign.rules.recharge_bonus_pct ? String(campaign.rules.recharge_bonus_pct) : '',
    blindboxExtraOpens: campaign.rules.blindbox_extra_opens ? String(campaign.rules.blindbox_extra_opens) : '',
    arenaScoreMultiplier: campaign.rules.arena_score_multiplier ? String(campaign.rules.arena_score_multiplier) : '',
  }
  campaignFormOpen.value = true
}

function closeCampaignForm() {
  campaignFormOpen.value = false
}

async function submitCampaign() {
  let input: AdminPlayCampaignInput
  try {
    input = buildCampaignInput(campaignForm.value)
  } catch (error) {
    appStore.showError((error as Error).message)
    return
  }

  campaignSaving.value = true
  try {
    if (campaignForm.value.id) {
      await adminPlayAPI.updateCampaign(campaignForm.value.id, input)
      appStore.showSuccess(t('admin.playOps.campaignUpdated'))
    } else {
      await adminPlayAPI.createCampaign(input)
      appStore.showSuccess(t('admin.playOps.campaignCreated'))
    }
    campaignFormOpen.value = false
    campaigns.value = await adminPlayAPI.listCampaigns()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.playOps.campaignSaveFailed')))
  } finally {
    campaignSaving.value = false
  }
}

async function deleteCampaign(campaign: AdminPlayCampaign) {
  if (!window.confirm(t('admin.playOps.deleteCampaignConfirm', { name: campaign.name }))) {
    return
  }
  campaignDeletingId.value = campaign.id
  try {
    await adminPlayAPI.deleteCampaign(campaign.id)
    campaigns.value = campaigns.value.filter((item) => item.id !== campaign.id)
    appStore.showSuccess(t('admin.playOps.campaignDeleted'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.playOps.campaignDeleteFailed')))
  } finally {
    campaignDeletingId.value = null
  }
}

function buildCampaignInput(form: CampaignFormState): AdminPlayCampaignInput {
  const start = new Date(form.startAt)
  const end = new Date(form.endAt)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
    throw new Error(t('admin.playOps.campaignTimeRequired'))
  }
  if (end <= start) {
    throw new Error(t('admin.playOps.campaignTimeInvalid'))
  }

  const nameI18n: Record<string, string> = {}
  if (form.nameZh.trim()) nameI18n.zh = form.nameZh.trim()
  if (form.nameEn.trim()) nameI18n.en = form.nameEn.trim()

  const rules = {
    recharge_bonus_pct: parseOptionalNumber(form.rechargeBonusPct),
    blindbox_extra_opens: parseOptionalInteger(form.blindboxExtraOpens),
    arena_score_multiplier: parseOptionalNumber(form.arenaScoreMultiplier),
    name_i18n: Object.keys(nameI18n).length ? nameI18n : undefined,
  }

  return {
    name: form.name.trim(),
    start_at: start.toISOString(),
    end_at: end.toISOString(),
    enabled: form.enabled,
    rules,
  }
}

function parseOptionalNumber(value: string | number | undefined): number | undefined {
  const raw = String(value ?? '').trim()
  if (!raw) return undefined
  const numberValue = Number(raw)
  return Number.isFinite(numberValue) ? numberValue : undefined
}

function parseOptionalInteger(value: string | number | undefined): number | undefined {
  const numberValue = parseOptionalNumber(value)
  return numberValue === undefined ? undefined : Math.trunc(numberValue)
}

function campaignStatus(campaign: AdminPlayCampaign): 'active' | 'upcoming' | 'ended' | 'disabled' {
  if (!campaign.enabled) return 'disabled'
  const now = Date.now()
  const start = new Date(campaign.start_at).getTime()
  const end = new Date(campaign.end_at).getTime()
  if (now < start) return 'upcoming'
  if (now >= end) return 'ended'
  return 'active'
}

function campaignStatusClass(campaign: AdminPlayCampaign) {
  const status = campaignStatus(campaign)
  if (status === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (status === 'upcoming') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
  if (status === 'ended') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
}

function campaignRuleLines(campaign: AdminPlayCampaign) {
  const rules = campaign.rules || {}
  const lines: string[] = []
  if (rules.recharge_bonus_pct) lines.push(t('admin.playOps.ruleRechargeBonus', { pct: rules.recharge_bonus_pct }))
  if (rules.blindbox_extra_opens) lines.push(t('admin.playOps.ruleBlindboxExtra', { count: rules.blindbox_extra_opens }))
  if (rules.arena_score_multiplier) lines.push(t('admin.playOps.ruleArenaMultiplier', { mult: rules.arena_score_multiplier }))
  return lines
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

function formatDateTime(value: string | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function toDateTimeLocal(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const offsetMs = date.getTimezoneOffset() * 60 * 1000
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16)
}

onMounted(load)
</script>
