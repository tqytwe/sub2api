<template>
  <div class="relative" ref="dropdownRef">
    <button
      type="button"
      @click="toggleDropdown"
      :class="buttonClass"
      :title="currentOption?.label"
    >
      <SunIcon v-if="preference === 'light'" class="h-5 w-5 flex-shrink-0" :class="{ 'text-amber-500': variant === 'sidebar' }" />
      <MoonIcon v-else-if="preference === 'dark'" class="h-5 w-5 flex-shrink-0" />
      <ComputerDesktopIcon v-else class="h-5 w-5 flex-shrink-0" />
      <span
        v-if="variant === 'sidebar'"
        class="sidebar-label"
        :class="{ 'sidebar-label-collapsed': sidebarCollapsed }"
        :aria-hidden="sidebarCollapsed ? 'true' : 'false'"
      >
        {{ currentOption?.label }}
      </span>
      <span v-else-if="showLabel" class="hidden sm:inline">{{ currentOption?.shortLabel }}</span>
      <Icon
        v-if="variant !== 'sidebar'"
        name="chevronDown"
        size="xs"
        class="text-gray-400 transition-transform duration-200 dark:text-gray-500"
        :class="{ 'rotate-180': isOpen }"
      />
    </button>

    <transition name="dropdown">
      <div v-if="isOpen" :class="menuClass">
        <button
          v-for="option in options"
          :key="option.value"
          type="button"
          @click="selectPreference(option.value)"
          :class="itemClass(option.value)"
        >
          <component :is="option.icon" class="h-4 w-4 flex-shrink-0" />
          <span>{{ option.label }}</span>
          <Icon
            v-if="preference === option.value"
            name="check"
            size="sm"
            class="ml-auto text-primary-500"
          />
        </button>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useTheme, type ThemePreference } from '@/composables/useTheme'
import { useAppStore } from '@/stores/app'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'public' | 'sidebar'
    showLabel?: boolean
  }>(),
  {
    variant: 'default',
    showLabel: true,
  },
)

const { t } = useI18n()
const { preference, setPreference } = useTheme()
const appStore = useAppStore()

const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)

const isOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

const SunIcon = () =>
  h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', strokeWidth: 1.5 }, [
    h('path', {
      'stroke-linecap': 'round',
      'stroke-linejoin': 'round',
      d: 'M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z',
    }),
  ])

const MoonIcon = () =>
  h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', strokeWidth: 1.5 }, [
    h('path', {
      'stroke-linecap': 'round',
      'stroke-linejoin': 'round',
      d: 'M21.752 15.002A9.718 9.718 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z',
    }),
  ])

const ComputerDesktopIcon = () =>
  h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', strokeWidth: 1.5 }, [
    h('path', {
      'stroke-linecap': 'round',
      'stroke-linejoin': 'round',
      d: 'M9 17.25v1.007a3 3 0 0 1-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0 1 15 18.257V17.25m6-12V15a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 15V5.25m18 0A2.25 2.25 0 0 0 18.75 3H5.25A2.25 2.25 0 0 0 3 5.25m18 0V12a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 12V5.25',
    }),
  ])

const options = computed(() => [
  {
    value: 'light' as ThemePreference,
    label: t('nav.lightMode'),
    shortLabel: t('nav.themeLightShort'),
    icon: SunIcon,
  },
  {
    value: 'dark' as ThemePreference,
    label: t('nav.darkMode'),
    shortLabel: t('nav.themeDarkShort'),
    icon: MoonIcon,
  },
  {
    value: 'system' as ThemePreference,
    label: t('nav.systemMode'),
    shortLabel: t('nav.themeSystemShort'),
    icon: ComputerDesktopIcon,
  },
])

const currentOption = computed(() => options.value.find((o) => o.value === preference.value))

const buttonClass = computed(() => {
  if (props.variant === 'sidebar') {
    return [
      'sidebar-link w-full',
      sidebarCollapsed.value ? 'sidebar-link-collapsed' : '',
    ]
  }
  if (props.variant === 'public') {
    return 'public-toolbar-btn'
  }
  return 'flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'
})

const menuClass = computed(() => {
  const base =
    'absolute right-0 z-50 mt-1 overflow-hidden rounded-lg border shadow-lg'
  if (props.variant === 'public') {
    return `${base} public-theme-menu`
  }
  return `${base} w-40 border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800`
})

function itemClass(value: ThemePreference) {
  const active = preference.value === value
  if (props.variant === 'public') {
    return ['public-theme-item', active ? 'is-active' : '']
  }
  return [
    'flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700',
    active ? 'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-400' : '',
  ]
}

function toggleDropdown() {
  isOpen.value = !isOpen.value
}

function selectPreference(value: ThemePreference) {
  setPreference(value)
  isOpen.value = false
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
</style>
