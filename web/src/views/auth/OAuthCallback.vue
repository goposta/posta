<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const error = ref('')

onMounted(async () => {
  const token = route.query.token as string
  const errorParam = route.query.error as string

  if (errorParam) {
    error.value = errorParam.replace(/_/g, ' ')
    setTimeout(() => router.push('/login'), 3000)
    return
  }

  if (!token) {
    error.value = 'No token received'
    setTimeout(() => router.push('/login'), 3000)
    return
  }

  // Set token and fetch user profile
  localStorage.setItem('posta_token', token)
  auth.token = token

  try {
    await auth.fetchUser()
    router.push('/')
  } catch {
    error.value = 'Failed to load user profile'
    setTimeout(() => router.push('/login'), 3000)
  }
})
</script>

<template>
  <div class="oauth-callback">
    <div v-if="error" class="oauth-error">
      <h2>Authentication Error</h2>
      <p>{{ error }}</p>
      <p style="font-size: 13px; color: var(--text-muted);">Redirecting to login...</p>
    </div>
    <div v-else class="oauth-loading">
      <div class="spinner"></div>
      <p>Completing sign in...</p>
    </div>
  </div>
</template>

<style scoped>
.oauth-callback {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--bg-secondary);
}

.oauth-loading, .oauth-error {
  text-align: center;
  padding: 40px;
}

.oauth-loading p {
  margin-top: 16px;
  font-size: 14px;
  color: var(--text-secondary);
}

.oauth-error h2 {
  color: var(--danger-600);
  margin-bottom: 8px;
}

.oauth-error p {
  color: var(--text-secondary);
  font-size: 14px;
}
</style>
