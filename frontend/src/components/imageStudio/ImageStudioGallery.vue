<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import imageStudioAPI, { type ImageStudioAsset, type ImageStudioJob } from '@/api/imageStudio'
import { useAppStore } from '@/stores/app'
import { filenameForAsset, isExternalAssetUrl, isStudioAssetApiPath, saveBlob } from '@/utils/imageStudioBlob'
import { buildApiUrl } from '@/api/url'
import { trackGrowthEvent } from '@/utils/growthAnalytics'

const props = defineProps<{
  jobs: ImageStudioJob[]
  latestJob?: ImageStudioJob | null
  resultMode?: boolean
}>()

const emit = defineEmits<{
  preview: [asset: ImageStudioAsset, jobId: string, index: number]
  delete: [jobId: string]
  regenerate: [job: ImageStudioJob]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const thumbUrls = ref<Record<string, string>>({})
const loadingAssets = ref<Set<string>>(new Set())

const displayJobs = computed(() => {
  if (props.resultMode && props.latestJob) return [props.latestJob]
  return props.jobs
})

function assetApiPath(asset: ImageStudioAsset) {
  return asset.preview_url || asset.url || ''
}

function isManagedAsset(asset: ImageStudioAsset) {
  const raw = assetApiPath(asset)
  return !!asset.id && (isStudioAssetApiPath(raw) || !isExternalAssetUrl(raw))
}

function legacySrc(asset: ImageStudioAsset) {
  const raw = assetApiPath(asset)
  if (!raw) return ''
  if (isExternalAssetUrl(raw)) return raw
  return buildApiUrl(raw)
}

async function ensureThumb(asset: ImageStudioAsset) {
  if (!isManagedAsset(asset) || thumbUrls.value[asset.id] || loadingAssets.value.has(asset.id)) return
  loadingAssets.value.add(asset.id)
  try {
    const blob = await imageStudioAPI.fetchImageStudioAssetBlob(asset.id, 'content')
    thumbUrls.value = { ...thumbUrls.value, [asset.id]: URL.createObjectURL(blob) }
  } catch {
    // leave broken image; user can retry download
  } finally {
    loadingAssets.value.delete(asset.id)
  }
}

function syncThumbs() {
  for (const job of displayJobs.value) {
    for (const asset of job.assets || []) {
      void ensureThumb(asset)
    }
  }
}

watch(displayJobs, syncThumbs, { immediate: true, deep: true })

onUnmounted(() => {
  Object.values(thumbUrls.value).forEach((url) => URL.revokeObjectURL(url))
})

function thumbSrc(asset: ImageStudioAsset) {
  if (thumbUrls.value[asset.id]) return thumbUrls.value[asset.id]
  return legacySrc(asset)
}

function openPreview(asset: ImageStudioAsset, jobId: string, index: number) {
  emit('preview', asset, jobId, index)
}

async function downloadAsset(asset: ImageStudioAsset, jobId: string, index: number) {
  trackGrowthEvent('image_studio_download_click', { job_id: jobId, asset_id: asset.id })
  try {
    if (isManagedAsset(asset)) {
      const blob = await imageStudioAPI.fetchImageStudioAssetBlob(asset.id, 'download')
      const filename = filenameForAsset(jobId, index, blob.type || asset.content_type)
      saveBlob(blob, filename)
    } else {
      const raw = asset.download_url || asset.preview_url || asset.url
      if (!raw) throw new Error('missing url')
      const href = isExternalAssetUrl(raw) ? raw : buildApiUrl(raw)
      const res = await fetch(href)
      if (!res.ok) throw new Error('download failed')
      saveBlob(await res.blob(), filenameForAsset(jobId, index, asset.content_type))
    }
    trackGrowthEvent('image_studio_download_success', { job_id: jobId, asset_id: asset.id })
  } catch {
    appStore.showToast('error', t('imageStudio.downloadFailed'))
    trackGrowthEvent('image_studio_download_fail', { job_id: jobId, asset_id: asset.id })
  }
}

function confirmDelete(jobId: string) {
  if (window.confirm(t('imageStudio.deleteConfirm'))) {
    emit('delete', jobId)
  }
}

function truncateError(msg?: string) {
  const text = String(msg || '').trim()
  if (!text) return t('imageStudio.generateFailed')
  return text.length > 120 ? `${text.slice(0, 120)}…` : text
}
</script>

<template>
  <div v-if="!displayJobs.length" class="gw-subtitle">{{ t('imageStudio.galleryEmpty') }}</div>
  <div v-else class="gw-gallery gw-gallery--studio" :class="{ 'gw-gallery--result': resultMode }">
    <div v-for="job in displayJobs" :key="job.id" class="space-y-2">
      <div class="flex flex-wrap items-center gap-2 text-xs" style="color: var(--gw-ink-3)">
        <span class="gw-size-badge">{{ job.size }}</span>
        <span v-if="job.status === 'failed'" class="gw-error text-xs">{{ truncateError(job.error_message) }}</span>
      </div>
      <div v-for="(asset, index) in job.assets || []" :key="asset.id" class="gw-thumb gw-thumb--studio">
        <button type="button" class="gw-thumb-btn" @click="openPreview(asset, job.id, index)">
          <img :src="thumbSrc(asset)" :alt="job.template_id" loading="lazy" />
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
      <div class="flex flex-wrap gap-3">
        <button
          v-if="job.status === 'failed' || job.status === 'completed'"
          type="button"
          class="text-xs"
          style="color: var(--gw-ink-2)"
          @click="emit('regenerate', job)"
        >
          {{ t('imageStudio.regenerateSame') }}
        </button>
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
  </div>
</template>
