<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode } from '@/utils/apiError'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import CouponRewardCard from '@/components/play/CouponRewardCard.vue'
import playAPI, { type PlayQuizSubmitResult, type PlayQuizToday } from '@/api/play'
import '@/styles/public-pages.css'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(true)
const submitting = ref(false)
const quiz = ref<PlayQuizToday | null>(null)
const choices = reactive<Record<number, number>>({})
const lastResult = ref<PlayQuizSubmitResult | null>(null)
let quizLoadRequest = 0
let quizSubmitRequest = 0

const couponPoolReady = computed(() => quiz.value?.coupon_pool_ready !== false)
const fullBalanceReward = computed(() =>
  (quiz.value?.questions.length ?? 0) * (quiz.value?.reward_per_correct ?? 0),
)
const completedCouponReward = computed(() => {
  if (lastResult.value?.reward_type === 'coupon' && lastResult.value.coupon) {
    return lastResult.value.coupon
  }
  if (quiz.value?.previous_reward_type === 'coupon' && quiz.value.previous_coupon) {
    return quiz.value.previous_coupon
  }
  return null
})
const completedQuizSummary = computed(() => {
  const score = quiz.value?.previous_score || 0
  const total = quiz.value?.previous_total || 0
  if (completedCouponReward.value) {
    return t('quiz.couponDone', { score, total })
  }
  return t('quiz.done', {
    score,
    total,
    reward: (quiz.value?.previous_reward || 0).toFixed(2),
  })
})

const canSubmit = computed(
  () =>
    authStore.isAuthenticated &&
    quiz.value?.enabled &&
    couponPoolReady.value &&
    !quiz.value.already_submitted &&
    !submitting.value &&
    quiz.value.questions.length > 0 &&
    quiz.value.questions.every((q) => typeof choices[q.id] === 'number' && choices[q.id] >= 0),
)

function quizSessionKey(): string {
  if (!authStore.isAuthenticated) return 'guest'
  return `user:${authStore.user?.id ?? ''}:token:${authStore.token ?? ''}`
}

function isCurrentQuizSession(sessionKey: string): boolean {
  return sessionKey === quizSessionKey()
}

function clearChoices() {
  for (const questionID of Object.keys(choices)) {
    delete choices[Number(questionID)]
  }
}

function resetQuizState() {
  quizLoadRequest += 1
  quizSubmitRequest += 1
  loading.value = false
  submitting.value = false
  quiz.value = null
  lastResult.value = null
  clearChoices()
}

async function loadQuiz() {
  const requestID = ++quizLoadRequest
  const sessionKey = quizSessionKey()
  loading.value = true
  try {
    const nextQuiz = await playAPI.getQuizToday()
    if (requestID !== quizLoadRequest || !isCurrentQuizSession(sessionKey)) return
    quiz.value = nextQuiz
    for (const q of nextQuiz.questions) {
      if (typeof choices[q.id] !== 'number') delete choices[q.id]
    }
  } catch {
    if (requestID !== quizLoadRequest || !isCurrentQuizSession(sessionKey)) return
    quiz.value = null
  } finally {
    if (requestID === quizLoadRequest && isCurrentQuizSession(sessionKey)) {
      loading.value = false
    }
  }
}

async function handleSubmit() {
  if (!canSubmit.value || !quiz.value) return
  const requestID = ++quizSubmitRequest
  const sessionKey = quizSessionKey()
  submitting.value = true
  try {
    const answers = quiz.value.questions.map((q) => ({
      question_id: q.id,
      choice_index: choices[q.id],
    }))
    const result = await playAPI.submitQuiz(answers)
    if (requestID !== quizSubmitRequest || !isCurrentQuizSession(sessionKey)) return
    lastResult.value = result
    appStore.showSuccess(result.reward_type === 'coupon' && result.coupon
      ? t('coupon.reward.issued', { name: result.coupon.name })
      : t('quiz.success', {
          score: result.score,
          total: result.total,
          reward: result.reward_amount.toFixed(2),
        }))
    try {
      await authStore.refreshUser()
    } catch {
      // The result is already settled. Continue with the authoritative quiz state.
    }
    if (requestID !== quizSubmitRequest || !isCurrentQuizSession(sessionKey)) return
    await loadQuiz()
  } catch (err: unknown) {
    if (requestID !== quizSubmitRequest || !isCurrentQuizSession(sessionKey)) return
    const code = extractApiErrorCode(err)
    if (code === 'PLAY_QUIZ_ALREADY_DONE') {
      appStore.showInfo(t('quiz.alreadyDone'))
      await loadQuiz()
      return
    }
    if (code === 'COUPON_REWARD_POOL_UNAVAILABLE') {
      appStore.showInfo(t('quiz.couponPoolUnavailable'))
      await loadQuiz()
      return
    }
    appStore.showError(t('quiz.failed'))
  } finally {
    if (requestID === quizSubmitRequest && isCurrentQuizSession(sessionKey)) {
      submitting.value = false
    }
  }
}

onMounted(loadQuiz)

onBeforeUnmount(() => {
  quizLoadRequest += 1
  quizSubmitRequest += 1
})

watch(
  () => [authStore.isAuthenticated, authStore.user?.id, authStore.token] as const,
  () => {
    resetQuizState()
    void loadQuiz()
  },
)
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
              <p v-if="quiz?.enabled && couponPoolReady" class="play-intro">
                {{ t('quiz.rewardHint', { amount: fullBalanceReward.toFixed(2) }) }}
              </p>
              <p v-else-if="quiz?.enabled" class="play-note">{{ t('quiz.couponPoolUnavailable') }}</p>
              <p v-else class="play-note">{{ loading ? t('models.loading') : t('quiz.disabled') }}</p>
              <div v-if="!authStore.isAuthenticated && quiz?.enabled && couponPoolReady" class="play-actions">
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
                {{ completedQuizSummary }}
              </p>
              <CouponRewardCard v-if="completedCouponReward" :coupon="completedCouponReward" />
            </div>
            <div v-else-if="!couponPoolReady" class="play-note">{{ t('quiz.couponPoolUnavailable') }}</div>
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
                ? completedQuizSummary
                : !couponPoolReady
                  ? t('quiz.couponPoolUnavailable')
                : t('quiz.rewardHint', { amount: fullBalanceReward.toFixed(2) }) }}
            </p>
          </aside>
        </div>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>
