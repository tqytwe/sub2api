<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import publicAPI, { type PublicModelPricingRow } from '@/api/public'
import playAPI, { type PlayVIPStatus } from '@/api/play'
import { useAuthStore } from '@/stores'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import '@/styles/public-pages.css'

const { t, te } = useI18n()
const authStore = useAuthStore()

const loading = ref(false)
const pricingRows = ref<PublicModelPricingRow[]>([])
const searchQuery = ref('')
const vip = ref<PlayVIPStatus | null>(null)
const loadError = ref(false)

const showVipBadge = computed(
  () => authStore.isAuthenticated && (vip.value?.perks?.includes('models_vip_tag') ?? false),
)

const guestPrimaryPath = computed(() => (te('home.jisudeng.cta.register') ? '/register' : '/login'))
const guestSecondaryPath = computed(() => (te('home.jisudeng.cta.register') ? '/login' : '/register'))
const guestPrimaryIsRegister = computed(() => te('home.jisudeng.cta.register'))

function formatPrice(value: number | null | undefined): string {
  if (value == null) return '—'
  return `$${value.toFixed(4)}`
}

function useCaseLabel(useCase: string): string {
  const key = `models.previewUseCases.${useCase}`
  return te(key) ? t(key) : t('models.previewUseCases.chat')
}

const filteredRows = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const rows = pricingRows.value
  if (!q) return rows
  return rows.filter(
    (row) =>
      row.name.toLowerCase().includes(q) ||
      row.platform.toLowerCase().includes(q) ||
      useCaseLabel(row.use_case).toLowerCase().includes(q),
  )
})

async function loadModels() {
  loading.value = true
  loadError.value = false
  vip.value = null
  try {
    if (authStore.isAuthenticated) {
      vip.value = (await playAPI.getPlayHub().catch(() => null))?.growth.vip ?? null
    }
    pricingRows.value = await publicAPI.getPublicModelPricing()
  } catch {
    loadError.value = true
    pricingRows.value = []
  } finally {
    loading.value = false
  }
}

onMounted(loadModels)
</script>

<template>
  <div class="models-page">
    <header class="public-page-header">
      <router-link to="/home" class="back-link">{{ t('models.backHome') }}</router-link>
      <PublicPageToolbar />
    </header>

    <main class="models-main">
      <p class="models-eyebrow">MODELS</p>
      <div class="models-title-row">
        <h1 class="models-title">{{ t('models.title') }}</h1>
        <span v-if="showVipBadge" class="models-vip-badge">
          {{ t('models.vipBadge', { label: vip?.label ?? 'VIP' }) }}
        </span>
      </div>
      <p class="models-subtitle">{{ t('models.subtitle') }}</p>
      <p class="models-preview-note">{{ t('models.priceUnitNote') }}</p>

      <div v-if="!authStore.isAuthenticated" class="models-auth-card">
        <p>{{ t('models.loginPrompt') }}</p>
        <div class="models-auth-actions">
          <router-link :to="guestPrimaryPath" class="models-btn models-btn-primary">{{ guestPrimaryIsRegister ? t('models.registerCta') : t('models.loginCta') }}</router-link>
          <router-link :to="guestSecondaryPath" class="models-btn models-btn-secondary">{{ guestPrimaryIsRegister ? t('models.loginCta') : t('models.registerCta') }}</router-link>
        </div>
      </div>

      <div class="models-toolbar">
        <input
          v-model="searchQuery"
          type="search"
          class="models-search"
          :placeholder="t('models.searchPlaceholder')"
        />
      </div>

      <div v-if="loading" class="models-state">{{ t('models.loading') }}</div>
      <div v-else-if="loadError" class="models-state">{{ t('models.loadFailed') }}</div>
      <div v-else-if="filteredRows.length === 0" class="models-state">{{ t('models.empty') }}</div>
      <div v-else class="models-table-wrap">
        <table class="models-table">
          <thead>
            <tr>
              <th>{{ t('models.columns.model') }}</th>
              <th>{{ t('models.columns.platform') }}</th>
              <th>{{ t('models.columns.useCase') }}</th>
              <th>{{ t('models.columns.officialInput') }}</th>
              <th>{{ t('models.columns.officialOutput') }}</th>
              <th>{{ t('models.columns.ourInput') }}</th>
              <th>{{ t('models.columns.ourOutput') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in filteredRows" :key="row.name">
              <td class="models-cell-name">{{ row.name }}</td>
              <td><span class="models-platform">{{ row.platform }}</span></td>
              <td>{{ useCaseLabel(row.use_case) }}</td>
              <td>{{ formatPrice(row.official_input_price) }}</td>
              <td>{{ formatPrice(row.official_output_price) }}</td>
              <td class="models-cell-our">{{ formatPrice(row.our_input_price) }}</td>
              <td class="models-cell-our">{{ formatPrice(row.our_output_price) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>

    <SupportFloatingCard />
  </div>
</template>

<style scoped>
.models-cell-our {
  font-weight: 600;
}
</style>
