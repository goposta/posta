<script setup lang="ts">
import { useRoute } from 'vue-router'

defineProps<{
  title: string
  tabs: { label: string; to: string }[]
}>()

const route = useRoute()

function isActive(to: string): boolean {
  return route.path === to || route.path.startsWith(to + '/')
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>{{ title }}</h1>
    </div>
    <div class="tabs">
      <router-link v-for="t in tabs" :key="t.to" :to="t.to" class="tab" :class="{ active: isActive(t.to) }">
        {{ t.label }}
      </router-link>
    </div>
  </div>
</template>

<style scoped>
.tab {
  text-decoration: none;
}
</style>
