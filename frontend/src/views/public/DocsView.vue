<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAuthStore } from '@/stores'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import {
  PUBLIC_DOC_TREE,
  defaultDocPageForCategory,
  findDocPage,
} from '@/content/public-docs'
import '@/styles/public-pages.css'

marked.setOptions({ gfm: true, breaks: true })

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

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

const categories = computed(() =>
  PUBLIC_DOC_TREE.map((cat) => ({
    key: cat.id,
    to: {
      path: '/docs',
      query: { cat: cat.id, page: defaultDocPageForCategory(cat.id) },
    },
  })),
)

const pageTitle = computed(() => {
  if (!isReaderMode.value) return ''
  const key = `docs.pages.${activeCat.value}.${activePage.value}.title`
  return te(key) ? t(key) : activePage.value
})

const pageHtml = computed(() => {
  if (!isReaderMode.value) return ''
  const key = `docs.pages.${activeCat.value}.${activePage.value}.body`
  if (!te(key)) return ''
  const raw = t(key)
  return DOMPurify.sanitize(marked.parse(raw) as string)
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
    if (page && findDocPage(cat, page)) return
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
          <h3 class="docs-card-title">{{ t(`docs.categories.${cat.key}.title`) }}</h3>
          <p class="docs-card-desc">{{ t(`docs.categories.${cat.key}.desc`) }}</p>
        </router-link>
      </div>
    </div>

    <div v-else class="docs-layout">
      <aside class="docs-sidebar">
        <p class="docs-sidebar-title">{{ t('docs.sidebarTitle') }}</p>
        <nav v-for="cat in PUBLIC_DOC_TREE" :key="cat.id" class="docs-sidebar-group">
          <p class="docs-sidebar-cat">{{ t(`docs.categories.${cat.categoryKey}.title`) }}</p>
          <router-link
            v-for="page in cat.pages"
            :key="page.id"
            :to="docLink(cat.id, page.id)"
            class="docs-sidebar-link"
            :class="{ 'is-active': isActivePage(cat.id, page.id) }"
          >
            {{
              te(`docs.pages.${cat.id}.${page.id}.title`)
                ? t(`docs.pages.${cat.id}.${page.id}.title`)
                : page.id
            }}
          </router-link>
        </nav>
      </aside>

      <article class="docs-article">
        <h1 class="docs-article-title">{{ pageTitle }}</h1>
        <div v-if="pageHtml" class="docs-prose" v-html="pageHtml" />
        <p v-else class="docs-state">{{ t('legal.empty') }}</p>
      </article>
    </div>

    <SupportFloatingCard />
  </div>
</template>
