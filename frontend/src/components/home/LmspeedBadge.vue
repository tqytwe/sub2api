<template>
  <a
    class="lmspeed-badge-link"
    href="https://lmspeed.net/"
    target="_blank"
    rel="noopener noreferrer nofollow"
    :aria-label="t('home.jisudeng.footer.lmspeedBadgeAlt')"
  >
    <img
      class="lmspeed-badge-image"
      src="https://lmspeed.net/api/provider/claim-badge/2039?claim=2039-1kHJJSboOUMX0Au9G01tAaBWozH20jbC"
      :alt="t('home.jisudeng.footer.lmspeedBadgeAlt')"
      width="190"
      height="64"
      decoding="async"
      loading="lazy"
      referrerpolicy="no-referrer"
      @error="hideBrokenImage"
    />
    <span v-if="badgeFailed" class="lmspeed-badge-fallback">{{ t('home.jisudeng.footer.lmspeedBadgeAlt') }}</span>
  </a>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const badgeFailed = ref(false)

function hideBrokenImage(event: Event) {
  badgeFailed.value = true
  const image = event.target
  if (image instanceof HTMLImageElement) image.hidden = true
}
</script>

<style scoped>
.lmspeed-badge-link {
  display: inline-flex;
  flex: 0 0 auto;
  max-width: 100%;
  border-radius: 10px;
}

.lmspeed-badge-link:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: 4px;
}

.lmspeed-badge-image {
  display: block;
  width: 190px;
  max-width: 100%;
  height: auto;
}

.lmspeed-badge-fallback {
  display: inline-flex;
  min-height: 32px;
  align-items: center;
  padding: 6px 10px;
  border: 1px solid currentColor;
  border-radius: 8px;
  font-size: 11px;
}
</style>
