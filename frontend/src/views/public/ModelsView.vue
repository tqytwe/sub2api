<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import publicAPI, { type PublicModelPricingRow } from '@/api/public'
import modelPricingAPI, { type MyModelPricingRow } from '@/api/modelPricing'
import playAPI, { type PlayVIPStatus } from '@/api/play'
import { useAuthStore, useAppStore } from '@/stores'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import { formatScaled } from '@/utils/pricing'
import { vipTierBadgeClass } from '@/utils/vipColors'
import '@/styles/public-pages.css'

const { t, te } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(false)
const publicRows = ref<PublicModelPricingRow[]>([])
const authRows = ref<MyModelPricingRow[]>([])
const authPricingEnabled = ref(true)
const searchQuery = ref('')
const vip = ref<PlayVIPStatus | null>(null)
const emptyState = ref<'none' | 'disabled' | 'empty' | 'error' | 'not_deployed'>('none')

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

const stateMessage = computed(() => {
  switch (emptyState.value) {
    case 'disabled':
      return t('models.disabled')
    case 'empty':
      return isAuthMode.value ? t('models.emptyNoChannels') : t('models.empty')
    case 'not_deployed':
      return t('models.emptyApiNotDeployed')
    case 'error':
      return t('models.loadFailed')
    default:
      return ''
  }
})

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

function goBack() {
  if (window.history.length > 1) {
    router.back()
    return
  }
  router.push(isAuthMode.value ? '/dashboard' : '/home')
}

async function loadModels() {
  loading.value = true
  emptyState.value = 'none'
  vip.value = null
  publicRows.value = []
  authRows.value = []

  try {
    if (authStore.isAuthenticated) {
      vip.value = (await playAPI.getPlayHub().catch(() => null))?.growth.vip ?? null
      const resp = await modelPricingAPI.getMyModelPricing()
      authPricingEnabled.value = resp.enabled
      if (!resp.enabled) {
        emptyState.value = 'disabled'
        return
      }
      authRows.value = resp.models ?? []
      if (authRows.value.length === 0) {
        emptyState.value = 'empty'
      }
      return
    }

    if (!publicModelsEnabled.value) {
      emptyState.value = 'disabled'
      return
    }
    publicRows.value = await publicAPI.getPublicModelPricing()
    if (publicRows.value.length === 0) {
      emptyState.value = 'empty'
    }
  } catch (err: unknown) {
    const status = (err as { status?: number })?.status
    emptyState.value = status === 404 ? 'not_deployed' : 'error'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void appStore.fetchPublicSettings().finally(loadModels)
})
</script>

<template>
  <AppLayout v-if="isAuthMode">
    <div class="models-app-page space-y-6">
      <div>
        <p class="models-eyebrow-app">MODELS</p>
        <div class="models-title-row">
          <h1 class="models-title-app">{{ t('models.title') }}</h1>
          <span v-if="showVipBadge" :class="vipTierBadgeClass(vip?.color_key)">
            {{ t('models.vipBadge', { label: vip?.label ?? 'VIP' }) }}
          </span>
        </div>
        <p class="models-subtitle-app">{{ t('models.subtitleAuth') }}</p>
        <p class="models-preview-note-app">{{ t('models.priceUnitNote') }}</p>
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
      <div v-else-if="emptyState !== 'none'" class="models-state">{{ stateMessage }}</div>

      <div v-else class="models-table-wrap">
        <div v-if="filteredAuthRows.length === 0" class="models-state">{{ t('models.empty') }}</div>
        <table v-else class="models-table models-table-auth">
          <thead>
            <tr>
              <th>{{ t('models.columns.model') }}</th>
              <th>{{ t('models.columns.platform') }}</th>
              <th>{{ t('models.columns.ourInput') }}</th>
              <th>{{ t('models.columns.ourOutput') }}</th>
              <th>{{ t('models.columns.group') }}</th>
              <th>{{ t('models.columns.effectiveInput') }}</th>
              <th>{{ t('models.columns.effectiveOutput') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, idx) in filteredAuthRows" :key="`${row.name}-${row.platform}-${row.groups[0]?.id ?? idx}`">
              <td class="models-cell-name">{{ row.name }}</td>
              <td><span class="models-platform">{{ row.platform }}</span></td>
              <td class="models-cell-our">{{ formatTokenPrice(row.site_input_price) }}</td>
              <td class="models-cell-our">{{ formatTokenPrice(row.site_output_price) }}</td>
              <td>
                <span
                  v-for="g in row.groups"
                  :key="g.id"
                  class="models-group-badge"
                >{{ groupBadge(g) }}</span>
                <span v-if="row.groups.length === 0">—</span>
              </td>
              <td class="models-cell-our">{{ formatTokenPrice(row.effective_input_price) }}</td>
              <td class="models-cell-our">{{ formatTokenPrice(row.effective_output_price) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </AppLayout>

  <div v-else class="models-page">
    <header class="public-page-header">
      <button type="button" class="back-link" @click="goBack">{{ t('models.backHome') }}</button>
      <PublicPageToolbar />
    </header>

    <main class="models-main">
      <p class="models-eyebrow">MODELS</p>
      <div class="models-title-row">
        <h1 class="models-title">{{ t('models.title') }}</h1>
      </div>
      <p class="models-subtitle">{{ t('models.subtitle') }}</p>
      <p class="models-preview-note">{{ t('models.priceUnitNote') }}</p>

      <div class="models-auth-card">
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
      <div v-else-if="emptyState !== 'none'" class="models-state">{{ stateMessage }}</div>

      <div v-else class="models-table-wrap">
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

:global(.dark) .models-group-badge {
  background: rgba(255, 255, 255, 0.1);
  color: #f5f5f5;
}

.models-table-auth {
  font-size: 0.875rem;
}

.models-app-page .models-table-wrap {
  overflow-x: auto;
}

.models-app-page .models-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.models-app-page .models-table th,
.models-app-page .models-table td {
  padding: 0.6rem 0.75rem;
  text-align: left;
  border-bottom: 1px solid rgba(10, 10, 10, 0.08);
}

:global(.dark) .models-app-page .models-table th,
:global(.dark) .models-app-page .models-table td {
  border-bottom-color: rgba(255, 255, 255, 0.08);
}

.models-eyebrow-app {
  font-size: 0.75rem;
  letter-spacing: 0.12em;
  color: #737373;
  margin-bottom: 0.25rem;
}

.models-title-app {
  font-size: 1.75rem;
  font-weight: 700;
  color: #0a0a0a;
}

:global(.dark) .models-title-app {
  color: #fafafa;
}

.models-subtitle-app,
.models-preview-note-app {
  color: #525252;
  margin-top: 0.35rem;
}

.models-search {
  width: 100%;
  max-width: 24rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid rgba(10, 10, 10, 0.12);
  border-radius: 0.5rem;
}

.models-state {
  padding: 2rem 0;
  color: #737373;
  text-align: center;
}

.back-link {
  background: none;
  border: none;
  cursor: pointer;
  font: inherit;
  color: inherit;
  text-decoration: underline;
}
</style>
