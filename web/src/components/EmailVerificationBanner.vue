<script setup lang="ts">
import { computed, ref } from 'vue'
import { authApi } from '../api/auth'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notification'

const auth = useAuthStore()
const notification = useNotificationStore()
const sending = ref(false)

const unverified = computed(() => {
  const u: any = auth.user
  if (!u) return false
  // Only nag when the backend says verification is enforced.
  if (u.email_verification_required === false) return false
  return !u.email_verified_at
})

async function resend() {
  sending.value = true
  try {
    await authApi.resendVerificationEmail()
    notification.success('Verification email sent. Check your inbox.')
  } catch (err: any) {
    const msg =
      err?.response?.data?.error?.message ||
      err?.response?.data?.error ||
      err?.message ||
      'Failed to send verification email.'
    notification.error(msg)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div v-if="unverified" class="app-banner app-banner--warning verify-banner" role="alert">
    <svg class="app-banner-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
      stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
      <line x1="12" y1="9" x2="12" y2="13" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
    <div class="app-banner-content">
      <p class="app-banner-title">Verify your email address</p>
      <p class="app-banner-text">
        Some actions — inviting members, creating API keys — stay locked until you confirm your address.
      </p>
    </div>
    <div class="app-banner-actions">
      <button class="app-banner-btn" :disabled="sending" @click="resend">
        {{ sending ? 'Sending…' : 'Resend email' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
/* Layout-only override; visual styling comes from the shared .app-banner system. */
.verify-banner {
  margin: 12px 16px 0;
}
</style>
