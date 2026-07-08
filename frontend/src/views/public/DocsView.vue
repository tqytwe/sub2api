<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import { useAuthStore, useAppStore } from '@/stores'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import {
  PUBLIC_DOC_CONTENT_ZH,
  PUBLIC_DOC_TREE,
  defaultDocPageForCategory,
  findDocContent,
} from '@/content/public-docs'
import '@/styles/public-pages.css'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const backTarget = computed(() => (authStore.isAuthenticated ? '/dashboard' : '/home'))
const backLabel = computed(() =>
  authStore.isAuthenticated ? t('docs.backDashboard') : t('contact.backHome'),
)

const activeCat = computed(() => {
  const cat = route.query.cat
  return typeof cat === 'string' ? cat : ''
})

const activePage = computed(() => {
  const page = route.query.page
  return typeof page === 'string' ? page : ''
})

const isReaderMode = computed(() => !!activeCat.value && !!activePage.value)

const activePageContent = computed(() =>
  isReaderMode.value ? findDocContent(activeCat.value, activePage.value) : undefined,
)

const categories = computed(() =>
  PUBLIC_DOC_CONTENT_ZH.map((cat) => ({
    key: cat.id,
    title: cat.title,
    desc: personalizeDocText(cat.description),
    to: {
      path: '/docs',
      query: { cat: cat.id, page: defaultDocPageForCategory(cat.id) },
    },
  })),
)

const pageTitle = computed(() => activePageContent.value?.title ?? activePage.value)

const pageSummary = computed(() =>
  activePageContent.value?.summary ? personalizeDocText(activePageContent.value.summary) : '',
)

function personalizeDocText(raw: string) {
  const siteName = appStore.siteName || '本站'
  const baseUrl = (appStore.apiBaseUrl || window.location.origin).replace(/\/$/, '')
  return raw
    .replace(/随想 AI/g, siteName)
    .replace(/随想/g, siteName)
    .replace(/本站/g, siteName)
    .replace(/https:\/\/sui-xiang\.com/g, baseUrl)
    .replace(/https:\/\/your-host/g, baseUrl)
}

function personalizeDocHtml(raw: string) {
  return personalizeDocText(raw)
}

const pageHtml = computed(() => {
  const html = activePageContent.value?.html
  if (!html) return ''
  return DOMPurify.sanitize(personalizeDocHtml(html))
})

function docLink(catId: string, pageId: string) {
  return { path: '/docs', query: { cat: catId, page: pageId } }
}

function isActivePage(catId: string, pageId: string) {
  return activeCat.value === catId && activePage.value === pageId
}

watch(
  () => route.query,
  (query) => {
    const cat = typeof query.cat === 'string' ? query.cat : ''
    if (!cat) return
    const page = typeof query.page === 'string' ? query.page : ''
    if (page && findDocContent(cat, page)) return
    const fallback = defaultDocPageForCategory(cat)
    if (fallback) {
      router.replace({ path: '/docs', query: { cat, page: fallback } })
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="docs-page">
    <header class="public-page-header docs-header">
      <router-link :to="isReaderMode ? { path: '/docs' } : backTarget" class="back-link">
        {{ isReaderMode ? t('docs.backToIndex') : backLabel }}
      </router-link>
      <PublicPageToolbar />
    </header>

    <div v-if="!isReaderMode" class="docs-main">
      <p class="docs-eyebrow">{{ t('docs.eyebrow') }}</p>
      <h1 class="docs-title">{{ t('docs.title') }}</h1>
      <p class="docs-desc">{{ t('docs.desc') }}</p>

      <div class="docs-grid">
        <router-link
          v-for="cat in categories"
          :key="cat.key"
          class="docs-card"
          :to="cat.to"
        >
          <h3 class="docs-card-title">{{ cat.title }}</h3>
          <p class="docs-card-desc">{{ cat.desc }}</p>
        </router-link>
      </div>
    </div>

    <div v-else class="docs-layout">
      <aside class="docs-sidebar">
        <p class="docs-sidebar-title">{{ t('docs.sidebarTitle') }}</p>
        <nav v-for="cat in PUBLIC_DOC_TREE" :key="cat.id" class="docs-sidebar-group">
          <p class="docs-sidebar-cat">
            {{ PUBLIC_DOC_CONTENT_ZH.find((c) => c.id === cat.id)?.title ?? cat.id }}
          </p>
          <router-link
            v-for="page in cat.pages"
            :key="page.id"
            :to="docLink(cat.id, page.id)"
            class="docs-sidebar-link"
            :class="{ 'is-active': isActivePage(cat.id, page.id) }"
          >
            {{ findDocContent(cat.id, page.id)?.title ?? page.id }}
          </router-link>
        </nav>
      </aside>

      <article class="docs-article">
        <header class="docs-article-head">
          <h1 class="docs-article-title">{{ pageTitle }}</h1>
          <p v-if="pageSummary" class="docs-article-summary">{{ pageSummary }}</p>
        </header>
        <div v-if="pageHtml" class="docs-prose" v-html="pageHtml" />
        <p v-else class="docs-state">{{ t('legal.empty') }}</p>
      </article>
    </div>

    <SupportFloatingCard />
  </div>
</template>
