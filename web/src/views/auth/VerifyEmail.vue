<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { authApi } from '../../api/auth'
import { useAuthStore } from '../../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const status = ref<'verifying' | 'success' | 'error'>('verifying')
const message = ref('')

onMounted(async () => {
  const token = route.query.token as string
  if (!token) {
    status.value = 'error'
    message.value = 'Missing verification token.'
    return
  }
  try {
    const res = await authApi.verifyEmail(token)
    message.value = res.data.data?.message || 'Email verified.'
    status.value = 'success'
    if (auth.isAuthenticated) {
      await auth.fetchUser()
    }
  } catch (err: any) {
    message.value =
      err?.response?.data?.error?.message ||
      err?.response?.data?.error ||
      err?.message ||
      'Verification failed.'
    status.value = 'error'
  }
})

function goHome() {
  router.push(auth.isAuthenticated ? '/' : '/login')
}
</script>

<template>
  <div class="verify-page">
    <div class="verify-card">
      <h1>Email verification</h1>

      <template v-if="status === 'verifying'">
        <div class="spinner"></div>
        <p class="muted">Verifying your email…</p>
      </template>

      <template v-else-if="status === 'success'">
        <p class="success">{{ message }}</p>
        <button class="btn btn-primary" @click="goHome">
          {{ auth.isAuthenticated ? 'Go to dashboard' : 'Sign in' }}
        </button>
      </template>

      <template v-else>
        <p class="error">{{ message }}</p>
        <button class="btn" @click="goHome">
          {{ auth.isAuthenticated ? 'Go to dashboard' : 'Sign in' }}
        </button>
      </template>
    </div>
  </div>
</template>

<style scoped>
.verify-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--bg-secondary);
  padding: 24px;
}
.verify-card {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 32px;
  max-width: 420px;
  width: 100%;
  text-align: center;
}
.verify-card h1 {
  font-size: 20px;
  margin: 0 0 16px;
  color: var(--text-primary);
}
.verify-card p {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 8px 0 20px;
}
.muted { color: var(--text-muted); }
.success { color: var(--success-600, #16a34a); }
.error { color: var(--danger-600, #dc2626); }
.btn {
  display: inline-block;
  padding: 10px 18px;
  border-radius: 6px;
  border: 1px solid var(--border-color);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
  cursor: pointer;
}
.btn-primary {
  background: var(--primary-600, #2563eb);
  border-color: var(--primary-600, #2563eb);
  color: #fff;
}
.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-color);
  border-top-color: var(--primary-600, #2563eb);
  border-radius: 50%;
  margin: 12px auto;
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
