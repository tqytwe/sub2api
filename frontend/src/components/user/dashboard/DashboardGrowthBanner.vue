<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import playAPI, { type PlayHubGrowth } from '@/api/play'

const { t } = useI18n()
const router = useRouter()

const growth = ref<PlayHubGrowth | null>(null)
const loading = ref(true)

const showBanner = computed(() => {
  const g = growth.value
  if (!g?.payment_enabled) return false
  return g.first_recharge_eligible || g.balance_low_warning
})

const bannerType = computed<'first' | 'low' | null>(() => {
  const g = growth.value
  if (!g) return null
  if (g.first_recharge_eligible) return 'first'
  if (g.balance_low_warning) return 'low'
  return null
})

async function load() {
  loading.value = true
  try {
    const hub = await playAPI.getPlayHub()
    growth.value = hub.growth
  } catch {
    growth.value = null
  } finally {
    loading.value = false
  }
}

function goPurchase() {
  router.push('/purchase')
}

onMounted(load)

defineExpose({ reload: load })
</script>

<template>
  <div
    v-if="!loading && showBanner"
    class="rounded-xl border px-4 py-3 sm:flex sm:items-center sm:justify-between sm:gap-4"
    :class="bannerType === 'first'
      ? 'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/30'
      : 'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30'"
  >
    <p
      class="text-sm font-medium"
      :class="bannerType === 'first' ? 'text-emerald-800 dark:text-emerald-200' : 'text-amber-800 dark:text-amber-200'"
    >
      <template v-if="bannerType === 'first'">{{ t('dashboard.growth.firstRecharge') }}</template>
      <template v-else>
        {{ t('dashboard.growth.balanceLow', { threshold: (growth?.balance_low_threshold ?? 0).toFixed(2) }) }}
      </template>
    </p>
    <button
      type="button"
      class="mt-2 rounded-lg px-4 py-2 text-sm font-semibold text-white sm:mt-0"
      :class="bannerType === 'first' ? 'bg-emerald-600 hover:bg-emerald-700' : 'bg-amber-600 hover:bg-amber-700'"
      @click="goPurchase"
    >
      {{ t('dashboard.growth.rechargeCta') }}
    </button>
  </div>
</template>
