<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { getAuthSessionIdentity, isAuthSessionCurrent } from '@/api/tokenRefresh'
import AuthenticatedPlayShell from '@/components/layout/AuthenticatedPlayShell.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode } from '@/utils/apiError'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import PublicPlayBackLink from '@/components/common/PublicPlayBackLink.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import CouponRewardCard from '@/components/play/CouponRewardCard.vue'
import playAPI, { type PlayGrowthEligibility, type PlayQuizSubmitResult, type PlayQuizToday } from '@/api/play'
import '@/styles/public-pages.css'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(false)
const loadError = ref('')
const submitting = ref(false)
const quiz = ref<PlayQuizToday | null>(null)
const choices = reactive<Record<number, number>>({})
const lastResult = ref<PlayQuizSubmitResult | null>(null)
let quizLoadRequest = 0
let quizSubmitRequest = 0
let quizRefreshDeferred = false
let quizAuthSession = getAuthSessionIdentity()

const growthEnergyMode = computed(() => quiz.value?.growth_eligibility?.reward_mode === 'energy')
const couponPoolReady = computed(() => growthEnergyMode.value || quiz.value?.coupon_pool_ready !== false)
const fullBalanceReward = computed(() =>
  (quiz.value?.questions.length ?? 0) * (quiz.value?.reward_per_correct ?? 0),
)
const answeredCount = computed(() =>
  quiz.value?.questions.filter((q) => typeof choices[q.id] === 'number').length ?? 0,
)

function growthProgressMessage(eligibility?: PlayGrowthEligibility) {
  const progress = eligibility?.progress
  if (!progress) return t('checkin.energyProgress')
  return t('checkin.energyProgress', {
    accountAge: progress.account_age_days,
    minimumAge: progress.minimum_account_age_days,
    recharge: progress.net_balance_recharge_30d.toFixed(2),
    minimumRecharge: progress.minimum_recharge_cny.toFixed(2),
  })
}
const completedCouponReward = computed(() => {
  if (lastResult.value?.reward_type === 'coupon' && lastResult.value.coupon) {
    return lastResult.value.coupon
  }
  if (quiz.value?.previous_reward_type === 'coupon' && quiz.value.previous_coupon) {
    return quiz.value.previous_coupon
  }
  return null
})
const completedRedeemCode = computed(() => {
  if (lastResult.value?.reward_type === 'redeem_code' && lastResult.value.redeem_code) {
    return lastResult.value.redeem_code
  }
  if (quiz.value?.previous_reward_type === 'redeem_code' && quiz.value.previous_redeem_code) {
    return quiz.value.previous_redeem_code
  }
  return null
})
const completedQuizSummary = computed(() => {
  const score = lastResult.value?.score ?? quiz.value?.previous_score ?? 0
  const total = lastResult.value?.total ?? quiz.value?.previous_total ?? 0
  const growthEnergy = lastResult.value?.growth_energy ?? quiz.value?.previous_growth_energy ?? 0
  if (growthEnergy > 0) {
    return t('quiz.energyDone', { score, total, amount: growthEnergy })
  }
  if (completedCouponReward.value) {
    return t('quiz.couponDone', { score, total })
  }
  if (completedRedeemCode.value) {
    return t('quiz.redeemDone', { score, total, code: completedRedeemCode.value.code })
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
    !loading.value &&
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

function scrollToQuestion(questionID: number) {
  document.getElementById(`quiz-question-${questionID}`)?.scrollIntoView({
    behavior: 'smooth',
    block: 'center',
  })
}

function resetQuizState() {
  quizLoadRequest += 1
  quizSubmitRequest += 1
  quizRefreshDeferred = false
  loading.value = false
  loadError.value = ''
  submitting.value = false
  quiz.value = null
  lastResult.value = null
  clearChoices()
}

function refreshQuiz() {
  if (!submitting.value) void loadQuiz()
}

async function loadQuiz(refreshBalance = true) {
  if (loading.value || !authStore.isAuthenticated) return
  quizRefreshDeferred = false
  const requestID = ++quizLoadRequest
  const sessionKey = quizSessionKey()
  loading.value = true
  loadError.value = ''
  try {
    const nextQuiz = await playAPI.getQuizToday()
    if (requestID !== quizLoadRequest || !isCurrentQuizSession(sessionKey)) return
    if (refreshBalance && nextQuiz.already_submitted) {
      try {
        await authStore.refreshUser()
      } catch {
        // A profile read cannot undo a settled reward. Refresh remains available.
      }
      if (requestID !== quizLoadRequest || !isCurrentQuizSession(sessionKey)) return
    }
    quiz.value = nextQuiz
    lastResult.value = null
    clearChoices()
  } catch (err: unknown) {
    if (requestID !== quizLoadRequest || !isCurrentQuizSession(sessionKey)) return
    quiz.value = null
    const status = (err as { status?: number })?.status
    loadError.value = status === 403 ? 'quiz.loadForbidden' : status === 0 ? 'quiz.loadNetworkFailed' : 'quiz.loadFailed'
  } finally {
    if (requestID === quizLoadRequest && isCurrentQuizSession(sessionKey)) {
      loading.value = false
    }
  }
}

async function handleSubmit() {
  if (!canSubmit.value || !quiz.value) return
  const requestID = ++quizSubmitRequest
  const userID = authStore.user?.id
  // A same-account token rotation does not change ownership of an in-flight POST.
  const isCurrentSubmit = () => requestID === quizSubmitRequest &&
    authStore.isAuthenticated && authStore.user?.id === userID
  submitting.value = true
  try {
    const answers = quiz.value.questions.map((q) => ({
      question_id: q.id,
      choice_index: choices[q.id],
    }))
    const result = await playAPI.submitQuiz(answers)
    if (!isCurrentSubmit()) return
    lastResult.value = result
    appStore.showSuccess(result.growth_energy && result.growth_energy > 0
      ? t('quiz.energySuccess', { score: result.score, total: result.total, amount: result.growth_energy })
      : result.reward_type === 'redeem_code' && result.redeem_code
        ? t('quiz.redeemDone', { score: result.score, total: result.total, code: result.redeem_code.code })
      : result.reward_type === 'coupon' && result.coupon
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
    if (!isCurrentSubmit()) return
    await loadQuiz(false)
  } catch (err: unknown) {
    if (!isCurrentSubmit()) return
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
    if (isCurrentSubmit()) {
      submitting.value = false
      if (quizRefreshDeferred) await loadQuiz()
    }
  }
}

onMounted(() => loadQuiz())

onBeforeUnmount(() => {
  quizLoadRequest += 1
  quizSubmitRequest += 1
})

watch(
  [() => authStore.isAuthenticated, () => authStore.user?.id, () => authStore.token],
  ([authenticated, userID], [wasAuthenticated, previousUserID]) => {
    const sameSession = isAuthSessionCurrent(quizAuthSession)
    quizAuthSession = getAuthSessionIdentity()
    if (authenticated && wasAuthenticated && userID === previousUserID && sameSession && submitting.value) {
      // Defer the read until this account's POST settles, so a pre-commit GET
      // cannot replace the final result or re-enable the submit action.
      quizLoadRequest += 1
      quizRefreshDeferred = true
      loading.value = false
      return
    }
    if (authenticated && wasAuthenticated && userID === previousUserID && sameSession) {
      // A token rotation keeps this account's in-progress answers. Only restart an
      // in-flight read whose token-bound response will now be discarded.
      if (loading.value) {
        quizLoadRequest += 1
        loading.value = false
        void loadQuiz()
      }
      return
    }
    resetQuizState()
    void loadQuiz()
  },
)
</script>

<template>
  <AuthenticatedPlayShell>
    <div class="play-page" :aria-busy="loading || submitting">
    <header v-if="!authStore.isAuthenticated" class="public-page-header">
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
              <button v-if="authStore.isAuthenticated" type="button" class="play-btn play-btn-secondary mt-3" data-testid="refresh-quiz" :disabled="loading || submitting" @click="refreshQuiz">{{ t('common.refresh') }}</button>
            </div>
            <div class="play-action-panel">
              <h2 class="play-section-title">{{ t('nav.quizQuest') }}</h2>
              <p v-if="!authStore.isAuthenticated" class="play-note">{{ t('quiz.loginHint') }}</p>
              <p v-else-if="loading" class="play-note">{{ t('models.loading') }}</p>
              <p v-else-if="loadError" class="play-note">{{ t(loadError) }}</p>
              <template v-else>
              <div v-if="quiz?.enabled" class="quiz-hero-progress">
                <div class="quiz-hero-progress__track">
                  <span :style="{ width: `${quiz.questions.length ? (answeredCount / quiz.questions.length) * 100 : 0}%` }" />
                </div>
                <span>{{ answeredCount }}/{{ quiz.questions.length }}</span>
              </div>
              <p v-if="quiz?.enabled && growthEnergyMode" class="play-intro">
                {{ t('quiz.energyHint') }}
              </p>
              <p v-if="quiz?.enabled && growthEnergyMode" class="play-note">
                {{ growthProgressMessage(quiz?.growth_eligibility) }}
              </p>
              <p v-else-if="quiz?.enabled && couponPoolReady" class="play-intro">
                {{ t('quiz.rewardHint', { amount: fullBalanceReward.toFixed(2) }) }}
              </p>
              <p v-else-if="quiz?.enabled" class="play-note">{{ t('quiz.couponPoolUnavailable') }}</p>
              <p v-else class="play-note">{{ t('quiz.disabled') }}</p>
              </template>
            </div>
          </div>
        </section>

        <div v-if="!authStore.isAuthenticated" class="play-note">
          <p>{{ t('quiz.loginHint') }}</p>
          <router-link to="/login?redirect=/quiz-quest" class="play-btn play-btn-primary mt-3">{{ t('home.login') }}</router-link>
        </div>
        <div v-else-if="loading" class="play-note">{{ t('models.loading') }}</div>
        <div v-else-if="loadError" class="play-note" role="alert">
          <p>{{ t(loadError) }}</p>
          <button type="button" class="play-btn play-btn-secondary mt-3" :disabled="loading || submitting" @click="refreshQuiz">{{ t('common.retry') }}</button>
        </div>
        <div v-else-if="!quiz?.enabled" class="play-note">{{ t('quiz.disabled') }}</div>
        <div v-else class="play-detail-grid">
          <section class="play-content-panel">
            <div v-if="quiz.already_submitted">
              <p class="play-intro">
                {{ completedQuizSummary }}
              </p>
              <CouponRewardCard v-if="completedCouponReward" :coupon="completedCouponReward" />
              <p v-if="completedRedeemCode" class="play-note">
                {{ t('quiz.redeemDone', { score: lastResult?.score ?? quiz.previous_score ?? 0, total: lastResult?.total ?? quiz.previous_total ?? 0, code: completedRedeemCode.code }) }}
              </p>
            </div>
            <div v-else-if="!couponPoolReady" class="play-note">{{ t('quiz.couponPoolUnavailable') }}</div>
            <p v-else-if="!quiz.questions.length" class="play-note">{{ t('quiz.empty') }}</p>
            <div v-else class="space-y-6">
              <div
                v-for="(q, idx) in quiz.questions"
                :key="q.id"
                :id="`quiz-question-${q.id}`"
                class="quiz-question-card"
              >
                <p class="quiz-question-card__title">
                  {{ idx + 1 }}. {{ q.prompt }}
                </p>
                <div class="quiz-options-grid">
                  <label
                    v-for="(opt, optIdx) in q.options"
                    :key="optIdx"
                    class="quiz-option"
                    :class="{ 'quiz-option--selected': choices[q.id] === optIdx }"
                  >
                    <input v-model="choices[q.id]" class="sr-only" type="radio" :value="optIdx" />
                    <span class="quiz-option__dot">{{ choices[q.id] === optIdx ? '✓' : '' }}</span>
                    <span>{{ String.fromCharCode(65 + optIdx) }}. {{ opt }}</span>
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
                <span class="play-mini-value">{{ answeredCount }}/{{ quiz.questions.length }}</span>
              </div>
            </div>
            <div v-if="!quiz.already_submitted" class="quiz-question-nav" :aria-label="t('nav.quizQuest')">
              <button
                v-for="(q, idx) in quiz.questions"
                :key="q.id"
                type="button"
                :class="{ 'quiz-question-nav__item--done': typeof choices[q.id] === 'number' }"
                @click="scrollToQuestion(q.id)"
              >
                {{ idx + 1 }}
              </button>
            </div>
            <p class="play-note mt-4">
              {{ quiz.already_submitted
                ? completedQuizSummary
                : growthEnergyMode
                  ? growthProgressMessage(quiz.growth_eligibility)
                : !couponPoolReady
                  ? t('quiz.couponPoolUnavailable')
                : t('quiz.rewardHint', { amount: fullBalanceReward.toFixed(2) }) }}
            </p>
          </aside>
        </div>
      </div>
    </main>

    <SupportFloatingCard v-if="!authStore.isAuthenticated" />
    </div>
  </AuthenticatedPlayShell>
</template>

<style scoped>
.quiz-hero-progress {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0 0 16px;
  color: var(--play-primary-strong);
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  font-weight: 700;
}

.quiz-hero-progress__track {
  height: 8px;
  flex: 1;
  overflow: hidden;
  border-radius: 999px;
  background: var(--play-primary-soft);
}

.quiz-hero-progress__track span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--play-primary);
  transition: width 180ms ease;
}

.quiz-question-card {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 18px;
  scroll-margin-top: 24px;
}

.quiz-question-card__title {
  margin: 0 0 14px;
  color: var(--ink);
  font-size: 15px;
  font-weight: 700;
}

.quiz-options-grid {
  display: grid;
  gap: 10px;
}

.quiz-option {
  display: flex;
  align-items: center;
  min-height: 46px;
  gap: 10px;
  border: 1px solid var(--line);
  border-radius: 7px;
  padding: 10px 12px;
  color: var(--ink-2);
  cursor: pointer;
  font-size: 14px;
}

.quiz-option:hover,
.quiz-option--selected {
  border-color: var(--play-primary);
  background: var(--play-primary-soft);
  color: var(--play-primary-strong);
}

.quiz-option__dot {
  display: inline-flex;
  width: 18px;
  height: 18px;
  flex: 0 0 18px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-strong);
  border-radius: 50%;
  color: var(--text-inverse);
  font-size: 11px;
}

.quiz-option--selected .quiz-option__dot {
  border-color: var(--play-primary);
  background: var(--play-primary);
}

.quiz-question-nav {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
}

.quiz-question-nav button {
  width: 40px;
  height: 36px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--card);
  color: var(--ink-2);
  cursor: pointer;
  font-weight: 700;
}

.quiz-question-nav button:hover {
  border-color: var(--play-primary);
  color: var(--play-primary-strong);
}

.quiz-question-nav .quiz-question-nav__item--done {
  border-color: var(--status-success-border);
  background: var(--status-success-surface);
  color: var(--status-success-text);
}
</style>
