<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { authApi } from '../../api/auth'
import { useNotificationStore } from '../../stores/notification'
import { useThemeStore } from '../../stores/theme'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const theme = useThemeStore()

const token = ref((route.query.token as string) || '')
const password = ref('')
const confirm = ref('')
const showPassword = ref(false)
const loading = ref(false)
const passwordError = ref('')
const confirmError = ref('')
const passwordInput = ref<HTMLInputElement | null>(null)

onMounted(() => passwordInput.value?.focus())

async function submit() {
  passwordError.value = ''
  confirmError.value = ''
  if (password.value.length < 8) {
    passwordError.value = 'Password must be at least 8 characters.'
    return
  }
  if (confirm.value !== password.value) {
    confirmError.value = 'Passwords do not match.'
    return
  }
  loading.value = true
  try {
    await authApi.resetPassword(token.value, password.value)
    notify.success('Your password has been reset. Please sign in.')
    router.push({ name: 'login' })
  } catch (err: any) {
    passwordError.value = err?.response?.data?.error?.message || 'Could not reset your password. The link may have expired.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-header">
        <div class="auth-wordmark" aria-label="Posta">Posta<span class="auth-wordmark-dot">.</span></div>
        <h1 class="auth-title">Set a new password</h1>
        <p class="auth-subtitle">Choose a strong password you don't use elsewhere.</p>
      </div>

      <div v-if="!token" class="auth-form">
        <div class="auth-alert" role="alert">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
          <span>This reset link is missing its token. Request a new one.</span>
        </div>
      </div>

      <form v-else class="auth-form" @submit.prevent="submit">
        <div class="form-group">
          <label class="form-label" for="password">New password</label>
          <div class="password-wrap">
            <input
              id="password"
              ref="passwordInput"
              v-model="password"
              :type="showPassword ? 'text' : 'password'"
              class="form-input"
              :class="{ 'form-input-error': passwordError }"
              placeholder="At least 8 characters"
              autocomplete="new-password"
              :disabled="loading"
              :aria-invalid="!!passwordError"
              :aria-describedby="passwordError ? 'password-error' : undefined"
              @input="passwordError = ''"
            />
            <button
              type="button"
              class="password-toggle"
              :aria-label="showPassword ? 'Hide password' : 'Show password'"
              @click="showPassword = !showPassword"
            >
              <svg v-if="showPassword" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-11-8-11-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 11 8 11 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
              <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
            </button>
          </div>
          <small v-if="passwordError" id="password-error" class="form-error">{{ passwordError }}</small>
        </div>
        <div class="form-group">
          <label class="form-label" for="confirm">Confirm password</label>
          <input
            id="confirm"
            v-model="confirm"
            :type="showPassword ? 'text' : 'password'"
            class="form-input"
            :class="{ 'form-input-error': confirmError }"
            placeholder="Re-enter your password"
            autocomplete="new-password"
            :disabled="loading"
            :aria-invalid="!!confirmError"
            :aria-describedby="confirmError ? 'confirm-error' : undefined"
            @input="confirmError = ''"
          />
          <small v-if="confirmError" id="confirm-error" class="form-error">{{ confirmError }}</small>
        </div>
        <button type="submit" class="btn btn-primary auth-btn" :disabled="loading">
          <span v-if="loading" class="spinner"></span>
          {{ loading ? 'Resetting...' : 'Reset password' }}
        </button>
      </form>

      <div class="auth-footer">
        <router-link :to="{ name: 'login' }">Back to sign in</router-link>
      </div>
    </div>

    <button class="theme-btn" @click="theme.toggle()" :title="theme.isDark ? 'Light mode' : 'Dark mode'">
      <svg v-if="theme.isDark" width="18" height="18" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="3" stroke="currentColor" stroke-width="1.5"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
      <svg v-else width="18" height="18" viewBox="0 0 16 16" fill="none"><path d="M14 9.5A6.5 6.5 0 016.5 2 6.5 6.5 0 1014 9.5z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
    </button>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-secondary);
  padding: 20px;
  position: relative;
}
.auth-card {
  width: 100%;
  max-width: 400px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-lg);
}
.auth-header { text-align: center; padding: 36px 32px 0; }
.auth-wordmark {
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -1px;
  color: var(--text-primary);
  margin-bottom: 24px;
  line-height: 1;
}
.auth-wordmark-dot { color: var(--primary-500); margin-left: 1px; }
.auth-title {
  font-size: 22px;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: -0.2px;
  margin: 0 0 8px;
}
.auth-subtitle { font-size: 14px; color: var(--text-muted); margin: 0; line-height: 1.5; }
.auth-form { padding: 28px 32px 20px; }
.auth-btn { width: 100%; padding: 11px 18px; font-size: 15px; margin-top: 4px; }
.password-wrap { position: relative; }
.password-wrap .form-input { padding-right: 40px; }
.password-toggle {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
}
.password-toggle:hover { color: var(--text-primary); }
.form-input-error { border-color: var(--danger-500, #ef4444); }
.form-error { display: block; font-size: 12px; color: var(--danger-600, #dc2626); margin-top: 6px; }
.auth-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  font-size: 13px;
  color: var(--danger-700, #b91c1c);
  background: var(--danger-50, #fef2f2);
  border: 1px solid var(--danger-200, #fecaca);
  border-radius: var(--radius);
}
.auth-alert svg { flex-shrink: 0; }
.auth-footer {
  text-align: center;
  padding: 0 32px 28px;
  font-size: 14px;
  color: var(--text-muted);
}
.auth-footer a { color: var(--primary-500); font-weight: 500; }
.theme-btn {
  position: fixed;
  top: 20px;
  right: 20px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  padding: 10px;
  cursor: pointer;
  color: var(--text-tertiary);
  display: flex;
  align-items: center;
  box-shadow: var(--shadow-sm);
}
.theme-btn:hover { color: var(--text-primary); border-color: var(--border-input); }
</style>
