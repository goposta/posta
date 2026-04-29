<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '../../api/admin'
import type { User, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';
import { usePagination } from '@/composables/usePagination'
import Pagination from '@/components/Pagination.vue'


const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()
const loading = ref(true)
const users = ref<User[]>([])
const page = ref(0)

const showCreateForm = ref(false)
const createLoading = ref(false)
const newUser = ref({ name: '', email: '', password: '', role: 'user' })

const editingUser = ref<User | null>(null)
const editForm = ref({ role: '', active: true })
const editLoading = ref(false)

async function createUser() {
  if (!newUser.value.email || !newUser.value.password) {
    notify.error('Email and password are required')
    return
  }
  if (newUser.value.password.length < 8) {
    notify.error('Password must be at least 8 characters')
    return
  }
  createLoading.value = true
  try {
    await adminApi.createUser(newUser.value.name, newUser.value.email, newUser.value.password, newUser.value.role)
    notify.success('User created successfully')
    showCreateForm.value = false
    newUser.value = { name: '', email: '', password: '', role: 'user' }
    await goToPage(pageable.value.current_page)
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to create user'
    notify.error(message)
  } finally {
    createLoading.value = false
  }
}


const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await adminApi.listUsers(page)
    users.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load API users', e)
  } finally {
    loading.value = false
  }
})

function roleBadgeClass(role: string) {
  switch (role) {
    case 'admin': return 'badge badge-info'
    case 'user': return 'badge badge-neutral'
    default: return 'badge'
  }
}

function formatDate(date: string) {
  return new Date(date).toLocaleString()
}

function startEdit(user: User) {
  editingUser.value = user
  editForm.value = { role: user.role, active: user.active }
}

function cancelEdit() {
  editingUser.value = null
}

async function saveEdit() {
  if (!editingUser.value) return
  editLoading.value = true
  try {
    const res = await adminApi.updateUser(editingUser.value.id, editForm.value)
    const updated = res.data.data
    const idx = users.value.findIndex(u => u.id === updated.id)
    if (idx !== -1) users.value[idx] = updated
    editingUser.value = null
    notify.success('User updated')
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to update user'
    notify.error(message)
  } finally {
    editLoading.value = false
  }
}

async function toggleActive(user: User) {
  try {
    const res = await adminApi.updateUser(user.id, { active: !user.active })
    const updated = res.data.data
    const idx = users.value.findIndex(u => u.id === updated.id)
    if (idx !== -1) users.value[idx] = updated
    notify.success(updated.active ? 'User enabled' : 'User disabled')
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to update user'
    notify.error(message)
  }
}

async function deleteUser(user: User) {
  const confirmed = await confirm({
    title: 'Delete User',
    message: `Are you sure you want to delete "${user.email}"? The account will be disabled immediately and permanently deleted after 7 days.`,
    confirmText: 'Delete User',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await adminApi.deleteUser(user.id)
    notify.success('Account disabled and scheduled for deletion.')
    await goToPage(pageable.value.current_page)
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to delete user'
    notify.error(message)
  }
}
async function forceDeleteUser(user: User) {
  const confirmed = await confirm({
    title: 'Force Delete User',
    message: `Are you sure you want to permanently delete "${user.email}"? This will immediately remove the user and all their data. This action cannot be undone.`,
    confirmText: 'Force Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await adminApi.forceDeleteUser(user.id)
    notify.success('User permanently deleted.')
    await goToPage(pageable.value.current_page)
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to force delete user'
    notify.error(message)
  }
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  cancelEdit()
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>User Management</h1>
      <button class="btn btn-primary" @click="showCreateForm = !showCreateForm">
        {{ showCreateForm ? 'Cancel' : 'Create User' }}
      </button>
    </div>

    <div v-if="showCreateForm" class="card" style="margin-bottom: 1.5rem;">
      <div class="card-header"><h2>Create New User</h2></div>
      <div class="card-body">
        <form @submit.prevent="createUser" style="display: grid; gap: 1rem; max-width: 480px;">
          <div class="form-group">
            <label class="form-label" for="new-name">Name</label>
            <input id="new-name" v-model="newUser.name" type="text" class="form-input" placeholder="Full name" />
          </div>
          <div class="form-group">
            <label class="form-label" for="new-email">Email</label>
            <input id="new-email" v-model="newUser.email" type="email" class="form-input" placeholder="user@example.com" required />
          </div>
          <div class="form-group">
            <label class="form-label" for="new-password">Password</label>
            <input id="new-password" v-model="newUser.password" type="password" class="form-input" placeholder="Minimum 8 characters" required minlength="8" />
          </div>
          <div class="form-group">
            <label class="form-label" for="new-role">Role</label>
            <select id="new-role" v-model="newUser.role" class="form-select">
              <option value="user">user</option>
              <option value="admin">admin</option>
            </select>
          </div>
          <button type="submit" class="btn btn-primary" :disabled="createLoading">
            {{ createLoading ? 'Creating...' : 'Create User' }}
          </button>
        </form>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <div class="card">
        <div class="card-header">
          <h2>Users</h2>
        </div>
        <div v-if="users.length === 0" class="empty-state">
          <h3>No users found</h3>
          <p>There are no registered users.</p>
        </div>
        <div v-else class="card-body">
          <table class="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Email</th>
                <th>Role</th>
                <th>Status</th>
                <th>Created At</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="user in users" :key="user.id">
                <td>{{ user.name }}</td>
                <td>{{ user.email }}</td>
                <td><span :class="roleBadgeClass(user.role)">{{ user.role }}</span></td>
                <td>
                  <span :class="user.active ? 'badge badge-success' : 'badge badge-danger'">
                    {{ user.active ? 'Active' : 'Disabled' }}
                  </span>
                </td>
                <td>{{ formatDate(user.created_at) }}</td>
                <td style="display: flex; gap: 0.5rem; align-items: center;">
                  <button class="btn btn-sm btn-secondary" @click="startEdit(user)">Edit</button>
                  <button class="btn btn-sm btn-secondary" @click="router.push(`/admin/users/${user.id}`)">Details</button>
                  <button
                    class="btn btn-sm"
                    :class="user.active ? 'btn-warning' : 'btn-primary'"
                    @click="toggleActive(user)"
                  >
                    {{ user.active ? 'Disable' : 'Enable' }}
                  </button>
                  <button class="btn btn-sm btn-danger" @click="deleteUser(user)">Delete</button>
                  <button v-if="!user.active" class="btn btn-sm btn-danger" @click="forceDeleteUser(user)">Force Delete</button>
                </td>
              </tr>
            </tbody>
          </table>
  <Pagination :pageable="pageable" @page="goToPage" />
          
        </div>
      </div>
    </template>

    <!-- Edit User Modal -->
    <div v-if="editingUser" class="modal-overlay" @mousedown="watchClickStart" 
      @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Edit User</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Role</label>
            <select v-model="editForm.role" class="form-select">
              <option value="user">user</option>
              <option value="admin">admin</option>
            </select>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="cancelEdit">Cancel</button>
          <button class="btn btn-primary" :disabled="editLoading" @click="saveEdit">
            {{ editLoading ? 'Saving...' : 'Save' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
