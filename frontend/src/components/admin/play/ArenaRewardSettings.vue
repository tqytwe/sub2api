<template>
  <section class="card">
    <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ text('农场奖励规则', 'Farm reward rules') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ text('按名次区间设置单人奖励；修改仅用于后续尚未结算的周期。', 'Set the per-user reward for each rank range. Changes apply only to future, unsettled periods.') }}</p>
      </div>
      <div class="inline-flex rounded border border-gray-200 p-1 text-sm dark:border-dark-700">
        <button type="button" class="rounded px-3 py-1" :class="mode === 'monthly' ? activeClass : idleClass" @click="mode = 'monthly'">{{ text('月榜', 'Monthly') }}</button>
        <button type="button" class="rounded px-3 py-1" :class="mode === 'daily' ? activeClass : idleClass" @click="mode = 'daily'">{{ text('日榜', 'Daily') }}</button>
      </div>
    </div>
    <!-- design-governance-allow: continuous-motion - this transient loading indicator is removed when the settings request settles. -->
    <div v-if="loading && !settings" class="flex min-h-36 items-center justify-center"><Icon name="refresh" size="lg" class="animate-spin text-gray-400" /></div>
    <div v-else-if="settings" class="space-y-4 p-5">
      <label v-if="mode === 'daily'" class="block max-w-sm">
        <span class="input-label">{{ text('日榜最高发放预算', 'Daily maximum payout budget') }}</span>
        <input v-model="settings.daily_budget" type="number" min="0.01" step="0.01" class="input" />
      </label>
      <div class="overflow-x-auto rounded border border-gray-200 dark:border-dark-700">
        <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">{{ text('名次截至', 'Rank through') }}</th><th class="px-4 py-3">{{ text('本档覆盖人数', 'People in tier') }}</th><th class="px-4 py-3">{{ text('单人奖励', 'Reward per user') }}</th><th class="w-16 px-4 py-3"><span class="sr-only">{{ text('操作', 'Actions') }}</span></th></tr></thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="(tier, index) in activeTiers" :key="index"><td class="px-4 py-3"><input v-model.number="tier.rank_max" type="number" min="1" step="1" class="input" /></td><td class="px-4 py-3 tabular-nums">{{ tierWidth(index) }}</td><td class="px-4 py-3"><input v-model.number="tier.amount" type="number" min="0.00000001" step="0.01" class="input" /></td><td class="px-4 py-3 text-right"><button type="button" class="inline-flex h-9 w-9 items-center justify-center rounded text-gray-400 hover:bg-red-50 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-red-950/30 dark:hover:text-red-400" :disabled="activeTiers.length <= 1 || saving" :title="text('删除档位', 'Remove tier')" @click="activeTiers.splice(index, 1)"><Icon name="trash" size="sm" /></button></td></tr>
          </tbody>
        </table>
      </div>
      <div class="grid gap-3 rounded border border-primary-100 bg-primary-50 p-3 text-sm text-primary-900 dark:border-primary-900/60 dark:bg-primary-950/30 dark:text-primary-100 sm:grid-cols-3"><p>{{ text('奖励名次', 'Rewarded ranks') }} <strong>{{ maximumRank }}</strong></p><p>{{ text('理论最高发放', 'Maximum payout') }} <strong>{{ formatMoney(totalBudget) }}</strong></p><p v-if="mode === 'daily'">{{ text('日预算上限', 'Daily budget cap') }} <strong>{{ formatMoney(settings.daily_budget) }}</strong></p></div>
      <p v-if="validationMessage" role="alert" class="rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">{{ validationMessage }}</p>
      <div class="flex justify-between gap-3"><button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="activeTiers.length >= 32 || saving" @click="addTier"><Icon name="plus" size="sm" />{{ text('新增档位', 'Add tier') }}</button><!-- design-governance-allow: continuous-motion - save progress spinner is temporary and stops after the request resolves. --><button type="button" class="btn btn-primary inline-flex items-center gap-2" :disabled="Boolean(validationMessage) || saving" @click="save"><Icon :name="saving ? 'refresh' : 'check'" size="sm" :class="{ 'animate-spin': saving }" />{{ text('保存奖励规则', 'Save reward rules') }}</button></div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import adminPlayAPI, { type AdminArenaRewardSettings, type AdminArenaRewardTier } from '@/api/admin/play'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { locale } = useI18n()
const appStore = useAppStore()
const settings = ref<AdminArenaRewardSettings | null>(null)
const loading = ref(false)
const saving = ref(false)
const mode = ref<'daily' | 'monthly'>('monthly')
const isZh = computed(() => locale.value.startsWith('zh'))
const text = (zh: string, en: string) => isZh.value ? zh : en
const activeClass = 'bg-primary-600 text-white'
const idleClass = 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'
const activeTiers = computed(() => mode.value === 'monthly' ? settings.value?.monthly ?? [] : settings.value?.daily ?? [])
const maximumRank = computed(() => activeTiers.value.at(-1)?.rank_max ?? 0)
const totalBudget = computed(() => activeTiers.value.reduce((total, tier, index) => total + Math.max(0, tier.rank_max - (index === 0 ? 0 : activeTiers.value[index - 1].rank_max)) * Number(tier.amount || 0), 0))
const validationMessage = computed(() => validate())

function clone(value: AdminArenaRewardSettings): AdminArenaRewardSettings { return { monthly: value.monthly.map((tier) => ({ ...tier })), daily: value.daily.map((tier) => ({ ...tier })), daily_budget: Number(value.daily_budget) } }
function tierWidth(index: number) { return Math.max(0, activeTiers.value[index].rank_max - (index === 0 ? 0 : activeTiers.value[index - 1].rank_max)) }
function validate() {
  if (!settings.value) return ''
  if (!Number.isFinite(Number(settings.value.daily_budget)) || Number(settings.value.daily_budget) <= 0) return text('日榜预算必须是正数', 'Daily budget must be positive')
  for (const tiers of [settings.value.monthly, settings.value.daily]) {
    if (!tiers.length || tiers.length > 32) return text('奖励档位必须为 1 到 32 个', 'Reward tiers must contain 1 to 32 entries')
    let previous = 0
    for (const tier of tiers) {
      if (!Number.isInteger(tier.rank_max) || tier.rank_max <= previous || !Number.isFinite(Number(tier.amount)) || Number(tier.amount) <= 0) return text('名次必须严格递增，单人奖励必须是正数', 'Ranks must strictly increase and rewards must be positive')
      previous = tier.rank_max
    }
  }
  return ''
}
function addTier() { if (!settings.value || activeTiers.value.length >= 32) return; const last = activeTiers.value.at(-1); activeTiers.value.push({ rank_max: (last?.rank_max ?? 0) + 1, amount: last?.amount ?? 0.01 } as AdminArenaRewardTier) }
function formatMoney(value: number) { return new Intl.NumberFormat(isZh.value ? 'zh-CN' : 'en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(Number(value || 0)) }
async function load() { loading.value = true; try { settings.value = clone(await adminPlayAPI.getArenaRewardSettings()) } catch (error) { appStore.showError(extractApiErrorMessage(error, text('加载农场奖励规则失败', 'Failed to load farm reward rules'))) } finally { loading.value = false } }
async function save() { if (!settings.value || validationMessage.value || saving.value) return; saving.value = true; try { settings.value = clone(await adminPlayAPI.updateArenaRewardSettings(clone(settings.value))); appStore.showSuccess(text('农场奖励规则已保存', 'Farm reward rules saved')) } catch (error) { appStore.showError(extractApiErrorMessage(error, text('保存农场奖励规则失败', 'Failed to save farm reward rules'))) } finally { saving.value = false } }
onMounted(load)
</script>
