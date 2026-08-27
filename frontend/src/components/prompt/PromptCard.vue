<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import type { PromptSummary } from '@/api/prompts'
import { shouldUseGeneratedPromptCover } from '@/utils/promptCover'
import {
  promptSourceMessageKey,
  referenceRequirementMessageKey,
} from '@/utils/promptLibrary'
import Icon from '@/components/icons/Icon.vue'
import PromptGeneratedCover from '@/components/prompt/PromptGeneratedCover.vue'

const props = defineProps<{
  prompt: PromptSummary
  busy?: boolean
}>()

const emit = defineEmits<{
  favorite: [prompt: PromptSummary]
  copy: [prompt: PromptSummary]
  use: [prompt: PromptSummary]
}>()

const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const brandLabel = computed(() => t(promptSourceMessageKey(props.prompt.source_attribution)))
const referenceLabel = computed(() => t(referenceRequirementMessageKey(props.prompt.reference_requirement)))
const useGeneratedCover = computed(() => shouldUseGeneratedPromptCover(props.prompt))

async function handleFavorite() {
  if (!authStore.isAuthenticated) {
    await router.push({
      path: '/login',
      query: { redirect: route.fullPath },
    })
    return
  }
  emit('favorite', props.prompt)
}
</script>

<template>
  <article class="prompt-card">
    <button
      type="button"
      class="prompt-card-media"
      :aria-label="t('promptLibrary.card.use')"
      @click="emit('use', prompt)"
    >
      <img
        v-if="!useGeneratedCover && prompt.preview_image_url"
        :src="prompt.preview_image_url"
        :alt="prompt.preview_image_alt || t('promptLibrary.card.previewAlt', { title: prompt.title })"
        loading="lazy"
      />
      <PromptGeneratedCover v-else :prompt="prompt" />
      <span class="prompt-brand-badge">{{ brandLabel }}</span>
    </button>

    <div class="prompt-card-body">
      <div class="prompt-card-heading">
        <div class="min-w-0">
          <h2>{{ prompt.title }}</h2>
          <p>{{ prompt.purpose_description }}</p>
        </div>
        <button
          type="button"
          class="prompt-icon-button"
          :class="{ 'is-active': prompt.is_favorited }"
          :aria-label="prompt.is_favorited ? t('promptLibrary.card.unfavorite') : t('promptLibrary.card.favorite')"
          :title="prompt.is_favorited ? t('promptLibrary.card.unfavorite') : t('promptLibrary.card.favorite')"
          :disabled="busy"
          @click="handleFavorite"
        >
          <Icon name="badge" size="sm" />
        </button>
      </div>

      <div class="prompt-card-tags">
        <span v-if="prompt.recommended_models[0]">{{ prompt.recommended_models[0] }}</span>
        <span v-if="prompt.recommended_sizes[0]">{{ prompt.recommended_sizes[0] }}</span>
        <span>{{ referenceLabel }}</span>
      </div>

      <div class="prompt-card-stats" :aria-label="t('promptLibrary.card.stats')">
        <span>{{ t('promptLibrary.card.useCount', { count: prompt.use_count || 0 }) }}</span>
        <span>{{ t('promptLibrary.card.favoriteCount', { count: prompt.favorite_count || 0 }) }}</span>
      </div>

      <div class="prompt-card-actions">
        <button
          type="button"
          class="prompt-icon-button"
          :aria-label="t('promptLibrary.card.copy')"
          :title="t('promptLibrary.card.copy')"
          @click="emit('copy', prompt)"
        >
          <Icon name="copy" size="sm" />
        </button>
        <button
          type="button"
          class="prompt-use-button"
          :aria-label="t('promptLibrary.card.use')"
          :disabled="busy"
          @click="emit('use', prompt)"
        >
          <Icon name="sparkles" size="sm" />
          {{ t('promptLibrary.card.use') }}
        </button>
      </div>
    </div>
  </article>
</template>
