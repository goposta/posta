<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { contactsApi } from '../../api/contacts'
import { suppressionsApi } from '../../api/bounces'
import { contactListsApi } from '../../api/contactLists'
import type { Contact, ContactListWithCount } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const loading = ref(true)
const contact = ref<Contact | null>(null)
const suppressingEmail = ref('')

// Add to list modal
const showAddToListModal = ref(false)
const contactLists = ref<ContactListWithCount[]>([])
const selectedListId = ref<number | null>(null)
const addingToList = ref(false)

onMounted(async () => {
  try {
    const id = Number(route.params.id)
    const res = await contactsApi.get(id)
    contact.value = res.data.data
  } catch {
    notify.error('Failed to load contact')
  } finally {
    loading.value = false
  }
})

async function suppressContact() {
  if (!contact.value) return
  const confirmed = await confirm({
    title: 'Suppress Contact',
    message: `Are you sure you want to suppress "${contact.value.email}"? This contact will no longer receive emails.`,
    confirmText: 'Suppress',
    variant: 'warning',
  })
  if (!confirmed) return
  suppressingEmail.value = contact.value.email
  try {
    await suppressionsApi.create({ email: contact.value.email, reason: 'Manually suppressed from contacts' })
    notify.success(`${contact.value.email} has been suppressed.`)
    contact.value = { ...contact.value, suppressed: true }
  } catch (err: any) {
    const message = err?.response?.data?.error?.message || err?.response?.data?.error || 'Failed to suppress contact.'
    notify.error(message)
  } finally {
    suppressingEmail.value = ''
  }
}

async function openAddToList() {
  selectedListId.value = null
  showAddToListModal.value = true
  try {
    const res = await contactListsApi.list(0, 100)
    contactLists.value = res.data.data
  } catch {
    notify.error('Failed to load contact lists')
  }
}

async function addToList() {
  if (!selectedListId.value || !contact.value) return
  addingToList.value = true
  try {
    await contactListsApi.addMember(selectedListId.value, contact.value.email, contact.value.name || '')
    notify.success(`${contact.value.email} added to list.`)
    showAddToListModal.value = false
  } catch (err: any) {
    notify.error(err?.response?.data?.error?.message || 'Failed to add to list')
  } finally {
    addingToList.value = false
  }
}

function formatDate(date: string | null) {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showAddToListModal.value = false;
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Contact Detail</h1>
      <div style="display: flex; gap: 8px">
        <button class="btn btn-secondary" @click="router.push('/contacts')">Back to Contacts</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="contact">
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>{{ contact.email }}</h2>
          <span v-if="contact.suppressed" class="badge badge-warning">Suppressed</span>
        </div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 140px">Email</td>
                <td>{{ contact.email }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Name</td>
                <td>{{ contact.name || '-' }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Sent Count</td>
                <td>{{ contact.sent_count }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Failed Count</td>
                <td>{{ contact.fail_count }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Last Sent</td>
                <td>{{ formatDate(contact.last_sent_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">First Seen</td>
                <td>{{ formatDate(contact.created_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Status</td>
                <td>
                  <span v-if="contact.suppressed" class="badge badge-warning">Suppressed</span>
                  <span v-else class="badge badge-success">Active</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h2>Actions</h2>
        </div>
        <div class="card-body" style="display: flex; gap: 8px">
          <button class="btn btn-secondary" @click="openAddToList">Add to List</button>
          <button
            v-if="!contact.suppressed"
            class="btn btn-danger"
            :disabled="suppressingEmail === contact.email"
            @click="suppressContact"
          >
            {{ suppressingEmail === contact.email ? 'Suppressing...' : 'Suppress' }}
          </button>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>Contact not found</h3>
      <p>The contact you are looking for does not exist.</p>
    </div>

    <!-- Add to Contact List Modal -->
    <div v-if="showAddToListModal" class="modal-overlay" @mousedown="watchClickStart" 
      @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h2>Add to Contact List</h2>
        </div>
        <div class="modal-body">
          <p style="margin-bottom: 12px">Add <strong>{{ contact?.email }}</strong> to a contact list:</p>
          <div class="form-group">
            <label class="form-label">Contact List</label>
            <select v-model="selectedListId" class="form-input">
              <option :value="null" disabled>Select a list...</option>
              <option v-for="list in contactLists" :key="list.id" :value="list.id">
                {{ list.name }} ({{ list.member_count }} members)
              </option>
            </select>
          </div>
          <div v-if="contactLists.length === 0" class="empty-hint">
            No contact lists found. Create one first.
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showAddToListModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="addingToList || !selectedListId" @click="addToList">
            {{ addingToList ? 'Adding...' : 'Add' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.empty-hint {
  color: var(--text-secondary);
  font-size: 13px;
  font-style: italic;
}
</style>
