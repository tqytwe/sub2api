<template>
  <div class="public-page-toolbar">
    <LocaleSwitcher variant="public" />
    <ThemeSwitcher variant="public" />
  </div>
</template>

<script setup lang="ts">
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import ThemeSwitcher from '@/components/common/ThemeSwitcher.vue'
</script>
