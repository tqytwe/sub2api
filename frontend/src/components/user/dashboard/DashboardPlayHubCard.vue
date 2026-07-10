<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import playAPI from '@/api/play'
import { isFeatureFlagEnabled, FeatureFlags } from '@/utils/featureFlags'

const { t } = useI18n()
const router = useRouter()

const pending = ref(0)
const anyPlay = ref(false)
const loading = ref(true)

const visible = computed(() => {
  if (loading.value) return false
  return anyPlay.value || isFeatureFlagEnabled(FeatureFlags.affiliate)
})

async function load() {
  loading.value = true
  try {
    const hub = await playAPI.getPlayHub()
    pending.value = hub.pending_actions
    anyPlay.value = hub.any_enabled
  } catch {
    pending.value = 0
    anyPlay.value = false
  } finally {
    loading.value = false
  }
}

function goHub() {
  router.push('/play')
}

onMounted(load)
</script>

<template>
  <div v-if="visible" class="card overflow-hidden">
    <button
      type="button"
      class="group flex w-full items-center gap-4 p-4 text-left transition hover:bg-gray-50 dark:hover:bg-dark-800/50"
      @click="goHub"
    >
      <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-violet-100 dark:bg-violet-900/30">
        <Icon name="gift" size="lg" class="text-violet-600 dark:text-violet-400" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('dashboard.playHub.title') }}</p>
          <span
            v-if="pending > 0"
            class="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-200"
          >
            {{ t('dashboard.playHub.pending', { count: pending }) }}
          </span>
        </div>
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.playHub.desc') }}</p>
      </div>
      <Icon name="chevronRight" size="md" class="text-gray-400 group-hover:text-violet-500 dark:text-dark-500" />
    </button>
  </div>
</template>
