<template>
  <section class="card" data-testid="mobile-release-manager">
    <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t("admin.playOps.release.title") }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t("admin.playOps.release.hint") }}</p>
      </div>
      <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="loading || uploading" @click="load">
        <Icon name="refresh" size="sm" />
        {{ t("admin.playOps.refresh") }}
      </button>
    </div>

    <div class="grid gap-5 p-5 xl:grid-cols-[minmax(0,360px)_minmax(0,1fr)]">
      <form class="space-y-4" @submit.prevent="upload">
        <label class="block">
          <span class="input-label">{{ t("admin.playOps.release.artifact") }}</span>
          <input ref="artifactInput" type="file" accept=".apk,.aab,application/vnd.android.package-archive" class="input" :disabled="uploading" @change="onArtifactChange" />
          <span class="mt-1 block text-xs text-gray-500">{{ artifact?.name || t("admin.playOps.release.noFile") }}</span>
        </label>
        <label class="block">
          <span class="input-label">{{ t("admin.playOps.release.manifest") }}</span>
          <input ref="manifestInput" type="file" accept="application/json,.json" class="input" :disabled="uploading" @change="onManifestChange" />
          <span class="mt-1 block text-xs text-gray-500">{{ manifest?.name || t("admin.playOps.release.noFile") }}</span>
        </label>
        <div v-if="manifestPreview" class="space-y-2 rounded border border-gray-200 p-3 text-sm dark:border-dark-700">
          <div class="grid gap-2 sm:grid-cols-2"><div><span class="text-xs text-gray-500">{{ t("admin.playOps.release.version") }}</span><strong class="ml-2">{{ manifestPreview.version }} · {{ manifestPreview.versionCode }}</strong></div><div><span class="text-xs text-gray-500">{{ t("admin.playOps.release.channel") }}</span><strong class="ml-2">{{ manifestPreview.distribution || manifestPreview.channel || "—" }}</strong></div></div>
          <p class="text-xs text-gray-500">{{ t("admin.playOps.release.previewHint") }}</p>
          <div class="grid gap-2 sm:grid-cols-2"><div v-for="localeKey in localeKeys" :key="localeKey"><p class="text-xs font-medium text-gray-500">{{ t(`admin.playOps.release.locales.${localeKey}`) }}</p><p class="mt-1 text-gray-700 dark:text-gray-200">{{ (manifestPreview.notes_i18n?.[localeKey] || []).join(" · ") || "—" }}</p></div></div>
        </div>
        <p class="rounded border border-primary-100 bg-primary-50 px-3 py-2 text-xs text-primary-900 dark:border-primary-900/60 dark:bg-primary-950/30 dark:text-primary-100">{{ t("admin.playOps.release.readOnlyNotes") }}</p>
        <p v-if="formError" role="alert" class="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">{{ formError }}</p>
        <button type="submit" class="btn btn-primary inline-flex w-full items-center justify-center gap-2" :disabled="uploading || !artifact || !manifest">
          <Icon :name="uploading ? 'refresh' : 'upload'" size="sm" />
          {{ uploading ? t("admin.playOps.release.uploading") : t("admin.playOps.release.upload") }}
        </button>
      </form>

      <div class="min-w-0">
        <div v-if="loading && !releases.length" class="flex min-h-36 items-center justify-center text-sm text-gray-500">{{ t("admin.playOps.loading") }}</div>
        <div v-else-if="!releases.length" class="rounded border border-dashed border-gray-300 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-600">{{ t("admin.playOps.release.empty") }}</div>
        <div v-else class="space-y-3">
          <article v-for="release in releases" :key="release.id" class="rounded border border-gray-200 p-4 dark:border-dark-700">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="font-semibold text-gray-900 dark:text-white">{{ release.version }} · {{ release.version_code }}</h3>
                  <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ t(`admin.playOps.release.channels.${release.distribution}`) }}</span>
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(release.status)">{{ t(`admin.playOps.release.statuses.${release.status}`) }}</span>
                </div>
                <p class="mt-1 text-xs text-gray-500">{{ release.artifact_type.toUpperCase() }} · {{ formatBytes(release.bytes) }} · {{ release.sha256.slice(0, 12) }}…</p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button v-if="release.status === 'ready' || release.status === 'paused'" type="button" class="btn btn-primary" :disabled="busyId === release.id" @click="changeStatus(release, 'publish')">{{ t("admin.playOps.release.publish") }}</button>
                <button v-if="release.status === 'published'" type="button" class="btn btn-secondary" :disabled="busyId === release.id" @click="changeStatus(release, 'pause')">{{ t("admin.playOps.release.pause") }}</button>
                <button v-if="release.status !== 'retired'" type="button" class="btn btn-secondary" :disabled="busyId === release.id" @click="changeStatus(release, 'retire')">{{ t("admin.playOps.release.retire") }}</button>
              </div>
            </div>
            <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <div v-for="localeKey in localeKeys" :key="localeKey" class="rounded border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t(`admin.playOps.release.locales.${localeKey}`) }}</p>
                <ul class="mt-1 space-y-1 text-sm text-gray-700 dark:text-gray-200"><li v-for="note in release.notes_i18n[localeKey] || []" :key="note">{{ note }}</li></ul>
              </div>
            </div>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import Icon from "@/components/icons/Icon.vue";
import adminPlayAPI, { type AdminMobileRelease, type AdminMobileReleaseManifest, type AdminMobileReleaseStatus } from "@/api/admin/play";
import { useAppStore } from "@/stores";
import { extractApiErrorMessage } from "@/utils/apiError";

const { t } = useI18n();
const appStore = useAppStore();
const artifact = ref<File | null>(null);
const manifest = ref<File | null>(null);
const releases = ref<AdminMobileRelease[]>([]);
const loading = ref(false);
const uploading = ref(false);
const busyId = ref<number | null>(null);
const formError = ref("");
const manifestPreview = ref<AdminMobileReleaseManifest & { distribution?: string; channel?: string } | null>(null);
const localeKeys = ["zh", "en", "ja", "ko"] as const;

function onArtifactChange(event: Event) { artifact.value = (event.target as HTMLInputElement).files?.[0] || null; formError.value = ""; }
async function onManifestChange(event: Event) {
  manifest.value = (event.target as HTMLInputElement).files?.[0] || null;
  manifestPreview.value = null;
  formError.value = "";
  if (!manifest.value) return;
  try {
    manifestPreview.value = JSON.parse(await manifest.value.text()) as AdminMobileReleaseManifest & { distribution?: string; channel?: string };
  } catch {
    formError.value = t("admin.playOps.release.manifestInvalid");
  }
}
function statusClass(status: AdminMobileReleaseStatus) {
  return status === "published" ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300" : status === "paused" ? "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300" : "bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300";
}
function formatBytes(value: number) { if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`; return `${(value / 1024 / 1024).toFixed(1)} MB`; }
async function load() { loading.value = true; try { releases.value = (await adminPlayAPI.listMobileReleases()).items; } catch (error) { appStore.showError(extractApiErrorMessage(error, t("admin.playOps.release.loadFailed"))); } finally { loading.value = false; } }
async function upload() {
  if (!artifact.value || !manifest.value || uploading.value) return;
  if (!/\.(apk|aab)$/i.test(artifact.value.name)) { formError.value = t("admin.playOps.release.artifactInvalid"); return; }
  uploading.value = true; formError.value = "";
  try { const created = await adminPlayAPI.uploadMobileRelease(artifact.value, manifest.value); releases.value = [created, ...releases.value]; artifact.value = null; manifest.value = null; manifestPreview.value = null; appStore.showSuccess(t("admin.playOps.release.uploaded")); } catch (error) { formError.value = extractApiErrorMessage(error, t("admin.playOps.release.uploadFailed")); } finally { uploading.value = false; }
}
async function changeStatus(release: AdminMobileRelease, action: "publish" | "pause" | "retire") {
  busyId.value = release.id;
  try {
    const updated = action === "publish" ? await adminPlayAPI.publishMobileRelease(release.id) : action === "pause" ? await adminPlayAPI.pauseMobileRelease(release.id) : await adminPlayAPI.retireMobileRelease(release.id);
    releases.value = releases.value.map((item) => {
      if (item.id === updated.id) return updated;
      if (action === "publish" && item.distribution === updated.distribution && item.status === "published") {
        return { ...item, status: "paused" };
      }
      return item;
    });
    if (action === "publish") await load();
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t("admin.playOps.release.saveFailed")));
  } finally {
    busyId.value = null;
  }
}
onMounted(load);
</script>
