<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, { type PlayArenaCurrent, type PlayArenaLeaderboard } from '@/api/play'
import '@/styles/public-pages.css'

const { t } = useI18n()
const authStore = useAuthStore()

const loading = ref(true)
const current = ref<PlayArenaCurrent | null>(null)
const leaderboard = ref<PlayArenaLeaderboard | null>(null)

const periodLabel = computed(() => {
  const p = current.value?.period || leaderboard.value?.period
  if (!p) return ''
  return p.name
})

async function load() {
  loading.value = true
  try {
    const [cur, board] = await Promise.all([
      playAPI.getArenaCurrent(),
      playAPI.getArenaLeaderboard(50),
    ])
    current.value = cur
    leaderboard.value = board
  } catch {
    current.value = null
    leaderboard.value = null
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="play-page">
    <header class="public-page-header">
      <PublicPlayBackLink />
      <PublicPageToolbar />
    </header>

    <main class="play-main">
      <p class="play-eyebrow">{{ t('play.arena.eyebrow') }}</p>
      <h1 class="play-title">{{ t('play.arena.title') }}</h1>
      <p class="play-subtitle">{{ t('play.arena.subtitle') }}</p>

      <div v-if="loading" class="play-note">{{ t('models.loading') }}</div>
      <div v-else-if="!current?.enabled" class="play-note">{{ t('arena.disabled') }}</div>
      <template v-else>
        <p v-if="periodLabel" class="play-intro">
          {{ t('arena.period', { name: periodLabel }) }}
        </p>

        <section v-if="authStore.isAuthenticated && (current?.rank || current?.token_sum)" class="play-section">
          <h2 class="play-section-title">{{ t('arena.myRank') }}</h2>
          <p v-if="current?.rank" class="play-intro">
            {{ t('arena.myStats', { rank: current.rank, tokens: (current.display_token_sum || current.token_sum || 0) }) }}
          </p>
          <p v-if="current?.recharge_boost_active" class="play-intro text-amber-700 dark:text-amber-300">
            {{ t('arena.boostActive', { mult: current.arena_score_multiplier || 1.5 }) }}
          </p>
          <p v-else-if="current?.token_sum" class="play-intro">
            {{ t('arena.myTokens', { tokens: current.token_sum }) }}
          </p>
          <p v-if="current?.tokens_to_prev_rank && current.tokens_to_prev_rank > 0" class="play-intro font-medium text-amber-700 dark:text-amber-300">
            {{ t('arena.gapToPrev', { gap: current.tokens_to_prev_rank.toLocaleString() }) }}
          </p>
          <router-link v-if="authStore.isAuthenticated" to="/keys" class="play-btn play-btn-primary mt-3 inline-flex">
            {{ t('playHub.actionUseApi') }}
          </router-link>
        </section>

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
                  <td class="px-4 py-3">{{ row.display_name }}</td>
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
      </template>

      <div class="play-actions">
        <router-link
          v-if="!authStore.isAuthenticated"
          to="/register"
          class="play-btn play-btn-primary"
        >
          {{ t('play.arena.ctaGuest') }}
        </router-link>
        <router-link v-else to="/dashboard" class="play-btn play-btn-primary">
          {{ t('play.arena.ctaAuth') }}
        </router-link>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>
