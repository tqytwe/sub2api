<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import publicAPI, { type PublicModelPricingRow } from '@/api/public'
import modelPricingAPI, { type MyModelPricingRow } from '@/api/modelPricing'
import playAPI, { type PlayVIPStatus } from '@/api/play'
import { useAuthStore, useAppStore } from '@/stores'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import { formatScaled } from '@/utils/pricing'
import '@/styles/public-pages.css'

const { t, te } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(false)
const publicRows = ref<PublicModelPricingRow[]>([])
const authRows = ref<MyModelPricingRow[]>([])
const authPricingEnabled = ref(true)
const searchQuery = ref('')
const vip = ref<PlayVIPStatus | null>(null)
const loadError = ref(false)
const pricingDisabled = ref(false)

const isAuthMode = computed(() => authStore.isAuthenticated)

const showVipBadge = computed(
  () => authStore.isAuthenticated && (vip.value?.perks?.includes('models_vip_tag') ?? false),
)

const guestPrimaryPath = computed(() => (te('home.jisudeng.cta.register') ? '/register' : '/login'))
const guestSecondaryPath = computed(() => (te('home.jisudeng.cta.register') ? '/login' : '/register'))
const guestPrimaryIsRegister = computed(() => te('home.jisudeng.cta.register'))

const publicModelsEnabled = computed(
  () => appStore.cachedPublicSettings?.public_models_enabled ?? true,
)

function formatTokenPrice(value: number | null | undefined): string {
  if (value == null) return '—'
  return formatScaled(value, 1_000_000)
}

function useCaseLabel(useCase: string): string {
  const key = `models.previewUseCases.${useCase}`
  return te(key) ? t(key) : t('models.previewUseCases.chat')
}

function groupBadge(g: { name: string; rate_multiplier: number }): string {
  return `${g.name} ×${g.rate_multiplier}`
}

const filteredPublicRows = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const rows = publicRows.value
  if (!q) return rows
  return rows.filter(
    (row) =>
      row.name.toLowerCase().includes(q) ||
      row.platform.toLowerCase().includes(q) ||
      useCaseLabel(row.use_case).toLowerCase().includes(q),
  )
})

const filteredAuthRows = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const rows = authRows.value
  if (!q) return rows
  return rows.filter(
    (row) =>
      row.name.toLowerCase().includes(q) ||
      row.platform.toLowerCase().includes(q) ||
      (row.channel ?? '').toLowerCase().includes(q) ||
      row.groups.some((g) => g.name.toLowerCase().includes(q)),
  )
})

async function loadModels() {
  loading.value = true
  loadError.value = false
  pricingDisabled.value = false
  vip.value = null
  publicRows.value = []
  authRows.value = []

  try {
    if (authStore.isAuthenticated) {
      vip.value = (await playAPI.getPlayHub().catch(() => null))?.growth.vip ?? null
      const resp = await modelPricingAPI.getMyModelPricing()
      authPricingEnabled.value = resp.enabled
      if (!resp.enabled) {
        pricingDisabled.value = true
        return
      }
      authRows.value = resp.models ?? []
      return
    }

    if (!publicModelsEnabled.value) {
      pricingDisabled.value = true
      return
    }
    publicRows.value = await publicAPI.getPublicModelPricing()
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void appStore.fetchPublicSettings().finally(loadModels)
})
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
      <p class="models-subtitle">
        {{ isAuthMode ? t('models.subtitleAuth') : t('models.subtitle') }}
      </p>
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
      <div v-else-if="pricingDisabled" class="models-state">{{ t('models.disabled') }}</div>

      <!-- Guest / public catalog -->
      <div v-else-if="!isAuthMode" class="models-table-wrap">
        <div v-if="filteredPublicRows.length === 0" class="models-state">{{ t('models.empty') }}</div>
        <table v-else class="models-table">
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
            <tr v-for="row in filteredPublicRows" :key="row.name">
              <td class="models-cell-name">{{ row.name }}</td>
              <td><span class="models-platform">{{ row.platform }}</span></td>
              <td>{{ useCaseLabel(row.use_case) }}</td>
              <td>{{ formatTokenPrice(row.official_input_price) }}</td>
              <td>{{ formatTokenPrice(row.official_output_price) }}</td>
              <td class="models-cell-our">{{ formatTokenPrice(row.our_input_price) }}</td>
              <td class="models-cell-our">{{ formatTokenPrice(row.our_output_price) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Authenticated: effective pricing per group -->
      <div v-else class="models-table-wrap">
        <div v-if="filteredAuthRows.length === 0" class="models-state">{{ t('models.empty') }}</div>
        <table v-else class="models-table models-table-auth">
          <thead>
            <tr>
              <th>{{ t('models.columns.model') }}</th>
              <th>{{ t('models.columns.platform') }}</th>
              <th>{{ t('models.columns.channel') }}</th>
              <th>{{ t('models.columns.group') }}</th>
              <th>{{ t('models.columns.officialInput') }}</th>
              <th>{{ t('models.columns.officialOutput') }}</th>
              <th>{{ t('models.columns.channelInput') }}</th>
              <th>{{ t('models.columns.channelOutput') }}</th>
              <th>{{ t('models.columns.effectiveInput') }}</th>
              <th>{{ t('models.columns.effectiveOutput') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, idx) in filteredAuthRows" :key="`${row.name}-${row.platform}-${row.groups[0]?.id ?? idx}`">
              <td class="models-cell-name">{{ row.name }}</td>
              <td><span class="models-platform">{{ row.platform }}</span></td>
              <td>{{ row.channel || '—' }}</td>
              <td>
                <span
                  v-for="g in row.groups"
                  :key="g.id"
                  class="models-group-badge"
                >{{ groupBadge(g) }}</span>
              </td>
              <td>{{ formatTokenPrice(row.official_input_price) }}</td>
              <td>{{ formatTokenPrice(row.official_output_price) }}</td>
              <td>{{ formatTokenPrice(row.base_input_price) }}</td>
              <td>{{ formatTokenPrice(row.base_output_price) }}</td>
              <td class="models-cell-our">{{ formatTokenPrice(row.effective_input_price) }}</td>
              <td class="models-cell-our">{{ formatTokenPrice(row.effective_output_price) }}</td>
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

.models-group-badge {
  display: inline-block;
  margin-right: 0.35rem;
  margin-bottom: 0.15rem;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  font-size: 0.75rem;
  background: rgba(10, 10, 10, 0.06);
  color: #0a0a0a;
}

.models-table-auth {
  font-size: 0.875rem;
}
</style>
