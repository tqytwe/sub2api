<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import ImageStudioGallery from '@/components/imageStudio/ImageStudioGallery.vue'
import ImageStudioPreviewModal from '@/components/imageStudio/ImageStudioPreviewModal.vue'
import { useImageStudioWizard } from '@/composables/useImageStudioWizard'
import { isFeatureFlagEnabled, FeatureFlags } from '@/utils/featureFlags'
import '@/styles/growth-world.css'
import Icon from '@/components/icons/Icon.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const enabled = isFeatureFlagEnabled(FeatureFlags.imageStudio)

const wizard = useImageStudioWizard()

type StudioIconName = 'cube' | 'grid' | 'sparkles' | 'document'

function studioIconFor(id: string): StudioIconName {
  if (id === 'ecommerce') return 'cube'
  if (id === 'social') return 'grid'
  if (id === 'creative') return 'sparkles'
  return 'document'
}
</script>

<template>
  <AppLayout>
    <div v-if="!enabled" class="gw-page py-12 text-center">
      <p class="gw-subtitle">{{ t('imageStudio.disabled') }}</p>
    </div>
    <div v-else class="gw-page gw-page--studio space-y-6 pb-10">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p class="gw-eyebrow">{{ t('imageStudio.eyebrow') }}</p>
          <h1 class="gw-title">{{ t('imageStudio.title') }}</h1>
          <p class="gw-subtitle">{{ t('imageStudio.subtitle') }}</p>
        </div>
        <div class="gw-balance-card" :class="{ 'gw-balance-card--low': wizard.balanceLow.value }">
          <p class="gw-balance-label">{{ t('imageStudio.balance') }}</p>
          <p class="gw-balance-value">${{ wizard.balance.value.toFixed(2) }}</p>
          <div class="gw-balance-actions">
            <router-link to="/purchase?return=/image-studio" class="gw-btn gw-btn-primary gw-btn-sm">
              {{ t('imageStudio.recharge') }}
            </router-link>
          </div>
        </div>
      </div>

      <div class="gw-steps">
        <button
          v-for="n in 4"
          :key="n"
          type="button"
          class="gw-step-pill"
          :class="{ active: wizard.step.value >= n, clickable: n < wizard.step.value && !wizard.polling.value && !wizard.generating.value }"
          :disabled="n >= wizard.step.value || wizard.polling.value || wizard.generating.value"
          @click="wizard.goToStep(n)"
        >
          {{ t('imageStudio.step', { n }) }}
        </button>
      </div>

      <div v-if="wizard.bootstrapping.value" class="gw-polling">{{ t('models.loading') }}</div>
      <template v-else>
        <div v-if="wizard.polling.value" class="gw-generating-banner">
          <span class="gw-generating-dot" />
          <span>{{ wizard.pollNotice.value || t('imageStudio.polling') }}</span>
        </div>

        <p v-if="wizard.errorMsg.value" class="gw-error">{{ wizard.errorMsg.value }}</p>

        <section v-if="!wizard.hasApiKeys.value" class="gw-panel space-y-3">
          <h2 class="gw-section-title">{{ t('imageStudio.noApiKeysTitle') }}</h2>
          <p class="gw-subtitle">{{ t('imageStudio.noApiKeysHint') }}</p>
          <router-link to="/keys" class="gw-btn gw-btn-primary">{{ t('imageStudio.goKeys') }}</router-link>
        </section>

        <section v-else-if="wizard.step.value === 1" class="gw-panel">
          <h2 class="gw-section-title">{{ t('imageStudio.pickIntent') }}</h2>
          <div class="gw-grid">
            <button
              v-for="intent in wizard.catalog.value?.intents || []"
              :key="intent.id"
              type="button"
              class="gw-card-btn"
              @click="wizard.pickIntent(intent)"
            >
              <div class="gw-card-icon">
                <Icon :name="studioIconFor(intent.id)" size="lg" />
              </div>
              <div class="gw-card-label">{{ wizard.labelFor(intent.label) }}</div>
            </button>
          </div>
        </section>

        <section v-else-if="wizard.step.value === 2 && wizard.selectedIntent.value" class="gw-panel">
          <h2 class="gw-section-title">{{ t('imageStudio.pickTemplate') }}</h2>
          <div class="gw-grid">
            <button
              v-for="tpl in wizard.selectedIntent.value.templates"
              :key="tpl.id"
              type="button"
              class="gw-card-btn"
              :class="{ selected: wizard.selectedTemplate?.value?.id === tpl.id }"
              @click="wizard.pickTemplate(tpl)"
            >
              <div class="gw-card-icon">
                <Icon :name="studioIconFor(wizard.selectedIntent.value?.id || tpl.id)" size="lg" />
              </div>
              <div class="gw-card-label">{{ wizard.labelFor(tpl.label) }}</div>
              <ul v-if="tpl.compliance_hints?.length" class="gw-hints">
                <li v-for="(hint, i) in tpl.compliance_hints" :key="i">{{ hint }}</li>
              </ul>
            </button>
          </div>
          <button type="button" class="gw-btn gw-btn-secondary mt-4" @click="wizard.goBack()">{{ t('imageStudio.back') }}</button>
        </section>

        <section v-else-if="wizard.step.value === 3 && wizard.selectedTemplate.value" class="gw-panel space-y-4">
          <h2 class="gw-section-title">{{ t('imageStudio.fillForm') }}</h2>
          <p v-if="wizard.isNewUser.value" class="text-sm" style="color: var(--gw-ink-3)">{{ t('imageStudio.newUserHint') }}</p>
          <label class="gw-field">
            <span class="gw-field-label">{{ t('imageStudio.promptLabel') }}</span>
            <input v-model="wizard.userPrompt.value" class="gw-input" :placeholder="t('imageStudio.promptPlaceholder')" />
          </label>
          <label v-if="wizard.showAccentColor.value" class="gw-field">
            <span class="gw-field-label">{{ t('imageStudio.accentColor') }}</span>
            <div class="flex items-center gap-3">
              <input v-model="wizard.accentColor.value" type="color" class="h-10 w-14 cursor-pointer rounded-lg border border-[var(--gw-line)] bg-transparent p-1" />
              <input v-model="wizard.accentColor.value" class="gw-input max-w-[8rem]" />
            </div>
          </label>
          <div class="gw-field-row">
            <label class="gw-field">
              <span class="gw-field-label">{{ t('imageStudio.size') }}</span>
              <select v-model="wizard.size.value" class="gw-select" :disabled="wizard.polling.value || wizard.generating.value">
                <option value="1024x1024">1:1</option>
                <option value="1024x1536">3:4</option>
                <option value="1536x1024">4:3</option>
              </select>
            </label>
            <label class="gw-field">
              <span class="gw-field-label">{{ t('imageStudio.count') }}</span>
              <select v-model.number="wizard.count.value" class="gw-select" :disabled="wizard.polling.value || wizard.generating.value">
                <option v-for="n in wizard.maxCount.value" :key="n" :value="n">{{ n }}</option>
              </select>
            </label>
            <label class="gw-field">
              <span class="gw-field-label">{{ t('imageStudio.apiKey') }}</span>
              <select v-model.number="wizard.apiKeyId.value" class="gw-select" :disabled="wizard.polling.value || wizard.generating.value">
                <option v-for="k in wizard.apiKeys.value" :key="k.id" :value="k.id">{{ k.name }}</option>
              </select>
            </label>
            <label class="gw-field">
              <span class="gw-field-label">{{ t('imageStudio.model') }}</span>
              <select
                v-model="wizard.selectedModel.value"
                class="gw-select"
                :disabled="wizard.loadingModels.value || !wizard.availableModels.value.length || wizard.polling.value || wizard.generating.value"
              >
                <option v-if="wizard.loadingModels.value" value="">{{ t('imageStudio.loadingModels') }}</option>
                <option v-else-if="!wizard.availableModels.value.length" value="">{{ t('imageStudio.noModels') }}</option>
                <option v-for="model in wizard.availableModels.value" :key="model.id" :value="model.id">
                  {{ model.display_name || model.id }}
                </option>
              </select>
            </label>
          </div>
          <p v-if="wizard.modelError.value" class="gw-error">{{ wizard.modelError.value }}</p>
          <p v-else-if="wizard.estimateError.value" class="gw-error">{{ wizard.estimateError.value }}</p>
          <details class="gw-field" @toggle="wizard.expertOpen.value = ($event.target as HTMLDetailsElement).open">
            <summary class="cursor-pointer text-sm" style="color: var(--gw-ink-2)">{{ t('imageStudio.expertPrompt') }}</summary>
            <textarea v-if="wizard.expertOpen.value" v-model="wizard.expertPrompt.value" class="gw-textarea mt-2" rows="3" />
          </details>
          <label class="gw-checkbox-row">
            <input v-model="wizard.autoCleanup.value" type="checkbox" :disabled="wizard.polling.value || wizard.generating.value" @change="wizard.onAutoCleanupChange()" />
            {{ t('imageStudio.autoCleanup') }}
          </label>
          <div v-if="wizard.estimate.value" class="flex flex-wrap items-center gap-3">
            <span class="gw-cost-pill" :class="wizard.estimate.value.sufficient ? 'ok' : 'warn'">
              {{ t('imageStudio.estimate', { cost: wizard.estimate.value.estimated_cost.toFixed(4) }) }}
            </span>
            <button type="button" class="gw-btn gw-btn-primary" :disabled="wizard.generating.value || wizard.polling.value || !wizard.apiKeyId.value || !wizard.selectedModel.value || !wizard.estimate.value" @click="wizard.generate()">
              {{ wizard.generating.value || wizard.polling.value ? t('imageStudio.generating') : t('imageStudio.generate') }}
            </button>
            <button type="button" class="gw-btn gw-btn-secondary" :disabled="wizard.polling.value || wizard.generating.value" @click="wizard.goBack()">
              {{ t('imageStudio.back') }}
            </button>
          </div>
        </section>

        <section v-else-if="wizard.step.value === 4" class="gw-panel space-y-4">
          <h2 class="gw-section-title">{{ t('imageStudio.doneTitle') }}</h2>
          <p class="gw-subtitle">{{ t('imageStudio.doneHint') }}</p>
          <ImageStudioGallery
            :jobs="wizard.jobs.value"
            :latest-job="wizard.latestJob.value"
            result-mode
            @preview="wizard.openPreview"
            @delete="wizard.removeJob"
          />
          <div class="flex flex-wrap gap-3">
            <button type="button" class="gw-btn gw-btn-primary" @click="wizard.step.value = 3">{{ t('imageStudio.makeAnother') }}</button>
            <button type="button" class="gw-btn gw-btn-secondary" @click="wizard.startOver()">{{ t('imageStudio.startOver') }}</button>
            <router-link to="/play" class="gw-btn gw-btn-secondary">{{ t('imageStudio.goHub') }}</router-link>
            <router-link to="/arena" class="gw-btn gw-btn-secondary">{{ t('imageStudio.goFarm') }}</router-link>
          </div>
        </section>
      </template>

      <section v-if="!wizard.bootstrapping.value" class="gw-panel space-y-3">
        <h2 class="gw-section-title">{{ t('imageStudio.gallery') }}</h2>
        <ImageStudioGallery
          :jobs="wizard.jobs.value"
          @preview="wizard.openPreview"
          @delete="wizard.removeJob"
        />
      </section>
    </div>

    <ImageStudioPreviewModal
      :url="wizard.previewUrl.value"
      :filename="wizard.previewFilename.value"
      @close="wizard.closePreview()"
    />

    <div v-if="wizard.showFirstWin.value" class="gw-first-win" @click.self="wizard.showFirstWin.value = false">
      <div class="gw-first-win-card">
        <h2>{{ t('imageStudio.firstWinTitle') }}</h2>
        <p>{{ t('imageStudio.firstWinHint') }}</p>
        <button type="button" class="gw-btn gw-btn-primary w-full" @click="wizard.showFirstWin.value = false">
          {{ t('imageStudio.firstWinCta') }}
        </button>
      </div>
    </div>
  </AppLayout>
</template>
