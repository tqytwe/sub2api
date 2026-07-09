<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchPublicVIPTiers, type PublicVIPTier } from '@/api/publicVipTiers'

const { t, locale } = useI18n()
const loading = ref(true)
const tiers = ref<PublicVIPTier[]>([])

function formatPerks(perks: string[] | undefined) {
  if (!perks?.length) {
    return locale.value === 'zh' ? '基础功能' : 'Base access'
  }
  return perks
    .map((key) => {
      const i18nKey = `docs.vipTiers.perks.${key}`
      return t(i18nKey, key)
    })
    .join(locale.value === 'zh' ? ' · ' : ' · ')
}

function formatRecharge(amount: number) {
  if (amount <= 0) {
    return locale.value === 'zh' ? '$0（新用户默认）' : '$0 (new users)'
  }
  return `$${amount.toLocaleString()}`
}

onMounted(async () => {
  try {
    const res = await fetchPublicVIPTiers()
    tiers.value = res.tiers ?? []
  } catch {
    tiers.value = []
  } finally {
    loading.value = false
  }
})

const hasTiers = computed(() => tiers.value.length > 0)
</script>

<template>
  <div v-if="loading" class="docs-vip-tiers docs-vip-tiers-loading">{{ t('docs.vipTiers.loading') }}</div>
  <div v-else-if="hasTiers" class="docs-vip-tiers">
    <p class="docs-lead">{{ t('docs.vipTiers.lead') }}</p>
    <h2>{{ t('docs.vipTiers.tableTitle') }}</h2>
    <div class="docs-table-wrap">
      <table class="docs-table">
        <thead>
          <tr>
            <th>{{ t('docs.vipTiers.colTier') }}</th>
            <th>{{ t('docs.vipTiers.colRecharge') }}</th>
            <th>{{ t('docs.vipTiers.colPerks') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="tier in tiers" :key="tier.tier">
            <td>
              <span class="docs-tier" :class="`docs-tier-${tier.tier}`">{{ tier.label }}</span>
            </td>
            <td>{{ formatRecharge(tier.min_recharge) }}</td>
            <td>{{ formatPerks(tier.perks) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="docs-tip">{{ t('docs.vipTiers.syncNote') }}</p>
  </div>
</template>
