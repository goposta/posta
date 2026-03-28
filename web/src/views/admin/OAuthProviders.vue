<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { oauthApi } from '../../api/oauth'
import type { OAuthProviderAdmin } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'

const notify = useNotificationStore()
const { confirm } = useConfirm()

const providers = ref<OAuthProviderAdmin[]>([])
const loading = ref(true)

const showCreateModal = ref(false)
const creating = ref(false)
const form = ref({
  name: '', slug: '', type: 'oidc',
  client_id: '', client_secret: '', issuer: '',
  auth_url: '', token_url: '', userinfo_url: '',
  scopes: 'openid email profile',
  auto_register: true, allowed_domains: '',
})

async function fetchProviders() {
  loading.value = true
  try {
    const res = await oauthApi.adminList()
    providers.value = res.data.data ?? []
  } catch {
    notify.error('Failed to load OAuth providers')
  } finally {
    loading.value = false
  }
}

function autoSlug() {
  form.value.slug = form.value.name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

async function createProvider() {
  if (!form.value.name || !form.value.slug || !form.value.client_id || !form.value.client_secret) return
  creating.value = true
  try {
    await oauthApi.adminCreate(form.value)
    notify.success('Provider created')
    showCreateModal.value = false
    form.value = { name: '', slug: '', type: 'oidc', client_id: '', client_secret: '', issuer: '', auth_url: '', token_url: '', userinfo_url: '', scopes: 'openid email profile', auto_register: true, allowed_domains: '' }
    await fetchProviders()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to create provider')
  } finally {
    creating.value = false
  }
}

async function toggleEnabled(p: OAuthProviderAdmin) {
  try {
    await oauthApi.adminUpdate(p.id, { enabled: !p.enabled })
    notify.success(p.enabled ? 'Provider disabled' : 'Provider enabled')
    await fetchProviders()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to update')
  }
}

async function deleteProvider(p: OAuthProviderAdmin) {
  const ok = await confirm({ title: 'Delete Provider', message: `Delete "${p.name}"? Users won't be able to sign in with this provider.`, confirmText: 'Delete', variant: 'danger' })
  if (!ok) return
  try {
    await oauthApi.adminDelete(p.id)
    notify.success('Provider deleted')
    await fetchProviders()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to delete')
  }
}

function formatDate(d: string) {
  return new Date(d).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

onMounted(fetchProviders)
</script>

<template>
  <div>
    <div class="page-header">
      <h1>OAuth Providers</h1>
      <button class="btn btn-primary" @click="showCreateModal = true">Add Provider</button>
    </div>

    <div v-if="loading" class="loading-page"><div class="spinner"></div></div>

    <div v-else class="card">
      <div v-if="providers.length === 0" class="empty-state">
        <h3>No OAuth providers</h3>
        <p>Add a provider to enable social login (Google, OIDC).</p>
      </div>
      <div v-else class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Slug</th>
              <th>Type</th>
              <th>Status</th>
              <th>Auto Register</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in providers" :key="p.id">
              <td style="font-weight: 500;">{{ p.name }}</td>
              <td><code>{{ p.slug }}</code></td>
              <td><span class="badge badge-neutral">{{ p.type }}</span></td>
              <td><span class="badge" :class="p.enabled ? 'badge-primary' : 'badge-neutral'">{{ p.enabled ? 'enabled' : 'disabled' }}</span></td>
              <td>{{ p.auto_register ? 'Yes' : 'No' }}</td>
              <td>{{ formatDate(p.created_at) }}</td>
              <td>
                <div style="display: flex; gap: 6px;">
                  <button class="btn btn-secondary btn-sm" @click="toggleEnabled(p)">
                    {{ p.enabled ? 'Disable' : 'Enable' }}
                  </button>
                  <button class="btn btn-danger btn-sm" @click="deleteProvider(p)">Delete</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create Modal -->
    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal = false">
      <div class="modal" style="max-width: 560px;">
        <div class="modal-header"><h3>Add OAuth Provider</h3></div>
        <form @submit.prevent="createProvider">
          <div class="modal-body">
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px;">
              <div class="form-group">
                <label class="form-label">Name</label>
                <input v-model="form.name" class="form-input" placeholder="Google" required @input="autoSlug" />
              </div>
              <div class="form-group">
                <label class="form-label">Slug</label>
                <input v-model="form.slug" class="form-input" placeholder="google" required />
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">Type</label>
              <select v-model="form.type" class="form-select">
                <option value="google">Google</option>
                <option value="oidc">OIDC</option>
              </select>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px;">
              <div class="form-group">
                <label class="form-label">Client ID</label>
                <input v-model="form.client_id" class="form-input" required />
              </div>
              <div class="form-group">
                <label class="form-label">Client Secret</label>
                <input v-model="form.client_secret" class="form-input" type="password" required />
              </div>
            </div>
            <div v-if="form.type === 'oidc'" class="form-group">
              <label class="form-label">Issuer URL</label>
              <input v-model="form.issuer" class="form-input" placeholder="https://accounts.google.com" />
              <small style="font-size: 12px; color: var(--text-muted); display: block; margin-top: 4px;">Used for OIDC auto-discovery</small>
            </div>
            <div class="form-group">
              <label class="form-label">Scopes</label>
              <input v-model="form.scopes" class="form-input" placeholder="openid email profile" />
            </div>
            <div class="form-group">
              <label class="form-label">Allowed Domains (optional)</label>
              <input v-model="form.allowed_domains" class="form-input" placeholder="example.com, company.org" />
              <small style="font-size: 12px; color: var(--text-muted); display: block; margin-top: 4px;">Comma-separated. Leave empty to allow all.</small>
            </div>
            <div class="form-group">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 13px;">
                <input type="checkbox" v-model="form.auto_register" />
                Auto-register new users on first login
              </label>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showCreateModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="creating">{{ creating ? 'Creating...' : 'Create' }}</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
