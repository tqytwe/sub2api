<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import userChannelsAPI, { type UserAvailableChannel, type UserSupportedModel } from '@/api/channels'
import publicAPI from '@/api/public'
import playAPI, { type PlayVIPStatus } from '@/api/play'
import { useAuthStore } from '@/stores'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import { FEATURED_PUBLIC_MODELS } from '@/content/featured-models'
import '@/styles/public-pages.css'

interface ModelRow {
  name: string
  platform: string
  channel: string
  pricing: UserSupportedModel['pricing']
}

const { t, te } = useI18n()
const authStore = useAuthStore()

const loading = ref(false)
const showPreview = ref(false)
const showGuestLoginBanner = ref(false)
const channels = ref<UserAvailableChannel[]>([])
const searchQuery = ref('')
const vip = ref<PlayVIPStatus | null>(null)

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

function flattenModels(list: UserAvailableChannel[]): ModelRow[] {
  const rows: ModelRow[] = []
  const seen = new Set<string>()
  for (const ch of list) {
    for (const section of ch.platforms) {
      for (const model of section.supported_models) {
        const key = `${model.name}::${section.platform}::${ch.name}`
        if (seen.has(key)) continue
        seen.add(key)
        rows.push({
          name: model.name,
          platform: section.platform,
          channel: ch.name,
          pricing: model.pricing,
        })
      }
    }
  }
  return rows.sort((a, b) => a.name.localeCompare(b.name))
}

const modelRows = computed(() => flattenModels(channels.value))

const filteredRows = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return modelRows.value
  return modelRows.value.filter(
    (row) =>
      row.name.toLowerCase().includes(q) ||
      row.platform.toLowerCase().includes(q) ||
      row.channel.toLowerCase().includes(q),
  )
})

const filteredPreviewRows = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return FEATURED_PUBLIC_MODELS
  return FEATURED_PUBLIC_MODELS.filter(
    (row) =>
      row.name.toLowerCase().includes(q) ||
      row.platform.toLowerCase().includes(q) ||
      t(row.useCaseKey).toLowerCase().includes(q),
  )
})

async function loadModels() {
  loading.value = true
  showPreview.value = false
  showGuestLoginBanner.value = false
  vip.value = null
  try {
    if (authStore.isAuthenticated) {
      const [available, hub] = await Promise.all([
        userChannelsAPI.getAvailable(),
        playAPI.getPlayHub().catch(() => null),
      ])
      channels.value = available
      vip.value = hub?.growth.vip ?? null
      return
    }

    const publicChannels = await publicAPI.getPublicModels().catch(() => [])
    if (publicChannels.length > 0) {
      channels.value = publicChannels
      showGuestLoginBanner.value = true
      return
    }

    showPreview.value = true
    channels.value = []
  } catch (err: unknown) {
    const status = (err as { response?: { status?: number } })?.response?.status
    if (status === 401) {
      showPreview.value = true
    }
    channels.value = []
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

      <div v-if="showGuestLoginBanner" class="models-auth-card">
        <p>{{ t('models.loginForRates') }}</p>
        <div class="models-auth-actions">
          <router-link :to="guestPrimaryPath" class="models-btn models-btn-primary">{{ guestPrimaryIsRegister ? t('models.registerCta') : t('models.loginCta') }}</router-link>
          <router-link :to="guestSecondaryPath" class="models-btn models-btn-secondary">{{ guestPrimaryIsRegister ? t('models.loginCta') : t('models.registerCta') }}</router-link>
        </div>
      </div>

      <div v-else-if="showPreview" class="models-auth-card">
        <p>{{ t('models.loginPrompt') }}</p>
        <div class="models-auth-actions">
          <router-link :to="guestPrimaryPath" class="models-btn models-btn-primary">{{ guestPrimaryIsRegister ? t('models.registerCta') : t('models.loginCta') }}</router-link>
          <router-link :to="guestSecondaryPath" class="models-btn models-btn-secondary">{{ guestPrimaryIsRegister ? t('models.loginCta') : t('models.registerCta') }}</router-link>
        </div>
      </div>

      <template v-if="showPreview">
        <div class="models-toolbar">
          <input
            v-model="searchQuery"
            type="search"
            class="models-search"
            :placeholder="t('models.searchPlaceholder')"
          />
        </div>
        <h2 class="models-preview-title">{{ t('models.previewTitle') }}</h2>
        <p class="models-preview-note">{{ t('models.previewNote') }}</p>
        <div v-if="filteredPreviewRows.length === 0" class="models-state">{{ t('models.empty') }}</div>
        <div v-else class="models-table-wrap">
          <table class="models-table">
            <thead>
              <tr>
                <th>{{ t('models.previewColumns.model') }}</th>
                <th>{{ t('models.previewColumns.platform') }}</th>
                <th>{{ t('models.previewColumns.useCase') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in filteredPreviewRows" :key="row.name">
                <td class="models-cell-name">{{ row.name }}</td>
                <td><span class="models-platform">{{ row.platform }}</span></td>
                <td>{{ t(row.useCaseKey) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <template v-else>
        <div class="models-toolbar">
          <input
            v-model="searchQuery"
            type="search"
            class="models-search"
            :placeholder="t('models.searchPlaceholder')"
          />
        </div>

        <div v-if="loading" class="models-state">{{ t('models.loading') }}</div>
        <div v-else-if="filteredRows.length === 0" class="models-state">{{ t('models.empty') }}</div>
        <div v-else class="models-table-wrap">
          <table class="models-table">
            <thead>
              <tr>
                <th>{{ t('models.columns.model') }}</th>
                <th>{{ t('models.columns.platform') }}</th>
                <th>{{ t('models.columns.channel') }}</th>
                <th>{{ t('models.columns.input') }}</th>
                <th>{{ t('models.columns.output') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in filteredRows" :key="`${row.name}-${row.platform}-${row.channel}`">
                <td class="models-cell-name">{{ row.name }}</td>
                <td><span class="models-platform">{{ row.platform }}</span></td>
                <td>{{ row.channel }}</td>
                <td>
                  {{
                    row.pricing?.input_price != null
                      ? formatPrice(row.pricing.input_price)
                      : t('models.noPricing')
                  }}
                </td>
                <td>
                  {{
                    row.pricing?.output_price != null
                      ? formatPrice(row.pricing.output_price)
                      : t('models.noPricing')
                  }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </main>

    <SupportFloatingCard />
  </div>
</template>
