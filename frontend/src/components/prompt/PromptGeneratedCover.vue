<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PromptSummary } from '@/api/prompts'
import {
  promptCoverBadgeFallback,
  promptCoverBadgeMessageKey,
  promptCoverKickerFallback,
  promptCoverKickerMessageKey,
  promptCoverTone,
} from '@/utils/promptCover'

const props = defineProps<{
  prompt: PromptSummary
  detail?: boolean
}>()

const { t } = useI18n()
const toneClass = computed(() => promptCoverTone(props.prompt))
const kicker = computed(() => {
  const key = promptCoverKickerMessageKey(props.prompt)
  return key ? t(key) : promptCoverKickerFallback(props.prompt) || t('promptLibrary.cover.defaultKicker')
})
const badge = computed(() => {
  const key = promptCoverBadgeMessageKey(props.prompt)
  return key ? t(key) : promptCoverBadgeFallback(props.prompt) || t('promptLibrary.cover.defaultBadge')
})
</script>

<template>
  <div
    class="prompt-generated-cover"
    :class="[toneClass, { 'is-detail': detail }]"
    role="img"
    :aria-label="t('promptLibrary.card.generatedCoverAlt', { title: prompt.title })"
  >
    <div class="prompt-generated-cover-grid" aria-hidden="true"></div>
    <div class="prompt-generated-cover-content">
      <span class="prompt-generated-cover-kicker">{{ kicker }}</span>
      <strong>{{ prompt.title }}</strong>
      <span class="prompt-generated-cover-badge">{{ badge }}</span>
    </div>
  </div>
</template>
