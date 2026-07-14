<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioAsset, ImageStudioJob } from '@/api/imageStudio'
import { buildApiUrl } from '@/api/url'

const props = defineProps<{
  jobs: ImageStudioJob[]
  latestJob?: ImageStudioJob | null
  resultMode?: boolean
}>()

const emit = defineEmits<{
  preview: [url: string, jobId: string, index: number]
  delete: [jobId: string]
}>()

const { t } = useI18n()
const previewTarget = ref<{ url: string; jobId: string; index: number } | null>(null)

const displayJobs = computed(() => {
  if (props.resultMode && props.latestJob) return [props.latestJob]
  return props.jobs
})

function assetUrl(asset: ImageStudioAsset) {
  return asset.preview_url || asset.url || ''
}

function resolveAssetSrc(asset: ImageStudioAsset) {
  const raw = assetUrl(asset)
  if (!raw) return ''
  if (raw.startsWith('http://') || raw.startsWith('https://') || raw.startsWith('data:')) return raw
  return buildApiUrl(raw)
}

function downloadUrl(asset: ImageStudioAsset) {
  return asset.download_url || asset.preview_url || asset.url
}

function assetFilename(jobId: string, index: number) {
  return `image-studio-${jobId.slice(0, 8)}-${index + 1}.png`
}

function openPreview(asset: ImageStudioAsset, jobId: string, index: number) {
  const url = assetUrl(asset)
  if (!url) return
  previewTarget.value = { url, jobId, index }
  emit('preview', url, jobId, index)
}

async function downloadAsset(asset: ImageStudioAsset, jobId: string, index: number) {
  const href = downloadUrl(asset)
  if (!href) return
  const full = href.startsWith('http') || href.startsWith('data:') ? href : buildApiUrl(href)
  const filename = assetFilename(jobId, index)
  try {
    const res = await fetch(full, { credentials: 'include' })
    if (!res.ok) throw new Error('download failed')
    const blob = await res.blob()
    const objectUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = filename
    a.click()
    URL.revokeObjectURL(objectUrl)
  } catch {
    window.open(full, '_blank', 'noopener,noreferrer')
  }
}

function confirmDelete(jobId: string) {
  if (window.confirm(t('imageStudio.deleteConfirm'))) {
    emit('delete', jobId)
  }
}
</script>

<template>
  <div v-if="!displayJobs.length" class="gw-subtitle">{{ t('imageStudio.galleryEmpty') }}</div>
  <div v-else class="gw-gallery gw-gallery--studio" :class="{ 'gw-gallery--result': resultMode }">
    <div v-for="job in displayJobs" :key="job.id" class="space-y-2">
      <div v-for="(asset, index) in job.assets || []" :key="asset.id" class="gw-thumb gw-thumb--studio">
        <button type="button" class="gw-thumb-btn" @click="openPreview(asset, job.id, index)">
          <img :src="resolveAssetSrc(asset)" :alt="job.template_id" loading="lazy" />
        </button>
        <div class="gw-thumb-actions">
          <button type="button" class="gw-thumb-action" @click="openPreview(asset, job.id, index)">
            {{ t('imageStudio.preview') }}
          </button>
          <button type="button" class="gw-thumb-action" @click="downloadAsset(asset, job.id, index)">
            {{ t('imageStudio.download') }}
          </button>
        </div>
      </div>
      <button
        v-if="!resultMode"
        type="button"
        class="text-xs"
        style="color: var(--gw-ink-3)"
        @click="confirmDelete(job.id)"
      >
        {{ t('imageStudio.delete') }}
      </button>
    </div>
  </div>
</template>
