<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import {
  favoritePrompt,
  getPrompt,
  reportPrompt,
  unfavoritePrompt,
  usePrompt,
  type PromptDetail,
} from '@/api/prompts'
import {
  openPromptInImageStudio,
  promptSourceLabel,
  referenceRequirementLabel,
} from '@/utils/promptLibrary'
import { copyPromptText } from '@/utils/promptLibraryClipboard'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import Icon from '@/components/icons/Icon.vue'
import PromptGeneratedCover from '@/components/prompt/PromptGeneratedCover.vue'
import '@/components/prompt/prompt-library.css'
import { isGenericPromptTemplateImage } from '@/utils/promptCover'
import { applyPromptPageMetadata, clearPromptPageMetadata } from '@/utils/promptPageMetadata'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const prompt = ref<PromptDetail | null>(null)
const loading = ref(true)
const loadFailed = ref(false)
const actionBusy = ref(false)
const reportOpen = ref(false)
const reportReason = ref('')
const reportDetail = ref('')

const promptId = computed(() => String(route.params.id || ''))
const exampleImages = computed(() => {
  if (!prompt.value) return []
  const realExamples = (prompt.value.example_images || []).filter((image) => !isGenericPromptTemplateImage(image.url))
  if (realExamples.length) return realExamples
  if (prompt.value.preview_image_url && !isGenericPromptTemplateImage(prompt.value.preview_image_url)) {
    return [{
      url: prompt.value.preview_image_url,
      alt: prompt.value.preview_image_alt || `${prompt.value.title}示例效果`,
    }]
  }
  return []
})
const generatedCoverPrompt = computed(() => {
  if (!prompt.value || exampleImages.value.length > 0) return null
  return prompt.value
})

async function loadDetail() {
  loading.value = true
  loadFailed.value = false
  try {
    prompt.value = await getPrompt(promptId.value)
    applyPromptPageMetadata({
      title: prompt.value.title,
      description: prompt.value.purpose_description,
      path: `/prompts/${encodeURIComponent(prompt.value.id)}`,
      image: prompt.value.preview_image_url,
      kind: 'detail',
      publishedAt: prompt.value.created_at,
    })
  } catch {
    loadFailed.value = true
    prompt.value = null
  } finally {
    loading.value = false
  }
}

async function handleCopy() {
  if (!prompt.value) return
  try {
    await copyPromptText(prompt.value.prompt_template)
    appStore.showSuccess('提示词已复制')
  } catch {
    appStore.showError('复制失败，请手动选择提示词')
  }
}

async function handleFavorite() {
  if (!prompt.value || actionBusy.value) return
  if (!authStore.isAuthenticated) {
    await router.push({
      path: '/login',
      query: { redirect: route.fullPath },
    })
    return
  }

  actionBusy.value = true
  try {
    if (prompt.value.is_favorited) {
      const state = await unfavoritePrompt(prompt.value.id)
      prompt.value.is_favorited = state.favorited
      prompt.value.favorite_count = state.favorite_count ?? prompt.value.favorite_count
    } else {
      const state = await favoritePrompt(prompt.value.id)
      prompt.value.is_favorited = state.favorited
      prompt.value.favorite_count = state.favorite_count ?? prompt.value.favorite_count
    }
  } catch {
    appStore.showError('收藏操作失败，请稍后重试')
  } finally {
    actionBusy.value = false
  }
}

async function handleUse() {
  if (!prompt.value || actionBusy.value) return
  if (!authStore.isAuthenticated) {
    await router.push({
      path: '/login',
      query: { redirect: route.fullPath },
    })
    return
  }
  actionBusy.value = true
  try {
    await openPromptInImageStudio(prompt.value.id, usePrompt, router)
  } catch {
    appStore.showError('暂时无法用于创作，请稍后重试')
  } finally {
    actionBusy.value = false
  }
}

async function submitReport() {
  if (!prompt.value || actionBusy.value || !reportReason.value.trim()) return
  if (!authStore.isAuthenticated) {
    await router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  actionBusy.value = true
  try {
    await reportPrompt(prompt.value.id, reportReason.value.trim(), reportDetail.value.trim())
    reportOpen.value = false
    reportReason.value = ''
    reportDetail.value = ''
    appStore.showSuccess('内容问题已提交，管理员会尽快核验')
  } catch {
    appStore.showError('提交失败，请稍后重试')
  } finally {
    actionBusy.value = false
  }
}

onMounted(loadDetail)
onBeforeUnmount(clearPromptPageMetadata)
</script>

<template>
  <div class="prompt-library-page">
    <header class="prompt-library-header">
      <div class="prompt-library-header-inner">
        <router-link to="/prompts" class="prompt-library-home-link">
          <Icon name="arrowLeft" size="sm" />
          返回选提示词
        </router-link>
        <PublicPageToolbar />
      </div>
    </header>

    <main class="prompt-library-main">
      <section v-if="loading" class="prompt-loading-grid" aria-label="正在加载">
        <div class="prompt-loading-item"></div>
        <div class="prompt-loading-item"></div>
      </section>

      <section v-else-if="loadFailed || !prompt" class="prompt-error">
        <Icon name="exclamationCircle" size="xl" />
        <h1>提示词详情加载失败</h1>
        <p>该提示词可能已下线，或网络暂时不可用。</p>
        <button type="button" class="prompt-primary-button" @click="loadDetail">重新加载</button>
      </section>

      <article v-else class="prompt-detail-layout">
        <section class="prompt-detail-gallery" aria-label="示例效果">
          <img
            v-for="(image, index) in exampleImages"
            :key="image.id || image.url"
            :src="image.url"
            :alt="image.alt || `${prompt.title}示例效果${index + 1}`"
          />
          <PromptGeneratedCover v-if="generatedCoverPrompt" :prompt="generatedCoverPrompt" detail />
        </section>

        <div class="prompt-detail-content">
          <span class="prompt-detail-chip">{{ promptSourceLabel(prompt.source_attribution) }}</span>
          <h1>{{ prompt.title }}</h1>
          <p class="prompt-detail-purpose">{{ prompt.purpose_description }}</p>

          <div class="prompt-card-tags">
            <span v-for="model in prompt.recommended_models" :key="model">{{ model }}</span>
            <span v-for="size in prompt.recommended_sizes" :key="size">{{ size }}</span>
            <span>{{ referenceRequirementLabel(prompt.reference_requirement) }}</span>
          </div>

          <div class="prompt-detail-actions">
            <button type="button" class="prompt-secondary-button" aria-label="复制提示词" @click="handleCopy">
              <Icon name="copy" size="sm" />
              复制提示词
            </button>
            <button
              type="button"
              class="prompt-secondary-button"
              :aria-label="prompt.is_favorited ? '取消收藏' : '收藏提示词'"
              :disabled="actionBusy"
              @click="handleFavorite"
            >
              <Icon name="badge" size="sm" />
              {{ prompt.is_favorited ? '取消收藏' : '收藏提示词' }}
            </button>
            <button
              type="button"
              class="prompt-use-button"
              aria-label="用于创作"
              :disabled="actionBusy"
              @click="handleUse"
            >
              <Icon name="sparkles" size="sm" />
              {{ actionBusy ? '正在准备' : '用于创作' }}
            </button>
          </div>

          <section class="prompt-detail-section">
            <h2>生成提示词（英文）</h2>
            <pre class="prompt-detail-prompt">{{ prompt.prompt_template }}</pre>
          </section>

          <section class="prompt-detail-section">
            <h2>变量定义</h2>
            <dl v-if="prompt.variables.length" class="prompt-detail-variables">
              <div v-for="variable in prompt.variables" :key="variable.name" class="prompt-detail-variable">
                <dt>{{ variable.label }}<span v-if="variable.required">（必填）</span></dt>
                <dd>{{ variable.description || `对应变量：${variable.name}` }}</dd>
              </div>
            </dl>
            <p v-else class="prompt-detail-purpose">该提示词无需填写变量，可直接用于创作。</p>
          </section>

          <section class="prompt-detail-section">
            <h2>参考图要求</h2>
            <p class="prompt-detail-purpose">
              {{ prompt.reference_instructions || referenceRequirementLabel(prompt.reference_requirement) }}
            </p>
          </section>

          <p class="prompt-detail-notice">
            {{ prompt.content_notice || '收录于极速蹬提示词库\n由极速蹬整理、翻译并完成模型适配' }}
          </p>
          <details v-if="prompt.source_notice" class="prompt-detail-section">
            <summary>内容说明</summary>
            <p class="prompt-detail-purpose">{{ prompt.source_notice }}</p>
          </details>
          <section class="prompt-detail-section">
            <button
              type="button"
              class="prompt-secondary-button"
              @click="reportOpen = !reportOpen"
            >
              <Icon name="exclamationCircle" size="sm" />
              反馈内容问题
            </button>
            <form v-if="reportOpen" class="prompt-report-form" @submit.prevent="submitReport">
              <label>
                <span>问题类型</span>
                <select v-model="reportReason" required>
                  <option value="">请选择</option>
                  <option value="版权或来源问题">版权或来源问题</option>
                  <option value="内容不安全">内容不安全</option>
                  <option value="示例效果不匹配">示例效果不匹配</option>
                  <option value="提示词无法使用">提示词无法使用</option>
                  <option value="其他问题">其他问题</option>
                </select>
              </label>
              <label>
                <span>补充说明</span>
                <textarea
                  v-model="reportDetail"
                  rows="3"
                  maxlength="2000"
                  placeholder="请说明需要核验的具体内容"
                ></textarea>
              </label>
              <button type="submit" class="prompt-primary-button" :disabled="actionBusy || !reportReason">
                提交反馈
              </button>
            </form>
          </section>
        </div>
      </article>
    </main>
  </div>
</template>
