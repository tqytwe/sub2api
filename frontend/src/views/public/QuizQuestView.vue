<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, { type PlayQuizToday } from '@/api/play'
import '@/styles/public-pages.css'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(true)
const submitting = ref(false)
const quiz = ref<PlayQuizToday | null>(null)
const choices = reactive<Record<number, number>>({})

const canSubmit = computed(
  () =>
    authStore.isAuthenticated &&
    quiz.value?.enabled &&
    !quiz.value.already_submitted &&
    !submitting.value &&
    quiz.value.questions.length > 0 &&
    quiz.value.questions.every((q) => typeof choices[q.id] === 'number' && choices[q.id] >= 0),
)

async function loadQuiz() {
  loading.value = true
  try {
    quiz.value = await playAPI.getQuizToday()
    for (const q of quiz.value.questions) {
      if (typeof choices[q.id] !== 'number') delete choices[q.id]
    }
  } catch {
    quiz.value = null
  } finally {
    loading.value = false
  }
}

async function handleSubmit() {
  if (!canSubmit.value || !quiz.value) return
  submitting.value = true
  try {
    const answers = quiz.value.questions.map((q) => ({
      question_id: q.id,
      choice_index: choices[q.id],
    }))
    const result = await playAPI.submitQuiz(answers)
    appStore.showSuccess(
      t('quiz.success', {
        score: result.score,
        total: result.total,
        reward: result.reward_amount.toFixed(2),
      }),
    )
    await authStore.refreshUser()
    await loadQuiz()
  } catch (err: unknown) {
    const code = (err as { response?: { data?: { code?: string } } })?.response?.data?.code
    if (code === 'PLAY_QUIZ_ALREADY_DONE') {
      appStore.showInfo(t('quiz.alreadyDone'))
      await loadQuiz()
      return
    }
    appStore.showError(t('quiz.failed'))
  } finally {
    submitting.value = false
  }
}

onMounted(loadQuiz)
</script>

<template>
  <div class="play-page">
    <header class="public-page-header">
      <PublicPlayBackLink />
      <PublicPageToolbar />
    </header>

    <main class="play-main">
      <div class="play-workspace">
        <section class="play-hero-panel">
          <div class="play-hero-grid">
            <div>
              <p class="play-eyebrow">{{ t('play.quizQuest.eyebrow') }}</p>
              <h1 class="play-title">{{ t('play.quizQuest.title') }}</h1>
              <p class="play-subtitle">{{ t('play.quizQuest.subtitle') }}</p>
            </div>
            <div class="play-action-panel">
              <h2 class="play-section-title">{{ t('nav.quizQuest') }}</h2>
              <p v-if="quiz?.enabled" class="play-intro">
                {{ t('quiz.rewardHint', { amount: quiz.reward_per_correct.toFixed(2) }) }}
              </p>
              <p v-else class="play-note">{{ loading ? t('models.loading') : t('quiz.disabled') }}</p>
              <div v-if="!authStore.isAuthenticated && quiz?.enabled" class="play-actions">
                <router-link to="/register" class="play-btn play-btn-primary">{{ t('play.quizQuest.ctaGuest') }}</router-link>
              </div>
            </div>
          </div>
        </section>

        <div v-if="loading" class="play-note">{{ t('models.loading') }}</div>
        <div v-else-if="!quiz?.enabled" class="play-note">{{ t('quiz.disabled') }}</div>
        <div v-else class="play-detail-grid">
          <section class="play-content-panel">
            <div v-if="quiz.already_submitted">
              <p class="play-intro">
                {{ t('quiz.done', { score: quiz.previous_score || 0, total: quiz.previous_total || 0, reward: (quiz.previous_reward || 0).toFixed(2) }) }}
              </p>
            </div>
            <div v-else-if="!authStore.isAuthenticated" class="play-actions">
              <router-link to="/register" class="play-btn play-btn-primary">{{ t('play.quizQuest.ctaGuest') }}</router-link>
            </div>
            <div v-else class="space-y-6">
              <div
                v-for="(q, idx) in quiz.questions"
                :key="q.id"
                class="rounded-xl border border-gray-200 p-4 dark:border-dark-600"
              >
                <p class="mb-3 font-medium text-gray-900 dark:text-white">
                  {{ idx + 1 }}. {{ q.prompt }}
                </p>
                <div class="space-y-2">
                  <label
                    v-for="(opt, optIdx) in q.options"
                    :key="optIdx"
                    class="flex cursor-pointer items-center gap-2 text-sm text-gray-700 dark:text-dark-200"
                  >
                    <input v-model="choices[q.id]" type="radio" :value="optIdx" />
                    <span>{{ opt }}</span>
                  </label>
                </div>
              </div>
              <button type="button" class="play-btn play-btn-primary" :disabled="!canSubmit" @click="handleSubmit">
                {{ submitting ? t('quiz.submitting') : t('quiz.submit') }}
              </button>
            </div>
          </section>

          <aside class="play-side-panel">
            <h2 class="play-section-title">{{ t('play.howItWorks') }}</h2>
            <div class="play-four-stat-grid">
              <div class="play-mini-stat">
                <span class="play-mini-label">{{ t('nav.quizQuest') }}</span>
                <span class="play-mini-value">{{ quiz.questions.length }}</span>
              </div>
              <div class="play-mini-stat">
                <span class="play-mini-label">{{ t('quiz.submit') }}</span>
                <span class="play-mini-value">{{ Object.keys(choices).length }}/{{ quiz.questions.length }}</span>
              </div>
            </div>
            <p class="play-note mt-4">
              {{ quiz.already_submitted
                ? t('quiz.done', { score: quiz.previous_score || 0, total: quiz.previous_total || 0, reward: (quiz.previous_reward || 0).toFixed(2) })
                : t('quiz.rewardHint', { amount: quiz.reward_per_correct.toFixed(2) }) }}
            </p>
          </aside>
        </div>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>
