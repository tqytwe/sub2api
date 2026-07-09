<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import playAPI, { type PlayBlindboxOpenResult, type PlayBlindboxStatus } from '@/api/play'
import '@/styles/public-pages.css'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(true)
const opening = ref(false)
const status = ref<PlayBlindboxStatus | null>(null)
const lastResult = ref<PlayBlindboxOpenResult | null>(null)

const canOpen = computed(
  () =>
    authStore.isAuthenticated &&
    status.value?.enabled &&
    status.value.can_open &&
    !opening.value,
)

async function loadStatus() {
  if (!authStore.isAuthenticated) {
    status.value = { enabled: false, cost_amount: 0, daily_limit: 0, opens_today: 0, can_open: false, server_date: '' }
    loading.value = false
    return
  }
  loading.value = true
  try {
    status.value = await playAPI.getBlindboxStatus()
  } catch {
    status.value = null
  } finally {
    loading.value = false
  }
}

async function handleOpen() {
  if (!canOpen.value) return
  opening.value = true
  try {
    lastResult.value = await playAPI.openBlindbox(`blindbox-${Date.now()}`)
    appStore.showSuccess(
      t('blindbox.success', {
        reward: lastResult.value.reward_amount.toFixed(2),
        net: lastResult.value.net_amount.toFixed(2),
      }),
    )
    await authStore.refreshUser()
    await loadStatus()
  } catch (err: unknown) {
    const code = (err as { response?: { data?: { code?: string } } })?.response?.data?.code
    if (code === 'INSUFFICIENT_BALANCE') {
      appStore.showError(t('blindbox.insufficientBalance'))
      return
    }
    if (code === 'PLAY_BLINDBOX_DAILY_LIMIT') {
      appStore.showInfo(t('blindbox.dailyLimit'))
      await loadStatus()
      return
    }
    appStore.showError(t('blindbox.failed'))
  } finally {
    opening.value = false
  }
}

onMounted(loadStatus)
</script>

<template>
  <div class="play-page">
    <header class="public-page-header">
      <router-link to="/home" class="back-link">{{ t('play.backHome') }}</router-link>
      <PublicPageToolbar />
    </header>

    <main class="play-main">
      <p class="play-eyebrow">{{ t('play.blindbox.eyebrow') }}</p>
      <h1 class="play-title">{{ t('play.blindbox.title') }}</h1>
      <p class="play-subtitle">{{ t('play.blindbox.subtitle') }}</p>
      <p class="play-intro">{{ t('play.blindbox.intro') }}</p>

      <div v-if="authStore.isAuthenticated" class="play-section space-y-4">
        <div v-if="loading" class="play-note">{{ t('models.loading') }}</div>
        <div v-else-if="!status?.enabled" class="play-note">{{ t('blindbox.disabled') }}</div>
        <template v-else>
          <p class="play-intro">
            {{ t('blindbox.costHint', { cost: status.cost_amount.toFixed(2), opens: status.opens_today, limit: status.daily_limit }) }}
          </p>
          <button type="button" class="play-btn play-btn-primary" :disabled="!canOpen" @click="handleOpen">
            {{ opening ? t('blindbox.opening') : t('blindbox.openButton') }}
          </button>
          <p v-if="lastResult" class="play-note">
            {{ t('blindbox.lastResult', { reward: lastResult.reward_amount.toFixed(2), net: lastResult.net_amount.toFixed(2) }) }}
          </p>
        </template>
      </div>

      <div class="play-actions">
        <router-link
          v-if="!authStore.isAuthenticated"
          to="/register"
          class="play-btn play-btn-primary"
        >
          {{ t('play.blindbox.ctaGuest') }}
        </router-link>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>
