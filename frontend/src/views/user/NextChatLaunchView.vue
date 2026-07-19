<template>
  <div class="flex min-h-[60vh] items-center justify-center px-4">
    <div class="w-full max-w-md text-center">
      <div class="mx-auto grid h-12 w-12 place-items-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
        <span class="h-5 w-5 animate-spin rounded-full border-2 border-current border-t-transparent" aria-hidden="true"></span>
      </div>
      <h1 class="mt-5 text-lg font-semibold text-gray-900 dark:text-white">{{ t('nextChatLaunch.title') }}</h1>
      <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ statusText }}</p>
      <button
        v-if="failed"
        type="button"
        class="btn btn-primary mt-5"
        @click="startLaunch"
      >
        {{ t('common.retry') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { launchNextChat } from '@/api/user'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const failed = ref(false)

const statusText = computed(() => (
  failed.value ? t('nextChatLaunch.failed') : t('nextChatLaunch.loading')
))

async function startLaunch(): Promise<void> {
  if (loading.value) return
  loading.value = true
  failed.value = false

  try {
    const result = await launchNextChat()
    const launchURL = result.launch_url?.trim()
    if (!launchURL) {
      throw new Error('Missing NextChat launch URL')
    }
    window.location.replace(launchURL)
  } catch {
    failed.value = true
    appStore.showToast('error', t('nextChatLaunch.failed'))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void startLaunch()
})
</script>
