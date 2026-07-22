<template>
  <Teleport to="body">
    <Transition name="popup-fade">
      <div
        v-if="announcementStore.currentPopup"
        class="fixed inset-0 z-[120] flex items-start justify-center overflow-y-auto bg-black/60 p-4 pt-[8vh] backdrop-blur-sm"
      >
        <section
          ref="popupDialogRef"
          class="w-full max-w-[680px] overflow-hidden rounded-[12px] border border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-800"
          role="dialog"
          aria-modal="true"
          :aria-label="announcementStore.currentPopup.title"
          tabindex="-1"
          @click.stop
        >
          <header class="border-b border-gray-100 bg-white px-6 py-5 dark:border-dark-700 dark:bg-dark-800 sm:px-8 sm:py-6">
            <div class="mb-3 flex flex-wrap items-center gap-2">
              <span class="flex h-10 w-10 items-center justify-center rounded-lg bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300">
                <Icon name="bell" size="md" :stroke-width="2" />
              </span>
              <span class="inline-flex items-center gap-1.5 rounded-lg bg-amber-100 px-2.5 py-1 text-xs font-medium text-amber-800 dark:bg-amber-900/30 dark:text-amber-300">
                <span class="h-2 w-2 rounded-full bg-amber-600 dark:bg-amber-300" aria-hidden="true"></span>
                {{ t('announcements.unread') }}
              </span>
            </div>

            <h2 class="mb-2 text-2xl font-bold leading-tight text-gray-900 dark:text-white">
              {{ announcementStore.currentPopup.title }}
            </h2>

            <div class="flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-400">
              <Icon name="clock" size="sm" />
              <time>{{ formatRelativeWithDateTime(announcementStore.currentPopup.created_at) }}</time>
            </div>
          </header>

          <div class="max-h-[50vh] overflow-y-auto bg-white px-6 py-6 dark:bg-dark-800 sm:px-8 sm:py-8">
            <div class="border-l-4 border-gray-200 pl-5 dark:border-dark-600">
              <AnnouncementContent :content="announcementStore.currentPopup.content" />
            </div>
          </div>

          <footer class="border-t border-gray-100 bg-gray-50 px-6 py-5 dark:border-dark-700 dark:bg-dark-900/30 sm:px-8">
            <div class="flex items-center justify-end">
              <button
                type="button"
                @click="handleDismiss"
                class="btn btn-primary inline-flex items-center justify-center gap-2"
              >
                <Icon name="check" size="sm" :stroke-width="2.5" />
                {{ t('announcements.markRead') }}
              </button>
            </div>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeWithDateTime } from '@/utils/format'
import { useDialogAccessibility } from '@/composables/useDialogAccessibility'
import AnnouncementContent from '@/components/common/AnnouncementContent.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const announcementStore = useAnnouncementStore()
const popupDialogRef = ref<HTMLElement | null>(null)
const popupVisible = computed(() => Boolean(announcementStore.currentPopup))

function handleDismiss() {
  announcementStore.dismissPopup()
}

useDialogAccessibility(popupVisible, popupDialogRef, {
  onClose: handleDismiss,
})
</script>

<style scoped>
.popup-fade-enter-active,
.popup-fade-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.popup-fade-enter-from,
.popup-fade-leave-to {
  opacity: 0;
}

.popup-fade-enter-from > section,
.popup-fade-leave-to > section {
  transform: translateY(-8px);
}

.overflow-y-auto::-webkit-scrollbar {
  width: 8px;
}

.overflow-y-auto::-webkit-scrollbar-track {
  background: transparent;
}

.overflow-y-auto::-webkit-scrollbar-thumb {
  @apply rounded bg-gray-300 dark:bg-dark-600;
}

.overflow-y-auto::-webkit-scrollbar-thumb:hover {
  @apply bg-gray-400 dark:bg-dark-500;
}

@media (prefers-reduced-motion: reduce) {
  .popup-fade-enter-active,
  .popup-fade-leave-active {
    transition: opacity 0.1s ease;
  }

  .popup-fade-enter-from > section,
  .popup-fade-leave-to > section {
    transform: none;
  }
}
</style>
