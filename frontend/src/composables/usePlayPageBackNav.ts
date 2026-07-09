import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

export function usePlayPageBackNav() {
  const { t } = useI18n()
  const authStore = useAuthStore()

  const backTarget = computed(() => (authStore.isAuthenticated ? '/play' : '/home'))
  const backLabel = computed(() =>
    authStore.isAuthenticated ? t('play.backPlayHub') : t('play.backHome'),
  )

  return { backTarget, backLabel }
}
