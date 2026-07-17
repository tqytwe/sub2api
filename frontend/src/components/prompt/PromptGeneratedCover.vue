<script setup lang="ts">
import { computed } from 'vue'
import type { PromptSummary } from '@/api/prompts'
import { promptCoverBadge, promptCoverKicker, promptCoverTone } from '@/utils/promptCover'

const props = defineProps<{
  prompt: PromptSummary
  detail?: boolean
}>()

const toneClass = computed(() => promptCoverTone(props.prompt))
const kicker = computed(() => promptCoverKicker(props.prompt))
const badge = computed(() => promptCoverBadge(props.prompt))
</script>

<template>
  <div
    class="prompt-generated-cover"
    :class="[toneClass, { 'is-detail': detail }]"
    role="img"
    :aria-label="`${prompt.title}生成封面`"
  >
    <div class="prompt-generated-cover-grid" aria-hidden="true"></div>
    <div class="prompt-generated-cover-content">
      <span class="prompt-generated-cover-kicker">{{ kicker }}</span>
      <strong>{{ prompt.title }}</strong>
      <span class="prompt-generated-cover-badge">{{ badge }}</span>
    </div>
  </div>
</template>
