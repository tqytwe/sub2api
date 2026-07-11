<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { keysAPI } from '@/api/keys'
import imageStudioAPI, {
  type ImageStudioCatalog,
  type ImageStudioEstimate,
  type ImageStudioIntent,
  type ImageStudioJob,
  type ImageStudioTemplate,
} from '@/api/imageStudio'
import { useAuthStore } from '@/stores/auth'
import { isFeatureFlagEnabled, FeatureFlags } from '@/utils/featureFlags'
import '@/styles/image-studio.css'

const { t, locale } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const loading = ref(true)
const generating = ref(false)
const step = ref(1)
const catalog = ref<ImageStudioCatalog | null>(null)
const selectedIntent = ref<ImageStudioIntent | null>(null)
const selectedTemplate = ref<ImageStudioTemplate | null>(null)
const userPrompt = ref('')
const accentColor = ref('#1a1a1a')
const size = ref('1024x1024')
const count = ref(1)
const expertOpen = ref(false)
const expertPrompt = ref('')
const apiKeyId = ref<number | null>(null)
const apiKeys = ref<Array<{ id: number; name: string }>>([])
const estimate = ref<ImageStudioEstimate | null>(null)
const jobs = ref<ImageStudioJob[]>([])
const errorMsg = ref('')

const enabled = computed(() => isFeatureFlagEnabled(FeatureFlags.imageStudio))
const balance = computed(() => authStore.user?.balance ?? estimate.value?.balance ?? 0)

function labelFor(obj?: { zh: string; en: string }) {
  if (!obj) return ''
  return locale.value.startsWith('zh') ? obj.zh : obj.en
}

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const [tpl, keyPage, jobList] = await Promise.all([
      imageStudioAPI.getImageStudioTemplates(),
      keysAPI.list(1, 20),
      imageStudioAPI.listImageStudioJobs(12).catch(() => []),
    ])
    catalog.value = tpl
    apiKeys.value = (keyPage.items ?? []).map((k) => ({ id: k.id, name: k.name || `Key #${k.id}` }))
    if (apiKeys.value.length && !apiKeyId.value) apiKeyId.value = apiKeys.value[0].id
    jobs.value = jobList
  } catch {
    errorMsg.value = t('imageStudio.loadFailed')
  } finally {
    loading.value = false
  }
}

async function refreshEstimate() {
  if (!selectedTemplate.value || !apiKeyId.value) {
    estimate.value = null
    return
  }
  try {
    estimate.value = await imageStudioAPI.estimateImageStudio({
      template_id: selectedTemplate.value.id,
      size: size.value,
      count: count.value,
      api_key_id: apiKeyId.value,
    })
  } catch {
    estimate.value = null
  }
}

watch([selectedTemplate, size, count, apiKeyId], refreshEstimate)

function pickIntent(intent: ImageStudioIntent) {
  selectedIntent.value = intent
  selectedTemplate.value = intent.templates[0] ?? null
  if (selectedTemplate.value) {
    size.value = selectedTemplate.value.defaults.size
    count.value = selectedTemplate.value.defaults.count
  }
  step.value = 2
}

function pickTemplate(tpl: ImageStudioTemplate) {
  selectedTemplate.value = tpl
  size.value = tpl.defaults.size
  count.value = tpl.defaults.count
  step.value = 3
}

async function generate() {
  if (!selectedTemplate.value || !apiKeyId.value) return
  if (estimate.value && !estimate.value.sufficient) {
    router.push('/purchase?return=/image-studio')
    return
  }
  generating.value = true
  errorMsg.value = ''
  try {
    const result = await imageStudioAPI.generateImageStudio({
      template_id: selectedTemplate.value.id,
      user_prompt: userPrompt.value,
      accent_color: accentColor.value,
      size: size.value,
      count: count.value,
      expert_prompt: expertOpen.value ? expertPrompt.value : null,
      api_key_id: apiKeyId.value,
    })
    jobs.value = [result.job, ...jobs.value.filter((j) => j.id !== result.job.id)]
    step.value = 4
    await authStore.refreshUser()
  } catch {
    errorMsg.value = t('imageStudio.generateFailed')
  } finally {
    generating.value = false
  }
}

async function removeJob(id: string) {
  await imageStudioAPI.deleteImageStudioJob(id)
  jobs.value = jobs.value.filter((j) => j.id !== id)
}

onMounted(load)
</script>

<template>
  <AppLayout>
    <div v-if="!enabled" class="image-studio-page py-12 text-center text-gray-500">
      {{ t('imageStudio.disabled') }}
    </div>
    <div v-else class="image-studio-page space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p class="text-sm font-medium text-primary-600">{{ t('imageStudio.eyebrow') }}</p>
          <h1 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ t('imageStudio.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('imageStudio.subtitle') }}</p>
        </div>
        <div class="text-right text-sm">
          <p class="text-gray-500">{{ t('imageStudio.balance') }}</p>
          <p class="text-lg font-semibold">${{ balance.toFixed(2) }}</p>
          <router-link to="/purchase?return=/image-studio" class="text-primary-600 hover:underline">
            {{ t('imageStudio.recharge') }}
          </router-link>
        </div>
      </div>

      <div class="image-studio-steps">
        <span v-for="n in 4" :key="n" class="image-studio-step-pill" :class="{ active: step >= n }">
          {{ t('imageStudio.step', { n }) }}
        </span>
      </div>

      <div v-if="loading" class="text-sm text-gray-500">{{ t('models.loading') }}</div>
      <p v-else-if="errorMsg" class="text-sm text-red-600">{{ errorMsg }}</p>

      <section v-else-if="step === 1" class="space-y-4">
        <h2 class="text-lg font-semibold">{{ t('imageStudio.pickIntent') }}</h2>
        <div class="image-studio-grid">
          <button
            v-for="intent in catalog?.intents || []"
            :key="intent.id"
            type="button"
            class="image-studio-card"
            @click="pickIntent(intent)"
          >
            <div class="text-2xl">{{ intent.templates[0]?.preview_emoji || '✨' }}</div>
            <div class="mt-2 font-medium">{{ labelFor(intent.label) }}</div>
          </button>
        </div>
      </section>

      <section v-else-if="step === 2 && selectedIntent" class="space-y-4">
        <h2 class="text-lg font-semibold">{{ t('imageStudio.pickTemplate') }}</h2>
        <div class="image-studio-grid">
          <button
            v-for="tpl in selectedIntent.templates"
            :key="tpl.id"
            type="button"
            class="image-studio-card"
            :class="{ selected: selectedTemplate?.id === tpl.id }"
            @click="pickTemplate(tpl)"
          >
            <div class="text-2xl">{{ tpl.preview_emoji || '🖼️' }}</div>
            <div class="mt-2 font-medium">{{ labelFor(tpl.label) }}</div>
            <ul v-if="tpl.compliance_hints?.length" class="mt-2 list-disc pl-4 text-xs text-gray-500">
              <li v-for="(hint, i) in tpl.compliance_hints" :key="i">{{ hint }}</li>
            </ul>
          </button>
        </div>
      </section>

      <section v-else-if="step === 3 && selectedTemplate" class="space-y-4">
        <h2 class="text-lg font-semibold">{{ t('imageStudio.fillForm') }}</h2>
        <label class="block text-sm">
          <span class="mb-1 block text-gray-600">{{ t('imageStudio.promptLabel') }}</span>
          <input v-model="userPrompt" class="input w-full" :placeholder="t('imageStudio.promptPlaceholder')" />
        </label>
        <div class="grid gap-4 sm:grid-cols-3">
          <label class="block text-sm">
            <span class="mb-1 block text-gray-600">{{ t('imageStudio.size') }}</span>
            <select v-model="size" class="input w-full">
              <option value="1024x1024">1:1</option>
              <option value="1024x1536">3:4</option>
              <option value="1536x1024">4:3</option>
            </select>
          </label>
          <label class="block text-sm">
            <span class="mb-1 block text-gray-600">{{ t('imageStudio.count') }}</span>
            <select v-model.number="count" class="input w-full">
              <option :value="1">1</option>
              <option :value="2">2</option>
              <option :value="4">4</option>
            </select>
          </label>
          <label class="block text-sm">
            <span class="mb-1 block text-gray-600">{{ t('imageStudio.apiKey') }}</span>
            <select v-model.number="apiKeyId" class="input w-full">
              <option v-for="k in apiKeys" :key="k.id" :value="k.id">{{ k.name }}</option>
            </select>
          </label>
        </div>
        <details>
          <summary class="cursor-pointer text-sm text-gray-600" @click="expertOpen = !expertOpen">{{ t('imageStudio.expertPrompt') }}</summary>
          <textarea v-if="expertOpen" v-model="expertPrompt" class="input mt-2 w-full" rows="3" />
        </details>
        <div v-if="estimate" class="flex flex-wrap items-center gap-3">
          <span
            class="image-studio-cost-pill"
            :class="estimate.sufficient ? 'ok' : 'warn'"
          >
            {{ t('imageStudio.estimate', { cost: estimate.estimated_cost.toFixed(4) }) }}
          </span>
          <button type="button" class="btn btn-primary" :disabled="generating || !apiKeyId" @click="generate">
            {{ generating ? t('imageStudio.generating') : t('imageStudio.generate') }}
          </button>
        </div>
      </section>

      <section v-else-if="step === 4" class="space-y-4">
        <h2 class="text-lg font-semibold">{{ t('imageStudio.doneTitle') }}</h2>
        <p class="text-sm text-gray-600">{{ t('imageStudio.doneHint') }}</p>
        <div class="flex gap-3">
          <button type="button" class="btn btn-primary" @click="step = 1">{{ t('imageStudio.makeAnother') }}</button>
          <router-link to="/play" class="btn btn-secondary">{{ t('imageStudio.goHub') }}</router-link>
          <router-link to="/arena" class="btn btn-secondary">{{ t('imageStudio.goFarm') }}</router-link>
        </div>
      </section>

      <section class="space-y-3">
        <h2 class="text-lg font-semibold">{{ t('imageStudio.gallery') }}</h2>
        <div v-if="!jobs.length" class="text-sm text-gray-500">{{ t('imageStudio.galleryEmpty') }}</div>
        <div v-else class="image-studio-gallery">
          <div v-for="job in jobs" :key="job.id" class="space-y-2">
            <div v-for="asset in job.assets || []" :key="asset.id" class="image-studio-thumb">
              <a :href="asset.url" target="_blank" rel="noopener">
                <img :src="asset.url" :alt="job.template_id" loading="lazy" />
              </a>
            </div>
            <button type="button" class="text-xs text-gray-500 hover:text-red-600" @click="removeJob(job.id)">
              {{ t('imageStudio.delete') }}
            </button>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>
