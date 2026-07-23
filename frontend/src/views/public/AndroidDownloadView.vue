<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import QRCode from 'qrcode'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PublicContentLayout from '@/components/layout/PublicContentLayout.vue'
import { useAppStore } from '@/stores'
import { localizedSiteName } from '@/utils/localizedPublicSettings'
import { sanitizeUrl } from '@/utils/url'

type AndroidManifest = {
  platform?: string
  version?: string
  versionCode?: number
  apkUrl?: string
  size?: string
  bytes?: number
  sha256?: string
  minAndroidVersion?: string
  releaseDate?: string
  notes?: string[]
}

type FeatureCopy = {
  title: string
  desc: string
}

const APK_PATH = '/downloads/jisudengchat-android.apk'
const MANIFEST_PATH = '/downloads/android-version.json'
const OFFICIAL_WEB_URL = 'https://www.jisudeng.com'

const { t, tm, locale } = useI18n()
const appStore = useAppStore()
const manifest = ref<AndroidManifest | null>(null)
const qrImage = ref('')
const manifestFailed = ref(false)

const siteName = computed(() =>
  localizedSiteName(appStore.cachedPublicSettings?.site_name || appStore.siteName, locale.value),
)
const siteLogo = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', {
    allowRelative: true,
    allowDataUrl: true,
  }),
)

function absoluteUrl(path: string): string {
  const base = typeof window === 'undefined' ? OFFICIAL_WEB_URL : window.location.origin
  try {
    return new URL(path, base).toString()
  } catch {
    return new URL(APK_PATH, OFFICIAL_WEB_URL).toString()
  }
}

const downloadUrl = computed(() => absoluteUrl(manifest.value?.apkUrl || APK_PATH))
const manifestUrl = computed(() => absoluteUrl(MANIFEST_PATH))
const notes = computed(() => manifest.value?.notes?.filter(Boolean) ?? [])
const featureRows = computed(() => {
  const raw = tm('androidDownload.features') as unknown
  if (!Array.isArray(raw)) return []
  return raw.filter((item): item is FeatureCopy => {
    return (
      typeof item === 'object' &&
      item !== null &&
      typeof (item as FeatureCopy).title === 'string' &&
      typeof (item as FeatureCopy).desc === 'string'
    )
  })
})
const manifestRows = computed(() => [
  {
    label: t('androidDownload.version'),
    value: manifest.value?.version || t('androidDownload.unknown'),
  },
  {
    label: t('androidDownload.size'),
    value: manifest.value?.size || t('androidDownload.defaultSize'),
  },
  {
    label: t('androidDownload.minAndroid'),
    value: manifest.value?.minAndroidVersion || '8.0',
  },
  {
    label: t('androidDownload.releaseDate'),
    value: manifest.value?.releaseDate || t('androidDownload.unknown'),
  },
])

onMounted(async () => {
  try {
    const response = await fetch(MANIFEST_PATH, {
      headers: { Accept: 'application/json' },
      cache: 'no-store',
    })
    if (!response.ok) throw new Error(`Manifest returned ${response.status}`)
    manifest.value = await response.json()
  } catch {
    manifestFailed.value = true
  } finally {
    qrImage.value = await QRCode.toDataURL(downloadUrl.value, {
      errorCorrectionLevel: 'M',
      margin: 1,
      width: 280,
    })
  }
})
</script>

<template>
  <PublicContentLayout :site-name="siteName" :site-logo="siteLogo" frame="content">
    <div class="space-y-8">
      <section class="grid gap-5 lg:grid-cols-[minmax(0,1.08fr)_minmax(280px,360px)]">
        <div class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-800 dark:bg-dark-900 sm:p-7">
          <p class="text-xs font-semibold uppercase tracking-[0.18em] text-gray-500 dark:text-dark-300">
            {{ t('androidDownload.eyebrow') }}
          </p>
          <h1 class="mt-3 text-3xl font-semibold tracking-normal text-gray-950 dark:text-white sm:text-4xl">
            {{ t('androidDownload.title') }}
          </h1>
          <p class="mt-4 text-base leading-7 text-gray-600 dark:text-dark-300">
            {{ t('androidDownload.lead') }}
          </p>

          <div class="mt-6 flex flex-wrap gap-3">
            <a
              :href="downloadUrl"
              download
              class="inline-flex min-h-11 items-center gap-2 rounded-lg bg-gray-950 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-gray-800 dark:bg-white dark:text-dark-950 dark:hover:bg-dark-100"
            >
              <Icon name="download" size="sm" />
              <span>{{ t('androidDownload.downloadApk') }}</span>
            </a>
            <a
              href="/"
              class="inline-flex min-h-11 items-center gap-2 rounded-lg border border-gray-200 px-4 py-2.5 text-sm font-semibold text-gray-700 transition-colors hover:bg-gray-50 dark:border-dark-700 dark:text-dark-200 dark:hover:bg-dark-800"
            >
              <Icon name="globe" size="sm" />
              <span>{{ t('androidDownload.openWeb') }}</span>
            </a>
          </div>

          <p v-if="manifestFailed" class="mt-4 text-sm text-amber-700 dark:text-amber-300">
            {{ t('androidDownload.manifestFailed') }}
          </p>
        </div>

        <aside class="rounded-lg border border-gray-200 bg-white p-5 text-center shadow-sm dark:border-dark-800 dark:bg-dark-900">
          <div class="flex justify-center">
            <div class="flex h-56 w-56 items-center justify-center rounded-lg border border-gray-100 bg-white p-3 dark:border-dark-700">
              <img v-if="qrImage" :src="qrImage" :alt="t('androidDownload.qrAlt')" class="h-full w-full" />
              <Icon v-else name="download" size="xl" class="text-gray-400" />
            </div>
          </div>
          <h2 class="mt-4 text-lg font-semibold text-gray-950 dark:text-white">
            {{ t('androidDownload.scanTitle') }}
          </h2>
          <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-300">
            {{ t('androidDownload.scanHint') }}
          </p>
        </aside>
      </section>

      <section class="grid gap-5 md:grid-cols-[minmax(0,1fr)_minmax(260px,340px)]">
        <div class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-800 dark:bg-dark-900">
          <h2 class="text-lg font-semibold text-gray-950 dark:text-white">
            {{ t('androidDownload.featureTitle') }}
          </h2>
          <div class="mt-4 grid gap-3 sm:grid-cols-2">
            <div
              v-for="feature in featureRows"
              :key="feature.title"
              class="rounded-lg border border-gray-100 p-4 dark:border-dark-800"
            >
              <h3 class="text-sm font-semibold text-gray-950 dark:text-white">{{ feature.title }}</h3>
              <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-300">{{ feature.desc }}</p>
            </div>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-800 dark:bg-dark-900">
          <div class="flex items-center gap-2 text-sm font-semibold text-gray-950 dark:text-white">
            <Icon name="infoCircle" size="sm" />
            <span>{{ t('androidDownload.packageInfo') }}</span>
          </div>
          <dl class="mt-4 space-y-3">
            <div v-for="row in manifestRows" :key="row.label" class="flex items-center justify-between gap-3 text-sm">
              <dt class="text-gray-500 dark:text-dark-300">{{ row.label }}</dt>
              <dd class="font-medium text-gray-900 dark:text-white">{{ row.value }}</dd>
            </div>
          </dl>
          <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-800">
            <p class="text-xs font-semibold uppercase tracking-[0.16em] text-gray-500 dark:text-dark-300">
              SHA256
            </p>
            <p class="mt-2 break-all font-mono text-xs text-gray-600 dark:text-dark-300">
              {{ manifest?.sha256 || t('androidDownload.unknown') }}
            </p>
          </div>
          <a
            :href="manifestUrl"
            class="mt-4 inline-flex items-center gap-1.5 text-sm font-semibold text-gray-700 hover:text-gray-950 dark:text-dark-300 dark:hover:text-white"
          >
            <Icon name="document" size="sm" />
            <span>{{ t('androidDownload.viewManifest') }}</span>
          </a>
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-800 dark:bg-dark-900">
        <h2 class="text-lg font-semibold text-gray-950 dark:text-white">
          {{ t('androidDownload.notesTitle') }}
        </h2>
        <ul class="mt-4 grid gap-3 text-sm leading-6 text-gray-600 dark:text-dark-300">
          <li v-for="note in notes.length ? notes : [t('androidDownload.defaultNote')]" :key="note" class="flex gap-2">
            <Icon name="checkCircle" size="sm" class="mt-1 flex-none text-emerald-600 dark:text-emerald-300" />
            <span>{{ note }}</span>
          </li>
        </ul>
      </section>
    </div>
  </PublicContentLayout>
</template>
