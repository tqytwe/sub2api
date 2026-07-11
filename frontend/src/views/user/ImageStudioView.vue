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
import playAPI from '@/api/play'
import { isFeatureFlagEnabled, FeatureFlags } from '@/utils/featureFlags'
import {
  getStudioAutoCleanup,
  getStudioLastTemplate,
  hasStudioFirstWin,
  markStudioFirstWin,
  setStudioAutoCleanup,
  setStudioLastTemplate,
  trackGrowthEvent,
  trackQuestCompleteOnce,
} from '@/utils/growthAnalytics'
import '@/styles/growth-world.css'

const { t, locale } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const loading = ref(true)
const generating = ref(false)
const polling = ref(false)
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
const autoCleanup = ref(getStudioAutoCleanup())
const showFirstWin = ref(false)
const activeJobId = ref<string | null>(null)
const latestJob = ref<ImageStudioJob | null>(null)
const totalRecharged = ref(0)

const enabled = computed(() => isFeatureFlagEnabled(FeatureFlags.imageStudio))
const balance = computed(() => authStore.user?.balance ?? estimate.value?.balance ?? 0)
const isNewUser = computed(() => totalRecharged.value <= 0)
const maxCount = computed(() => (isNewUser.value ? 1 : 4))
const showAccentColor = computed(() => selectedIntent.value?.id === 'ecommerce')

function labelFor(obj?: { zh: string; en: string }) {
  if (!obj) return ''
  return locale.value.startsWith('zh') ? obj.zh : obj.en
}

function applyQuickStart() {
  const lastId = getStudioLastTemplate()
  if (!lastId || !catalog.value) return false
  for (const intent of catalog.value.intents) {
    const tpl = intent.templates.find((x) => x.id === lastId)
    if (tpl) {
      selectedIntent.value = intent
      selectedTemplate.value = tpl
      size.value = tpl.defaults.size
      count.value = isNewUser.value ? 1 : Math.min(tpl.defaults.count, maxCount.value)
      step.value = 3
      return true
    }
  }
  return false
}

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const [tpl, keyPage, jobList, hub] = await Promise.all([
      imageStudioAPI.getImageStudioTemplates(),
      keysAPI.list(1, 20),
      imageStudioAPI.listImageStudioJobs(12).catch(() => []),
      playAPI.getPlayHub().catch(() => null),
    ])
    totalRecharged.value = hub?.growth?.total_recharged ?? 0
    catalog.value = tpl
    apiKeys.value = (keyPage.items ?? []).map((k) => ({ id: k.id, name: k.name || `Key #${k.id}` }))
    if (apiKeys.value.length && !apiKeyId.value) apiKeyId.value = apiKeys.value[0].id
    jobs.value = jobList
    applyQuickStart()
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
watch(maxCount, (max) => {
  if (count.value > max) count.value = max
})

function pickIntent(intent: ImageStudioIntent) {
  trackGrowthEvent('image_studio_intent_select', { intent_id: intent.id })
  selectedIntent.value = intent
  selectedTemplate.value = intent.templates[0] ?? null
  if (selectedTemplate.value) {
    size.value = selectedTemplate.value.defaults.size
    count.value = isNewUser.value ? 1 : Math.min(selectedTemplate.value.defaults.count, maxCount.value)
  }
  step.value = 2
}

function pickTemplate(tpl: ImageStudioTemplate) {
  selectedTemplate.value = tpl
  size.value = tpl.defaults.size
  count.value = isNewUser.value ? 1 : Math.min(tpl.defaults.count, maxCount.value)
  step.value = 3
}

function onAutoCleanupChange() {
  setStudioAutoCleanup(autoCleanup.value)
}

async function generate() {
  if (!selectedTemplate.value || !apiKeyId.value) return
  if (estimate.value && !estimate.value.sufficient) {
    trackGrowthEvent('image_studio_insufficient_balance', { balance: estimate.value.balance })
    router.push('/purchase?return=/image-studio')
    return
  }
  trackGrowthEvent('image_studio_generate_click', {
    template_id: selectedTemplate.value.id,
    estimated_cost: estimate.value?.estimated_cost,
  })
  generating.value = true
  polling.value = false
  errorMsg.value = ''
  activeJobId.value = null
  try {
    const result = await imageStudioAPI.generateImageStudio({
      template_id: selectedTemplate.value.id,
      user_prompt: userPrompt.value,
      accent_color: accentColor.value,
      size: size.value,
      count: count.value,
      expert_prompt: expertOpen.value ? expertPrompt.value : null,
      api_key_id: apiKeyId.value,
      retain_days: autoCleanup.value ? 7 : 0,
    })
    setStudioLastTemplate(selectedTemplate.value.id)
    activeJobId.value = result.job.id
    polling.value = true
    const job = await imageStudioAPI.pollImageStudioJob(result.job.id)
    polling.value = false
    if (job.status === 'failed') {
      errorMsg.value = job.error_message || t('imageStudio.generateFailed')
      return
    }
    trackGrowthEvent('image_studio_generate_success', {
      template_id: job.template_id,
      actual_cost: job.actual_cost ?? job.estimated_cost,
      count: job.count,
    })
    trackQuestCompleteOnce('image_generate')
    latestJob.value = job
    jobs.value = [job, ...jobs.value.filter((j) => j.id !== job.id)]
    step.value = 4
    await authStore.refreshUser()
    if (!hasStudioFirstWin()) {
      markStudioFirstWin()
      showFirstWin.value = true
    }
  } catch {
    errorMsg.value = t('imageStudio.generateFailed')
  } finally {
    generating.value = false
    polling.value = false
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
    <div v-if="!enabled" class="gw-page py-12 text-center">
      <p class="gw-subtitle">{{ t('imageStudio.disabled') }}</p>
    </div>
    <div v-else class="gw-page space-y-6 pb-10">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p class="gw-eyebrow">{{ t('imageStudio.eyebrow') }}</p>
          <h1 class="gw-title">{{ t('imageStudio.title') }}</h1>
          <p class="gw-subtitle">{{ t('imageStudio.subtitle') }}</p>
        </div>
        <div class="gw-balance-card">
          <p class="gw-balance-label">{{ t('imageStudio.balance') }}</p>
          <p class="gw-balance-value">${{ balance.toFixed(2) }}</p>
          <router-link to="/purchase?return=/image-studio" class="gw-link">{{ t('imageStudio.recharge') }}</router-link>
        </div>
      </div>

      <div class="gw-steps">
        <span v-for="n in 4" :key="n" class="gw-step-pill" :class="{ active: step >= n }">
          {{ t('imageStudio.step', { n }) }}
        </span>
      </div>

      <div v-if="loading" class="gw-polling">{{ t('models.loading') }}</div>
      <template v-else>
        <p v-if="errorMsg" class="gw-error">{{ errorMsg }}</p>
        <p v-if="polling" class="gw-polling">{{ t('imageStudio.polling') }}</p>

        <section v-if="!polling && step === 1" class="gw-panel">
        <h2 class="gw-section-title">{{ t('imageStudio.pickIntent') }}</h2>
        <div class="gw-grid">
          <button
            v-for="intent in catalog?.intents || []"
            :key="intent.id"
            type="button"
            class="gw-card-btn"
            @click="pickIntent(intent)"
          >
            <div class="gw-card-emoji">{{ intent.templates[0]?.preview_emoji || '✨' }}</div>
            <div class="gw-card-label">{{ labelFor(intent.label) }}</div>
          </button>
        </div>
      </section>

        <section v-else-if="!polling && step === 2 && selectedIntent" class="gw-panel">
        <h2 class="gw-section-title">{{ t('imageStudio.pickTemplate') }}</h2>
        <div class="gw-grid">
          <button
            v-for="tpl in selectedIntent.templates"
            :key="tpl.id"
            type="button"
            class="gw-card-btn"
            :class="{ selected: selectedTemplate?.id === tpl.id }"
            @click="pickTemplate(tpl)"
          >
            <div class="gw-card-emoji">{{ tpl.preview_emoji || '🖼️' }}</div>
            <div class="gw-card-label">{{ labelFor(tpl.label) }}</div>
            <ul v-if="tpl.compliance_hints?.length" class="gw-hints">
              <li v-for="(hint, i) in tpl.compliance_hints" :key="i">{{ hint }}</li>
            </ul>
          </button>
        </div>
        <button type="button" class="gw-btn gw-btn-secondary mt-4" @click="step = 1">{{ t('imageStudio.back') }}</button>
      </section>

        <section v-else-if="!polling && step === 3 && selectedTemplate" class="gw-panel space-y-4">
        <h2 class="gw-section-title">{{ t('imageStudio.fillForm') }}</h2>
        <p v-if="isNewUser" class="text-sm" style="color: var(--gw-ink-3)">{{ t('imageStudio.newUserHint') }}</p>
        <label class="gw-field">
          <span class="gw-field-label">{{ t('imageStudio.promptLabel') }}</span>
          <input v-model="userPrompt" class="gw-input" :placeholder="t('imageStudio.promptPlaceholder')" />
        </label>
        <label v-if="showAccentColor" class="gw-field">
          <span class="gw-field-label">{{ t('imageStudio.accentColor') }}</span>
          <div class="flex items-center gap-3">
            <input v-model="accentColor" type="color" class="h-10 w-14 cursor-pointer rounded-lg border border-[var(--gw-line)] bg-transparent p-1" />
            <input v-model="accentColor" class="gw-input max-w-[8rem]" />
          </div>
        </label>
        <div class="gw-field-row">
          <label class="gw-field">
            <span class="gw-field-label">{{ t('imageStudio.size') }}</span>
            <select v-model="size" class="gw-select">
              <option value="1024x1024">1:1</option>
              <option value="1024x1536">3:4</option>
              <option value="1536x1024">4:3</option>
            </select>
          </label>
          <label class="gw-field">
            <span class="gw-field-label">{{ t('imageStudio.count') }}</span>
            <select v-model.number="count" class="gw-select">
              <option v-for="n in maxCount" :key="n" :value="n">{{ n }}</option>
            </select>
          </label>
          <label class="gw-field">
            <span class="gw-field-label">{{ t('imageStudio.apiKey') }}</span>
            <select v-model.number="apiKeyId" class="gw-select">
              <option v-for="k in apiKeys" :key="k.id" :value="k.id">{{ k.name }}</option>
            </select>
          </label>
        </div>
        <details class="gw-field" @toggle="expertOpen = ($event.target as HTMLDetailsElement).open">
          <summary class="cursor-pointer text-sm" style="color: var(--gw-ink-2)">{{ t('imageStudio.expertPrompt') }}</summary>
          <textarea v-if="expertOpen" v-model="expertPrompt" class="gw-textarea mt-2" rows="3" />
        </details>
        <label class="gw-checkbox-row">
          <input v-model="autoCleanup" type="checkbox" @change="onAutoCleanupChange" />
          {{ t('imageStudio.autoCleanup') }}
        </label>
        <div v-if="estimate" class="flex flex-wrap items-center gap-3">
          <span class="gw-cost-pill" :class="estimate.sufficient ? 'ok' : 'warn'">
            {{ t('imageStudio.estimate', { cost: estimate.estimated_cost.toFixed(4) }) }}
          </span>
          <button type="button" class="gw-btn gw-btn-primary" :disabled="generating || polling || !apiKeyId" @click="generate">
            {{ generating || polling ? t('imageStudio.generating') : t('imageStudio.generate') }}
          </button>
        </div>
      </section>

        <section v-else-if="!polling && step === 4" class="gw-panel space-y-4">
        <h2 class="gw-section-title">{{ t('imageStudio.doneTitle') }}</h2>
        <p class="gw-subtitle">{{ t('imageStudio.doneHint') }}</p>
        <div v-if="latestJob?.assets?.length" class="gw-gallery">
          <div v-for="asset in latestJob.assets" :key="asset.id" class="gw-thumb">
            <a :href="asset.url" target="_blank" rel="noopener">
              <img :src="asset.url" :alt="latestJob.template_id" loading="lazy" />
            </a>
          </div>
        </div>
        <div class="flex flex-wrap gap-3">
          <button type="button" class="gw-btn gw-btn-primary" @click="step = 3">{{ t('imageStudio.makeAnother') }}</button>
          <router-link to="/play" class="gw-btn gw-btn-secondary">{{ t('imageStudio.goHub') }}</router-link>
          <router-link to="/arena" class="gw-btn gw-btn-secondary">{{ t('imageStudio.goFarm') }}</router-link>
        </div>
        </section>
      </template>

      <section class="gw-panel space-y-3">
        <h2 class="gw-section-title">{{ t('imageStudio.gallery') }}</h2>
        <div v-if="!jobs.length" class="gw-subtitle">{{ t('imageStudio.galleryEmpty') }}</div>
        <div v-else class="gw-gallery">
          <div v-for="job in jobs" :key="job.id" class="space-y-2">
            <div v-for="asset in job.assets || []" :key="asset.id" class="gw-thumb">
              <a :href="asset.url" target="_blank" rel="noopener">
                <img :src="asset.url" :alt="job.template_id" loading="lazy" />
              </a>
            </div>
            <button type="button" class="text-xs" style="color: var(--gw-ink-3)" @click="removeJob(job.id)">
              {{ t('imageStudio.delete') }}
            </button>
          </div>
        </div>
      </section>
    </div>

    <div v-if="showFirstWin" class="gw-first-win" @click.self="showFirstWin = false">
      <div class="gw-first-win-card">
        <h2>{{ t('imageStudio.firstWinTitle') }}</h2>
        <p>{{ t('imageStudio.firstWinHint') }}</p>
        <button type="button" class="gw-btn gw-btn-primary w-full" @click="showFirstWin = false">
          {{ t('imageStudio.firstWinCta') }}
        </button>
      </div>
    </div>
  </AppLayout>
</template>
