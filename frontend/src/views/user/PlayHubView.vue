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
import { trackGrowthEvent } from '@/utils/growthAnalytics'
import { vipTierBadgeClass } from '@/utils/vipColors'
import '@/styles/growth-world.css'

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
    const tm = hub.value?.team
    const team = tm?.team
    let subtitle = t('playHub.teamNone')
    if (team) {
      subtitle = t('playHub.teamJoined', {
        members: team.member_count,
        tokens: (team.token_sum ?? 0).toLocaleString(),
      })
      const aff = team.affiliate
      if (aff?.enabled && !aff.milestone_reached && (aff.tokens_to_milestone ?? 0) > 0) {
        subtitle = t('playHub.teamAffiliateRemaining', {
          remaining: (aff.tokens_to_milestone ?? 0).toLocaleString(),
        })
      } else if (aff?.enabled && (aff.milestone_reached || aff.captain_bonus_granted)) {
        subtitle = t('playHub.teamAffiliateReached')
      }
    }
    cards.push({
      key: 'agent-team',
      title: t('nav.agentTeam'),
      subtitle,
      route: '/agent-team',
      badge: tm?.enabled && !team ? t('playHub.badgePending') : undefined,
      enabled: !!tm?.enabled,
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

function trackHubClick(cardKey: string) {
  trackGrowthEvent('play_hub_action_click', { card_key: cardKey })
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
    <div
      data-testid="play-hub-shell"
      class="w-full max-w-none space-y-5 overflow-x-hidden pb-10 text-[var(--gw-ink)] 2xl:space-y-6"
    >
      <section class="gw-panel p-4 sm:p-5 lg:p-6 2xl:p-7">
        <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_24rem] 2xl:grid-cols-[minmax(0,1fr)_28rem]">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-3">
              <p class="gw-eyebrow mb-0">{{ t('playHub.eyebrow') }}</p>
              <span
                v-if="hub?.pending_actions"
                class="gw-buff"
              >
                {{ t('playHub.pending', { count: hub.pending_actions }) }}
              </span>
            </div>
            <h1 class="gw-title mt-2 break-words">{{ t('playHub.title') }}</h1>
            <p class="gw-subtitle max-w-3xl break-words">{{ t('playHub.subtitle') }}</p>
          </div>

          <div class="rounded-xl border border-[var(--gw-line)] bg-[var(--gw-soft)] p-4 2xl:p-5">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="min-w-0">
                <p class="gw-balance-label">{{ t('playHub.balanceLabel') }}</p>
                <p class="gw-balance-value">${{ balance.toFixed(2) }}</p>
              </div>
              <button
                v-if="hub?.growth.payment_enabled"
                type="button"
                class="gw-btn gw-btn-primary shrink-0 gap-2"
                @click="goPurchase"
              >
                <Icon name="creditCard" size="sm" />
                <span>{{ t('playHub.rechargeCta') }}</span>
              </button>
            </div>
            <p
              v-if="hub?.growth.recharge_multiplier && hub.growth.recharge_multiplier !== 1"
              class="gw-subtitle"
            >
              {{ t('playHub.rechargeBoost', { mult: hub.growth.recharge_multiplier }) }}
            </p>
            <p v-if="showGrowthCta" class="gw-subtitle mt-3 border-t border-[var(--gw-line)] pt-3">
              <span v-if="hub?.growth.first_recharge_eligible">{{ t('playHub.firstRecharge') }}</span>
              <span v-else-if="hub?.growth.balance_low_warning">
                {{ t('playHub.balanceLow', { threshold: (hub.growth.balance_low_threshold ?? 0).toFixed(2) }) }}
              </span>
            </p>
          </div>
        </div>
      </section>

      <div v-if="loading" class="gw-polling py-12 text-center">{{ t('models.loading') }}</div>
      <div v-else-if="!hub?.any_enabled && playCards.length === 0" class="gw-panel py-8 text-center gw-subtitle">
        {{ t('playHub.empty') }}
      </div>
      <template v-else>
        <div
          v-if="vip || primaryCampaign || (hub?.quests?.enabled && hub.quests.tasks?.length)"
          data-testid="play-hub-summary-grid"
          class="grid gap-4 lg:grid-cols-2 2xl:grid-cols-[minmax(18rem,0.85fr)_minmax(18rem,0.85fr)_minmax(26rem,1.3fr)]"
        >
          <section v-if="vip" class="gw-panel min-h-[12rem]">
            <div class="flex h-full flex-col">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="gw-balance-label">{{ t('playHub.vipTitle') }}</p>
                  <div class="mt-2 flex flex-wrap items-center gap-2">
                    <span :class="vipTierBadgeClass(vip.color_key)">{{ vip.label }}</span>
                    <span class="gw-buff">
                      {{ t('playHub.vipRechargeBonus', { pct: vip.recharge_bonus_pct ?? 0 }) }}
                    </span>
                    <span class="gw-subtitle mt-0">
                      {{ t('playHub.vipRecharged', { amount: (hub?.growth.total_recharged ?? 0).toFixed(2) }) }}
                    </span>
                  </div>
                </div>
                <button
                  v-if="hub?.growth.payment_enabled && vip.next_tier"
                  type="button"
                  class="gw-btn gw-btn-secondary gw-btn-sm shrink-0 gap-2"
                  @click="goPurchase"
                >
                  <Icon name="creditCard" size="sm" />
                  <span>{{ t('playHub.rechargeCta') }}</span>
                </button>
              </div>
              <p v-if="vip.next_tier" class="gw-subtitle mt-3 break-words">
                {{ t('playHub.vipNext', { amount: (vip.amount_to_next ?? 0).toFixed(2), label: vip.next_label ?? '' }) }}
              </p>
              <p v-else class="gw-subtitle mt-3" style="color: var(--gw-ok)">{{ t('playHub.vipMax') }}</p>
              <ul v-if="vipPerks.length" class="gw-subtitle mt-4 grid gap-1 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
                <li v-for="perk in vipPerks" :key="perk" class="break-words">· {{ perkLabel(perk) }}</li>
              </ul>
            </div>
          </section>

          <section v-if="primaryCampaign" class="gw-panel min-h-[12rem]">
            <div class="flex h-full flex-col">
              <p class="gw-balance-label">{{ t('playHub.campaignEyebrow') }}</p>
              <h2 class="gw-section-title mt-2 break-words">{{ campaignDisplayName }}</h2>
              <ul v-if="campaignPerkLines.length" class="gw-subtitle mt-0 space-y-1">
                <li v-for="(line, idx) in campaignPerkLines" :key="idx" class="break-words">· {{ line }}</li>
              </ul>
              <button
                v-if="hub?.growth.payment_enabled && primaryCampaign.rules.recharge_bonus_pct"
                type="button"
                class="gw-btn gw-btn-primary mt-4 w-fit gap-2"
                @click="goPurchase"
              >
                <Icon name="gift" size="sm" />
                <span>{{ t('playHub.rechargeCta') }}</span>
              </button>
            </div>
          </section>

          <section
            v-if="hub?.quests?.enabled && hub.quests.tasks?.length"
            class="gw-quest-banner m-0 min-h-[12rem]"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h2 class="gw-quest-banner-title">{{ t('playHub.questsTitle') }}</h2>
              <span class="gw-buff">
                {{ t('playHub.questsEnergy', { energy: hub.quests.energy, level: hub.quests.level }) }}
              </span>
            </div>
            <ul class="mt-3">
              <li v-for="task in hub.quests.tasks" :key="task.key" class="gw-quest-item">
                <span class="min-w-0 break-words">
                  <span class="gw-quest-check" :class="{ done: task.completed }">{{ task.completed ? '✓' : '' }}</span>
                  {{ t(`playHub.quest.${task.key}`, task.key) }} (+{{ task.energy }})
                </span>
                <router-link v-if="!task.completed && task.cta_route" :to="task.cta_route" class="gw-link shrink-0">
                  {{ t('playHub.actionGo') }}
                </router-link>
              </li>
            </ul>
          </section>
        </div>

        <section
          data-testid="play-hub-entry-grid"
          class="grid gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
        >
          <router-link
            v-if="hub?.image_studio?.enabled"
            to="/image-studio"
            class="gw-hub-card group min-h-[10rem] xl:col-span-2 2xl:min-h-[11rem]"
            @click="trackHubClick('image_studio')"
          >
            <div class="flex h-full items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="mb-3 inline-flex h-10 w-10 items-center justify-center rounded-lg border border-[var(--gw-line)] bg-[var(--gw-soft)]">
                  <Icon name="sparkles" size="sm" />
                </div>
                <h2 class="gw-section-title break-words">{{ t('nav.imageStudio') }}</h2>
                <p class="gw-subtitle break-words">
                  {{
                    hub.image_studio.has_completed_job
                      ? t('playHub.studioDone', { count: hub.image_studio.images_today })
                      : t('playHub.studioPending')
                  }}
                </p>
                <p
                  v-if="!hub.image_studio.has_completed_job"
                  class="mt-2 text-xs font-medium"
                  style="color: var(--gw-ink)"
                >
                  {{ t('playHub.actionStudio') }} →
                </p>
              </div>
              <span
                v-if="!hub.image_studio.has_completed_job"
                class="gw-buff shrink-0"
              >
                {{ t('playHub.badgePending') }}
              </span>
              <Icon v-else name="chevronRight" size="sm" class="shrink-0" style="color: var(--gw-ink-3)" />
            </div>
          </router-link>

          <router-link
            v-for="card in playCards"
            :key="card.key"
            :to="card.route"
            class="gw-hub-card group min-h-[10rem]"
            @click="trackHubClick(card.key)"
          >
            <div class="flex h-full items-start justify-between gap-3">
              <div class="min-w-0">
                <h2 class="gw-section-title break-words">{{ card.title }}</h2>
                <p class="gw-subtitle break-words">{{ card.subtitle }}</p>
                <p v-if="card.action" class="mt-2 text-xs font-medium" style="color: var(--gw-ink)">{{ card.action }} →</p>
              </div>
              <span v-if="card.badge" class="gw-buff shrink-0">{{ card.badge }}</span>
              <Icon v-else name="chevronRight" size="sm" class="shrink-0" style="color: var(--gw-ink-3)" />
            </div>
          </router-link>
        </section>
      </template>

      <div class="flex flex-wrap gap-3">
        <router-link to="/keys" class="gw-btn gw-btn-secondary gap-2">
          <Icon name="key" size="sm" />
          <span>{{ t('playHub.goKeys') }}</span>
        </router-link>
        <router-link to="/purchase" class="gw-btn gw-btn-secondary gap-2">
          <Icon name="creditCard" size="sm" />
          <span>{{ t('nav.buySubscription') }}</span>
        </router-link>
      </div>
    </div>
  </AppLayout>
</template>
