<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { contactListsApi } from '../../api/contactLists'
import type { ContactListWithCount, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';
import { useWorkspaceStore } from '../../stores/workspace'
import { usePagination } from '@/composables/usePagination'
import Pagination from '@/components/Pagination.vue'


const router = useRouter()
const notify = useNotificationStore()
const wsStore = useWorkspaceStore()
const { confirm } = useConfirm()

const lists = ref<ContactListWithCount[]>([])
const loading = ref(true)

// Create/Edit modal
const showModal = ref(false)
const editingId = ref<number | null>(null)
const formName = ref('')
const formDescription = ref('')
const saving = ref(false)

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await contactListsApi.list(page, pageable.value.size)
    lists.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load contact lists', e)
  } finally {
    loading.value = false
  }
})


function openCreate() {
  editingId.value = null
  formName.value = ''
  formDescription.value = ''
  showModal.value = true
}

function openEdit(list: ContactListWithCount) {
  editingId.value = list.id
  formName.value = list.name
  formDescription.value = list.description
  showModal.value = true
}

async function saveList() {
  if (!formName.value.trim()) return
  saving.value = true
  try {
    if (editingId.value) {
      await contactListsApi.update(editingId.value, formName.value.trim(), formDescription.value.trim())
      notify.success('Contact list updated')
    } else {
      await contactListsApi.create(formName.value.trim(), formDescription.value.trim())
      notify.success('Contact list created')
    }
    showModal.value = false
    await goToPage(pageable.value.current_page)
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to save contact list')
  } finally {
    saving.value = false
  }
}

async function deleteList(list: ContactListWithCount) {
  const confirmed = await confirm({
    title: 'Delete Contact List',
    message: `Are you sure you want to delete "${list.name}"? All members will be removed.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await contactListsApi.delete(list.id)
    notify.success('Contact list deleted')
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to delete contact list')
  }
}

function openMembers(list: ContactListWithCount) {
  router.push({ name: 'contact-list-members', params: { id: list.id } })
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false;
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Contact Lists</h1>
      <button v-if="wsStore.canEdit" class="btn btn-primary" @click="openCreate">Create List</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="lists.length === 0" class="empty-state">
        <h3>No Contact Lists</h3>
        <p>Create a contact list to organize your recipients.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Description</th>
                <th>Members</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="list in lists" :key="list.id">
                <td>
                  <a class="link" @click="openMembers(list)">{{ list.name }}</a>
                </td>
                <td>{{ list.description || '-' }}</td>
                <td>{{ list.member_count }}</td>
                <td>{{ formatDate(list.created_at) }}</td>
                <td>
                  <div style="display: flex; gap: 6px">
                    <button class="btn btn-secondary btn-sm" @click="openMembers(list)">Members</button>
                    <button v-if="wsStore.canEdit" class="btn btn-secondary btn-sm" @click="openEdit(list)">Edit</button>
                    <button v-if="wsStore.canEdit" class="btn btn-danger btn-sm" @click="deleteList(list)">Delete</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

  <Pagination :pageable="pageable" @page="goToPage" />
      
      </template>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>{{ editingId ? 'Edit Contact List' : 'Create Contact List' }}</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="formName" class="form-input" placeholder="e.g. Newsletter Subscribers" @keyup.enter="saveList" />
          </div>
          <div class="form-group">
            <label class="form-label">Description</label>
            <input v-model="formDescription" class="form-input" placeholder="Optional description" />
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="saving || !formName.trim()" @click="saveList">
            {{ saving ? 'Saving...' : (editingId ? 'Update' : 'Create') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.link {
  color: var(--primary-600);
  cursor: pointer;
  font-weight: 500;
}
.link:hover {
  text-decoration: underline;
}
</style>
