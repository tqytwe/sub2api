<template>
  <div class="plaza-pricing-table overflow-x-auto" :style="accentStyle">
    <table class="w-full min-w-[620px] table-fixed border-collapse text-sm tabular-nums">
      <colgroup>
        <col class="w-[34%]" />
        <col class="w-[33%]" />
        <col class="w-[33%]" />
      </colgroup>
      <thead>
        <tr
          class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400"
        >
          <th class="border-r border-gray-100 py-2.5 pl-5 pr-4 text-left align-middle dark:border-dark-700/60">
            {{ t('modelPlaza.table.model') }}
          </th>
          <th class="pz-bg px-3 py-2.5 text-center font-semibold">
            <span class="pz-title">{{ t('modelPlaza.table.paidPrice') }}</span>
            <span class="pz-unit ml-1 normal-case font-normal">{{ t('modelPlaza.table.unitPerMillion') }}</span>
          </th>
          <th class="border-l border-gray-100 px-3 py-2.5 text-center font-semibold dark:border-dark-700/60">
            <span class="text-gray-500 dark:text-dark-300">{{ t('modelPlaza.table.officialPrice') }}</span>
            <span class="ml-1 normal-case font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.unitPerMillion') }}</span>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="m in sortedModels"
          :key="`${m.platform}:${m.name}`"
          class="border-b border-gray-100 transition-colors last:border-b-0 hover:bg-gray-50/70 dark:border-dark-800 dark:hover:bg-dark-800/50"
        >
          <!-- 模型名 + 非 token 计费模式徽章 -->
          <td class="border-r border-gray-100 py-2.5 pl-5 pr-4 align-middle dark:border-dark-700/60">
            <div class="flex flex-wrap items-center gap-1.5">
              <span class="font-medium text-gray-900 dark:text-white">{{ m.name }}</span>
              <span
                v-if="platform && m.platform !== platform"
                :class="[
                  'inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium',
                  platformBadgeLightClass(m.platform)
                ]"
              >
                {{ platformLabel(m.platform) }}
              </span>
              <span
                v-if="billingMode(m) !== BILLING_MODE_TOKEN"
                class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700/70 dark:text-dark-300"
              >
                {{ billingModeLabel(m) }}
              </span>
            </div>
          </td>

          <td class="pz-cell px-3 py-2.5 align-middle font-mono font-semibold text-gray-900 dark:text-gray-50">
            <template v-if="billingMode(m) === BILLING_MODE_TOKEN">
              <div v-if="tokenIntervals(m).length" class="space-y-0.5 text-xs leading-5">
                <div v-for="(iv, idx) in tokenIntervals(m)" :key="idx">
                  <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
                  {{ paidPerMillion(iv.input_price) }} / {{ paidPerMillion(iv.output_price) }}
                </div>
              </div>
              <span v-else>{{ paidPerMillion(m.pricing?.input_price) }} / {{ paidPerMillion(m.pricing?.output_price) }}</span>
            </template>
            <template v-else>
              <div v-if="requestIntervals(m).length" class="space-y-0.5 text-xs leading-5">
                <div v-for="(iv, idx) in requestIntervals(m)" :key="idx">
                  <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
                  {{ paidRequestPrice(m, iv.per_request_price) }} {{ perUnitSuffix(m) }}
                </div>
              </div>
              <span v-else-if="m.pricing?.per_request_price != null">{{ paidRequestPrice(m, m.pricing.per_request_price) }} {{ perUnitSuffix(m) }}</span>
              <span v-else>-</span>
            </template>
          </td>
          <td class="border-l border-gray-100 px-3 py-2.5 align-middle font-mono text-xs text-gray-500 dark:border-dark-700/60 dark:text-dark-400">
            <span>{{ official(m.official_pricing?.input_price) }} / {{ official(m.official_pricing?.output_price) }}</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatScaled } from '@/utils/pricing'
import { platformAccentColor, platformBadgeLightClass, platformLabel } from '@/utils/platformColors'
import {
  BILLING_MODE_TOKEN,
  BILLING_MODE_IMAGE,
  type BillingMode
} from '@/constants/channel'
import type { PlazaModel } from '@/api/modelPlaza'
import type { UserPricingInterval } from '@/api/channels'

const props = defineProps<{
  models: PlazaModel[]
  /** 分组平台;实付分区底色随平台着色,未知平台回退品牌青。 */
  platform?: string
  /** 分组默认倍率。 */
  rateMultiplier: number
  /** 用户专属倍率;与默认不同,实付价按此计算并划线展示原倍率。 */
  userRateMultiplier?: number | null
  /** 生图独立倍率:true 时图片计费模型的实付倍率取 imageRateMultiplier,不取分组/专属倍率。 */
  imageRateIndependent?: boolean
  imageRateMultiplier?: number | null
}>()

const { t } = useI18n()

/** 实付分区只从平台拿一个主色,浅底/标题/下划线全部由 scoped CSS 用 color-mix 派生。 */
const accentStyle = computed(() => ({ '--plaza-accent': platformAccentColor(props.platform ?? '') }))

const PER_MILLION = 1_000_000

/**
 * 展示顺序:
 * 1. token 计费的排在前,按图/按次计费的沉到末尾——它们的官方 token 价与实付的按张/按次价不同量纲,混排无意义;
 * 2. 组内按官方输出价从高到低,无官方价的排最后;
 * 3. 同价按名称降序(新版本号在前,如 gpt-5.6 先于 gpt-5.5)。
 */
const sortedModels = computed(() => {
  return [...props.models].sort((a, b) => {
    const ta = billingMode(a) === BILLING_MODE_TOKEN
    const tb = billingMode(b) === BILLING_MODE_TOKEN
    if (ta !== tb) return ta ? -1 : 1
    const pa = a.official_pricing?.output_price ?? null
    const pb = b.official_pricing?.output_price ?? null
    if (pa != null && pb != null && pa !== pb) return pb - pa
    if (pa != null && pb == null) return -1
    if (pa == null && pb != null) return 1
    return b.name.localeCompare(a.name)
  })
})

const effectiveRate = computed(() => props.userRateMultiplier ?? props.rateMultiplier)

function billingMode(m: PlazaModel): BillingMode {
  return (m.pricing?.billing_mode || BILLING_MODE_TOKEN) as BillingMode
}

function billingModeLabel(m: PlazaModel): string {
  return billingMode(m) === BILLING_MODE_IMAGE
    ? t('modelPlaza.table.perImage')
    : t('modelPlaza.table.perRequest')
}

/** 价格统一保底 2 位小数,更长的有效小数原样保留。 */
const MIN_DECIMALS = 2

/** 实付价 = 渠道单价 × 生效倍率,按 $/1M token 展示。 */
function paidPerMillion(value: number | null | undefined): string {
  if (value == null) return '-'
  return formatScaled(value * effectiveRate.value, PER_MILLION, MIN_DECIMALS)
}

/** 图片计费模型且分组开启生图独立倍率:实付倍率取独立倍率,与计费口径一致。 */
function usesIndependentImageRate(m: PlazaModel): boolean {
  return billingMode(m) === BILLING_MODE_IMAGE && props.imageRateIndependent === true
}

/** 按次/按图片行的生效倍率。 */
function requestRate(m: PlazaModel): number {
  return usesIndependentImageRate(m) ? (props.imageRateMultiplier ?? 1) : effectiveRate.value
}

/** 按次 / 按图片单价(乘该行生效倍率,不换算 1M)。 */
function paidRequestPrice(m: PlazaModel, value: number | null | undefined): string {
  if (value == null) return '-'
  return formatScaled(value * requestRate(m), 1, MIN_DECIMALS)
}

/** 官方参考价不乘倍率。 */
function official(value: number | null | undefined): string {
  if (value == null) return '-'
  return formatScaled(value, PER_MILLION, MIN_DECIMALS)
}

/** 非 token 计费的单位后缀:按图片 → “/ 张”,按次 → “/ 次”。 */
function perUnitSuffix(m: PlazaModel): string {
  return billingMode(m) === BILLING_MODE_IMAGE
    ? t('modelPlaza.table.perUnitImage')
    : t('modelPlaza.table.perUnitRequest')
}

/** token 模式的阶梯定价(内联进输入/输出列)。 */
function tokenIntervals(m: PlazaModel): UserPricingInterval[] {
  return m.pricing?.intervals ?? []
}

function requestIntervals(m: PlazaModel): UserPricingInterval[] {
  return (m.pricing?.intervals ?? []).filter((iv) => iv.per_request_price != null)
}

/** 档位标签:优先管理员配置的 tier_label,否则按 token 区间生成(≤200K / >200K / 200K–1M)。 */
function tierLabel(iv: UserPricingInterval): string {
  if (iv.tier_label) return iv.tier_label
  const { min_tokens: min, max_tokens: max } = iv
  if (max == null) return `>${formatTokenCount(min)}`
  if (min === 0) return `≤${formatTokenCount(max)}`
  return `${formatTokenCount(min)}–${formatTokenCount(max)}`
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${trimZero(n / 1_000_000)}M`
  if (n >= 1_000) return `${trimZero(n / 1_000)}K`
  return String(n)
}

function trimZero(n: number): string {
  return String(Math.round(n * 100) / 100)
}
</script>

<style scoped>
/* 实付分区配色统一从 --plaza-accent(平台主色)派生,新增平台无需扩展样式 */
.plaza-pricing-table {
  --pz-title: color-mix(in srgb, var(--plaza-accent) 88%, black);
  --pz-bg: color-mix(in srgb, var(--plaza-accent) 7%, transparent);
  --pz-bg-hover: color-mix(in srgb, var(--plaza-accent) 13%, transparent);
}

.dark .plaza-pricing-table {
  --pz-title: color-mix(in srgb, var(--plaza-accent) 70%, white);
  --pz-bg: color-mix(in srgb, var(--plaza-accent) 6%, transparent);
  --pz-bg-hover: color-mix(in srgb, var(--plaza-accent) 10%, transparent);
}

.pz-bg,
.pz-cell {
  background-color: var(--pz-bg);
}

.pz-cell {
  transition: background-color 150ms cubic-bezier(0.4, 0, 0.2, 1);
}

tbody tr:hover .pz-cell {
  background-color: var(--pz-bg-hover);
}

.pz-title {
  /* color-mix 不可用的老浏览器回退为平台原色 */
  color: var(--plaza-accent);
  color: var(--pz-title);
  border-color: color-mix(in srgb, var(--pz-title) 30%, transparent);
}

.pz-unit {
  color: color-mix(in srgb, var(--pz-title) 62%, transparent);
}
</style>
