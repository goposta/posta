<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { workspaceApi } from '../../api/workspaces'
import type { Workspace, WorkspaceInvitation } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useWorkspaceStore } from '../../stores/workspace'

const router = useRouter()
const notify = useNotificationStore()
const wsStore = useWorkspaceStore()

const workspaces = ref<Workspace[]>([])
const invitations = ref<WorkspaceInvitation[]>([])
const loading = ref(true)

const showCreateModal = ref(false)
const newName = ref('')
const newSlug = ref('')
const newDescription = ref('')
const creating = ref(false)

async function fetchData() {
  loading.value = true
  try {
    const [wsRes, invRes] = await Promise.all([
      workspaceApi.list(),
      workspaceApi.myInvitations(),
    ])
    workspaces.value = wsRes.data.data ?? []
    invitations.value = invRes.data.data ?? []
  } catch {
    notify.error('Failed to load workspaces')
  } finally {
    loading.value = false
  }
}

function autoSlug() {
  newSlug.value = newName.value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

async function createWorkspace() {
  if (!newName.value.trim() || !newSlug.value.trim()) return
  creating.value = true
  try {
    await workspaceApi.create({ name: newName.value.trim(), slug: newSlug.value.trim(), description: newDescription.value.trim() })
    notify.success('Workspace created')
    showCreateModal.value = false
    newName.value = ''
    newSlug.value = ''
    newDescription.value = ''
    await fetchData()
    await wsStore.fetchWorkspaces()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to create workspace')
  } finally {
    creating.value = false
  }
}

async function acceptInvitation(inv: WorkspaceInvitation) {
  try {
    await workspaceApi.acceptInvitationById(inv.id)
    notify.success(`Joined workspace "${inv.workspace}"`)
    await fetchData()
    await wsStore.fetchWorkspaces()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to accept invitation')
  }
}

async function declineInvitation(inv: WorkspaceInvitation) {
  try {
    await workspaceApi.declineInvitationById(inv.id)
    notify.success('Invitation declined')
    invitations.value = invitations.value.filter(i => i.id !== inv.id)
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to decline invitation')
  }
}

function switchToWorkspace(ws: Workspace) {
  wsStore.setWorkspace(ws.id)
  router.push('/')
}

function viewWorkspace(ws: Workspace) {
  router.push(`/workspaces/${ws.id}`)
}

function roleBadgeClass(role: string): string {
  if (role === 'owner') return 'badge-primary'
  if (role === 'admin') return 'badge-warning'
  return 'badge-neutral'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

onMounted(fetchData)
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Workspaces</h1>
      <button class="btn btn-primary" @click="showCreateModal = true">Create Workspace</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <!-- Pending Invitations -->
      <div v-if="invitations.length > 0" class="card" style="margin-bottom: 20px;">
        <div class="card-header" style="display: flex; align-items: center; justify-content: space-between; padding: 14px 20px; border-bottom: 1px solid var(--border-primary); font-weight: 600; font-size: 14px;">
          <span>Pending Invitations ({{ invitations.length }})</span>
        </div>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Workspace</th>
                <th>Role</th>
                <th>Expires</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="inv in invitations" :key="inv.id">
                <td style="font-weight: 500;">{{ inv.workspace }}</td>
                <td><span class="badge" :class="roleBadgeClass(inv.role)">{{ inv.role }}</span></td>
                <td>{{ formatDate(inv.expires_at) }}</td>
                <td>
                  <div style="display: flex; gap: 6px;">
                    <button class="btn btn-primary btn-sm" @click="acceptInvitation(inv)">Accept</button>
                    <button class="btn btn-secondary btn-sm" @click="declineInvitation(inv)">Decline</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Workspaces List -->
      <div class="card">
        <div v-if="workspaces.length === 0 && invitations.length === 0" class="empty-state">
          <h3>No workspaces</h3>
          <p>Create a workspace to share resources with your team.</p>
        </div>

        <template v-else-if="workspaces.length > 0">
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Slug</th>
                  <th>Your Role</th>
                  <th>Created</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="ws in workspaces" :key="ws.id">
                  <td style="font-weight: 500;">{{ ws.name }}</td>
                  <td><code>{{ ws.slug }}</code></td>
                  <td>
                    <span class="badge" :class="roleBadgeClass(ws.role)">{{ ws.role }}</span>
                  </td>
                  <td>{{ formatDate(ws.created_at) }}</td>
                  <td>
                    <div style="display: flex; gap: 6px;">
                      <button
                        class="btn btn-primary btn-sm"
                        :class="{ 'btn-success': wsStore.currentWorkspaceId === ws.id }"
                        @click="switchToWorkspace(ws)"
                      >
                        {{ wsStore.currentWorkspaceId === ws.id ? 'Active' : 'Switch' }}
                      </button>
                      <button class="btn btn-secondary btn-sm" @click="viewWorkspace(ws)">Manage</button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>

        <div v-else class="empty-state">
          <h3>No workspaces yet</h3>
          <p>You have pending invitations above. Accept one to get started, or create your own workspace.</p>
        </div>
      </div>
    </template>

    <!-- Create Workspace Modal -->
    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal = false">
      <div class="modal">
        <div class="modal-header">
          <h3>Create Workspace</h3>
        </div>
        <form @submit.prevent="createWorkspace">
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Name</label>
              <input
                v-model="newName"
                class="form-input"
                placeholder="My Team"
                required
                @input="autoSlug"
              />
            </div>
            <div class="form-group">
              <label class="form-label">Slug</label>
              <input
                v-model="newSlug"
                class="form-input"
                placeholder="my-team"
                pattern="[a-z0-9]+(-[a-z0-9]+)*"
                required
              />
              <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block;">
                URL-friendly identifier. Lowercase letters, numbers, and hyphens only.
              </small>
            </div>
            <div class="form-group">
              <label class="form-label">Description (optional)</label>
              <input
                v-model="newDescription"
                class="form-input"
                placeholder="What is this workspace for?"
              />
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showCreateModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="creating || !newName.trim() || !newSlug.trim()">
              {{ creating ? 'Creating...' : 'Create' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
