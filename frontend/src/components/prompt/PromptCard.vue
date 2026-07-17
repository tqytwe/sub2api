<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import type { PromptSummary } from '@/api/prompts'
import { shouldUseGeneratedPromptCover } from '@/utils/promptCover'
import {
  promptSourceLabel,
  referenceRequirementLabel,
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
  details: [prompt: PromptSummary]
  use: [prompt: PromptSummary]
}>()

const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()

const brandLabel = computed(() => promptSourceLabel(props.prompt.source_attribution))
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
      aria-label="查看详情"
      @click="emit('details', prompt)"
    >
      <img
        v-if="!useGeneratedCover && prompt.preview_image_url"
        :src="prompt.preview_image_url"
        :alt="prompt.preview_image_alt || `${prompt.title}示例效果`"
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
          :aria-label="prompt.is_favorited ? '取消收藏' : '收藏提示词'"
          :title="prompt.is_favorited ? '取消收藏' : '收藏提示词'"
          :disabled="busy"
          @click="handleFavorite"
        >
          <Icon name="badge" size="sm" />
        </button>
      </div>

      <div class="prompt-card-tags">
        <span v-if="prompt.recommended_models[0]">{{ prompt.recommended_models[0] }}</span>
        <span v-if="prompt.recommended_sizes[0]">{{ prompt.recommended_sizes[0] }}</span>
        <span>{{ referenceRequirementLabel(prompt.reference_requirement) }}</span>
      </div>

      <div class="prompt-card-stats" aria-label="提示词数据">
        <span>使用 {{ prompt.use_count || 0 }}</span>
        <span>收藏 {{ prompt.favorite_count || 0 }}</span>
      </div>

      <div class="prompt-card-actions">
        <button
          type="button"
          class="prompt-icon-button"
          aria-label="复制提示词"
          title="复制提示词"
          @click="emit('copy', prompt)"
        >
          <Icon name="copy" size="sm" />
        </button>
        <button
          type="button"
          class="prompt-icon-button"
          aria-label="查看详情"
          title="查看详情"
          @click="emit('details', prompt)"
        >
          <Icon name="eye" size="sm" />
        </button>
        <button
          type="button"
          class="prompt-use-button"
          aria-label="用于创作"
          :disabled="busy"
          @click="emit('use', prompt)"
        >
          <Icon name="sparkles" size="sm" />
          用于创作
        </button>
      </div>
    </div>
  </article>
</template>
