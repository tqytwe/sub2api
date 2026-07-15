<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioAsset } from '@/api/imageStudio'
import imageStudioAPI from '@/api/imageStudio'
import { useAppStore } from '@/stores/app'
import { filenameForAsset, isExternalAssetUrl, isStudioAssetApiPath, saveBlob } from '@/utils/imageStudioBlob'
import { buildApiUrl } from '@/api/url'
import { trackGrowthEvent } from '@/utils/growthAnalytics'

const props = defineProps<{
  asset: ImageStudioAsset | null
  jobId: string
  index: number
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const previewUrl = ref<string | null>(null)
let objectUrl: string | null = null

function isManagedAsset(asset: ImageStudioAsset) {
  const raw = asset.preview_url || asset.url || ''
  return !!asset.id && (isStudioAssetApiPath(raw) || !isExternalAssetUrl(raw))
}

function revokeObjectUrl() {
  if (objectUrl) {
    URL.revokeObjectURL(objectUrl)
    objectUrl = null
  }
  previewUrl.value = null
}

async function loadPreview() {
  revokeObjectUrl()
  const asset = props.asset
  if (!asset) return
  try {
    if (isManagedAsset(asset)) {
      const blob = await imageStudioAPI.fetchImageStudioAssetBlob(asset.id, 'content')
      if (!blob || blob.size === 0 || String(blob.type || '').includes('json')) {
        throw new Error('invalid preview blob')
      }
      objectUrl = URL.createObjectURL(blob)
      previewUrl.value = objectUrl
      return
    }
    const raw = asset.preview_url || asset.url || ''
    previewUrl.value = isExternalAssetUrl(raw) ? raw : buildApiUrl(raw)
  } catch {
    appStore.showToast('error', t('imageStudio.previewFailed'))
  }
}

async function download() {
  const asset = props.asset
  if (!asset) return
  trackGrowthEvent('image_studio_download_click', { job_id: props.jobId, asset_id: asset.id })
  try {
    if (isManagedAsset(asset)) {
      await imageStudioAPI.downloadImageStudioAsset(asset, props.jobId, props.index)
    } else {
      const raw = asset.download_url || asset.preview_url || asset.url
      if (!raw) throw new Error('missing url')
      const href = isExternalAssetUrl(raw) ? raw : buildApiUrl(raw)
      const res = await fetch(href)
      if (!res.ok) throw new Error('download failed')
      saveBlob(await res.blob(), filenameForAsset(props.jobId, props.index, asset.content_type))
    }
    trackGrowthEvent('image_studio_download_success', { job_id: props.jobId, asset_id: asset.id })
  } catch {
    appStore.showToast('error', t('imageStudio.downloadFailed'))
    trackGrowthEvent('image_studio_download_fail', { job_id: props.jobId, asset_id: asset.id })
  }
}

async function openNewTab() {
  if (!previewUrl.value && props.asset) {
    await loadPreview()
  }
  if (!previewUrl.value) {
    appStore.showToast('error', t('imageStudio.previewFailed'))
    return
  }
  window.open(previewUrl.value, '_blank', 'noopener,noreferrer')
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

watch(() => props.asset?.id, () => { void loadPreview() }, { immediate: true })

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  revokeObjectUrl()
})
</script>

<template>
  <Teleport to="body">
    <div v-if="asset" class="gw-preview-overlay" @click.self="emit('close')">
      <div class="gw-preview-card">
        <img v-if="previewUrl" :src="previewUrl" alt="preview" />
        <div v-else class="gw-polling py-16">{{ t('imageStudio.loadingPreview') }}</div>
        <div class="gw-preview-toolbar">
          <button type="button" class="gw-btn gw-btn-primary" @click="download">
            {{ t('imageStudio.download') }}
          </button>
          <button type="button" class="gw-btn gw-btn-secondary" @click="openNewTab">
            {{ t('imageStudio.openNewTab') }}
          </button>
          <button type="button" class="gw-btn gw-btn-secondary" @click="emit('close')">
            {{ t('imageStudio.closePreview') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
