<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import PlayUserAvatar from '@/components/play/PlayUserAvatar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, {
  type PlayArenaCurrent,
  type PlayArenaLeaderboard,
  type PlayQuestToday,
} from '@/api/play'
import '@/styles/public-pages.css'
import '@/styles/arena-rpg.css'
import { trackGrowthEvent, trackQuestCompleteOnce } from '@/utils/growthAnalytics'

const { t, te } = useI18n()
const authStore = useAuthStore()

type BoardTab = 'daily' | 'monthly'

function readStringList(key: string): string[] {
  if (!te(key)) return []
  const val = t(key, { returnObjects: true }) as unknown
  return Array.isArray(val) ? val.filter((item): item is string => typeof item === 'string') : []
}

const loading = ref(true)
const tab = ref<BoardTab>('daily')
const monthlyCurrent = ref<PlayArenaCurrent | null>(null)
const dailyCurrent = ref<PlayArenaCurrent | null>(null)
const monthlyBoard = ref<PlayArenaLeaderboard | null>(null)
const dailyBoard = ref<PlayArenaLeaderboard | null>(null)
const quests = ref<PlayQuestToday | null>(null)

const current = computed(() => (tab.value === 'daily' ? dailyCurrent.value : monthlyCurrent.value))
const leaderboard = computed(() => (tab.value === 'daily' ? dailyBoard.value : monthlyBoard.value))
const periodLabel = computed(() => current.value?.period?.name || leaderboard.value?.period?.name || '')
const steps = computed(() => readStringList('play.arena.steps'))
const rules = computed(() => readStringList('play.arena.rules'))

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
</script>

<template>
  <div class="play-page arena-rpg-page">
    <header class="public-page-header">
      <PublicPlayBackLink />
      <PublicPageToolbar />
    </header>

    <main class="play-main">
      <p class="play-eyebrow">{{ t('play.arena.eyebrow') }}</p>
      <h1 class="play-title">{{ t('play.arena.title') }}</h1>
      <p class="play-subtitle">{{ t('play.arena.subtitle') }}</p>

      <div v-if="loading" class="play-note">{{ t('models.loading') }}</div>
      <div v-else-if="!monthlyCurrent?.enabled && !dailyCurrent?.enabled" class="play-note">{{ t('arena.disabled') }}</div>
      <template v-else>
        <section v-if="quests?.enabled" class="arena-rpg-hud">
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
          <div v-if="monthlyCurrent?.recharge_boost_active || monthlyCurrent?.campaign_active" class="arena-buff-row">
            <span v-if="monthlyCurrent?.recharge_boost_active" class="arena-buff">
              {{ t('arena.boostActive', { mult: monthlyCurrent.arena_score_multiplier || 1.5 }) }}
            </span>
            <span v-if="monthlyCurrent?.campaign_active" class="arena-buff">{{ t('arena.rpg.campaignBuff') }}</span>
          </div>
        </section>

        <section v-if="quests?.enabled && quests.tasks?.length" class="arena-quest-bar">
          <h2 class="play-section-title">{{ t('arena.rpg.dailyQuests') }}</h2>
          <div
            v-for="task in quests.tasks"
            :key="task.key"
            class="arena-quest-item"
            :class="{ done: task.completed }"
          >
            <span>{{ task.completed ? '☑' : '☐' }} {{ questLabel(task.key) }} (+{{ task.energy }})</span>
            <router-link v-if="!task.completed && task.cta_route" :to="task.cta_route" class="text-sm text-primary-600">
              {{ t('arena.rpg.go') }}
            </router-link>
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

        <section v-if="authStore.isAuthenticated" class="arena-field-card">
          <h2 class="play-section-title">{{ t('arena.rpg.mainField') }}</h2>
          <p class="play-intro">
            {{ t('arena.rpg.monthTokens', { tokens: (monthlyCurrent?.display_token_sum || monthlyCurrent?.token_sum || 0).toLocaleString() }) }}
          </p>
          <p v-if="monthlyCurrent?.rank" class="play-intro">
            {{ t('arena.myStats', { rank: monthlyCurrent.rank, tokens: (monthlyCurrent.display_token_sum || monthlyCurrent.token_sum || 0).toLocaleString() }) }}
          </p>
          <p v-if="monthlyCurrent?.tokens_to_prev_rank" class="play-intro font-medium text-amber-700 dark:text-amber-300">
            {{ t('arena.gapToPrev', { gap: monthlyCurrent.tokens_to_prev_rank.toLocaleString() }) }}
          </p>
          <p v-if="monthlyCurrent?.rank" class="play-intro">
            {{ t('arena.estimatedReward', { amount: Number(monthlyCurrent.estimated_reward || 0).toFixed(2) }) }}
          </p>
        </section>

        <div class="arena-rpg-tabs">
          <button type="button" class="arena-rpg-tab" :class="{ active: tab === 'daily' }" @click="switchTab('daily')">
            {{ t('arena.rpg.tabDaily') }}
          </button>
          <button type="button" class="arena-rpg-tab" :class="{ active: tab === 'monthly' }" @click="switchTab('monthly')">
            {{ t('arena.rpg.tabMonthly') }}
          </button>
        </div>

        <p v-if="periodLabel" class="play-intro">{{ t('arena.period', { name: periodLabel }) }}</p>

        <section class="play-section">
          <h2 class="play-section-title">{{ t('arena.leaderboard') }}</h2>
          <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-600">
            <table class="min-w-full text-sm">
              <thead class="bg-gray-50 text-left text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                <tr>
                  <th class="px-4 py-3">{{ t('arena.colRank') }}</th>
                  <th class="px-4 py-3">{{ t('arena.colUser') }}</th>
                  <th class="px-4 py-3">{{ t('arena.colTokens') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in leaderboard?.rows || []"
                  :key="row.user_id"
                  class="border-t border-gray-100 dark:border-dark-700"
                >
                  <td class="px-4 py-3 font-medium">#{{ row.rank }}</td>
                  <td class="px-4 py-3">
                    <PlayUserAvatar :name="row.display_name" :avatar-url="row.avatar_url" size-class="h-8 w-8" />
                  </td>
                  <td class="px-4 py-3">{{ row.token_sum.toLocaleString() }}</td>
                </tr>
                <tr v-if="!(leaderboard?.rows?.length)">
                  <td colspan="3" class="px-4 py-6 text-center text-gray-500 dark:text-dark-400">
                    {{ t('arena.empty') }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section v-if="steps.length" class="play-section">
          <h2 class="play-section-title">{{ t('play.howItWorks') }}</h2>
          <ol class="play-steps">
            <li v-for="(step, idx) in steps" :key="idx">{{ step }}</li>
          </ol>
        </section>

        <section v-if="rules.length" class="play-section">
          <h2 class="play-section-title">{{ t('play.arena.rulesTitle') }}</h2>
          <ul class="play-rules">
            <li v-for="(rule, idx) in rules" :key="idx">{{ rule }}</li>
          </ul>
        </section>
      </template>

      <div v-if="!authStore.isAuthenticated" class="play-actions">
        <router-link to="/register" class="play-btn play-btn-primary">{{ t('play.arena.ctaGuest') }}</router-link>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>
