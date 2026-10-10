<template><main class="p-4 md:p-6 lg:p-8"><PageFrame frame="content"><slot /></PageFrame></main></template>
<script setup lang="ts">
// Keep the production view, table, dialog, controls, styles and API client.
// Only the unrelated global navigation shell is excluded from this fixture.
import PageFrame from '../../src/components/layout/PageFrame.vue'
</script>
