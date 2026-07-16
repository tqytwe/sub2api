<template>
  <section class="card">
    <div class="flex flex-col gap-3 border-b border-gray-100 px-6 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ localText('盲盒奖池', 'Blind Box Pool') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ localText('当前生效的付费开箱奖池', 'Active paid blind box pool') }}
        </p>
      </div>
      <button
        type="button"
        class="btn btn-secondary inline-flex items-center gap-2 self-start"
        :disabled="loading || saving"
        :title="localText('重新加载奖池', 'Reload pool')"
        @click="loadPool"
      >
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
        {{ localText('重新加载', 'Reload') }}
      </button>
    </div>

    <div v-if="loading && !pool" class="flex min-h-40 items-center justify-center p-6">
      <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
    </div>

    <div v-else-if="loadError && !pool" class="p-6">
      <div
        role="alert"
        aria-live="polite"
        class="rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300"
      >
        {{ loadError }}
      </div>
    </div>

    <div v-else-if="pool" class="space-y-6 p-6">
      <div class="grid gap-4 md:grid-cols-3">
        <label class="block">
          <span class="input-label">{{ localText('版本', 'Version') }}</span>
          <input
            v-model="pool.version"
            data-testid="pool-version"
            type="text"
            class="input"
            maxlength="64"
            autocomplete="off"
          />
        </label>
        <label class="block">
          <span class="input-label">{{ localText('开箱成本', 'Open cost') }}</span>
          <input
            v-model.number="pool.cost"
            data-testid="pool-cost"
            type="number"
            min="0"
            step="0.01"
            class="input"
          />
        </label>
        <label class="block">
          <span class="input-label">{{ localText('RTP 上限', 'RTP cap') }}</span>
          <input
            v-model.number="pool.rtp_cap"
            data-testid="pool-rtp-cap"
            type="number"
            min="0"
            max="1"
            step="0.01"
            class="input"
          />
        </label>
      </div>

      <div class="overflow-x-auto rounded border border-gray-200 dark:border-dark-700">
        <table class="min-w-full table-fixed divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="w-16 px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">
                #
              </th>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ localText('奖励金额', 'Reward amount') }}
              </th>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ localText('权重', 'Weight') }}
              </th>
              <th class="w-16 px-4 py-3">
                <span class="sr-only">{{ localText('操作', 'Actions') }}</span>
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
            <tr
              v-for="(tier, index) in pool.tiers"
              :key="index"
              data-testid="tier-row"
            >
              <td class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                {{ index + 1 }}
              </td>
              <td class="px-4 py-3">
                <input
                  v-model.number="tier.amount"
                  :data-testid="`tier-amount-${index}`"
                  :aria-label="localText(`档位 ${index + 1} 奖励金额`, `Tier ${index + 1} reward amount`)"
                  type="number"
                  min="0"
                  step="0.01"
                  class="input min-w-32"
                />
              </td>
              <td class="px-4 py-3">
                <input
                  v-model.number="tier.weight"
                  :data-testid="`tier-weight-${index}`"
                  :aria-label="localText(`档位 ${index + 1} 权重`, `Tier ${index + 1} weight`)"
                  type="number"
                  min="1"
                  max="10000"
                  step="1"
                  class="input min-w-32"
                />
              </td>
              <td class="px-4 py-3 text-right">
                <button
                  type="button"
                  class="inline-flex h-9 w-9 items-center justify-center rounded text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-red-950/30 dark:hover:text-red-400"
                  :data-testid="`remove-tier-${index}`"
                  :disabled="pool.tiers.length <= 1 || saving"
                  :title="localText('删除档位', 'Remove tier')"
                  @click="removeTier(index)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <button
          type="button"
          class="btn btn-secondary inline-flex items-center gap-2 self-start"
          data-testid="add-tier"
          :disabled="pool.tiers.length >= 32 || saving"
          @click="addTier"
        >
          <Icon name="plus" size="sm" />
          {{ localText('添加档位', 'Add tier') }}
        </button>

        <dl class="grid flex-1 grid-cols-1 gap-3 sm:grid-cols-3 lg:max-w-2xl">
          <div class="rounded border border-gray-200 px-4 py-3 dark:border-dark-700">
            <dt class="text-xs text-gray-500 dark:text-gray-400">
              {{ localText('总权重', 'Total weight') }}
            </dt>
            <dd
              data-testid="total-weight"
              class="mt-1 text-base font-semibold"
              :class="totalWeight === weightDenominator ? 'text-gray-900 dark:text-white' : 'text-red-600 dark:text-red-400'"
            >
              {{ totalWeight }} / {{ weightDenominator }}
            </dd>
          </div>
          <div class="rounded border border-gray-200 px-4 py-3 dark:border-dark-700">
            <dt class="text-xs text-gray-500 dark:text-gray-400">
              {{ localText('期望奖励', 'Expected reward') }}
            </dt>
            <dd data-testid="expected-reward" class="mt-1 text-base font-semibold text-gray-900 dark:text-white">
              {{ formatMoney(expectedReward) }}
            </dd>
          </div>
          <div class="rounded border border-gray-200 px-4 py-3 dark:border-dark-700">
            <dt class="text-xs text-gray-500 dark:text-gray-400">
              {{ localText('有效 RTP', 'Effective RTP') }}
            </dt>
            <dd
              data-testid="effective-rtp"
              class="mt-1 text-base font-semibold"
              :class="rtpWithinCap ? 'text-gray-900 dark:text-white' : 'text-red-600 dark:text-red-400'"
            >
              {{ formatPercent(effectiveRTP) }}
            </dd>
          </div>
        </dl>
      </div>

      <div
        v-if="validationMessage"
        role="alert"
        aria-live="polite"
        class="rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300"
      >
        {{ validationMessage }}
      </div>

      <div class="flex justify-end">
        <button
          type="button"
          class="btn btn-primary inline-flex items-center gap-2"
          data-testid="save-pool"
          :disabled="!canSave"
          @click="savePool"
        >
          <Icon :name="saving ? 'refresh' : 'check'" size="sm" :class="{ 'animate-spin': saving }" />
          {{ saving ? localText('保存中', 'Saving') : localText('保存奖池', 'Save pool') }}
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Decimal from 'decimal.js'
import adminPlayAPI, { type PlayBlindboxPool } from '@/api/admin/play'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const weightDenominator = 10_000
const BlindboxDecimal = Decimal.clone({
  precision: 1000,
  rounding: Decimal.ROUND_HALF_UP,
})
const appStore = useAppStore()
const { locale } = useI18n()
const pool = ref<PlayBlindboxPool | null>(null)
const loading = ref(false)
const saving = ref(false)
const loadError = ref('')

const isZhLocale = computed(() => locale.value.startsWith('zh'))

function localText(zh: string, en: string): string {
  return isZhLocale.value ? zh : en
}

const totalWeight = computed(() =>
  pool.value?.tiers.reduce((total, tier) => total + numericValue(tier.weight), 0) ?? 0,
)

const expectedRewardDecimal = computed(() => {
  if (!pool.value) return new BlindboxDecimal(0)
  try {
    return pool.value.tiers.reduce(
      (total, tier) => total.plus(new BlindboxDecimal(tier.amount).mul(tier.weight)),
      new BlindboxDecimal(0),
    ).div(weightDenominator).toDecimalPlaces(16, Decimal.ROUND_HALF_UP)
  } catch {
    return new BlindboxDecimal(NaN)
  }
})

const expectedReward = computed(() => expectedRewardDecimal.value.toNumber())

const effectiveRTP = computed(() => {
  if (!pool.value || !isPositiveFinite(pool.value.cost) || !expectedRewardDecimal.value.isFinite()) {
    return 0
  }
  return expectedRewardDecimal.value.div(pool.value.cost).toNumber()
})

const rtpWithinCap = computed(() => {
  if (!pool.value || !expectedRewardDecimal.value.isFinite()) return false
  try {
    const limit = new BlindboxDecimal(pool.value.cost).mul(pool.value.rtp_cap)
    return expectedRewardDecimal.value.lte(limit)
  } catch {
    return false
  }
})

const validationMessage = computed(() => {
  if (!pool.value) return localText('奖池不可用', 'Pool is unavailable')
  if (!pool.value.version.trim()) return localText('版本不能为空', 'Version is required')
  if (!isPositiveFinite(pool.value.cost)) return localText('开箱成本必须大于 0', 'Open cost must be greater than 0')
  if (!isPositiveFinite(pool.value.rtp_cap) || pool.value.rtp_cap > 1) {
    return localText('RTP 上限必须在 0 到 1 之间', 'RTP cap must be within (0, 1]')
  }
  if (pool.value.tiers.length < 1 || pool.value.tiers.length > 32) {
    return localText('奖池必须包含 1 到 32 个档位', 'Pool must contain 1 to 32 tiers')
  }
  if (pool.value.tiers.some((tier) => !isNonNegativeFinite(tier.amount))) {
    return localText('奖励金额必须是非负有限数', 'Reward amounts must be non-negative finite numbers')
  }
  if (pool.value.tiers.some((tier) => !Number.isInteger(tier.weight) || tier.weight <= 0)) {
    return localText('权重必须是正整数', 'Weights must be positive integers')
  }
  if (totalWeight.value !== weightDenominator) {
    return localText('总权重必须等于 10000', 'Total weight must equal 10000')
  }
  if (!rtpWithinCap.value) {
    return localText('有效 RTP 不能超过 RTP 上限', 'Effective RTP cannot exceed the RTP cap')
  }
  return ''
})

const canSave = computed(() => Boolean(pool.value) && !loading.value && !saving.value && !validationMessage.value)

function numericValue(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function isPositiveFinite(value: number): boolean {
  return Number.isFinite(value) && value > 0
}

function isNonNegativeFinite(value: number): boolean {
  return Number.isFinite(value) && value >= 0
}

function formatMoney(value: number): string {
  return `$${value.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}`
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(2)}%`
}

function addTier(): void {
  if (!pool.value || pool.value.tiers.length >= 32) return
  pool.value.tiers.push({ amount: 0, weight: 1 })
}

function removeTier(index: number): void {
  if (!pool.value || pool.value.tiers.length <= 1) return
  pool.value.tiers.splice(index, 1)
}

async function loadPool(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const loaded = await adminPlayAPI.getBlindboxPool()
    pool.value = {
      ...loaded,
      tiers: loaded.tiers.map((tier) => ({ ...tier })),
    }
  } catch (error) {
    loadError.value = extractApiErrorMessage(
      error,
      localText('加载盲盒奖池失败', 'Failed to load blind box pool'),
    )
    appStore.showError(loadError.value)
  } finally {
    loading.value = false
  }
}

async function savePool(): Promise<void> {
  if (!pool.value || !canSave.value) return
  saving.value = true
  try {
    const payload: PlayBlindboxPool = {
      version: pool.value.version.trim(),
      cost: pool.value.cost,
      rtp_cap: pool.value.rtp_cap,
      tiers: pool.value.tiers.map((tier) => ({ ...tier })),
    }
    const updated = await adminPlayAPI.updateBlindboxPool(payload)
    pool.value = {
      ...updated,
      tiers: updated.tiers.map((tier) => ({ ...tier })),
    }
    appStore.showSuccess(localText('盲盒奖池已保存', 'Blind box pool saved'))
  } catch (error) {
    appStore.showError(
      extractApiErrorMessage(error, localText('保存盲盒奖池失败', 'Failed to save blind box pool')),
    )
  } finally {
    saving.value = false
  }
}

onMounted(loadPool)
</script>
