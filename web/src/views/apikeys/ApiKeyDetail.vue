<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiKeysApi } from '../../api/apikeys'
import type { ApiKey } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useWorkspaceStore } from '../../stores/workspace'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const wsStore = useWorkspaceStore()
const { confirm } = useConfirm()

const loading = ref(true)
const key = ref<ApiKey | null>(null)

const keyId = Number(route.params.id)

onMounted(async () => {
  try {
    const res = await apiKeysApi.get(keyId)
    key.value = res.data.data
  } catch {
    notify.error('Failed to load API key')
  } finally {
    loading.value = false
  }
})

const status = computed<{ label: string; class: string }>(() => {
  const k = key.value
  if (!k) return { label: '', class: '' }
  if (k.revoked) return { label: 'Revoked', class: 'badge-danger' }
  if (k.expires_at && new Date(k.expires_at) < new Date()) return { label: 'Expired', class: 'badge-warning' }
  return { label: 'Active', class: 'badge-success' }
})

// An empty/missing scope set means the key predates scopes; show it as send.
const keyScopes = computed<string[]>(() => key.value?.scopes ?? [])

const isActive = computed(() => {
  const k = key.value
  return !!k && !k.revoked && !(k.expires_at && new Date(k.expires_at) < new Date())
})

const canDelete = computed(() => {
  const k = key.value
  return !!k && (k.revoked || (!!k.expires_at && new Date(k.expires_at) < new Date()))
})

function formatDate(dateStr: string | null): string {
  if (!dateStr) return 'Never'
  return new Date(dateStr).toLocaleString()
}

async function revoke() {
  if (!key.value) return
  const confirmed = await confirm({
    title: 'Revoke API Key',
    message: `Are you sure you want to revoke "${key.value.name}"? This key will immediately stop working and cannot be reactivated.`,
    confirmText: 'Revoke',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await apiKeysApi.revoke(key.value.id)
    notify.success('API key revoked')
    const res = await apiKeysApi.get(keyId)
    key.value = res.data.data
  } catch {
    notify.error('Failed to revoke API key')
  }
}

async function remove() {
  if (!key.value) return
  const confirmed = await confirm({
    title: 'Delete API Key',
    message: `Are you sure you want to permanently delete "${key.value.name}"? This action cannot be undone.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await apiKeysApi.delete(key.value.id)
    notify.success('API key deleted')
    router.push('/api-keys')
  } catch {
    notify.error('Failed to delete API key')
  }
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>{{ key?.name || 'API Key' }}</h1>
      <div style="display: flex; gap: 8px">
        <button v-if="wsStore.canEdit && isActive" class="btn btn-danger" @click="revoke">Revoke</button>
        <button v-if="wsStore.canEdit && canDelete" class="btn btn-danger" @click="remove">Delete</button>
        <button class="btn btn-secondary" @click="router.push('/api-keys')">Back to API Keys</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="!key" class="card">
      <div class="empty-state">
        <h3>API key not found</h3>
        <p>It may have been deleted, or belongs to another workspace.</p>
      </div>
    </div>

    <div v-else class="card">
      <div class="card-header">
        <h2>{{ key.name }}</h2>
        <span class="badge" :class="status.class">{{ status.label }}</span>
      </div>
      <div class="card-body">
        <table>
          <tbody>
            <tr>
              <td style="font-weight: 600; width: 160px">Key ID</td>
              <td><code>{{ key.id }}</code></td>
            </tr>
            <tr>
              <td style="font-weight: 600">Prefix</td>
              <td><code>{{ key.key_prefix }}…</code></td>
            </tr>
            <tr>
              <td style="font-weight: 600">Created</td>
              <td>{{ formatDate(key.created_at) }}</td>
            </tr>
            <tr v-if="key.created_by">
              <td style="font-weight: 600">Created by</td>
              <td>{{ key.created_by.name }}</td>
            </tr>
            <tr>
              <td style="font-weight: 600">Last used</td>
              <td>{{ formatDate(key.last_used_at) }}</td>
            </tr>
            <tr>
              <td style="font-weight: 600">Expires</td>
              <td>{{ formatDate(key.expires_at) }}</td>
            </tr>
            <tr>
              <td style="font-weight: 600">Scopes</td>
              <td>
                <template v-if="keyScopes.length > 0">
                  <span
                    v-for="s in keyScopes"
                    :key="s"
                    style="display: inline-block; padding: 2px 8px; margin-right: 4px; font-size: 12px; border-radius: 4px; background: var(--bg-muted, #eef0f3); color: var(--text-secondary)"
                    >{{ s }}</span
                  >
                </template>
                <span v-else style="color: var(--text-muted)">send</span>
              </td>
            </tr>
            <tr>
              <td style="font-weight: 600">Allowed IPs</td>
              <td>
                <template v-if="key.allowed_ips && key.allowed_ips.length > 0">
                  <code v-for="(ip, i) in key.allowed_ips" :key="i" style="margin-right: 6px; font-size: 12px">{{ ip }}</code>
                </template>
                <span v-else style="color: var(--text-muted)">Any IP</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
