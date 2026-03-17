<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiKeysApi } from '../../api/apikeys'
import { settingsApi } from '../../api/settings'
import type { ApiKey, ApiKeyCreateResponse, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const notify = useNotificationStore()
const { confirm } = useConfirm()

const keys = ref<ApiKey[]>([])
const pageable = ref<Pageable>({ current_page: 0, size: 20, total_pages: 0, total_elements: 0, empty: true })
const loading = ref(true)

const showCreateModal = ref(false)
const newKeyName = ref('')
const newKeyIPs = ref('')
const newKeyExpiry = ref('default')
const defaultExpiryDays = ref(90)
const creating = ref(false)

const createdKey = ref<ApiKeyCreateResponse | null>(null)
const showKeyModal = ref(false)
const copied = ref(false)

const expiryOptions = [
  { label: 'Default', value: 'default' },
  { label: '30 days', value: '30' },
  { label: '60 days', value: '60' },
  { label: '90 days', value: '90' },
  { label: '180 days', value: '180' },
  { label: '365 days', value: '365' },
  { label: 'Never', value: 'never' },
]

async function loadSettings() {
  try {
    const res = await settingsApi.getUserSettings()
    defaultExpiryDays.value = res.data.data.api_key_expiry_days || 90
  } catch {
    // Use fallback default
  }
}

async function loadKeys(page = 0) {
  loading.value = true
  try {
    const res = await apiKeysApi.list(page, pageable.value.size)
    keys.value = res.data.data
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load API keys')
  } finally {
    loading.value = false
  }
}

function resolveExpiryDays(): number | undefined {
  if (newKeyExpiry.value === 'never') return 0
  if (newKeyExpiry.value === 'default') return undefined // let backend use user setting
  return parseInt(newKeyExpiry.value, 10)
}

async function createKey() {
  if (!newKeyName.value.trim()) return
  creating.value = true
  try {
    const allowedIPs = newKeyIPs.value
      .split(/[,\n]/)
      .map(ip => ip.trim())
      .filter(ip => ip.length > 0)
    const expiresInDays = resolveExpiryDays()
    const res = await apiKeysApi.create(
      newKeyName.value.trim(),
      allowedIPs.length > 0 ? allowedIPs : undefined,
      expiresInDays,
    )
    createdKey.value = res.data.data
    showCreateModal.value = false
    newKeyName.value = ''
    newKeyIPs.value = ''
    newKeyExpiry.value = 'default'
    showKeyModal.value = true
    notify.success('API key created')
    await loadKeys(pageable.value.current_page)
  } catch {
    notify.error('Failed to create API key')
  } finally {
    creating.value = false
  }
}

async function revokeKey(key: ApiKey) {
  const confirmed = await confirm({
    title: 'Revoke API Key',
    message: `Are you sure you want to revoke "${key.name}"? This key will immediately stop working and cannot be reactivated.`,
    confirmText: 'Revoke',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await apiKeysApi.revoke(key.id)
    notify.success('API key revoked')
    await loadKeys(pageable.value.current_page)
  } catch {
    notify.error('Failed to revoke API key')
  }
}

async function deleteKey(key: ApiKey) {
  const confirmed = await confirm({
    title: 'Delete API Key',
    message: `Are you sure you want to permanently delete "${key.name}"? This action cannot be undone.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await apiKeysApi.delete(key.id)
    notify.success('API key deleted')
    await loadKeys(pageable.value.current_page)
  } catch {
    notify.error('Failed to delete API key')
  }
}

function copyKey() {
  if (!createdKey.value) return
  navigator.clipboard.writeText(createdKey.value.key)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}

function closeKeyModal() {
  showKeyModal.value = false
  createdKey.value = null
  copied.value = false
}

function keyStatus(key: ApiKey): { label: string; class: string } {
  if (key.revoked) return { label: 'Revoked', class: 'badge-danger' }
  if (key.expires_at && new Date(key.expires_at) < new Date()) return { label: 'Expired', class: 'badge-warning' }
  return { label: 'Active', class: 'badge-success' }
}

function isActive(key: ApiKey): boolean {
  return !key.revoked && !(key.expires_at && new Date(key.expires_at) < new Date())
}

function canDelete(key: ApiKey): boolean {
  return key.revoked || (!!key.expires_at && new Date(key.expires_at) < new Date())
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showCreateModal.value = false;
});

onMounted(() => {
  loadSettings()
  loadKeys()
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1>API Keys</h1>
      <button class="btn btn-primary" @click="showCreateModal = true">Create Key</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="keys.length === 0" class="empty-state">
        <h3>No API Keys</h3>
        <p>Create your first API key to start sending emails.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Key Prefix</th>
                <th>Created</th>
                <th>Last Used</th>
                <th>Expires</th>
                <th>IP Allowlist</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="key in keys" :key="key.id">
                <td>{{ key.name }}</td>
                <td><code>{{ key.key_prefix }}...</code></td>
                <td>{{ formatDate(key.created_at) }}</td>
                <td>{{ key.last_used_at ? formatDate(key.last_used_at) : 'Never' }}</td>
                <td>{{ key.expires_at ? formatDate(key.expires_at) : 'Never' }}</td>
                <td>
                  <template v-if="key.allowed_ips && key.allowed_ips.length > 0">
                    <code v-for="(ip, i) in key.allowed_ips.slice(0, 2)" :key="i" style="margin-right: 4px; font-size: 12px">{{ ip }}</code>
                    <span v-if="key.allowed_ips.length > 2" style="font-size: 12px; color: var(--text-muted)">+{{ key.allowed_ips.length - 2 }} more</span>
                  </template>
                  <span v-else style="color: var(--text-muted)">Any</span>
                </td>
                <td>
                  <span class="badge" :class="keyStatus(key).class">{{ keyStatus(key).label }}</span>
                </td>
                <td>
                  <div style="display: flex; gap: 6px">
                    <button
                      v-if="isActive(key)"
                      class="btn btn-warning btn-sm"
                      @click="revokeKey(key)"
                    >
                      Revoke
                    </button>
                    <button
                      v-if="canDelete(key)"
                      class="btn btn-danger btn-sm"
                      @click="deleteKey(key)"
                    >
                      Delete
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="pagination">
          <span class="pagination-info">
            Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }} ({{ pageable.total_elements }} keys)
          </span>
          <div class="pagination-buttons">
            <button
              class="btn btn-secondary btn-sm"
              :disabled="pageable.current_page === 0"
              @click="loadKeys(pageable.current_page - 1)"
            >
              Previous
            </button>
            <button
              class="btn btn-secondary btn-sm"
              :disabled="pageable.current_page >= pageable.total_pages - 1"
              @click="loadKeys(pageable.current_page + 1)"
            >
              Next
            </button>
          </div>
        </div>
      </template>
    </div>

    <!-- Create Key Modal -->
    <div v-if="showCreateModal" class="modal-overlay" @mousedown="watchClickStart" 
      @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Create API Key</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input
              v-model="newKeyName"
              class="form-input"
              placeholder="e.g. Production, Staging"
              @keyup.enter="createKey"
            />
          </div>
          <div class="form-group">
            <label class="form-label">Expiration</label>
            <select v-model="newKeyExpiry" class="form-input">
              <option v-for="opt in expiryOptions" :key="opt.value" :value="opt.value">
                {{ opt.value === 'default' ? `${opt.label} (${defaultExpiryDays} days)` : opt.label }}
              </option>
            </select>
            <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block">
              Choose when this key expires. "Never" means the key will not expire.
            </small>
          </div>
          <div class="form-group">
            <label class="form-label">Allowed IPs <span style="font-weight: 400; color: var(--text-muted)">(optional)</span></label>
            <textarea
              v-model="newKeyIPs"
              class="form-input"
              rows="3"
              placeholder="Comma or newline separated, e.g.&#10;192.168.1.1&#10;10.0.0.0/24"
            ></textarea>
            <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block">
              Restrict this key to specific IP addresses. Leave empty to allow all.
            </small>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showCreateModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="creating || !newKeyName.trim()" @click="createKey">
            {{ creating ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Show Key Modal -->
    <div v-if="showKeyModal" class="modal-overlay">
      <div class="modal">
        <div class="modal-header">
          <h3>API Key Created</h3>
        </div>
        <div class="modal-body">
          <p class="text-sm" style="color: var(--danger-600); font-weight: 500; margin-bottom: 12px;">
            Save this key. It won't be shown again.
          </p>
          <div class="code-block">{{ createdKey?.key }}</div>
          <p v-if="createdKey?.expires_at" class="text-sm" style="margin-top: 8px; color: var(--text-muted)">
            Expires: {{ formatDate(createdKey.expires_at) }}
          </p>
          <p v-else class="text-sm" style="margin-top: 8px; color: var(--text-muted)">
            This key never expires.
          </p>
          <button class="btn btn-secondary btn-sm mt-4" @click="copyKey">
            {{ copied ? 'Copied!' : 'Copy Key' }}
          </button>
        </div>
        <div class="modal-footer">
          <button class="btn btn-primary" @click="closeKeyModal">Done</button>
        </div>
      </div>
    </div>
  </div>
</template>
