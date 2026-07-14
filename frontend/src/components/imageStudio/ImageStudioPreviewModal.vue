<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { buildApiUrl } from '@/api/url'

const props = defineProps<{
  url: string | null
  filename: string
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()

function resolveUrl(raw: string) {
  if (raw.startsWith('http://') || raw.startsWith('https://') || raw.startsWith('data:')) {
    return raw
  }
  return buildApiUrl(raw)
}

async function download() {
  if (!props.url) return
  const href = resolveUrl(props.url)
  try {
    const res = await fetch(href, { credentials: 'include' })
    if (!res.ok) throw new Error('download failed')
    const blob = await res.blob()
    const objectUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = props.filename
    a.click()
    URL.revokeObjectURL(objectUrl)
  } catch {
    window.open(href, '_blank', 'noopener,noreferrer')
  }
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <div v-if="url" class="gw-preview-overlay" @click.self="emit('close')">
      <div class="gw-preview-card">
        <img :src="resolveUrl(url)" alt="preview" />
        <div class="gw-preview-toolbar">
          <button type="button" class="gw-btn gw-btn-primary" @click="download">
            {{ t('imageStudio.download') }}
          </button>
          <a
            :href="resolveUrl(url)"
            class="gw-btn gw-btn-secondary"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ t('imageStudio.openNewTab') }}
          </a>
          <button type="button" class="gw-btn gw-btn-secondary" @click="emit('close')">
            {{ t('imageStudio.closePreview') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
