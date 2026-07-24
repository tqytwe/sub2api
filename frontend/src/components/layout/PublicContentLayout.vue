<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'

const props = withDefaults(defineProps<{
  siteName?: string
  siteLogo?: string
  docUrl?: string
  frame?: 'compact' | 'reading' | 'content'
  showFooter?: boolean
  githubUrl?: string
}>(), {
  siteName: '极速蹬',
  siteLogo: '',
  docUrl: '',
  frame: 'content',
  showFooter: true,
  githubUrl: '',
})

const { t } = useI18n()
const currentYear = computed(() => new Date().getFullYear())
const frameClass = computed(() => ({
  compact: 'max-w-2xl',
  reading: 'max-w-[50rem]',
  content: 'max-w-6xl',
}[props.frame]))
</script>

<template>
  <div class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-gray-200 bg-white/95 dark:border-dark-800 dark:bg-dark-900/95">
      <div class="mx-auto flex w-full max-w-6xl items-center justify-between gap-4 px-4 py-4 sm:px-6 lg:px-8">
        <RouterLink to="/home" class="flex min-w-0 items-center gap-3">
          <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700">
            <img :src="siteLogo || '/logo.png'" :alt="siteName" class="h-full w-full object-contain" />
          </span>
          <span class="truncate text-base font-semibold text-gray-950 dark:text-white">
            {{ siteName }}
          </span>
        </RouterLink>

        <nav class="flex flex-shrink-0 items-center gap-2" :aria-label="t('nav.publicActions')">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex h-10 items-center gap-1.5 rounded-lg px-3 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
          >
            <Icon name="book" size="sm" />
            <span class="hidden sm:inline">{{ t('home.viewDocs') }}</span>
          </a>
          <PublicPageToolbar />
          <slot name="actions" />
        </nav>
      </div>
    </header>

    <main class="flex-1">
      <div :class="['mx-auto w-full px-4 py-10 sm:px-6 lg:px-8 lg:py-12', frameClass]">
        <slot />
      </div>
    </main>

    <footer
      v-if="showFooter"
      class="border-t border-gray-200/70 px-4 py-7 dark:border-dark-800/70 sm:px-6"
    >
      <div class="mx-auto flex w-full max-w-6xl flex-col items-center justify-between gap-3 text-center sm:flex-row sm:text-left">
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-900 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            v-if="githubUrl"
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-900 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>
