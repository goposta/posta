<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { authApi } from '../../api/auth'
import { useThemeStore } from '../../stores/theme'

const theme = useThemeStore()

const email = ref('')
const emailError = ref('')
const loading = ref(false)
const sent = ref(false)
const message = ref('')
const emailInput = ref<HTMLInputElement | null>(null)

onMounted(() => emailInput.value?.focus())

async function submit() {
  emailError.value = ''
  if (!email.value) {
    emailError.value = 'Enter your email address.'
    return
  }
  loading.value = true
  try {
    const res = await authApi.forgotPassword(email.value)
    message.value = res.data.data?.message || 'If an account exists for that email, a reset link is on its way.'
    sent.value = true
  } catch (err: any) {
    emailError.value = err?.response?.data?.error?.message || 'Could not send the reset email. Please try again.'
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
        <h1 class="auth-title">{{ sent ? 'Check your email' : 'Forgot your password?' }}</h1>
        <p class="auth-subtitle">
          {{ sent ? message : "Enter your account email and we'll send you a link to reset your password." }}
        </p>
      </div>

      <form v-if="!sent" class="auth-form" @submit.prevent="submit">
        <div class="form-group">
          <label class="form-label" for="email">Email</label>
          <input
            id="email"
            ref="emailInput"
            v-model="email"
            type="email"
            class="form-input"
            :class="{ 'form-input-error': emailError }"
            placeholder="you@example.com"
            autocomplete="email"
            :disabled="loading"
            :aria-invalid="!!emailError"
            :aria-describedby="emailError ? 'email-error' : undefined"
            @input="emailError = ''"
          />
          <small v-if="emailError" id="email-error" class="form-error">{{ emailError }}</small>
        </div>
        <button type="submit" class="btn btn-primary auth-btn" :disabled="loading">
          <span v-if="loading" class="spinner"></span>
          {{ loading ? 'Sending...' : 'Send reset link' }}
        </button>
      </form>

      <div v-else class="auth-form">
        <div class="auth-success" role="status">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
          <span>Reset link sent. The link expires in 1 hour.</span>
        </div>
      </div>

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
.form-input-error { border-color: var(--danger-500, #ef4444); }
.form-error { display: block; font-size: 12px; color: var(--danger-600, #dc2626); margin-top: 6px; }
.auth-success {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  font-size: 14px;
  color: var(--success-700, #15803d);
  background: var(--success-50, #f0fdf4);
  border: 1px solid var(--success-200, #bbf7d0);
  border-radius: var(--radius);
}
.auth-success svg { flex-shrink: 0; }
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
