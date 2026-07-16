<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import { useAuthStore, useAppStore } from '@/stores'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import DocsVipTiersTable from '@/components/public/DocsVipTiersTable.vue'
import {
  PUBLIC_DOC_CONTENT_ZH,
  PUBLIC_DOC_TREE,
  defaultDocPageForCategory,
  findDocContent,
  normalizePublicDocLocation,
} from '@/content/public-docs'

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

const activeCategory = computed(() =>
  PUBLIC_DOC_CONTENT_ZH.find((c) => c.id === activeCat.value),
)

const expandedSections = ref<Set<string>>(new Set())

watch(
  activeCat,
  (cat) => {
    if (cat) expandedSections.value.add(cat)
  },
  { immediate: true },
)

function toggleSection(catId: string) {
  const next = new Set(expandedSections.value)
  if (next.has(catId)) next.delete(catId)
  else next.add(catId)
  expandedSections.value = next
}

function isSectionOpen(catId: string) {
  return expandedSections.value.has(catId)
}

const categories = computed(() =>
  PUBLIC_DOC_CONTENT_ZH.map((cat) => ({
    key: cat.id,
    title: cat.title,
    desc: personalizeDocText(cat.description),
    pages: cat.pages.slice(0, 4).map((p) => p.title),
    moreCount: Math.max(0, cat.pages.length - 4),
    to: {
      path: '/docs',
      query: { cat: cat.id, page: defaultDocPageForCategory(cat.id) },
    },
  })),
)

const flatPages = computed(() =>
  PUBLIC_DOC_CONTENT_ZH.flatMap((cat) =>
    cat.pages.map((page) => ({
      catId: cat.id,
      pageId: page.id,
      title: page.title,
      catTitle: cat.title,
    })),
  ),
)

const currentPageIndex = computed(() =>
  flatPages.value.findIndex(
    (p) => p.catId === activeCat.value && p.pageId === activePage.value,
  ),
)

const prevPage = computed(() => {
  const i = currentPageIndex.value
  return i > 0 ? flatPages.value[i - 1] : null
})

const nextPage = computed(() => {
  const i = currentPageIndex.value
  return i >= 0 && i < flatPages.value.length - 1 ? flatPages.value[i + 1] : null
})

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

const vipLevelsTailHtml = computed(() => {
  if (activeCat.value !== 'recharge-vip' || activePage.value !== 'vip-levels') return ''
  return pageHtml.value
})

function docLink(catId: string, pageId: string) {
  return { path: '/docs', query: { cat: catId, page: pageId } }
}

function isActivePage(catId: string, pageId: string) {
  return activeCat.value === catId && activePage.value === pageId
}

function goToIndex() {
  router.push({ path: '/docs' })
}

function openCategory(catId: string) {
  const page = defaultDocPageForCategory(catId)
  if (page) router.push(docLink(catId, page))
}

watch(
  () => route.query,
  (query) => {
    const cat = typeof query.cat === 'string' ? query.cat : ''
    if (!cat) return
    const page = typeof query.page === 'string' ? query.page : ''
    const normalized = normalizePublicDocLocation(cat, page)
    if (normalized.catId !== cat || normalized.pageId !== page) {
      router.replace({
        path: '/docs',
        query: { cat: normalized.catId, page: normalized.pageId },
      })
      return
    }
    if (page && findDocContent(cat, page)) return
    const fallback = defaultDocPageForCategory(cat)
    if (fallback) {
      router.replace({ path: '/docs', query: { cat, page: fallback } })
    }
  },
  { immediate: true },
)

const categoryIcons: Record<string, string> = {
  tutorial: '📘',
  'recharge-vip': '⭐',
  about: '🛡️',
  'model-learning': '🧠',
  deploy: '🚀',
  tools: '🧰',
  environment: '⚙️',
  'vibe-coding': '✨',
}
</script>

<template>
  <div class="docs-page">
    <header class="docs-topbar">
      <div class="docs-topbar-inner">
        <button type="button" class="docs-back" @click="isReaderMode ? goToIndex() : router.push(backTarget)">
          ← {{ isReaderMode ? t('docs.backToIndex') : backLabel }}
        </button>
        <div class="docs-topbar-title">
          <span class="docs-topbar-eyebrow">DOCS</span>
          <span class="docs-topbar-cn">{{ t('docs.title') }}</span>
        </div>
        <div class="docs-topbar-actions">
          <PublicPageToolbar />
          <router-link
            v-if="!authStore.isAuthenticated"
            to="/register"
            class="docs-cta"
          >
            {{ t('auth.signUp') }}
          </router-link>
        </div>
      </div>
    </header>

    <div class="docs-shell" :class="{ 'is-index': !isReaderMode }">
      <aside v-if="isReaderMode" class="docs-sidebar">
        <button
          type="button"
          class="docs-sidebar-overview"
          :class="{ 'is-active': false }"
          @click="goToIndex"
        >
          <span class="docs-sidebar-overview-icon" aria-hidden="true">☰</span>
          <span>{{ t('docs.sidebarTitle') }}</span>
        </button>

        <div
          v-for="cat in PUBLIC_DOC_TREE"
          :key="cat.id"
          class="docs-sidebar-section"
        >
          <button
            type="button"
            class="docs-sidebar-section-head"
            :class="{ 'is-active': activeCat === cat.id }"
            @click="toggleSection(cat.id)"
          >
            <span class="docs-sidebar-section-title">
              {{ PUBLIC_DOC_CONTENT_ZH.find((c) => c.id === cat.id)?.title ?? cat.id }}
            </span>
            <span
              class="docs-sidebar-section-chevron"
              :class="{ 'is-open': isSectionOpen(cat.id) }"
              aria-hidden="true"
            >▾</span>
          </button>
          <ul v-show="isSectionOpen(cat.id)" class="docs-sidebar-pages">
            <li v-for="page in cat.pages" :key="page.id">
              <router-link
                :to="docLink(cat.id, page.id)"
                class="docs-sidebar-page"
                :class="{ 'is-active': isActivePage(cat.id, page.id) }"
              >
                {{ findDocContent(cat.id, page.id)?.title ?? page.id }}
              </router-link>
            </li>
          </ul>
        </div>
      </aside>

      <main class="docs-content">
        <div v-if="!isReaderMode" class="docs-hero">
          <p class="docs-hero-eyebrow">{{ t('docs.eyebrow') }}</p>
          <h1 class="docs-hero-title">{{ t('docs.title') }}</h1>
          <p class="docs-hero-desc">{{ t('docs.desc') }}</p>

          <div class="docs-cards">
            <button
              v-for="cat in categories"
              :key="cat.key"
              type="button"
              class="docs-card"
              @click="openCategory(cat.key)"
            >
              <span class="docs-card-icon" aria-hidden="true">{{ categoryIcons[cat.key] ?? '📄' }}</span>
              <h3 class="docs-card-title">{{ cat.title }}</h3>
              <p class="docs-card-desc">{{ cat.desc }}</p>
              <div v-if="cat.pages.length" class="docs-card-pages">
                <span v-for="title in cat.pages" :key="title" class="docs-card-page">{{ title }}</span>
                <span v-if="cat.moreCount" class="docs-card-page docs-card-page-more">+{{ cat.moreCount }}</span>
              </div>
              <span class="docs-card-arrow" aria-hidden="true">→</span>
            </button>
          </div>
        </div>

        <article v-else class="docs-article">
          <nav class="docs-breadcrumb" aria-label="Breadcrumb">
            <button type="button" class="docs-crumb" @click="goToIndex">{{ t('docs.title') }}</button>
            <span class="docs-crumb-sep">/</span>
            <button
              type="button"
              class="docs-crumb"
              @click="openCategory(activeCat)"
            >
              {{ activeCategory?.title }}
            </button>
            <span class="docs-crumb-sep">/</span>
            <span class="docs-crumb docs-crumb-current">{{ pageTitle }}</span>
          </nav>

          <header class="docs-article-head">
            <img
              v-if="appStore.siteLogo"
              :src="appStore.siteLogo"
              :alt="appStore.siteName"
              class="docs-article-brand-logo"
            />
            <p class="docs-article-eyebrow">{{ activeCategory?.title }}</p>
            <h1 class="docs-article-title">{{ pageTitle }}</h1>
            <p v-if="pageSummary" class="docs-article-summary">{{ pageSummary }}</p>
          </header>

          <DocsVipTiersTable
            v-if="activeCat === 'recharge-vip' && activePage === 'vip-levels'"
            class="docs-prose docs-vip-tiers-wrap"
          />

          <div v-if="pageHtml && !(activeCat === 'recharge-vip' && activePage === 'vip-levels')" class="docs-prose" v-html="pageHtml" />
          <div
            v-else-if="activeCat === 'recharge-vip' && activePage === 'vip-levels' && vipLevelsTailHtml"
            class="docs-prose"
            v-html="vipLevelsTailHtml"
          />
          <p v-else-if="!pageHtml && !(activeCat === 'recharge-vip' && activePage === 'vip-levels')" class="docs-state">{{ t('legal.empty') }}</p>

          <footer v-if="prevPage || nextPage" class="docs-article-foot">
            <router-link
              v-if="prevPage"
              :to="docLink(prevPage.catId, prevPage.pageId)"
              class="docs-article-foot-btn docs-article-foot-prev"
            >
              <span class="docs-article-foot-dir">{{ t('docs.prevArticle') }}</span>
              <span class="docs-article-foot-name">{{ prevPage.title }}</span>
            </router-link>
            <router-link
              v-if="nextPage"
              :to="docLink(nextPage.catId, nextPage.pageId)"
              class="docs-article-foot-btn docs-article-foot-next"
            >
              <span class="docs-article-foot-dir">{{ t('docs.nextArticle') }}</span>
              <span class="docs-article-foot-name">{{ nextPage.title }}</span>
            </router-link>
          </footer>
        </article>
      </main>
    </div>

    <SupportFloatingCard hide-on-mobile />
  </div>
</template>
