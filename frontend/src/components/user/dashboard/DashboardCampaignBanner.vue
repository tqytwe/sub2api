<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import playAPI, { type PlayCampaignSummary } from '@/api/play'

const { t } = useI18n()
const router = useRouter()

const campaigns = ref<PlayCampaignSummary[]>([])
const loading = ref(true)

const primary = computed(() => campaigns.value[0] ?? null)

const perkLines = computed(() => {
  const c = primary.value
  if (!c?.rules) return []
  const lines: string[] = []
  const r = c.rules
  if (r.recharge_bonus_pct && r.recharge_bonus_pct > 0) {
    lines.push(t('dashboard.campaign.rechargeBonus', { pct: r.recharge_bonus_pct }))
  }
  if (r.blindbox_extra_opens && r.blindbox_extra_opens > 0) {
    lines.push(t('dashboard.campaign.blindboxExtra', { count: r.blindbox_extra_opens }))
  }
  if (r.arena_score_multiplier && r.arena_score_multiplier > 1) {
    lines.push(t('dashboard.campaign.arenaMult', { mult: r.arena_score_multiplier }))
  }
  return lines
})

async function load() {
  loading.value = true
  try {
    campaigns.value = await playAPI.getActiveCampaigns()
  } catch {
    campaigns.value = []
  } finally {
    loading.value = false
  }
}

function goPlayHub() {
  router.push('/play')
}

function goPurchase() {
  router.push('/purchase')
}

onMounted(load)

defineExpose({ reload: load })
</script>

<template>
  <div
    v-if="!loading && primary"
    class="rounded-xl border border-violet-200 bg-gradient-to-r from-violet-50 to-indigo-50 px-4 py-4 dark:border-violet-800 dark:from-violet-950/40 dark:to-indigo-950/30"
  >
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <p class="text-xs font-semibold uppercase tracking-wide text-violet-600 dark:text-violet-300">
          {{ t('dashboard.campaign.eyebrow') }}
        </p>
        <p class="mt-1 text-base font-semibold text-gray-900 dark:text-white">{{ primary.name }}</p>
        <ul v-if="perkLines.length" class="mt-2 space-y-1 text-sm text-violet-900 dark:text-violet-200">
          <li v-for="(line, idx) in perkLines" :key="idx">· {{ line }}</li>
        </ul>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn btn-secondary btn-sm" @click="goPlayHub">
          {{ t('dashboard.campaign.viewHub') }}
        </button>
        <button
          v-if="primary.rules.recharge_bonus_pct"
          type="button"
          class="btn btn-primary btn-sm"
          @click="goPurchase"
        >
          {{ t('dashboard.campaign.rechargeCta') }}
        </button>
      </div>
    </div>
  </div>
</template>
