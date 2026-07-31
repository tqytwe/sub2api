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
              <th class="w-16 px-4 py-3">
                <span class="sr-only">{{ text('操作', 'Actions') }}</span>
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="(tier, index) in settings.tiers" :key="index" data-testid="team-reward-tier-row">
              <td class="px-4 py-3">
                <input
                  v-model="tier.threshold"
                  :data-testid="`team-threshold-${index}`"
                  :aria-label="text(`档位 ${index + 1} 消费阈值`, `Tier ${index + 1} spend threshold`)"
                  type="number"
                  min="0.00000001"
                  step="0.00000001"
                  class="input"
                />
              </td>
              <td class="px-4 py-3">
                <input
                  v-model="tier.rate"
                  :data-testid="`team-rate-${index}`"
                  :aria-label="text(`档位 ${index + 1} 返还比例`, `Tier ${index + 1} reward rate`)"
                  type="number"
                  min="0.00000001"
                  max="1"
                  step="0.00000001"
                  class="input"
                />
              </td>
              <td class="px-4 py-3 text-right">
                <button
                  type="button"
                  class="inline-flex h-9 w-9 items-center justify-center rounded text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-red-950/30 dark:hover:text-red-400"
                  :data-testid="`remove-team-reward-tier-${index}`"
                  :disabled="settings.tiers.length <= 1 || saving"
                  :title="text('删除挡位', 'Remove tier')"
                  :aria-label="text(`删除第 ${index + 1} 档`, `Remove tier ${index + 1}`)"
                  @click="removeTier(index)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <button
          type="button"
          class="btn btn-secondary inline-flex items-center gap-2 self-start"
          data-testid="add-team-reward-tier"
          :disabled="!canAddTier || saving"
          @click="addTier"
        >
          <Icon name="plus" size="sm" />
          {{ text('新增挡位', 'Add tier') }}
        </button>
        <button
          type="button"
          class="btn btn-primary inline-flex items-center gap-2 self-start sm:self-auto"
          data-testid="save-team-rewards"
          :disabled="!canSave"
          @click="save"
        >
          <Icon :name="saving ? 'refresh' : 'check'" size="sm" :class="{ 'animate-spin': saving }" />
          {{ text('保存共享奖励', 'Save shared rewards') }}
        </button>
      </div>

      <p
        v-if="validationMessage"
        id="team-reward-validation"
        data-testid="team-reward-validation"
        role="alert"
        aria-live="polite"
        class="rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300"
      >
        {{ validationMessage }}
      </p>

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
import Decimal from 'decimal.js'
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
const maxTiers = 32
const maxDecimalPlaces = 8
const maxIntegerDigits = 12

const validationMessage = computed(() => validateSettings(settings.value))
const canSave = computed(() => Boolean(settings.value) && !saving.value && !validationMessage.value)
const canAddTier = computed(() => nextTier(settings.value?.tiers ?? []) !== null)

function parseDecimal(value: unknown): Decimal | null {
  const raw = String(value ?? '').trim()
  if (!raw) return null
  try {
    const parsed = new Decimal(raw)
    if (!parsed.isFinite() || parsed.decimalPlaces() > maxDecimalPlaces) return null
    if (parsed.abs().floor().toFixed(0).length > maxIntegerDigits) return null
    return parsed
  } catch {
    return null
  }
}

function validateSettings(value: TeamRewardSettings | null): string {
  if (!value) return ''
  const cap = parseDecimal(value.cap)
  if (!cap || !cap.greaterThan(0)) return text('月度奖池上限必须是最多 8 位小数的正数', 'Monthly pool cap must be a positive number with at most 8 decimal places')
  if (!/^\d{4}-(0[1-9]|1[0-2])$/.test(value.start_month)) return text('开始月份必须采用 YYYY-MM 格式', 'Start month must use YYYY-MM format')
  if (value.tiers.length === 0 || value.tiers.length > maxTiers) return text(`奖励挡位必须为 1 到 ${maxTiers} 个`, `Reward tiers must contain 1 to ${maxTiers} entries`)

  let previousThreshold: Decimal | null = null
  let previousRate: Decimal | null = null
  for (const [index, tier] of value.tiers.entries()) {
    const threshold = parseDecimal(tier.threshold)
    const rate = parseDecimal(tier.rate)
    if (!threshold || !threshold.greaterThan(0)) return text(`第 ${index + 1} 档消费阈值必须是正数`, `Tier ${index + 1} spend threshold must be positive`)
    if (!rate || !rate.greaterThan(0) || rate.greaterThan(1)) return text(`第 ${index + 1} 档返还比例必须在 0 到 1 之间`, `Tier ${index + 1} reward rate must be within (0, 1]`)
    if (previousThreshold && !threshold.greaterThan(previousThreshold)) return text('消费阈值必须严格递增', 'Spend thresholds must be strictly increasing')
    if (previousRate && !rate.greaterThan(previousRate)) return text('返还比例必须严格递增', 'Reward rates must be strictly increasing')
    previousThreshold = threshold
    previousRate = rate
  }
  return ''
}

function nextTier(tiers: TeamRewardSettings['tiers']): TeamRewardSettings['tiers'][number] | null {
  if (tiers.length === 0 || tiers.length >= maxTiers) return null
  const lastTier = tiers[tiers.length - 1]
  const threshold = parseDecimal(lastTier.threshold)
  const rate = parseDecimal(lastTier.rate)
  if (!threshold || !rate || !threshold.greaterThan(0) || !rate.greaterThan(0)) return null

  const nextThreshold = threshold.mul(2)
  const previousRate = tiers.length > 1 ? parseDecimal(tiers[tiers.length - 2].rate) : null
  const rateIncrement = previousRate && rate.greaterThan(previousRate) ? rate.minus(previousRate) : new Decimal('0.01')
  const nextRate = rate.plus(rateIncrement)
  if (nextThreshold.abs().floor().toFixed(0).length > maxIntegerDigits || nextRate.greaterThan(1)) return null
  return { threshold: nextThreshold.toString(), rate: nextRate.toString() }
}

function addTier() {
  if (!settings.value || saving.value) return
  const tier = nextTier(settings.value.tiers)
  if (tier) settings.value.tiers.push(tier)
}

function removeTier(index: number) {
  if (!settings.value || settings.value.tiers.length <= 1 || saving.value) return
  settings.value.tiers.splice(index, 1)
}

function cloneSettings(value: TeamRewardSettings): TeamRewardSettings {
  return {
    ...value,
    cap: String(value.cap),
    start_month: String(value.start_month),
    tiers: value.tiers.map((tier) => ({
      threshold: String(tier.threshold),
      rate: String(tier.rate),
    })),
  }
}

async function load() {
  loading.value = true
  try {
    const [config, history] = await Promise.all([
      adminPlayAPI.getTeamRewardSettings(),
      adminPlayAPI.listTeamRewardSettlements(),
    ])
    settings.value = cloneSettings(config)
    settlements.value = history
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, text('加载团队奖励失败', 'Failed to load team rewards')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!settings.value || saving.value || validationMessage.value) return
  saving.value = true
  try {
    settings.value = cloneSettings(await adminPlayAPI.updateTeamRewardSettings(cloneSettings(settings.value)))
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
