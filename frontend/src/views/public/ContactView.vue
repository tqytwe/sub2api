<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import '@/styles/public-pages.css'

const { t } = useI18n()
const authStore = useAuthStore()

const backTarget = computed(() => (authStore.isAuthenticated ? '/dashboard' : '/home'))
const backLabel = computed(() =>
  authStore.isAuthenticated ? t('contact.backDashboard') : t('contact.backHome')
)

const channels = [
  {
    key: 'telegram',
    href: 'https://t.me/jisudeng_official',
    glyph: '✈'
  },
  {
    key: 'qq',
    href: 'https://qm.qq.com/cgi-bin/qm/qr?k=placeholder',
    glyph: 'Q'
  },
  {
    key: 'wechat',
    href: '/contact/qq',
    glyph: '微'
  }
] as const
</script>

<template>
  <div class="contact-page">
    <header class="contact-header public-page-header">
      <router-link :to="backTarget" class="back-link">{{ backLabel }}</router-link>
      <PublicPageToolbar />
    </header>

    <main class="contact-main">
      <p class="contact-eyebrow">{{ t('contact.eyebrow') }}</p>
      <h1 class="contact-title">{{ t('contact.title') }}</h1>
      <p class="contact-desc">{{ t('contact.desc') }}</p>

      <div class="contact-grid">
        <a
          v-for="ch in channels"
          :key="ch.key"
          class="contact-card"
          :href="ch.href"
          :target="ch.key === 'wechat' ? undefined : '_blank'"
          rel="noopener noreferrer"
        >
          <span class="contact-card-glyph">{{ ch.glyph }}</span>
          <div class="contact-card-body">
            <h3 class="contact-card-name">{{ t(`contact.${ch.key}.name`) }}</h3>
            <p class="contact-card-handle">{{ t(`contact.${ch.key}.handle`) }}</p>
            <p class="contact-card-desc">{{ t(`contact.${ch.key}.desc`) }}</p>
          </div>
          <span class="contact-card-cta">
            {{ t(`contact.${ch.key}.cta`) }}
            <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6">
              <path d="M3 8h10M9 4l4 4-4 4" />
            </svg>
          </span>
        </a>
      </div>

      <p class="contact-footnote">{{ t('contact.footnote') }}</p>
    </main>

    <SupportFloatingCard />
  </div>
</template>
