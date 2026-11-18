<template>
  <nav :aria-label="t('accountOps.smartTitle')" class="flex flex-wrap gap-2 border-b border-gray-200 pb-3 dark:border-dark-700">
    <RouterLink
      v-for="item in items"
      :key="item.path"
      :to="item.path"
      class="rounded-md px-3 py-2 text-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500"
      :class="route.path === item.path ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-950/30 dark:text-primary-300' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-800'"
    >{{ t(item.label) }}</RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const route = useRoute()
const items = [
  { path: '/admin/account-ops', label: 'accountOps.title' },
  { path: '/admin/token-guard-v2', label: 'tokenGuardV2.title' },
  { path: '/admin/account-quality', label: 'admin.qualityOps.nav' },
]
</script>
