<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ImageStudioGallery from '@/components/imageStudio/ImageStudioGallery.vue'
import ImageStudioPreviewModal from '@/components/imageStudio/ImageStudioPreviewModal.vue'
import ImageStudioSizePicker from '@/components/imageStudio/ImageStudioSizePicker.vue'
import { useImageStudioWorkspace } from '@/composables/useImageStudioWorkspace'
import { isFeatureFlagEnabled, FeatureFlags } from '@/utils/featureFlags'
import type { ImageStudioJob, ImageStudioTemplate } from '@/api/imageStudio'
import {
  IMAGE_STUDIO_PROMPT_LIMIT,
  countImageStudioCodePoints,
  flattenImageStudioTemplates,
  resizeImageStudioTextarea,
  validateImageStudioPrompt,
} from '@/utils/imageStudioWorkspace'

const { t } = useI18n()
const enabled = isFeatureFlagEnabled(FeatureFlags.imageStudio)
const workspace = useImageStudioWorkspace()

const mobileView = ref<'create' | 'works'>('create')
const promptTouched = ref(false)
const expertPromptTouched = ref(false)
const promptTextarea = ref<HTMLTextAreaElement | null>(null)
const expertPromptTextarea = ref<HTMLTextAreaElement | null>(null)

const templateOptions = computed(() => flattenImageStudioTemplates(workspace.catalog.value))

const featuredJob = computed<ImageStudioJob | null>(() =>
  workspace.latestJob.value ?? workspace.jobs.value[0] ?? null,
)

const historyJobs = computed(() => {
  const featuredId = featuredJob.value?.id
  return workspace.jobs.value.filter((job) => job.id !== featuredId)
})

const selectedTemplateDescription = computed(() =>
  workspace.labelFor(workspace.selectedTemplate.value?.description),
)

const selectedTemplatePreview = computed(() => workspace.selectedTemplate.value?.preview_url || '')

const promptLength = computed(() => countImageStudioCodePoints(workspace.userPrompt.value))
const expertPromptLength = computed(() => countImageStudioCodePoints(workspace.expertPrompt.value))
const promptError = computed(() => validateImageStudioPrompt(workspace.userPrompt.value))
const expertPromptError = computed(() =>
  workspace.expertOpen.value
    ? validateImageStudioPrompt(workspace.expertPrompt.value, { required: false })
    : null,
)
const canGenerate = computed(() =>
  workspace.promptValid.value
    && workspace.expertPromptValid.value
    && !!workspace.selectedTemplate.value
    && !!workspace.apiKeyId.value
    && !!workspace.selectedModel.value
    && !!workspace.estimate.value
    && !workspace.generating.value
    && !workspace.polling.value,
)

const generateLabel = computed(() => {
  if (workspace.generating.value || workspace.polling.value) return t('imageStudio.generating')
  return t('imageStudio.generateCount', { count: workspace.count.value })
})

const selectedModelLabel = computed(() =>
  workspace.selectedModelOption.value?.display_name || workspace.selectedModel.value || t('imageStudio.noModelSelected'),
)

function selectTemplate(template: ImageStudioTemplate) {
  workspace.pickTemplate(template)
}

function changeCount(delta: number) {
  const next = Math.min(workspace.maxCount.value, Math.max(1, workspace.count.value + delta))
  workspace.count.value = next
}

async function generate() {
  promptTouched.value = true
  if (workspace.expertOpen.value) expertPromptTouched.value = true
  if (!workspace.promptValid.value || !workspace.expertPromptValid.value) {
    mobileView.value = 'create'
    return
  }
  const succeeded = await workspace.generate()
  if (!succeeded) mobileView.value = 'create'
}

function reuseJob(job: ImageStudioJob) {
  workspace.regenerateFromJob(job)
  mobileView.value = 'create'
}

function resizePromptTextarea() {
  if (promptTextarea.value) resizeImageStudioTextarea(promptTextarea.value)
}

function resizeExpertPromptTextarea() {
  if (workspace.expertOpen.value && expertPromptTextarea.value) {
    resizeImageStudioTextarea(expertPromptTextarea.value)
  }
}

function resizePromptTextareas() {
  resizePromptTextarea()
  resizeExpertPromptTextarea()
}

function toggleExpertSettings(event: Event) {
  const open = (event.target as HTMLDetailsElement).open
  workspace.expertOpen.value = open
  if (open) void nextTick(resizeExpertPromptTextarea)
}

watch(
  () => workspace.polling.value,
  (value) => {
    if (value) mobileView.value = 'works'
  },
  { flush: 'sync' },
)
watch(
  () => workspace.errorMsg.value,
  (value) => {
    if (value) mobileView.value = 'create'
  },
  { flush: 'sync' },
)
watch(
  () => workspace.userPrompt.value,
  () => { void nextTick(resizePromptTextarea) },
  { immediate: true },
)
watch(
  () => workspace.bootstrapping.value,
  (value) => {
    if (!value) void nextTick(resizePromptTextareas)
  },
  { immediate: true },
)
watch(
  () => workspace.expertPrompt.value,
  () => { void nextTick(resizeExpertPromptTextarea) },
  { immediate: true },
)
onMounted(() => {
  window.addEventListener('resize', resizePromptTextareas)
  window.visualViewport?.addEventListener('resize', resizePromptTextareas)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', resizePromptTextareas)
  window.visualViewport?.removeEventListener('resize', resizePromptTextareas)
})
</script>

<template>
  <AppLayout>
    <div v-if="!enabled" class="mx-auto max-w-3xl py-16 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('imageStudio.disabled') }}
    </div>

    <div v-else class="mx-auto max-w-[1440px]">
      <div class="mb-4 lg:hidden">
        <h1 class="text-xl font-bold text-gray-900 dark:text-white">{{ t('imageStudio.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('imageStudio.workspaceSubtitle') }}</p>
      </div>

      <div class="mb-4 grid grid-cols-2 rounded-xl bg-gray-200/70 p-1 lg:hidden dark:bg-dark-800">
        <button
          type="button"
          class="rounded-lg px-3 py-2.5 text-sm font-semibold transition"
          :class="mobileView === 'create' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300' : 'text-gray-500 dark:text-gray-400'"
          @click="mobileView = 'create'"
        >
          {{ t('imageStudio.createTab') }}
        </button>
        <button
          type="button"
          class="rounded-lg px-3 py-2.5 text-sm font-semibold transition"
          :class="mobileView === 'works' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-300' : 'text-gray-500 dark:text-gray-400'"
          @click="mobileView = 'works'"
        >
          {{ t('imageStudio.worksTab') }}
        </button>
      </div>

      <div v-if="workspace.bootstrapping.value" class="card flex min-h-72 items-center justify-center p-8">
        <div class="flex items-center gap-3 text-sm text-gray-500 dark:text-gray-400">
          <span class="h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
          {{ t('models.loading') }}
        </div>
      </div>

      <div v-else class="grid items-start gap-5 xl:grid-cols-[minmax(360px,420px)_minmax(0,1fr)]">
        <section
          class="card overflow-hidden xl:sticky xl:top-24"
          :class="mobileView === 'create' ? 'block' : 'hidden lg:block'"
        >
          <header class="flex items-start justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('imageStudio.createTitle') }}</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('imageStudio.createHint') }}</p>
            </div>
            <span class="flex-shrink-0 text-xs text-gray-400 dark:text-gray-500">{{ t('imageStudio.settingsRetained') }}</span>
          </header>

          <p
            v-if="workspace.errorMsg.value"
            data-testid="mobile-generation-error"
            role="alert"
            class="mx-5 mt-4 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700 lg:hidden dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300"
          >
            {{ workspace.errorMsg.value }}
          </p>

          <div v-if="!workspace.hasApiKeys.value" class="p-5">
            <div class="rounded-xl border border-dashed border-gray-200 bg-gray-50 p-5 text-center dark:border-dark-600 dark:bg-dark-900">
              <div class="mx-auto grid h-11 w-11 place-items-center rounded-xl bg-white text-gray-500 shadow-sm dark:bg-dark-800 dark:text-gray-300">
                <Icon name="key" />
              </div>
              <h3 class="mt-3 font-semibold text-gray-900 dark:text-white">{{ t('imageStudio.noApiKeysTitle') }}</h3>
              <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('imageStudio.noApiKeysHint') }}</p>
              <router-link to="/keys" class="btn btn-primary mt-4">{{ t('imageStudio.goKeys') }}</router-link>
            </div>
          </div>

          <template v-else>
            <div class="border-b border-gray-100 p-5 dark:border-dark-700">
              <div class="mb-3 flex items-center justify-between gap-3">
                <h3 class="input-label mb-0">{{ t('imageStudio.template') }}</h3>
                <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('imageStudio.templateHint') }}</span>
              </div>
              <div class="grid grid-cols-3 gap-2">
                <button
                  v-for="option in templateOptions"
                  :key="option.template.id"
                  type="button"
                  class="group relative min-w-0 rounded-xl border bg-white p-1.5 text-left transition focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:bg-dark-800"
                  :class="workspace.selectedTemplate.value?.id === option.template.id
                    ? 'border-primary-500 ring-2 ring-primary-500/10 dark:border-primary-400'
                    : 'border-gray-200 hover:border-gray-300 dark:border-dark-600 dark:hover:border-dark-500'"
                  @click="selectTemplate(option.template)"
                >
                  <div class="relative aspect-[4/3] overflow-hidden rounded-lg bg-gray-100 dark:bg-dark-900">
                    <img v-if="option.template.preview_url" :src="option.template.preview_url" :alt="workspace.labelFor(option.template.label)" class="h-full w-full object-cover" />
                    <span v-else class="grid h-full place-items-center text-2xl">{{ option.template.preview_emoji }}</span>
                    <span v-if="workspace.selectedTemplate.value?.id === option.template.id" class="absolute right-1.5 top-1.5 grid h-5 w-5 place-items-center rounded-full bg-primary-500 text-white shadow ring-2 ring-white dark:ring-dark-800">
                      <Icon name="check" size="xs" :stroke-width="2.5" />
                    </span>
                  </div>
                  <p class="mt-2 h-8 overflow-hidden px-0.5 text-xs font-semibold leading-4 text-gray-800 dark:text-gray-100">{{ workspace.labelFor(option.template.label) }}</p>
                  <p class="mt-0.5 hidden truncate px-0.5 text-[10px] text-gray-400 sm:block dark:text-gray-500">{{ workspace.labelFor(option.template.description) }}</p>
                </button>
              </div>
            </div>

            <div class="space-y-4 border-b border-gray-100 p-5 dark:border-dark-700">
              <label class="block">
                <span class="mb-2 flex items-center justify-between gap-3">
                  <span class="input-label mb-0">{{ t('imageStudio.promptLabel') }}</span>
                  <span class="text-xs text-gray-400 dark:text-gray-500">{{ promptLength }} / {{ IMAGE_STUDIO_PROMPT_LIMIT }}</span>
                </span>
                <textarea
                  ref="promptTextarea"
                  v-model="workspace.userPrompt.value"
                  class="input studio-prompt-textarea min-h-[88px] resize-none leading-6"
                  :class="{ 'input-error': promptTouched && !workspace.promptValid.value }"
                  rows="3"
                  :placeholder="t('imageStudio.promptPlaceholder')"
                  @input="resizePromptTextarea"
                  @blur="promptTouched = true"
                />
                <span v-if="promptTouched && promptError" class="input-error-text">
                  {{ t(promptError === 'too_long' ? 'imageStudio.promptTooLong' : 'imageStudio.promptRequired') }}
                </span>
              </label>

              <label v-if="workspace.showAccentColor.value" class="block">
                <span class="mb-2 flex items-center justify-between gap-3">
                  <span class="input-label mb-0">{{ t('imageStudio.accentColor') }}</span>
                  <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('imageStudio.optional') }}</span>
                </span>
                <span class="flex items-center gap-2">
                  <input v-model="workspace.accentColor.value" type="color" class="h-11 w-12 cursor-pointer rounded-xl border border-gray-200 bg-white p-1 dark:border-dark-600 dark:bg-dark-800" />
                  <input v-model="workspace.accentColor.value" class="input max-w-32 font-mono uppercase" maxlength="7" />
                </span>
              </label>

              <ImageStudioSizePicker
                :capabilities="workspace.capabilities.value"
                :aspect="workspace.aspect.value"
                :tier="workspace.tier.value"
                :selected-model="workspace.selectedModelOption.value"
                :disabled="workspace.polling.value || workspace.generating.value"
                @update:aspect="workspace.onAspectChange"
                @update:tier="workspace.onTierChange"
              />

              <div>
                <span class="input-label">{{ t('imageStudio.count') }}</span>
                <div class="grid h-11 grid-cols-[44px_1fr_44px] items-center rounded-xl border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800">
                  <button type="button" class="grid h-full place-items-center rounded-l-xl text-gray-500 hover:bg-gray-50 disabled:opacity-30 dark:text-gray-400 dark:hover:bg-dark-700" :disabled="workspace.count.value <= 1" :aria-label="t('imageStudio.decreaseCount')" @click="changeCount(-1)">
                    <span class="text-lg">−</span>
                  </button>
                  <strong class="text-center tabular-nums text-gray-900 dark:text-white">{{ workspace.count.value }}</strong>
                  <button type="button" class="grid h-full place-items-center rounded-r-xl text-gray-500 hover:bg-gray-50 disabled:opacity-30 dark:text-gray-400 dark:hover:bg-dark-700" :disabled="workspace.count.value >= workspace.maxCount.value" :aria-label="t('imageStudio.increaseCount')" @click="changeCount(1)">
                    <Icon name="plus" size="sm" />
                  </button>
                </div>
                <p v-if="workspace.isNewUser.value" class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">{{ t('imageStudio.newUserHint') }}</p>
              </div>
            </div>

            <details :open="workspace.expertOpen.value" class="group border-b border-gray-100 dark:border-dark-700" @toggle="toggleExpertSettings">
              <summary class="flex cursor-pointer list-none items-center justify-between gap-3 px-5 py-3.5 text-sm font-medium text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700/50">
                <span>{{ t('imageStudio.advancedSettings') }}</span>
                <span class="flex min-w-0 items-center gap-2 text-xs font-normal text-gray-400 dark:text-gray-500">
                  <span class="max-w-48 truncate">{{ selectedModelLabel }} · {{ workspace.apiKeys.value.find((key) => key.id === workspace.apiKeyId.value)?.name }}</span>
                  <Icon name="chevronDown" size="xs" class="transition group-open:rotate-180" />
                </span>
              </summary>
              <div class="space-y-4 border-t border-gray-100 bg-gray-50/70 px-5 py-4 dark:border-dark-700 dark:bg-dark-900/50">
                <label class="block">
                  <span class="input-label">{{ t('imageStudio.apiKey') }}</span>
                  <select v-model.number="workspace.apiKeyId.value" class="input" :disabled="workspace.polling.value || workspace.generating.value">
                    <option v-for="key in workspace.apiKeys.value" :key="key.id" :value="key.id">{{ key.name }}</option>
                  </select>
                </label>
                <label class="block">
                  <span class="input-label">{{ t('imageStudio.model') }}</span>
                  <select v-model="workspace.selectedModel.value" class="input" :disabled="workspace.loadingModels.value || !workspace.availableModels.value.length || workspace.polling.value || workspace.generating.value">
                    <option v-if="workspace.loadingModels.value" value="">{{ t('imageStudio.loadingModels') }}</option>
                    <option v-for="model in workspace.availableModels.value" :key="model.id" :value="model.id">{{ model.display_name || model.id }}</option>
                  </select>
                </label>
                <label v-if="workspace.showQuality.value" class="block">
                  <span class="input-label">{{ t('imageStudio.renderQuality') }}</span>
                  <select v-model="workspace.quality.value" class="input" :disabled="workspace.polling.value || workspace.generating.value">
                    <option v-for="quality in workspace.selectedModelOption.value?.supported_qualities || []" :key="quality" :value="quality">{{ t(`imageStudio.qualityOptions.${quality}`, quality) }}</option>
                  </select>
                </label>
                <label class="block">
                  <span class="mb-2 flex items-center justify-between gap-3">
                    <span class="input-label mb-0">{{ t('imageStudio.expertPrompt') }}</span>
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{ expertPromptLength }} / {{ IMAGE_STUDIO_PROMPT_LIMIT }}</span>
                  </span>
                  <textarea
                    ref="expertPromptTextarea"
                    v-model="workspace.expertPrompt.value"
                    class="input studio-prompt-textarea min-h-20 resize-none font-mono text-xs leading-5"
                    :class="{ 'input-error': expertPromptTouched && !workspace.expertPromptValid.value }"
                    rows="3"
                    @input="resizeExpertPromptTextarea"
                    @blur="expertPromptTouched = true"
                  />
                  <span v-if="expertPromptTouched && expertPromptError" class="input-error-text">
                    {{ t('imageStudio.expertPromptTooLong') }}
                  </span>
                </label>
                <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
                  <input v-model="workspace.autoCleanup.value" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" :disabled="workspace.polling.value || workspace.generating.value" @change="workspace.onAutoCleanupChange()" />
                  {{ t('imageStudio.autoCleanup') }}
                </label>
              </div>
            </details>

            <div class="bg-gray-50/80 p-5 dark:bg-dark-900/60">
              <p v-if="workspace.modelError.value || workspace.estimateError.value" class="mb-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">
                {{ workspace.modelError.value || workspace.estimateError.value }}
              </p>
              <p v-if="workspace.errorMsg.value" role="alert" class="mb-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">
                {{ workspace.errorMsg.value }}
              </p>
              <div class="mb-3 flex items-center justify-between gap-3 text-xs">
                <span class="text-gray-500 dark:text-gray-400">{{ t('imageStudio.estimateLabel') }}</span>
                <span v-if="workspace.estimate.value" class="font-semibold tabular-nums text-gray-900 dark:text-white">
                  ${{ workspace.estimate.value.estimated_cost.toFixed(4) }}
                  <span class="ml-1 font-normal" :class="workspace.estimate.value.sufficient ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'">
                    {{ workspace.estimate.value.sufficient ? t('imageStudio.balanceSufficient') : t('imageStudio.balanceInsufficient') }}
                  </span>
                </span>
                <span v-else class="text-gray-400">{{ t('imageStudio.estimatePending') }}</span>
              </div>
              <button type="button" class="btn btn-primary w-full" :disabled="!canGenerate" @click="generate">
                <Icon name="sparkles" size="sm" />
                {{ workspace.estimate.value && !workspace.estimate.value.sufficient ? t('imageStudio.rechargeToGenerate') : generateLabel }}
              </button>
            </div>
          </template>
        </section>

        <section
          class="card min-w-0 overflow-hidden"
          :class="mobileView === 'works' ? 'block' : 'hidden lg:block'"
        >
          <header class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('imageStudio.latestResult') }}</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('imageStudio.latestResultHint') }}</p>
            </div>
            <span v-if="workspace.polling.value" class="inline-flex items-center gap-2 text-xs font-medium text-amber-600 dark:text-amber-400">
              <span class="h-2 w-2 animate-pulse rounded-full bg-current" />
              {{ workspace.pollNotice.value || t('imageStudio.polling') }}
            </span>
          </header>

          <div class="p-4 sm:p-5">
            <div v-if="workspace.polling.value" class="flex min-h-[420px] flex-col items-center justify-center rounded-xl bg-gray-50 px-6 text-center dark:bg-dark-900">
              <span class="h-10 w-10 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
              <h3 class="mt-4 font-semibold text-gray-900 dark:text-white">{{ t('imageStudio.generatingTitle') }}</h3>
              <p class="mt-2 max-w-md text-sm leading-6 text-gray-500 dark:text-gray-400">{{ workspace.pollNotice.value || t('imageStudio.polling') }}</p>
            </div>

            <ImageStudioGallery
              v-else-if="featuredJob"
              :jobs="[featuredJob]"
              :latest-job="featuredJob"
              featured
              @preview="workspace.openPreview"
              @delete="workspace.removeJob"
              @regenerate="reuseJob"
            />

            <div v-else-if="workspace.galleryError.value" class="flex min-h-[320px] flex-col items-center justify-center rounded-xl border border-red-200 bg-red-50 px-6 text-center dark:border-red-900/60 dark:bg-red-950/30">
              <Icon name="exclamationCircle" class="text-red-500 dark:text-red-300" />
              <p class="mt-3 max-w-md text-sm leading-6 text-red-700 dark:text-red-300">{{ workspace.galleryError.value }}</p>
              <button data-testid="retry-gallery" type="button" class="btn btn-secondary mt-4" @click="workspace.refreshJobs">
                <Icon name="refresh" size="sm" />
                {{ t('imageStudio.retryGallery') }}
              </button>
            </div>

            <div v-else-if="selectedTemplatePreview" class="relative overflow-hidden rounded-xl bg-gray-100 dark:bg-dark-900">
              <img :src="selectedTemplatePreview" :alt="workspace.labelFor(workspace.selectedTemplate.value?.label)" class="max-h-[62vh] min-h-72 w-full object-cover" />
              <span class="absolute left-3 top-3 rounded-lg bg-gray-950/70 px-2.5 py-1.5 text-xs font-medium text-white backdrop-blur">{{ t('imageStudio.templatePreview') }}</span>
              <div class="border-t border-gray-100 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
                <p class="font-medium text-gray-900 dark:text-white">{{ workspace.labelFor(workspace.selectedTemplate.value?.label) }}</p>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ selectedTemplateDescription }}</p>
              </div>
            </div>

            <div v-else class="flex min-h-[420px] items-center justify-center rounded-xl border border-dashed border-gray-200 bg-gray-50 px-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-400">
              {{ t('imageStudio.galleryEmpty') }}
            </div>
          </div>

          <div v-if="historyJobs.length" class="border-t border-gray-100 px-4 py-5 sm:px-5 dark:border-dark-700">
            <div class="mb-4 flex items-center justify-between gap-3">
              <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('imageStudio.recentWorks') }}</h2>
              <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('imageStudio.recentWorksCount', { count: historyJobs.length }) }}</span>
            </div>
            <ImageStudioGallery
              :jobs="historyJobs"
              @preview="workspace.openPreview"
              @delete="workspace.removeJob"
              @regenerate="reuseJob"
            />
          </div>
        </section>
      </div>
    </div>

    <ImageStudioPreviewModal
      :asset="workspace.previewAsset.value"
      :job-id="workspace.previewJobId.value"
      :index="workspace.previewIndex.value"
      @close="workspace.closePreview()"
    />

    <div v-if="workspace.showFirstWin.value" class="fixed inset-0 z-[190] flex items-center justify-center bg-gray-950/60 p-5 backdrop-blur-sm" @click.self="workspace.showFirstWin.value = false">
      <div class="w-full max-w-sm rounded-2xl bg-white p-6 text-center shadow-2xl dark:bg-dark-800">
        <div class="mx-auto grid h-12 w-12 place-items-center rounded-xl bg-emerald-50 text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-300">
          <Icon name="checkCircle" size="lg" />
        </div>
        <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">{{ t('imageStudio.firstWinTitle') }}</h2>
        <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('imageStudio.firstWinHint') }}</p>
        <button type="button" class="btn btn-primary mt-5 w-full" @click="workspace.showFirstWin.value = false">{{ t('imageStudio.firstWinCta') }}</button>
      </div>
    </div>
  </AppLayout>
</template>

<style scoped>
.studio-prompt-textarea {
  max-height: 320px;
  overflow-y: hidden;
}

@media (max-width: 1023px) {
  .studio-prompt-textarea {
    max-height: 42dvh;
  }
}
</style>
