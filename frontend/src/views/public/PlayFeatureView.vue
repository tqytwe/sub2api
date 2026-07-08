<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import { findPlayFeature, type PlayFeatureId } from '@/content/play-features'
import '@/styles/public-pages.css'

const route = useRoute()
const { t, tm, te } = useI18n()
const authStore = useAuthStore()

const featureId = computed(() => {
  const fromMeta = route.meta.playFeature
  if (typeof fromMeta === 'string') return fromMeta as PlayFeatureId
  return ''
})

const feature = computed(() => (featureId.value ? findPlayFeature(featureId.value) : undefined))

const i18nBase = computed(() => {
  if (featureId.value === 'quiz-quest') return 'play.quizQuest'
  if (featureId.value === 'agent-team') return 'play.agentTeam'
  return featureId.value ? `play.${featureId.value}` : ''
})

const steps = computed(() => {
  if (!i18nBase.value) return [] as string[]
  const key = `${i18nBase.value}.steps`
  if (!te(key)) return []
  const val = tm(key)
  return Array.isArray(val) ? (val as string[]) : []
})

const rules = computed(() => {
  if (!i18nBase.value) return [] as string[]
  const key = `${i18nBase.value}.rules`
  if (!te(key)) return []
  const val = tm(key)
  return Array.isArray(val) ? (val as string[]) : []
})

const ctaTarget = computed(() =>
  authStore.isAuthenticated ? '/dashboard' : '/login',
)
</script>

<template>
  <div v-if="feature && i18nBase" class="play-page">
    <header class="public-page-header">
      <router-link to="/home" class="back-link">{{ t('play.backHome') }}</router-link>
      <PublicPageToolbar />
    </header>

    <main class="play-main">
      <p class="play-eyebrow">{{ t(`${i18nBase}.eyebrow`) }}</p>
      <h1 class="play-title">{{ t(`${i18nBase}.title`) }}</h1>
      <p class="play-subtitle">{{ t(`${i18nBase}.subtitle`) }}</p>
      <p class="play-intro">{{ t(`${i18nBase}.intro`) }}</p>

      <section v-if="steps.length" class="play-section">
        <h2 class="play-section-title">{{ t('play.howItWorks') }}</h2>
        <ol class="play-steps">
          <li v-for="(step, idx) in steps" :key="idx">{{ step }}</li>
        </ol>
      </section>

      <section v-if="rules.length" class="play-section">
        <h2 class="play-section-title">{{ t(`${i18nBase}.rulesTitle`) }}</h2>
        <ul class="play-rules">
          <li v-for="(rule, idx) in rules" :key="idx">{{ rule }}</li>
        </ul>
      </section>

      <p class="play-note">{{ t(`${i18nBase}.statusNote`) }}</p>

      <div class="play-actions">
        <router-link :to="ctaTarget" class="play-btn play-btn-primary">
          {{ authStore.isAuthenticated ? t(`${i18nBase}.ctaAuth`) : t(`${i18nBase}.ctaGuest`) }}
        </router-link>
        <router-link to="/docs?cat=vip&page=check-in" class="play-btn play-btn-secondary">
          {{ t('play.learnMore') }}
        </router-link>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>
