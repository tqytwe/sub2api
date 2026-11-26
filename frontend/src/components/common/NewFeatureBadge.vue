<template>
  <button
    v-if="visible"
    type="button"
    data-testid="new-feature-badge"
    :aria-label="t('common.dismissNewFeature')"
    :title="t('common.dismissNewFeature')"
    class="inline-flex shrink-0 items-center justify-center rounded border border-amber-500 px-1.5 py-0.5 text-xs font-medium leading-tight text-amber-600 transition-colors motion-reduce:transition-none hover:bg-amber-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500 dark:border-amber-400 dark:text-amber-300 dark:hover:bg-amber-950/40"
    @click.stop.prevent="dismiss"
  >
    {{ t('common.newFeature') }}
  </button>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useNewFeatureNoticeStore } from '@/stores/newFeatureNotices'

const props = defineProps<{ noticeId: string; userId: number | null | undefined }>()
const { t } = useI18n()
const notices = useNewFeatureNoticeStore()
watch(() => props.userId, (userId) => notices.loadUser(userId), { immediate: true })
const visible = computed(() => props.userId != null && !notices.isDismissed(props.userId, props.noticeId))
function dismiss(): void {
  notices.dismiss(props.userId, props.noticeId)
}
</script>
