<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { workspaceApi } from '../../api/workspaces'
import type { Workspace, WorkspaceMember, WorkspaceInvitation, WorkspaceRole, TransferResult, Plan } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useWorkspaceStore } from '../../stores/workspace'
import { useConfirm } from '../../composables/useConfirm'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const wsStore = useWorkspaceStore()
const { confirm } = useConfirm()

const wsId = Number(route.params.id)
const ws = ref<Workspace | null>(null)
const loading = ref(true)

// Tabs
const activeTab = ref<'members' | 'invitations' | 'plan' | 'transfer' | 'settings'>('members')

// Members
const members = ref<WorkspaceMember[]>([])
const membersLoading = ref(false)

// Invitations
const invitations = ref<WorkspaceInvitation[]>([])
const invitationsLoading = ref(false)

// Plan
const currentPlan = ref<Plan | null>(null)
const planLoading = ref(false)
const planSource = ref<string | null>(null)

// Invite modal
const showInviteModal = ref(false)
const inviteEmail = ref('')
const inviteRole = ref<WorkspaceRole>('editor')
const inviting = ref(false)

// Data Export/Import
const exporting = ref(false)
const importing = ref(false)
const importFileRef = ref<HTMLInputElement | null>(null)

// Settings
const editName = ref('')
const editDescription = ref('')
const saving = ref(false)

// Data Transfer
const availableResources = [
  { key: 'templates', label: 'Templates' },
  { key: 'stylesheets', label: 'Stylesheets' },
  { key: 'languages', label: 'Languages' },
  { key: 'smtp_servers', label: 'SMTP Servers' },
  { key: 'domains', label: 'Domains' },
  { key: 'webhooks', label: 'Webhooks' },
  { key: 'contacts', label: 'Contacts' },
  { key: 'subscribers', label: 'Subscribers' },
  { key: 'subscriber_lists', label: 'Lists' },
  { key: 'suppressions', label: 'Suppressions' },
  { key: 'api_keys', label: 'API Keys' },
  { key: 'bounces', label: 'Bounces' },
  { key: 'emails', label: 'Emails' },
]
const selectedResources = ref<string[]>([])
const transferring = ref(false)
const transferResults = ref<TransferResult[] | null>(null)
const transferTotal = ref(0)

const myRole = computed(() => {
  const membership = wsStore.workspaces.find(w => w.id === wsId)
  return membership?.role ?? 'viewer'
})

const isAdminOrOwner = computed(() => myRole.value === 'owner' || myRole.value === 'admin')
const isOwner = computed(() => myRole.value === 'owner')

async function fetchWorkspace() {
  loading.value = true
  // Temporarily set workspace context for API calls
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    const res = await workspaceApi.getCurrent()
    ws.value = res.data.data
    editName.value = ws.value.name
    editDescription.value = ws.value.description || ''
  } catch {
    notify.error('Failed to load workspace')
    router.push('/workspaces')
  } finally {
    loading.value = false
    wsStore.setWorkspace(prevWs)
  }
}

async function fetchMembers() {
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  membersLoading.value = true
  try {
    const res = await workspaceApi.listMembers()
    members.value = res.data.data ?? []
  } catch {
    notify.error('Failed to load members')
  } finally {
    membersLoading.value = false
    wsStore.setWorkspace(prevWs)
  }
}

async function fetchInvitations() {
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  invitationsLoading.value = true
  try {
    const res = await workspaceApi.listInvitations()
    invitations.value = res.data.data ?? []
  } catch {
    notify.error('Failed to load invitations')
  } finally {
    invitationsLoading.value = false
    wsStore.setWorkspace(prevWs)
  }
}

async function fetchPlan() {
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  planLoading.value = true
  try {
    const res = await workspaceApi.getPlan()
    const data = res.data.data
    if (data && typeof data === 'object' && 'id' in data) {
      currentPlan.value = data as Plan
      planSource.value = null
    } else if (data && typeof data === 'object' && 'source' in data) {
      currentPlan.value = null
      planSource.value = (data as any).source ?? 'global_settings'
    } else {
      currentPlan.value = null
      planSource.value = 'global_settings'
    }
  } catch {
    currentPlan.value = null
    planSource.value = 'global_settings'
  } finally {
    planLoading.value = false
    wsStore.setWorkspace(prevWs)
  }
}

function formatLimit(value: number): string {
  return value === 0 ? 'Unlimited' : value.toLocaleString()
}

async function inviteMember() {
  if (!inviteEmail.value.trim()) return
  inviting.value = true
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    await workspaceApi.invite({ email: inviteEmail.value.trim(), role: inviteRole.value })
    notify.success('Invitation sent')
    showInviteModal.value = false
    inviteEmail.value = ''
    inviteRole.value = 'editor'
    await fetchInvitations()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to invite')
  } finally {
    inviting.value = false
    wsStore.setWorkspace(prevWs)
  }
}

async function updateRole(member: WorkspaceMember, newRole: WorkspaceRole) {
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    await workspaceApi.updateMemberRole(member.user_id, newRole)
    notify.success('Role updated')
    await fetchMembers()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to update role')
  } finally {
    wsStore.setWorkspace(prevWs)
  }
}

async function removeMember(member: WorkspaceMember) {
  const ok = await confirm({ title: 'Remove Member', message: `Remove ${member.name || member.email} from this workspace?`, confirmText: 'Remove', variant: 'danger' })
  if (!ok) return
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    await workspaceApi.removeMember(member.user_id)
    notify.success('Member removed')
    await fetchMembers()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to remove')
  } finally {
    wsStore.setWorkspace(prevWs)
  }
}

async function revokeInvitation(inv: WorkspaceInvitation) {
  const ok = await confirm({ title: 'Revoke Invitation', message: `Revoke the invitation for ${inv.email}?`, confirmText: 'Revoke', variant: 'warning' })
  if (!ok) return
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    await workspaceApi.cancelInvitation(inv.id)
    notify.success('Invitation revoked')
    await fetchInvitations()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to revoke')
  } finally {
    wsStore.setWorkspace(prevWs)
  }
}

async function saveSettings() {
  saving.value = true
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    await workspaceApi.update({ name: editName.value.trim(), description: editDescription.value.trim() })
    notify.success('Workspace updated')
    await wsStore.fetchWorkspaces()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to update')
  } finally {
    saving.value = false
    wsStore.setWorkspace(prevWs)
  }
}

async function deleteWorkspace() {
  const ok = await confirm({ title: 'Delete Workspace', message: 'Are you sure you want to delete this workspace? This cannot be undone.', confirmText: 'Delete', variant: 'danger' })
  if (!ok) return
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    await workspaceApi.delete()
    notify.success('Workspace deleted')
    wsStore.setWorkspace(null)
    await wsStore.fetchWorkspaces()
    router.push('/workspaces')
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to delete')
    wsStore.setWorkspace(prevWs)
  }
}

function toggleResource(key: string) {
  const idx = selectedResources.value.indexOf(key)
  if (idx >= 0) {
    selectedResources.value.splice(idx, 1)
  } else {
    selectedResources.value.push(key)
  }
}

function selectAllResources() {
  if (selectedResources.value.length === availableResources.length) {
    selectedResources.value = []
  } else {
    selectedResources.value = availableResources.map(r => r.key)
  }
}

async function transferData() {
  if (selectedResources.value.length === 0) return
  const ok = await confirm({ title: 'Transfer Data', message: `Transfer ${selectedResources.value.length} resource type(s) from your personal account to this workspace? This cannot be undone.`, confirmText: 'Transfer', variant: 'warning' })
  if (!ok) return

  transferring.value = true
  transferResults.value = null
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    const res = await workspaceApi.transferData(selectedResources.value)
    transferResults.value = res.data.data.results
    transferTotal.value = res.data.data.total
    notify.success(res.data.data.message)
    selectedResources.value = []
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to transfer data')
  } finally {
    transferring.value = false
    wsStore.setWorkspace(prevWs)
  }
}

async function exportWorkspaceData() {
  exporting.value = true
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    const res = await workspaceApi.exportData()
    const blob = new Blob([JSON.stringify(res.data.data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `posta-workspace-export-${ws.value?.slug || wsId}-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    notify.success('Workspace data exported successfully')
  } catch {
    notify.error('Failed to export workspace data')
  } finally {
    exporting.value = false
    wsStore.setWorkspace(prevWs)
  }
}

function triggerWorkspaceImport() {
  importFileRef.value?.click()
}

async function handleWorkspaceImportFile(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return

  const confirmed = await confirm({
    title: 'Import Workspace Data',
    message: 'This will import data into this workspace from the selected file. Duplicate items will be skipped. SMTP servers will be imported as disabled (passwords are not exported). Domains will require re-verification. Continue?',
    confirmText: 'Import',
    variant: 'danger',
  })
  if (!confirmed) {
    if (importFileRef.value) importFileRef.value.value = ''
    return
  }

  importing.value = true
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    const text = await file.text()
    const data = JSON.parse(text)
    const res = await workspaceApi.importData(data)
    notify.success(res.data.data.message || 'Workspace data imported successfully')
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      notify.error('Invalid JSON file')
    } else {
      notify.error(e.response?.data?.error?.message || 'Failed to import workspace data')
    }
  } finally {
    importing.value = false
    if (importFileRef.value) importFileRef.value.value = ''
    wsStore.setWorkspace(prevWs)
  }
}

function roleBadgeClass(role: string): string {
  if (role === 'owner') return 'badge-primary'
  if (role === 'admin') return 'badge-warning'
  return 'badge-neutral'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

onMounted(async () => {
  await fetchWorkspace()
  await Promise.all([fetchMembers(), fetchInvitations(), fetchPlan()])
})
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>{{ ws?.name || 'Workspace' }}</h1>
        <p v-if="ws?.slug" style="color: var(--text-muted); font-size: 13px; margin-top: 2px;">
          <code>{{ ws.slug }}</code>
        </p>
      </div>
      <button class="btn btn-secondary" @click="router.push('/workspaces')">Back</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <!-- Tabs -->
      <div class="tabs">
        <button :class="['tab', { active: activeTab === 'members' }]" @click="activeTab = 'members'">
          Members ({{ members.length }})
        </button>
        <button :class="['tab', { active: activeTab === 'invitations' }]" @click="activeTab = 'invitations'">
          Invitations ({{ invitations.length }})
        </button>
        <button :class="['tab', { active: activeTab === 'plan' }]" @click="activeTab = 'plan'">
          Plan
        </button>
        <button v-if="isAdminOrOwner" :class="['tab', { active: activeTab === 'transfer' }]" @click="activeTab = 'transfer'">
          Data Transfer
        </button>
        <button v-if="isAdminOrOwner" :class="['tab', { active: activeTab === 'settings' }]" @click="activeTab = 'settings'">
          Settings
        </button>
      </div>

      <!-- Members Tab -->
      <div v-if="activeTab === 'members'" class="card">
        <div class="card-header">
          <span>Members</span>
          <button v-if="isAdminOrOwner" class="btn btn-primary btn-sm" @click="showInviteModal = true">Invite</button>
        </div>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Email</th>
                <th>Role</th>
                <th>Joined</th>
                <th v-if="isAdminOrOwner">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in members" :key="m.id">
                <td style="font-weight: 500;">{{ m.name }}</td>
                <td>{{ m.email }}</td>
                <td>
                  <select
                    v-if="isAdminOrOwner && m.role !== 'owner'"
                    :value="m.role"
                    class="form-select form-select-sm"
                    @change="updateRole(m, ($event.target as HTMLSelectElement).value as WorkspaceRole)"
                  >
                    <option value="admin">Admin</option>
                    <option value="editor">Editor</option>
                    <option value="viewer">Viewer</option>
                  </select>
                  <span v-else class="badge" :class="roleBadgeClass(m.role)">{{ m.role }}</span>
                </td>
                <td>{{ formatDate(m.created_at) }}</td>
                <td v-if="isAdminOrOwner">
                  <button v-if="m.role !== 'owner'" class="btn btn-danger btn-sm" @click="removeMember(m)">Remove</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Invitations Tab -->
      <div v-if="activeTab === 'invitations'" class="card">
        <div class="card-header">
          <span>Pending Invitations</span>
          <button v-if="isAdminOrOwner" class="btn btn-primary btn-sm" @click="showInviteModal = true">Invite</button>
        </div>
        <div v-if="invitations.length === 0" class="empty-state">
          <p>No pending invitations.</p>
        </div>
        <div v-else class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Email</th>
                <th>Role</th>
                <th>Expires</th>
                <th>Sent</th>
                <th v-if="isAdminOrOwner">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="inv in invitations" :key="inv.id">
                <td>{{ inv.email }}</td>
                <td><span class="badge" :class="roleBadgeClass(inv.role)">{{ inv.role }}</span></td>
                <td>{{ formatDate(inv.expires_at) }}</td>
                <td>{{ formatDate(inv.created_at) }}</td>
                <td v-if="isAdminOrOwner">
                  <button class="btn btn-danger btn-sm" @click="revokeInvitation(inv)">Revoke</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Plan Tab -->
      <div v-if="activeTab === 'plan'">
        <div v-if="planLoading" class="loading-page"><div class="spinner"></div></div>
        <template v-else-if="currentPlan">
          <div class="card" style="margin-bottom: 16px">
            <div class="card-header">
              <div>
                <h2 style="margin: 0">{{ currentPlan.name }}</h2>
                <p v-if="currentPlan.description" style="margin: 4px 0 0; font-size: 13px; color: var(--text-secondary)">{{ currentPlan.description }}</p>
              </div>
              <div class="flex gap-2">
                <span class="badge badge-success" v-if="currentPlan.is_active">Active</span>
                <span class="badge badge-info" v-if="currentPlan.is_default">Default</span>
              </div>
            </div>
          </div>

          <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 16px;">
            <!-- Rate Limits -->
            <div class="card">
              <div class="card-header"><h3 style="margin: 0; font-size: 14px">Rate Limits</h3></div>
              <div class="card-body">
                <table>
                  <tbody>
                    <tr>
                      <td style="font-weight: 600; width: 140px">Hourly</td>
                      <td>{{ formatLimit(currentPlan.hourly_rate_limit) }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">Daily</td>
                      <td>{{ formatLimit(currentPlan.daily_rate_limit) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Email Constraints -->
            <div class="card">
              <div class="card-header"><h3 style="margin: 0; font-size: 14px">Email Constraints</h3></div>
              <div class="card-body">
                <table>
                  <tbody>
                    <tr>
                      <td style="font-weight: 600; width: 140px">Attachment</td>
                      <td>{{ currentPlan.max_attachment_size_mb === 0 ? 'Unlimited' : currentPlan.max_attachment_size_mb + ' MB' }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">Batch Size</td>
                      <td>{{ formatLimit(currentPlan.max_batch_size) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Resource Limits -->
            <div class="card">
              <div class="card-header"><h3 style="margin: 0; font-size: 14px">Resource Limits</h3></div>
              <div class="card-body">
                <table>
                  <tbody>
                    <tr>
                      <td style="font-weight: 600; width: 140px">API Keys</td>
                      <td>{{ formatLimit(currentPlan.max_api_keys) }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">Domains</td>
                      <td>{{ formatLimit(currentPlan.max_domains) }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">SMTP Servers</td>
                      <td>{{ formatLimit(currentPlan.max_smtp_servers) }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">Workspaces</td>
                      <td>{{ formatLimit(currentPlan.max_workspaces) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Retention -->
            <div class="card">
              <div class="card-header"><h3 style="margin: 0; font-size: 14px">Data Retention</h3></div>
              <div class="card-body">
                <table>
                  <tbody>
                    <tr>
                      <td style="font-weight: 600; width: 140px">Email Logs</td>
                      <td>{{ currentPlan.email_log_retention_days === 0 ? 'Default' : currentPlan.email_log_retention_days + ' days' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </template>
        <div v-else class="card">
          <div class="empty-state">
            <h3>No plan assigned</h3>
            <p>This workspace is using the platform's global default settings. Contact your administrator to assign a plan.</p>
          </div>
        </div>
      </div>

      <!-- Data Transfer Tab -->
      <div v-if="activeTab === 'transfer' && isAdminOrOwner" class="card">
        <div class="card-header"><span>Transfer Personal Data to Workspace</span></div>
        <div class="card-body">
          <p style="font-size: 13px; color: var(--text-secondary); margin-bottom: 16px;">
            Move your personal resources into this workspace. Transferred data will no longer appear in your personal account.
          </p>

          <div style="margin-bottom: 16px;">
            <label class="form-label" style="margin-bottom: 8px;">Select resources to transfer</label>
            <div style="margin-bottom: 8px;">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 13px; font-weight: 600; color: var(--text-primary); padding: 6px 0;">
                <input
                  type="checkbox"
                  :checked="selectedResources.length === availableResources.length"
                  @change="selectAllResources"
                />
                Select All
              </label>
            </div>
            <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 4px;">
              <label
                v-for="res in availableResources"
                :key="res.key"
                style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 13px; color: var(--text-secondary); padding: 6px 8px; border-radius: var(--radius-sm); transition: background var(--transition);"
                :style="{ background: selectedResources.includes(res.key) ? 'var(--primary-50)' : 'transparent' }"
              >
                <input
                  type="checkbox"
                  :checked="selectedResources.includes(res.key)"
                  @change="toggleResource(res.key)"
                />
                {{ res.label }}
              </label>
            </div>
          </div>

          <button
            class="btn btn-primary"
            :disabled="transferring || selectedResources.length === 0"
            @click="transferData"
          >
            {{ transferring ? 'Transferring...' : `Transfer ${selectedResources.length} resource type(s)` }}
          </button>

          <!-- Transfer Results -->
          <div v-if="transferResults" style="margin-top: 20px; border-top: 1px solid var(--border-primary); padding-top: 16px;">
            <h4 style="font-size: 14px; font-weight: 600; margin-bottom: 12px;">
              Transfer Complete — {{ transferTotal }} record(s) moved
            </h4>
            <div class="table-wrapper">
              <table>
                <thead>
                  <tr>
                    <th>Resource</th>
                    <th>Records Transferred</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="r in transferResults" :key="r.resource">
                    <td style="font-weight: 500; text-transform: capitalize;">{{ r.resource.replace(/_/g, ' ') }}</td>
                    <td>
                      <span v-if="r.count > 0" class="badge badge-primary">{{ r.count }}</span>
                      <span v-else style="color: var(--text-muted);">0</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <!-- Settings Tab -->
      <div v-if="activeTab === 'settings' && isAdminOrOwner" class="card">
        <div class="card-header"><span>Workspace Settings</span></div>
        <div class="card-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="editName" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">Description</label>
            <input v-model="editDescription" class="form-input" placeholder="Optional description" />
          </div>
          <div class="form-group">
            <label class="form-label">Slug</label>
            <input :value="ws?.slug" class="form-input" disabled />
            <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block;">Slug cannot be changed after creation.</small>
          </div>
          <button class="btn btn-primary" :disabled="saving" @click="saveSettings">
            {{ saving ? 'Saving...' : 'Save Changes' }}
          </button>
        </div>

        <div class="card-body" style="border-top: 1px solid var(--border-primary);">
          <h4 style="font-size: 14px; font-weight: 600; margin-bottom: 8px;">Data Export / Import</h4>
          <p style="font-size: 13px; color: var(--text-secondary); margin-bottom: 12px;">
            Export all workspace data (settings, templates, stylesheets, languages, contacts, contact lists, webhooks,
            SMTP servers, domains, subscribers, subscriber lists, and suppressions) as a JSON file.
          </p>
          <p style="font-size: 12px; color: var(--text-muted); margin-bottom: 16px;">
            SMTP server passwords are not included in exports. Imported SMTP servers will be disabled until reconfigured.
            Imported domains will require re-verification.
          </p>
          <div style="display: flex; gap: 8px; flex-wrap: wrap;">
            <button class="btn btn-primary" :disabled="exporting" @click="exportWorkspaceData">
              {{ exporting ? 'Exporting...' : 'Export Workspace Data' }}
            </button>
            <button class="btn btn-secondary" :disabled="importing" @click="triggerWorkspaceImport">
              {{ importing ? 'Importing...' : 'Import Data' }}
            </button>
            <input ref="importFileRef" type="file" accept=".json" style="display: none" @change="handleWorkspaceImportFile" />
          </div>
        </div>

        <div v-if="isOwner" class="card-body" style="border-top: 1px solid var(--border-primary);">
          <h4 style="color: var(--danger-600); margin-bottom: 8px;">Danger Zone</h4>
          <p style="font-size: 13px; color: var(--text-muted); margin-bottom: 12px;">
            Deleting this workspace will permanently remove all its resources. This cannot be undone.
          </p>
          <button class="btn btn-danger" @click="deleteWorkspace">Delete Workspace</button>
        </div>
      </div>
    </template>

    <!-- Invite Modal -->
    <div v-if="showInviteModal" class="modal-overlay" @click.self="showInviteModal = false">
      <div class="modal">
        <div class="modal-header">
          <h3>Invite Member</h3>
        </div>
        <form @submit.prevent="inviteMember">
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Email Address</label>
              <input v-model="inviteEmail" type="email" class="form-input" placeholder="user@example.com" required />
            </div>
            <div class="form-group">
              <label class="form-label">Role</label>
              <select v-model="inviteRole" class="form-select">
                <option value="admin">Admin</option>
                <option value="editor">Editor</option>
                <option value="viewer">Viewer</option>
              </select>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showInviteModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="inviting || !inviteEmail.trim()">
              {{ inviting ? 'Sending...' : 'Send Invitation' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--border-primary);
  margin-bottom: 20px;
}
.tab {
  padding: 10px 20px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-muted);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: all var(--transition);
}
.tab:hover { color: var(--text-primary); }
.tab.active {
  color: var(--primary-600);
  border-bottom-color: var(--primary-600);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border-primary);
  font-weight: 600;
  font-size: 14px;
}

.card-body { padding: 20px; }
</style>
