<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import Icon from '@/components/icons/Icon.vue'
import DocsVipTiersTable from '@/components/public/DocsVipTiersTable.vue'
import { useClipboard } from '@/composables/useClipboard'
import { DOC_READER_COPY, DOC_TOOL_GROUPS, DOC_TOOL_INTEGRATIONS } from '@/content/public-docs-integrations'
import { buildDocArticle, personalizeDocCopy, searchPublicDocs } from '@/content/public-docs-reader'
import { normalizePublicDocLocation, type PublicDocLocale } from '@/content/public-docs-tree'
import type { PublicDocCategoryContent } from '@/content/public-docs-data.zh'
import { localizedSiteName } from '@/utils/localizedPublicSettings'
import { sanitizeUrl } from '@/utils/url'

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const isEnglish = computed(() => route.name === 'EnglishDocs')
const docLocale = computed<PublicDocLocale>(() => (isEnglish.value || locale.value === 'en' ? 'en' : 'zh'))
const copy = computed(() => DOC_READER_COPY[docLocale.value])
const docsRoute = computed(() => ({ name: docLocale.value === 'en' ? 'EnglishDocs' : 'Docs' }))
const homeRoute = computed(() => ({ name: docLocale.value === 'en' ? 'EnglishHome' : 'Home' }))
const modelRoute = computed(() => ({ name: docLocale.value === 'en' ? 'EnglishModels' : 'Models' }))
const workspaceQuery = computed(() => docLocale.value === 'en' ? { lang: 'en' } : {})
const consoleRoute = computed(() => ({ name: authStore.isAuthenticated ? 'Dashboard' : 'Login', query: workspaceQuery.value }))
const keysRoute = computed(() => ({ name: 'Keys', query: workspaceQuery.value }))
const siteName = computed(() => localizedSiteName(appStore.cachedPublicSettings?.site_name || appStore.siteName, docLocale.value))
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', {
  allowRelative: true,
  allowDataUrl: true,
}))
const apiBase = computed(() => (sanitizeUrl(appStore.apiBaseUrl) || 'https://api.jisudeng.com').replace(/\/+$/, '').replace(/\/v1$/i, ''))
const apiV1 = computed(() => `${apiBase.value}/v1`)

const docContent = ref<PublicDocCategoryContent[]>([])
const loadState = ref<'loading' | 'ready' | 'error'>('loading')
let requestId = 0

async function loadDocContent() {
  const id = ++requestId
  const language = docLocale.value
  loadState.value = 'loading'
  docContent.value = []
  try {
    const content = language === 'en'
      ? (await import('@/content/public-docs-data.en')).PUBLIC_DOC_CONTENT_EN
      : (await import('@/content/public-docs-data.zh')).PUBLIC_DOC_CONTENT_ZH
    if (id !== requestId) return
    docContent.value = content
    loadState.value = 'ready'
  } catch {
    if (id === requestId) loadState.value = 'error'
  }
}

watch(docLocale, () => void loadDocContent(), { immediate: true })

const activeCat = computed(() => typeof route.query.cat === 'string' ? route.query.cat : '')
const activePage = computed(() => typeof route.query.page === 'string' ? route.query.page : '')
const isIndex = computed(() => !activeCat.value && !activePage.value)
const activeCategory = computed(() => docContent.value.find((category) => category.id === activeCat.value))
const isCategoryIndex = computed(() => Boolean(activeCategory.value && !activePage.value))
const activeArticle = computed(() => activeCategory.value?.pages.find((page) => page.id === activePage.value))
const isToolsIndex = computed(() => isCategoryIndex.value && activeCat.value === 'tools')
const toolSections = computed(() => DOC_TOOL_GROUPS.map((group) => ({
  id: `tools-${group}`,
  title: copy.value.groups[group],
  tools: DOC_TOOL_INTEGRATIONS.filter((tool) => tool.group === group).flatMap((tool) => {
    const page = activeCategory.value?.pages.find(({ id }) => id === tool.page)
    if (!page) return []
    return [{
      id: page.id,
      name: page.id === 'app-deployment' ? page.title : tool.name,
      summary: page.summary,
      protocol: copy.value.protocols[tool.protocol],
    }]
  }),
})))
const isVipArticle = computed(() => activeCat.value === 'recharge-vip' && activePage.value === 'vip-levels')
const expanded = ref<Set<string>>(new Set(['deploy']))
const sidebarOpen = ref(false)
const mobileToc = ref<HTMLDetailsElement | null>(null)

function docLink(cat: string, page?: string) {
  return { ...docsRoute.value, query: page ? { cat, page } : { cat } }
}

watch([activeCat, activePage, loadState], ([cat, page, state]) => {
  if (!cat || state !== 'ready') return
  const normalized = normalizePublicDocLocation(cat, page)
  if (normalized.catId !== cat || normalized.pageId !== page) {
    void router.replace({ ...docLink(normalized.catId, normalized.pageId), hash: route.hash })
    return
  }
}, { immediate: true })

watch([activeCat, activePage], ([cat]) => {
  if (cat) expanded.value.add(cat)
  sidebarOpen.value = false
  if (mobileToc.value) mobileToc.value.open = false
}, { immediate: true })

function toggleCategory(id: string) {
  if (expanded.value.has(id)) expanded.value.delete(id)
  else expanded.value.add(id)
}

function personalize(raw: string) {
  return personalizeDocCopy(raw, siteName.value || 'Jisudeng', apiBase.value)
}

const article = computed(() => buildDocArticle(personalize(activeArticle.value?.html ?? ''), copy.value.copyCode, docLocale.value))
const headings = computed(() => isToolsIndex.value ? [
  { id: 'tools-basics', title: copy.value.universal, level: 2 },
  ...toolSections.value.map(({ id, title }) => ({ id, title, level: 2 })),
  { id: 'tools-choice', title: copy.value.choose, level: 2 },
  { id: 'tools-diagnose', title: copy.value.diagnose, level: 2 },
] : article.value.headings)
const main = ref<HTMLElement | null>(null)

watch([() => route.hash, headings, loadState], async ([hash]) => {
  if (!hash) return
  await nextTick()
  const id = hash.slice(1)
  const target = [...(main.value?.querySelectorAll('[id]') ?? [])].find((element) => element.id === id)
  requestAnimationFrame(() => target?.scrollIntoView({ block: 'start' }))
}, { flush: 'post', immediate: true })

function headingLink(id: string) {
  return { ...docsRoute.value, query: route.query, hash: `#${id}` }
}

const flatPages = computed(() => docContent.value.flatMap((category) => category.pages.map((page) => {
  const integration = DOC_TOOL_INTEGRATIONS.find((tool) => tool.cat === category.id && tool.page === page.id)
  return {
    title: page.title,
    cat: category.id,
    page: page.id,
    category: category.title,
    summary: personalize(page.summary),
    keywords: [category.title, page.summary, page.id, integration?.name, integration && copy.value.protocols[integration.protocol]].filter(Boolean).join(' '),
  }
})))
const pageIndex = computed(() => flatPages.value.findIndex((page) => page.cat === activeCat.value && page.page === activePage.value))
const prevPage = computed(() => pageIndex.value > 0 ? flatPages.value[pageIndex.value - 1] : undefined)
const nextPage = computed(() => pageIndex.value >= 0 ? flatPages.value[pageIndex.value + 1] : undefined)

const searchOpen = ref(false)
const searchQuery = ref('')
const searchInput = ref<HTMLInputElement | null>(null)
const searchResults = computed(() => searchPublicDocs(flatPages.value, searchQuery.value).slice(0, 16))

async function openSearch() {
  if (searchOpen.value) return
  searchQuery.value = ''
  searchOpen.value = true
  await nextTick()
  searchInput.value?.focus()
}

function navigateToFirstResult() {
  const result = searchResults.value[0]
  if (!result || !searchQuery.value.trim()) return
  searchOpen.value = false
  void router.push(docLink(result.cat, result.page))
}

function onSearchShortcut(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    void openSearch()
  }
}

onMounted(() => window.addEventListener('keydown', onSearchShortcut))
onUnmounted(() => window.removeEventListener('keydown', onSearchShortcut))

const linkCopied = ref(false)
watch(() => route.fullPath, () => { linkCopied.value = false })

async function copyLink() {
  const path = route.fullPath
  const copied = await copyToClipboard(new URL(path, window.location.origin).href)
  if (copied && path === route.fullPath) linkCopied.value = true
}

async function copySnippet(event: MouseEvent) {
  if (!(event.target instanceof Element)) return
  const button = event.target.closest<HTMLButtonElement>('button[data-doc-copy]')
  if (!button || button.disabled) return
  const text = article.value.code[Number(button.dataset.docCopy)]
  if (!text) return
  button.disabled = true
  const success = await copyToClipboard(text)
  button.disabled = false
  if (success) button.textContent = copy.value.copied
}
</script>

<template>
  <div class="docs-page docs-reader">
    <header class="docs-reader-topbar">
      <RouterLink :to="homeRoute" class="docs-reader-brand">
        <img :src="siteLogo || '/logo.png'" alt="" width="32" height="32" />
        <span>{{ siteName }}</span>
      </RouterLink>
      <nav class="docs-reader-topnav" :aria-label="copy.docs">
        <RouterLink :to="modelRoute">{{ copy.models }}</RouterLink>
        <RouterLink :to="consoleRoute">{{ copy.console }}</RouterLink>
      </nav>
      <div class="docs-reader-actions">
        <button type="button" class="docs-action docs-search-trigger" :aria-label="copy.search" @click="openSearch">
          <Icon name="search" size="sm" aria-hidden="true" />
          <span>{{ copy.search }}</span>
        </button>
        <PublicPageToolbar />
        <button
          type="button"
          class="docs-action docs-mobile-menu"
          :aria-label="copy.navigation"
          :aria-expanded="sidebarOpen"
          aria-controls="docs-site-navigation"
          @click="sidebarOpen = !sidebarOpen"
        >
          <Icon :name="sidebarOpen ? 'x' : 'menu'" size="sm" aria-hidden="true" />
        </button>
      </div>
    </header>

    <details v-if="headings.length" ref="mobileToc" class="docs-mobile-toc">
      <summary><Icon name="document" size="sm" aria-hidden="true" />{{ copy.onPage }}</summary>
      <nav :aria-label="copy.onPage">
        <RouterLink v-for="heading in headings" :key="heading.id" :to="headingLink(heading.id)" @click="mobileToc && (mobileToc.open = false)">{{ heading.title }}</RouterLink>
      </nav>
    </details>

    <div class="docs-reader-layout">
      <nav id="docs-site-navigation" class="docs-reader-sidebar" :class="{ 'is-open': sidebarOpen }" :aria-label="copy.navigation">
        <RouterLink :to="docsRoute" class="docs-tree-root" :class="{ 'is-current': isIndex }" :aria-current="isIndex ? 'page' : undefined" @click="sidebarOpen = false">
          <Icon name="terminal" size="sm" aria-hidden="true" />{{ copy.docsHome }}
        </RouterLink>
        <div v-for="category in docContent" :key="category.id" class="docs-tree-group">
          <div v-if="category.id === 'tools'" class="docs-tree-category-row">
            <RouterLink
              :to="docLink(category.id)"
              class="docs-tree-toggle docs-tree-category-link"
              :aria-current="isToolsIndex ? 'page' : undefined"
              @click="sidebarOpen = false"
            >{{ category.title }}</RouterLink>
            <button
              type="button"
              class="docs-tree-category-expand"
              :aria-label="`${copy.navigation}: ${category.title}`"
              :aria-expanded="expanded.has(category.id)"
              :aria-controls="`docs-tree-${category.id}`"
              @click="toggleCategory(category.id)"
            >
              <Icon :name="expanded.has(category.id) ? 'chevronDown' : 'chevronRight'" size="xs" aria-hidden="true" />
            </button>
          </div>
          <button
            v-else
            type="button"
            class="docs-tree-toggle"
            :aria-expanded="expanded.has(category.id)"
            :aria-controls="`docs-tree-${category.id}`"
            @click="toggleCategory(category.id)"
          >
            <span>{{ category.title }}</span>
            <Icon :name="expanded.has(category.id) ? 'chevronDown' : 'chevronRight'" size="xs" aria-hidden="true" />
          </button>
          <ul v-show="expanded.has(category.id)" :id="`docs-tree-${category.id}`" class="docs-tree-items">
            <li v-for="page in category.pages" :key="page.id">
              <RouterLink
                :to="docLink(category.id, page.id)"
                :aria-current="activeCat === category.id && activePage === page.id ? 'page' : undefined"
                @click="sidebarOpen = false"
              >{{ page.title }}</RouterLink>
            </li>
          </ul>
        </div>
      </nav>

      <main id="docs-main" ref="main" class="docs-reader-main" :aria-busy="loadState === 'loading'">
        <nav class="docs-reader-crumb" :aria-label="copy.docs">
          <RouterLink v-if="activeCategory && activePage" :to="docLink(activeCat)" class="docs-back-link">
            <Icon name="arrowLeft" size="xs" aria-hidden="true" />{{ copy.backToCategory }}
          </RouterLink>
          <RouterLink v-else-if="!isIndex" :to="docsRoute" class="docs-back-link">
            <Icon name="arrowLeft" size="xs" aria-hidden="true" />{{ copy.backToDocs }}
          </RouterLink>
          <RouterLink v-else :to="docsRoute">{{ copy.docs }}</RouterLink>
          <Icon name="chevronRight" size="xs" aria-hidden="true" />
          <span>{{ isIndex ? copy.docsHome : (activeCategory?.title ?? copy.navigation) }}</span>
        </nav>

        <p v-if="loadState === 'loading'" role="status" class="docs-reader-state">{{ copy.load }}</p>
        <div v-else-if="loadState === 'error'" class="docs-reader-state" role="alert">
          <p>{{ copy.loadError }}</p><button type="button" class="docs-action" @click="loadDocContent">{{ copy.retry }}</button>
        </div>
        <article v-else-if="isIndex" class="docs-reader-article">
          <header class="docs-reader-heading">
            <h1>{{ copy.docsHome }}</h1>
            <p>{{ copy.intro }}</p>
          </header>
          <div class="docs-prose">
            <div class="docs-tip docs-integration-basics">
              <strong>{{ copy.universal }}</strong>
              <ul>
                <li>{{ copy.openAIBase }}：<code>{{ apiV1 }}</code></li>
                <li>{{ copy.anthropicBase }}：<code>{{ apiBase }}</code> ({{ copy.anthropicSuffix }})</li>
                <li><RouterLink :to="keysRoute">{{ copy.key }}</RouterLink> · <RouterLink :to="modelRoute">{{ copy.models }}</RouterLink>. {{ copy.modelHelp }}</li>
              </ul>
            </div>
            <h2>{{ copy.firstRequest }}</h2>
            <p>{{ copy.firstRequestText }} <RouterLink :to="docLink('tutorial', 'quick-start')">{{ copy.quickStart }}</RouterLink></p>
          </div>
        </article>

        <article v-else-if="isCategoryIndex && activeCategory" class="docs-reader-article">
          <header class="docs-reader-heading">
            <h1>{{ activeCategory.title }}</h1>
            <p>{{ activeCategory.description }}</p>
          </header>
          <div v-if="isToolsIndex" class="docs-prose">
            <h2 id="tools-basics">{{ copy.universal }}</h2>
            <div class="docs-tip docs-integration-basics">
              <ul>
                <li><RouterLink :to="docLink('tutorial', 'api-key')">{{ copy.key }}</RouterLink> · <RouterLink :to="modelRoute">{{ copy.models }}</RouterLink> · {{ copy.modelHelp }}</li>
                <li>{{ copy.openAIBase }} <code>{{ apiV1 }}</code></li>
                <li>{{ copy.anthropicBase }} <code>{{ apiBase }}</code> ({{ copy.anthropicSuffix }})</li>
                <li>{{ copy.geminiBase }} <code>{{ apiBase }}</code> (<RouterLink :to="docLink('tools', 'gemini-cli')">Gemini CLI</RouterLink>)</li>
              </ul>
            </div>
            <section v-for="section in toolSections" :key="section.id" :aria-labelledby="section.id">
              <h2 :id="section.id">{{ section.title }}</h2>
              <table class="docs-table docs-tool-table" :aria-labelledby="section.id">
                <thead><tr><th scope="col">{{ copy.tool }}</th><th scope="col">{{ copy.protocol }}</th></tr></thead>
                <tbody>
                  <tr v-for="tool in section.tools" :key="tool.id">
                    <td>
                      <RouterLink :to="docLink('tools', tool.id)" class="docs-tool-link">{{ tool.name }}</RouterLink>
                      <span class="docs-tool-summary">{{ personalize(tool.summary) }}</span>
                    </td>
                    <td>{{ tool.protocol }}</td>
                  </tr>
                </tbody>
              </table>
            </section>
            <h2 id="tools-choice">{{ copy.choose }}</h2>
            <p>{{ copy.chooseText }}</p>
            <h2 id="tools-diagnose">{{ copy.diagnose }}</h2>
            <ol><li>{{ copy.diagnoseKey }}</li><li>{{ copy.diagnoseUrl }}</li><li>{{ copy.diagnoseUsage }}</li></ol>
          </div>
          <ul v-else class="docs-prose">
            <li v-for="page in activeCategory.pages" :key="page.id">
              <RouterLink :to="docLink(activeCat, page.id)">{{ page.title }}</RouterLink>
            </li>
          </ul>
        </article>

        <article v-else-if="activeArticle" class="docs-reader-article">
          <header class="docs-reader-heading">
            <h1>{{ activeArticle.title }}</h1>
            <p v-if="activeArticle.summary">{{ personalize(activeArticle.summary) }}</p>
          </header>
          <DocsVipTiersTable v-if="isVipArticle" class="docs-prose docs-vip-tiers-wrap" />
          <!-- The article builder sanitizes the localized payload before adding navigation and copy controls. -->
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div v-if="article.html" class="docs-prose" @click="copySnippet" v-html="article.html" />
          <p v-else-if="!isVipArticle" class="docs-reader-state">{{ t('legal.empty') }}</p>
          <footer v-if="prevPage || nextPage" class="docs-reader-pagination">
            <RouterLink v-if="prevPage" :to="docLink(prevPage.cat, prevPage.page)">
              <small>{{ t('docs.prevArticle') }}</small><span>{{ prevPage.title }}</span>
            </RouterLink>
            <RouterLink v-if="nextPage" :to="docLink(nextPage.cat, nextPage.page)">
              <small>{{ t('docs.nextArticle') }}</small><span>{{ nextPage.title }}</span>
            </RouterLink>
          </footer>
        </article>

        <div v-else class="docs-reader-state">
          <p>{{ copy.notFound }}</p><RouterLink :to="docsRoute">{{ copy.docsHome }}</RouterLink>
        </div>
      </main>

      <aside class="docs-reader-rail">
        <nav v-if="headings.length" :aria-label="copy.onPage" class="docs-reader-toc">
          <strong><Icon name="document" size="sm" aria-hidden="true" />{{ copy.onPage }}</strong>
          <RouterLink
            v-for="heading in headings"
            :key="heading.id"
            :to="headingLink(heading.id)"
            :class="{ 'docs-toc-child': heading.level === 3, 'is-current': route.hash === `#${heading.id}` }"
            :aria-current="route.hash === `#${heading.id}` ? 'location' : undefined"
          >{{ heading.title }}</RouterLink>
        </nav>
        <div v-if="loadState === 'ready' && (isIndex || isCategoryIndex || activeArticle)" class="docs-reader-share">
          <strong>{{ copy.share }}</strong>
          <button type="button" class="docs-action" @click="copyLink">
            <Icon :name="linkCopied ? 'check' : 'copy'" size="sm" aria-hidden="true" />{{ linkCopied ? copy.copied : copy.copyLink }}
          </button>
        </div>
      </aside>
    </div>

    <BaseDialog :show="searchOpen" :title="copy.search" close-on-click-outside @close="searchOpen = false">
      <form role="search" class="docs-search-form" @submit.prevent="navigateToFirstResult">
        <input ref="searchInput" v-model="searchQuery" type="search" class="input" :aria-label="copy.search" :placeholder="copy.searchHint" />
      </form>
      <p v-if="loadState !== 'ready'" class="docs-search-state" role="status">{{ loadState === 'error' ? copy.loadError : copy.load }}</p>
      <p v-else-if="!searchResults.length" class="docs-search-state" role="status">{{ copy.searchEmpty }}</p>
      <ul v-else class="docs-search-results">
        <li v-for="result in searchResults" :key="`${result.cat}:${result.page}`">
          <RouterLink :to="docLink(result.cat, result.page)" @click="searchOpen = false">
            <strong>{{ result.title }}</strong><span>{{ result.category }} · {{ result.summary }}</span>
          </RouterLink>
        </li>
      </ul>
    </BaseDialog>
    <SupportFloatingCard hide-on-mobile />
  </div>
</template>

<style src="@/styles/public-docs.css"></style>
