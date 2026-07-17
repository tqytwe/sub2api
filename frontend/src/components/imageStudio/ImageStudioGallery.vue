<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import imageStudioAPI, {
  isImageStudioJobActive,
  isImageStudioJobTerminal,
  type ImageStudioAsset,
  type ImageStudioJob,
} from '@/api/imageStudio'
import { useAppStore } from '@/stores/app'
import { filenameForAsset, isExternalAssetUrl, isStudioAssetApiPath, saveBlob } from '@/utils/imageStudioBlob'
import { buildApiUrl } from '@/api/url'
import { trackGrowthEvent } from '@/utils/growthAnalytics'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  jobs: ImageStudioJob[]
  latestJob?: ImageStudioJob | null
  resultMode?: boolean
  featured?: boolean
  cancelingJobIds?: Set<string>
}>()

const emit = defineEmits<{
  preview: [asset: ImageStudioAsset, jobId: string, index: number]
  cancel: [jobId: string]
  delete: [jobId: string]
  regenerate: [job: ImageStudioJob]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const thumbUrls = ref<Record<string, string>>({})
const loadingAssets = ref<Set<string>>(new Set())
const failedAssets = ref<Set<string>>(new Set())
const attemptedAssets = new Set<string>()
const contentFallbackAssets = new Set<string>()
const downloadingJobIds = ref<Set<string>>(new Set())
const measuredDimensions = ref<Record<string, { width: number; height: number }>>({})
const thumbnailElements = new Map<string, Element>()
const thumbnailAssets = new Map<string, ImageStudioAsset>()
let thumbnailObserver: IntersectionObserver | null = null
let disposed = false

const displayJobs = computed(() => {
  if ((props.resultMode || props.featured) && props.latestJob) return [props.latestJob]
  return props.jobs
})

const isFeatured = computed(() => props.featured || props.resultMode)

function assetApiPath(asset: ImageStudioAsset) {
  return asset.thumbnail_url || asset.preview_url || asset.url || ''
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

function displayedManagedAssetIds() {
  const ids = new Set<string>()
  for (const job of displayJobs.value) {
    for (const asset of job.assets || []) {
      if (isManagedAsset(asset)) ids.add(asset.id)
    }
  }
  return ids
}

async function ensureThumb(asset: ImageStudioAsset) {
  if (
    !isManagedAsset(asset)
    || thumbUrls.value[asset.id]
    || loadingAssets.value.has(asset.id)
    || attemptedAssets.has(asset.id)
  ) return
  attemptedAssets.add(asset.id)
  loadingAssets.value.add(asset.id)
  failedAssets.value.delete(asset.id)
  try {
    let blob: Blob
    if (asset.thumbnail_url) {
      try {
        blob = await imageStudioAPI.fetchImageStudioAssetBlob(asset.id, 'thumbnail')
        if (!blob || blob.size === 0 || String(blob.type || '').includes('json')) {
          throw new Error('empty or invalid thumbnail blob')
        }
      } catch {
        if (disposed || !displayedManagedAssetIds().has(asset.id)) return
        if (contentFallbackAssets.has(asset.id)) {
          throw new Error('content fallback already attempted')
        }
        contentFallbackAssets.add(asset.id)
        blob = await imageStudioAPI.fetchImageStudioAssetBlob(asset.id, 'content')
      }
    } else {
      blob = await imageStudioAPI.fetchImageStudioAssetBlob(asset.id, 'content')
    }
    if (!blob || blob.size === 0 || String(blob.type || '').includes('json')) {
      throw new Error('empty or invalid asset blob')
    }
    const objectUrl = URL.createObjectURL(blob)
    if (disposed || !displayedManagedAssetIds().has(asset.id)) {
      URL.revokeObjectURL(objectUrl)
      return
    }
    const previousUrl = thumbUrls.value[asset.id]
    if (previousUrl && previousUrl !== objectUrl) URL.revokeObjectURL(previousUrl)
    thumbUrls.value = { ...thumbUrls.value, [asset.id]: objectUrl }
  } catch {
    if (!disposed && displayedManagedAssetIds().has(asset.id)) failedAssets.value.add(asset.id)
  } finally {
    loadingAssets.value.delete(asset.id)
  }
}

function syncThumbs() {
  const displayedIds = displayedManagedAssetIds()
  const retainedUrls = { ...thumbUrls.value }
  for (const [assetId, url] of Object.entries(retainedUrls)) {
    if (displayedIds.has(assetId)) continue
    URL.revokeObjectURL(url)
    delete retainedUrls[assetId]
  }
  thumbUrls.value = retainedUrls
  failedAssets.value = new Set([...failedAssets.value].filter((assetId) => displayedIds.has(assetId)))
  for (const assetId of attemptedAssets) {
    if (displayedIds.has(assetId)) continue
    attemptedAssets.delete(assetId)
    contentFallbackAssets.delete(assetId)
  }
  measuredDimensions.value = Object.fromEntries(
    Object.entries(measuredDimensions.value).filter(([assetId]) => displayedIds.has(assetId)),
  )

  for (const job of displayJobs.value) {
    for (const asset of job.assets || []) {
      if (isManagedAsset(asset)) thumbnailAssets.set(asset.id, asset)
    }
  }
  for (const [assetId, element] of thumbnailElements) {
    if (displayedIds.has(assetId)) continue
    thumbnailObserver?.unobserve(element)
    thumbnailElements.delete(assetId)
    thumbnailAssets.delete(assetId)
  }
}

watch(displayJobs, syncThumbs, { immediate: true, deep: true })

onUnmounted(() => {
  disposed = true
  thumbnailObserver?.disconnect()
  thumbnailObserver = null
  thumbnailElements.clear()
  thumbnailAssets.clear()
  attemptedAssets.clear()
  contentFallbackAssets.clear()
  Object.values(thumbUrls.value).forEach((url) => URL.revokeObjectURL(url))
  thumbUrls.value = {}
})

function getThumbnailObserver() {
  if (thumbnailObserver || typeof IntersectionObserver === 'undefined') {
    return thumbnailObserver
  }
  thumbnailObserver = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue
      const assetId = [...thumbnailElements.entries()]
        .find(([, element]) => element === entry.target)?.[0]
      if (!assetId) continue
      const asset = thumbnailAssets.get(assetId)
      thumbnailObserver?.unobserve(entry.target)
      thumbnailElements.delete(assetId)
      if (asset) void ensureThumb(asset)
    }
  }, {
    root: null,
    rootMargin: '240px 0px',
    threshold: 0.01,
  })
  return thumbnailObserver
}

function setThumbnailTarget(asset: ImageStudioAsset, element: Element | null) {
  if (!isManagedAsset(asset)) return
  thumbnailAssets.set(asset.id, asset)
  const previous = thumbnailElements.get(asset.id)
  if (previous && previous !== element) thumbnailObserver?.unobserve(previous)
  if (!element) {
    thumbnailElements.delete(asset.id)
    return
  }
  if (thumbUrls.value[asset.id] || loadingAssets.value.has(asset.id)) return
  const observer = getThumbnailObserver()
  if (!observer) {
    void ensureThumb(asset)
    return
  }
  thumbnailElements.set(asset.id, element)
  observer.observe(element)
}

function thumbSrc(asset: ImageStudioAsset) {
  if (thumbUrls.value[asset.id]) return thumbUrls.value[asset.id]
  if (isManagedAsset(asset)) return ''
  return legacySrc(asset)
}

function jobMissingAssets(job: ImageStudioJob) {
  return (
    (job.status === 'completed' || job.status === 'partial')
    && !(job.assets && job.assets.length > 0)
  )
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

async function downloadJob(job: ImageStudioJob) {
  if (downloadingJobIds.value.has(job.id)) return
  const next = new Set(downloadingJobIds.value)
  next.add(job.id)
  downloadingJobIds.value = next
  trackGrowthEvent('image_studio_download_click', { job_id: job.id, download_scope: 'job_zip' })
  try {
    await imageStudioAPI.downloadImageStudioJob(job.id)
    trackGrowthEvent('image_studio_download_success', { job_id: job.id, download_scope: 'job_zip' })
  } catch {
    appStore.showToast('error', t('imageStudio.downloadFailed'))
    trackGrowthEvent('image_studio_download_fail', { job_id: job.id, download_scope: 'job_zip' })
  } finally {
    const remaining = new Set(downloadingJobIds.value)
    remaining.delete(job.id)
    downloadingJobIds.value = remaining
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

function statusLabel(status: string) {
  const key = `imageStudio.status.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

function itemCounts(job: ImageStudioJob) {
  const itemStatuses = job.items?.map((item) => String(item.status || '').toLowerCase()) ?? []
  const countedSuccess = itemStatuses
    .filter((status) => ['completed', 'succeeded', 'success'].includes(status))
    .length
  const success = job.success_count
    ?? (job.items?.length ? countedSuccess : (job.assets?.length ?? 0))
  const failed = job.fail_count
    ?? itemStatuses.filter((status) => ['failed', 'error'].includes(status)).length
  return {
    success,
    failed,
    finished: Math.min(job.count, success + failed),
  }
}

function hasItemCounts(job: ImageStudioJob) {
  return job.success_count !== undefined || job.fail_count !== undefined || !!job.items?.length
}

function statusClass(job: ImageStudioJob) {
  if (job.status === 'completed') return 'text-emerald-600 dark:text-emerald-400'
  if (job.status === 'partial') return 'text-orange-600 dark:text-orange-300'
  if (job.status === 'failed' || job.status === 'cancelled') return 'text-red-600 dark:text-red-400'
  return 'text-amber-600 dark:text-amber-400'
}

function positiveDimension(value: unknown) {
  const dimension = Number(value)
  return Number.isFinite(dimension) && dimension > 0 ? dimension : null
}

function nativeDimensionsForAsset(asset: ImageStudioAsset) {
  const width = positiveDimension(asset.width)
  const height = positiveDimension(asset.height)
  if (width && height) return { width, height }
  return null
}

function dimensionsForAsset(asset: ImageStudioAsset) {
  const nativeDimensions = nativeDimensionsForAsset(asset)
  if (nativeDimensions) return nativeDimensions
  return measuredDimensions.value[asset.id] ?? null
}

function aspectRatioForAsset(asset: ImageStudioAsset) {
  const dimensions = dimensionsForAsset(asset)
  if (dimensions) return `${dimensions.width} / ${dimensions.height}`
  const ratioMatch = String(asset.aspect_ratio || '').match(
    /^\s*(\d+(?:\.\d+)?)\s*[:/]\s*(\d+(?:\.\d+)?)\s*$/,
  )
  if (ratioMatch && Number(ratioMatch[1]) > 0 && Number(ratioMatch[2]) > 0) {
    return `${ratioMatch[1]} / ${ratioMatch[2]}`
  }
  return '1 / 1'
}

function assetDimensionsLabel(asset: ImageStudioAsset) {
  const dimensions = dimensionsForAsset(asset)
  if (!dimensions) return ''
  return t('imageStudio.assetDimensions', dimensions)
}

function recordNaturalDimensions(asset: ImageStudioAsset, event: Event) {
  if (nativeDimensionsForAsset(asset)) return
  const image = event.currentTarget as HTMLImageElement | null
  const width = positiveDimension(image?.naturalWidth)
  const height = positiveDimension(image?.naturalHeight)
  if (!width || !height) return
  measuredDimensions.value = {
    ...measuredDimensions.value,
    [asset.id]: { width, height },
  }
}

function formatCreatedAt(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date)
}
</script>

<template>
  <div v-if="!displayJobs.length" class="flex min-h-40 items-center justify-center rounded-xl border border-dashed border-gray-200 bg-gray-50 px-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-400">
    {{ t('imageStudio.galleryEmpty') }}
  </div>
  <div
    v-else
    :class="isFeatured ? 'space-y-4' : 'grid gap-4 sm:grid-cols-2 xl:grid-cols-3'"
  >
    <article
      v-for="job in displayJobs"
      :key="job.id"
      class="min-w-0 overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800"
      :class="{ 'border-0 rounded-none dark:border-0': isFeatured }"
    >
      <div class="flex flex-wrap items-center justify-between gap-2 px-3 py-2.5 text-xs text-gray-500 dark:text-gray-400" :class="{ 'px-0 pt-0': isFeatured }">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <span class="rounded-md bg-gray-100 px-2 py-1 font-mono text-[11px] text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ job.size }}</span>
          <span>{{ formatCreatedAt(job.created_at) }}</span>
        </div>
        <span
          class="inline-flex items-center gap-1 font-medium"
          :class="statusClass(job)"
        >
          <span class="h-1.5 w-1.5 rounded-full bg-current" />
          {{ statusLabel(job.status) }}
        </span>
      </div>

      <div v-if="job.status === 'failed'" class="mx-3 mb-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300" :class="{ 'mx-0': isFeatured }">
        {{ truncateError(job.error_message) }}
      </div>
      <div
        v-else-if="isImageStudioJobActive(job)"
        class="mx-3 mb-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-xs text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200"
        :class="{ 'mx-0': isFeatured }"
      >
        <div class="flex items-center justify-between gap-3">
          <span>{{ t('imageStudio.jobProgress', { finished: itemCounts(job).finished, count: job.count }) }}</span>
          <span class="font-medium tabular-nums">
            {{ t('imageStudio.itemCounts', { success: itemCounts(job).success, failed: itemCounts(job).failed }) }}
          </span>
        </div>
        <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-amber-200/70 dark:bg-amber-900/60">
          <div
            class="h-full rounded-full bg-amber-500 transition-all"
            :style="{ width: `${Math.max(6, Math.round((itemCounts(job).finished / Math.max(1, job.count)) * 100))}%` }"
          />
        </div>
      </div>
      <div v-else-if="jobMissingAssets(job)" class="mx-3 mb-3 flex min-h-40 items-center justify-center rounded-lg bg-gray-50 px-4 text-center text-sm text-gray-500 dark:bg-dark-900 dark:text-gray-400" :class="{ 'mx-0 min-h-72': isFeatured }">
        {{ t('imageStudio.assetsMissingHint') }}
      </div>

      <div
        v-if="job.assets?.length"
        class="grid gap-2"
        :class="isFeatured ? (job.assets.length > 1 ? 'sm:grid-cols-2' : 'grid-cols-1') : (job.assets.length > 1 ? 'grid-cols-2 px-3' : 'grid-cols-1 px-3')"
      >
        <div
          v-for="(asset, index) in job.assets"
          :key="asset.id"
          :ref="(element) => setThumbnailTarget(asset, element as Element | null)"
          data-testid="asset-frame"
          class="group relative min-w-0 overflow-hidden rounded-lg bg-gray-100 dark:bg-dark-900"
          :style="{ aspectRatio: aspectRatioForAsset(asset) }"
        >
          <button type="button" class="block h-full w-full focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" @click="openPreview(asset, job.id, index)">
            <img
              v-if="thumbSrc(asset) && !failedAssets.has(asset.id)"
              :src="thumbSrc(asset)"
              :alt="job.template_id"
              class="h-full w-full transition duration-200 group-hover:scale-[1.01]"
              :class="isFeatured ? 'object-contain' : 'object-cover'"
              loading="lazy"
              @load="recordNaturalDimensions(asset, $event)"
            />
            <div v-else class="flex h-full min-h-40 items-center justify-center px-4 text-center text-sm text-gray-500 dark:text-gray-400" :class="{ 'min-h-72': isFeatured }">
              {{ failedAssets.has(asset.id) ? t('imageStudio.previewFailed') : t('imageStudio.loadingPreview') }}
            </div>
          </button>
          <span
            v-if="assetDimensionsLabel(asset)"
            class="absolute bottom-2 left-2 rounded-md bg-gray-950/75 px-2 py-1 text-[11px] font-medium text-white backdrop-blur"
          >
            {{ assetDimensionsLabel(asset) }}
          </span>
          <div class="absolute right-2 top-2 flex gap-1 opacity-0 transition group-hover:opacity-100 group-focus-within:opacity-100">
            <button type="button" class="btn-icon grid h-9 w-9 place-items-center rounded-lg bg-white/95 text-gray-700 shadow-md backdrop-blur dark:bg-dark-700/95 dark:text-gray-100" :title="t('imageStudio.preview')" :aria-label="t('imageStudio.preview')" @click="openPreview(asset, job.id, index)">
              <Icon name="eye" size="sm" />
            </button>
            <button type="button" class="btn-icon grid h-9 w-9 place-items-center rounded-lg bg-white/95 text-gray-700 shadow-md backdrop-blur dark:bg-dark-700/95 dark:text-gray-100" :title="t('imageStudio.download')" :aria-label="t('imageStudio.download')" @click="downloadAsset(asset, job.id, index)">
              <Icon name="download" size="sm" />
            </button>
          </div>
        </div>
      </div>

      <div class="flex items-center justify-between gap-2 px-3 py-3" :class="{ 'px-0 pb-0': isFeatured }">
        <div class="flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span>{{ t('imageStudio.imageCount', { count: job.count }) }}</span>
          <template v-if="hasItemCounts(job)">
            <span>·</span>
            <span>{{ t('imageStudio.itemCounts', { success: itemCounts(job).success, failed: itemCounts(job).failed }) }}</span>
          </template>
          <span>·</span>
          <span>${{ (job.actual_cost ?? job.estimated_cost).toFixed(4) }}</span>
        </div>
        <div class="flex gap-1">
          <button
            v-if="job.status === 'completed' || job.status === 'partial'"
            type="button"
            class="btn-icon grid h-9 w-9 place-items-center rounded-lg text-gray-500 hover:bg-gray-100 hover:text-primary-600 disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-primary-300"
            :disabled="downloadingJobIds.has(job.id)"
            :title="t('imageStudio.downloadAll')"
            :aria-label="t('imageStudio.downloadAll')"
            @click="downloadJob(job)"
          >
            <Icon name="download" size="sm" :class="{ 'animate-pulse': downloadingJobIds.has(job.id) }" />
          </button>
          <button
            v-if="isImageStudioJobTerminal(job)"
            type="button"
            class="btn-icon grid h-9 w-9 place-items-center rounded-lg text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-primary-300"
            :title="t('imageStudio.reuseSettings')"
            :aria-label="t('imageStudio.reuseSettings')"
            @click="emit('regenerate', job)"
          >
            <Icon name="refresh" size="sm" />
          </button>
          <button
            v-if="isImageStudioJobActive(job)"
            type="button"
            class="btn-icon grid h-9 w-9 place-items-center rounded-lg text-amber-600 hover:bg-amber-50 hover:text-amber-700 disabled:opacity-50 dark:text-amber-300 dark:hover:bg-amber-950/30"
            :disabled="cancelingJobIds?.has(job.id)"
            :title="t('imageStudio.cancel')"
            :aria-label="t('imageStudio.cancel')"
            @click="emit('cancel', job.id)"
          >
            <Icon name="xCircle" size="sm" :class="{ 'animate-pulse': cancelingJobIds?.has(job.id) }" />
          </button>
          <button
            v-if="!isFeatured && isImageStudioJobTerminal(job)"
            type="button"
            class="btn-icon grid h-9 w-9 place-items-center rounded-lg text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30 dark:hover:text-red-300"
            :title="t('imageStudio.delete')"
            :aria-label="t('imageStudio.delete')"
            @click="confirmDelete(job.id)"
          >
            <Icon name="trash" size="sm" />
          </button>
        </div>
      </div>
    </article>
  </div>
</template>
