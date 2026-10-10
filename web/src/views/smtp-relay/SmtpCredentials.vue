<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { smtpRelayApi } from '../../api/smtpRelay'
import type { SMTPCredential, SMTPCredentialCreateResponse } from '../../api/types'
import { infoApi, type AppInfo } from '../../api/info'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose'
import { useWorkspaceStore } from '../../stores/workspace'
import { usePagination } from '@/composables/usePagination'
import Pagination from '../../components/Pagination.vue'

const notify = useNotificationStore()
const wsStore = useWorkspaceStore()
const { confirm } = useConfirm()

const credentials = ref<SMTPCredential[]>([])
const loading = ref(true)
const featureDisabled = ref(false)
const appInfo = ref<AppInfo | null>(null)

const showCreateModal = ref(false)
const newCredentialName = ref('')
const newCredentialIPs = ref('')
const creating = ref(false)

const createdCredential = ref<SMTPCredentialCreateResponse | null>(null)
const showCredentialModal = ref(false)
const copied = ref(false)

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await smtpRelayApi.list(page, pageable.value.size)
    credentials.value = res.data.data
    pageable.value = res.data.pageable
    featureDisabled.value = false
  } catch (e: any) {
    if (e?.response?.status === 404) {
      featureDisabled.value = true
    } else {
      console.error('Failed to load SMTP credentials', e)
    }
  } finally {
    loading.value = false
  }
})

async function createCredential() {
  if (!newCredentialName.value.trim()) return
  creating.value = true
  try {
    const allowedIPs = newCredentialIPs.value
      .split(/[,\n]/)
      .map(ip => ip.trim())
      .filter(ip => ip.length > 0)
    const res = await smtpRelayApi.create(
      newCredentialName.value.trim(),
      allowedIPs.length > 0 ? allowedIPs : undefined,
    )
    createdCredential.value = res.data.data
    showCreateModal.value = false
    newCredentialName.value = ''
    newCredentialIPs.value = ''
    showCredentialModal.value = true
    notify.success('SMTP credential created')
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to create SMTP credential')
  } finally {
    creating.value = false
  }
}

async function revokeCredential(cred: SMTPCredential) {
  const confirmed = await confirm({
    title: 'Revoke SMTP Credential',
    message: `Are you sure you want to revoke "${cred.name}"? This credential will immediately stop working and cannot be reactivated.`,
    confirmText: 'Revoke',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await smtpRelayApi.revoke(cred.id)
    notify.success('SMTP credential revoked')
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to revoke SMTP credential')
  }
}

async function deleteCredential(cred: SMTPCredential) {
  const confirmed = await confirm({
    title: 'Delete SMTP Credential',
    message: `Are you sure you want to permanently delete "${cred.name}"? This action cannot be undone.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await smtpRelayApi.delete(cred.id)
    notify.success('SMTP credential deleted')
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to delete SMTP credential')
  }
}

function copyCredential() {
  if (!createdCredential.value) return
  const c = createdCredential.value
  const text = `Host: ${c.host}\nPort: ${c.port}\nEncryption: ${c.encryption}\nUsername: ${c.username}\nPassword: ${c.password}`
  navigator.clipboard.writeText(text)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}

function closeCredentialModal() {
  showCredentialModal.value = false
  createdCredential.value = null
  copied.value = false
}

function credentialStatus(cred: SMTPCredential): { label: string; class: string } {
  if (cred.revoked) return { label: 'Revoked', class: 'badge-danger' }
  return { label: 'Active', class: 'badge-success' }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showCreateModal.value = false
})

onMounted(() => {
  infoApi.get().then((res) => { appInfo.value = res.data.data }).catch(() => {})
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1>SMTP Relay</h1>
      <button v-if="wsStore.canEdit && !featureDisabled" class="btn btn-primary" @click="showCreateModal = true">Create Credential</button>
    </div>

    <div v-if="appInfo?.openapi_docs && !featureDisabled" class="card api-docs-callout">
      <div class="api-docs-text">
        <strong>Developer resources</strong>
        <span>Authenticate an existing SMTP client with these credentials to relay mail through Posta's outbound pipeline.</span>
      </div>
      <div class="api-docs-links">
        <a class="btn btn-secondary btn-sm" href="/docs" target="_blank" rel="noopener noreferrer">API Reference</a>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="featureDisabled" class="card">
      <div class="empty-state">
        <h3>SMTP Relay is disabled</h3>
        <p>
          Ask your administrator to enable it by setting
          <code>POSTA_SMTP_RELAY_ENABLED=true</code> and configuring the SMTP listener.
        </p>
      </div>
    </div>

    <div v-else class="card">
      <div v-if="credentials.length === 0" class="empty-state">
        <h3>No SMTP Credentials</h3>
        <p>Create a credential to let an SMTP client relay mail through Posta.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Username</th>
                <th>Created</th>
                <th>Last Used</th>
                <th>IP Allowlist</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="cred in credentials" :key="cred.id">
                <td>{{ cred.name }}</td>
                <td><code>{{ cred.username }}</code></td>
                <td>{{ formatDate(cred.created_at) }}</td>
                <td>{{ cred.last_used_at ? formatDate(cred.last_used_at) : 'Never' }}</td>
                <td>
                  <template v-if="cred.allowed_ips && cred.allowed_ips.length > 0">
                    <code v-for="(ip, i) in cred.allowed_ips.slice(0, 2)" :key="i" style="margin-right: 4px; font-size: 12px">{{ ip }}</code>
                    <span v-if="cred.allowed_ips.length > 2" style="font-size: 12px; color: var(--text-muted)">+{{ cred.allowed_ips.length - 2 }} more</span>
                  </template>
                  <span v-else style="color: var(--text-muted)">Any</span>
                </td>
                <td>
                  <span class="badge" :class="credentialStatus(cred).class">{{ credentialStatus(cred).label }}</span>
                </td>
                <td>
                  <div style="display: flex; gap: 6px">
                    <button
                      v-if="wsStore.canEdit && !cred.revoked"
                      class="btn btn-warning btn-sm"
                      @click="revokeCredential(cred)"
                    >
                      Revoke
                    </button>
                    <button
                      v-if="wsStore.canEdit && cred.revoked"
                      class="btn btn-danger btn-sm"
                      @click="deleteCredential(cred)"
                    >
                      Delete
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination :pageable="pageable" @page="goToPage" />
      </template>
    </div>

    <!-- Create Credential Modal -->
    <div v-if="showCreateModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Create SMTP Credential</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input
              v-model="newCredentialName"
              class="form-input"
              placeholder="e.g. Legacy App, Staging"
              @keyup.enter="createCredential"
            />
          </div>
          <div class="form-group">
            <label class="form-label">Allowed IPs <span style="font-weight: 400; color: var(--text-muted)">(optional)</span></label>
            <textarea
              v-model="newCredentialIPs"
              class="form-input"
              rows="3"
              placeholder="Comma or newline separated, e.g.&#10;192.168.1.1&#10;10.0.0.0/24"
            ></textarea>
            <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block">
              Restrict this credential to specific IP addresses. Leave empty to allow all.
            </small>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showCreateModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="creating || !newCredentialName.trim()" @click="createCredential">
            {{ creating ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Show Credential Modal -->
    <div v-if="showCredentialModal" class="modal-overlay">
      <div class="modal">
        <div class="modal-header">
          <h3>SMTP Credential Created</h3>
        </div>
        <div class="modal-body">
          <p class="text-sm" style="color: var(--danger-600); font-weight: 500; margin-bottom: 12px;">
            {{ createdCredential?.message || "Save this password securely. It will not be shown again." }}
          </p>
          <div class="form-group">
            <label class="form-label">Host</label>
            <div class="code-block">{{ createdCredential?.host }}</div>
          </div>
          <div class="form-group">
            <label class="form-label">Port</label>
            <div class="code-block">{{ createdCredential?.port }}</div>
          </div>
          <div class="form-group">
            <label class="form-label">Encryption</label>
            <div class="code-block">{{ createdCredential?.encryption === 'starttls' ? 'STARTTLS (required)' : 'None — private network only' }}</div>
          </div>
          <div class="form-group">
            <label class="form-label">Username</label>
            <div class="code-block">{{ createdCredential?.username }}</div>
          </div>
          <div class="form-group">
            <label class="form-label">Password</label>
            <div class="code-block">{{ createdCredential?.password }}</div>
          </div>
          <button class="btn btn-secondary btn-sm mt-4" @click="copyCredential">
            {{ copied ? 'Copied!' : 'Copy Credentials' }}
          </button>
        </div>
        <div class="modal-footer">
          <button class="btn btn-primary" @click="closeCredentialModal">Done</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.api-docs-callout {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding: 16px 20px;
  margin-bottom: 20px;
}
.api-docs-text { display: flex; flex-direction: column; gap: 2px; }
.api-docs-text strong { font-size: 14px; color: var(--text-primary); }
.api-docs-text span { font-size: 13px; color: var(--text-secondary); }
.api-docs-links { display: flex; gap: 8px; flex-shrink: 0; }
.api-docs-links .btn { text-decoration: none; }
</style>
