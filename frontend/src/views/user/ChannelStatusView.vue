<template>
  <ChannelStatusV3View v-if="isV3" />
  <ChannelStatusV1View v-else-if="isV1" />
  <ChannelStatusV2View v-else />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { isChannelMonitorV1Mode, isChannelMonitorV3Mode } from '@/utils/featureFlags'
import ChannelStatusV1View from './ChannelStatusV1View.vue'
import ChannelStatusV2View from './ChannelStatusV2View.vue'
import ChannelStatusV3View from './ChannelStatusV3View.vue'

const isV1 = computed(() => isChannelMonitorV1Mode())
const isV3 = computed(() => isChannelMonitorV3Mode())
</script>
