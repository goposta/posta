import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'

export type ThemeMode = 'light' | 'dark' | 'system'

export const useThemeStore = defineStore('theme', () => {
  const stored = localStorage.getItem('posta_theme') as ThemeMode | null
  const mode = ref<ThemeMode>(stored && ['light', 'dark', 'system'].includes(stored) ? stored as ThemeMode : 'system')

  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')

  const isDark = computed(() => {
    if (mode.value === 'system') return mediaQuery.matches
    return mode.value === 'dark'
  })

  function apply() {
    document.documentElement.setAttribute('data-theme', isDark.value ? 'dark' : 'light')
  }

  function setMode(m: ThemeMode) {
    mode.value = m
  }

  function toggle() {
    mode.value = isDark.value ? 'light' : 'dark'
  }

  watch(mode, (val) => {
    localStorage.setItem('posta_theme', val)
    apply()
  }, { immediate: true })

  mediaQuery.addEventListener('change', () => {
    if (mode.value === 'system') apply()
  })

  return { mode, isDark, toggle, setMode }
})
