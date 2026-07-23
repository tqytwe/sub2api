<template>
  <div class="lmspeed-proof">
    <div class="section-head">
      <span class="section-tag">{{ t('home.jisudeng.lmspeedProof.tag') }}</span>
      <h2 class="section-title">{{ t('home.jisudeng.lmspeedProof.title') }}</h2>
      <p class="section-lede">{{ t('home.jisudeng.lmspeedProof.lede') }}</p>
    </div>

    <div class="lmspeed-proof-grid" :aria-label="t('home.jisudeng.lmspeedProof.gridLabel')">
      <a
        v-for="item in proofItems"
        :key="item.type"
        class="lmspeed-proof-card"
        :class="`lmspeed-proof-card--${item.type}`"
        data-testid="lmspeed-provider-proof-card"
        :href="providerUrl"
        target="_blank"
        rel="noopener noreferrer nofollow"
        referrerpolicy="no-referrer"
        :aria-label="item.alt"
      >
        <span class="lmspeed-proof-card-title">{{ item.label }}</span>
        <img
          class="lmspeed-proof-image"
          :src="item.src"
          :alt="item.alt"
          :width="item.width"
          :height="item.height"
          loading="lazy"
          decoding="async"
          referrerpolicy="no-referrer"
        />
      </a>
    </div>

    <a
      class="lmspeed-proof-link"
      :href="providerUrl"
      target="_blank"
      rel="noopener noreferrer nofollow"
      referrerpolicy="no-referrer"
    >
      {{ t('home.jisudeng.lmspeedProof.providerLink') }}
      <span aria-hidden="true">↗</span>
    </a>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const PROVIDER_ID = 'jisudeng'
const LMSPEED_ORIGIN = 'https://lmspeed.net'

const { t, locale } = useI18n()

const providerUrl = computed(() => {
  const pathLocale = String(locale.value).startsWith('en') ? 'en' : 'zh'
  return `${LMSPEED_ORIGIN}/${pathLocale}/provider/${PROVIDER_ID}`
})

const proofItems = computed(() => [
  {
    type: 'health',
    src: `${LMSPEED_ORIGIN}/api/og/provider/${PROVIDER_ID}/health`,
    width: 1200,
    height: 424,
    label: t('home.jisudeng.lmspeedProof.items.health.label'),
    alt: t('home.jisudeng.lmspeedProof.items.health.alt'),
  },
  {
    type: 'models',
    src: `${LMSPEED_ORIGIN}/api/og/provider/${PROVIDER_ID}/models`,
    width: 1200,
    height: 192,
    label: t('home.jisudeng.lmspeedProof.items.models.label'),
    alt: t('home.jisudeng.lmspeedProof.items.models.alt'),
  },
  {
    type: 'recent',
    src: `${LMSPEED_ORIGIN}/api/og/provider/${PROVIDER_ID}/recent`,
    width: 1200,
    height: 192,
    label: t('home.jisudeng.lmspeedProof.items.recent.label'),
    alt: t('home.jisudeng.lmspeedProof.items.recent.alt'),
  },
])
</script>

<style scoped>
.lmspeed-proof {
  display: grid;
  gap: 24px;
}

.lmspeed-proof-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.12fr) minmax(280px, 0.88fr);
  gap: 16px;
  align-items: stretch;
}

.lmspeed-proof-card {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--card);
  color: var(--ink);
  text-decoration: none;
  transition: border-color 0.2s, box-shadow 0.2s, transform 0.2s;
}

.lmspeed-proof-card--health {
  grid-row: span 2;
}

.lmspeed-proof-card:focus-visible,
.lmspeed-proof-link:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: 4px;
}

.lmspeed-proof-card-title {
  font-family: IBM Plex Mono, ui-monospace, monospace;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--ink-2);
}

.lmspeed-proof-image {
  display: block;
  width: 100%;
  height: auto;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--bg-soft);
}

.lmspeed-proof-link {
  justify-self: start;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--ink);
  font-family: IBM Plex Mono, ui-monospace, monospace;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  border-bottom: 1px solid var(--line);
  transition: border-color 0.2s, gap 0.2s;
}

@media (hover: hover) {
  .lmspeed-proof-card:hover {
    border-color: var(--ink);
    box-shadow: 0 18px 38px color-mix(in srgb, var(--ink) 10%, transparent);
    transform: translateY(-2px);
  }

  .lmspeed-proof-link:hover {
    gap: 12px;
    border-color: var(--ink);
  }
}

@media (max-width: 900px) {
  .lmspeed-proof-grid {
    grid-template-columns: 1fr;
  }

  .lmspeed-proof-card--health {
    grid-row: auto;
  }
}

@media (max-width: 560px) {
  .lmspeed-proof {
    gap: 18px;
  }

  .lmspeed-proof-card {
    gap: 10px;
    padding: 10px;
  }
}
</style>
