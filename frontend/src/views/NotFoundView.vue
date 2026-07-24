<template>
  <PublicStatusLayout>
    <CompactStatusPanel
      eyebrow="404"
      :title="t('errors.pageNotFound')"
      :description="t('errors.pageNotFoundHint')"
      icon="exclamationTriangle"
      tone="warning"
    >
      <template #actions>
        <button @click="goBack" class="btn btn-secondary">
          <Icon name="arrowLeft" size="md" />
          {{ t('common.back') }}
        </button>
        <router-link :to="homeTarget" class="btn btn-primary">
          <Icon name="home" size="md" />
          {{ homeLabel }}
        </router-link>
      </template>

      <template #support>
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ t('errors.needHelp') }}
          <router-link
            to="/contact"
            class="font-medium text-primary-600 transition-colors hover:text-primary-500 dark:text-primary-400 dark:hover:text-primary-300"
          >
            {{ t('errors.contactSupport') }}
          </router-link>
        </p>
      </template>
    </CompactStatusPanel>

    <template #floating>
      <SupportFloatingCard />
    </template>
  </PublicStatusLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import Icon from '@/components/icons/Icon.vue'
import CompactStatusPanel from '@/components/common/CompactStatusPanel.vue'
import PublicStatusLayout from '@/components/layout/PublicStatusLayout.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const homeTarget = computed(() => (authStore.isAuthenticated ? '/dashboard' : '/home'))
const homeLabel = computed(() =>
  authStore.isAuthenticated ? t('home.goToDashboard') : t('errors.backHome'),
)

function goBack(): void {
  router.back()
}
</script>
