<template>
  <details class="max-h-48 overflow-auto rounded border border-gray-200 p-3 text-sm dark:border-dark-700" data-testid="pelican-question-snapshot">
    <summary>{{ t('pelicanShowcase.questionSnapshot.title') }}<span v-if="question"> · {{ question.id }} · {{ question.version }}</span></summary>
    <template v-if="question">
      <p class="mt-2 break-all text-xs text-gray-500">{{ question.source }} · {{ question.round_id }}</p>
      <pre class="mt-2 whitespace-pre-wrap">{{ actualPrompt || question.prompt }}</pre>
      <ul class="mt-2 list-inside list-disc"><li v-for="criterion in question.criteria" :key="criterion">{{ criterion }}</li></ul>
    </template>
    <template v-else><p>{{ t('pelicanShowcase.questionSnapshot.legacy') }}</p><pre v-if="actualPrompt" class="whitespace-pre-wrap">{{ actualPrompt }}</pre></template>
  </details>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { PelicanQuestion } from '@/types'
defineProps<{ question?: PelicanQuestion; actualPrompt?: string }>()
const { t } = useI18n()
</script>
