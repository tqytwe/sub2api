import { computed, ref } from 'vue'

export type ThemePreference = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'theme'

const preference = ref<ThemePreference>(readStoredPreference())
let mediaBound = false

function readStoredPreference(): ThemePreference {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'light' || saved === 'dark' || saved === 'system') {
    return saved
  }
  // Match public marketing pages: light by default unless user picks otherwise.
  return 'light'
}

function systemPrefersDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function resolveIsDark(pref: ThemePreference = preference.value): boolean {
  if (pref === 'dark') return true
  if (pref === 'light') return false
  return systemPrefersDark()
}

function applyThemeClass() {
  const dark = resolveIsDark()
  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
}

function bindSystemPreferenceListener() {
  if (mediaBound || typeof window === 'undefined') return
  mediaBound = true
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (preference.value === 'system') {
      applyThemeClass()
    }
  })
}

/** Call once before app mount (see main.ts). */
export function initTheme() {
  preference.value = readStoredPreference()
  applyThemeClass()
  bindSystemPreferenceListener()
}

export function useTheme() {
  const isDark = computed(() => resolveIsDark())

  function setPreference(next: ThemePreference) {
    preference.value = next
    localStorage.setItem(STORAGE_KEY, next)
    applyThemeClass()
  }

  return {
    preference,
    isDark,
    setPreference,
    applyThemeClass,
  }
}
