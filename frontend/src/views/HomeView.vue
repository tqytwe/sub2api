<template>
  <!-- 管理员自定义首页内容优先 -->
  <div v-if="homeContent" class="custom-home-page min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    />
    <div v-else v-html="safeHomeContent" />
    <footer class="custom-home-claim-footer">
      <LmspeedBadge />
    </footer>
  </div>

  <div
    v-else
    class="home-page"
    :class="{
      'is-intro': isIntro,
      'has-guest-sticky-cta': !isAuthenticated && showGuestStickyCta,
    }"
  >
    <header class="page-header" :class="{ scrolled: headerScrolled }">
      <div class="page-container header-row">
        <div class="header-left">
          <router-link :to="homeLogoRoute" class="brand">
            <span v-if="siteLogo" class="brand-mark" aria-hidden="true">
              <img :src="siteLogo" :alt="siteName" />
            </span>
            <span class="brand-name">{{ siteName }}</span>
          </router-link>
          <nav class="header-nav">
            <router-link
              v-for="item in homePrimaryNavItems"
              :key="item.key"
              :to="item.to"
              class="nav-link"
            >
              {{ t(item.labelKey) }}
            </router-link>
          </nav>
        </div>
        <nav class="page-nav">
          <PublicPageToolbar />
          <router-link :to="downloadRoute" class="nav-download">
            {{ t('home.jisudeng.nav.androidApp') }}
          </router-link>
          <template v-if="isAuthenticated">
            <router-link v-if="isAdmin" :to="adminDashboardRoute" class="nav-link">{{ t('home.jisudeng.nav.admin') }}</router-link>
            <router-link :to="dashboardRoute" class="nav-cta">{{ t('home.jisudeng.nav.console') }}</router-link>
          </template>
          <template v-else>
            <router-link :to="loginRoute" class="nav-link">{{ t('home.jisudeng.nav.signIn') }}</router-link>
            <router-link :to="registerRoute" class="nav-cta">{{ t('home.jisudeng.nav.signUp') }}</router-link>
          </template>
        </nav>
      </div>
    </header>

    <section class="hero-section">
      <HeroSphere @reveal="onReveal" />
      <div class="page-container hero-block">
        <p class="hero-eyebrow">
          <template v-for="(bit, idx) in eyebrowBits" :key="idx">
            <span v-if="idx > 0" class="eb-dot" :style="{ '--ebi': idx }" aria-hidden="true">·</span>
            <span class="eb-bit" :style="{ '--ebi': idx }">
              <template v-if="bit.pre">
                <span class="eb-no">{{ bit.pre }}</span>
                <span class="eb-strike">{{ bit.obj }}</span>
              </template>
              <span v-else class="eb-em">{{ bit.text }}</span>
            </span>
          </template>
        </p>
        <h1 class="hero-title" :class="{ 'hero-title--en': isEnglishPublicRoute }">
          <span class="hero-zh">
            <span class="hz-brand">{{ t('home.jisudeng.hero.titleParts.brand') }}</span>
            <span class="hz-mid">{{ t('home.jisudeng.hero.titleParts.mid') }}</span>
            <span class="hz-tail">{{ t('home.jisudeng.hero.titleParts.tail') }}</span>
          </span>
          <span class="hero-en">{{ heroSubtitle }}</span>
        </h1>
        <p class="hero-slogan">{{ t('home.jisudeng.hero.tagline') }}</p>
        <ul v-if="perkLines.length" class="hero-perks">
          <li v-for="(line, idx) in perkLines" :key="idx">{{ line }}</li>
        </ul>
        <div class="hero-ctas">
          <button type="button" class="cta-primary" @click="goStart">
            {{ isAuthenticated ? t('home.jisudeng.cta.console') : t('home.jisudeng.cta.start') }}
            <span class="arrow">→</span>
          </button>
          <button v-if="docUrl || isEnglishPublicRoute" type="button" class="cta-text" @click="openDocs">
            {{ t('home.jisudeng.cta.docs') }}
            <span class="arrow-tiny">↗</span>
          </button>
        </div>
        <ul class="active-on">
          <li class="active-on-label">{{ t('home.jisudeng.hero.activeOn') }}</li>
          <li>Claude Code</li>
          <li class="dot">·</li>
          <li>Codex CLI</li>
          <li class="dot">·</li>
          <li>Cline</li>
          <li class="dot">·</li>
          <li>Gemini CLI</li>
          <li class="dot">·</li>
          <li>Cursor</li>
          <li class="dot">·</li>
          <li>Continue</li>
        </ul>
      </div>
      <a class="hero-scroll-cue" href="#manifesto" :aria-label="t('home.jisudeng.anchors.scrollToContent')">
        <span class="scroll-track"><span class="scroll-dot" /></span>
      </a>
    </section>

    <section id="manifesto" class="manifesto-section" :class="{ 'in-view': inView.manifesto }">
      <div class="page-container manifesto-block">
        <p class="manifesto-tag">{{ t('home.jisudeng.manifesto.tag') }}</p>
        <h2 class="manifesto-title has-token-play">
          <span class="manifesto-sweep" aria-hidden="true" />
          <template v-if="manifestoParts.keyword">
            <span>{{ manifestoParts.before }}</span>
            <span class="title-token">{{ manifestoParts.keyword }}</span>
            <span>{{ manifestoParts.after }}</span>
          </template>
          <template v-else>{{ t('home.jisudeng.manifesto.title') }}</template>
        </h2>
        <div v-if="manifestoParts.keyword && showVerifyLink" class="integrity-check">
          <router-link :to="aboutRoute" class="integrity-verify-link">
            {{ t('home.jisudeng.manifesto.verifyLink') }}
          </router-link>
        </div>
        <div class="manifesto-body">
          <p>{{ t('home.jisudeng.manifesto.body1') }}</p>
          <p>{{ t('home.jisudeng.manifesto.body2') }}</p>
        </div>
        <ul class="manifesto-pledges">
          <li v-for="n in 4" :key="n" class="pledge" tabindex="0">
            <span class="pledge-mark" aria-hidden="true">
              <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">
                <template v-if="n === 1">
                  <rect x="5" y="11" width="14" height="9" rx="1.5" />
                  <path d="M8 11V7a4 4 0 0 1 8 0v4" />
                  <path d="M12 14v3" />
                </template>
                <template v-else-if="n === 2">
                  <path d="M4 9h13l-3-3" />
                  <path d="M20 15H7l3 3" />
                </template>
                <template v-else-if="n === 3">
                  <path d="M12 3v18" />
                  <path d="M5 21h14" />
                  <path d="M4 7h16" />
                  <path d="M4 7l-2.5 6h5L4 7z" />
                  <path d="M20 7l-2.5 6h5L20 7z" />
                </template>
                <path v-else d="M13 2L4 14h7l-1 8 9-12h-7l1-8z" />
              </svg>
            </span>
            <span class="pledge-label">{{ t(`home.jisudeng.manifesto.pledges.p${n}`) }}</span>
            <div class="pledge-card" role="tooltip">
              <p class="pledge-card-title">{{ t(`home.jisudeng.manifesto.pledges.p${n}`) }}</p>
              <p class="pledge-card-desc">{{ t(`home.jisudeng.manifesto.pledges.d${n}`) }}</p>
            </div>
          </li>
        </ul>
      </div>
    </section>

    <section id="stats" class="stats-section" :class="{ 'in-view': inView.stats }">
      <div class="page-container">
        <div class="stats-strip">
          <div v-for="stat in statItems" :key="stat.key" class="stat" :class="`stat--${stat.key}`">
            <span class="sr-only">{{ stat.value }}{{ stat.unit }} {{ t(`home.jisudeng.stats.${stat.key}`) }}</span>
            <HomeStatOdometer
              :value="stat.value"
              :unit="stat.unit"
              :active="inView.stats"
              :spin-tail="stat.key === 'uptime' ? 2 : 3"
            />
            <span class="stat-label" aria-hidden="true">{{ t(`home.jisudeng.stats.${stat.key}`) }}</span>
          </div>
        </div>
        <div v-if="statsFreshness.length" class="stats-freshness" :class="{ 'is-stale': isStatsStale }">
          <span v-for="item in statsFreshness" :key="item">{{ item }}</span>
          <strong v-if="isStatsStale">{{ t('home.jisudeng.stats.stale') }}</strong>
        </div>
      </div>
    </section>

    <section id="lmspeed" class="lmspeed-proof-section section-block" :class="{ 'in-view': inView.lmspeed }">
      <div class="page-container">
        <LmspeedProviderProof />
      </div>
    </section>

    <section id="image" class="image-section" :class="{ 'in-view': inView.image }">
      <div class="page-container section-block">
        <div class="section-head">
          <span class="section-tag">{{ t('home.jisudeng.sections.imageTag') }}</span>
          <h2 class="section-title">{{ t('home.jisudeng.sections.imageTitle') }}</h2>
          <p class="section-lede">{{ t('home.jisudeng.sections.imageLede') }}</p>
        </div>
        <div class="image-showcase">
          <div class="img-info">
            <span class="chc-rule" aria-hidden="true" />
            <div class="img-model-line">
              <span class="img-model">{{ t('home.jisudeng.image.model') }}</span>
              <span class="img-badge">{{ t('home.jisudeng.image.badge') }}</span>
              <span class="img-vendor">· OpenAI</span>
            </div>
            <p class="img-desc">{{ t('home.jisudeng.image.desc') }}</p>
            <div class="img-endpoints">
              <div v-for="ep in imageEndpoints" :key="ep.path" class="img-endpoint">
                <span class="img-ep-method">{{ ep.method }}</span>
                <span class="img-ep-path">{{ ep.path }}</span>
              </div>
            </div>
            <ul class="img-caps">
              <li v-for="(cap, i) in imageCaps" :key="i" class="img-cap">{{ cap }}</li>
            </ul>
            <router-link :to="imageDocsRoute" class="img-doclink">
              {{ t('home.jisudeng.image.docLink') }}
              <span class="img-doclink-arrow">→</span>
            </router-link>
            <router-link :to="studioCtaLink" class="img-doclink mt-2 inline-flex">
              {{ t('home.jisudeng.image.studioCta') }}
              <span class="img-doclink-arrow">→</span>
            </router-link>
          </div>
          <div class="img-demo" aria-hidden="true">
            <div class="demo-prompt">
              <span class="demo-prompt-label">{{ t('home.jisudeng.image.promptLabel') }}</span>
              <span class="demo-prompt-body">
                <span class="demo-prompt-text">{{ t('home.jisudeng.image.promptText') }}</span>
                <span class="demo-prompt-caret" />
              </span>
            </div>
            <div class="demo-canvas">
              <div class="demo-canvas-loading" />
              <div class="demo-canvas-photo" />
              <div class="demo-canvas-noise" />
              <div class="demo-canvas-sheen" />
            </div>
            <div class="demo-foot">
              <div class="demo-progress">
                <div class="demo-progress-fill"><div class="demo-progress-shimmer" /></div>
              </div>
              <div class="demo-status">
                <span class="demo-status-gen">
                  <span class="demo-dot" />
                  {{ t('home.jisudeng.image.statusGen') }}
                  <span class="demo-status-meta"> · {{ t('home.jisudeng.image.statusMeta') }}</span>
                </span>
                <span class="demo-status-done">{{ t('home.jisudeng.image.statusDone') }}</span>
              </div>
            </div>
            <div class="demo-badge">
              <span class="demo-badge-dot" />
              {{ t('home.jisudeng.image.demoBadge') }}
            </div>
          </div>
        </div>
      </div>
    </section>

    <section id="channels" class="channels-section" :class="{ 'in-view': inView.channels }">
      <div class="page-container section-block">
        <div class="section-head">
          <span class="section-tag">{{ t('home.jisudeng.channels.tag') }}</span>
          <h2 class="section-title">{{ t('home.jisudeng.channels.title') }}</h2>
        </div>
        <div class="channels-layout">
          <div class="channels-copy">
            <span class="chc-rule" aria-hidden="true" />
            <p class="chc-title">{{ t('home.jisudeng.channels.copyTitle') }}</p>
            <p class="chc-body">{{ t('home.jisudeng.channels.copyBody') }}</p>
          </div>
          <div class="channels-tv"><ChannelTV /></div>
        </div>
      </div>
    </section>

    <section id="features" class="features-section" :class="{ 'in-view': inView.features }">
      <div class="page-container section-block">
        <div class="section-head">
          <span class="section-tag">{{ t('home.jisudeng.sections.featuresTag') }}</span>
          <h2 class="section-title">{{ t('home.jisudeng.sections.featuresTitle') }}</h2>
        </div>
        <ul class="why-ledger" @mousemove="onWhyMove" @mouseleave="onWhyLeave">
          <li
            v-for="(row, idx) in featureRows"
            :key="row.en"
            class="why-row"
            :style="{ '--d': `${idx * 90}ms` }"
            @mouseenter="onWhyEnter(idx, $event)"
          >
            <span class="why-idx">{{ row.idx }}</span>
            <div class="why-head">
              <h3 class="why-title">{{ row.title }}</h3>
              <span class="why-en">{{ row.en }}</span>
            </div>
            <p class="why-desc">{{ row.desc }}</p>
          </li>
        </ul>
        <WhyHoverCard :active="whyActive" :x="whyX" :y="whyY" />
      </div>
    </section>

    <section id="onboard" class="code-section" :class="{ 'in-view': inView.onboard }">
      <div class="page-container section-block">
        <div class="section-head">
          <span class="section-tag">{{ t('home.jisudeng.sections.codeTag') }}</span>
          <h2 class="section-title">{{ t('home.jisudeng.sections.codeTitle') }}</h2>
          <p class="section-lede">{{ t('home.jisudeng.sections.codeLede') }}</p>
        </div>
        <div class="onboard-grid">
          <ol class="onboard-steps">
            <li
              v-for="(step, idx) in onboardSteps"
              :key="idx"
              class="onboard-step"
              :class="{ 'is-done': onboardPhase > idx + 1, 'is-now': onboardPhase === idx + 1 }"
            >
              <span class="onboard-no">{{ idx + 1 }}</span>
              <div class="onboard-step-body">
                <h3 class="onboard-step-title">{{ step.t }}</h3>
                <p class="onboard-step-desc">{{ step.d }}</p>
              </div>
            </li>
          </ol>
          <TerminalDemo @phase="(p) => (onboardPhase = p)" />
        </div>
        <p class="onboard-foot">
          {{ t('home.jisudeng.onboard.docLink') }}
          <router-link :to="quickStartDocsRoute" class="onboard-foot-link">
            {{ t('home.jisudeng.onboard.docLinkCta') }}
          </router-link>
        </p>
      </div>
    </section>

    <section id="pricing" class="pricing-section" :class="{ 'in-view': inView.pricing }">
      <div class="page-container section-block">
        <span class="section-tag">{{ t('home.jisudeng.sections.pricingTag') }}</span>
        <h2 class="pricing-headline">
          <span>{{ t('home.jisudeng.pricing.lineA') }}</span>
          <span class="pricing-line2">{{ t('home.jisudeng.pricing.lineB') }}</span>
          <span>{{ t('home.jisudeng.pricing.lineC') }}</span>
        </h2>
        <p class="pricing-blurb">{{ t('home.jisudeng.pricing.blurb') }}</p>
        <ul class="pricing-tags">
          <li v-for="tag in pricingTags" :key="tag">— {{ tag }}</li>
        </ul>
        <button type="button" class="cta-text-large" @click="goStart">
          {{ t('home.jisudeng.cta.viewPrice') }}
          <span class="arrow">→</span>
        </button>
      </div>
    </section>

    <section v-if="faqItems.length" id="faq" class="faq-section section-block" :class="{ 'in-view': inView.faq }">
      <div class="page-container">
        <div class="section-head">
          <span class="section-tag">{{ t('home.jisudeng.sections.faqTag') }}</span>
          <h2 class="section-title">{{ t('home.jisudeng.sections.faqTitle') }}</h2>
        </div>
        <dl class="faq-list">
          <div v-for="(item, idx) in faqItems" :key="idx" class="faq-item">
            <dt class="faq-q">{{ item.q }}</dt>
            <dd class="faq-a">{{ item.a }}</dd>
          </div>
        </dl>
      </div>
    </section>

    <section id="closer" class="closer-section" :class="{ 'in-view': inView.closer }">
      <div class="closer-stage">
        <svg viewBox="0 0 1200 500" class="closer-map" aria-hidden="true" preserveAspectRatio="xMidYMid meet">
          <g class="closer-dots">
            <circle v-for="(d, i) in closerDots" :key="`d-${i}`" :cx="d.cx" :cy="d.cy" :r="d.r" :opacity="d.opacity" fill="#0a0a0a" />
          </g>
          <g class="closer-lines">
            <path
              v-for="(line, i) in closerLines"
              :key="`l-${i}`"
              :d="line.d"
              stroke="#0a0a0a"
              stroke-width="0.8"
              fill="none"
              stroke-dasharray="3 5"
              stroke-opacity="0.45"
              :class="`flyline flyline-${i}`"
            />
            <circle v-for="(line, i) in closerLines" :key="`s-${i}`" :cx="line.sx" :cy="line.sy" r="2.5" fill="#0a0a0a" opacity="0.55" />
            <circle v-for="(line, i) in closerLines" :key="`e-${i}`" :cx="line.ex" :cy="line.ey" r="3" fill="#0a0a0a" opacity="0.85" />
          </g>
        </svg>
        <div class="closer-overlay">
          <div class="closer-logo">
            <img :src="siteLogo || '/logo.png'" :alt="siteName" />
          </div>
          <h2 class="closer-title">{{ t('home.jisudeng.closer.title') }}</h2>
          <p class="closer-sub">{{ t('home.jisudeng.closer.sub') }}</p>
          <button type="button" class="cta-primary" @click="goStart">
            {{ isAuthenticated ? t('home.jisudeng.cta.console') : t('home.jisudeng.cta.start') }}
            <span class="arrow">→</span>
          </button>
        </div>
      </div>
    </section>

    <nav v-if="isGtmHome" class="home-anchor-nav" aria-label="Page sections">
      <a
        v-for="section in anchorSections"
        :key="section.id"
        :href="`#${section.id}`"
        class="home-anchor-link"
        :class="{ 'is-active': activeAnchor === section.id }"
        @click.prevent="scrollToSection(section.id)"
      >
        {{ section.label }}
      </a>
    </nav>

    <button
      v-if="showBackToTop"
      type="button"
      class="home-back-top"
      :aria-label="t('home.jisudeng.anchors.backToTop')"
      @click="scrollToTop"
    >
      ↑
    </button>

    <footer class="page-footer">
      <div class="page-container footer-row">
        <span class="f-brand">{{ siteName }} · {{ t('home.jisudeng.footer.tagline') }}</span>
        <LmspeedBadge />
        <span class="f-links">
          <router-link v-if="isEnglishPublicRoute" :to="{ name: PUBLIC_ROUTE_NAMES.englishDocs }">{{ t('home.jisudeng.footer.docs') }}</router-link>
          <a v-else-if="docUrl" :href="docUrl" target="_blank" rel="noopener">{{ t('home.jisudeng.footer.docs') }}</a>
          <span class="f-copy">© {{ year }} {{ siteName }}</span>
        </span>
      </div>
    </footer>

    <div v-if="!isAuthenticated && showGuestStickyCta" class="home-sticky-cta">
      <router-link :to="downloadRoute" class="home-sticky-download">
        {{ t('home.jisudeng.nav.androidApp') }}
      </router-link>
      <button type="button" class="cta-primary home-sticky-cta-btn" @click="goRegister">
        {{ t('home.jisudeng.cta.register') }}
        <span class="arrow">→</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/home-view.css'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore, useAppStore } from '@/stores'
import HeroSphere from '@/components/home/HeroSphere.vue'
import ChannelTV from '@/components/home/ChannelTV.vue'
import TerminalDemo from '@/components/home/TerminalDemo.vue'
import WhyHoverCard from '@/components/home/WhyHoverCard.vue'
import LmspeedBadge from '@/components/home/LmspeedBadge.vue'
import LmspeedProviderProof from '@/components/home/LmspeedProviderProof.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import HomeStatOdometer from '@/components/home/HomeStatOdometer.vue'
import { useHomeLiveStats } from '@/composables/useHomeLiveStats'
import { usePublicGrowthTeaser } from '@/composables/usePublicGrowthTeaser'
import { formatHomeStatsTimestamp } from '@/utils/homeLiveStats'
import { sanitizeUrl } from '@/utils/url'
import { enabledSupportContacts } from '@/utils/supportContact'
import { localizedSiteName, localizedSiteSubtitle } from '@/utils/localizedPublicSettings'
import { isHomeContentUrl as isCustomHomeContentUrl, sanitizeHomeContent } from '@/utils/homeContent'
import { recoverFromChunkLoadError } from '@/router/chunkRecovery'
import {
  PUBLIC_ROUTE_NAMES,
  authEntryRoute,
  buildHomePrimaryNav,
  dashboardEntryRoute,
  docsTopicRoute,
  englishDocsTopicRoute,
  imageStudioEntryRoute,
} from '@/router/publicNavigation'

const { t, tm, te, locale } = useI18n()
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const appStore = useAppStore()

const isIntro = ref(true)
const headerScrolled = ref(false)
const whyActive = ref<number | null>(null)
const whyX = ref(0)
const whyY = ref(0)
const onboardPhase = ref(1)
const year = new Date().getFullYear()
const {
  statItems,
  computedAt: statsComputedAt,
  opsDataThrough: statsOpsDataThrough,
  isStale: isStatsStale,
} = useHomeLiveStats()
const { perkLines } = usePublicGrowthTeaser()

const statsFreshness = computed(() => {
  const items: string[] = []
  const through = formatHomeStatsTimestamp(statsOpsDataThrough.value, locale.value)
  const computed = formatHomeStatsTimestamp(statsComputedAt.value, locale.value)
  if (through) items.push(t('home.jisudeng.stats.through', { time: through }))
  if (computed) items.push(t('home.jisudeng.stats.computed', { time: computed }))
  return items
})

const inView = ref<Record<string, boolean>>({})

let observer: IntersectionObserver | null = null
let scrollHandler: (() => void) | null = null
let whyRaf = 0
let whyTargetX = 0
let whyTargetY = 0
let fontEl: HTMLLinkElement | null = null

const siteName = computed(() =>
  localizedSiteName(appStore.cachedPublicSettings?.site_name || appStore.siteName, locale.value)
)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const isEnglishPublicRoute = computed(() => route.path === '/en' || route.path.startsWith('/en/'))
const hasSupportContact = computed(() => enabledSupportContacts(appStore.supportContact).length > 0)
const studioCtaLink = computed(() => imageStudioEntryRoute(isAuthenticated.value))
const adminDashboardRoute = { name: PUBLIC_ROUTE_NAMES.adminDashboard }
const aboutRoute = { name: PUBLIC_ROUTE_NAMES.about }
const downloadRoute = { name: PUBLIC_ROUTE_NAMES.androidDownload }
const loginRoute = authEntryRoute(false)
const registerRoute = authEntryRoute(true)
const homeLogoRoute = computed(() =>
  isEnglishPublicRoute.value ? { name: PUBLIC_ROUTE_NAMES.englishHome } : { path: '/' },
)
const imageDocsRoute = computed(() =>
  isEnglishPublicRoute.value
    ? englishDocsTopicRoute('deploy', 'text-to-image-api')
    : docsTopicRoute('deploy', 'text-to-image-api'),
)
const quickStartDocsRoute = computed(() =>
  isEnglishPublicRoute.value
    ? englishDocsTopicRoute('tutorial', 'quick-start')
    : docsTopicRoute('tutorial', 'quick-start'),
)
const dashboardRoute = computed(() => dashboardEntryRoute(isAdmin.value))
const homePrimaryNavItems = computed(() =>
  buildHomePrimaryNav(isAuthenticated.value)
    .map((item) => {
      if (!isEnglishPublicRoute.value) return item
      if (item.key === 'models') return { ...item, to: { name: PUBLIC_ROUTE_NAMES.englishModels } }
      if (item.key === 'docs') return { ...item, to: { name: PUBLIC_ROUTE_NAMES.englishDocs } }
      return item
    })
    .filter((item) => !item.requiresSupportContact || hasSupportContact.value),
)
const siteLogo = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', {
    allowRelative: true,
    allowDataUrl: true
  })
)
const siteSubtitle = computed(() => {
  return localizedSiteSubtitle(
    appStore.cachedPublicSettings?.site_subtitle,
    locale.value,
    t('authAside.siteSubtitleDefault'),
  )
})
const docUrl = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
)
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const safeHomeContent = ref('')
const isHomeContentUrl = computed(() => isCustomHomeContentUrl(homeContent.value))

const isGtmHome = computed(() => te('home.jisudeng.hero.subtitle') && te('home.jisudeng.cta.register'))
const heroSubtitle = computed(() => {
  if (isGtmHome.value) {
    return t('home.jisudeng.hero.subtitle')
  }
  return siteSubtitle.value
})
const showVerifyLink = computed(() => isGtmHome.value && te('home.jisudeng.manifesto.verifyLink'))
const showGuestStickyCta = computed(() => isGtmHome.value)

type FaqItem = { q: string; a: string }
const faqItems = computed((): FaqItem[] => {
  if (!te('home.jisudeng.faq.items')) return []
  const raw = tm('home.jisudeng.faq.items') as unknown
  if (!Array.isArray(raw)) return []
  return raw.filter((item): item is FaqItem => {
    return typeof item === 'object' && item !== null && 'q' in item && 'a' in item
  })
})

const anchorSections = computed(() => {
  const sections = [
    { id: 'manifesto', label: t('home.jisudeng.anchors.manifesto') },
    { id: 'stats', label: t('home.jisudeng.anchors.stats') },
    { id: 'lmspeed', label: t('home.jisudeng.anchors.lmspeed') },
    { id: 'image', label: t('home.jisudeng.anchors.image') },
    { id: 'channels', label: t('home.jisudeng.anchors.channels') },
    { id: 'features', label: t('home.jisudeng.anchors.features') },
    { id: 'onboard', label: t('home.jisudeng.anchors.onboard') },
    { id: 'pricing', label: t('home.jisudeng.anchors.pricing') },
    { id: 'faq', label: t('home.jisudeng.anchors.faq') },
    { id: 'closer', label: t('home.jisudeng.anchors.closer') },
  ]
  if (faqItems.value.length === 0) {
    return sections.filter((section) => section.id !== 'faq')
  }
  return sections
})

const showBackToTop = ref(false)
const activeAnchor = ref('manifesto')

let sanitizeVersion = 0
watch(
  homeContent,
  async (content) => {
    const version = ++sanitizeVersion

    if (!content.trim() || isCustomHomeContentUrl(content)) {
      safeHomeContent.value = ''
      return
    }

    try {
      const sanitized = await sanitizeHomeContent(content)
      if (version === sanitizeVersion) {
        safeHomeContent.value = sanitized
      }
    } catch (error) {
      if (recoverFromChunkLoadError(error, route.fullPath)) return
      console.error('Failed to sanitize custom home content:', error)
      if (version === sanitizeVersion) {
        safeHomeContent.value = ''
      }
    }
  },
  { immediate: true }
)

const eyebrowBits = computed(() =>
  t('home.jisudeng.hero.eyebrow')
    .split(/\s*·\s*/)
    .filter(Boolean)
    .map((part) => {
      const m = part.match(/^(\u62d2\u7edd|NO\s|REFUSE\s)(.+)$/i)
      return m ? { pre: m[1], obj: m[2] } : { text: part }
    })
)

const manifestoParts = computed(() => {
  const title = t('home.jisudeng.manifesto.title')
  const m = title.match(/Tokens?/i)
  if (!m || m.index === undefined) return { before: title, keyword: '', after: '' }
  return {
    before: title.slice(0, m.index),
    keyword: m[0],
    after: title.slice(m.index + m[0].length)
  }
})

const imageEndpoints = [
  { method: 'POST', path: '/v1/images/generations' },
  { method: 'POST', path: '/v1/images/edits' }
]

const imageCaps = computed(() => [
  t('home.jisudeng.image.caps.0'),
  t('home.jisudeng.image.caps.1'),
  t('home.jisudeng.image.caps.2'),
  t('home.jisudeng.image.caps.3')
])

const featureRows = computed(() => [
  { idx: '01', en: 'Multi-Model', title: t('home.jisudeng.features.multiModel.title'), desc: t('home.jisudeng.features.multiModel.desc') },
  { idx: '02', en: 'Reliability', title: t('home.jisudeng.features.stable.title'), desc: t('home.jisudeng.features.stable.desc') },
  { idx: '03', en: 'Privacy', title: t('home.jisudeng.features.privacy.title'), desc: t('home.jisudeng.features.privacy.desc') },
  { idx: '04', en: 'Instant Access', title: t('home.jisudeng.features.instant.title'), desc: t('home.jisudeng.features.instant.desc') },
  { idx: '05', en: 'Fair Billing', title: t('home.jisudeng.features.transparent.title'), desc: t('home.jisudeng.features.transparent.desc') },
  { idx: '06', en: 'Self-Service', title: t('home.jisudeng.features.selfService.title'), desc: t('home.jisudeng.features.selfService.desc') }
])

const onboardSteps = computed(() => [
  { t: t('home.jisudeng.onboard.s1t'), d: t('home.jisudeng.onboard.s1d') },
  { t: t('home.jisudeng.onboard.s2t'), d: t('home.jisudeng.onboard.s2d') },
  { t: t('home.jisudeng.onboard.s3t'), d: t('home.jisudeng.onboard.s3d') }
])

const pricingTags = computed(() => [
  t('home.jisudeng.pricing.tags.0'),
  t('home.jisudeng.pricing.tags.1'),
  t('home.jisudeng.pricing.tags.2')
])

const closerLines = [
  { d: 'M 160,360 C 320,180 460,160 580,240', sx: 160, sy: 360, ex: 580, ey: 240 },
  { d: 'M 1040,380 C 880,260 760,200 620,240', sx: 1040, sy: 380, ex: 620, ey: 240 },
  { d: 'M 240,140 C 380,200 480,230 580,250', sx: 240, sy: 140, ex: 580, ey: 250 },
  { d: 'M 970,130 C 830,190 720,220 620,260', sx: 970, sy: 130, ex: 620, ey: 260 }
]

const closerDots = (() => {
  const dots: Array<{ cx: number; cy: number; r: number; opacity: number }> = []
  for (let x = 0; x <= 1200; x += 14) {
    for (let y = 0; y <= 500; y += 14) {
      const nx = (x - 600) / 540
      const ny = (y - 250) / 220
      const n = nx * nx + ny * ny
      if (n > 1) continue
      const d = (x - 600) / 280
      const u = (y - 250) / 110
      if (d * d + u * u < 1) continue
      const p = Math.max(0, 1 - n * 0.95)
      if (Math.random() > 0.42 + p * 0.45) continue
      dots.push({ cx: x, cy: y, r: 0.7 + Math.random() * 0.5, opacity: 0.16 + p * 0.55 })
    }
  }
  return dots
})()

function onReveal() {
  isIntro.value = false
}

function goRegister() {
  router.push(registerRoute)
}

function goStart() {
  if (isAuthenticated.value) {
    router.push(dashboardRoute.value)
    return
  }
  router.push(authEntryRoute(isGtmHome.value))
}

function openDocs() {
  if (isEnglishPublicRoute.value) {
    router.push({ name: PUBLIC_ROUTE_NAMES.englishDocs })
    return
  }
  if (docUrl.value) window.open(docUrl.value, '_blank', 'noopener')
}

function scrollToSection(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function updateScrollState() {
  headerScrolled.value = window.scrollY > 12
  showBackToTop.value = window.scrollY > 480
  let current = anchorSections.value[0]?.id ?? 'manifesto'
  for (const section of anchorSections.value) {
    const el = document.getElementById(section.id)
    if (el && el.getBoundingClientRect().top <= 120) {
      current = section.id
    }
  }
  activeAnchor.value = current
}

function cardPos(ev: MouseEvent) {
  return {
    x: Math.min(ev.clientX + 30, window.innerWidth - 400),
    y: Math.min(Math.max(ev.clientY - 124, 12), window.innerHeight - 296)
  }
}

function whyLoop() {
  whyX.value += (whyTargetX - whyX.value) * 0.16
  whyY.value += (whyTargetY - whyY.value) * 0.16
  if (whyActive.value !== null) whyRaf = requestAnimationFrame(whyLoop)
}

function onWhyEnter(idx: number, ev: MouseEvent) {
  if (!window.matchMedia('(hover: hover)').matches || window.innerWidth < 1024) return
  const pos = cardPos(ev)
  whyTargetX = pos.x
  whyTargetY = pos.y
  if (whyActive.value === null) {
    whyX.value = pos.x
    whyY.value = pos.y
    whyRaf = requestAnimationFrame(whyLoop)
  }
  whyActive.value = idx
}

function onWhyMove(ev: MouseEvent) {
  if (whyActive.value === null) return
  const pos = cardPos(ev)
  whyTargetX = pos.x
  whyTargetY = pos.y
}

function onWhyLeave() {
  whyActive.value = null
  cancelAnimationFrame(whyRaf)
}

function ensureFonts() {
  if (document.querySelector('link[data-jisudeng-fonts]')) return
  fontEl = document.createElement('link')
  fontEl.rel = 'stylesheet'
  fontEl.setAttribute('data-jisudeng-fonts', 'true')
  fontEl.href =
    'https://fonts.googleapis.com/css2?family=Fraunces:ital,opsz,wght@0,9..144,400;0,9..144,500;0,9..144,700;1,9..144,400&family=JetBrains+Mono:wght@600;700;800&family=Noto+Serif+SC:wght@500;700;900&family=Noto+Sans+SC:wght@300;400;500;600;700&family=IBM+Plex+Mono:wght@400&display=swap'
  document.head.appendChild(fontEl)
}

onMounted(() => {
  window.scrollTo(0, 0)
  ensureFonts()

  if (!appStore.publicSettingsLoaded) {
    void appStore.fetchPublicSettings()
  }

  scrollHandler = () => {
    updateScrollState()
  }
  window.addEventListener('scroll', scrollHandler, { passive: true })
  updateScrollState()

  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        const id = (entry.target as HTMLElement).id
        if (!id) continue
        if (entry.isIntersecting) {
          entry.target.classList.add('in-view')
          inView.value = { ...inView.value, [id]: true }
          observer?.unobserve(entry.target)
        }
      }
    },
    { rootMargin: '0px 0px -80px 0px', threshold: 0.08 }
  )
  document.querySelectorAll('.home-page section:not(.hero-section)').forEach((el) => observer?.observe(el))
})

onBeforeUnmount(() => {
  observer?.disconnect()
  if (scrollHandler) window.removeEventListener('scroll', scrollHandler)
  cancelAnimationFrame(whyRaf)
})
</script>

<style scoped>
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.hero-title--en .hero-zh {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  justify-items: center;
  gap: 6px;
  width: min(1040px, calc(100vw - 48px));
  white-space: normal;
}

.hero-title--en .hz-brand,
.hero-title--en .hz-tail {
  font-size: 88px;
  letter-spacing: 0;
  line-height: 0.96;
}

.hero-title--en .hz-mid {
  font-size: 96px;
  letter-spacing: 0;
  line-height: 0.96;
}

@media (width >= 1600px) {
  .hero-title--en .hz-brand,
  .hero-title--en .hz-tail {
    font-size: 104px;
  }

  .hero-title--en .hz-mid {
    font-size: 112px;
  }
}

@media (width <= 767px) {
  .hero-title--en .hero-zh {
    gap: 4px;
    width: calc(100vw - 32px);
  }

  .hero-title--en .hz-brand,
  .hero-title--en .hz-tail {
    font-size: 42px;
  }

  .hero-title--en .hz-mid {
    font-size: 48px;
  }
}
</style>
