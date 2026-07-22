<script setup lang="ts">
import { computed } from 'vue'
import { renderAnnouncementMarkdown } from '@/utils/announcementMarkdown'

const props = withDefaults(defineProps<{
  content: string
  dense?: boolean
}>(), {
  dense: false,
})

const html = computed(() => renderAnnouncementMarkdown(props.content))
</script>

<template>
  <div
    class="announcement-content"
    :class="{ 'announcement-content--dense': dense }"
    v-html="html"
  />
</template>

<style scoped>
.announcement-content {
  @apply text-gray-700;
  font-size: 15px;
  line-height: 1.75;
}

.dark .announcement-content {
  @apply text-gray-300;
}

.announcement-content--dense {
  font-size: 14px;
  line-height: 1.65;
}

.announcement-content :deep(h1),
.announcement-content :deep(h2),
.announcement-content :deep(h3),
.announcement-content :deep(h4) {
  margin: 1.4em 0 0.65em;
  @apply text-gray-900;
  font-weight: 700;
  line-height: 1.25;
}

.dark .announcement-content :deep(h1),
.dark .announcement-content :deep(h2),
.dark .announcement-content :deep(h3),
.dark .announcement-content :deep(h4) {
  @apply text-white;
}

.announcement-content :deep(p),
.announcement-content :deep(ul),
.announcement-content :deep(ol),
.announcement-content :deep(pre),
.announcement-content :deep(table),
.announcement-content :deep(blockquote) {
  margin: 0 0 1em;
}

.announcement-content :deep(ul),
.announcement-content :deep(ol) {
  padding-left: 1.35rem;
}

.announcement-content :deep(ul) {
  list-style: disc;
}

.announcement-content :deep(ol) {
  list-style: decimal;
}

.announcement-content :deep(a) {
  @apply text-blue-600;
  font-weight: 600;
  text-decoration: underline;
  text-underline-offset: 3px;
}

.announcement-content :deep(code) {
  border-radius: 6px;
  @apply bg-gray-100 text-pink-600;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.9em;
  padding: 0.15rem 0.35rem;
}

.dark .announcement-content :deep(code) {
  @apply bg-gray-800 text-pink-400;
}

.announcement-content :deep(pre) {
  overflow-x: auto;
  @apply border border-gray-200 bg-gray-50;
  border-radius: 10px;
  padding: 1rem;
}

.dark .announcement-content :deep(pre) {
  @apply border-gray-700 bg-gray-900/65;
}

.announcement-content :deep(pre code) {
  background: transparent;
  color: inherit;
  padding: 0;
}

.announcement-content :deep(img) {
  display: block;
  max-width: 100%;
  max-height: min(58vh, 560px);
  margin: 1rem auto;
  @apply border border-gray-200;
  border-radius: 8px;
  object-fit: contain;
}

.dark .announcement-content :deep(img) {
  @apply border-gray-700;
}

.announcement-content :deep(mark) {
  border-radius: 5px;
  @apply bg-yellow-200 text-yellow-900;
  padding: 0.05rem 0.25rem;
}

.announcement-content :deep([data-announcement-tone]) {
  border-radius: 5px;
  font-weight: 650;
  padding: 0.05rem 0.25rem;
}

.announcement-content :deep([data-announcement-tone="info"]) {
  @apply bg-blue-100 text-blue-700;
}

.announcement-content :deep([data-announcement-tone="success"]) {
  @apply bg-green-100 text-green-700;
}

.announcement-content :deep([data-announcement-tone="warning"]) {
  @apply bg-amber-100 text-amber-700;
}

.announcement-content :deep([data-announcement-tone="danger"]) {
  @apply bg-red-100 text-red-700;
}

.announcement-content :deep([data-announcement-tone="muted"]) {
  @apply bg-gray-100 text-gray-600;
}

.announcement-content :deep(blockquote) {
  @apply border-l-4 border-blue-500 bg-blue-50;
  border-radius: 0 8px 8px 0;
  padding: 0.85rem 1rem;
}

.announcement-content :deep(blockquote[data-announcement-alert="success"]) {
  @apply border-l-green-500 bg-green-50;
}

.announcement-content :deep(blockquote[data-announcement-alert="warning"]) {
  @apply border-l-amber-500 bg-amber-50;
}

.announcement-content :deep(blockquote[data-announcement-alert="danger"]) {
  @apply border-l-red-500 bg-red-50;
}

.dark .announcement-content :deep(blockquote) {
  @apply bg-blue-950/20;
}

.dark .announcement-content :deep(blockquote[data-announcement-alert="success"]) {
  @apply bg-green-950/20;
}

.dark .announcement-content :deep(blockquote[data-announcement-alert="warning"]) {
  @apply bg-amber-950/25;
}

.dark .announcement-content :deep(blockquote[data-announcement-alert="danger"]) {
  @apply bg-red-950/20;
}

.announcement-content :deep(table) {
  width: 100%;
  border-collapse: collapse;
  overflow: hidden;
  @apply border border-gray-200;
  border-radius: 8px;
}

.announcement-content :deep(th),
.announcement-content :deep(td) {
  @apply border-b border-gray-200;
  padding: 0.65rem 0.8rem;
  text-align: left;
}

.announcement-content :deep(th) {
  @apply bg-gray-50 text-gray-900;
  font-weight: 700;
}
</style>
