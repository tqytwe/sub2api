<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import PlayUserAvatar from '@/components/play/PlayUserAvatar.vue'
import RewardCelebrationOverlay from '@/components/play/RewardCelebrationOverlay.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, {
  type PlayArenaCurrent,
  type PlayArenaLeaderboard,
  type PlayArenaScore,
  type PlayQuestToday,
} from '@/api/play'
import '@/styles/public-pages.css'
import '@/styles/arena-rpg.css'
import { trackGrowthEvent, trackQuestCompleteOnce } from '@/utils/growthAnalytics'

const { t, te } = useI18n()
const authStore = useAuthStore()

type BoardTab = 'daily' | 'monthly'
type RankTone = 'gold' | 'silver' | 'bronze' | 'standard'

function readStringList(key: string): string[] {
  if (!te(key)) return []
  const val = t(key, { returnObjects: true }) as unknown
  return Array.isArray(val) ? val.filter((item): item is string => typeof item === 'string') : []
}

const loading = ref(true)
const tab = ref<BoardTab>('monthly')
const monthlyCurrent = ref<PlayArenaCurrent | null>(null)
const dailyCurrent = ref<PlayArenaCurrent | null>(null)
const monthlyBoard = ref<PlayArenaLeaderboard | null>(null)
const dailyBoard = ref<PlayArenaLeaderboard | null>(null)
const quests = ref<PlayQuestToday | null>(null)
const arenaCelebrationDismissed = ref(false)

const current = computed(() => (tab.value === 'daily' ? dailyCurrent.value : monthlyCurrent.value))
const leaderboard = computed(() => (tab.value === 'daily' ? dailyBoard.value : monthlyBoard.value))
const periodLabel = computed(() => current.value?.period?.name || leaderboard.value?.period?.name || '')
const steps = computed(() => readStringList('play.arena.steps'))
const rules = computed(() => readStringList('play.arena.rules'))
const leaderboardRows = computed(() => leaderboard.value?.rows ?? [])
const podiumRows = computed(() => {
  const byRank = new Map(leaderboardRows.value.map(row => [row.rank, row]))
  return [2, 1, 3].map(rank => byRank.get(rank)).filter((row): row is PlayArenaScore => Boolean(row))
})
const rankRows = computed(() => leaderboardRows.value.filter(row => row.rank > 3))
const selectedTokenSum = computed(() => current.value?.display_token_sum ?? current.value?.token_sum ?? 0)
const selectedRank = computed(() => current.value?.rank ?? null)
const selectedGap = computed(() => current.value?.tokens_to_prev_rank ?? 0)
const selectedEstimatedReward = computed(() => current.value?.estimated_reward ?? 0)
const showArenaSettlementCelebration = computed(() =>
  !arenaCelebrationDismissed.value &&
  current.value?.period?.status === 'settled' &&
  selectedEstimatedReward.value > 0,
)
const rankProgressPercent = computed(() => {
  if (!selectedTokenSum.value || !selectedGap.value) return selectedRank.value ? 100 : 0
  return Math.max(6, Math.min(100, Math.round((selectedTokenSum.value / (selectedTokenSum.value + selectedGap.value)) * 100)))
})
const rewardStatus = computed(() => {
  if (!selectedRank.value) return t('arena.competitive.noRank')
  return selectedRank.value <= 10 ? t('arena.competitive.rewardZone') : t('arena.competitive.keepClimbing')
})

const xpPercent = computed(() => {
  const q = quests.value
  if (!q?.enabled || q.energy_to_next_level <= 0) return 100
  const total = q.energy + q.energy_to_next_level
  return total > 0 ? Math.min(100, Math.round((q.energy / total) * 100)) : 0
})

const showSeedGuide = computed(() => {
  if (!authStore.isAuthenticated) return false
  const tokens = (monthlyCurrent.value?.token_sum ?? 0) + (dailyCurrent.value?.token_sum ?? 0)
  return tokens <= 0
})

function questLabel(key: string) {
  const k = `arena.quests.${key}`
  return te(k) ? t(k) : key
}

function formatTokens(value?: number) {
  return (value ?? 0).toLocaleString()
}

function formatMoney(value?: number) {
  return (value ?? 0).toFixed(2)
}

function toneForRank(rank: number): RankTone {
  if (rank === 1) return 'gold'
  if (rank === 2) return 'silver'
  if (rank === 3) return 'bronze'
  return 'standard'
}

function arenaCelebrationKey() {
  const period = current.value?.period
  return period?.id ? `play-arena-settled:${tab.value}:${period.id}` : ''
}

function syncArenaCelebrationSeen() {
  const key = arenaCelebrationKey()
  if (!key) {
    arenaCelebrationDismissed.value = false
    return
  }
  try {
    arenaCelebrationDismissed.value = window.sessionStorage.getItem(key) === '1'
  } catch {
    arenaCelebrationDismissed.value = false
  }
}

function dismissArenaCelebration() {
  const key = arenaCelebrationKey()
  if (key) {
    try {
      window.sessionStorage.setItem(key, '1')
    } catch {
      // Ignore storage failures; closing the overlay for this view is enough.
    }
  }
  arenaCelebrationDismissed.value = true
}

function isCurrentRank(row: PlayArenaScore) {
  return selectedRank.value != null && row.rank === selectedRank.value
}

function switchTab(next: BoardTab) {
  tab.value = next
  if (next === 'daily') {
    trackGrowthEvent('farm_daily_tab_view')
  }
}

async function load() {
  loading.value = true
  try {
    const [mCur, dCur, mBoard, dBoard, q] = await Promise.all([
      playAPI.getArenaCurrent(),
      playAPI.getArenaDailyCurrent(),
      playAPI.getArenaLeaderboard(50),
      playAPI.getArenaDailyLeaderboard(50),
      authStore.isAuthenticated ? playAPI.getQuestsToday() : Promise.resolve(null),
    ])
    monthlyCurrent.value = mCur
    dailyCurrent.value = dCur
    monthlyBoard.value = mBoard
    dailyBoard.value = dBoard
    quests.value = q
    syncArenaCelebrationSeen()
    if (q?.tasks?.some((task) => task.key === 'api_call' && task.completed)) {
      trackQuestCompleteOnce('api_call')
    }
  } catch {
    monthlyCurrent.value = null
    dailyCurrent.value = null
    monthlyBoard.value = null
    dailyBoard.value = null
    quests.value = null
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch([tab, current], syncArenaCelebrationSeen)
</script>

<template>
  <div class="play-page arena-rpg-page">
    <header class="public-page-header">
      <PublicPlayBackLink />
      <PublicPageToolbar />
    </header>

    <main class="play-main arena-competitive-main">
      <div class="play-workspace">
        <section class="play-hero-panel">
          <div class="play-hero-grid">
            <div>
              <p class="play-eyebrow">{{ t('play.arena.eyebrow') }}</p>
              <h1 class="play-title">{{ t('play.arena.title') }}</h1>
              <p class="play-subtitle">{{ t('play.arena.subtitle') }}</p>
              <p v-if="periodLabel" class="arena-period-pill">{{ t('arena.period', { name: periodLabel }) }}</p>
            </div>

            <div class="play-action-panel">
              <div class="arena-rpg-tabs">
                <button type="button" class="arena-rpg-tab" :class="{ active: tab === 'daily' }" @click="switchTab('daily')">
                  {{ t('arena.rpg.tabDaily') }}
                </button>
                <button type="button" class="arena-rpg-tab" :class="{ active: tab === 'monthly' }" @click="switchTab('monthly')">
                  {{ t('arena.rpg.tabMonthly') }}
                </button>
              </div>
              <p class="play-note m-0">{{ rewardStatus }}</p>
            </div>
          </div>
        </section>

        <div v-if="loading" class="play-note">{{ t('models.loading') }}</div>
        <div v-else-if="!monthlyCurrent?.enabled && !dailyCurrent?.enabled" class="play-note">{{ t('arena.disabled') }}</div>
        <template v-else>
          <section class="arena-hero-grid">
            <div class="arena-season-panel">
              <p class="arena-panel-label">{{ t('arena.competitive.mySeason') }}</p>
              <div class="arena-season-rank">
                {{ selectedRank ? `#${selectedRank}` : t('arena.competitive.noRank') }}
              </div>
              <p v-if="selectedRank" class="arena-season-copy">
                {{ t('arena.myStats', { rank: selectedRank, tokens: formatTokens(selectedTokenSum) }) }}
              </p>
              <p v-if="selectedGap" class="arena-season-copy">
                {{ t('arena.gapToPrev', { gap: formatTokens(selectedGap) }) }}
              </p>
              <div class="arena-season-meter" aria-label="rank progress">
                <span :style="{ width: `${rankProgressPercent}%` }" />
              </div>
              <div class="arena-season-stats">
                <span>{{ rewardStatus }}</span>
                <span>{{ t('arena.competitive.topRange') }}</span>
              </div>
              <p v-if="selectedEstimatedReward > 0" class="arena-estimated-reward">
                {{ t('arena.estimatedReward', { amount: formatMoney(selectedEstimatedReward) }) }}
              </p>
              <div v-if="monthlyCurrent?.recharge_boost_active || monthlyCurrent?.campaign_active" class="arena-buff-row">
                <span v-if="monthlyCurrent?.recharge_boost_active" class="arena-buff">
                  {{ t('arena.boostActive', { mult: monthlyCurrent.arena_score_multiplier || 1.5 }) }}
                </span>
                <span v-if="monthlyCurrent?.campaign_active" class="arena-buff">{{ t('arena.rpg.campaignBuff') }}</span>
              </div>
            </div>

            <div class="arena-reward-panel">
              <p class="arena-panel-label">{{ t('arena.competitive.rewardTitle') }}</p>
              <ul>
                <li>{{ t('arena.competitive.rewardRuleRanked') }}</li>
                <li>{{ t('arena.competitive.rewardRuleSettle') }}</li>
                <li>{{ t('arena.competitive.rewardRuleEnergy') }}</li>
              </ul>
              <div class="arena-formula-panel">
                <strong>{{ t('arena.competitive.formulaTitle') }}</strong>
                <span>{{ t('arena.competitive.formulaRank') }}</span>
                <span>{{ t('arena.competitive.formulaBoost') }}</span>
              </div>
            </div>
          </section>

          <div class="play-detail-grid">
            <div class="play-content-panel">
              <section v-if="podiumRows.length" class="play-section">
                <h2 class="play-section-title">{{ t('arena.competitive.podium') }}</h2>
                <div class="arena-podium">
                  <article
                    v-for="row in podiumRows"
                    :key="row.user_id"
                    class="arena-podium-card"
                    :class="[`tone-${toneForRank(row.rank)}`, { champion: row.rank === 1 }]"
                  >
                    <div class="arena-podium-rank">#{{ row.rank }}</div>
                    <PlayUserAvatar :name="row.display_name" :avatar-url="row.avatar_url" size-class="h-10 w-10" />
                    <strong>{{ formatTokens(row.token_sum) }}</strong>
                    <span>{{ row.rank <= 10 ? t('arena.competitive.rewardZone') : t('arena.competitive.keepClimbing') }}</span>
                  </article>
                </div>
              </section>

              <section class="play-section mb-0">
                <h2 class="play-section-title">{{ t('arena.leaderboard') }}</h2>
                <div v-if="rankRows.length" class="arena-rank-list">
                  <div
                    v-for="row in rankRows"
                    :key="row.user_id"
                    class="arena-rank-row"
                    :class="{ current: isCurrentRank(row) }"
                  >
                    <span class="arena-rank-number">#{{ row.rank }}</span>
                    <PlayUserAvatar :name="row.display_name" :avatar-url="row.avatar_url" />
                    <span class="arena-rank-tokens">{{ formatTokens(row.token_sum) }} tokens</span>
                    <span class="arena-rank-reward">
                      {{ row.rank <= 10 ? t('arena.competitive.rewardZone') : t('arena.competitive.keepClimbing') }}
                    </span>
                  </div>
                </div>
                <div v-else-if="!leaderboardRows.length" class="play-note">{{ t('arena.empty') }}</div>
              </section>
            </div>

            <aside class="play-workspace">
              <section v-if="quests?.enabled" class="arena-level-card">
                <div class="arena-rpg-level">
                  <div>
                    <p class="text-sm text-gray-500">{{ t('arena.rpg.season') }} · {{ periodLabel }}</p>
                    <p class="text-lg font-semibold">
                      {{ t('arena.rpg.level', { level: quests.level }) }}
                      <span class="text-sm font-normal text-gray-500">· {{ t('arena.rpg.farmer') }}</span>
                    </p>
                  </div>
                  <p class="text-sm text-gray-600 dark:text-dark-300">
                    {{ t('arena.rpg.energyGap', { gap: quests.energy_to_next_level.toLocaleString() }) }}
                  </p>
                </div>
                <div class="arena-rpg-xp-bar">
                  <div class="arena-rpg-xp-fill" :style="{ width: `${xpPercent}%` }" />
                </div>
              </section>

              <section v-if="quests?.enabled && quests.tasks?.length" class="play-section">
                <h2 class="play-section-title">{{ t('arena.rpg.dailyQuests') }}</h2>
                <div class="arena-quest-grid">
                  <div
                    v-for="task in quests.tasks"
                    :key="task.key"
                    class="arena-quest-card"
                    :class="{ done: task.completed }"
                  >
                    <div>
                      <p>{{ questLabel(task.key) }}</p>
                      <span>{{ task.completed ? t('arena.competitive.questDone') : t('arena.rpg.go') }}</span>
                    </div>
                    <strong>{{ t('arena.competitive.questEnergy', { energy: task.energy }) }}</strong>
                    <router-link v-if="!task.completed && task.cta_route" :to="task.cta_route" class="arena-quest-link">
                      {{ t('arena.rpg.go') }}
                    </router-link>
                  </div>
                </div>
              </section>

              <section v-if="showSeedGuide" class="arena-field-card arena-seed-guide">
                <h2 class="play-section-title">{{ t('arena.rpg.seedTitle') }}</h2>
                <ol>
                  <li>{{ t('arena.rpg.seedStep1') }}</li>
                  <li>{{ t('arena.rpg.seedStep2') }}</li>
                  <li>{{ t('arena.rpg.seedStep3') }}</li>
                </ol>
              </section>

              <section v-if="steps.length" class="play-section">
                <h2 class="play-section-title">{{ t('play.howItWorks') }}</h2>
                <ol class="play-steps">
                  <li v-for="(step, idx) in steps" :key="idx">{{ step }}</li>
                </ol>
              </section>

              <section v-if="rules.length" class="play-section mb-0">
                <h2 class="play-section-title">{{ t('play.arena.rulesTitle') }}</h2>
                <ul class="play-rules">
                  <li v-for="(rule, idx) in rules" :key="idx">{{ rule }}</li>
                </ul>
              </section>
            </aside>
          </div>
        </template>

        <div v-if="!authStore.isAuthenticated" class="play-actions">
          <router-link to="/register" class="play-btn play-btn-primary">{{ t('play.arena.ctaGuest') }}</router-link>
        </div>
      </div>
    </main>

    <RewardCelebrationOverlay
      :open="showArenaSettlementCelebration"
      :title="t('arena.competitive.settlementCelebrationTitle')"
      :amount="`$${formatMoney(selectedEstimatedReward)}`"
      :subtitle="t('arena.competitive.settlementCelebrationSubtitle')"
      :details="[rewardStatus, t('arena.competitive.formulaRank')]"
      color-key="gold"
      variant="settlement"
      :secondary-label="t('arena.competitive.viewDetails')"
      @close="dismissArenaCelebration"
      @secondary="dismissArenaCelebration"
    />
    <SupportFloatingCard />
  </div>
</template>

<style scoped>
.arena-period-pill {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  margin: 0 0 18px;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: var(--card);
  padding: 4px 10px;
  color: var(--ink-2);
  font-size: 12px;
}

.arena-hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(280px, 0.9fr);
  gap: 14px;
  margin: 0 0 22px;
}

.arena-season-panel,
.arena-reward-panel,
.arena-level-card,
.arena-podium-card,
.arena-rank-row,
.arena-quest-card {
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--card);
}

.arena-season-panel,
.arena-reward-panel,
.arena-level-card {
  padding: 20px;
}

.arena-panel-label {
  margin: 0 0 8px;
  font-family: 'IBM Plex Mono', monospace;
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--ink-3);
}

.arena-season-rank {
  font-family: 'JetBrains Mono', monospace;
  font-size: clamp(34px, 6vw, 54px);
  font-weight: 800;
  line-height: 1;
  color: var(--ink);
}

.arena-season-copy,
.arena-reward-panel li {
  color: var(--ink-2);
  font-size: 14px;
  line-height: 1.7;
}

.arena-season-copy {
  margin: 8px 0 0;
}

.arena-reward-panel ul {
  margin: 0;
  padding-left: 18px;
}

.arena-season-meter {
  height: 8px;
  margin: 18px 0 12px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(120, 113, 108, 0.16);
}

.arena-season-meter span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: #1f7a5b;
}

.arena-season-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.arena-season-stats span {
  border-radius: 999px;
  background: rgba(120, 113, 108, 0.12);
  padding: 7px 10px;
  color: var(--ink-2);
  font-size: 12px;
}

.arena-podium {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  align-items: end;
}

.arena-podium-card {
  display: grid;
  gap: 10px;
  min-height: 176px;
  padding: 18px;
}

.arena-podium-card.champion {
  min-height: 218px;
}

.arena-podium-rank,
.arena-rank-number {
  font-family: 'JetBrains Mono', monospace;
  font-weight: 800;
}

.arena-podium-card strong {
  font-family: 'JetBrains Mono', monospace;
  font-size: 20px;
}

.arena-podium-card span,
.arena-rank-tokens,
.arena-rank-reward,
.arena-quest-card span {
  color: var(--ink-2);
  font-size: 13px;
}

.tone-gold {
  border-color: rgba(184, 135, 25, 0.55);
  background: #fff4c7;
}

.tone-silver {
  border-color: rgba(109, 119, 136, 0.55);
  background: #eef2f6;
}

.tone-bronze {
  border-color: rgba(154, 91, 36, 0.55);
  background: #f8e1cc;
}

.dark .tone-gold {
  background: rgba(184, 135, 25, 0.18);
}

.dark .tone-silver {
  background: rgba(109, 119, 136, 0.22);
}

.dark .tone-bronze {
  background: rgba(154, 91, 36, 0.22);
}

.arena-rank-list {
  display: grid;
  gap: 10px;
}

.arena-rank-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 14px;
  padding: 12px 14px;
}

.arena-rank-row.current {
  box-shadow: inset 3px 0 0 #1f7a5b;
}

.arena-estimated-reward {
  margin: 12px 0 0;
  border-radius: 8px;
  background: rgba(31, 122, 91, 0.1);
  padding: 10px 12px;
  color: #1f7a5b;
  font-weight: 700;
}

.arena-formula-panel {
  display: grid;
  gap: 6px;
  margin-top: 14px;
  border-top: 1px solid var(--line);
  padding-top: 12px;
  color: var(--muted);
  font-size: 13px;
}

.arena-formula-panel strong {
  color: var(--text);
}

.arena-quest-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.arena-quest-card {
  position: relative;
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 14px;
}

.arena-quest-card.done {
  border-color: #b8d7c5;
  background: #edf7f1;
}

.dark .arena-quest-card.done {
  border-color: rgba(31, 122, 91, 0.55);
  background: rgba(31, 122, 91, 0.18);
}

.arena-quest-card p {
  margin: 0 0 4px;
  font-weight: 700;
}

.arena-quest-card strong,
.arena-quest-link {
  white-space: nowrap;
}

.arena-quest-link {
  position: absolute;
  right: 14px;
  bottom: 10px;
  color: #1f7a5b;
  font-size: 12px;
  font-weight: 700;
}

@media (max-width: 820px) {
  .arena-hero-grid,
  .arena-podium,
  .arena-quest-grid {
    grid-template-columns: 1fr;
  }

  .arena-podium-card,
  .arena-podium-card.champion {
    min-height: auto;
  }

  .arena-rank-row {
    grid-template-columns: 1fr;
    align-items: start;
  }

  .arena-rank-tokens,
  .arena-rank-reward {
    text-align: left;
  }
}
</style>
