<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import playAPI, { type PlayHubGrowth } from '@/api/play'
import '@/styles/growth-world.css'

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
    class="gw-quest-banner flex flex-wrap items-center justify-between gap-3"
    :class="bannerType === 'low' ? 'gw-quest-banner--warn' : ''"
  >
    <p class="gw-subtitle mb-0">
      <template v-if="bannerType === 'first'">{{ t('dashboard.growth.firstRecharge') }}</template>
      <template v-else>
        {{ t('dashboard.growth.balanceLow', { threshold: (growth?.balance_low_threshold ?? 0).toFixed(2) }) }}
      </template>
    </p>
    <button type="button" class="gw-btn gw-btn-primary shrink-0" @click="goPurchase">
      {{ t('dashboard.growth.rechargeCta') }}
    </button>
  </div>
</template>
