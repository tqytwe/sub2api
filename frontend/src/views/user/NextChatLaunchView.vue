<template>
  <AppLayout>
    <div class="py-10">
      <CompactStatusPanel
        :title="t('nextChatLaunch.title')"
        :description="statusText"
        :tone="failed ? 'danger' : 'primary'"
        :icon="failed ? 'exclamationCircle' : undefined"
        :loading="!failed"
      >
        <template v-if="failed" #actions>
          <button
            type="button"
            class="btn btn-primary"
            @click="startLaunch"
          >
            <Icon name="refresh" size="md" />
            {{ t('common.retry') }}
          </button>
          <router-link to="/dashboard" class="btn btn-secondary">
            <Icon name="home" size="md" />
            {{ t('home.goToDashboard') }}
          </router-link>
        </template>
      </CompactStatusPanel>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import { launchNextChat } from '@/api/user'
import CompactStatusPanel from '@/components/common/CompactStatusPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'

const { t } = useI18n()
const route = useRoute()
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
    const promptID = Number(route.query.prompt)
    const promptVersion = Number(route.query.version)
    const intent = Number.isSafeInteger(promptID) && promptID > 0
      ? {
          type: 'image_prompt' as const,
          prompt_id: promptID,
          ...(Number.isSafeInteger(promptVersion) && promptVersion > 0 ? { prompt_version: promptVersion } : {}),
        }
      : undefined
    const result = await launchNextChat(intent)
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
