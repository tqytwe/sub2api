<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import playAPI, { type PlayHubSummary } from '@/api/play'
import { resolveCampaignDisplayName } from '@/utils/playCampaign'
import { useAuthStore } from '@/stores/auth'
import { isFeatureFlagEnabled, FeatureFlags } from '@/utils/featureFlags'

const { t, locale } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const loading = ref(true)
const hub = ref<PlayHubSummary | null>(null)

const balance = computed(() => hub.value?.growth.balance ?? authStore.user?.balance ?? 0)
const showGrowthCta = computed(() => {
  const g = hub.value?.growth
  if (!g?.payment_enabled) return false
  return g.first_recharge_eligible || g.balance_low_warning
})

const vip = computed(() => hub.value?.growth.vip)
const vipPerks = computed(() => vip.value?.perks ?? [])
const primaryCampaign = computed(() => hub.value?.campaigns?.[0] ?? null)
const campaignDisplayName = computed(() =>
  resolveCampaignDisplayName(primaryCampaign.value, locale.value),
)

const campaignPerkLines = computed(() => {
  const rules = primaryCampaign.value?.rules
  if (!rules) return []
  const lines: string[] = []
  if (rules.recharge_bonus_pct && rules.recharge_bonus_pct > 0) {
    lines.push(t('playHub.campaignRechargeBonus', { pct: rules.recharge_bonus_pct }))
  }
  if (rules.blindbox_extra_opens && rules.blindbox_extra_opens > 0) {
    lines.push(t('playHub.campaignBlindboxExtra', { count: rules.blindbox_extra_opens }))
  }
  if (rules.arena_score_multiplier && rules.arena_score_multiplier > 1) {
    lines.push(t('playHub.campaignArenaMult', { mult: rules.arena_score_multiplier }))
  }
  return lines
})

const playCards = computed(() => {
  const cards: Array<{
    key: string
    title: string
    subtitle: string
    route: string
    badge?: string
    action?: string
    enabled: boolean
  }> = []

  if (isFeatureFlagEnabled(FeatureFlags.playCheckin)) {
    const c = hub.value?.checkin
    cards.push({
      key: 'checkin',
      title: t('nav.checkIn'),
      subtitle: c?.checked_in_today
        ? t('playHub.checkinDone', { streak: c.streak_count || 0 })
        : t('playHub.checkinPending', { amount: (c?.reward_amount ?? 0).toFixed(2), streak: c?.streak_count || 0 }),
      route: '/check-in',
      badge: c && !c.checked_in_today ? t('playHub.badgePending') : undefined,
      action: c && !c.checked_in_today ? t('playHub.actionCheckin') : undefined,
      enabled: !!c?.enabled,
    })
  }

  if (isFeatureFlagEnabled(FeatureFlags.playArena)) {
    const a = hub.value?.arena
    const gap = a?.tokens_to_prev_rank ?? 0
    let subtitle = t('playHub.arenaNoRank')
    if (a?.rank) {
      subtitle = gap > 0
        ? t('playHub.arenaGap', { rank: a.rank, gap: gap.toLocaleString() })
        : t('playHub.arenaRank', { rank: a.rank, tokens: (a.token_sum ?? 0).toLocaleString() })
    }
    cards.push({
      key: 'arena',
      title: t('nav.arena'),
      subtitle,
      route: '/arena',
      action: t('playHub.actionUseApi'),
      enabled: !!a?.enabled,
    })
  }

  if (isFeatureFlagEnabled(FeatureFlags.playBlindbox)) {
    const b = hub.value?.blindbox
    cards.push({
      key: 'blindbox',
      title: t('nav.blindbox'),
      subtitle: t('playHub.blindboxStatus', {
        opens: b?.opens_today ?? 0,
        limit: b?.daily_limit ?? 0,
      }),
      route: '/blindbox',
      badge: b?.can_open ? t('playHub.badgePending') : undefined,
      action: b?.can_open ? t('playHub.actionOpen') : undefined,
      enabled: !!b?.enabled,
    })
  }

  if (isFeatureFlagEnabled(FeatureFlags.playQuiz)) {
    const q = hub.value?.quiz
    cards.push({
      key: 'quiz',
      title: t('nav.quizQuest'),
      subtitle: q?.already_submitted
        ? t('playHub.quizDone', { score: q.previous_score ?? 0, total: q.previous_total ?? 0 })
        : t('playHub.quizPending', { reward: (q?.reward_per_correct ?? 0).toFixed(2) }),
      route: '/quiz-quest',
      badge: q && !q.already_submitted ? t('playHub.badgePending') : undefined,
      enabled: !!q?.enabled,
    })
  }

  if (isFeatureFlagEnabled(FeatureFlags.playAgentTeam)) {
    const team = hub.value?.team?.team
    let subtitle = t('playHub.teamNone')
    if (team) {
      subtitle = t('playHub.teamJoined', { members: team.member_count, tokens: team.token_sum.toLocaleString() })
      const aff = team.affiliate
      if (aff?.enabled && !aff.milestone_reached && aff.tokens_to_milestone) {
        subtitle = t('playHub.teamAffiliateRemaining', { remaining: aff.tokens_to_milestone.toLocaleString() })
      } else if (aff?.milestone_reached) {
        subtitle = t('playHub.teamAffiliateReached')
      }
    }
    cards.push({
      key: 'team',
      title: t('nav.agentTeam'),
      subtitle: team
        ? subtitle
        : t('playHub.teamNone'),
      route: '/agent-team',
      enabled: !!hub.value?.team?.enabled,
    })
  }

  if (isFeatureFlagEnabled(FeatureFlags.affiliate)) {
    cards.push({
      key: 'affiliate',
      title: t('nav.affiliate'),
      subtitle: t('playHub.affiliateHint'),
      route: '/affiliate',
      enabled: true,
    })
  }

  return cards.filter((c) => c.enabled)
})

async function load() {
  loading.value = true
  try {
    await authStore.refreshUser()
    hub.value = await playAPI.getPlayHub()
  } catch {
    hub.value = null
  } finally {
    loading.value = false
  }
}

function goPurchase() {
  router.push('/purchase')
}

function perkLabel(perk: string): string {
  const key = `playHub.vipPerks.${perk}`
  const translated = t(key)
  return translated === key ? perk : translated
}

onMounted(load)
</script>

<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p class="text-sm font-medium text-primary-600 dark:text-primary-400">{{ t('playHub.eyebrow') }}</p>
          <h1 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ t('playHub.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('playHub.subtitle') }}</p>
        </div>
        <div v-if="hub?.pending_actions" class="rounded-full bg-amber-100 px-3 py-1 text-sm font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-200">
          {{ t('playHub.pending', { count: hub.pending_actions }) }}
        </div>
      </div>

      <div class="card overflow-hidden">
        <div class="bg-gradient-to-br from-violet-600 to-indigo-700 px-6 py-6 text-white">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div>
              <p class="text-sm text-violet-200">{{ t('playHub.balanceLabel') }}</p>
              <p class="text-3xl font-bold">${{ balance.toFixed(2) }}</p>
              <p v-if="hub?.growth.recharge_multiplier && hub.growth.recharge_multiplier !== 1" class="mt-1 text-sm text-violet-200">
                {{ t('playHub.rechargeBoost', { mult: hub.growth.recharge_multiplier }) }}
              </p>
            </div>
            <button
              v-if="hub?.growth.payment_enabled"
              type="button"
              class="rounded-xl bg-white px-5 py-2.5 text-sm font-semibold text-indigo-700 transition hover:bg-violet-50"
              @click="goPurchase"
            >
              {{ t('playHub.rechargeCta') }}
            </button>
          </div>
        </div>
        <div v-if="showGrowthCta" class="border-t border-violet-500/20 bg-violet-50 px-6 py-3 text-sm text-violet-900 dark:bg-violet-950/30 dark:text-violet-200">
          <span v-if="hub?.growth.first_recharge_eligible">{{ t('playHub.firstRecharge') }}</span>
          <span v-else-if="hub?.growth.balance_low_warning">
            {{ t('playHub.balanceLow', { threshold: (hub.growth.balance_low_threshold ?? 0).toFixed(2) }) }}
          </span>
        </div>
      </div>

      <div v-if="vip" class="card p-5">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p class="text-sm font-medium text-gray-500 dark:text-dark-400">{{ t('playHub.vipTitle') }}</p>
            <div class="mt-1 flex flex-wrap items-center gap-2">
              <span class="rounded-full bg-amber-100 px-3 py-1 text-sm font-semibold text-amber-900 dark:bg-amber-900/40 dark:text-amber-100">
                {{ vip.label }}
              </span>
              <span class="text-sm text-gray-500 dark:text-dark-400">
                {{ t('playHub.vipRecharged', { amount: (hub?.growth.total_recharged ?? 0).toFixed(2) }) }}
              </span>
            </div>
            <p v-if="vip.next_tier" class="mt-2 text-sm text-primary-600 dark:text-primary-400">
              {{ t('playHub.vipNext', { amount: (vip.amount_to_next ?? 0).toFixed(2), label: vip.next_label ?? '' }) }}
            </p>
            <p v-else class="mt-2 text-sm text-emerald-600 dark:text-emerald-400">{{ t('playHub.vipMax') }}</p>
          </div>
          <button
            v-if="hub?.growth.payment_enabled && vip.next_tier"
            type="button"
            class="btn btn-secondary text-sm"
            @click="goPurchase"
          >
            {{ t('playHub.rechargeCta') }}
          </button>
        </div>
        <ul v-if="vipPerks.length" class="mt-4 space-y-1 text-sm text-gray-600 dark:text-dark-300">
          <li v-for="perk in vipPerks" :key="perk">· {{ perkLabel(perk) }}</li>
        </ul>
      </div>

      <div v-if="primaryCampaign" class="card border-violet-200 bg-violet-50 p-5 dark:border-violet-800 dark:bg-violet-950/30">
        <p class="text-xs font-semibold uppercase tracking-wide text-violet-600 dark:text-violet-300">
          {{ t('playHub.campaignEyebrow') }}
        </p>
        <h2 class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ campaignDisplayName }}</h2>
        <ul v-if="campaignPerkLines.length" class="mt-2 space-y-1 text-sm text-violet-900 dark:text-violet-200">
          <li v-for="(line, idx) in campaignPerkLines" :key="idx">· {{ line }}</li>
        </ul>
        <button
          v-if="hub?.growth.payment_enabled && primaryCampaign.rules.recharge_bonus_pct"
          type="button"
          class="btn btn-primary btn-sm mt-4"
          @click="goPurchase"
        >
          {{ t('playHub.rechargeCta') }}
        </button>
      </div>

      <div v-if="loading" class="py-12 text-center text-gray-500 dark:text-dark-400">{{ t('models.loading') }}</div>
      <div v-else-if="!hub?.any_enabled && playCards.length === 0" class="card p-8 text-center text-gray-500 dark:text-dark-400">
        {{ t('playHub.empty') }}
      </div>
      <div v-else class="grid gap-4 sm:grid-cols-2">
        <router-link
          v-for="card in playCards"
          :key="card.key"
          :to="card.route"
          class="card group p-5 transition hover:shadow-md"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="font-semibold text-gray-900 group-hover:text-primary-600 dark:text-white dark:group-hover:text-primary-400">
                {{ card.title }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ card.subtitle }}</p>
              <p v-if="card.action" class="mt-2 text-xs font-medium text-primary-600 dark:text-primary-400">{{ card.action }} →</p>
            </div>
            <span
              v-if="card.badge"
              class="flex-shrink-0 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-200"
            >
              {{ card.badge }}
            </span>
            <Icon name="chevronRight" size="sm" class="flex-shrink-0 text-gray-300 group-hover:text-primary-500 dark:text-dark-600" />
          </div>
        </router-link>
      </div>

      <div class="flex flex-wrap gap-3">
        <router-link to="/keys" class="btn btn-secondary text-sm">{{ t('playHub.goKeys') }}</router-link>
        <router-link to="/purchase" class="btn btn-secondary text-sm">{{ t('nav.buySubscription') }}</router-link>
      </div>
    </div>
  </AppLayout>
</template>
