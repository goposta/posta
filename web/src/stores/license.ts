import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { licenseApi, type Entitlements } from '../api/license'

// Whether Posta Enterprise features are on. Enterprise entry points stay
// visible without a license: they show a lock and say what the feature does.
// The server enforces the license; this only decides what the page shows.
export const useLicenseStore = defineStore('license', () => {
  const entitlements = ref<Entitlements | null>(null)
  /** The first answer, or failure, is in: until then nothing is shown locked. */
  const ready = ref(false)
  let inflight: Promise<void> | null = null

  const licensed = computed(() => entitlements.value?.licensed ?? false)
  const mutable = computed(() => entitlements.value?.mutable ?? false)
  /** Licensed but expired past grace: Enterprise settings are read-only. */
  const readOnly = computed(() => licensed.value && !mutable.value)
  const state = computed(() => entitlements.value?.state ?? 'none')

  function load(force = false): Promise<void> {
    if (entitlements.value && !force) return Promise.resolve()
    if (inflight) return inflight
    inflight = licenseApi
      .entitlements()
      .then((res) => {
        entitlements.value = res.data.data
      })
      .catch(() => {
        // Unknown: show Enterprise features as locked. The server decides anyway.
      })
      .finally(() => {
        ready.value = true
        inflight = null
      })
    return inflight
  }

  function set(next: Entitlements) {
    entitlements.value = next
    ready.value = true
  }

  return { entitlements, ready, licensed, mutable, readOnly, state, load, set }
})
