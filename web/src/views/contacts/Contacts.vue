<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { contactsApi } from '../../api/contacts'
import { suppressionsApi } from '../../api/bounces'
import { contactListsApi } from '../../api/contactLists'
import type { Contact, ContactListWithCount } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
import { usePagination } from '../../composables/usePagination'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const contacts = ref<Contact[]>([])
const loading = ref(true)
const suppressingEmail = ref('')
const search = ref('')
let searchTimeout: ReturnType<typeof setTimeout> | null = null

// Add to list modal
const showAddToListModal = ref(false)
const addToListContact = ref<Contact | null>(null)
const contactLists = ref<ContactListWithCount[]>([])
const selectedListId = ref<number | null>(null)
const addingToList = ref(false)

const { pageable, goToPage } = usePagination(loadContacts)

async function loadContacts(page = 0) {
  loading.value = true
  try {
    const res = await contactsApi.list(page, pageable.value.size, search.value)
    contacts.value = res.data.data
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load contacts')
  } finally {
    loading.value = false
  }
}

function onSearchInput() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => goToPage(0), 300)
}

async function suppressContact(contact: Contact) {
  const confirmed = await confirm({
    title: 'Suppress Contact',
    message: `Are you sure you want to suppress "${contact.email}"? This contact will no longer receive emails.`,
    confirmText: 'Suppress',
    variant: 'warning',
  })
  if (!confirmed) return
  suppressingEmail.value = contact.email
  try {
    await suppressionsApi.create({ email: contact.email, reason: 'Manually suppressed from contacts' })
    notify.success(`${contact.email} has been suppressed.`)
    await loadContacts(pageable.value.current_page)
  } catch (err: any) {
    const message = err?.response?.data?.error?.message || err?.response?.data?.error || 'Failed to suppress contact.'
    notify.error(message)
  } finally {
    suppressingEmail.value = ''
  }
}

async function openAddToList(contact: Contact) {
  addToListContact.value = contact
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
  if (!selectedListId.value || !addToListContact.value) return
  addingToList.value = true
  try {
    await contactListsApi.addMember(selectedListId.value, addToListContact.value.email, addToListContact.value.name || '')
    notify.success(`${addToListContact.value.email} added to list.`)
    showAddToListModal.value = false
  } catch (err: any) {
    notify.error(err?.response?.data?.error?.message || 'Failed to add to list')
  } finally {
    addingToList.value = false
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showAddToListModal.value = false;
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Contacts</h1>
    </div>

    <div class="card">
      <div class="card-header">
        <input
          v-model="search"
          type="text"
          class="form-input"
          placeholder="Search by email or name..."
          style="max-width: 320px"
          @input="onSearchInput"
        />
      </div>

      <div v-if="loading" class="loading-page">
        <div class="spinner"></div>
      </div>

      <template v-else>
        <div v-if="contacts.length === 0" class="empty-state">
          <h3>No Contacts</h3>
          <p v-if="search">No contacts matching "{{ search }}".</p>
          <p v-else>Contacts will appear here once you start sending emails.</p>
        </div>

        <template v-else>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Name</th>
                  <th>Sent</th>
                  <th>Failed</th>
                  <th>Last Sent</th>
                  <th>First Seen</th>
                  <th style="width: 1%"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="contact in contacts" :key="contact.id" style="cursor: pointer" @click="router.push(`/contacts/${contact.id}`)">
                  <td>
                    {{ contact.email }}
                    <span v-if="contact.suppressed" class="badge badge-warning">Suppressed</span>
                  </td>
                  <td>{{ contact.name || '-' }}</td>
                  <td>{{ contact.sent_count }}</td>
                  <td>{{ contact.fail_count }}</td>
                  <td>{{ contact.last_sent_at ? formatDate(contact.last_sent_at) : 'Never' }}</td>
                  <td>{{ formatDate(contact.created_at) }}</td>
                  <td>
                    <div style="display: flex; gap: 6px; white-space: nowrap" @click.stop>
                      <button class="btn btn-secondary btn-sm" @click="openAddToList(contact)">Add to List</button>
                      <button
                        v-if="!contact.suppressed"
                        class="btn btn-danger btn-sm"
                        :disabled="suppressingEmail === contact.email"
                        @click="suppressContact(contact)"
                      >
                        {{ suppressingEmail === contact.email ? 'Suppressing...' : 'Suppress' }}
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <Pagination :pageable="pageable" @page="goToPage" />
        </template>
      </template>
    </div>

    <!-- Add to Contact List Modal -->
    <div v-if="showAddToListModal" class="modal-overlay" @mousedown="watchClickStart" 
      @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h2>Add to Contact List</h2>
        </div>
        <div class="modal-body">
          <p style="margin-bottom: 12px">Add <strong>{{ addToListContact?.email }}</strong> to a contact list:</p>
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
