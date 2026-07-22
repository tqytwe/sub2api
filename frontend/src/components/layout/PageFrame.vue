<script setup lang="ts">
const props = withDefaults(defineProps<{
  frame?: 'compact' | 'reading' | 'form' | 'content' | 'workspace' | 'fluid'
}>(), {
  frame: 'content',
})
</script>

<template>
  <div class="page-frame" :class="`page-frame--${props.frame}`">
    <slot />
  </div>
</template>

<style scoped>
.page-frame {
  width: 100%;
}

.page-frame--compact,
.page-frame--reading,
.page-frame--form {
  margin-inline: auto;
}

.page-frame--compact {
  max-width: 42rem;
}

.page-frame--reading {
  max-width: 50rem;
}

.page-frame--form {
  max-width: 60rem;
}

.page-frame--content {
  max-width: none;
}

.page-frame--workspace {
  max-width: none;
}

.page-frame--fluid {
  max-width: none;
}
</style>
