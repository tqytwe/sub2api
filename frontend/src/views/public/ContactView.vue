<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import SupportContactPanel from '@/components/common/SupportContactPanel.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import '@/styles/public-pages.css'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const backTarget = computed(() => (authStore.isAuthenticated ? '/dashboard' : '/home'))
const backLabel = computed(() =>
  authStore.isAuthenticated ? t('contact.backDashboard') : t('contact.backHome')
)

onMounted(() => {
  void appStore.fetchPublicSettings(true)
})
</script>

<template>
  <div class="contact-page">
    <header class="contact-header public-page-header">
      <router-link :to="backTarget" class="back-link">{{ backLabel }}</router-link>
      <PublicPageToolbar />
    </header>

    <main class="contact-main">
      <div class="contact-copy">
        <p class="contact-eyebrow">{{ t('contact.eyebrow') }}</p>
        <h1 class="contact-title">{{ t('contact.title') }}</h1>
        <p class="contact-desc">{{ t('contact.desc') }}</p>
        <p class="contact-footnote">{{ t('contact.footnote') }}</p>
      </div>

      <SupportContactPanel
        class="contact-support-panel"
        :config="appStore.supportContact"
      />
    </main>

    <SupportFloatingCard />
  </div>
</template>
