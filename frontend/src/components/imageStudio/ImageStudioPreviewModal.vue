<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageStudioAsset } from '@/api/imageStudio'
import imageStudioAPI from '@/api/imageStudio'
import { useAppStore } from '@/stores/app'
import { filenameForAsset, isExternalAssetUrl, isStudioAssetApiPath, saveBlob } from '@/utils/imageStudioBlob'
import { buildApiUrl } from '@/api/url'
import { trackGrowthEvent } from '@/utils/growthAnalytics'
import Icon from '@/components/icons/Icon.vue'

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
let previewRequestSequence = 0
let previewController: AbortController | null = null
let disposed = false

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

function isCurrentPreviewRequest(
  requestSequence: number,
  controller: AbortController,
  assetId: string,
) {
  return (
    !disposed
    && !controller.signal.aborted
    && requestSequence === previewRequestSequence
    && previewController === controller
    && props.asset?.id === assetId
  )
}

async function loadPreview() {
  previewController?.abort()
  const requestSequence = ++previewRequestSequence
  const controller = new AbortController()
  previewController = controller
  revokeObjectUrl()
  const asset = props.asset
  if (!asset) {
    if (previewController === controller) previewController = null
    return
  }
  try {
    if (isManagedAsset(asset)) {
      const blob = await imageStudioAPI.fetchImageStudioAssetBlob(
        asset.id,
        'content',
        controller.signal,
      )
      if (!blob || blob.size === 0 || String(blob.type || '').includes('json')) {
        throw new Error('invalid preview blob')
      }
      const nextObjectUrl = URL.createObjectURL(blob)
      if (!isCurrentPreviewRequest(requestSequence, controller, asset.id)) {
        URL.revokeObjectURL(nextObjectUrl)
        return
      }
      objectUrl = nextObjectUrl
      previewUrl.value = nextObjectUrl
      return
    }
    if (!isCurrentPreviewRequest(requestSequence, controller, asset.id)) return
    const raw = asset.preview_url || asset.url || ''
    previewUrl.value = isExternalAssetUrl(raw) ? raw : buildApiUrl(raw)
  } catch {
    if (!isCurrentPreviewRequest(requestSequence, controller, asset.id)) return
    appStore.showToast('error', t('imageStudio.previewFailed'))
  } finally {
    if (previewController === controller) previewController = null
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
  disposed = true
  previewRequestSequence += 1
  previewController?.abort()
  previewController = null
  window.removeEventListener('keydown', onKeydown)
  revokeObjectUrl()
})
</script>

<template>
  <Teleport to="body">
    <div v-if="asset" class="fixed inset-0 z-[200] flex items-center justify-center bg-gray-950/85 p-4 backdrop-blur-sm sm:p-8" role="dialog" aria-modal="true" @click.self="emit('close')">
      <div class="flex max-h-[94vh] w-full max-w-6xl flex-col gap-3">
        <div class="relative flex min-h-64 flex-1 items-center justify-center overflow-hidden rounded-xl bg-gray-900 shadow-2xl">
          <img v-if="previewUrl" :src="previewUrl" :alt="t('imageStudio.preview')" class="max-h-[82vh] max-w-full object-contain" />
          <div v-else class="py-20 text-sm text-gray-300">{{ t('imageStudio.loadingPreview') }}</div>
          <button type="button" class="absolute right-3 top-3 grid h-10 w-10 place-items-center rounded-lg bg-gray-950/70 text-white backdrop-blur hover:bg-gray-950" :title="t('imageStudio.closePreview')" :aria-label="t('imageStudio.closePreview')" @click="emit('close')">
            <Icon name="x" size="sm" />
          </button>
        </div>
        <div class="flex flex-wrap justify-center gap-2">
          <button type="button" class="btn btn-primary" @click="download">
            <Icon name="download" size="sm" />
            {{ t('imageStudio.download') }}
          </button>
          <button type="button" class="btn btn-secondary" @click="openNewTab">
            <Icon name="externalLink" size="sm" />
            {{ t('imageStudio.openNewTab') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
