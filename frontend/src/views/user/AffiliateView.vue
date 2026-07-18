<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/growth-world.css'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

function formatCount(value: number): string {
  return value.toLocaleString()
}

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

onMounted(() => {
  void loadAffiliateDetail()
})
</script>

<template>
  <AppLayout>
    <div class="gw-page gw-page--wide gw-workspace pb-10">
      <section class="gw-hero-panel">
        <p class="gw-eyebrow">{{ t('affiliate.eyebrow') }}</p>
        <h1 class="gw-title">{{ t('affiliate.title') }}</h1>
        <p class="gw-subtitle">{{ t('affiliate.description') }}</p>
      </section>

      <div v-if="loading" class="gw-polling py-12 text-center">{{ t('models.loading') }}</div>

      <template v-else-if="detail">
        <div class="gw-stat-grid">
          <div class="gw-stat-card">
            <p class="gw-balance-label">{{ t('affiliate.stats.rebateRate') }}</p>
            <p class="gw-stat-value">{{ formattedRebateRate }}<span class="text-base">%</span></p>
            <p class="gw-subtitle text-xs mt-1">{{ t('affiliate.stats.rebateRateHint') }}</p>
          </div>
          <div class="gw-stat-card">
            <p class="gw-balance-label">{{ t('affiliate.stats.invitedUsers') }}</p>
            <p class="gw-stat-value">{{ formatCount(detail.aff_count) }}</p>
          </div>
          <div class="gw-stat-card">
            <p class="gw-balance-label">{{ t('affiliate.stats.availableQuota') }}</p>
            <p class="gw-stat-value ok">{{ formatCurrency(detail.aff_quota) }}</p>
          </div>
          <div class="gw-stat-card">
            <p class="gw-balance-label">{{ t('affiliate.stats.totalQuota') }}</p>
            <p class="gw-stat-value">{{ formatCurrency(detail.aff_history_quota) }}</p>
            <p v-if="detail.aff_frozen_quota > 0" class="gw-subtitle text-xs mt-1" style="color: var(--gw-warn)">
              {{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}
            </p>
          </div>
        </div>

        <div class="gw-detail-grid">
          <div class="gw-panel">
            <div class="grid gap-4 md:grid-cols-2">
              <div class="gw-field">
                <span class="gw-field-label">{{ t('affiliate.yourCode') }}</span>
                <div class="gw-code-row">
                  <code>{{ detail.aff_code }}</code>
                  <button type="button" class="gw-btn gw-btn-secondary" @click="copyCode">
                    <Icon name="copy" size="sm" />
                    <span>{{ t('affiliate.copyCode') }}</span>
                  </button>
                </div>
              </div>
              <div class="gw-field">
                <span class="gw-field-label">{{ t('affiliate.inviteLink') }}</span>
                <div class="gw-code-row">
                  <code>{{ inviteLink }}</code>
                  <button type="button" class="gw-btn gw-btn-secondary" @click="copyInviteLink">
                    <Icon name="copy" size="sm" />
                    <span>{{ t('affiliate.copyLink') }}</span>
                  </button>
                </div>
              </div>
            </div>

            <div class="gw-tips">
              <p class="gw-section-title text-base">{{ t('affiliate.tips.title') }}</p>
              <ul class="mt-2 space-y-1 gw-subtitle text-sm">
                <li>1. {{ t('affiliate.tips.line1') }}</li>
                <li>2. {{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
                <li>3. {{ t('affiliate.tips.line3') }}</li>
                <li v-if="detail.aff_frozen_quota > 0">4. {{ t('affiliate.tips.line4') }}</li>
              </ul>
            </div>
          </div>

          <div class="gw-panel">
            <div class="flex flex-col gap-3 sm:items-start">
              <div>
                <h3 class="gw-section-title">{{ t('affiliate.transfer.title') }}</h3>
                <p class="gw-subtitle">{{ t('affiliate.transfer.description') }}</p>
              </div>
              <button
                type="button"
                class="gw-btn gw-btn-primary"
                :disabled="transferring || detail.aff_quota <= 0"
                @click="transferQuota"
              >
                <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
                <Icon v-else name="dollar" size="sm" />
                <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
              </button>
            </div>
            <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm" style="color: var(--gw-warn)">
              {{ t('affiliate.transfer.empty') }}
            </p>
          </div>
        </div>

        <div class="gw-panel">
          <h3 class="gw-section-title">{{ t('affiliate.invitees.title') }}</h3>
          <div v-if="detail.invitees.length === 0" class="mt-4 gw-subtitle text-center py-8 border border-dashed rounded-xl" style="border-color: var(--gw-line)">
            {{ t('affiliate.invitees.empty') }}
          </div>
          <div v-else class="gw-table-wrap mt-4 overflow-x-auto">
            <table class="gw-leaderboard min-w-[560px]">
              <thead>
                <tr>
                  <th>{{ t('affiliate.invitees.columns.email') }}</th>
                  <th>{{ t('affiliate.invitees.columns.username') }}</th>
                  <th class="text-right">{{ t('affiliate.invitees.columns.rebate') }}</th>
                  <th>{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in detail.invitees" :key="item.user_id">
                  <td>{{ item.email || '-' }}</td>
                  <td>{{ item.username || '-' }}</td>
                  <td class="text-right font-medium" style="color: var(--gw-ok)">{{ formatCurrency(item.total_rebate) }}</td>
                  <td>{{ formatDateTime(item.created_at) || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>
