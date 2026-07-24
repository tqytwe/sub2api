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
import { useAppStore } from '@/stores'
import { launchNextChat } from '@/api/user'
import CompactStatusPanel from '@/components/common/CompactStatusPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'

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
