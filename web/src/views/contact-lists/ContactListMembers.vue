<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { contactListsApi } from '../../api/contactLists'
import type { ContactListWithCount, ContactListMember, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const listId = Number(route.params.id)
const list = ref<ContactListWithCount | null>(null)
const members = ref<ContactListMember[]>([])
const pageable = ref<Pageable>({ current_page: 0, size: 20, total_pages: 0, total_elements: 0, empty: true })
const loading = ref(true)

const showAddModal = ref(false)
const addForm = ref({ email: '', name: '' })
const addingMember = ref(false)

async function loadList() {
  try {
    const res = await contactListsApi.list(0, 100)
    list.value = res.data.data.find((l) => l.id === listId) || null
    if (!list.value) {
      notify.error('Contact list not found')
      router.push({ name: 'contact-lists' })
    }
  } catch {
    notify.error('Failed to load contact list')
    router.push({ name: 'contact-lists' })
  }
}

async function loadMembers(page = 0) {
  loading.value = true
  try {
    const res = await contactListsApi.listMembers(listId, page, pageable.value.size)
    members.value = res.data.data
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load members')
  } finally {
    loading.value = false
  }
}

function openAddModal() {
  addForm.value = { email: '', name: '' }
  showAddModal.value = true
}

async function addMember() {
  if (!addForm.value.email.trim()) return
  addingMember.value = true
  try {
    await contactListsApi.addMember(listId, addForm.value.email.trim(), addForm.value.name.trim())
    notify.success('Member added')
    showAddModal.value = false
    await loadMembers(pageable.value.current_page)
    await loadList()
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to add member')
  } finally {
    addingMember.value = false
  }
}

async function removeMember(member: ContactListMember) {
  const confirmed = await confirm({
    title: 'Remove Member',
    message: `Remove "${member.email}" from this list?`,
    confirmText: 'Remove',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await contactListsApi.removeMember(listId, member.email)
    notify.success('Member removed')
    await loadMembers(pageable.value.current_page)
    await loadList()
  } catch {
    notify.error('Failed to remove member')
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showAddModal.value = false;
});
onMounted(async () => {
  await loadList()
  await loadMembers()
})
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <div class="breadcrumb">
          <router-link :to="{ name: 'contact-lists' }">Contact Lists</router-link>
          <span class="separator">/</span>
          <span>{{ list?.name || '...' }}</span>
        </div>
        <h1>{{ list?.name || 'Members' }}</h1>
        <p v-if="list?.description" class="page-description">{{ list.description }}</p>
      </div>
      <button class="btn btn-primary" @click="openAddModal">Add Member</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="members.length === 0" class="empty-state">
        <h3>No Members</h3>
        <p>Add members to this contact list.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Email</th>
                <th>Name</th>
                <th>Added</th>
                <th style="width: 1%"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="member in members" :key="member.id">
                <td>{{ member.email }}</td>
                <td>{{ member.name || '-' }}</td>
                <td>{{ formatDate(member.created_at) }}</td>
                <td>
                  <button class="btn btn-danger btn-sm" @click="removeMember(member)">Remove</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="pagination">
          <span class="pagination-info">
            Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }} ({{ pageable.total_elements }} members)
          </span>
          <div class="pagination-buttons">
            <button class="btn btn-secondary btn-sm" :disabled="pageable.current_page === 0" @click="loadMembers(pageable.current_page - 1)">Previous</button>
            <button class="btn btn-secondary btn-sm" :disabled="pageable.current_page >= pageable.total_pages - 1" @click="loadMembers(pageable.current_page + 1)">Next</button>
          </div>
        </div>
      </template>
    </div>

    <!-- Add Member Modal -->
    <div v-if="showAddModal" class="modal-overlay" @mousedown="watchClickStart" 
      @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h2>Add Member</h2>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Email</label>
            <input v-model="addForm.email" type="email" class="form-input" placeholder="user@example.com" />
          </div>
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="addForm.name" type="text" class="form-input" placeholder="Name (optional)" />
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showAddModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="addingMember || !addForm.email.trim()" @click="addMember">
            {{ addingMember ? 'Adding...' : 'Add' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  margin-bottom: 4px;
  color: var(--text-secondary);
}

.breadcrumb a {
  color: var(--primary-600);
  text-decoration: none;
}

.breadcrumb a:hover {
  text-decoration: underline;
}

.breadcrumb .separator {
  color: var(--text-tertiary);
}

.page-description {
  color: var(--text-secondary);
  margin-top: 4px;
  font-size: 14px;
}
</style>
