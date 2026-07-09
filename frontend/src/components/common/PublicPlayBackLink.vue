<script setup lang="ts">
import { usePlayPageBackNav } from '@/composables/usePlayPageBackNav'

const { backTarget, backLabel } = usePlayPageBackNav()
</script>

<template>
  <router-link :to="backTarget" class="back-link">{{ backLabel }}</router-link>
</template>
