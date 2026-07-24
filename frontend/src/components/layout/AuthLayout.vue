<!-- jisudeng-brand: 极速蹬定制登录/注册布局，合并 upstream 时禁止覆盖为默认 AuthLayout -->
<template>
  <div class="auth-page">
    <header class="auth-header">
      <div class="auth-header-row">
        <router-link to="/home" class="auth-brand">
          <span class="brand-mark">
            <img :src="siteLogo || '/logo.png'" :alt="siteName" />
          </span>
          <span class="brand-name">{{ siteName }}</span>
        </router-link>
        <nav class="auth-nav">
          <PublicPageToolbar />
          <router-link to="/home" class="nav-link">{{ t('authAside.backHome') }}</router-link>
        </nav>
      </div>
    </header>

    <main class="auth-main">
      <div class="auth-grid">
        <aside v-if="asideMode !== 'none'" class="auth-aside">
          <p class="aside-eyebrow">
            {{ asideMode === 'register' ? t('authAside.eyebrowRegister') : t('authAside.eyebrow') }}
          </p>
          <h1 class="aside-title">
            <span>{{ asideMode === 'register' ? t('authAside.titleRegister') : t('authAside.titleLogin') }}</span>
            <span class="aside-title-faint">
              {{ asideMode === 'register' ? t('authAside.titleFaintRegister') : t('authAside.titleFaintLogin') }}
            </span>
          </h1>
          <p v-if="asideMode === 'register'" class="aside-pitch">{{ t('authAside.pitchRegister') }}</p>

          <ul class="aside-pledges">
            <li v-for="(pledge, idx) in pledges" :key="idx">
              <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.5">
                <path d="M3 8.5l3 3 7-7" />
              </svg>
              <span>{{ pledge }}</span>
            </li>
          </ul>

          <p v-if="asideMode === 'register'" class="aside-fineprint">{{ t('authAside.fineprintRegister') }}</p>

          <SupportContactPanel
            class="aside-support"
            :config="supportContactConfig"
            compact
          />
        </aside>

        <div class="auth-card-wrap">
          <div class="auth-card">
            <div v-if="settingsLoaded" class="auth-brand-block">
              <div class="brand-mark-card">
                <img :src="siteLogo || '/logo.png'" :alt="siteName" />
              </div>
              <h2 class="auth-card-title">{{ siteName }}</h2>
              <p class="auth-card-sub">{{ siteSubtitle }}</p>
            </div>
            <slot />
            <div v-if="$slots.footer" class="auth-card-footer">
              <slot name="footer" />
            </div>
          </div>
          <p class="auth-copyright">
            &copy; {{ currentYear }} {{ siteName }} · {{ t('authAside.copyrightTagline') }}
          </p>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import {
  localizedSiteName,
  localizedSiteSubtitle,
  localizedSupportContactConfig,
} from '@/utils/localizedPublicSettings'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import SupportContactPanel from '@/components/common/SupportContactPanel.vue'
import '@/styles/auth-layout-jisudeng.css'
import '@/styles/public-pages.css'

const props = withDefaults(
  defineProps<{
    asideMode?: 'login' | 'register' | 'none'
  }>(),
  { asideMode: 'login' }
)

const { t, locale } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => localizedSiteName(appStore.siteName, locale.value))
const siteLogo = computed(() =>
  sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
)

const siteSubtitle = computed(() => {
  return localizedSiteSubtitle(
    appStore.cachedPublicSettings?.site_subtitle,
    locale.value,
    t('authAside.siteSubtitleDefault'),
  )
})
const supportContactConfig = computed(() =>
  localizedSupportContactConfig(appStore.supportContact, locale.value)
)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = computed(() => new Date().getFullYear())

const pledges = computed(() => {
  if (props.asideMode === 'register') {
    return [
      t('authAside.regPledge1'),
      t('authAside.regPledge2'),
      t('authAside.regPledge3'),
      t('authAside.regPledge4')
    ]
  }
  return [t('authAside.pledge1'), t('authAside.pledge2'), t('authAside.pledge3')]
})

onMounted(() => {
  void appStore.fetchPublicSettings()
})
</script>
