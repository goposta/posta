<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'
import { workspaceApi } from '../../api/workspaces'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const token = computed(() => (route.query.token as string) || '')
const status = ref<'idle' | 'accepting' | 'success' | 'error'>('idle')
const message = ref('')

const loginHref = computed(() => {
  const target = `/invitations?token=${encodeURIComponent(token.value)}`
  return `/login?redirect=${encodeURIComponent(target)}`
})

onMounted(async () => {
  if (!token.value) {
    status.value = 'error'
    message.value = 'Missing invitation token.'
    return
  }
  if (!auth.isAuthenticated) {
    return
  }
  await accept()
})

async function accept() {
  status.value = 'accepting'
  try {
    const res = await workspaceApi.acceptInvitationByToken(token.value)
    message.value = res.data.data?.message || 'Invitation accepted.'
    status.value = 'success'
  } catch (err: any) {
    const apiMsg =
      err?.response?.data?.error?.message ||
      err?.response?.data?.error ||
      err?.message ||
      'Failed to accept invitation.'
    message.value = apiMsg
    status.value = 'error'
  }
}

function goToWorkspaces() {
  router.push('/workspaces')
}
</script>

<template>
  <div class="invitation-page">
    <div class="invitation-card">
      <h1>Workspace invitation</h1>

      <template v-if="status === 'error' && !token">
        <p class="muted">{{ message }}</p>
        <router-link to="/" class="btn">Go to dashboard</router-link>
      </template>

      <template v-else-if="!auth.isAuthenticated">
        <p>Please sign in to accept this workspace invitation.</p>
        <a :href="loginHref" class="btn btn-primary">Sign in to continue</a>
      </template>

      <template v-else-if="status === 'accepting' || status === 'idle'">
        <div class="spinner"></div>
        <p class="muted">Accepting invitation…</p>
      </template>

      <template v-else-if="status === 'success'">
        <p class="success">{{ message }}</p>
        <button class="btn btn-primary" @click="goToWorkspaces">Open workspaces</button>
      </template>

      <template v-else>
        <p class="error">{{ message }}</p>
        <router-link to="/workspaces" class="btn">Go to workspaces</router-link>
      </template>
    </div>
  </div>
</template>

<style scoped>
.invitation-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--bg-secondary);
  padding: 24px;
}
.invitation-card {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 32px;
  max-width: 420px;
  width: 100%;
  text-align: center;
}
.invitation-card h1 {
  font-size: 20px;
  margin: 0 0 16px;
  color: var(--text-primary);
}
.invitation-card p {
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
  text-decoration: none;
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
