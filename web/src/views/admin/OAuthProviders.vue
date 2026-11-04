<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { oauthApi } from '../../api/oauth'
import type { OAuthProviderAdmin } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useLicenseStore } from '../../stores/license'
import { ENTERPRISE_URL } from '../../api/license'
import NameField from '../../components/NameField.vue'

const notify = useNotificationStore()
const { confirm } = useConfirm()
const license = useLicenseStore()

// Posta Community configures one OAuth provider; more need Enterprise.
const COMMUNITY_PROVIDERS = 1
const atCommunityLimit = computed(() => !license.mutable && providers.value.length >= COMMUNITY_PROVIDERS)

const providers = ref<OAuthProviderAdmin[]>([])
const loading = ref(true)

const showCreateModal = ref(false)
const creating = ref(false)
const emptyForm = () => ({
  name: '', type: 'oidc',
  client_id: '', client_secret: '', issuer: '',
  auth_url: '', token_url: '', userinfo_url: '',
  scopes: 'openid email profile',
  auto_register: true, hidden: false, allowed_domains: '',
})
const form = ref(emptyForm())

// Edit modal
const showEditModal = ref(false)
const editing = ref(false)
const editingProvider = ref<OAuthProviderAdmin | null>(null)
const editForm = ref({
  client_id: '', client_secret: '', issuer: '',
  auth_url: '', token_url: '', userinfo_url: '',
  scopes: '', auto_register: true, hidden: false, allowed_domains: '',
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

async function createProvider() {
  if (!form.value.name.trim() || !form.value.client_id || !form.value.client_secret) return
  creating.value = true
  try {
    await oauthApi.adminCreate(form.value)
    notify.success('Provider created')
    showCreateModal.value = false
    form.value = emptyForm()
    await fetchProviders()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to create provider')
  } finally {
    creating.value = false
  }
}

function openEdit(p: OAuthProviderAdmin) {
  editingProvider.value = p
  editForm.value = {
    client_id: '',
    client_secret: '',
    issuer: p.issuer || '',
    auth_url: '',
    token_url: '',
    userinfo_url: '',
    scopes: p.scopes || '',
    auto_register: p.auto_register,
    hidden: p.hidden,
    allowed_domains: p.allowed_domains || '',
  }
  showEditModal.value = true
}

async function updateProvider() {
  if (!editingProvider.value) return
  editing.value = true
  try {
    const data: Record<string, any> = {}
    if (editForm.value.client_id) data.client_id = editForm.value.client_id
    if (editForm.value.client_secret) data.client_secret = editForm.value.client_secret
    if (editForm.value.issuer) data.issuer = editForm.value.issuer
    if (editForm.value.scopes) data.scopes = editForm.value.scopes
    data.auto_register = editForm.value.auto_register
    data.hidden = editForm.value.hidden
    data.allowed_domains = editForm.value.allowed_domains
    await oauthApi.adminUpdate(editingProvider.value.id, data)
    notify.success('Provider updated')
    showEditModal.value = false
    await fetchProviders()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to update provider')
  } finally {
    editing.value = false
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
  const ok = await confirm({ title: 'Delete Provider', message: `Delete "${p.display_name}"? Users won't be able to sign in with this provider.`, confirmText: 'Delete', variant: 'danger' })
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

onMounted(() => {
  license.load()
  fetchProviders()
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1>OAuth Providers</h1>
      <button
        class="btn btn-primary"
        :disabled="atCommunityLimit"
        :title="atCommunityLimit ? 'More than one provider needs Posta Enterprise' : undefined"
        @click="showCreateModal = true"
      >
        Add Provider
      </button>
    </div>

    <div v-if="!loading && atCommunityLimit" class="app-banner app-banner--info oauth-limit" role="status">
      <div class="app-banner-content">
        <p class="app-banner-text">
          Posta Community signs in through one OAuth provider. Several providers, and LDAP / Active Directory,
          come with <span class="ee-badge">Enterprise</span>.
          <router-link to="/admin/license">Install a license</router-link> ·
          <a :href="ENTERPRISE_URL" target="_blank" rel="noopener noreferrer">About Posta Enterprise</a>
        </p>
      </div>
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
              <th>Display name</th>
              <th>Name</th>
              <th>Type</th>
              <th>Status</th>
              <th>Visibility</th>
              <th>Auto Register</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in providers" :key="p.id">
              <td style="font-weight: 500;">{{ p.display_name }}</td>
              <td><code>{{ p.name }}</code></td>
              <td><span class="badge badge-neutral">{{ p.type }}</span></td>
              <td><span class="badge" :class="p.enabled ? 'badge-primary' : 'badge-neutral'">{{ p.enabled ? 'enabled' : 'disabled' }}</span></td>
              <td><span class="badge" :class="p.hidden ? 'badge-neutral' : 'badge-primary'">{{ p.hidden ? 'hidden' : 'visible' }}</span></td>
              <td>{{ p.auto_register ? 'Yes' : 'No' }}</td>
              <td>{{ formatDate(p.created_at) }}</td>
              <td>
                <div style="display: flex; gap: 6px;">
                  <button class="btn btn-secondary btn-sm" @click="openEdit(p)">Edit</button>
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
            <NameField v-model="form.name" resource="provider" usage="Shown on the sign-in button, and in its URLs."
              placeholder="e.g. Google" autofocus />
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
            <div class="form-group">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 13px;">
                <input type="checkbox" v-model="form.hidden" />
                Hide from login page (only reachable via email-based SSO discovery)
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

    <!-- Edit Modal -->
    <div v-if="showEditModal && editingProvider" class="modal-overlay" @click.self="showEditModal = false">
      <div class="modal" style="max-width: 560px;">
        <div class="modal-header"><h3>Edit {{ editingProvider.display_name }}</h3></div>
        <form @submit.prevent="updateProvider">
          <div class="modal-body">
            <NameField :existing="editingProvider" resource="provider" usage="Shown on the sign-in button, and in its URLs." />
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px;">
              <div class="form-group">
                <label class="form-label">Client ID</label>
                <input v-model="editForm.client_id" class="form-input" placeholder="Leave empty to keep current" />
              </div>
              <div class="form-group">
                <label class="form-label">Client Secret</label>
                <input v-model="editForm.client_secret" class="form-input" type="password" placeholder="Leave empty to keep current" />
              </div>
            </div>
            <div v-if="editingProvider.type === 'oidc'" class="form-group">
              <label class="form-label">Issuer URL</label>
              <input v-model="editForm.issuer" class="form-input" placeholder="https://accounts.google.com" />
            </div>
            <div class="form-group">
              <label class="form-label">Scopes</label>
              <input v-model="editForm.scopes" class="form-input" placeholder="openid email profile" />
            </div>
            <div class="form-group">
              <label class="form-label">Allowed Domains</label>
              <input v-model="editForm.allowed_domains" class="form-input" placeholder="example.com, company.org" />
              <small style="font-size: 12px; color: var(--text-muted); display: block; margin-top: 4px;">Comma-separated. Leave empty to allow all.</small>
            </div>
            <div class="form-group">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 13px;">
                <input type="checkbox" v-model="editForm.auto_register" />
                Auto-register new users on first login
              </label>
            </div>
            <div class="form-group">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 13px;">
                <input type="checkbox" v-model="editForm.hidden" />
                Hide from login page (only reachable via email-based SSO discovery)
              </label>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showEditModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="editing">{{ editing ? 'Saving...' : 'Save' }}</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.oauth-limit {
  margin-bottom: 16px;
}
</style>
