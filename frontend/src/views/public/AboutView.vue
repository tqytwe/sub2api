<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import { isEnglishLocale } from '@/utils/localizedPublicSettings'
import '@/styles/about-view.css'
import '@/styles/public-pages.css'

const { t, tm, locale } = useI18n()

const isEnglish = computed(() => isEnglishLocale(locale.value))
const homeRoute = computed(() => (isEnglish.value ? '/en' : '/home'))
const docsRoute = computed(() =>
  isEnglish.value
    ? { name: 'EnglishDocs', query: { cat: 'tutorial', page: 'quick-start' } }
    : { name: 'Docs', query: { cat: 'tutorial', page: 'quick-start' } },
)
const vipRoute = computed(() =>
  isEnglish.value
    ? { name: 'EnglishDocs', query: { cat: 'recharge-vip', page: 'vip-levels' } }
    : '/blindbox',
)
const contactRoute = computed(() => (isEnglish.value ? '/en' : '/contact'))
const s5items = computed<string[]>(() => {
  const value = tm('about.s5items') as unknown
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
})
</script>

<template>
  <div class="about-page">
    <header class="about-header public-page-header">
      <router-link :to="homeRoute" class="back-link">{{ t('about.backHome') }}</router-link>
      <PublicPageToolbar />
    </header>
    <div class="about-header-intro">
      <p class="about-eyebrow">{{ t('about.eyebrow') }}</p>
      <h1 class="about-title">{{ t('about.title') }}</h1>
      <p class="about-lede">{{ t('about.lede') }}</p>
    </div>

    <main class="about-main">
      <section class="about-section">
        <p class="about-section-num">01</p>
        <h2 class="about-section-title">{{ t('about.s1Title') }}</h2>
        <p>{{ t('about.s1p1') }}</p>
        <p>{{ t('about.s1p2') }}</p>
        <p class="about-verify">{{ t('about.s1verify') }}</p>
      </section>

      <section class="about-section">
        <p class="about-section-num">02</p>
        <h2 class="about-section-title">{{ t('about.s2Title') }}</h2>
        <p>
          {{ t('about.s2p1') }}
        </p>
        <p>{{ t('about.s2p2') }}</p>
        <p>{{ t('about.s2p3') }}</p>
      </section>

      <section class="about-section about-section--accent">
        <p class="about-section-num">03</p>
        <h2 class="about-section-title">{{ t('about.s3Title') }}</h2>
        <p>{{ t('about.s3intro') }}</p>

        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti1Title') }}</h3>
          <p>{{ t('about.anti1p1') }}</p>
          <p>{{ t('about.anti1p2') }}</p>
        </div>
        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti2Title') }}</h3>
          <p>{{ t('about.anti2p1') }}</p>
        </div>
        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti3Title') }}</h3>
          <p>{{ t('about.anti3p1') }}</p>
        </div>
        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti4Title') }}</h3>
          <p>{{ t('about.anti4p1') }}</p>
          <p>{{ t('about.anti4p2') }}</p>
        </div>
        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti5Title') }}</h3>
          <p>{{ t('about.anti5p1') }}</p>
        </div>
      </section>

      <section class="about-section">
        <p class="about-section-num">04</p>
        <h2 class="about-section-title">{{ t('about.s4Title') }}</h2>
        <p>{{ t('about.s4p1') }}</p>
      </section>

      <section class="about-section">
        <p class="about-section-num">05</p>
        <h2 class="about-section-title">{{ t('about.s5Title') }}</h2>
        <ul class="about-list">
          <li v-for="(item, idx) in s5items" :key="idx">
            {{ item }}
          </li>
        </ul>
      </section>

      <section class="about-cta">
        <p>{{ t('about.ctaLead') }}</p>
        <div class="about-cta-row">
          <router-link :to="docsRoute" class="about-cta-link">{{ t('about.ctaDocs') }}</router-link>
          <router-link :to="vipRoute" class="about-cta-link">{{ t('about.ctaVip') }}</router-link>
          <router-link :to="contactRoute" class="about-cta-link about-cta-link--ghost">{{ t('about.ctaContact') }}</router-link>
        </div>
      </section>
    </main>

    <SupportFloatingCard />
  </div>
</template>
