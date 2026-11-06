<script setup lang="ts">
import { onMounted } from 'vue'
import { ENTERPRISE_URL } from '../api/license'
import { useAuthStore } from '../stores/auth'
import { useLicenseStore } from '../stores/license'

defineProps<{
  feature: string
  description: string
}>()

const license = useLicenseStore()
const auth = useAuthStore()

onMounted(() => license.load())
</script>

<template>
  <template v-if="license.ready">
    <template v-if="license.licensed">
      <div v-if="license.readOnly" class="app-banner app-banner--warning ee-readonly" role="status">
        <div class="app-banner-content">
          <p class="app-banner-title">{{ feature }} is read-only</p>
          <p class="app-banner-text">
            The Posta Enterprise license has expired. {{ feature }} keeps working, but can't be changed until
            the license is renewed.
          </p>
        </div>
      </div>
      <slot />
    </template>
    <div v-else class="card ee-lock">
      <div class="ee-lock-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round"
          stroke-linejoin="round">
          <rect x="4" y="11" width="16" height="10" rx="2" />
          <path d="M8 11V7a4 4 0 0 1 8 0v4" />
        </svg>
      </div>
      <span class="ee-badge">Enterprise</span>
      <h3>{{ feature }}</h3>
      <p>{{ description }}</p>
      <p class="ee-lock-note">
        {{ feature }} is part of Posta Enterprise.
        <template v-if="auth.isAdmin">Install a license to turn it on; it works at once, with no restart.</template>
        <template v-else>Ask your Posta administrator about a license.</template>
      </p>
      <div class="ee-lock-actions">
        <router-link v-if="auth.isAdmin" class="btn btn-primary" to="/admin/license">Install a license</router-link>
        <a class="btn btn-secondary" :href="ENTERPRISE_URL" target="_blank" rel="noopener noreferrer">
          About Posta Enterprise
        </a>
      </div>
    </div>
  </template>
</template>

<style scoped>
.ee-lock {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 48px 24px;
  gap: 8px;
}

.ee-lock-icon {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}

.ee-lock-icon svg {
  width: 22px;
  height: 22px;
}

.ee-lock h3 {
  margin: 4px 0 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.ee-lock p {
  margin: 0;
  max-width: 520px;
  color: var(--text-secondary);
  font-size: 14px;
}

.ee-lock .ee-lock-note {
  color: var(--text-muted);
  font-size: 13px;
}

.ee-lock-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  margin-top: 8px;
}

.ee-readonly {
  margin-bottom: 16px;
}
</style>
