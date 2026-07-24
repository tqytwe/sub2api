<template>
  <div>
    <button
      type="button"
      @click="openModal"
      class="relative flex h-9 w-9 items-center justify-center rounded-lg text-gray-600 transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:text-gray-400 dark:hover:bg-dark-800 dark:focus-visible:ring-offset-dark-900"
      :class="{ 'text-blue-600 dark:text-blue-400': unreadCount > 0 }"
      :aria-label="t('announcements.title')"
    >
      <Icon name="bell" size="md" />
      <span
        v-if="unreadCount > 0"
        class="absolute right-1 top-1 h-2 w-2 rounded-full bg-red-500 ring-2 ring-white dark:ring-dark-900"
        aria-hidden="true"
      ></span>
    </button>

    <Teleport to="body">
      <Transition name="modal-fade">
        <div
          v-if="isModalOpen"
          class="fixed inset-0 z-[100] flex items-start justify-center overflow-y-auto bg-black/60 p-4 pt-[8vh] backdrop-blur-sm"
          @click="closeModal"
          >
            <section
              ref="modalDialogRef"
              class="w-full max-w-[620px] overflow-hidden rounded-[12px] border border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-800"
              role="dialog"
              aria-modal="true"
              :aria-label="t('announcements.title')"
              tabindex="-1"
              @click.stop
            >
            <header class="border-b border-gray-100 bg-white px-6 py-5 dark:border-dark-700 dark:bg-dark-800">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
                      <Icon name="bell" size="sm" />
                    </span>
                    <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                      {{ t('announcements.title') }}
                    </h2>
                  </div>
                  <p v-if="unreadCount > 0" class="mt-2 text-sm text-gray-600 dark:text-gray-400">
                    <span class="font-medium text-blue-600 dark:text-blue-400">{{ unreadCount }}</span>
                    {{ t('announcements.unread') }}
                  </p>
                </div>
                <div class="flex shrink-0 items-center gap-2">
                  <button
                    v-if="unreadCount > 0"
                    type="button"
                    @click="markAllAsRead"
                    :disabled="loading"
                    class="btn btn-primary btn-sm"
                  >
                    {{ t('announcements.markAllRead') }}
                  </button>
                  <button
                    type="button"
                    @click="closeModal"
                    class="flex h-9 w-9 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 transition-colors hover:bg-gray-50 hover:text-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600 dark:hover:text-gray-200 dark:focus-visible:ring-offset-dark-800"
                    :aria-label="t('common.close')"
                  >
                    <Icon name="x" size="sm" />
                  </button>
                </div>
              </div>
            </header>

            <div class="max-h-[65vh] overflow-y-auto">
              <div v-if="loading" class="flex items-center justify-center py-16" aria-busy="true">
                <LoadingSpinner size="md" />
              </div>

              <div v-else-if="announcements.length > 0">
                <button
                  v-for="item in announcements"
                  :key="item.id"
                  type="button"
                  class="group relative flex min-h-[72px] w-full items-center gap-4 border-b border-gray-100 px-6 py-4 text-left transition-colors hover:bg-gray-50 focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset dark:border-dark-700 dark:hover:bg-dark-700/30"
                  :class="{ 'bg-blue-50/40 dark:bg-blue-900/10': !item.read_at }"
                  @click="openDetail(item)"
                >
                  <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" :class="item.read_at ? 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-gray-500' : 'bg-blue-600 text-white dark:bg-blue-500'">
                    <Icon :name="item.read_at ? 'checkCircle' : 'infoCircle'" size="md" :stroke-width="2" />
                  </span>

                  <span class="flex min-w-0 flex-1 items-center justify-between gap-4">
                    <span class="min-w-0 flex-1">
                      <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">
                        {{ item.title }}
                      </span>
                      <span class="mt-1 flex items-center gap-2">
                        <time class="text-xs text-gray-500 dark:text-gray-400">
                          {{ formatRelativeTime(item.created_at) }}
                        </time>
                        <span
                          v-if="!item.read_at"
                          class="inline-flex items-center gap-1 rounded-md bg-blue-100 px-1.5 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
                        >
                          <span class="h-1.5 w-1.5 rounded-full bg-blue-600 dark:bg-blue-300" aria-hidden="true"></span>
                          {{ t('announcements.unread') }}
                        </span>
                      </span>
                    </span>

                    <Icon name="chevronRight" size="md" class="shrink-0 text-gray-400 dark:text-gray-600" />
                  </span>

                  <span
                    v-if="!item.read_at"
                    class="absolute left-0 top-0 h-full w-1 bg-blue-600 dark:bg-blue-400"
                    aria-hidden="true"
                  ></span>
                </button>
              </div>

              <div v-else class="flex flex-col items-center justify-center px-6 py-16 text-center">
                <div class="relative mb-4">
                  <div class="flex h-20 w-20 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
                    <Icon name="inbox" size="xl" class="text-gray-400 dark:text-gray-500" />
                  </div>
                  <div class="absolute -right-1 -top-1 flex h-6 w-6 items-center justify-center rounded-full bg-green-600 text-white">
                    <Icon name="check" size="xs" :stroke-width="2.5" />
                  </div>
                </div>
                <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('announcements.empty') }}</p>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('announcements.emptyDescription') }}</p>
              </div>
            </div>
          </section>
        </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <Transition name="modal-fade">
        <div
          v-if="detailModalOpen && selectedAnnouncement"
          class="fixed inset-0 z-[110] flex items-start justify-center overflow-y-auto bg-black/60 p-4 pt-[6vh] backdrop-blur-sm"
          @click="closeDetail"
        >
          <section
            ref="detailDialogRef"
            class="w-full max-w-[780px] overflow-hidden rounded-[12px] border border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-800"
            role="dialog"
            aria-modal="true"
            :aria-label="selectedAnnouncement.title"
            tabindex="-1"
            @click.stop
          >
            <header class="border-b border-gray-100 bg-white px-6 py-5 dark:border-dark-700 dark:bg-dark-800 sm:px-8 sm:py-6">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0 flex-1">
                  <div class="mb-3 flex flex-wrap items-center gap-2">
                    <span class="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
                      <Icon name="infoCircle" size="md" :stroke-width="2" />
                    </span>
                    <span class="rounded-lg bg-blue-100 px-2.5 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                      {{ t('announcements.title') }}
                    </span>
                    <span
                      v-if="!selectedAnnouncement.read_at"
                      class="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-2.5 py-1 text-xs font-medium text-white dark:bg-blue-500"
                    >
                      <span class="h-2 w-2 rounded-full bg-white" aria-hidden="true"></span>
                      {{ t('announcements.unread') }}
                    </span>
                  </div>

                  <h2 class="mb-3 text-2xl font-bold leading-tight text-gray-900 dark:text-white">
                    {{ selectedAnnouncement.title }}
                  </h2>

                  <div class="flex flex-wrap items-center gap-4 text-sm text-gray-600 dark:text-gray-400">
                    <div class="flex items-center gap-1.5">
                      <Icon name="clock" size="sm" />
                      <time>{{ formatRelativeWithDateTime(selectedAnnouncement.created_at) }}</time>
                    </div>
                    <div class="flex items-center gap-1.5">
                      <Icon name="eye" size="sm" />
                      <span>{{ selectedAnnouncement.read_at ? t('announcements.read') : t('announcements.unread') }}</span>
                    </div>
                  </div>
                </div>

                <button
                  type="button"
                  @click="closeDetail"
                  class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 transition-colors hover:bg-gray-50 hover:text-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600 dark:hover:text-gray-200 dark:focus-visible:ring-offset-dark-800"
                  :aria-label="t('common.close')"
                >
                  <Icon name="x" size="md" />
                </button>
              </div>
            </header>

            <div class="max-h-[60vh] overflow-y-auto bg-white px-6 py-6 dark:bg-dark-800 sm:px-8 sm:py-8">
              <div class="border-l-4 border-gray-200 pl-5 dark:border-dark-600">
                <AnnouncementContent :content="selectedAnnouncement.content" />
              </div>
            </div>

            <footer class="border-t border-gray-100 bg-gray-50 px-6 py-5 dark:border-dark-700 dark:bg-dark-900/30 sm:px-8">
              <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                  <Icon name="infoCircle" size="sm" />
                  <span>{{ selectedAnnouncement.read_at ? t('announcements.readStatus') : t('announcements.markReadHint') }}</span>
                </div>
                <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <button type="button" @click="closeDetail" class="btn btn-secondary">
                    {{ t('common.close') }}
                  </button>
                  <button
                    v-if="!selectedAnnouncement.read_at"
                    type="button"
                    @click="markAsReadAndClose(selectedAnnouncement.id)"
                    class="btn btn-primary inline-flex items-center justify-center gap-2"
                  >
                    <Icon name="check" size="sm" :stroke-width="2.5" />
                    {{ t('announcements.markRead') }}
                  </button>
                </div>
              </div>
            </footer>
          </section>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { useAppStore } from '@/stores/app'
import { useAnnouncementStore } from '@/stores/announcements'
import { useDialogAccessibility } from '@/composables/useDialogAccessibility'
import { formatRelativeTime, formatRelativeWithDateTime } from '@/utils/format'
import type { UserAnnouncement } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import AnnouncementContent from '@/components/common/AnnouncementContent.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const { t } = useI18n()
const appStore = useAppStore()
const announcementStore = useAnnouncementStore()

const { announcements, loading } = storeToRefs(announcementStore)
const unreadCount = computed(() => announcementStore.unreadCount)

const isModalOpen = ref(false)
const detailModalOpen = ref(false)
const selectedAnnouncement = ref<UserAnnouncement | null>(null)
const modalDialogRef = ref<HTMLElement | null>(null)
const detailDialogRef = ref<HTMLElement | null>(null)
const modalDialogActive = computed(() => isModalOpen.value && !detailModalOpen.value)

function openModal() {
  isModalOpen.value = true
}

function closeModal() {
  isModalOpen.value = false
}

function openDetail(announcement: UserAnnouncement) {
  selectedAnnouncement.value = announcement
  detailModalOpen.value = true
  if (!announcement.read_at) {
    markAsRead(announcement.id)
  }
}

function closeDetail() {
  detailModalOpen.value = false
  selectedAnnouncement.value = null
}

async function markAsRead(id: number) {
  try {
    await announcementStore.markAsRead(id)
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  }
}

async function markAsReadAndClose(id: number) {
  await markAsRead(id)
  appStore.showSuccess(t('announcements.markedAsRead'))
  closeDetail()
}

async function markAllAsRead() {
  try {
    await announcementStore.markAllAsRead()
    appStore.showSuccess(t('announcements.allMarkedAsRead'))
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  }
}

useDialogAccessibility(modalDialogActive, modalDialogRef, {
  onClose: closeModal,
})

useDialogAccessibility(detailModalOpen, detailDialogRef, {
  onClose: closeDetail,
})
</script>

<style scoped>
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-from > section,
.modal-fade-leave-to > section {
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
  .modal-fade-enter-active,
  .modal-fade-leave-active {
    transition: opacity 0.1s ease;
  }

  .modal-fade-enter-from > section,
  .modal-fade-leave-to > section {
    transform: none;
  }
}
</style>
