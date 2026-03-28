<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { subscribersApi } from '../../api/subscribers'
import type { Subscriber, SubscriberStatus } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
import { usePagination } from '../../composables/usePagination'
import { useNotificationStore } from '../../stores/notification'
import { useModalSafeClose } from '../../composables/useModalSafeClose'

const router = useRouter()
const notify = useNotificationStore()

const subscribers = ref<Subscriber[]>([])
const loading = ref(true)
const search = ref('')
const statusFilter = ref<SubscriberStatus | ''>('')
let searchTimeout: ReturnType<typeof setTimeout> | null = null

// Create modal
const showCreateModal = ref(false)
const createForm = ref({ email: '', name: '', custom_fields: '' })
const creating = ref(false)

// Import JSON modal
const showImportJsonModal = ref(false)
const importJsonText = ref('')
const importing = ref(false)

// CSV import
const csvFileInput = ref<HTMLInputElement | null>(null)
const importingCsv = ref(false)

const { pageable, goToPage } = usePagination(loadSubscribers)

async function loadSubscribers(page = 0) {
  loading.value = true
  try {
    const res = await subscribersApi.list(page, pageable.value.size, search.value, statusFilter.value || undefined)
    subscribers.value = res.data.data ?? []
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load subscribers')
  } finally {
    loading.value = false
  }
}

function onSearchInput() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => goToPage(0), 300)
}

function onStatusChange() {
  goToPage(0)
}

function openCreate() {
  createForm.value = { email: '', name: '', custom_fields: '' }
  showCreateModal.value = true
}

async function createSubscriber() {
  if (!createForm.value.email.trim()) return
  creating.value = true
  try {
    let customFields: Record<string, any> = {}
    if (createForm.value.custom_fields.trim()) {
      customFields = JSON.parse(createForm.value.custom_fields)
    }
    await subscribersApi.create({
      email: createForm.value.email.trim(),
      name: createForm.value.name.trim(),
      custom_fields: customFields,
    })
    notify.success('Subscriber created')
    showCreateModal.value = false
    await loadSubscribers(pageable.value.current_page)
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      notify.error('Invalid JSON in custom fields')
    } else {
      notify.error(e?.response?.data?.error?.message || 'Failed to create subscriber')
    }
  } finally {
    creating.value = false
  }
}

function openImportJson() {
  importJsonText.value = ''
  showImportJsonModal.value = true
}

async function importJson() {
  if (!importJsonText.value.trim()) return
  importing.value = true
  try {
    const data = JSON.parse(importJsonText.value)
    const subscribers = Array.isArray(data) ? data : [data]
    const res = await subscribersApi.bulkImportJSON(subscribers)
    const result = res.data.data
    notify.success(`Import complete: ${result.created} created, ${result.skipped} skipped out of ${result.total}`)
    showImportJsonModal.value = false
    await loadSubscribers(pageable.value.current_page)
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      notify.error('Invalid JSON format')
    } else {
      notify.error(e?.response?.data?.error?.message || 'Failed to import subscribers')
    }
  } finally {
    importing.value = false
  }
}

function triggerCsvUpload() {
  csvFileInput.value?.click()
}

async function onCsvFileSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  importingCsv.value = true
  try {
    const res = await subscribersApi.bulkImportCSV(file)
    const result = res.data.data
    notify.success(`CSV import complete: ${result.created} created, ${result.skipped} skipped out of ${result.total}`)
    await loadSubscribers(pageable.value.current_page)
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to import CSV')
  } finally {
    importingCsv.value = false
    input.value = ''
  }
}

function statusBadgeClass(status: SubscriberStatus): string {
  switch (status) {
    case 'subscribed': return 'badge badge-primary'
    case 'unsubscribed': return 'badge badge-neutral'
    case 'bounced': return 'badge badge-warning'
    case 'complained': return 'badge badge-warning'
    default: return 'badge'
  }
}

function truncateCustomFields(fields: Record<string, any>): string {
  if (!fields || Object.keys(fields).length === 0) return '-'
  const str = JSON.stringify(fields)
  return str.length > 60 ? str.substring(0, 60) + '...' : str
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

const { watchClickStart: watchCreateStart, confirmClickEnd: confirmCreateEnd } = useModalSafeClose(() => {
  showCreateModal.value = false
})
const { watchClickStart: watchImportStart, confirmClickEnd: confirmImportEnd } = useModalSafeClose(() => {
  showImportJsonModal.value = false
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Subscribers</h1>
      <div style="display: flex; gap: 8px">
        <button class="btn btn-secondary" @click="openImportJson">Import JSON</button>
        <button class="btn btn-secondary" :disabled="importingCsv" @click="triggerCsvUpload">
          {{ importingCsv ? 'Importing...' : 'Import CSV' }}
        </button>
        <button class="btn btn-primary" @click="openCreate">Add Subscriber</button>
        <input ref="csvFileInput" type="file" accept=".csv" style="display: none" @change="onCsvFileSelected" />
      </div>
    </div>

    <div class="card">
      <div class="card-header" style="display: flex; gap: 12px; align-items: center">
        <input
          v-model="search"
          type="text"
          class="form-input"
          placeholder="Search by email or name..."
          style="max-width: 320px"
          @input="onSearchInput"
        />
        <select v-model="statusFilter" class="form-select" style="max-width: 180px" @change="onStatusChange">
          <option value="">All Statuses</option>
          <option value="subscribed">Subscribed</option>
          <option value="unsubscribed">Unsubscribed</option>
          <option value="bounced">Bounced</option>
          <option value="complained">Complained</option>
        </select>
      </div>

      <div v-if="loading" class="loading-page">
        <div class="spinner"></div>
      </div>

      <template v-else>
        <div v-if="subscribers.length === 0" class="empty-state">
          <h3>No Subscribers</h3>
          <p v-if="search || statusFilter">No subscribers matching your filters.</p>
          <p v-else>Add subscribers manually or import them from a file.</p>
        </div>

        <template v-else>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Custom Fields</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="sub in subscribers" :key="sub.id" style="cursor: pointer" @click="router.push(`/subscribers/${sub.id}`)">
                  <td>{{ sub.email }}</td>
                  <td>{{ sub.name || '-' }}</td>
                  <td><span :class="statusBadgeClass(sub.status)">{{ sub.status }}</span></td>
                  <td>{{ truncateCustomFields(sub.custom_fields) }}</td>
                  <td>{{ formatDate(sub.created_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <Pagination :pageable="pageable" @page="goToPage" />
        </template>
      </template>
    </div>

    <!-- Create Subscriber Modal -->
    <div v-if="showCreateModal" class="modal-overlay" @mousedown="watchCreateStart" @mouseup="confirmCreateEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h2>Add Subscriber</h2>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Email</label>
            <input v-model="createForm.email" type="email" class="form-input" placeholder="user@example.com" />
          </div>
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="createForm.name" type="text" class="form-input" placeholder="Name (optional)" />
          </div>
          <div class="form-group">
            <label class="form-label">Custom Fields (JSON)</label>
            <textarea v-model="createForm.custom_fields" class="form-input" rows="4" placeholder='{"company": "Acme", "plan": "pro"}'></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showCreateModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="creating || !createForm.email.trim()" @click="createSubscriber">
            {{ creating ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Import JSON Modal -->
    <div v-if="showImportJsonModal" class="modal-overlay" @mousedown="watchImportStart" @mouseup="confirmImportEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h2>Import Subscribers (JSON)</h2>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">JSON Data</label>
            <textarea v-model="importJsonText" class="form-input" rows="10" placeholder='[{"email": "user@example.com", "name": "User"}]'></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showImportJsonModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="importing || !importJsonText.trim()" @click="importJson">
            {{ importing ? 'Importing...' : 'Import' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
