<template>
  <header
    class="glass sticky top-0 z-30 border-b border-gray-200/50 dark:border-dark-700/50"
  >
    <div class="public-content-frame flex items-center justify-between gap-4 px-4 py-3.5 sm:px-6 lg:px-8">
      <!-- 左:站点 logo + 名称 -->
      <div class="flex min-w-0 items-center gap-3">
        <template v-if="settings">
          <span
            class="flex h-9 w-9 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700"
          >
            <img
              :src="siteLogo || '/logo.png'"
              :alt="siteName"
              :class="['brand-logo-asset', { 'brand-logo-asset--deng': !siteLogo }, 'h-full w-full object-contain']"
              data-testid="model-plaza-logo"
            />
          </span>
          <span class="truncate text-base font-semibold text-gray-950 dark:text-white">
            {{ siteName }}
          </span>
        </template>
        <template v-else>
          <!-- design-governance-allow: continuous-motion - public settings skeleton is transient and hidden after the settings request resolves. -->
          <span class="h-9 w-9 flex-shrink-0 animate-pulse rounded-lg bg-gray-200 dark:bg-dark-700" aria-hidden="true"></span>
          <!-- design-governance-allow: continuous-motion - public settings skeleton is transient and hidden after the settings request resolves. -->
          <span class="h-5 w-28 animate-pulse rounded bg-gray-200 dark:bg-dark-700" aria-hidden="true"></span>
        </template>
      </div>

      <!-- 右:登录 / 回到后台 -->
      <RouterLink
        v-if="isAuthenticated"
        :to="backTarget"
        class="inline-flex flex-shrink-0 items-center justify-center gap-1.5 rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-md shadow-primary-500/20 transition-[background-color,box-shadow,transform] duration-200 hover:bg-primary-700 hover:shadow-lg hover:shadow-primary-500/25 active:scale-[0.98] dark:bg-primary-500 dark:hover:bg-primary-400 dark:shadow-primary-500/20"
      >
        {{ t('modelPlaza.nav.backToDashboard') }}
      </RouterLink>
      <RouterLink
        v-else
        :to="{ path: '/login', query: { redirect: locale === 'en' ? '/en/catalog' : '/catalog' } }"
        class="inline-flex flex-shrink-0 items-center justify-center rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-md shadow-primary-500/20 transition-[background-color,box-shadow,transform] duration-200 hover:bg-primary-700 hover:shadow-lg hover:shadow-primary-500/25 active:scale-[0.98] dark:bg-primary-500 dark:hover:bg-primary-400 dark:shadow-primary-500/20"
      >
        {{ t('modelPlaza.nav.login') }}
      </RouterLink>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { sanitizeUrl } from '@/utils/url'
import { localizedSiteName } from '@/utils/localizedPublicSettings'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const { t, locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const settings = computed(() => appStore.cachedPublicSettings)
const siteName = computed(() => localizedSiteName(settings.value?.site_name, locale.value))
const siteLogo = computed(() =>
  sanitizeUrl(settings.value?.site_logo || '', { allowRelative: true, allowDataUrl: true })
)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const backTarget = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
</script>
