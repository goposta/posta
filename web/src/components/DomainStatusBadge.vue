<script setup lang="ts">
import { computed } from 'vue'
import type { Domain } from '../api/types'

const props = defineProps<{
  domain: Domain
  size?: 'sm' | 'md'
}>()

const checks = computed(() => [
  props.domain.ownership_verified,
  props.domain.spf_verified,
  props.domain.dkim_verified,
  props.domain.dmarc_verified,
])

const verifiedCount = computed(() => checks.value.filter(Boolean).length)
const total = 4

const status = computed(() => {
  if (verifiedCount.value === total) return 'verified'
  if (verifiedCount.value === 0) return 'unverified'
  return 'partial'
})

const label = computed(() => {
  if (status.value === 'verified') return 'Fully verified'
  if (status.value === 'unverified') return 'Not verified'
  return `${verifiedCount.value} of ${total} verified`
})

const icon = computed(() => {
  if (status.value === 'verified') return '✓'
  if (status.value === 'unverified') return '○'
  return '◐'
})
</script>

<template>
  <span
    class="status-pill"
    :class="[`status-pill-${status}`, size === 'sm' ? 'status-pill-sm' : '']"
    :title="`Ownership: ${domain.ownership_verified ? 'yes' : 'no'} · SPF: ${domain.spf_verified ? 'yes' : 'no'} · DKIM: ${domain.dkim_verified ? 'yes' : 'no'} · DMARC: ${domain.dmarc_verified ? 'yes' : 'no'}`"
  >
    <span class="status-pill-icon">{{ icon }}</span>
    <span>{{ label }}</span>
  </span>
</template>
