<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import imageStudioAPI, { type ImageStudioAsset, type ImageStudioJob } from '@/api/imageStudio'
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
}>()

const emit = defineEmits<{
  preview: [asset: ImageStudioAsset, jobId: string, index: number]
  delete: [jobId: string]
  regenerate: [job: ImageStudioJob]
}>()

const { t, locale } = useI18n()
const appStore = useAppStore()
const thumbUrls = ref<Record<string, string>>({})
const loadingAssets = ref<Set<string>>(new Set())
const failedAssets = ref<Set<string>>(new Set())
let disposed = false

const displayJobs = computed(() => {
  if ((props.resultMode || props.featured) && props.latestJob) return [props.latestJob]
  return props.jobs
})

const isFeatured = computed(() => props.featured || props.resultMode)

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
  if (!isManagedAsset(asset) || thumbUrls.value[asset.id] || loadingAssets.value.has(asset.id)) return
  loadingAssets.value.add(asset.id)
  failedAssets.value.delete(asset.id)
  try {
    const blob = await imageStudioAPI.fetchImageStudioAssetBlob(asset.id, 'content')
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

  for (const job of displayJobs.value) {
    for (const asset of job.assets || []) {
      void ensureThumb(asset)
    }
  }
}

watch(displayJobs, syncThumbs, { immediate: true, deep: true })

onUnmounted(() => {
  disposed = true
  Object.values(thumbUrls.value).forEach((url) => URL.revokeObjectURL(url))
  thumbUrls.value = {}
})

function thumbSrc(asset: ImageStudioAsset) {
  if (thumbUrls.value[asset.id]) return thumbUrls.value[asset.id]
  if (isManagedAsset(asset)) return ''
  return legacySrc(asset)
}

function jobMissingAssets(job: ImageStudioJob) {
  return job.status === 'completed' && !(job.assets && job.assets.length > 0)
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

function statusLabel(status: string) {
  const key = `imageStudio.status.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

function formatCreatedAt(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const localeValue = typeof locale?.value === 'string' ? locale.value : 'zh-CN'
  const activeLocale = localeValue.startsWith('zh') ? 'zh-CN' : localeValue
  return new Intl.DateTimeFormat(activeLocale, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
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
          :class="job.status === 'completed' ? 'text-emerald-600 dark:text-emerald-400' : job.status === 'failed' ? 'text-red-600 dark:text-red-400' : 'text-amber-600 dark:text-amber-400'"
        >
          <span class="h-1.5 w-1.5 rounded-full bg-current" />
          {{ statusLabel(job.status) }}
        </span>
      </div>

      <div v-if="job.status === 'failed'" class="mx-3 mb-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300" :class="{ 'mx-0': isFeatured }">
        {{ truncateError(job.error_message) }}
      </div>
      <div v-else-if="jobMissingAssets(job)" class="mx-3 mb-3 flex min-h-40 items-center justify-center rounded-lg bg-gray-50 px-4 text-center text-sm text-gray-500 dark:bg-dark-900 dark:text-gray-400" :class="{ 'mx-0 min-h-72': isFeatured }">
        {{ t('imageStudio.assetsMissingHint') }}
      </div>

      <div
        v-if="job.assets?.length"
        class="grid gap-2"
        :class="isFeatured ? (job.assets.length > 1 ? 'sm:grid-cols-2' : 'grid-cols-1') : (job.assets.length > 1 ? 'grid-cols-2 px-3' : 'grid-cols-1 px-3')"
      >
        <div v-for="(asset, index) in job.assets" :key="asset.id" class="group relative min-w-0 overflow-hidden rounded-lg bg-gray-100 dark:bg-dark-900">
          <button type="button" class="block w-full focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" @click="openPreview(asset, job.id, index)">
            <img
              v-if="thumbSrc(asset) && !failedAssets.has(asset.id)"
              :src="thumbSrc(asset)"
              :alt="job.template_id"
              class="w-full object-cover transition duration-200 group-hover:scale-[1.01]"
              :class="isFeatured ? 'max-h-[58vh] min-h-64 object-contain' : 'aspect-square'"
              loading="lazy"
            />
            <div v-else class="flex items-center justify-center px-4 text-center text-sm text-gray-500 dark:text-gray-400" :class="isFeatured ? 'min-h-72' : 'aspect-square'">
              {{ failedAssets.has(asset.id) ? t('imageStudio.previewFailed') : t('imageStudio.loadingPreview') }}
            </div>
          </button>
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
          <span>·</span>
          <span>${{ (job.actual_cost ?? job.estimated_cost).toFixed(4) }}</span>
        </div>
        <div class="flex gap-1">
          <button
            v-if="job.status === 'failed' || job.status === 'completed'"
            type="button"
            class="btn-icon grid h-9 w-9 place-items-center rounded-lg text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-primary-300"
            :title="t('imageStudio.reuseSettings')"
            :aria-label="t('imageStudio.reuseSettings')"
            @click="emit('regenerate', job)"
          >
            <Icon name="refresh" size="sm" />
          </button>
          <button
            v-if="!isFeatured"
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
