<template>
  <div
    class="relative flex min-h-screen flex-col overflow-hidden bg-gray-50 dark:bg-dark-950"
  >
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <router-link to="/home" class="text-sm font-medium text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white">
          {{ t('play.backHome') }}
        </router-link>
        <PublicPageToolbar />
      </nav>
    </header>

    <div class="relative flex flex-1 items-center justify-center px-4 pb-16">
      <div class="pointer-events-none absolute inset-0 overflow-hidden">
        <div class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/10 blur-3xl" />
        <div class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/10 blur-3xl" />
      </div>

      <div class="relative z-10 w-full max-w-md text-center">
        <div class="mb-8">
          <div class="relative inline-block">
            <span class="text-[12rem] font-bold leading-none text-gray-100 dark:text-dark-800">404</span>
            <div class="absolute inset-0 flex items-center justify-center">
              <div class="flex h-24 w-24 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-500 to-primary-600 shadow-lg shadow-primary-500/30">
                <svg class="h-12 w-12 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                </svg>
              </div>
            </div>
          </div>
        </div>

        <div class="mb-8">
          <h1 class="mb-3 text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('errors.pageNotFound') }}
          </h1>
          <p class="text-gray-500 dark:text-dark-400">
            {{ t('errors.pageNotFoundHint') }}
          </p>
        </div>

        <div class="flex flex-col justify-center gap-3 sm:flex-row">
          <button type="button" class="btn btn-secondary" @click="goBack">
            <Icon name="arrowLeft" size="md" class="mr-2" />
            {{ t('common.back') }}
          </button>
          <router-link to="/home" class="btn btn-primary">
            <Icon name="home" size="md" class="mr-2" />
            {{ t('errors.backHome') }}
          </router-link>
        </div>

        <p class="mt-8 text-sm text-gray-400 dark:text-dark-500">
          {{ t('errors.needHelp') }}
          <router-link to="/contact" class="text-primary-600 hover:text-primary-500 dark:text-primary-400">
            {{ t('errors.contactSupport') }}
          </router-link>
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'

const router = useRouter()
const { t } = useI18n()

function goBack() {
  router.back()
}
</script>
