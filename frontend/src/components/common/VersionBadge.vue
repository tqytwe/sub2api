<template>
  <div ref="badgeRef" class="relative">
    <template v-if="isAdmin">
      <button
        :aria-expanded="dropdownOpen"
        :title="hasUpdate ? t('version.updateAvailable') : t('version.currentVersion')"
        class="flex items-center gap-1.5 rounded-lg px-2 py-1 text-xs transition-colors focus-visible:ring-2 focus-visible:ring-primary-500"
        :class="hasUpdate
          ? 'bg-amber-100 text-amber-700 hover:bg-amber-200 dark:bg-amber-900/30 dark:text-amber-400'
          : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-800 dark:text-dark-400'"
        @click="dropdownOpen = !dropdownOpen"
        @keydown.esc="dropdownOpen = false"
      >
        <span class="font-medium">{{ currentVersion ? 'v' + currentVersion : '--' }}</span>
        <span v-if="hasUpdate" class="h-2 w-2 rounded-full bg-amber-500"></span>
      </button>
      <div
        v-if="dropdownOpen"
        class="absolute left-0 z-50 mt-2 w-72 overflow-hidden whitespace-normal rounded-[12px] border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-800"
        @keydown.esc="closeDropdown"
      >
        <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <span class="text-sm font-medium text-gray-700 dark:text-dark-300">{{ t('version.currentVersion') }}</span>
          <button
            :disabled="loading"
            :aria-label="t('version.refresh')"
            class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-primary-500 disabled:opacity-50 dark:text-dark-400 dark:hover:bg-dark-700"
            @click="refreshVersion"
          >
            <Icon name="refresh" size="sm" />
          </button>
        </div>
        <div class="space-y-3 p-4" :aria-busy="loading">
          <p class="text-center text-2xl font-bold text-gray-900 dark:text-white">{{ currentVersion ? 'v' + currentVersion : '--' }}</p>
          <p v-if="loading" role="status" class="text-xs text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</p>
          <p v-else-if="warning" role="status" class="text-xs text-amber-700 dark:text-amber-400">{{ t('version.checkFailed') }}</p>
          <template v-else>
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('version.latestVersion') }}: {{ latestVersion ? 'v' + latestVersion : '--' }}</p>
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('version.upstreamBaseline', { version: upstreamBaseline || '--' }) }}</p>
          </template>
          <p v-if="hasUpdate && !loading" role="status" class="text-xs text-amber-700 dark:text-amber-400">{{ t('version.updateAvailable') }}</p>
          <a :href="upstreamReleaseURL" target="_blank" rel="noopener noreferrer" class="flex items-center gap-2 text-sm text-primary-700 underline dark:text-primary-300">
            <Icon name="externalLink" size="sm" />
            {{ t('version.viewRelease') }}
          </a>
          <div class="space-y-2 border-t border-gray-100 pt-3 dark:border-dark-700">
            <p class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('version.forkInstall') }}</p>
            <p class="text-xs leading-5 text-gray-600 dark:text-dark-400">{{ t('version.sourceModeHint') }}</p>
            <a :href="deploymentGuideURL" target="_blank" rel="noopener noreferrer" class="text-sm text-primary-700 underline dark:text-primary-300">{{ t('version.deploymentGuide') }}</a>
            <p class="text-xs leading-5 text-gray-600 dark:text-dark-400">{{ t('version.rollbackSourceHint') }}</p>
          </div>
        </div>
      </div>
    </template>
    <span v-else-if="version" class="text-xs text-gray-500 dark:text-dark-400">v{{ version }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const props = defineProps<{ version?: string }>()
const appStore = useAppStore()
const authStore = useAuthStore()
const isAdmin = computed(() => authStore.isAdmin)
const dropdownOpen = ref(false)
const badgeRef = ref<HTMLElement | null>(null)
const currentVersion = computed(() => appStore.currentVersion || props.version || '')
const latestVersion = computed(() => appStore.latestVersion)
const upstreamBaseline = computed(() => appStore.upstreamBaseline)
const hasUpdate = computed(() => appStore.hasUpdate)
const loading = computed(() => appStore.versionLoading)
const warning = computed(() => appStore.versionWarning)
const deploymentGuideURL = 'https://github.com/tqytwe/sub2api/blob/play/main/deploy/FORK_SOURCE_BUILD.md'
// A cached URL must never turn the upstream link into another repository or installer.
const upstreamReleaseURL = computed(() => {
  const url = appStore.releaseInfo?.html_url || ''
  return /^https:\/\/github\.com\/ranxi2001\/sub2api\/releases\/tag\/v\d+\.\d+\.\d+$/.test(url)
    ? url : 'https://github.com/ranxi2001/sub2api/releases'
})
function closeDropdown() {
  dropdownOpen.value = false
  badgeRef.value?.querySelector('button')?.focus()
}
async function refreshVersion() {
  if (isAdmin.value && !loading.value) await appStore.fetchVersion(true)
}
function handleClickOutside(event: MouseEvent) {
  if (!badgeRef.value?.contains(event.target as Node)) dropdownOpen.value = false
}
onMounted(() => {
  if (isAdmin.value) appStore.fetchVersion(false)
  document.addEventListener('click', handleClickOutside)
})
onBeforeUnmount(() => document.removeEventListener('click', handleClickOutside))
</script>
