<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { jisudengPagesZh } from '@/i18n/locales/jisudeng-pages.zh'
import SupportFloatingCard from '@/components/common/SupportFloatingCard.vue'
import PublicPageToolbar from '@/components/common/PublicPageToolbar.vue'
import '@/styles/about-view.css'
import '@/styles/public-pages.css'

const { t } = useI18n()
const s5items = jisudengPagesZh.about.s5items
</script>

<template>
  <div class="about-page">
    <header class="about-header public-page-header">
      <router-link to="/home" class="back-link">{{ t('about.backHome') }}</router-link>
      <PublicPageToolbar />
    </header>
    <div class="about-header-intro">
      <p class="about-eyebrow">{{ t('about.eyebrow') }}</p>
      <h1 class="about-title">{{ t('about.title') }}</h1>
      <p class="about-lede">{{ t('about.lede') }}</p>
    </div>

    <main class="about-main">
      <section class="about-section">
        <p class="about-section-num">01</p>
        <h2 class="about-section-title">{{ t('about.s1Title') }}</h2>
        <p>{{ t('about.s1p1') }}</p>
        <p>{{ t('about.s1p2') }}</p>
        <p class="about-verify">{{ t('about.s1verify') }}</p>
      </section>

      <section class="about-section">
        <p class="about-section-num">02</p>
        <h2 class="about-section-title">{{ t('about.s2Title') }}</h2>
        <p>
          每个 API Key 绑定一个<strong>服务分组</strong>，分组下面挂的是哪些上游通道、用的什么计费模式、哪个模型走哪条线，全部在
          <code><router-link to="/available-channels">/available-channels</router-link></code>
          透明列出。
        </p>
        <p>{{ t('about.s2p2') }}</p>
        <p>{{ t('about.s2p3') }}</p>
      </section>

      <section class="about-section about-section--accent">
        <p class="about-section-num">03</p>
        <h2 class="about-section-title">{{ t('about.s3Title') }}</h2>
        <p>{{ t('about.s3intro') }}</p>

        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti1Title') }}</h3>
          <p>{{ t('about.anti1p1') }}</p>
          <p>{{ t('about.anti1p2') }}</p>
        </div>
        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti2Title') }}</h3>
          <p>{{ t('about.anti2p1') }}</p>
        </div>
        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti3Title') }}</h3>
          <p>{{ t('about.anti3p1') }}</p>
        </div>
        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti4Title') }}</h3>
          <p>{{ t('about.anti4p1') }}</p>
          <p>{{ t('about.anti4p2') }}</p>
        </div>
        <div class="about-anti">
          <h3 class="about-anti-title">{{ t('about.anti5Title') }}</h3>
          <p>{{ t('about.anti5p1') }}</p>
        </div>
      </section>

      <section class="about-section">
        <p class="about-section-num">04</p>
        <h2 class="about-section-title">{{ t('about.s4Title') }}</h2>
        <p>{{ t('about.s4p1') }}</p>
      </section>

      <section class="about-section">
        <p class="about-section-num">05</p>
        <h2 class="about-section-title">{{ t('about.s5Title') }}</h2>
        <ul class="about-list">
          <li v-for="(item, idx) in s5items" :key="idx">
            {{ item }}
          </li>
        </ul>
      </section>

      <section class="about-cta">
        <p>{{ t('about.ctaLead') }}</p>
        <div class="about-cta-row">
          <router-link to="/docs" class="about-cta-link">{{ t('about.ctaDocs') }}</router-link>
          <router-link to="/blindbox" class="about-cta-link">{{ t('about.ctaVip') }}</router-link>
          <router-link to="/contact" class="about-cta-link about-cta-link--ghost">{{ t('about.ctaContact') }}</router-link>
        </div>
      </section>
    </main>

    <SupportFloatingCard />
  </div>
</template>
