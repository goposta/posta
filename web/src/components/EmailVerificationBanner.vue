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
  <div v-if="unverified" class="verify-banner">
    <span class="verify-banner-text">
      Your email is not verified. Some actions (inviting members, creating API keys) are blocked until you confirm your address.
    </span>
    <button class="verify-banner-btn" :disabled="sending" @click="resend">
      {{ sending ? 'Sending…' : 'Resend email' }}
    </button>
  </div>
</template>

<style scoped>
.verify-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 16px;
  margin: 12px 16px 0;
  background: var(--warning-50, #fffbeb);
  border: 1px solid var(--warning-200, #fde68a);
  color: var(--warning-900, #78350f);
  border-radius: 6px;
  font-size: 13px;
}
.verify-banner-text { flex: 1; }
.verify-banner-btn {
  padding: 6px 12px;
  border-radius: 4px;
  border: 1px solid var(--warning-400, #fbbf24);
  background: var(--warning-100, #fef3c7);
  color: var(--warning-900, #78350f);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
}
.verify-banner-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
