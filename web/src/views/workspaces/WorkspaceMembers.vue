<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { workspaceApi } from '../../api/workspaces'
import type { Workspace, WorkspaceMember, WorkspaceInvitation, WorkspaceRole } from '../../api/types'
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
type MembersTab = 'members' | 'invitations'
const validTabs: MembersTab[] = ['members', 'invitations']
function tabFromQuery(value: unknown): MembersTab {
  return validTabs.includes(value as MembersTab) ? (value as MembersTab) : 'members'
}
const activeTab = ref<MembersTab>(tabFromQuery(route.query.tab))
watch(() => route.query.tab, (value) => {
  if (value) activeTab.value = tabFromQuery(value)
})

const members = ref<WorkspaceMember[]>([])
const invitations = ref<WorkspaceInvitation[]>([])

// Invite modal
const showInviteModal = ref(false)
const inviteEmail = ref('')
const inviteRole = ref<WorkspaceRole>('editor')
const inviting = ref(false)

const myRole = computed(() => {
  const membership = wsStore.workspaces.find(w => w.id === wsId)
  return membership?.role ?? 'viewer'
})
const isAdminOrOwner = computed(() => myRole.value === 'owner' || myRole.value === 'admin')

async function withWorkspace<T>(fn: () => Promise<T>): Promise<T> {
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    return await fn()
  } finally {
    wsStore.setWorkspace(prevWs)
  }
}

async function fetchWorkspace() {
  loading.value = true
  try {
    const res = await withWorkspace(() => workspaceApi.getCurrent())
    ws.value = res.data.data
  } catch {
    notify.error('Failed to load workspace')
    router.push('/workspaces')
  } finally {
    loading.value = false
  }
}

async function fetchMembers() {
  try {
    const res = await withWorkspace(() => workspaceApi.listMembers())
    members.value = res.data.data ?? []
  } catch {
    notify.error('Failed to load members')
  }
}

async function fetchInvitations() {
  try {
    const res = await withWorkspace(() => workspaceApi.listInvitations())
    invitations.value = res.data.data ?? []
  } catch {
    notify.error('Failed to load invitations')
  }
}

async function inviteMember() {
  if (!inviteEmail.value.trim()) return
  inviting.value = true
  try {
    await withWorkspace(() => workspaceApi.invite({ email: inviteEmail.value.trim(), role: inviteRole.value }))
    notify.success('Invitation sent')
    showInviteModal.value = false
    inviteEmail.value = ''
    inviteRole.value = 'editor'
    await fetchInvitations()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to invite')
  } finally {
    inviting.value = false
  }
}

async function updateRole(member: WorkspaceMember, newRole: WorkspaceRole) {
  try {
    await withWorkspace(() => workspaceApi.updateMemberRole(member.user_id, newRole))
    notify.success('Role updated')
    await fetchMembers()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to update role')
  }
}

async function removeMember(member: WorkspaceMember) {
  const ok = await confirm({ title: 'Remove Member', message: `Remove ${member.name || member.email} from this workspace?`, confirmText: 'Remove', variant: 'danger' })
  if (!ok) return
  try {
    await withWorkspace(() => workspaceApi.removeMember(member.user_id))
    notify.success('Member removed')
    await fetchMembers()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to remove')
  }
}

async function revokeInvitation(inv: WorkspaceInvitation) {
  const ok = await confirm({ title: 'Revoke Invitation', message: `Revoke the invitation for ${inv.email}?`, confirmText: 'Revoke', variant: 'warning' })
  if (!ok) return
  try {
    await withWorkspace(() => workspaceApi.cancelInvitation(inv.id))
    notify.success('Invitation revoked')
    await fetchInvitations()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to revoke')
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
  await Promise.all([fetchMembers(), fetchInvitations()])
})
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>{{ ws?.name || 'Workspace' }} — Members</h1>
        <p v-if="ws?.slug" style="color: var(--text-muted); font-size: 13px; margin-top: 2px;">
          <code>{{ ws.slug }}</code>
        </p>
      </div>
      <div style="display: flex; gap: 8px;">
        <button class="btn btn-secondary" @click="router.push(`/workspaces/${wsId}`)">Workspace Settings</button>
        <button class="btn btn-secondary" @click="router.push('/workspaces')">Back</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <div class="tabs">
        <button :class="['tab', { active: activeTab === 'members' }]" @click="activeTab = 'members'">
          Members ({{ members.length }})
        </button>
        <button :class="['tab', { active: activeTab === 'invitations' }]" @click="activeTab = 'invitations'">
          Invitations ({{ invitations.length }})
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
</style>
