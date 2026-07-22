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
.page-frame--form,
.page-frame--content,
.page-frame--workspace {
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
  max-width: 72rem;
}

.page-frame--workspace {
  max-width: 100rem;
}

.page-frame--fluid {
  max-width: none;
}
</style>
